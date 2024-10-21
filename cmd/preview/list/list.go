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
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var ListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "This command lists all the preview environments",
	Long:    `This command lists all the preview environments that are currently running on your VPS.`,
	Run: func(cmd *cobra.Command, args []string) {
		appConfig, appConfigErr := utils.LoadAppConfig()
		if appConfigErr != nil {
			log.Fatalf("Unable to load your config file. Might be corrupted")
		}
		if len(appConfig.PreviewEnvs) == 0 {
			render.GetLogger(log.Options{Prefix: "Preview Envs"}).Info("Not Found in current project")
			os.Exit(0)
		}
		header := lipgloss.NewStyle().Foreground(lipgloss.Color("77")).MarginTop(1).MarginLeft(1).Render("Currently running preview envs:")
		tableString := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("99"))).
			StyleFunc(func(row, col int) lipgloss.Style {
				switch {
				case row == 0:
					return lipgloss.NewStyle().Foreground(lipgloss.Color("60")).Align(lipgloss.Center)
				default:
					return lipgloss.NewStyle().Foreground(lipgloss.Color("78")).PaddingLeft(1).PaddingRight(1)
				}
			}).
			Headers("Commit", "Image", "Deployed At", "URL")

		hashSlice := []huh.Option[string]{}
		for v := range appConfig.PreviewEnvs {
			hashSlice = append(hashSlice, huh.NewOption(v, v))
			tableString.Row(v, appConfig.PreviewEnvs[v].Image, appConfig.PreviewEnvs[v].CreatedAt, appConfig.PreviewEnvs[v].Url)
		}
		fmt.Println(header)
		fmt.Println(tableString)
	},
}
