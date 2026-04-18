package cmd

import (
	"encoding/xml"
	"fmt"
	"perfana-cli/logger"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Maven XML structures for parsing eventSchedulerConfig

type MavenProject struct {
	XMLName    xml.Name        `xml:"project"`
	Properties MavenProperties `xml:"properties"`
	Build      MavenBuild      `xml:"build"`
	Profiles   MavenProfiles   `xml:"profiles"`
}

type MavenProfiles struct {
	Profile []MavenProfile `xml:"profile"`
}

type MavenProfile struct {
	ID         string          `xml:"id"`
	Properties MavenProperties `xml:"properties"`
}

type MavenProperties struct {
	Values []MavenProperty `xml:",any"`
}

type MavenProperty struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type MavenBuild struct {
	Plugins MavenPlugins `xml:"plugins"`
}

type MavenPlugins struct {
	Plugin []MavenPlugin `xml:"plugin"`
}

type MavenPlugin struct {
	GroupID       string             `xml:"groupId"`
	ArtifactID    string             `xml:"artifactId"`
	Configuration MavenConfiguration `xml:"configuration"`
}

type MavenConfiguration struct {
	EventSchedulerConfig MavenEventSchedulerConfig `xml:"eventSchedulerConfig"`
}

type MavenEventSchedulerConfig struct {
	DebugEnabled             string             `xml:"debugEnabled"`
	SchedulerEnabled         string             `xml:"schedulerEnabled"`
	FailOnError              string             `xml:"failOnError"`
	KeepAliveIntervalSeconds string             `xml:"keepAliveIntervalSeconds"`
	TestConfig               MavenTestConfig    `xml:"testConfig"`
	PerfanaConfig            MavenPerfanaConfig `xml:"perfanaConfig"`
	ScheduleScript           string             `xml:"scheduleScript"`
	EventConfigs             MavenEventConfigs  `xml:"eventConfigs"`
}

type MavenTestConfig struct {
	SystemUnderTest           string `xml:"systemUnderTest"`
	Version                   string `xml:"version"`
	Workload                  string `xml:"workload"`
	TestEnvironment           string `xml:"testEnvironment"`
	TestRunID                 string `xml:"testRunId"`
	BuildResultsUrl           string `xml:"buildResultsUrl"`
	RampupTimeInSeconds       string `xml:"rampupTimeInSeconds"`
	ConstantLoadTimeInSeconds string `xml:"constantLoadTimeInSeconds"`
	Annotations               string `xml:"annotations"`
	Tags                      string `xml:"tags"`
}

type MavenPerfanaConfig struct {
	PerfanaUrl           string `xml:"perfanaUrl"`
	ApiKey               string `xml:"apiKey"`
	AssertResultsEnabled string `xml:"assertResultsEnabled"`
}

type MavenEventConfigs struct {
	EventConfig []MavenEventConfig `xml:"eventConfig"`
}

type MavenEventConfig struct {
	Implementation                 string `xml:"implementation,attr"`
	Name                           string `xml:"name"`
	Enabled                        string `xml:"enabled"`
	ContinueOnKeepAliveParticipant string `xml:"continueOnKeepAliveParticipant"`
	// Command runner fields
	OnBeforeTest string `xml:"onBeforeTest"`
	OnStartTest  string `xml:"onStartTest"`
	OnKeepAlive  string `xml:"onKeepAlive"`
	OnAbort      string `xml:"onAbort"`
	OnAfterTest  string `xml:"onAfterTest"`
	// Config collector fields
	Command  string `xml:"command"`
	Output   string `xml:"output"`
	Key      string `xml:"key"`
	Tags     string `xml:"tags"`
	Includes string `xml:"includes"`
	Excludes string `xml:"excludes"`
	// Perfana fields (skipped in migration)
	PerfanaUrl           string `xml:"perfanaUrl"`
	ApiKey               string `xml:"apiKey"`
	AssertResultsEnabled string `xml:"assertResultsEnabled"`
	// SpringBoot fields
	ActuatorBaseUrl       string `xml:"actuatorBaseUrl"`
	ActuatorEnvProperties string `xml:"actuatorEnvProperties"`
	// WireMock fields
	WiremockUrl string `xml:"wiremockUrl"`
}

// MigratedConfig is the output YAML structure
type MigratedConfig struct {
	Perfana   MigratedPerfana   `yaml:"perfana"`
	Test      MigratedTest      `yaml:"test"`
	Scheduler MigratedScheduler `yaml:"scheduler"`
	Events    []MigratedEvent   `yaml:"events,omitempty"`
}

type MigratedPerfana struct {
	ApiKey  string `yaml:"apiKey"`
	BaseUrl string `yaml:"baseUrl"`
}

type MigratedTest struct {
	SystemUnderTest     string   `yaml:"systemUnderTest"`
	Environment         string   `yaml:"environment"`
	Workload            string   `yaml:"workload"`
	Version             string   `yaml:"version,omitempty"`
	AnalysisStartOffset string   `yaml:"analysisStartOffset"`
	ConstantLoadTime    string   `yaml:"constantLoadTime"`
	Tags                []string `yaml:"tags,omitempty"`
	Annotations         string   `yaml:"annotations,omitempty"`
}

type MigratedScheduler struct {
	Enabled                  bool   `yaml:"enabled"`
	FailOnError              bool   `yaml:"failOnError"`
	KeepAliveIntervalSeconds int    `yaml:"keepAliveIntervalSeconds,omitempty"`
	ScheduleScript           string `yaml:"scheduleScript,omitempty"`
}

type MigratedEvent struct {
	Name                           string            `yaml:"name"`
	Type                           string            `yaml:"type"`
	ContinueOnKeepAliveParticipant bool              `yaml:"continueOnKeepAliveParticipant,omitempty"`
	Commands                       *MigratedCommands `yaml:"commands,omitempty"`
	Command                        string            `yaml:"command,omitempty"`
	Output                         string            `yaml:"output,omitempty"`
	Key                            string            `yaml:"key,omitempty"`
	Includes                       []string          `yaml:"includes,omitempty"`
	Excludes                       []string          `yaml:"excludes,omitempty"`
	Tags                           []string          `yaml:"tags,omitempty"`
}

type MigratedCommands struct {
	OnBeforeTest string `yaml:"onBeforeTest,omitempty"`
	OnStartTest  string `yaml:"onStartTest,omitempty"`
	OnKeepAlive  string `yaml:"onKeepAlive,omitempty"`
	OnAbort      string `yaml:"onAbort,omitempty"`
	OnAfterTest  string `yaml:"onAfterTest,omitempty"`
}

var (
	migrateInput  string
	migrateOutput string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Convert Maven pom.xml to perfana.yaml",
	Long: `Converts a Maven pom.xml with event-scheduler-maven-plugin configuration
to a perfana.yaml configuration file.

Handles:
- eventSchedulerConfig test settings
- CommandRunnerEventConfig → command events
- TestRunConfigCommandEventConfig → config-collector events
- Duration conversion (seconds → ISO 8601 PT format)
- Maven property references are flagged with TODO comments`,
	Run: func(cmd *cobra.Command, args []string) {
		if migrateInput == "" {
			migrateInput = "pom.xml"
		}
		if migrateOutput == "" {
			migrateOutput = "perfana.yaml"
		}

		if err := runMigrate(migrateInput, migrateOutput); err != nil {
			fmt.Printf("Migration failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.Flags().StringVar(&migrateInput, "input", "pom.xml", "Path to Maven pom.xml")
	migrateCmd.Flags().StringVar(&migrateOutput, "output", "perfana.yaml", "Output path for generated YAML")
}

func runMigrate(inputPath, outputPath string) error {
	// Check for multi-module: if input is a directory, scan recursively
	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", inputPath, err)
	}

	var pomPaths []string
	if info.IsDir() {
		filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.Name() == "pom.xml" {
				pomPaths = append(pomPaths, path)
			}
			return nil
		})
		if len(pomPaths) == 0 {
			return fmt.Errorf("no pom.xml found in %s", inputPath)
		}
		logger.Info("found pom.xml files", "count", len(pomPaths))
	} else {
		pomPaths = []string{inputPath}
	}

	// Process the first pom.xml that has eventSchedulerConfig
	for _, pomPath := range pomPaths {
		migrated, warnings, profiles, err := migratePom(pomPath)
		if err != nil {
			logger.Warn("skipping pom", "path", pomPath, "err", err)
			continue
		}

		// Write YAML output
		yamlBytes, err := yaml.Marshal(migrated)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}

		// Add header comments
		header := "# Generated by perfana-cli migrate\n# Source: " + pomPath + "\n#\n"
		for _, w := range warnings {
			header += "# TODO: " + w + "\n"
		}
		header += "\n"

		if err := os.WriteFile(outputPath, []byte(header+string(yamlBytes)), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outputPath, err)
		}

		logger.Info("migration complete", "source", pomPath, "output", outputPath)

		// Write profile .env files
		if len(profiles) > 0 {
			outputDir := filepath.Dir(outputPath)
			profileDir := filepath.Join(outputDir, "profiles")
			if err := os.MkdirAll(profileDir, 0755); err != nil {
				logger.Warn("could not create profiles directory", "err", err)
			} else {
				for profileID, envVars := range profiles {
					envPath := filepath.Join(profileDir, profileID+".env")
					if err := os.WriteFile(envPath, []byte(envVars), 0644); err != nil {
						logger.Warn("could not write profile", "path", envPath, "err", err)
					} else {
						logger.Info("wrote profile", "path", envPath)
					}
				}
			}
		}

		for _, w := range warnings {
			logger.Warn("migration warning", "msg", w)
		}
		return nil
	}

	return fmt.Errorf("no valid eventSchedulerConfig found in any pom.xml")
}

