# YAML Configuration Reference

The `perfana-cli` reads configuration from a YAML file. By default it looks for `~/.perfana-cli/perfana.yaml`, or you can specify a custom path with `--config`.

## Full example

```yaml
perfana:
  apiKey: "${PERFANA_API_KEY}"
  apiUrl: "https://perfana.example.com"
  appUrl: "https://perfana.example.com"
  mtls:
    clientKeyPath: "/path/to/key.pem"
    clientCertPath: "/path/to/cert.pem"

test:
  systemUnderTest: "MyApp"
  environment: "loadtest"
  workload: "peak-hours"
  version: "1.0.0"
  analysisStartOffset: "PT2M"
  constantLoadTime: "PT15M"
  tags:
    - "nightly"
  annotations: "Nightly peak-hour simulation"
  deepLinks:
    - name: "Grafana"
      url: "https://grafana.example.com/d/abc"
  variables:
    - placeholder: "__region__"
      value: "eu-west-1"

scheduler:
  enabled: true
  failOnError: true
  keepAliveIntervalSeconds: 30
  scheduleScript: |
    PT30S|scale-up(scale to 3)|name=k8sScaler;replicas=3
    PT10M|scale-down(scale to 1)|name=k8sScaler;replicas=1

events:
  - name: "load-runner"
    type: command
    continueOnKeepAliveParticipant: true
    commands:
      onBeforeTest: "echo Preparing test environment"
      onStartTest: "k6 run /tests/load-test.js"
      onKeepAlive: "pgrep k6"
      onAbort: "pkill k6"
      onAfterTest: "echo Test completed"

  - name: "git-info"
    type: config-collector
    command: "git log -1 --format='%H'"
    output: key
    key: "git-commit"
    tags:
      - "GitHub"
```

## Section reference

### `perfana` - Connection settings

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `apiKey` | Yes | | Perfana API key. Supports env var substitution: `${PERFANA_API_KEY}` |
| `apiUrl` | Yes | | Perfana API base URL (e.g. `http://localhost:3001`) |
| `appUrl` | No | | Perfana UI URL — when set, a direct link to the test run is printed at the end (e.g. `http://localhost:4000`) |
| `mtls.clientKeyPath` | No | | Path to PEM-encoded private key for mTLS |
| `mtls.clientCertPath` | No | | Path to PEM-encoded certificate for mTLS |

### `test` - Test session settings

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `systemUnderTest` | Yes | | Name of the system being tested (alphanumeric, `.`, `_`, `-`) |
| `environment` | Yes | | Test environment identifier |
| `workload` | Yes | | Workload name |
| `version` | No | | Version of the system under test |
| `analysisStartOffset` | No | `PT5M` | Offset before analysis starts (typically the ramp-up window, ISO 8601) |
| `constantLoadTime` | No | `PT15M` | Constant load duration (ISO 8601) |
| `tags` | No | | List of tags for filtering and grouping |
| `annotations` | No | | Free-text annotation for the test run |
| `deepLinks` | No | | Links displayed in the Perfana dashboard |
| `deepLinks[].name` | Yes | | Display name for the link |
| `deepLinks[].url` | Yes | | URL target |
| `variables` | No | | Key-value pairs sent to Perfana |
| `variables[].placeholder` | Yes | | Variable name |
| `variables[].value` | Yes | | Variable value |

### `scheduler` - Event scheduler settings

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `enabled` | No | `false` | Enable the event scheduler |
| `failOnError` | No | `true` | Fail the test if a scheduled event errors |
| `keepAliveIntervalSeconds` | No | `30` | Interval between keep-alive heartbeats (seconds) |
| `scheduleScript` | No | | Multi-line schedule script (see format below) |

#### Schedule script format

Each line defines a timed event:

```
<delay>|<event-name>(<description>)|<parameters>
```

- `<delay>` - ISO 8601 duration from test start (e.g. `PT30S`, `PT5M`)
- `<event-name>` - name of the event to fire
- `<description>` - optional description in parentheses
- `<parameters>` - semicolon-separated `key=value` pairs

Example:
```
PT30S|scale-up(scale to 3 replicas)|name=k8sScaler;replicas=3
PT10M|scale-down(scale to 1 replica)|name=k8sScaler;replicas=1
```

### `events` - Event handlers

Events define actions that run at specific lifecycle points.

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | | Event handler name |
| `type` | Yes | | Event type: `command` or `config-collector` |
| `continueOnKeepAliveParticipant` | No | `false` | Participate in keep-alive consensus (see below) |

#### `continueOnKeepAliveParticipant` behavior

When `true`, this event participates in consensus-based test stopping:

- **`onStartTest`** runs **asynchronously** (non-blocking), so multiple load generators can start in parallel.
- **`onKeepAlive`** is called every tick. When the command fails (non-zero exit), the participant signals "done".
- **The test stops early** when **all** `continueOnKeepAliveParticipant` events have signaled done. If only some are done, the test keeps running.
- **`onBeforeTest`** always runs **synchronously**, even for keep-alive participants. Use this for setup that must complete before the test starts (e.g. `kubectl rollout status`).

This matches the Java event-scheduler's `StopTestRunException` consensus behavior.

#### Type: `command`

Runs shell commands at lifecycle hooks. Commands support `__testRunId__`, `__systemUnderTest__`, `__environment__`, `__workload__`, and `__version__` placeholder substitution.

| Field | Description |
|-------|-------------|
| `commands.onBeforeTest` | Runs before the test starts (always synchronous) |
| `commands.onStartTest` | Runs when the test starts (async when `continueOnKeepAliveParticipant: true`) |
| `commands.onKeepAlive` | Runs on each keep-alive tick. Non-zero exit signals "done" for keep-alive participants |
| `commands.onAbort` | Runs if the test is aborted (e.g. SIGINT) |
| `commands.onAfterTest` | Runs after the test completes |

#### Type: `config-collector`

Collects configuration data and sends it to Perfana.

| Field | Description |
|-------|-------------|
| `command` | Shell command to execute |
| `output` | Output format: `key` (single value) or `json` |
| `key` | Config key name (when `output: key`) |
| `tags` | Tags for the config entry |

## Environment variable substitution

String values in the YAML support `${VAR_NAME}` syntax for environment variable substitution. This is useful for secrets:

```yaml
perfana:
  apiKey: "${PERFANA_API_KEY}"
```
