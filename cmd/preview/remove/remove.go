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
package previewRemove

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var RemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "This command removes a preview environment",
	Long:    "This command removes a preview environment by the git hash associated with them",
	Run: func(cmd *cobra.Command, args []string) {
		if configErr := utils.ViperInit(); configErr != nil {
			render.GetLogger(log.Options{Prefix: "Sidekick Config"}).Fatal("Not found - Run Sidekick init first")
		}
		if !utils.FileExists("./sidekick.yml") {
			render.GetLogger(log.Options{Prefix: "Project Config"}).Fatal("Not found in current directory Run sidekick launch")
		}

		appConfig, appConfigErr := utils.LoadAppConfig()
		if appConfigErr != nil {
			log.Fatalf("Unable to load your config file. Might be corrupted")
		}

		var selected string
		var confirm bool

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
		huh.NewSelect[string]().
			Title("Which preview env would you like to delete?").
			Options(hashSlice...).
			Value(&selected).
			Run()
		huh.NewConfirm().
			Title("Are you sure?").
			Affirmative("Yes!").
			Negative("No.").
			Value(&confirm).
			Run()
		if !confirm {
			os.Exit(0)
		} else {
			action := func() {
				deletePreviewEnv(selected)
			}
			spinner.New().
				Title("Deleting your selected preview environment...").
				Action(action).
				Run()

			fmt.Println("Preview env deleted successfully!")
		}

	},
}

func deletePreviewEnv(hash string) {

	appConfig, appConfigErr := utils.LoadAppConfig()
	if appConfigErr != nil {
		log.Fatalf("Unable to load your config file. Might be corrupted")
	}
	sshClient, err := utils.Login(viper.GetString("serverAddress"), "sidekick", viper.GetString("sshProvider"))
	if err != nil {
		log.Fatal("Unable to login to your VPS")
	}

	_, _, dockerDwnErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s/preview/%s && docker rm -f sidekick-%s-%s-1 && docker image rm %s:%s", appConfig.Name, hash, appConfig.Name, hash, appConfig.Name, hash))
	if dockerDwnErr != nil {
		log.Fatalf("Issue happened stopping your service: %s", dockerDwnErr)
	}
	_, _, folderRmErr := utils.RunCommand(sshClient, fmt.Sprintf("rm -rf %s/preview/%s", appConfig.Name, hash))
	if folderRmErr != nil {
		log.Fatalf("Issue happened deleting the preview folder: %s", folderRmErr)
	}

	delete(appConfig.PreviewEnvs, hash)
	ymlData, _ := yaml.Marshal(&appConfig)
	os.WriteFile("./sidekick.yml", ymlData, 0644)
}