func migratePom(pomPath string) (*MigratedConfig, []string, map[string]string, error) {
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read %s: %w", pomPath, err)
	}

	var project MavenProject
	if err := xml.Unmarshal(data, &project); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	// Build property map for resolving ${property} references
	props := make(map[string]string)
	for _, p := range project.Properties.Values {
		props[p.XMLName.Local] = p.Value
	}

	// Detect properties overridden by profiles — these become env vars
	profileOverrides := make(map[string]bool)
	profileProps := make(map[string]map[string]string) // profileID -> propName -> value
	for _, profile := range project.Profiles.Profile {
		pProps := make(map[string]string)
		for _, p := range profile.Properties.Values {
			profileOverrides[p.XMLName.Local] = true
			pProps[p.XMLName.Local] = p.Value
		}
		if len(pProps) > 0 {
			profileProps[profile.ID] = pProps
		}
	}

	if len(profileOverrides) > 0 {
		var names []string
		for name := range profileOverrides {
			names = append(names, name)
		}
		logger.Info("profile-overridden properties will use env vars", "names", strings.Join(names, ", "))
	}

	// Find any plugin that contains an eventSchedulerConfig
	// This matches event-scheduler-maven-plugin, events-jmeter-maven-plugin,
	// events-gatling-maven-plugin, and other Perfana event plugins.
	var esc *MavenEventSchedulerConfig
	for _, plugin := range project.Build.Plugins.Plugin {
		if plugin.Configuration.EventSchedulerConfig.TestConfig.SystemUnderTest != "" ||
			len(plugin.Configuration.EventSchedulerConfig.EventConfigs.EventConfig) > 0 {
			esc = &plugin.Configuration.EventSchedulerConfig
			break
		}
	}

	if esc == nil {
		return nil, nil, nil, fmt.Errorf("no eventSchedulerConfig found in any plugin")
	}

	var warnings []string

	// Resolve Maven property references.
	// Properties overridden by profiles are converted to env var references
	// instead of being resolved to their default values.
	resolve := func(val string) string {
		resolved, w := resolveMavenProperty(val, props, profileOverrides)
		warnings = append(warnings, w...)
		return resolved
	}

	// Convert test config — durations may be env var references from profiles
	rampupResolved := resolve(esc.TestConfig.RampupTimeInSeconds)
	constantResolved := resolve(esc.TestConfig.ConstantLoadTimeInSeconds)
	keepAliveResolved := resolve(esc.KeepAliveIntervalSeconds)

	// Convert seconds to ISO 8601, but keep env var references as-is
	analysisStartOffset := convertDuration(rampupResolved)
	constantLoadTime := convertDuration(constantResolved)
	keepAliveSec := parseIntOrZero(keepAliveResolved)

	tagStr := resolve(esc.TestConfig.Tags)
	var tagList []string
	if tagStr != "" {
		// If tags resolved to an env var reference, keep as single entry
		if strings.Contains(tagStr, "${") {
			tagList = []string{tagStr}
		} else {
			for _, t := range strings.Split(tagStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tagList = append(tagList, t)
				}
			}
		}
	}

	// Try to find perfanaUrl from perfanaConfig or from PerfanaEventConfig
	baseUrl := resolve(esc.PerfanaConfig.PerfanaUrl)
	apiKey := resolve(esc.PerfanaConfig.ApiKey)
	if baseUrl == "" || apiKey == "" {
		for _, ec := range esc.EventConfigs.EventConfig {
			if strings.Contains(ec.Implementation, "PerfanaEventConfig") {
				if baseUrl == "" && ec.PerfanaUrl != "" {
					baseUrl = resolve(ec.PerfanaUrl)
				}
				if apiKey == "" && ec.ApiKey != "" {
					apiKey = resolve(ec.ApiKey)
				}
			}
		}
	}

	migrated := &MigratedConfig{
		Perfana: MigratedPerfana{
			ApiKey:  convertApiKey(apiKey),
			BaseUrl: baseUrl,
		},
		Test: MigratedTest{
			SystemUnderTest:     resolve(esc.TestConfig.SystemUnderTest),
			Environment:         resolve(esc.TestConfig.TestEnvironment),
			Workload:            resolve(esc.TestConfig.Workload),
			Version:             resolve(esc.TestConfig.Version),
			AnalysisStartOffset: analysisStartOffset,
			ConstantLoadTime:    constantLoadTime,
			Tags:                tagList,
			Annotations:         resolve(esc.TestConfig.Annotations),
		},
		Scheduler: MigratedScheduler{
			Enabled:                  resolveBool(resolve(esc.SchedulerEnabled), true),
			FailOnError:              resolveBool(resolve(esc.FailOnError), false),
			KeepAliveIntervalSeconds: keepAliveSec,
			ScheduleScript:           strings.TrimSpace(esc.ScheduleScript),
		},
	}

	// Convert events
	for _, ec := range esc.EventConfigs.EventConfig {
		impl := ec.Implementation
		name := resolve(ec.Name)

		switch {
		case strings.Contains(impl, "CommandRunnerEventConfig"):
			event := MigratedEvent{
				Name:                           name,
				Type:                           "command",
				ContinueOnKeepAliveParticipant: resolveBool(ec.ContinueOnKeepAliveParticipant, false),
				Commands: &MigratedCommands{
					OnBeforeTest: cleanCommand(resolve(ec.OnBeforeTest)),
					OnStartTest:  cleanCommand(resolve(ec.OnStartTest)),
					OnKeepAlive:  cleanCommand(resolve(ec.OnKeepAlive)),
					OnAbort:      cleanCommand(resolve(ec.OnAbort)),
					OnAfterTest:  cleanCommand(resolve(ec.OnAfterTest)),
				},
			}
			migrated.Events = append(migrated.Events, event)

		case strings.Contains(impl, "TestRunConfigCommand"):
			var includes, excludes []string
			if ec.Includes != "" {
				includes = splitTrimmed(resolve(ec.Includes), ",")
			}
			if ec.Excludes != "" {
				excludes = splitTrimmed(resolve(ec.Excludes), ",")
			}
			var eventTags []string
			if ec.Tags != "" {
				eventTags = splitTrimmed(resolve(ec.Tags), ",")
			}
			event := MigratedEvent{
				Name:     name,
				Type:     "config-collector",
				Command:  cleanCommand(resolve(ec.Command)),
				Output:   resolve(ec.Output),
				Key:      resolve(ec.Key),
				Includes: includes,
				Excludes: excludes,
				Tags:     eventTags,
			}
			migrated.Events = append(migrated.Events, event)

		case strings.Contains(impl, "PerfanaEventConfig"):
			// Skip — handled natively by perfana-cli
			logger.Debug("skipping PerfanaEventConfig, handled natively", "name", name)

		case strings.Contains(impl, "SpringBootEventConfig"):
			warnings = append(warnings, fmt.Sprintf("SpringBootEventConfig %q needs manual migration to command type (actuator URL: %s)", name, ec.ActuatorBaseUrl))

		case strings.Contains(impl, "WireMockEventConfig"):
			warnings = append(warnings, fmt.Sprintf("WireMockEventConfig %q needs manual migration to command type (wiremock URL: %s)", name, ec.WiremockUrl))

		default:
			warnings = append(warnings, fmt.Sprintf("Unknown event implementation %q for event %q — needs manual migration", impl, name))
		}
	}

	// Properties that represent durations in seconds — convert to ISO 8601 in .env files
	durationProps := map[string]bool{
		"rampupTimeInSeconds":           true,
		"constantLoadTimeInSeconds":     true,
		"durationInSeconds":             true,
		"excludeRampUpTimeFromAnalysis": true,
		"jmeterRampup":                  true,
		"jmeterHold":                    true,
		"k6Rampup":                      true,
		"k6Hold":                        true,
		"k6Rampdown":                    true,
	}

	// Generate profile .env files
	envFiles := make(map[string]string)
	for profileID, pProps := range profileProps {
		var lines []string
		lines = append(lines, "# Profile: "+profileID)
		lines = append(lines, "# Source: perfana-cli migrate from Maven profile")
		lines = append(lines, "#")
		lines = append(lines, "# Usage: source profiles/"+profileID+".env && perfana-cli run start")
		lines = append(lines, "")
		for propName, propValue := range pProps {
			envVar := camelToUpperSnake(propName)
			value := propValue
			// Convert duration seconds to ISO 8601
			if durationProps[propName] {
				sec := parseIntOrZero(propValue)
				if sec > 0 {
					value = secondsToISO8601(sec)
				}
			}
			// Flag values with unresolved Maven cross-references
			if strings.Contains(value, "${") {
				lines = append(lines, "# TODO: contains Maven references — set manually")
			}
			lines = append(lines, envVar+"="+value)
		}
		lines = append(lines, "")
		envFiles[profileID] = strings.Join(lines, "\n")
	}

	return migrated, warnings, envFiles, nil
}

