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
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Configuration struct to represent the YAML structure
type Configuration struct {
	Key              string `yaml:"key"`
	BaseUrl          string `yaml:"baseUrl"`
	ClientIdentifier string `yaml:"clientIdentifier"`
	SystemUnderTest  string `yaml:"systemUnderTest"`
	Environment      string `yaml:"environment"`
	Workload         string `yaml:"workload"`
	MTLS             struct {
		Cert string `yaml:"cert"`
		Key  string `yaml:"key"`
	} `yaml:"mtls"`
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration for Perfana",
	Long: `The 'init' command creates a ~/.perfana-cli directory with a 'perfana.yaml' 
  YAML-based configuration file containing setup data, including optional flags for customizing the file.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get the user's home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("Error finding home directory:", err)
			return
		}

		// Create the .perfana directory
		perfanaDir := filepath.Join(homeDir, ".perfana-cli")
		if err := os.MkdirAll(perfanaDir, 0755); err != nil {
			fmt.Println("Error creating .perfana-cli directory:", err)
			return
		}

		// Path for the configuration file
		configFile := filepath.Join(perfanaDir, "perfana.yaml")

		// Initialize default configuration
		config := Configuration{
			Key:              "your-api-key",
			BaseUrl:          "http://localhost:4000",
			ClientIdentifier: "your-client-identifier",
			SystemUnderTest:  "your-system-under-test",
			Environment:      "your-environment",
			Workload:         "your-workload",
		}
		config.MTLS.Key = "-----BEGIN PRIVATE KEY-----\n<your-private-key-here>\n-----END PRIVATE KEY-----"
		config.MTLS.Cert = "-----BEGIN CERTIFICATE-----\n<your-cert-here>\n-----END CERTIFICATE-----"

		// Read flags
		clientIdentifier, _ := cmd.Flags().GetString("clientIdentifier")
		baseUrl, _ := cmd.Flags().GetString("baseUrl")
		systemUnderTest, _ := cmd.Flags().GetString("systemUnderTest")
		environment, _ := cmd.Flags().GetString("environment")
		workload, _ := cmd.Flags().GetString("workload")
		certPath, _ := cmd.Flags().GetString("certPath")
		keyPath, _ := cmd.Flags().GetString("keyPath")

		// Update configuration values if flags are present
		if clientIdentifier != "" {
			config.ClientIdentifier = clientIdentifier
		}
		if baseUrl != "" {
			config.BaseUrl = baseUrl
		}
		if systemUnderTest != "" {
			config.SystemUnderTest = systemUnderTest
		}
		if environment != "" {
			config.Environment = environment
		}
		if workload != "" {
			config.Workload = workload
		}
		if certPath != "" {
			certData, err := os.ReadFile(certPath)
			if err != nil {
				fmt.Printf("Error reading certificate file %s: %s\n", certPath, err)
				return
			}
			config.MTLS.Cert = string(certData)
		}
		if keyPath != "" {
			keyData, err := os.ReadFile(keyPath)
			if err != nil {
				fmt.Printf("Error reading private key file %s: %s\n", keyPath, err)
				return
			}
			config.MTLS.Key = string(keyData)
		}

		// Marshal configuration into YAML format
		data, err := yaml.Marshal(&config)
		if err != nil {
			fmt.Println("Error generating YAML configuration:", err)
			return
		}

		// Write configuration to the file
		if err := os.WriteFile(configFile, data, 0644); err != nil {
			fmt.Println("Error writing perfana.yaml:", err)
			return
		}

		fmt.Printf("Configuration initialized successfully at: %s\n", configFile)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Add flags for optional customization
	initCmd.Flags().String("clientIdentifier", "", "Client identifier for Perfana configuration")
	initCmd.Flags().String("baseUrl", "", "Base URL to use for calling Perfana, e.g. http://localhost:4000")
	initCmd.Flags().String("systemUnderTest", "", "System under test for Perfana configuration")
	initCmd.Flags().String("environment", "", "Environment for Perfana configuration")
	initCmd.Flags().String("workload", "", "Workload for Perfana configuration")
	initCmd.Flags().String("certPath", "", "Path to PEM-encoded certificate file for mTLS")
	initCmd.Flags().String("keyPath", "", "Path to PEM-encoded private key file for mTLS")
}
