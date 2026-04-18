package scheduler

import "perfana-cli/perfana_client"

// TestContext holds the runtime context passed to each event lifecycle method.
type TestContext struct {
	TestRunID           string
	SystemUnderTest     string
	Environment         string
	Workload            string
	Version             string
	Tags                []string
	Variables           map[string]string
	Annotations         string
	AnalysisStartOffset int
	Duration            int
	BuildResultsUrl     string
	DeepLinks           []perfana_client.DeepLink
	Client              *perfana_client.PerfanaClient
}

// Event defines the lifecycle interface for test events.
// Each event participates in the scheduler's orchestrated test lifecycle.
type Event interface {
	// Name returns the unique name of this event.
	Name() string

	// BeforeTest is called before the test starts. Use for setup/preparation.
	BeforeTest(ctx TestContext) error

	// StartTest is called when the test begins.
	StartTest(ctx TestContext) error

	// KeepAlive is called periodically during the test to check event health.
	// Return a non-nil error to signal that this event's work is done (for continueOnKeepAliveParticipant).
	KeepAlive(ctx TestContext) error

	// OnEvent is called when a scheduled event fires targeting this event by name.
	OnEvent(ctx TestContext, settings map[string]string) error

	// CheckResults is called after the test completes to verify results.
	CheckResults(ctx TestContext) error

	// AfterTest is called after test completion for cleanup.
	AfterTest(ctx TestContext) error

	// AbortTest is called when the test is aborted (e.g., SIGINT/SIGTERM).
	AbortTest(ctx TestContext) error

	// IsContinueOnKeepAliveParticipant returns true if this event participates
	// in the keep-alive consensus: the test stops early when ALL participants
	// with this flag have signaled done.
	IsContinueOnKeepAliveParticipant() bool
}

// ScheduleEntry represents a parsed schedule script line.
// Format: PT<duration>|<eventName(description)>|<key=value;key=value>
type ScheduleEntry struct {
	// Delay is the offset from test start in seconds when this event fires.
	Delay int

	// EventName is the name of the event to target.
	EventName string

	// Description is the optional parenthesized description.
	Description string

	// Settings are the key=value pairs parsed from the settings segment.
	Settings map[string]string
}
