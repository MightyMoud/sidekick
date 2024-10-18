/*
Copyright © 2024 Mahmoud Mousa <m.mousa@hey.com>

Licensed under the GNU GPL License, Version 3.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.gnu.org/licenses/gpl-3.0.en.html

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"os"

	"github.com/mightymoud/sidekick/cmd/deploy"
	"github.com/mightymoud/sidekick/cmd/launch"
	"github.com/mightymoud/sidekick/cmd/preview"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "sidekick",
	Version: version,
	Short:   "CLI to self-host all your apps on a single VPS without vendor locking",
	Long:    `With sidekick you can deploy any number of applications to a single VPS, connect multiple domains and much more.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate(`{{println .Version}}`)
	rootCmd.AddCommand(preview.PreviewCmd)
	rootCmd.AddCommand(deploy.DeployCmd)
	rootCmd.AddCommand(launch.LaunchCmd)
}
