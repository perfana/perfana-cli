package events

import (
	"testing"

	"perfana-cli/scheduler"
)

func TestConfigCollectorFailsOnNoOutput(t *testing.T) {
	for _, tc := range []struct{ output, command string }{
		{"key", "true"},
		{"key", "false | cat"}, // a failed curl piped into jq exits 0 with no output
		{"keys", "echo not-a-key-value-line"},
	} {
		e := NewConfigCollectorEvent(ConfigCollectorConfig{Name: "c", Output: tc.output, Command: tc.command})
		if err := e.BeforeTest(scheduler.TestContext{}); err == nil {
			t.Errorf("%q with output %s: want an error", tc.command, tc.output)
		}
	}
}
