package cmd

import (
	"os"
	"path/filepath"
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
test: {workload: stress, tags: [douane]}
scheduler: {failOnError: false}
events:
  - {name: collector, type: config-collector}
`)
	overlay := writeConfig(t, `
perfana: {apiUrl: "http://base", apiKey: base-key}
test: {tags: [k6]}
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
	if len(config.Events) != 2 || config.Events[0].Name != "collector" || config.Events[1].Name != "workers" {
		t.Errorf("want collector then workers, got %+v", config.Events)
	}
}
