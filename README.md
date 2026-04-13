# perfana-cli

Command line interface for [Perfana](https://perfana.io) — orchestrate performance test runs, collect configs, and manage event lifecycles from the terminal.

Runs on macOS (arm/amd), Linux, and Windows with no external dependencies like Java or Maven.

## Installation

### Docker

```bash
docker run --rm perfana/perfana-cli version
```

### Binary download

Download the latest release from [GitHub Releases](https://github.com/perfana/perfana-cli/releases).

### Go install

```bash
go install github.com/perfana/perfana-cli@latest
```

## Quick start

Initialize a configuration file at `~/.perfana-cli/perfana.yaml`:

```bash
perfana-cli init \
  --baseUrl https://acme.perfana.cloud \
  --clientIdentifier acme \
  --apiKey "$PERFANA_API_KEY" \
  --systemUnderTest my-service \
  --environment test \
  --workload loadTest
```

For mTLS (Perfana Cloud), add certificate paths:

```bash
perfana-cli init \
  --baseUrl https://acme.perfana.cloud \
  --clientIdentifier acme \
  --clientKeyPath /path/to/tls.key \
  --clientCertPath /path/to/tls.crt \
  --apiKey "$PERFANA_API_KEY" \
  --systemUnderTest my-service \
  --environment test \
  --workload loadTest
```

Start a test run:

```bash
perfana-cli run start \
  --rampupTime=PT5M \
  --constantLoadTime=PT15M \
  --tags="k6,spring-boot" \
  --version="2.0.1" \
  --annotation="Nightly regression test"
```

The CLI orchestrates the full event lifecycle: BeforeTest → StartTest → KeepAlive → CheckResults → AfterTest. It keeps running until the configured duration completes or you stop it with `ctrl-C`.

## Commands

| Command | Description |
|---------|-------------|
| `init` | Create `~/.perfana-cli/perfana.yaml` with connection and test defaults |
| `init-project` | Generate an annotated `perfana.yaml` template in the current directory |
| `validate` | Validate a `perfana.yaml` file (syntax, required fields, durations, event schemas) |
| `run start` | Start a test run with full event lifecycle orchestration |
| `migrate` | Convert a Maven pom.xml (event-scheduler-maven-plugin) to `perfana.yaml` |
| `version` | Print version, commit hash, and build date |

## Configuration

Configuration is a YAML file (default: `~/.perfana-cli/perfana.yaml`, override with `--config`). Environment variables are expanded automatically.

```yaml
perfana:
  apiKey: "${PERFANA_API_KEY}"
  baseUrl: https://acme.perfana.cloud
  clientIdentifier: acme
  mtls:
    enabled: true
    clientCert: /path/to/tls.crt
    clientKey: /path/to/tls.key

test:
  systemUnderTest: my-service
  environment: test
  workload: loadTest
  version: "2.0.1"
  rampupTime: PT5M
  constantLoadTime: PT15M
  tags:
    - k6
    - spring-boot
  deepLinks:
    - name: CI Build
      url: https://ci.example.com/builds/123
  variables:
    - placeholder: service
      value: my-service

scheduler:
  enabled: true
  keepAliveIntervalSeconds: 30
  failOnError: true

events:
  - name: collect-config
    type: config-collector
    command: kubectl get configmap my-config -o json
    output: json
    key: k8s-config

  - name: restart-pods
    type: command
    commands:
      onBeforeTest: kubectl rollout restart deployment/my-service
      onAfterTest: echo "done"
```

Generate a fully annotated template with `perfana-cli init-project`.

## Migrating from Maven

If you have an existing `pom.xml` with `event-scheduler-maven-plugin` configuration:

```bash
perfana-cli migrate --input pom.xml --output perfana.yaml
```

This converts the Maven plugin config (event configs, property references, durations) to `perfana.yaml` format.

## Use cases

- Run alongside a load test to send Perfana events (start, stop, keep-alive)
- Schedule daily short runs on production to detect regressions over time
- Quick way to try out a Perfana setup without Maven or Java

## What it is not

- A replacement for `x2i` (used to send load data metrics to Perfana)
- A substitute for a production-grade `perfana-secure-gateway`
- A load generator

## License

Apache License 2.0
