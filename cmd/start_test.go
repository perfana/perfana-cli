package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"perfana-cli/perfana_client"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "perfana.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigOverlay(t *testing.T) {
	repo := writeConfig(t, `
perfana: {apiUrl: "http://repo", apiKey: repo-key, appUrl: "http://repo-app"}
test:
  workload: stress
  tags: [douane]
  additionalSutMonitoring: [{systemUnderTest: a, testEnvironment: b}, {systemUnderTest: c, testEnvironment: d}]
scheduler: {failOnError: false}
events:
  - {name: collector, type: config-collector}
`)
	overlay := writeConfig(t, `
perfana: {apiUrl: "http://base", apiKey: base-key}
test:
  tags: [k6]
  additionalSutMonitoring: [{systemUnderTest: foo, testEnvironment: bar, profile: k8s}]
scheduler: {failOnError: true}
events:
  - {name: workers, type: command}
`)

	config, err := loadConfig(repo, overlay)
	if err != nil {
		t.Fatal(err)
	}

	if config.Perfana.ApiUrl != "http://base" || config.Perfana.ApiKey != "base-key" {
		t.Errorf("overlay should win, got %+v", config.Perfana)
	}
	if !config.Scheduler.FailOnError {
		t.Error("overlay should set failOnError")
	}
	if config.Test.Workload != "stress" || config.Perfana.AppUrl != "http://repo-app" {
		t.Errorf("values the overlay doesn't set should be kept, got %+v", config)
	}
	if strings.Join(config.Test.Tags, ",") != "douane,k6" {
		t.Errorf("want tags douane,k6, got %v", config.Test.Tags)
	}
	if sm := config.Test.AdditionalSutMonitoring; len(sm) != 1 || sm[0].SystemUnderTest != "foo" || sm[0].Profile != "k8s" {
		t.Errorf("overlay additionalSutMonitoring should replace, got %+v", sm)
	}
	if len(config.Events) != 2 || config.Events[0].Name != "collector" || config.Events[1].Name != "workers" {
		t.Errorf("want collector then workers, got %+v", config.Events)
	}
}

func TestSutMonitoringPayload(t *testing.T) {
	b, _ := json.Marshal(perfana_client.PerfanaMessage{AdditionalSutMonitoring: []perfana_client.SutMonitoring{
		{SystemUnderTest: "foo", TestEnvironment: "bar"},
		{SystemUnderTest: "acme", TestEnvironment: "test", Profile: "k8s"},
	}})
	want := `"additionalSutMonitoring":[{"systemUnderTest":"foo","testEnvironment":"bar"},{"systemUnderTest":"acme","testEnvironment":"test","profile":"k8s"}]`
	if !strings.Contains(string(b), want) {
		t.Errorf("want %s in %s", want, b)
	}
	if b, _ := json.Marshal(perfana_client.PerfanaMessage{}); strings.Contains(string(b), "additionalSutMonitoring") {
		t.Errorf("unset field should be omitted, got %s", b)
	}
}

func TestSutMonitoringErrors(t *testing.T) {
	ok := perfana_client.SutMonitoring{SystemUnderTest: "foo", TestEnvironment: "bar"}
	if errs := sutMonitoringErrors([]perfana_client.SutMonitoring{ok}); len(errs) != 0 {
		t.Errorf("valid entry rejected: %v", errs)
	}
	if errs := sutMonitoringErrors(make([]perfana_client.SutMonitoring, 21)); len(errs) != 1+21*2 {
		t.Errorf("want count error plus 2 required errors per entry, got %d: %v", len(errs), errs)
	}
	long := ok
	long.Profile = strings.Repeat("x", 256)
	if errs := sutMonitoringErrors([]perfana_client.SutMonitoring{long}); len(errs) != 1 {
		t.Errorf("want 1 length error, got %v", errs)
	}
}
