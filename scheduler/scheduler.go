package scheduler

import (
	"fmt"
	"perfana-cli/logger"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"perfana-cli/perfana_client"
)

// EventScheduler orchestrates the full test lifecycle:
// BeforeTest → StartTest → KeepAlive loop (+ scheduled events) → CheckResults → AfterTest
type EventScheduler struct {
	Client               *perfana_client.PerfanaClient
	Events               []Event
	ScheduleEntries      []ScheduleEntry
	KeepAliveIntervalSec int
	TestDurationSec      int
	TestContext          TestContext
	FailOnError          bool

	testRunID string
}

// Run executes the full event lifecycle. It blocks until the test completes,
// is aborted by signal, or a fatal error occurs.
func (s *EventScheduler) Run() error {
	// 1. Initialize Perfana session
	testRunID, err := s.Client.Init()
	if err != nil {
		return fmt.Errorf("perfana init failed: %w", err)
	}
	s.testRunID = testRunID
	s.TestContext.TestRunID = testRunID
	logger.Info("session initialized", "testRunId", testRunID)

	// 2. BeforeTest on all events
	if err := s.runLifecyclePhase("BeforeTest", func(e Event) error {
		return e.BeforeTest(s.TestContext)
	}); err != nil {
		return err
	}

	// 3. StartTest on all events
	if err := s.runLifecyclePhase("StartTest", func(e Event) error {
		return e.StartTest(s.TestContext)
	}); err != nil {
		s.runAbort()
		return err
	}

	// Send initial test event to Perfana
	if err := s.sendTestEvent(false); err != nil {
		logger.Warn("failed to send initial test event", "err", err)
	}

	// 4. KeepAlive loop with signal handling and scheduled events
	aborted := s.runKeepAliveLoop()

	if aborted {
		// 5a. AbortTest flow
		s.runAbort()
		if err := s.Client.AbortTest(s.testRunID, s.buildAdditionalData()); err != nil {
			logger.Warn("failed to send abort", "err", err)
		}
		logger.Info("test aborted")
		return fmt.Errorf("test aborted by signal")
	}

	// 5b. Normal completion: send completed event to Perfana
	if err := s.sendTestEvent(true); err != nil {
		logger.Warn("failed to send completion event", "err", err)
	}

	// 6. CheckResults on all events
	if err := s.runLifecyclePhase("CheckResults", func(e Event) error {
		return e.CheckResults(s.TestContext)
	}); err != nil {
		logger.Warn("check results error", "err", err)
	}

	// Check Perfana results
	s.checkPerfanaResults()

	// 7. AfterTest on all events
	if err := s.runLifecyclePhase("AfterTest", func(e Event) error {
		return e.AfterTest(s.TestContext)
	}); err != nil {
		logger.Warn("after test error", "err", err)
	}

	logger.Info("test completed")
	return nil
}

// runLifecyclePhase calls fn on each event in order. If failOnError is true,
// the first error stops execution.
func (s *EventScheduler) runLifecyclePhase(phase string, fn func(Event) error) error {
	for _, event := range s.Events {
		if err := fn(event); err != nil {
			logger.Warn("event error", "phase", phase, "event", event.Name(), "err", err)
			if s.FailOnError {
				return fmt.Errorf("%s failed for event %s: %w", phase, event.Name(), err)
			}
		}
	}
	return nil
}

// runKeepAliveLoop runs the keep-alive ticker, fires scheduled events at their times,
// and listens for SIGINT/SIGTERM. Returns true if aborted by signal.
//
// The loop stops early when ALL events with continueOnKeepAliveParticipant=true
// have signaled done (consensus-based stop, matching the Java event-scheduler behavior).
func (s *EventScheduler) runKeepAliveLoop() bool {
	keepAliveInterval := time.Duration(s.KeepAliveIntervalSec) * time.Second
	if keepAliveInterval <= 0 {
		keepAliveInterval = 30 * time.Second
	}

	keepAliveTicker := time.NewTicker(keepAliveInterval)
	defer keepAliveTicker.Stop()

	testTimeout := time.After(time.Duration(s.TestDurationSec) * time.Second)

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Prepare scheduled events sorted by delay
	scheduleTimers := s.startScheduleTimers()
	defer func() {
		for _, t := range scheduleTimers {
			t.Stop()
		}
	}()

	// Count keep-alive participants for consensus-based stop
	keepAliveParticipantCount := 0
	for _, event := range s.Events {
		if event.IsContinueOnKeepAliveParticipant() {
			keepAliveParticipantCount++
		}
	}
	if keepAliveParticipantCount > 0 {
		logger.Info("keep-alive participants registered", "count", keepAliveParticipantCount)
	}

	// Track which keep-alive participants have signaled done
	keepAliveParticipantsDone := make(map[string]bool)

	for {
		select {
		case <-testTimeout:
			logger.Info("test duration reached")
			return false

		case <-sigChan:
			logger.Info("signal received, aborting")
			return true

		case <-keepAliveTicker.C:
			if err := s.sendTestEvent(false); err != nil {
				logger.Warn("keep-alive failed", "err", err)
			}

			for _, event := range s.Events {
				if err := event.KeepAlive(s.TestContext); err != nil {
					if event.IsContinueOnKeepAliveParticipant() && !keepAliveParticipantsDone[event.Name()] {
						keepAliveParticipantsDone[event.Name()] = true
						logger.Info("participant done", "event", event.Name(), "done", len(keepAliveParticipantsDone), "total", keepAliveParticipantCount)
					} else if !keepAliveParticipantsDone[event.Name()] {
						logger.Warn("keep-alive error", "event", event.Name(), "err", err)
					}
				}
			}

			if keepAliveParticipantCount > 0 && len(keepAliveParticipantsDone) >= keepAliveParticipantCount {
				logger.Info("all participants done, stopping")
				return false
			}
		}
	}
}

