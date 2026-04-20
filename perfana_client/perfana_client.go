package perfana_client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"perfana-cli/util"
	"time"
)

// PerfanaEvent represents the structure of the JSON payload for the /api/events endpoint
type PerfanaEvent struct {
	SystemUnderTest string   `json:"systemUnderTest"`
	TestEnvironment string   `json:"testEnvironment"`
	Workload        string   `json:"workload"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags,omitempty"`
}

// PerfanaMessage represents the JSON payload sent to start a session
type PerfanaMessage struct {
	TestRunID           string     `json:"testRunId"`
	Workload            string     `json:"workload"`
	TestEnvironment     string     `json:"testEnvironment"`
	SystemUnderTest     string     `json:"systemUnderTest"`
	Version             string     `json:"version,omitempty"`             // Optional
	CIBuildResultsURL   string     `json:"CIBuildResultsUrl,omitempty"`   // Optional
	AnalysisStartOffset int        `json:"analysisStartOffset,omitempty"` // Optional, seconds
	Duration            int        `json:"duration,omitempty"`            // Optional, seconds
	Completed           bool       `json:"completed"`
	Abort               bool       `json:"abort,omitempty"`
	Annotations         string     `json:"annotations,omitempty"` // Optional
	Tags                []string   `json:"tags,omitempty"`        // Optional
	Variables           []Variable `json:"variables,omitempty"`   // Optional
	DeepLinks           []DeepLink `json:"deepLinks,omitempty"`   // Optional
}

// Variable is used in PerfanaMessage to send key-value pairs
type Variable struct {
	Placeholder string `json:"placeholder"`
	Value       string `json:"value"`
}

// DeepLink is used in PerfanaMessage to send links
type DeepLink struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Type       string `json:"type"`
	PluginName string `json:"pluginName"`
}

// TestRunResult represents the response from the test run status API.
type TestRunResult struct {
	ID                   string   `json:"id"`
	TestRunID            string   `json:"test_run_id"`
	TestEnvironment      string   `json:"test_environment"`
	Workload             string   `json:"workload"`
	StartTime            string   `json:"start_time"`
	EndTime              string   `json:"end_time"`
	Duration             int      `json:"duration"`
	PlannedDuration      int      `json:"planned_duration"`
	AnalysisStartOffset  int      `json:"analysis_start_offset"`
	Completed            bool     `json:"completed"`
	Abort                bool     `json:"abort"`
	Valid                bool     `json:"valid"`
	CompletionPercentage int      `json:"completion_percentage"`
	Status *struct {
		LastUpdate      string `json:"lastUpdate"`
		EvaluatingAdapt string `json:"evaluatingAdapt"`
		EvaluatingChecks string `json:"evaluatingChecks"`
	} `json:"status"`
	ConsolidatedResult *struct {
		Overall         bool `json:"overall"`
		MeetsRequirement bool `json:"meetsRequirement"`
	} `json:"consolidated_result"`
	ApplicationRelease   string   `json:"application_release"`
	Tags                 []string `json:"tags"`
	Annotations          []string `json:"annotations"`
	IsChangepoint        bool     `json:"is_changepoint"`
	AdaptConfig          struct {
		Mode string `json:"mode"`
	} `json:"adapt_config"`
	SystemsUnderTest     struct {
		Name string `json:"name"`
	} `json:"systems_under_test"`
}

// CheckRequirement holds the operator and threshold for a check.
type CheckRequirement struct {
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
}

// CheckTarget holds per-target result data.
type CheckTarget struct {
	Target          string  `json:"target"`
	Value           float64 `json:"value"`
	MeetsRequirement bool   `json:"meets_requirement"`
}

// CheckResult represents a single SLO check result.
type CheckResult struct {
	DashboardLabel   string           `json:"dashboard_label"`
	PanelTitle       string           `json:"panel_title"`
	MetricUnit       string           `json:"metric_unit"`
	Message          string           `json:"message"`
	MeetsRequirement bool             `json:"meets_requirement"`
	PanelAverage     string           `json:"panel_average"`
	Requirement      CheckRequirement `json:"requirement"`
	Targets          []CheckTarget    `json:"targets"`
}

