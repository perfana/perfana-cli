package cmd

import (
	"encoding/xml"
	"fmt"
	"log"
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
	GroupID       string              `xml:"groupId"`
	ArtifactID    string              `xml:"artifactId"`
	Configuration MavenConfiguration  `xml:"configuration"`
}

type MavenConfiguration struct {
	EventSchedulerConfig MavenEventSchedulerConfig `xml:"eventSchedulerConfig"`
}

type MavenEventSchedulerConfig struct {
	DebugEnabled              string             `xml:"debugEnabled"`
	SchedulerEnabled          string             `xml:"schedulerEnabled"`
	FailOnError               string             `xml:"failOnError"`
	KeepAliveIntervalSeconds  string             `xml:"keepAliveIntervalSeconds"`
	TestConfig                MavenTestConfig    `xml:"testConfig"`
	PerfanaConfig             MavenPerfanaConfig `xml:"perfanaConfig"`
	ScheduleScript            string             `xml:"scheduleScript"`
	EventConfigs              MavenEventConfigs  `xml:"eventConfigs"`
}

type MavenTestConfig struct {
	SystemUnderTest          string `xml:"systemUnderTest"`
	Version                  string `xml:"version"`
	Workload                 string `xml:"workload"`
	TestEnvironment          string `xml:"testEnvironment"`
	TestRunID                string `xml:"testRunId"`
	BuildResultsUrl          string `xml:"buildResultsUrl"`
	RampupTimeInSeconds      string `xml:"rampupTimeInSeconds"`
	ConstantLoadTimeInSeconds string `xml:"constantLoadTimeInSeconds"`
	Annotations              string `xml:"annotations"`
	Tags                     string `xml:"tags"`
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
	ActuatorBaseUrl      string `xml:"actuatorBaseUrl"`
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
	SystemUnderTest  string   `yaml:"systemUnderTest"`
	Environment      string   `yaml:"environment"`
	Workload         string   `yaml:"workload"`
	Version          string   `yaml:"version,omitempty"`
	RampupTime       string   `yaml:"rampupTime"`
	ConstantLoadTime string   `yaml:"constantLoadTime"`
	Tags             []string `yaml:"tags,omitempty"`
	Annotations      string   `yaml:"annotations,omitempty"`
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
		log.Printf("Found %d pom.xml files", len(pomPaths))
	} else {
		pomPaths = []string{inputPath}
	}

	// Process the first pom.xml that has eventSchedulerConfig
	for _, pomPath := range pomPaths {
		migrated, warnings, err := migratePom(pomPath)
		if err != nil {
			log.Printf("Skipping %s: %v", pomPath, err)
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

		log.Printf("Migration complete: %s → %s", pomPath, outputPath)
		if len(warnings) > 0 {
			log.Printf("Warnings (%d):", len(warnings))
			for _, w := range warnings {
				log.Printf("  - %s", w)
			}
		}
		return nil
	}

	return fmt.Errorf("no valid eventSchedulerConfig found in any pom.xml")
}

func migratePom(pomPath string) (*MigratedConfig, []string, error) {
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", pomPath, err)
	}

	var project MavenProject
	if err := xml.Unmarshal(data, &project); err != nil {
		return nil, nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	// Build property map for resolving ${property} references
	props := make(map[string]string)
	for _, p := range project.Properties.Values {
		props[p.XMLName.Local] = p.Value
	}

	// Find the event-scheduler-maven-plugin
	var esc *MavenEventSchedulerConfig
	for _, plugin := range project.Build.Plugins.Plugin {
		if plugin.ArtifactID == "event-scheduler-maven-plugin" {
			esc = &plugin.Configuration.EventSchedulerConfig
			break
		}
	}

	if esc == nil {
		return nil, nil, fmt.Errorf("no event-scheduler-maven-plugin found")
	}

	var warnings []string

	// Resolve Maven property references
	resolve := func(val string) string {
		resolved, w := resolveMavenProperty(val, props)
		warnings = append(warnings, w...)
		return resolved
	}

	// Convert test config
	rampupSec := parseIntOrZero(resolve(esc.TestConfig.RampupTimeInSeconds))
	constantSec := parseIntOrZero(resolve(esc.TestConfig.ConstantLoadTimeInSeconds))
	keepAliveSec := parseIntOrZero(resolve(esc.KeepAliveIntervalSeconds))

	tagStr := resolve(esc.TestConfig.Tags)
	var tagList []string
	if tagStr != "" {
		for _, t := range strings.Split(tagStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagList = append(tagList, t)
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
			SystemUnderTest:  resolve(esc.TestConfig.SystemUnderTest),
			Environment:      resolve(esc.TestConfig.TestEnvironment),
			Workload:         resolve(esc.TestConfig.Workload),
			Version:          resolve(esc.TestConfig.Version),
			RampupTime:       secondsToISO8601(rampupSec),
			ConstantLoadTime: secondsToISO8601(constantSec),
			Tags:             tagList,
			Annotations:      resolve(esc.TestConfig.Annotations),
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
				Name: name,
				Type: "command",
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
			log.Printf("Skipping PerfanaEventConfig %q (handled natively)", name)

		case strings.Contains(impl, "SpringBootEventConfig"):
			warnings = append(warnings, fmt.Sprintf("SpringBootEventConfig %q needs manual migration to command type (actuator URL: %s)", name, ec.ActuatorBaseUrl))

		case strings.Contains(impl, "WireMockEventConfig"):
			warnings = append(warnings, fmt.Sprintf("WireMockEventConfig %q needs manual migration to command type (wiremock URL: %s)", name, ec.WiremockUrl))

		default:
			warnings = append(warnings, fmt.Sprintf("Unknown event implementation %q for event %q — needs manual migration", impl, name))
		}
	}

	return migrated, warnings, nil
}

// resolveMavenProperty resolves ${property} references against the property map.
// Returns the resolved value and any warnings for unresolvable references.
func resolveMavenProperty(val string, props map[string]string) (string, []string) {
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

		// Check if it's an ENV reference
		if strings.HasPrefix(propName, "ENV.") || strings.HasPrefix(propName, "env.") {
			envVar := propName[4:]
			result = result[:start] + "${" + envVar + "}" + result[end+1:]
			continue
		}

		if resolved, ok := props[propName]; ok {
			result = result[:start] + resolved + result[end+1:]
		} else {
			warnings = append(warnings, fmt.Sprintf("Maven property ${%s} could not be resolved — set manually", propName))
			break
		}
	}

	return result, warnings
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
		if line != "" {
			cleaned = append(cleaned, line)
		}
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
