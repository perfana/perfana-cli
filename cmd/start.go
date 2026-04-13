/*
Copyright © 2024 Peter Paul Bakker <peterpaul@perfana.io>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"perfana-cli/perfana_client"
	"perfana-cli/scheduler"
	"perfana-cli/util"
)

// Define command-line flags with default values
var (
	rampupTime       string
	constantLoadTime string
	tags             string
	annotation       string
	testVersion      string
	buildResultsUrl  string
	variablesFlag    []string
	deepLinksFlag    []string
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a Perfana run",
	Long: `The 'run start' command starts a Perfana test run with full event lifecycle
orchestration. It runs BeforeTest → StartTest → KeepAlive loop → CheckResults → AfterTest.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Load the configuration file
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("Error finding home directory:", err)
			return
		}

		configPath := filepath.Join(homeDir, ".perfana-cli", "perfana.yaml")
		file, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("Error reading configuration file: %v\n", err)
			return
		}

		// Parse the YAML configuration
		var config perfana_client.Configuration
		err = yaml.Unmarshal(file, &config)
		if err != nil {
			fmt.Printf("Error parsing configuration file: %v\n", err)
			return
		}

		// Parse Variables
		variables := make(map[string]string)
		for _, v := range variablesFlag {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("Invalid variable format: '%s'\n", v)
				continue
			}
			variables[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}

		// Parse durations
		rampupSec, err := util.ParseISODurationToSeconds(rampupTime)
		if err != nil {
			fmt.Printf("Error parsing rampupTime: %v\n", err)
			return
		}

		constantLoadSec, err := util.ParseISODurationToSeconds(constantLoadTime)
		if err != nil {
			fmt.Printf("Error parsing constantLoadTime: %v\n", err)
			return
		}

		totalDurationSec := rampupSec + constantLoadSec
		log.Printf("Starting Perfana run for %d seconds (rampup: %ds, constant: %ds)",
			totalDurationSec, rampupSec, constantLoadSec)

		// Initialize the Perfana client
		client, err := perfana_client.NewClient(config)
		if err != nil {
			fmt.Printf("Error initializing Perfana client: %v\n", err)
			return
		}

		// Build test context
		tagList := strings.Split(tags, ",")
		testCtx := scheduler.TestContext{
			SystemUnderTest: config.SystemUnderTest,
			Environment:     config.Environment,
			Workload:        config.Workload,
			Version:         testVersion,
			Tags:            tagList,
			Variables:       variables,
			Client:          client,
		}

		// Create the event scheduler
		eventScheduler := &scheduler.EventScheduler{
			Client:               client,
			Events:               []scheduler.Event{}, // events will be added in Phase 2
			ScheduleEntries:      []scheduler.ScheduleEntry{},
			KeepAliveIntervalSec: 30,
			TestDurationSec:      totalDurationSec,
			TestContext:          testCtx,
			FailOnError:          true,
		}

		// Run the full lifecycle
		if err := eventScheduler.Run(); err != nil {
			fmt.Printf("Test run failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	runCmd.AddCommand(startCmd)

	startCmd.Flags().StringVar(&rampupTime, "rampupTime", "PT5M", "Ramp-up time in ISO8601 format (e.g., PT5M for 5 minutes)")
	startCmd.Flags().StringVar(&constantLoadTime, "constantLoadTime", "PT15M", "Constant load time in ISO8601 format (e.g., PT15M for 15 minutes)")
	startCmd.Flags().StringVar(&tags, "tags", "k6,jfr", "Comma-separated tags for the test session")
	startCmd.Flags().StringVar(&annotation, "annotation", "", "Annotation message for the test session")
	startCmd.Flags().StringVar(&testVersion, "version", "1.0.0", "Version of the test session")
	startCmd.Flags().StringVar(&buildResultsUrl, "buildResultsUrl", "", "URL to CI build results")
	startCmd.Flags().StringSliceVar(&variablesFlag, "variable", []string{}, "Set variables (name=value). Example: --variable key1=value1 --variable key2=value2")
	startCmd.Flags().StringSliceVar(&deepLinksFlag, "deeplink", []string{}, "Add deep links (title|url). Example: --deeplink MyTitle|http://example.com")
}
