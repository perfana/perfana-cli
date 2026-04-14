package events

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"perfana-cli/perfana_client"
	"perfana-cli/scheduler"
)

// ConfigCollectorConfig holds the YAML configuration for a config-collector event.
type ConfigCollectorConfig struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Command  string   `yaml:"command"`
	Output   string   `yaml:"output"`   // "key", "keys", or "json"
	Key      string   `yaml:"key"`      // used when output=key
	Includes []string `yaml:"includes"` // regex filters for json output
	Excludes []string `yaml:"excludes"` // regex filters for json output
	Tags     []string `yaml:"tags"`
}

// ConfigCollectorEvent executes a command, captures stdout, and uploads to Perfana.
type ConfigCollectorEvent struct {
	name     string
	command  string
	output   string
	key      string
	includes []string
	excludes []string
	tags     []string
}

// NewConfigCollectorEvent creates a ConfigCollectorEvent from config.
func NewConfigCollectorEvent(cfg ConfigCollectorConfig) *ConfigCollectorEvent {
	output := cfg.Output
	if output == "" {
		output = "key"
	}
	return &ConfigCollectorEvent{
		name:     cfg.Name,
		command:  cfg.Command,
		output:   output,
		key:      cfg.Key,
		includes: cfg.Includes,
		excludes: cfg.Excludes,
		tags:     cfg.Tags,
	}
}

func (e *ConfigCollectorEvent) Name() string                        { return e.name }
func (e *ConfigCollectorEvent) IsContinueOnKeepAliveParticipant() bool { return false }

// BeforeTest runs the config collection command and uploads to Perfana.
func (e *ConfigCollectorEvent) BeforeTest(ctx scheduler.TestContext) error {
	if e.command == "" {
		return nil
	}

	expanded := substituteVariables(e.command, ctx)
	log.Printf("[%s] Collecting config: %s", e.name, expanded)

	cmd := exec.Command("sh", "-c", expanded)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("config collection command failed: %w", err)
	}

	stdout := strings.TrimSpace(string(output))
	if stdout == "" {
		log.Printf("[%s] Config command produced no output, skipping upload", e.name)
		return nil
	}

	return e.uploadConfig(ctx, stdout)
}

func (e *ConfigCollectorEvent) StartTest(ctx scheduler.TestContext) error  { return nil }
func (e *ConfigCollectorEvent) KeepAlive(ctx scheduler.TestContext) error  { return nil }
func (e *ConfigCollectorEvent) CheckResults(ctx scheduler.TestContext) error { return nil }
func (e *ConfigCollectorEvent) AfterTest(ctx scheduler.TestContext) error  { return nil }
func (e *ConfigCollectorEvent) AbortTest(ctx scheduler.TestContext) error  { return nil }

func (e *ConfigCollectorEvent) OnEvent(ctx scheduler.TestContext, settings map[string]string) error {
	return nil
}

// uploadConfig sends the collected output to Perfana based on the output mode.
func (e *ConfigCollectorEvent) uploadConfig(ctx scheduler.TestContext, stdout string) error {
	client := ctx.Client
	testRunID := ctx.TestRunID

	systemUnderTest := ctx.SystemUnderTest
	environment := ctx.Environment
	workload := ctx.Workload

	switch e.output {
	case "key":
		log.Printf("[%s] Uploading single key: %s", e.name, e.key)
		return client.SendConfigKey(testRunID, systemUnderTest, environment, workload, e.key, stdout, e.tags)

	case "keys":
		items := parseKeyValueLines(stdout)
		if len(items) == 0 {
			log.Printf("[%s] No key=value pairs found in output", e.name)
			return nil
		}
		log.Printf("[%s] Uploading %d config keys", e.name, len(items))
		return client.SendConfigKeys(testRunID, systemUnderTest, environment, workload, items, e.tags)

	case "json":
		var jsonData interface{}
		if err := json.Unmarshal([]byte(stdout), &jsonData); err != nil {
			return fmt.Errorf("config output is not valid JSON: %w", err)
		}
		log.Printf("[%s] Uploading JSON config", e.name)
		return client.SendConfigJSON(testRunID, systemUnderTest, environment, workload, jsonData, e.includes, e.excludes, e.tags)

	default:
		return fmt.Errorf("unsupported output mode: %s (expected key, keys, or json)", e.output)
	}
}

// parseKeyValueLines parses "key=value" lines from stdout.
func parseKeyValueLines(stdout string) []perfana_client.ConfigItem {
	var items []perfana_client.ConfigItem
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			items = append(items, perfana_client.ConfigItem{
				Key:   strings.TrimSpace(parts[0]),
				Value: strings.TrimSpace(parts[1]),
			})
		}
	}
	return items
}
