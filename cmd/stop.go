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

	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a Perfana run",
	Long:  "The 'run stop' command stops a currently running Perfana test.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Stopping the Perfana run...")
		// Add logic here to stop a running test (currently stubbed)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
