# perfana-cli

The command line interface to Perfana.

## Goal

Use `perfana` command to:
* initialize config to connect to Perfana Cloud or Perfana local
* start and stop a load test, will send keep-alive until a defined end of run

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

    perfana init

If no `~/.perfana` directory exists, it will be created with `perfana.conf` file that contains
placeholders for key and mTLS certificates. Also, defaults for `systemUnderTest`, `environment` and `workload`.
These will be used in the commands with no explicit overrides on the command line.

For Perfana Cloud, fill the `clientIdentifier` that is used in the `<clientIdentifier>.perfana.cloud` domain name, among others.

To start a run of 30 minutes use:

    perfana run start --run-duration=PT30m

This will start a test run and send keep-alives for 30 minutes. It will keep running until the 
run duration has passed or when you kill the command (e.g. using `ctrl-C`).

During the run, metrics and other data sent to Perfana is actually received and stored. See the
live data in the Perfana dashboards. After the run the automatic analysis in Perfana is started.

To stop an existing run use:

    perfana run stop

This command is needed only to reset a current run that does not stop automatically.
   
In the `~/.perfana` directory the current run information is stored in `perfana-run.state`.

## What it is not

This command line tool is not: 
* a replacement for `x2i` that is used to send load data metrics to Perfana
* a substitute for a production grade `perfana-secure-gateway`
* a load generator
* a stand alone tool