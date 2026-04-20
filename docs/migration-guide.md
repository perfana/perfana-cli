# Migration Guide: Maven Plugins to perfana-cli

This guide helps you migrate from the Perfana Maven plugins (`perfana-java-client`, `event-scheduler-maven-plugin`) to `perfana-cli`.

## Why migrate?

- **No Java/Maven dependency** - `perfana-cli` is a single Go binary
- **Works with any load test tool** - JMeter, Gatling, k6, Locust, or any CLI tool
- **YAML configuration** - simpler than XML pom.xml configuration
- **CI/CD native** - GitHub Action, Docker image, and examples for all major CI platforms
- **Faster startup** - no JVM overhead

## Concept mapping

| Maven Plugin Concept | perfana-cli Equivalent |
|---------------------|----------------------|
| `pom.xml` plugin config | `perfana.yaml` |
| `perfana-java-client` | Built-in Perfana API client |
| `event-scheduler-maven-plugin` | Built-in event scheduler |
| Maven profile for test config | `perfana.yaml` per environment |
| `mvn perfana:event-schedule` | `perfana-cli run start` |
| `systemUnderTest` property | `test.systemUnderTest` in YAML |
| `testEnvironment` property | `test.environment` in YAML |
| Event plugins (Java classes) | `events[].type: command` (shell commands) |
| `perfanaUrl` | `perfana.apiUrl` |
| `apiKey` | `perfana.apiKey` |

## Step-by-step migration

### 1. Install perfana-cli

```bash
brew install perfana/tap/perfana-cli
```

### 2. Initialize configuration

```bash
perfana-cli init \
  --apiUrl https://your-perfana-url \
  --apiKey "$PERFANA_API_KEY" \
  --systemUnderTest "YourApp" \
  --environment "loadtest" \
  --workload "baseline"
```

### 3. Run the migrate command

The `migrate` command automatically converts your pom.xml:

```bash
perfana-cli migrate --input pom.xml --output perfana.yaml
```

This handles:
- Any plugin containing `eventSchedulerConfig` (event-scheduler-maven-plugin, events-jmeter-maven-plugin, events-gatling-maven-plugin, etc.)
- `CommandRunnerEventConfig` → `command` events
- `TestRunConfigCommandEventConfig` → `config-collector` events
- `PerfanaEventConfig` → skipped (handled natively by perfana-cli)
- Maven `${env.VAR}` references → shell `${VAR}` syntax
- Duration conversion (seconds → ISO 8601)
- Maven property resolution with TODO warnings for unresolvable references

#### Maven profiles → env files

Properties overridden by Maven profiles are converted to `${ENV_VAR}` references in the YAML. The migrate command generates `.env` files for each profile:

```
profiles/
  test-type-load.env
  test-type-stress.env
  test-type-endurance.env
```

Usage:
```bash
source profiles/test-type-stress.env && perfana-cli run start
```

Example generated `.env` file:
```bash
# Profile: test-type-stress
# Usage: source profiles/test-type-stress.env && perfana-cli run start

WORKLOAD=stressTest
JMETER_REPLICAS=1
JMETER_THREADS=250
RAMPUP_TIME_IN_SECONDS=PT2M
CONSTANT_LOAD_TIME_IN_SECONDS=PT10M
```

### 4. Convert duration format

Maven plugins use seconds; `perfana-cli` uses ISO 8601 durations:

| Maven (seconds) | perfana-cli (ISO 8601) |
|-----------------|----------------------|
| `120` | `PT2M` |
| `300` | `PT5M` |
| `900` | `PT15M` |
| `1800` | `PT30M` |
| `3600` | `PT1H` |

### 5. Update your CI/CD pipeline

**Before (Maven in CI):**
```bash
mvn event-scheduler:test -Dperfana.apiKey=$API_KEY
```

**After (perfana-cli in CI):**
```bash
perfana-cli run start \
  --analysisStartOffset=PT2M \
  --constantLoadTime=PT15M
```

Or use the GitHub Action:
```yaml
- uses: perfana/perfana-cli-action@v1
  with:
    command: "run start"
  env:
    PERFANA_API_KEY: ${{ secrets.PERFANA_API_KEY }}
```

### 6. Remove Maven plugin dependencies

Once you've verified `perfana-cli` works, remove from your `pom.xml`:

- `io.perfana:event-scheduler-maven-plugin`
- `io.perfana:perfana-java-client`
- `io.perfana:event-*` (event plugins)
- Any Perfana-specific Maven profiles

## Key differences

| Feature | Maven Plugin | perfana-cli |
|---------|-------------|-------------|
| Event plugins | Java classes loaded via classpath | Shell commands |
| Config format | XML (pom.xml) | YAML |
| Auth | API key in Maven settings | API key in YAML or env var |
| mTLS | Java keystore | PEM files |
| Scheduled events | Java event scheduler | Built-in Go scheduler |
| Duration format | Seconds (integer) | ISO 8601 (`PT5M`) |
| Profiles | Maven profiles (`-P`) | Env files (`source profiles/stress.env`) |
| Migration | Manual | `perfana-cli migrate` (auto-generates YAML + profiles) |

## Troubleshooting

**Q: My custom Java event plugins won't work with perfana-cli?**

Replace Java event plugins with shell commands. Any action your Java plugin performed can be expressed as a shell command in the `events[].commands` section.

**Q: How do I pass Maven properties?**

Use environment variables in your YAML: `${MY_VAR}`. Set them in your CI/CD pipeline.

**Q: My load test data doesn't show up in Perfana?**

The load generator must write metrics with the correct test run ID. Use the `__testRunId__` placeholder in your commands — perfana-cli substitutes it with the actual Perfana test run ID at runtime:

```yaml
events:
  - name: jmeter-load
    type: command
    continueOnKeepAliveParticipant: true
    commands:
      onStartTest: "jmeter -n -t test.jmx -Jtest.testRunId=__testRunId__"
      onKeepAlive: "pgrep -f jmeter || exit 1"
```

**Q: Where is the config file?**

Default: `~/.perfana-cli/perfana.yaml`. Override with `--config /path/to/perfana.yaml`.
