package perfana_client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PerfanaEvent represents the structure of the JSON payload for the /api/events endpoint
type PerfanaEvent struct {
	SystemUnderTest string   `json:"systemUnderTest"`
	TestEnvironment string   `json:"testEnvironment"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags,omitempty"`
}

// PerfanaMessage represents the JSON payload sent to start a session
type PerfanaMessage struct {
	TestRunID         string     `json:"testRunId"`
	Workload          string     `json:"workload"`
	TestEnvironment   string     `json:"testEnvironment"`
	SystemUnderTest   string     `json:"systemUnderTest"`
	Version           string     `json:"version,omitempty"`           // Optional
	CIBuildResultsURL string     `json:"CIBuildResultsUrl,omitempty"` // Optional
	RampUp            string     `json:"rampUp,omitempty"`            // Optional (e.g., "PT5M" for a 5-minute ramp-up)
	Duration          string     `json:"duration,omitempty"`          // Optional (e.g., "PT30M" for 30 minutes)
	Completed         bool       `json:"completed"`
	Annotations       string     `json:"annotations,omitempty"` // Optional
	Tags              []string   `json:"tags,omitempty"`        // Optional
	Variables         []Variable `json:"variables,omitempty"`   // Optional
	DeepLinks         []DeepLink `json:"deepLinks,omitempty"`   // Optional
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

// Config represents the configuration for Perfana API calls
type Config struct {
	Key              string `yaml:"key"`
	ClientIdentifier string `yaml:"clientIdentifier"`
	SystemUnderTest  string `yaml:"systemUnderTest"`
	Environment      string `yaml:"environment"`
	Workload         string `yaml:"workload"`
	BaseURL          string `yaml:"baseUrl"`
	MTLSCert         string `yaml:"-"` // Optional: mTLS certificate
	MTLSKey          string `yaml:"-"` // Optional: mTLS private key
}

// Client represents the Perfana Client that makes API calls
type Client struct {
	config Config
	client *http.Client
}

// NewClient initializes a new Perfana client with the given configuration
func NewClient(config Config) (*Client, error) {
	if config.BaseURL == "" {
		return nil, errors.New("baseUrl is required")
	}

	// Initialize an HTTP client (can include mTLS support if necessary)
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &Client{
		config: config,
		client: httpClient,
	}, nil
}

// Init performs a POST request to /api/init and starts a test run.
// It sends systemUnderTest, environment, and workload in the JSON payload
// and receives a testRunId in the response.
func (c *Client) Init() (string, error) {
	url := fmt.Sprintf("%s/api/init", c.config.BaseURL)

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
func (c *Client) TestEvent(testRunID string, additionalData map[string]interface{}, completed bool) error {
	url := fmt.Sprintf("%s/api/test", c.config.BaseURL)

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
	if rampUp, ok := additionalData["rampUp"]; ok {
		message.RampUp = rampUp.(string)
	}
	if duration, ok := additionalData["duration"]; ok {
		message.Duration = duration.(string)
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

	// Marshal the message to JSON
	reqBody, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	fmt.Printf("TestEvent request: %s\n", string(reqBody))

	// Make the HTTP request
	resp, err := c.makeRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	// Typically, Perfana doesn't return extra data for this operation,
	// but you can log or check the server's response body if needed.
	fmt.Printf("TestEvent response: %s\n", string(resp))

	return nil
}

// Shared helper method for HTTP requests
func (c *Client) makeRequest(method, url string, body io.Reader) ([]byte, error) {

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
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

// sendPerfanaEvent sends a PerfanaEvent to the /api/events endpoint.
// It returns an error if the request fails or if the response status is non-200,
// along with the server response for non-200 statuses.
func (c *Client) SendPerfanaEvent(event PerfanaEvent) (string, error) {
	url := fmt.Sprintf("%s/api/events", c.config.BaseURL)

	// Marshal the event struct into JSON
	reqBody, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.config.Key)
	req.Header.Set("Content-Type", "application/json")

	// Perform the request
	resp, err := c.client.Do(req)
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
