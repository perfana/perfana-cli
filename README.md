# perfana-cli

The command line interface to Perfana. 

This is a beta release.

## Goal

Use `perfana-cli` command to:
* initialize config to connect to Perfana Cloud or Perfana local
* start and stop a load test, send keep-alive until a defined end of run

Possible extensions:
* send run config to Perfana
* run scheduled commands
* coordinate distributed load test
* generate needed configurations (perfana-secure-gateway, otel-collector, load test script extensions)
* check and troubleshoot configurations
* act as simple psg for testing

## Features

* runs on many targets: MacOS (arm/amd), Linux, Windows
* easy to download, configure and use
* no need to install other tools like Java and Maven to use Perfana

## Use cases

* run next to load test and send Perfana events (start, stop, etc...)
* scheduled to run for 20 minutes every day on production system to compare and find changes over time
* simple way to test or try-out Perfana setup

## How it works

To access Perfana you need a key and for Perfana Cloud you need mTLS certificates. To initialize:

    perfana-cli init

If no `~/.perfana-cli` directory exists, it will be created with `perfana.yaml` file that contains
placeholders for key and mTLS certificates. Also, defaults for `systemUnderTest`, `environment` and `workload`.
These will be used in the commands with no explicit overrides on the command line.

For Perfana Cloud, fill the `clientIdentifier` that is used in the `<clientIdentifier>.perfana.cloud` domain name, among others.

To start a run of 30 minutes use:

    perfana-cli run start \
      --rampupTime=PT10m \
      --constantLoadTime=PT20m \
      --tags="k6,jfr" \
      --annotation="Running custom test" \
      --version="2.0.1" \ 
      --buildResultsUrl="http://example.com/results"

This will start a test run and send keep-alives for 30 minutes. It will keep running until the 
run duration has passed or when you kill the command (e.g. using `ctrl-C`).

During the run, metrics and other data sent to Perfana is actually received and stored. See the
live data in the Perfana dashboards. After the run the automatic analysis in Perfana is started.

To stop the run use `ctrl-c` or kill the process.

# Example calls

Init with data:

    perfana-cli init \
      --baseUrl http://acme.perfana.cloud \
      --clientIdentifier acme \
      --clientKeyPath /Users/alice/keys/tls.key \
      --clientCertPath /Users/alice/keys/tls.crt \
      --systemUnderTest cli-demo \
      --environment local \
      --workload loadTest \
      --apiKey "$PERFANA_API_KEY"

Send deep links or variables:

    perfana-cli run start \
      --rampupTime=PT1m \
      --constantLoadTime=PT2m \
      --tags="k6,jfr,spring-boot-kubernetes" \
      --annotation="Short running custom test" \
      --version="2.0.1" \
      --buildResultsUrl="http://github.com/cli-demo/results/$GHA_BUILD_ID" \
      --deeplink "Test Link|https://www.perfana.io" \
      --deeplink "Test Link 2|https://my.loki.local/page/$PAGE_ID" \
      --variable "service=tiny-bank-service" \
      --variable "region=eu"

# Todo

* in the `~/.perfana-cli` directory the current run information is stored in `perfana-run.state`.
* stop command

## What it is not

This command line tool is not: 
* a replacement for `x2i` that is used to send load data metrics to Perfana
* a substitute for a production grade `perfana-secure-gateway`
* a load generator
* a stand alone tool