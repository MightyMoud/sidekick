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
package previewList

import (
	"log"
	"os"

	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var ListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "This command lists all the preview environments",
	Long:    `This command lists all the preview environments that are currently running on your VPS.`,
	Run: func(cmd *cobra.Command, args []string) {
		pterm.Println()
		appConfig, appConfigErr := utils.LoadAppConfig()
		if appConfigErr != nil {
			log.Fatalln("Unable to load your config file. Might be corrupted")
			os.Exit(1)
		}
		tableData := pterm.TableData{
			{"Commit", "Image", "Deployed at", "URL"},
		}
		for v := range appConfig.PreviewEnvs {
			tableData = append(tableData, []string{v, appConfig.PreviewEnvs[v].Image, appConfig.PreviewEnvs[v].CreatedAt, appConfig.PreviewEnvs[v].Url})
		}

		pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()

		pterm.Println()
	},
}