// resolveMavenProperty resolves ${property} references against the property map.
// Properties in profileOverrides are converted to env var references (${UPPER_SNAKE})
// instead of being resolved to their default values.
// Returns the resolved value and any warnings for unresolvable references.
func resolveMavenProperty(val string, props map[string]string, profileOverrides map[string]bool) (string, []string) {
	var warnings []string
	result := val

	for strings.Contains(result, "${") {
		start := strings.Index(result, "${")
		end := strings.Index(result[start:], "}")
		if end < 0 {
			break
		}
		end += start

		propName := result[start+2 : end]

		// Check if it's an ENV reference (Maven ${env.VAR} → shell ${VAR})
		if strings.HasPrefix(propName, "ENV.") || strings.HasPrefix(propName, "env.") {
			envVar := propName[4:]
			result = result[:start] + "${" + envVar + "}" + result[end+1:]
			continue
		}

		// Profile-overridden properties → env var reference
		if profileOverrides[propName] {
			envVar := camelToUpperSnake(propName)
			placeholder := "\x00ENVVAR:" + envVar + "\x00"
			result = result[:start] + placeholder + result[end+1:]
			continue
		}

		if resolved, ok := props[propName]; ok {
			result = result[:start] + resolved + result[end+1:]
		} else {
			// Skip warning for likely shell variables (ALL_UPPER or single-word upper)
			if !isLikelyShellVar(propName) {
				warnings = append(warnings, fmt.Sprintf("Maven property ${%s} could not be resolved — set manually", propName))
			}
			// Replace with a placeholder to avoid re-processing, then restore after the loop
			placeholder := "\x00UNRESOLVED:" + propName + "\x00"
			result = result[:start] + placeholder + result[end+1:]
		}
	}

	// Restore placeholders back to ${...} syntax
	for strings.Contains(result, "\x00UNRESOLVED:") {
		start := strings.Index(result, "\x00UNRESOLVED:")
		end := strings.Index(result[start+1:], "\x00") + start + 1
		propName := result[start+len("\x00UNRESOLVED:") : end]
		result = result[:start] + "${" + propName + "}" + result[end+1:]
	}
	for strings.Contains(result, "\x00ENVVAR:") {
		start := strings.Index(result, "\x00ENVVAR:")
		end := strings.Index(result[start+1:], "\x00") + start + 1
		envVar := result[start+len("\x00ENVVAR:") : end]
		result = result[:start] + "${" + envVar + "}" + result[end+1:]
	}

	return result, warnings
}

