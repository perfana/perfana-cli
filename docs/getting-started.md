# Getting Started with perfana-cli

This guide walks you through installing `perfana-cli`, connecting it to your Perfana instance, and running your first performance test session.

## Prerequisites

- A running Perfana instance (Cloud or self-hosted)
- A Perfana API key
- For Perfana Cloud: mTLS client certificate and key

## Installation

### Homebrew (macOS / Linux)

```bash
brew install perfana/tap/perfana-cli
```

### Docker

```bash
docker pull perfana/perfana-cli:latest
```

### Download binary

Download the latest release for your platform from [GitHub Releases](https://github.com/perfana/perfana-cli/releases).

Extract and add to your PATH:

```bash
# Linux / macOS
tar xzf perfana-cli_*_linux_amd64.tar.gz
sudo mv perfana-cli /usr/local/bin/

# Windows (PowerShell)
scoop bucket add perfana https://github.com/perfana/scoop-bucket
scoop install perfana-cli
```

### Verify installation

```bash
perfana-cli version
```

## Step 1: Initialize configuration

Run the init command to create `~/.perfana-cli/perfana.yaml`:

```bash
perfana-cli init \
  --baseUrl https://your-perfana.example.com \
  --apiKey "$PERFANA_API_KEY" \
  --systemUnderTest "MyApp" \
  --environment "loadtest" \
  --workload "baseline"
```

For Perfana Cloud with mTLS:

```bash
perfana-cli init \
  --baseUrl https://acme.perfana.cloud \
  --clientIdentifier acme \
  --clientCertPath /path/to/tls.crt \
  --clientKeyPath /path/to/tls.key \
  --apiKey "$PERFANA_API_KEY" \
  --systemUnderTest "MyApp" \
  --environment "loadtest" \
  --workload "baseline"
```

## Step 2: Run a test session

Start a test session with a 5-minute rampup and 15-minute constant load:

```bash
perfana-cli run start \
  --analysisStartOffset=PT5M \
  --constantLoadTime=PT15M \
  --version="1.2.0" \
  --tags="k6,nightly" \
  --annotation="Nightly baseline test"
```

The CLI will:
1. Initialize a test session with Perfana (`POST /init`)
2. Send keep-alive heartbeats every 30 seconds
3. Run for the total duration (rampup + constant load)
4. Complete the session and trigger analysis

To stop early, press `Ctrl+C`.

## Step 3: Add metadata

Enrich your test session with deep links and variables:

```bash
perfana-cli run start \
  --analysisStartOffset=PT2M \
  --constantLoadTime=PT10M \
  --version="1.2.0" \
  --tags="k6,spring-boot" \
  --deeplink "Grafana|https://grafana.example.com/d/abc" \
  --deeplink "CI Build|https://github.com/myorg/myapp/actions/runs/123" \
  --variable "region=eu-west-1" \
  --variable "replicas=3"
```

## Step 4: Use in CI/CD

See the [CI/CD examples](../examples/ci/) for GitHub Actions, GitLab CI, and Jenkins integration.

Quick GitHub Actions example using the Perfana action:

```yaml
- name: Run performance test
  uses: perfana/perfana-cli-action@v1
  with:
    command: "run start"
    config: "perfana.yaml"
  env:
    PERFANA_API_KEY: ${{ secrets.PERFANA_API_KEY }}
```

## Next steps

- [YAML Configuration Reference](configuration-reference.md) - all configuration options
- [Command Reference](command-reference.md) - all CLI commands and flags
- [Migration Guide](migration-guide.md) - migrating from Maven plugins
