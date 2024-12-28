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
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"perfana-cli/perfana_client"
	"perfana-cli/util"
)

// Define command-line flags with default values
var rampupTime string
var constantLoadTime string
var tags string
var annotation string
var version string
var buildResultsUrl string

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a Perfana run",
	Long: `The 'run start' command starts a Perfana test run. You can optionally
  specify the run duration with the '--run-duration' flag.`,
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

		rampupTimeMinutes, err := util.ParseISODuration(rampupTime)
		if err != nil {
			fmt.Printf("Error parsing rampupTime: %v\n", err)
		}

		constantLoadTimeMinutes, err := util.ParseISODuration(constantLoadTime)
		if err != nil {
			fmt.Printf("Error parsing constantLoadTime: %v\n", err)
		}

		runDuration := rampupTimeMinutes + constantLoadTimeMinutes

		fmt.Printf("Starting the Perfana run for %d minutes...\n", runDuration)

		// Initialize the Perfana client
		client, err := perfana_client.NewClient(config)
		if err != nil {
			fmt.Printf("Error initializing Perfana client: %v\n", err)
			return
		}

		// Call Init to get the testRunId
		testRunID, err := client.Init()
		if err != nil {
			fmt.Printf("Error during Init call: %v\n", err)
			return
		}

		fmt.Printf("Test run initialized successfully! TestRunID: %s\n", testRunID)

		// Start the session
		additionalData := map[string]interface{}{
			"version":           version,
			"cibuildResultsUrl": buildResultsUrl,
			"rampUp":            fmt.Sprintf("%d", rampupTimeMinutes*60),
			"duration":          fmt.Sprintf("%d", constantLoadTimeMinutes*60),
			"annotations":       annotation,
			"tags":              strings.Split(tags, ","),
		}

		// Start a Perfana session
		err = client.TestEvent(testRunID, additionalData, false)
		if err != nil {
			fmt.Printf("Error starting session: %v\n", err)
			return
		}

		fmt.Printf("Session started successfully! testRunId: %s\n", testRunID)

		runMinutes := rampupTimeMinutes + constantLoadTimeMinutes
		// Define the duration of the session (in seconds)
		sessionDuration := time.Duration(runMinutes) * time.Minute
		testTimeout := time.After(sessionDuration) // Creates a channel that signals after testDuration

		// Start keep alive in a goroutine
		keepAliveTicker := time.NewTicker(30 * time.Second) // Adjust keep alive interval as needed
		stopChan := make(chan struct{})

		go func() {
			for {
				select {
				case <-keepAliveTicker.C:
					err := client.TestEvent(testRunID, additionalData, false)
					if err != nil {
						fmt.Printf("Error sending abort event: %v\n", err)
					} else {
						fmt.Println("Keep alive sent successfully")
					}
				case <-stopChan:
					keepAliveTicker.Stop()
					return
				}
			}
		}()

		// Handle CTRL+C (manual interruption)
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

		// Wait for either test completion or CTRL+C
		select {
		case <-testTimeout: // Test duration passed
			close(stopChan) // Stop keep alive

			err := client.TestEvent(testRunID, additionalData, true)
			if err != nil {
				fmt.Printf("Error sending completion event: %v\n", err)
			}

			fmt.Println("Test duration completed. Exiting gracefully...")

		case <-signalChan: // Interrupted by CTRL+C
			close(stopChan) // Stop keep alive

			abortEvent := perfana_client.PerfanaEvent{
				SystemUnderTest: config.SystemUnderTest,
				TestEnvironment: config.Environment,
				Title:           "Test aborted",
				Description:     "Manually aborted",
				Tags:            []string{"aborted", "manual"},
			}

			response, err := client.SendPerfanaEvent(abortEvent)
			if err != nil {
				fmt.Printf("Error sending abort event: %v\n", err)
				if response != "" {
					fmt.Printf("Server response: %s\n", response)
				}
			} else {
				fmt.Println("Abort event sent successfully!")
			}
		}

		// Final message
		fmt.Println("Finished...")

	},
}

func init() {
	runCmd.AddCommand(startCmd)

	// Add flags to the startCmd
	startCmd.Flags().StringVar(&rampupTime, "rampupTime", "PT5m", "Ramp-up time period in ISO8601 format (e.g., PT5m for 5 minutes)")
	startCmd.Flags().StringVar(&constantLoadTime, "constantLoadTime", "PT15m", "Constant load time period in ISO8601 format (e.g., PT15m for 15 minutes)")
	startCmd.Flags().StringVar(&tags, "tags", "k6,jfr", "Comma-separated tags for the test session")
	startCmd.Flags().StringVar(&annotation, "annotation", "", "Annotation message for the test session")
	startCmd.Flags().StringVar(&version, "version", "1.0.0", "Version of the test session")
	startCmd.Flags().StringVar(&buildResultsUrl, "buildResultsUrl", "", "URL to CI build results")

}
