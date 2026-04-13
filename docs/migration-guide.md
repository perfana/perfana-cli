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
| `perfanaUrl` | `perfana.baseUrl` |
| `apiKey` | `perfana.apiKey` |

## Step-by-step migration

### 1. Install perfana-cli

```bash
brew install perfana/tap/perfana-cli
```

### 2. Initialize configuration

```bash
perfana-cli init \
  --baseUrl https://your-perfana-url \
  --apiKey "$PERFANA_API_KEY" \
  --systemUnderTest "YourApp" \
  --environment "loadtest" \
  --workload "baseline"
```

### 3. Convert your Maven configuration

**Before (pom.xml):**
```xml
<plugin>
  <groupId>io.perfana</groupId>
  <artifactId>event-scheduler-maven-plugin</artifactId>
  <configuration>
    <eventSchedulerConfig>
      <debugEnabled>true</debugEnabled>
      <schedulerEnabled>true</schedulerEnabled>
      <failOnError>true</failOnError>
      <continueOnEventCheckFailure>false</continueOnEventCheckFailure>
      <testConfig>
        <systemUnderTest>MyApp</systemUnderTest>
        <version>${app.version}</version>
        <workload>peak-load</workload>
        <testEnvironment>acceptance</testEnvironment>
        <rampupTimeInSeconds>120</rampupTimeInSeconds>
        <constantLoadTimeInSeconds>900</constantLoadTimeInSeconds>
        <tags>
          <tag>k6</tag>
          <tag>nightly</tag>
        </tags>
      </testConfig>
      <perfanaConfig>
        <perfanaUrl>https://perfana.example.com</perfanaUrl>
        <apiKey>${perfana.apiKey}</apiKey>
      </perfanaConfig>
      <eventConfigs>
        <eventConfig>
          <name>load-runner</name>
          <eventFactory>io.perfana.events.commandrunner.CommandRunnerEventFactory</eventFactory>
          <customEvents>
            <onStartTest>k6 run /tests/load.js</onStartTest>
            <onAfterTest>echo done</onAfterTest>
          </customEvents>
        </eventConfig>
      </eventConfigs>
    </eventSchedulerConfig>
  </configuration>
</plugin>
```

**After (perfana.yaml):**
```yaml
perfana:
  apiKey: "${PERFANA_API_KEY}"
  baseUrl: "https://perfana.example.com"

test:
  systemUnderTest: "MyApp"
  environment: "acceptance"
  workload: "peak-load"
  version: "1.2.0"
  rampupTime: "PT2M"
  constantLoadTime: "PT15M"
  tags:
    - "k6"
    - "nightly"

scheduler:
  enabled: true
  failOnError: true

events:
  - name: "load-runner"
    type: command
    commands:
      onStartTest: "k6 run /tests/load.js"
      onAfterTest: "echo done"
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
  --rampupTime=PT2M \
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

## Troubleshooting

**Q: My custom Java event plugins won't work with perfana-cli?**

Replace Java event plugins with shell commands. Any action your Java plugin performed can be expressed as a shell command in the `events[].commands` section.

**Q: How do I pass Maven properties?**

Use environment variables in your YAML: `${MY_VAR}`. Set them in your CI/CD pipeline.

**Q: Where is the config file?**

Default: `~/.perfana-cli/perfana.yaml`. Override with `--config /path/to/perfana.yaml`.