// isLikelyShellVar returns true if the name looks like a shell variable
// (all uppercase letters, digits, and underscores). These appear in command
// strings and should not generate Maven property warnings.
func isLikelyShellVar(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// camelToUpperSnake converts camelCase to UPPER_SNAKE_CASE.
// e.g. "jmeterReplicas" → "JMETER_REPLICAS", "k6Hold" → "K6_HOLD"
func camelToUpperSnake(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Don't insert underscore between consecutive uppercase (e.g. "k6" stays "K6")
			prev := rune(s[i-1])
			if prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9' {
				result = append(result, '_')
			}
		}
		result = append(result, r)
	}
	return strings.ToUpper(string(result))
}

// convertDuration converts a resolved value to ISO 8601 duration.
// If the value contains an env var reference, returns it as-is.
func convertDuration(val string) string {
	val = strings.TrimSpace(val)
	if strings.Contains(val, "${") {
		return val
	}
	return secondsToISO8601(parseIntOrZero(val))
}

func convertApiKey(key string) string {
	if strings.Contains(key, "${") {
		return key // Already an env var reference
	}
	if key == "" {
		return "${PERFANA_API_KEY}"
	}
	return key
}

func secondsToISO8601(seconds int) string {
	if seconds <= 0 {
		return "PT0S"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60

	result := "PT"
	if h > 0 {
		result += fmt.Sprintf("%dH", h)
	}
	if m > 0 {
		result += fmt.Sprintf("%dM", m)
	}
	if s > 0 || result == "PT" {
		result += fmt.Sprintf("%dS", s)
	}
	return result
}

func parseIntOrZero(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func resolveBool(s string, defaultVal bool) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "true":
		return true
	case "false":
		return false
	default:
		return defaultVal
	}
}

func cleanCommand(cmd string) string {
	// Collapse internal whitespace (multi-line XML values)
	lines := strings.Split(cmd, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Strip trailing backslash — we re-add continuation on join
		line = strings.TrimRight(line, " ")
		line = strings.TrimSuffix(line, "\\")
		line = strings.TrimRight(line, " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	if len(cleaned) == 1 {
		return cleaned[0]
	}
	return strings.Join(cleaned, " \\\n    ")
}

func splitTrimmed(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
