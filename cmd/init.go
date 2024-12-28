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
	"perfana-cli/perfana_client"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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
		config := perfana_client.Configuration{
			ApiKey:           "your-api-key",
			BaseUrl:          "http://localhost:4000",
			ClientIdentifier: "your-client-identifier",
			SystemUnderTest:  "your-system-under-test",
			Environment:      "your-environment",
			Workload:         "your-workload",
		}
		config.MTLS.ClientKey = "-----BEGIN PRIVATE KEY-----\n<your-private-key-here, mind indentation>\n-----END PRIVATE KEY-----"
		config.MTLS.ClientCert = "-----BEGIN CERTIFICATE-----\n<your-cert-here, mind indentation>\n-----END CERTIFICATE-----"

		// Read flags
		clientIdentifier, _ := cmd.Flags().GetString("clientIdentifier")
		baseUrl, _ := cmd.Flags().GetString("baseUrl")
		systemUnderTest, _ := cmd.Flags().GetString("systemUnderTest")
		environment, _ := cmd.Flags().GetString("environment")
		workload, _ := cmd.Flags().GetString("workload")
		clientCertPath, _ := cmd.Flags().GetString("clientCertPath")
		clientKeyPath, _ := cmd.Flags().GetString("clientKeyPath")
		apiKey, _ := cmd.Flags().GetString("apiKey")

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
		if apiKey != "" {
			config.ApiKey = apiKey
		}
		// only enable when certs are present
		certPresent := false
		keyPresent := false
		if clientCertPath != "" {
			certData, err := os.ReadFile(clientCertPath)
			if err != nil {
				fmt.Printf("Error reading certificate file %s: %s\n", clientCertPath, err)
				return
			}
			config.MTLS.ClientCert = string(certData)
			certPresent = true
		}
		if clientKeyPath != "" {
			keyData, err := os.ReadFile(clientKeyPath)
			if err != nil {
				fmt.Printf("Error reading private key file %s: %s\n", clientKeyPath, err)
				return
			}
			config.MTLS.ClientKey = string(keyData)
			keyPresent = true
		}
		if (certPresent && !keyPresent) || (!certPresent && keyPresent) {
			fmt.Println("Both client certificate and private key must be provided for mTLS")
			return
		}
		fmt.Printf("mTLS enabled: %t\n", certPresent && keyPresent)
		config.MTLS.Enabled = certPresent && keyPresent

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
	initCmd.Flags().String("apiKey", "", "Perfana API key")
	initCmd.Flags().String("systemUnderTest", "", "System under test for Perfana configuration")
	initCmd.Flags().String("environment", "", "Environment for Perfana configuration")
	initCmd.Flags().String("workload", "", "Workload for Perfana configuration")
	initCmd.Flags().String("clientCertPath", "", "Path to PEM-encoded certificate file for mTLS")
	initCmd.Flags().String("clientKeyPath", "", "Path to PEM-encoded private key file for mTLS")
}
