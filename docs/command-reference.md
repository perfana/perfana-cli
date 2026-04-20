# Command Reference

## Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.perfana-cli/perfana.yaml` | Path to config file |

## `perfana-cli init`

Initialize configuration for Perfana. Creates `~/.perfana-cli/perfana.yaml` with connection settings.

```bash
perfana-cli init [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--apiUrl` | | Perfana API base URL (e.g. `http://localhost:3001`) |
| `--apiKey` | | Perfana API key |
| `--clientIdentifier` | | Client identifier (for Perfana Cloud) |
| `--systemUnderTest` | | System under test name |
| `--environment` | | Test environment name |
| `--workload` | | Workload name |
| `--clientCertPath` | | Path to PEM client certificate (mTLS) |
| `--clientKeyPath` | | Path to PEM private key (mTLS) |

### Example

```bash
perfana-cli init \
  --apiUrl https://acme.perfana.cloud \
  --apiKey "$PERFANA_API_KEY" \
  --systemUnderTest "WebShop" \
  --environment "acceptance" \
  --workload "peak-load"
```

## `perfana-cli run start`

Start a Perfana test session with full event lifecycle orchestration. Runs until the total duration (rampup + constant load) elapses or the process is killed.

```bash
perfana-cli run start [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--analysisStartOffset` | `PT5M` | Offset before analysis starts (typically the ramp-up window), ISO 8601 format |
| `--constantLoadTime` | `PT15M` | Constant load duration in ISO 8601 format |
| `--version` | `1.0.0` | Version of the system under test |
| `--tags` | `k6,jfr` | Comma-separated tags for the test session |
| `--annotation` | | Annotation message for the test session |
| `--buildResultsUrl` | | URL to CI build results |
| `--variable` | | Variables as `key=value` (repeatable) |
| `--deeplink` | | Deep links as `title\|url` (repeatable) |

### Duration format

Durations use ISO 8601 format:
- `PT30S` - 30 seconds
- `PT5M` - 5 minutes
- `PT1H` - 1 hour
- `PT1H30M` - 1 hour 30 minutes

### Lifecycle

1. **Init** - registers the test session with Perfana
2. **BeforeTest** - runs pre-test events synchronously (e.g. deploy infrastructure, wait for readiness)
3. **StartTest** - begins the test. Events with `continueOnKeepAliveParticipant: true` run asynchronously; others run sequentially
4. **KeepAlive** - heartbeats every 30 seconds. When all keep-alive participants signal done, the test stops early
5. **CheckResults** - queries Perfana for analysis results
6. **AfterTest** - runs post-test cleanup events

### Example

```bash
perfana-cli run start \
  --analysisStartOffset=PT2M \
  --constantLoadTime=PT20M \
  --version="2.1.0" \
  --tags="gatling,sprint-42" \
  --annotation="Sprint 42 regression test" \
  --deeplink "Dashboard|https://grafana.example.com/d/abc" \
  --variable "region=eu-west-1"
```

## `perfana-cli run stop`

Stop a currently running Perfana test session.

```bash
perfana-cli run stop
```

## `perfana-cli version`

Print version, commit hash, and build date.

```bash
perfana-cli version
```

Output:
```
perfana-cli 1.0.0 (commit: a1b2c3d, built: 2026-04-13T10:00:00Z)
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `PERFANA_API_KEY` | API key (can be used in `perfana.yaml` as `${PERFANA_API_KEY}`) |
