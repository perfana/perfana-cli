/*
Copyright © 2024 Peter Paul Bakker <peterpaul@perfana.io>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"perfana-cli/logger"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"perfana-cli/events"
	"perfana-cli/perfana_client"
	"perfana-cli/scheduler"
	"perfana-cli/util"
)

// FullConfig represents the complete perfana.yaml configuration.
type FullConfig struct {
	Perfana   perfana_client.Configuration `yaml:"perfana"`
	Test      TestConfig                   `yaml:"test"`
	Scheduler SchedulerConfig              `yaml:"scheduler"`
	Events    []EventConfig                `yaml:"events"`
}

// TestConfig holds test-level settings from YAML.
type TestConfig struct {
	SystemUnderTest     string                    `yaml:"systemUnderTest"`
	Environment         string                    `yaml:"environment"`
	Workload            string                    `yaml:"workload"`
	Version             string                    `yaml:"version"`
	AnalysisStartOffset string                    `yaml:"analysisStartOffset"`
	AnalysisEndOffset   string                    `yaml:"analysisEndOffset"`
	Duration            string                    `yaml:"duration"`
	ConstantLoadTime    yaml.Node                 `yaml:"constantLoadTime"` // removed; only read to reject it
	Tags                []string                  `yaml:"tags"`
	Annotations         string                    `yaml:"annotations"`
	DeepLinks           []perfana_client.DeepLink `yaml:"deepLinks"`
	Variables           []VariableConfig          `yaml:"variables"`
}

const constantLoadTimeRemoved = "test.constantLoadTime has been removed: set test.duration to the total test length (analysisStartOffset + constantLoadTime)"

// VariableConfig holds a placeholder/value pair from YAML.
type VariableConfig struct {
	Placeholder string `yaml:"placeholder"`
	Value       string `yaml:"value"`
}

// SchedulerConfig holds scheduler settings from YAML.
type SchedulerConfig struct {
	Enabled                  bool   `yaml:"enabled"`
	FailOnError              bool   `yaml:"failOnError"`
	KeepAliveIntervalSeconds int    `yaml:"keepAliveIntervalSeconds"`
	ScheduleScript           string `yaml:"scheduleScript"`
}

// EventConfig is a generic event entry from YAML that supports both types.
type EventConfig struct {
	Name                           string              `yaml:"name"`
	Type                           string              `yaml:"type"`
	ContinueOnKeepAliveParticipant bool                `yaml:"continueOnKeepAliveParticipant"`
	Commands                       events.CommandHooks `yaml:"commands"`
	// Config collector fields
	Command  string   `yaml:"command"`
	Output   string   `yaml:"output"`
	Key      string   `yaml:"key"`
	Includes []string `yaml:"includes"`
	Excludes []string `yaml:"excludes"`
	Tags     []string `yaml:"tags"`
}

// loadConfig reads configPath and then overlayPath (optional) on top of it, with
// env vars expanded. Values set in the overlay win, except that events and
// test.tags are combined, with the overlay's last.
func loadConfig(configPath, overlayPath string) (FullConfig, error) {
	var config FullConfig
	for _, path := range []string{configPath, overlayPath} {
		if path == "" {
			continue
		}
		file, err := os.ReadFile(path)
		if err != nil {
			return config, fmt.Errorf("cannot read %s: %w", path, err)
		}
		events, tags := config.Events, config.Test.Tags
		config.Events, config.Test.Tags = nil, nil
		if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(file))), &config); err != nil {
			return config, fmt.Errorf("YAML parse error in %s: %w", path, err)
		}
		config.Events = append(events, config.Events...)
		config.Test.Tags = append(tags, config.Test.Tags...)
	}
	return config, nil
}

// Define command-line flags with default values
var (
	overlayFile         string
	analysisStartOffset string
	analysisEndOffset   string
	testDuration        string
	tags                string
	annotation          string
	testVersion         string
	buildResultsUrl     string
	variablesFlag       []string
	deepLinksFlag       []string
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a Perfana run",
	Long: `The 'run start' command starts a Perfana test run with full event lifecycle
orchestration. It runs BeforeTest → StartTest → KeepAlive loop → CheckResults → AfterTest.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Load the configuration file
		configPath := cfgFile
		if configPath == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				fmt.Println("Error finding home directory:", err)
				os.Exit(1)
			}
			configPath = filepath.Join(homeDir, ".perfana-cli", "perfana.yaml")
		}

		// Also check for ./perfana.yaml in current directory
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if _, err2 := os.Stat("perfana.yaml"); err2 == nil {
				configPath = "perfana.yaml"
			}
		}

		fullConfig, err := loadConfig(configPath, overlayFile)
		if err != nil {
			fmt.Printf("Error loading configuration: %v\n", err)
			os.Exit(1)
		}
		if !fullConfig.Test.ConstantLoadTime.IsZero() {
			fmt.Printf("Error in configuration file: %s\n", constantLoadTimeRemoved)
			os.Exit(1)
		}

		// Apply test config to perfana client config if not set directly
		config := fullConfig.Perfana
		if config.SystemUnderTest == "" {
			config.SystemUnderTest = fullConfig.Test.SystemUnderTest
		}
		if config.Environment == "" {
			config.Environment = fullConfig.Test.Environment
		}
		if config.Workload == "" {
			config.Workload = fullConfig.Test.Workload
		}

		// CLI flags override YAML values
		effectiveAnalysisStartOffset := fullConfig.Test.AnalysisStartOffset
		if analysisStartOffset != "" {
			effectiveAnalysisStartOffset = analysisStartOffset
		}
		if effectiveAnalysisStartOffset == "" {
			effectiveAnalysisStartOffset = "PT5M"
		}

		effectiveDuration := fullConfig.Test.Duration
		if testDuration != "" {
			effectiveDuration = testDuration
		}
		if effectiveDuration == "" {
			effectiveDuration = "PT20M"
		}

		effectiveVersion := fullConfig.Test.Version
		if testVersion != "" {
			effectiveVersion = testVersion
		}

		// Parse Variables from YAML + CLI flags
		variables := make(map[string]string)
		for _, v := range fullConfig.Test.Variables {
			variables[v.Placeholder] = v.Value
		}
		for _, v := range variablesFlag {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) == 2 {
				variables[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		// Parse durations
		analysisStartOffsetSec, err := util.ParseISODurationToSeconds(effectiveAnalysisStartOffset)
		if err != nil {
			fmt.Printf("Error parsing analysisStartOffset: %v\n", err)
			os.Exit(1)
		}

		effectiveAnalysisEndOffset := fullConfig.Test.AnalysisEndOffset
		if analysisEndOffset != "" {
			effectiveAnalysisEndOffset = analysisEndOffset
		}

		analysisEndOffsetSec := 0
		if effectiveAnalysisEndOffset != "" {
			analysisEndOffsetSec, err = util.ParseISODurationToSeconds(effectiveAnalysisEndOffset)
			if err != nil {
				fmt.Printf("Error parsing analysisEndOffset: %v\n", err)
				os.Exit(1)
			}
		}

		durationSec, err := util.ParseISODurationToSeconds(effectiveDuration)
		if err != nil {
			fmt.Printf("Error parsing duration: %v\n", err)
			os.Exit(1)
		}

		logger.Info("starting test run", "durationSec", durationSec, "analysisStartOffsetSec", analysisStartOffsetSec)

		// Initialize the Perfana client
		client, err := perfana_client.NewClient(config)
		if err != nil {
			fmt.Printf("Error initializing Perfana client: %v\n", err)
			os.Exit(1)
		}

		// Build tag list from YAML + CLI
		tagList := fullConfig.Test.Tags
		if tags != "" {
			for _, t := range strings.Split(tags, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tagList = append(tagList, t)
				}
			}
		}

		// Resolve annotation from CLI flag or YAML
		effectiveAnnotation := fullConfig.Test.Annotations
		if annotation != "" {
			effectiveAnnotation = annotation
		}

		// Resolve buildResultsUrl from CLI flag or YAML
		effectiveBuildResultsUrl := buildResultsUrl

		// Build test context
		testCtx := scheduler.TestContext{
			SystemUnderTest:     config.SystemUnderTest,
			Environment:         config.Environment,
			Workload:            config.Workload,
			Version:             effectiveVersion,
			Tags:                tagList,
			Variables:           variables,
			Annotations:         effectiveAnnotation,
			AnalysisStartOffset: analysisStartOffsetSec,
			AnalysisEndOffset:   analysisEndOffsetSec,
			Duration:            durationSec,
			BuildResultsUrl:     effectiveBuildResultsUrl,
			DeepLinks:           fullConfig.Test.DeepLinks,
			Client:              client,
		}

		// Parse events from YAML config
		var eventList []scheduler.Event
		for _, ec := range fullConfig.Events {
			switch ec.Type {
			case "command":
				eventList = append(eventList, events.NewCommandEvent(events.CommandEventConfig{
					Name:                           ec.Name,
					Type:                           ec.Type,
					ContinueOnKeepAliveParticipant: ec.ContinueOnKeepAliveParticipant,
					Commands:                       ec.Commands,
				}))
				logger.Debug("registered event", "type", "command", "name", ec.Name)

			case "config-collector":
				eventList = append(eventList, events.NewConfigCollectorEvent(events.ConfigCollectorConfig{
					Name:     ec.Name,
					Type:     ec.Type,
					Command:  ec.Command,
					Output:   ec.Output,
					Key:      ec.Key,
					Includes: ec.Includes,
					Excludes: ec.Excludes,
					Tags:     ec.Tags,
				}))
				logger.Debug("registered event", "type", "config-collector", "name", ec.Name)

			default:
				logger.Warn("unknown event type, skipping", "type", ec.Type, "name", ec.Name)
			}
		}

		// Parse schedule script
		var scheduleEntries []scheduler.ScheduleEntry
		if fullConfig.Scheduler.ScheduleScript != "" {
			scheduleEntries, err = scheduler.ParseScheduleScript(fullConfig.Scheduler.ScheduleScript)
			if err != nil {
				fmt.Printf("Error parsing schedule script: %v\n", err)
				os.Exit(1)
			}
			logger.Info("schedule parsed", "entries", len(scheduleEntries))
		}

		keepAliveInterval := fullConfig.Scheduler.KeepAliveIntervalSeconds
		if keepAliveInterval <= 0 {
			keepAliveInterval = 30
		}

		// Create the event scheduler
		eventScheduler := &scheduler.EventScheduler{
			Client:               client,
			Events:               eventList,
			ScheduleEntries:      scheduleEntries,
			KeepAliveIntervalSec: keepAliveInterval,
			TestDurationSec:      durationSec,
			TestContext:          testCtx,
			FailOnError:          fullConfig.Scheduler.FailOnError,
		}

		logger.Info("scheduler configured", "events", len(eventList), "scheduleEntries", len(scheduleEntries), "keepAliveIntervalSec", keepAliveInterval)

		// Run the full lifecycle
		if err := eventScheduler.Run(); err != nil {
			fmt.Printf("Test run failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	runCmd.AddCommand(startCmd)

	startCmd.Flags().StringVar(&overlayFile, "overlay", "", "Config file applied on top of --config. Its values win; events and test.tags are combined, with its events running last.")
	startCmd.Flags().StringVar(&analysisStartOffset, "analysisStartOffset", "", "Offset before analysis starts (typically the ramp-up window) in ISO8601 format (e.g., PT5M). Overrides YAML.")
	startCmd.Flags().StringVar(&analysisEndOffset, "analysisEndOffset", "", "Trailing period excluded from analysis (typically the ramp-down window) in ISO8601 format (e.g., PT1M). Overrides YAML.")
	startCmd.Flags().StringVar(&testDuration, "duration", "", "Total test duration in ISO8601 format, including any analysis offsets (e.g., PT20M). Overrides YAML.")
	startCmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags to add to the test session (merged with YAML tags)")
	startCmd.Flags().StringVar(&annotation, "annotation", "", "Annotation message for the test session")
	startCmd.Flags().StringVar(&testVersion, "version", "", "Version of the test session. Overrides YAML.")
	startCmd.Flags().StringVar(&buildResultsUrl, "buildResultsUrl", "", "URL to CI build results")
	startCmd.Flags().StringSliceVar(&variablesFlag, "variable", []string{}, "Set variables (name=value)")
	startCmd.Flags().StringSliceVar(&deepLinksFlag, "deeplink", []string{}, "Add deep links (title|url)")
}
