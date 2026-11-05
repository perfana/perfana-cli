package scheduler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"perfana-cli/events"
	"perfana-cli/perfana_client"
	"perfana-cli/scheduler"
)

// A keep-alive participant outlives the test duration, then fails: the run must
// wait for it and return its failure.
func TestRunWaitsForParticipantAndReturnsItsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/init":
			w.Write([]byte(`{"testRunId":"run-1"}`))
		case strings.HasPrefix(r.URL.Path, "/api/test-runs/"):
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := perfana_client.NewClient(perfana_client.Configuration{ApiUrl: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	load := events.NewCommandEvent(events.CommandEventConfig{
		Name:                           "load",
		ContinueOnKeepAliveParticipant: true,
		Commands:                       events.CommandHooks{OnStartTest: "sleep 2; exit 3"},
	})

	s := &scheduler.EventScheduler{
		Client:               client,
		Events:               []scheduler.Event{load},
		KeepAliveIntervalSec: 1,
		TestDurationSec:      1,
		TestContext:          scheduler.TestContext{Client: client},
		FailOnError:          true,
	}

	err = s.Run()
	if err == nil || !strings.Contains(err.Error(), "exit status 3") {
		t.Fatalf("want exit status 3 from the participant, got %v", err)
	}
}
