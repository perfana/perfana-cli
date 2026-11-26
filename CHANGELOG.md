# Changelog

## Unreleased

### Breaking changes

- **`additionalSutMonitoring` entries take `tags` instead of `profile`.** With `tags`, an entry gets the profiles that share at least one of those tags. Without `tags`, it gets the profiles the run's own tags match. Perfana no longer reads `profile`, and the CLI now ignores it, so move any `profile` to `tags`:

  ```yaml
  # Before
  - {systemUnderTest: kafka, testEnvironment: acceptance, profile: kafka}
  # After
  - {systemUnderTest: kafka, testEnvironment: acceptance, tags: [kafka]}
  ```

- **Stricter `additionalSutMonitoring` validation**, to match the API: `systemUnderTest` and `testEnvironment` must be 1-100 characters of letters, digits, `.`, `_` or `-`. An entry can have at most 50 tags, each 1-100 characters.

## v0.9.0 (2026-11-26)

### Features

- **`test.additionalSutMonitoring`.** List extra `systemUnderTest`/`testEnvironment` combinations (with an optional `profile`) whose dashboards, profile SLOs and series-grouping rules grafana-sync auto-config adds to the run. They are stored under the run's own SUT/environment; no SUT/environment is created for them. The list is sent on every test-run call. An overlay that sets it replaces the list. `validate` and `run start` reject more than 20 entries, and empty or over-255-character values. Needs a Perfana server that supports the field.

## v0.8.0 (2026-11-05)

### Breaking changes

- **`constantLoadTime` is replaced by `duration`.** The `--constantLoadTime` flag and the `test.constantLoadTime` YAML key are removed. `duration` is the total test length; analysis offsets only select the time range to analyse within it.

  To upgrade, set `duration` to your old `analysisStartOffset` + `constantLoadTime`:

  ```yaml
  # Before
  analysisStartOffset: PT5M
  constantLoadTime: PT15M

  # After
  analysisStartOffset: PT5M
  duration: PT20M
  ```

  The old flag now fails with an unknown flag error. `run start` and `validate` both reject a config that still has the old YAML key.

- **The `duration` sent to Perfana is now the total test length.** It used to leave out the analysis start offset, so a 20-minute test was reported as 15 minutes.

- **`duration` no longer stops a test with keep-alive participants.** If any event has `continueOnKeepAliveParticipant: true`, the test runs until all participants are done. It only stops on `duration` when there are no participants.

- **A failing `onStartTest` command now fails the run.** For keep-alive participants, a non-zero exit used to be logged only. `run start` now exits non-zero when `scheduler.failOnError` is `true`. This includes k6 exiting with code 99 when a threshold fails.

- **`run start` exits non-zero on setup errors.** An unreadable or invalid config, an invalid duration, or a client setup failure used to print an error and exit 0.

- **A `config-collector` command with no output is now an error.** It used to be skipped, which hid failures: a failed `curl | jq` exits 0 with no output. The same applies to `output: keys` with no `key=value` lines. With `scheduler.failOnError: true` this stops the test before it starts. To let a test run without a value, give the command a fallback, for example `cat version.txt || echo unknown`.

### Added

- `run start` and `validate` accept `--overlay <file>`, a config file applied on top of `--config`. Its values win, except that `events` and `test.tags` are combined, with its events running after those from `--config`.
- `run start --analysisEndOffset` sets the trailing period excluded from analysis and overrides `test.analysisEndOffset`.
- A keep-alive participant is now done when its `onStartTest` command exits, so `onKeepAlive` is optional.

### Fixed

- `--version=1.0.0` and `--analysisStartOffset=PT5M` are no longer ignored when the YAML sets a different value or none.