// startScheduleTimers creates time.Timer instances for each scheduled event entry.
// When a timer fires, it calls OnEvent on the matching event and posts to Perfana /events.
func (s *EventScheduler) startScheduleTimers() []*time.Timer {
	// Sort by delay for predictable ordering
	sorted := make([]ScheduleEntry, len(s.ScheduleEntries))
	copy(sorted, s.ScheduleEntries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Delay < sorted[j].Delay
	})

	var timers []*time.Timer
	for _, entry := range sorted {
		entry := entry // capture
		t := time.AfterFunc(time.Duration(entry.Delay)*time.Second, func() {
			logger.Info("firing scheduled event", "event", entry.EventName, "delaySeconds", entry.Delay)

			for _, event := range s.Events {
				if event.Name() == entry.EventName {
					if err := event.OnEvent(s.TestContext, entry.Settings); err != nil {
						logger.Warn("scheduled event error", "event", entry.EventName, "err", err)
					}
					break
				}
			}

			title := entry.EventName
			if entry.Description != "" {
				title = entry.Description
			}
			perfanaEvent := perfana_client.PerfanaEvent{
				SystemUnderTest: s.TestContext.SystemUnderTest,
				TestEnvironment: s.TestContext.Environment,
				Workload:        s.TestContext.Workload,
				Title:           title,
				Description:     fmt.Sprintf("Scheduled event: %s", entry.EventName),
				Tags:            s.TestContext.Tags,
			}
			if _, err := s.Client.SendPerfanaEvent(perfanaEvent); err != nil {
				logger.Warn("failed to post event", "event", entry.EventName, "err", err)
			}
		})
		timers = append(timers, t)
	}

	return timers
}

// runAbort calls AbortTest on all events.
func (s *EventScheduler) runAbort() {
	for _, event := range s.Events {
		if err := event.AbortTest(s.TestContext); err != nil {
			logger.Warn("abort error", "event", event.Name(), "err", err)
		}
	}
}

// checkPerfanaResults calls the Perfana API to check test run results.
func (s *EventScheduler) checkPerfanaResults() {
	result, err := s.Client.GetTestRunStatus(s.testRunID)
	if err != nil {
		logger.Warn("failed to check results", "err", err)
		return
	}
	logger.Info("test run result", "result", result)
}

// sendTestEvent sends a keep-alive or completion event to Perfana.
func (s *EventScheduler) sendTestEvent(completed bool) error {
	return s.Client.TestEvent(s.testRunID, s.buildAdditionalData(), completed)
}

// buildAdditionalData constructs the additional data map for TestEvent calls.
func (s *EventScheduler) buildAdditionalData() map[string]interface{} {
	data := map[string]interface{}{
		"tags": s.TestContext.Tags,
	}
	if s.TestContext.Version != "" {
		data["version"] = s.TestContext.Version
	}
	if s.TestContext.Annotations != "" {
		data["annotations"] = s.TestContext.Annotations
	}
	if s.TestContext.AnalysisStartOffset > 0 {
		data["analysisStartOffset"] = s.TestContext.AnalysisStartOffset
	}
	if s.TestContext.Duration > 0 {
		data["duration"] = s.TestContext.Duration
	}
	if s.TestContext.BuildResultsUrl != "" {
		data["cibuildResultsUrl"] = s.TestContext.BuildResultsUrl
	}
	if len(s.TestContext.DeepLinks) > 0 {
		data["deepLinks"] = s.TestContext.DeepLinks
	}
	if len(s.TestContext.Variables) > 0 {
		vars := make([]perfana_client.Variable, 0, len(s.TestContext.Variables))
		for k, v := range s.TestContext.Variables {
			vars = append(vars, perfana_client.Variable{Placeholder: k, Value: v})
		}
		data["variables"] = vars
	}
	return data
}
