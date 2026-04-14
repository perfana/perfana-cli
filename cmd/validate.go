package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"perfana-cli/util"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate perfana.yaml configuration",
	Long: `Validates a perfana.yaml configuration file without running any tests.
Checks all required fields, duration formats, event type schemas, and
reports clear error messages.`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath := cfgFile
		if configPath == "" {
			if _, err := os.Stat("perfana.yaml"); err == nil {
				configPath = "perfana.yaml"
			} else {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					fmt.Println("Error finding home directory:", err)
					os.Exit(1)
				}
				configPath = homeDir + "/.perfana-cli/perfana.yaml"
			}
		}

		if err := runValidate(configPath); err != nil {
			fmt.Printf("Validation FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Validation PASSED: %s is valid\n", configPath)
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(configPath string) error {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", configPath, err)
	}

	expandedContent := os.ExpandEnv(string(file))

	var config FullConfig
	if err := yaml.Unmarshal([]byte(expandedContent), &config); err != nil {
		return fmt.Errorf("YAML parse error: %w", err)
	}

	var errors []string

	// Validate perfana section
	if config.Perfana.BaseUrl == "" {
		errors = append(errors, "perfana.baseUrl is required")
	}

	// Validate test section
	if config.Test.SystemUnderTest == "" {
		errors = append(errors, "test.systemUnderTest is required")
	}
	if config.Test.Environment == "" {
		errors = append(errors, "test.environment is required")
	}
	if config.Test.Workload == "" {
		errors = append(errors, "test.workload is required")
	}

	// Validate durations
	if config.Test.AnalysisStartOffset != "" {
		if _, err := util.ParseISODurationToSeconds(config.Test.AnalysisStartOffset); err != nil {
			errors = append(errors, fmt.Sprintf("test.analysisStartOffset: invalid ISO 8601 duration %q: %v", config.Test.AnalysisStartOffset, err))
		}
	}
	if config.Test.ConstantLoadTime != "" {
		if _, err := util.ParseISODurationToSeconds(config.Test.ConstantLoadTime); err != nil {
			errors = append(errors, fmt.Sprintf("test.constantLoadTime: invalid ISO 8601 duration %q: %v", config.Test.ConstantLoadTime, err))
		}
	}

	// Validate scheduler
	if config.Scheduler.KeepAliveIntervalSeconds < 0 {
		errors = append(errors, "scheduler.keepAliveIntervalSeconds must be >= 0")
	}

	// Validate events
	for i, ec := range config.Events {
		prefix := fmt.Sprintf("events[%d] (%s)", i, ec.Name)

		if ec.Name == "" {
			errors = append(errors, fmt.Sprintf("events[%d].name is required", i))
		}

		switch ec.Type {
		case "command":
			// Command events should have at least one command
			hasCmd := ec.Commands.OnBeforeTest != "" ||
				ec.Commands.OnStartTest != "" ||
				ec.Commands.OnKeepAlive != "" ||
				ec.Commands.OnAbort != "" ||
				ec.Commands.OnAfterTest != ""
			if !hasCmd {
				errors = append(errors, fmt.Sprintf("%s: command event should have at least one command hook", prefix))
			}

		case "config-collector":
			if ec.Command == "" {
				errors = append(errors, fmt.Sprintf("%s: config-collector requires 'command' field", prefix))
			}
			switch ec.Output {
			case "key":
				if ec.Key == "" {
					errors = append(errors, fmt.Sprintf("%s: output=key requires 'key' field", prefix))
				}
			case "keys", "json", "":
				// valid
			default:
				errors = append(errors, fmt.Sprintf("%s: output must be 'key', 'keys', or 'json' (got %q)", prefix, ec.Output))
			}

		case "":
			errors = append(errors, fmt.Sprintf("%s: event type is required", prefix))

		default:
			errors = append(errors, fmt.Sprintf("%s: unknown event type %q (expected 'command' or 'config-collector')", prefix, ec.Type))
		}
	}

	if len(errors) > 0 {
		msg := fmt.Sprintf("%d validation error(s):\n", len(errors))
		for _, e := range errors {
			msg += "  - " + e + "\n"
		}
		return fmt.Errorf(msg)
	}

	return nil
}