// AdaptMetric holds a single regression/improvement/difference entry.
type AdaptMetric struct {
	MetricName     string  `json:"metric_name"`
	Dashboard      string  `json:"dashboard"`
	Panel          string  `json:"panel"`
	Unit           string  `json:"unit"`
	Current        float64 `json:"current"`
	Baseline       float64 `json:"baseline"`
	ChangePct      float64 `json:"change_pct"`
	AbsoluteChange float64 `json:"absolute_change"`
	Conclusion     string  `json:"conclusion"`
}

// AdaptConclusion holds the enriched adapt conclusion for a test run.
type AdaptConclusion struct {
	TestRunID      string        `json:"test_run_id"`
	Conclusion     string        `json:"conclusion"`
	ControlGroupID string        `json:"control_group_id"`
	UpdatedAt      string        `json:"updated_at"`
	Regressions    []AdaptMetric `json:"regressions"`
	Improvements   []AdaptMetric `json:"improvements"`
	Differences    []AdaptMetric `json:"differences"`
}

// PerfanaClient is the client implementation for Perfana
type PerfanaClient struct {
	httpClient *http.Client
	config     Configuration
}

// NewClient initializes and returns a Perfana client
func NewClient(config Configuration) (*PerfanaClient, error) {
	if config.ApiUrl == "" {
		return nil, errors.New("apiUrl is required")
	}

	if !config.MTLS.Enabled {
		// Default HTTP Client
		httpClient := &http.Client{
			Timeout: 30 * time.Second,
		}
		return &PerfanaClient{
			httpClient: httpClient,
			config:     config,
		}, nil
	} else {
		tlsClient, err := createTLSClient(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS client: %w", err)
		}
		return &PerfanaClient{
			httpClient: tlsClient,
			config:     config,
		}, nil
	}
}

// createTLSClient sets up a HTTP client with mutual TLS
func createTLSClient(config Configuration) (*http.Client, error) {
	// Load client certificate and key from PEM strings
	cert, err := tls.X509KeyPair([]byte(config.MTLS.ClientCert), []byte(config.MTLS.ClientKey))
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate and key: %w", err)
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false, // Ensure certificate validation
	}

	// Create a transport with TLS configuration
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Return a client with the transport
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}, nil
}

