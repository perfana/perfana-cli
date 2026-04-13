package events

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"perfana-cli/scheduler"
)

// CommandEventConfig holds the YAML configuration for a command event.
type CommandEventConfig struct {
	Name                           string            `yaml:"name"`
	Type                           string            `yaml:"type"`
	ContinueOnKeepAliveParticipant bool              `yaml:"continueOnKeepAliveParticipant"`
	Commands                       CommandHooks      `yaml:"commands"`
}

// CommandHooks maps lifecycle hooks to shell commands.
type CommandHooks struct {
	OnBeforeTest string `yaml:"onBeforeTest"`
	OnStartTest  string `yaml:"onStartTest"`
	OnKeepAlive  string `yaml:"onKeepAlive"`
	OnAbort      string `yaml:"onAbort"`
	OnAfterTest  string `yaml:"onAfterTest"`
}

// CommandEvent executes shell commands at each lifecycle hook.
type CommandEvent struct {
	name                           string
	commands                       CommandHooks
	continueOnKeepAliveParticipant bool
	keepAliveDone                  bool
}

// NewCommandEvent creates a CommandEvent from config.
func NewCommandEvent(cfg CommandEventConfig) *CommandEvent {
	return &CommandEvent{
		name:                           cfg.Name,
		commands:                       cfg.Commands,
		continueOnKeepAliveParticipant: cfg.ContinueOnKeepAliveParticipant,
	}
}

func (e *CommandEvent) Name() string { return e.name }

func (e *CommandEvent) BeforeTest(ctx scheduler.TestContext) error {
	return e.runCommand(ctx, e.commands.OnBeforeTest, "BeforeTest")
}

func (e *CommandEvent) StartTest(ctx scheduler.TestContext) error {
	return e.runCommand(ctx, e.commands.OnStartTest, "StartTest")
}

func (e *CommandEvent) KeepAlive(ctx scheduler.TestContext) error {
	if e.keepAliveDone {
		return fmt.Errorf("keep-alive participant already done")
	}

	if e.commands.OnKeepAlive == "" {
		return nil
	}

	err := e.runCommand(ctx, e.commands.OnKeepAlive, "KeepAlive")
	if err != nil && e.continueOnKeepAliveParticipant {
		// Non-zero exit signals this event's work is done
		e.keepAliveDone = true
		return fmt.Errorf("keep-alive participant done: %w", err)
	}
	return err
}

func (e *CommandEvent) OnEvent(ctx scheduler.TestContext, settings map[string]string) error {
	// Scheduled events can pass settings as __key__ placeholders
	// Build a command from settings if present, otherwise no-op
	cmd, ok := settings["command"]
	if !ok {
		log.Printf("[%s] OnEvent: no command in settings, skipping", e.name)
		return nil
	}
	return e.runCommand(ctx, cmd, "OnEvent")
}

func (e *CommandEvent) CheckResults(ctx scheduler.TestContext) error {
	// Command events don't check results
	return nil
}

func (e *CommandEvent) AfterTest(ctx scheduler.TestContext) error {
	return e.runCommand(ctx, e.commands.OnAfterTest, "AfterTest")
}

func (e *CommandEvent) AbortTest(ctx scheduler.TestContext) error {
	return e.runCommand(ctx, e.commands.OnAbort, "AbortTest")
}

// runCommand executes a shell command with variable substitution.
func (e *CommandEvent) runCommand(ctx scheduler.TestContext, command, phase string) error {
	if command == "" {
		return nil
	}

	// Variable substitution
	expanded := substituteVariables(command, ctx)

	log.Printf("[%s] %s: %s", e.name, phase, expanded)

	cmd := exec.Command("sh", "-c", expanded)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

// substituteVariables replaces __key__ placeholders in a command string.
func substituteVariables(command string, ctx scheduler.TestContext) string {
	result := command
	result = strings.ReplaceAll(result, "__testRunId__", ctx.TestRunID)
	result = strings.ReplaceAll(result, "__systemUnderTest__", ctx.SystemUnderTest)
	result = strings.ReplaceAll(result, "__environment__", ctx.Environment)
	result = strings.ReplaceAll(result, "__workload__", ctx.Workload)
	result = strings.ReplaceAll(result, "__version__", ctx.Version)

	// Substitute user-defined variables
	for k, v := range ctx.Variables {
		result = strings.ReplaceAll(result, fmt.Sprintf("__%s__", k), v)
	}

	return result
}
