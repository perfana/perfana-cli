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

type stopReason int

const (
	stopNormal   stopReason = iota
	stopSignal              // SIGINT / SIGTERM
	stopUIAbort             // abort flag set on test run via Perfana UI
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
	reason := s.runKeepAliveLoop()

	switch reason {
	case stopSignal:
		// 5a. Local signal abort: notify events and Perfana.
		s.runAbort()
		if err := s.Client.AbortTest(s.testRunID, s.buildAdditionalData()); err != nil {
			logger.Warn("failed to send abort", "err", err)
		}
		logger.Info("test aborted by signal")
		return fmt.Errorf("test aborted by signal")

	case stopUIAbort:
		// 5b. UI abort: Perfana already owns the abort state; just clean up events.
		s.runAbort()
		_ = s.runLifecyclePhase("AfterTest", func(e Event) error {
			return e.AfterTest(s.TestContext)
		})
		logger.Info("test aborted from UI, exiting gracefully")
		return nil
	}

	// 5c. Normal completion: send completed event to Perfana
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
	if err := s.checkPerfanaResults(); err != nil {
		// Run AfterTest before returning the failure so cleanup still happens.
		_ = s.runLifecyclePhase("AfterTest", func(e Event) error {
			return e.AfterTest(s.TestContext)
		})
		return err
	}

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
// and listens for SIGINT/SIGTERM. Returns the reason the loop stopped.
//
// The loop stops early when ALL events with continueOnKeepAliveParticipant=true
// have signaled done (consensus-based stop, matching the Java event-scheduler behavior).
func (s *EventScheduler) runKeepAliveLoop() stopReason {
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
			return stopNormal

		case <-sigChan:
			logger.Info("signal received, aborting")
			return stopSignal

		case <-keepAliveTicker.C:
			if err := s.sendTestEvent(false); err != nil {
				logger.Warn("keep-alive failed", "err", err)
			}

			if status, err := s.Client.GetTestRunStatus(s.testRunID); err == nil && status.Abort {
				logger.Info("test run aborted from UI")
				return stopUIAbort
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
				return stopNormal
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

// checkPerfanaResults polls the Perfana API until the test run is completed,
// retrying every 15 seconds for up to 5 minutes. Returns an error if the
// test results indicate failure, so the CI job exits non-zero.
func (s *EventScheduler) checkPerfanaResults() error {
	const (
		pollInterval = 15 * time.Second
		pollTimeout  = 5 * time.Minute
	)

	deadline := time.Now().Add(pollTimeout)
	for {
		result, err := s.Client.GetTestRunStatus(s.testRunID)
		if err != nil {
			logger.Warn("failed to check results", "err", err)
			return nil
		}

		if result.Completed {
			s.logTestRunResult(result)
			slosPassed := s.reportCheckResults(result)
			adaptPassed := s.reportAdaptResults()
			if !slosPassed || !adaptPassed {
				return fmt.Errorf("test run failed: slos_passed=%v adapt_passed=%v", slosPassed, adaptPassed)
			}
			return nil
		}

		if time.Now().After(deadline) {
			logger.Warn("timed out waiting for test run to complete", "test_run_id", s.testRunID)
			return nil
		}

		logger.Info("test run not yet completed, retrying", "test_run_id", s.testRunID)
		time.Sleep(pollInterval)
	}
}

func (s *EventScheduler) logTestRunResult(result *perfana_client.TestRunResult) {
	consolidated := "none"
	if result.ConsolidatedResult != nil {
		consolidated = *result.ConsolidatedResult
	}
	logger.Info("test run result",
		"test_run_id", result.TestRunID,
		"system", result.SystemsUnderTest.Name,
		"environment", result.TestEnvironment,
		"workload", result.Workload,
		"release", result.ApplicationRelease,
		"duration_s", result.Duration,
		"valid", result.Valid,
		"consolidated_result", consolidated,
		"changepoint", result.IsChangepoint,
	)
}

func (s *EventScheduler) reportCheckResults(result *perfana_client.TestRunResult) bool {
	checks, err := s.Client.GetCheckResults(
		s.testRunID,
		result.SystemsUnderTest.Name,
		result.TestEnvironment,
		result.Workload,
	)
	if err != nil {
		logger.Warn("failed to get check results", "err", err)
		return true // don't fail CI on fetch error
	}

	pass, fail := 0, 0
	for _, c := range checks {
		if c.MeetsRequirement {
			pass++
		} else {
			fail++
		}
	}
	logger.Info("check results summary", "total", len(checks), "pass", pass, "fail", fail)

	for _, c := range checks {
		status := "PASS"
		if !c.MeetsRequirement {
			status = "FAIL"
		}
		logger.Info("check",
			"status", status,
			"dashboard", c.DashboardLabel,
			"panel", c.PanelTitle,
			"average", c.PanelAverage,
			"requirement", fmt.Sprintf("%s %.4g %s", c.Requirement.Operator, c.Requirement.Value, c.MetricUnit),
			"message", c.Message,
		)
	}

	return fail == 0
}

func (s *EventScheduler) reportAdaptResults() bool {
	adapt, err := s.Client.GetAdaptConclusion(s.testRunID)
	if err != nil {
		logger.Warn("failed to get adapt conclusion", "err", err)
		return true // don't fail CI on fetch error
	}

	logger.Info("adapt conclusion",
		"conclusion", adapt.Conclusion,
		"regressions", len(adapt.Regressions),
		"improvements", len(adapt.Improvements),
		"differences", len(adapt.Differences),
	)

	for _, r := range adapt.Regressions {
		logger.Info("regression",
			"metric", r.MetricName,
			"dashboard", r.Dashboard,
			"panel", r.Panel,
			"current", fmt.Sprintf("%.4g %s", r.Current, r.Unit),
			"baseline", fmt.Sprintf("%.4g %s", r.Baseline, r.Unit),
			"change_pct", fmt.Sprintf("%+.1f%%", r.ChangePct),
		)
	}

	for _, i := range adapt.Improvements {
		logger.Info("improvement",
			"metric", i.MetricName,
			"dashboard", i.Dashboard,
			"panel", i.Panel,
			"current", fmt.Sprintf("%.4g %s", i.Current, i.Unit),
			"baseline", fmt.Sprintf("%.4g %s", i.Baseline, i.Unit),
			"change_pct", fmt.Sprintf("%+.1f%%", i.ChangePct),
		)
	}

	return adapt.Conclusion != "REGRESSION"
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