// Init performs a POST request to /api/init and starts a test run.
// It sends systemUnderTest, environment, and workload in the JSON payload
// and receives a testRunId in the response.
func (c *PerfanaClient) Init() (string, error) {
	url := fmt.Sprintf("%s/api/init", c.config.ApiUrl)

	// Prepare the request body
	reqBody, err := json.Marshal(map[string]string{
		"systemUnderTest": c.config.SystemUnderTest,
		"testEnvironment": c.config.Environment,
		"workload":        c.config.Workload,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Make the HTTP request
	resp, err := c.makeRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}

	// Parse the response
	var response struct {
		TestRunID string `json:"testRunId"`
	}
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if response.TestRunID == "" {
		return "", fmt.Errorf("received empty testRunId in the response")
	}

	return response.TestRunID, nil
}

// TestEvent makes a POST request to start a Perfana session
func (c *PerfanaClient) TestEvent(testRunID string, additionalData map[string]interface{}, completed bool) error {
	url := fmt.Sprintf("%s/api/test", c.config.ApiUrl)

	// Create the JSON payload (PerfanaMessage with additional fields as needed)
	message := PerfanaMessage{
		TestRunID:       testRunID,
		Workload:        c.config.Workload,
		TestEnvironment: c.config.Environment,
		SystemUnderTest: c.config.SystemUnderTest,
		Completed:       completed,
	}

	// Add optional values from additionalData map (if provided)
	if version, ok := additionalData["version"]; ok {
		message.Version = version.(string)
	}
	if cibuildResultsUrl, ok := additionalData["cibuildResultsUrl"]; ok {
		message.CIBuildResultsURL = cibuildResultsUrl.(string)
	}
	if analysisStartOffset, ok := additionalData["analysisStartOffset"]; ok {
		seconds, err := normalizeDurationToSeconds(analysisStartOffset)
		if err != nil {
			return fmt.Errorf("invalid analysisStartOffset: %w", err)
		}
		message.AnalysisStartOffset = seconds
	}
	if duration, ok := additionalData["duration"]; ok {
		seconds, err := normalizeDurationToSeconds(duration)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		message.Duration = seconds
	}
	if annotations, ok := additionalData["annotations"]; ok {
		message.Annotations = annotations.(string)
	}
	if tags, ok := additionalData["tags"]; ok {
		message.Tags = tags.([]string)
	}
	if variables, ok := additionalData["variables"]; ok {
		message.Variables = variables.([]Variable)
	}
	if deepLinks, ok := additionalData["deepLinks"]; ok {
		message.DeepLinks = deepLinks.([]DeepLink)
	}

	reqBody, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	_, err = c.makeRequest("POST", url, bytes.NewReader(reqBody))
	return err
}

func normalizeDurationToSeconds(raw interface{}) (int, error) {
	switch v := raw.(type) {
	case int:
		if v < 0 {
			return 0, fmt.Errorf("must be non-negative")
		}
		return v, nil
	case int64:
		if v < 0 {
			return 0, fmt.Errorf("must be non-negative")
		}
		return int(v), nil
	case float64:
		if v < 0 {
			return 0, fmt.Errorf("must be non-negative")
		}
		if v != float64(int(v)) {
			return 0, fmt.Errorf("must be an integer number of seconds")
		}
		return int(v), nil
	case string:
		seconds, err := util.ParseISODurationToSeconds(v)
		if err != nil {
			return 0, err
		}
		return seconds, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", raw)
	}
}

// Shared helper method for HTTP requests
func (c *PerfanaClient) makeRequest(method, url string, body io.Reader) ([]byte, error) {

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle HTTP response errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body) // Read response body for better error messages
		return nil, fmt.Errorf("HTTP error: %s (%d): %s", resp.Status, resp.StatusCode, string(body))
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

// AbortTest sends an abort signal to the Perfana API for the given test run.
func (c *PerfanaClient) AbortTest(testRunID string, additionalData map[string]interface{}) error {
	url := fmt.Sprintf("%s/api/test", c.config.ApiUrl)

	message := PerfanaMessage{
		TestRunID:       testRunID,
		Workload:        c.config.Workload,
		TestEnvironment: c.config.Environment,
		SystemUnderTest: c.config.SystemUnderTest,
		Completed:       false,
		Abort:           true,
	}

	if tags, ok := additionalData["tags"]; ok {
		message.Tags = tags.([]string)
	}
	if version, ok := additionalData["version"]; ok {
		message.Version = version.(string)
	}

	reqBody, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal abort request: %w", err)
	}

	_, err = c.makeRequest("POST", url, bytes.NewReader(reqBody))
	return err
}

// GetTestRunStatus retrieves the status of a test run from the Perfana API.
func (c *PerfanaClient) GetTestRunStatus(testRunID string) (*TestRunResult, error) {
	url := fmt.Sprintf("%s/api/test-runs/%s", c.config.ApiUrl, testRunID)

	resp, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result TestRunResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse test run result: %w", err)
	}

	return &result, nil
}

// GetCheckResults retrieves SLO check results for a completed test run.
func (c *PerfanaClient) GetCheckResults(testRunID, system, environment, workload string) ([]CheckResult, error) {
	url := fmt.Sprintf("%s/api/test-runs/%s/check-results?system=%s&environment=%s&workload=%s",
		c.config.ApiUrl, testRunID, system, environment, workload)

	resp, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var results []CheckResult
	if err := json.Unmarshal(resp, &results); err != nil {
		return nil, fmt.Errorf("failed to parse check results: %w", err)
	}

	return results, nil
}

// GetAdaptConclusion retrieves the enriched adapt conclusion for a completed test run.
// Returns nil, nil when no conclusion exists yet.
func (c *PerfanaClient) GetAdaptConclusion(testRunID string) (*AdaptConclusion, error) {
	url := fmt.Sprintf("%s/api/adapt/conclusion/%s/enriched", c.config.ApiUrl, testRunID)

	resp, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(resp)) == 0 {
		return nil, nil
	}

	var result AdaptConclusion
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse adapt conclusion: %w", err)
	}

	return &result, nil
}

// GetDefaultOrganizationID returns the ID of the first organization available to the API key.
func (c *PerfanaClient) GetDefaultOrganizationID() (string, error) {
	url := fmt.Sprintf("%s/api/organizations", c.config.ApiUrl)
	resp, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	var orgs []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &orgs); err != nil {
		return "", fmt.Errorf("failed to parse organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", nil
	}
	return orgs[0].ID, nil
}

// AppUrl returns the configured UI application URL.
func (c *PerfanaClient) AppUrl() string {
	return c.config.AppUrl
}

// ConfigKeyRequest represents a single key-value config upload.
type ConfigKeyRequest struct {
	TestRunID       string   `json:"testRunId"`
	SystemUnderTest string   `json:"systemUnderTest"`
	TestEnvironment string   `json:"testEnvironment"`
	Workload        string   `json:"workload"`
	Key             string   `json:"key"`
	Value           string   `json:"value"`
	Tags            []string `json:"tags,omitempty"`
}

// ConfigKeysRequest represents a multi key-value config upload.
type ConfigKeysRequest struct {
	TestRunID       string       `json:"testRunId"`
	SystemUnderTest string       `json:"systemUnderTest"`
	TestEnvironment string       `json:"testEnvironment"`
	Workload        string       `json:"workload"`
	ConfigItems     []ConfigItem `json:"configItems"`
	Tags            []string     `json:"tags,omitempty"`
}

// ConfigItem is a single key-value pair for config upload.
type ConfigItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ConfigJSONRequest represents a JSON config upload with regex filters.
type ConfigJSONRequest struct {
	TestRunID       string      `json:"testRunId"`
	SystemUnderTest string      `json:"systemUnderTest"`
	TestEnvironment string      `json:"testEnvironment"`
	Workload        string      `json:"workload"`
	JSON            interface{} `json:"json"`
	Includes        []string    `json:"includes,omitempty"`
	Excludes        []string    `json:"excludes,omitempty"`
	Tags            []string    `json:"tags,omitempty"`
}

// SendConfigKey uploads a single key-value config to Perfana.
func (c *PerfanaClient) SendConfigKey(testRunID, systemUnderTest, testEnvironment, workload, key, value string, tags []string) error {
	url := fmt.Sprintf("%s/api/config/key", c.config.ApiUrl)

	reqBody, err := json.Marshal(ConfigKeyRequest{
		TestRunID:       testRunID,
		SystemUnderTest: systemUnderTest,
		TestEnvironment: testEnvironment,
		Workload:        workload,
		Key:             key,
		Value:           value,
		Tags:            tags,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal config key request: %w", err)
	}

	_, err = c.makeRequest("POST", url, bytes.NewReader(reqBody))
	return err
}

// SendConfigKeys uploads multiple key-value configs to Perfana.
func (c *PerfanaClient) SendConfigKeys(testRunID, systemUnderTest, testEnvironment, workload string, items []ConfigItem, tags []string) error {
	url := fmt.Sprintf("%s/api/config/keys", c.config.ApiUrl)

	reqBody, err := json.Marshal(ConfigKeysRequest{
		TestRunID:       testRunID,
		SystemUnderTest: systemUnderTest,
		TestEnvironment: testEnvironment,
		Workload:        workload,
		ConfigItems:     items,
		Tags:            tags,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal config keys request: %w", err)
	}

	_, err = c.makeRequest("POST", url, bytes.NewReader(reqBody))
	return err
}

// SendConfigJSON uploads JSON config with regex filters to Perfana.
func (c *PerfanaClient) SendConfigJSON(testRunID, systemUnderTest, testEnvironment, workload string, jsonData interface{}, includes, excludes, tags []string) error {
	url := fmt.Sprintf("%s/api/config/json", c.config.ApiUrl)

	reqBody, err := json.Marshal(ConfigJSONRequest{
		TestRunID:       testRunID,
		SystemUnderTest: systemUnderTest,
		TestEnvironment: testEnvironment,
		Workload:        workload,
		JSON:            jsonData,
		Includes:        includes,
		Excludes:        excludes,
		Tags:            tags,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal config json request: %w", err)
	}

	_, err = c.makeRequest("POST", url, bytes.NewReader(reqBody))
	return err
}

// sendPerfanaEvent sends a PerfanaEvent to the /api/events endpoint.
// It returns an error if the request fails or if the response status is non-200,
// along with the server response for non-200 statuses.
func (c *PerfanaClient) SendPerfanaEvent(event PerfanaEvent) (string, error) {
	url := fmt.Sprintf("%s/api/events", c.config.ApiUrl)

	// Marshal the event struct into JSON
	reqBody, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.config.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	// Perform the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Handle non-200 response status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // Read the response body for error details
		return string(body), fmt.Errorf("non-200 response received: %s (%d)", resp.Status, resp.StatusCode)
	}

	// Successful response
	return "Event sent successfully.", nil
}
