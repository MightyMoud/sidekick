/*
Copyright © 2024 Mahmoud Mosua <m.mousa@hey.com>

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
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if configErr := utils.ViperInit(); configErr != nil {
			pterm.Error.Println("Sidekick config not found - Run sidekick init")
			os.Exit(1)
		}
		if !utils.FileExists("./sidekick.yml") {
			pterm.Error.Println(`Sidekick config not found in current directory 
Run sidekick launch`)
			os.Exit(1)
		}
		pterm.Println()
		pterm.DefaultHeader.WithFullWidth().Println("Deploying a new version of your app 😏")
		pterm.Println()

		multi := pterm.DefaultMultiPrinter
		loginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging into VPS")
		dockerBuildStageSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Building latest docker image of your app")
		deployStageSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Deploying a new version of your application")

		multi.Start()

		appConfig, loadError := utils.LoadAppConfig()
		if loadError != nil {
			panic(loadError)
		}
		replacer := strings.NewReplacer(
			"$service_name", appConfig.Name,
			"$app_port", fmt.Sprint(appConfig.Port),
		)

		loginSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		sshClient, err := utils.Login(viper.Get("serverAddress").(string), "sidekick")
		if err != nil {
			loginSpinner.Fail("Something went wrong logging in to your VPS")
			panic(err)
		}
		loginSpinner.Success("Logged in successfully!")

		dockerBuildStageSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		envFileChanged := false
		currentEnvFileHash := ""
		if appConfig.Env.File != "" {
			envFileContent, envFileErr := os.ReadFile(fmt.Sprintf("./%s", appConfig.Env.File))
			if envFileErr != nil {
				pterm.Error.Println("Unable to process your env file")
				os.Exit(1)
			}
			currentEnvFileHash = fmt.Sprintf("%x", md5.Sum(envFileContent))
			envFileChanged = appConfig.Env.Hash != currentEnvFileHash
			if envFileChanged {
				// encrypt new env file
				envCmd := exec.Command("sh", "-s", "-", viper.Get("publicKey").(string), fmt.Sprintf("./%s", appConfig.Env.File))
				envCmd.Stdin = strings.NewReader(utils.EnvEncryptionScript)
				if envCmdErr := envCmd.Run(); envCmdErr != nil {
					panic(envCmdErr)
				}
				encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appConfig.Name)))
				encryptSync.Run()
			}
		}
		defer os.Remove("encrypted.env")

		cwd, _ := os.Getwd()
		dockerBuildCommd := exec.Command("sh", "-s", "-", appConfig.Name, cwd)
		dockerBuildCommd.Stdin = strings.NewReader(utils.DockerBuildAndSaveScript)
		if dockerBuildErr := dockerBuildCommd.Run(); dockerBuildErr != nil {
			panic(dockerBuildErr)
		}
		dockerBuildStageSpinner.Success("Latest docker image built")

		deployStageSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		imgMoveCmd := exec.Command("sh", "-s", "-", appConfig.Name, "sidekick", viper.GetString("serverAddress"))
		imgMoveCmd.Stdin = strings.NewReader(utils.ImageMoveScript)
		_, imgMoveErr := imgMoveCmd.Output()
		if imgMoveErr != nil {
			log.Fatalf("Issue occured with moving image to your VPS: %s", imgMoveErr)
			os.Exit(1)
		}
		if _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s-latest.tar", appConfig.Name, appConfig.Name)); sessionErr != nil {
			log.Fatal("Issue happened loading docker image")
		}

		if appConfig.Env.File != "" {
			deployScript := replacer.Replace(utils.DeployAppWithEnvScript)
			_, sessionErr := utils.RunCommand(sshClient, deployScript)
			if sessionErr != nil {
				panic(sessionErr)
			}
		} else {
			deployScript := replacer.Replace(utils.DeployAppScript)
			_, sessionErr := utils.RunCommand(sshClient, deployScript)
			if sessionErr != nil {
				panic(sessionErr)
			}
		}
		if _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && rm %s", appConfig.Name, fmt.Sprintf("%s-latest.tar", appConfig.Name))); sessionErr != nil {
			log.Fatal("Issue happened cleaning up the image file")
		}

		latestVersion := strings.Split(appConfig.Version, "")[1]
		latestVersionInt, _ := strconv.ParseInt(latestVersion, 0, 64)
		appConfig.Version = fmt.Sprintf("V%d", latestVersionInt+1)
		// env file changed ? -> update hash
		if envFileChanged {
			appConfig.Env.Hash = currentEnvFileHash
		}
		ymlData, err := yaml.Marshal(&appConfig)
		os.WriteFile("./sidekick.yml", ymlData, 0644)
		deployStageSpinner.Success("🙌 Deployed new version successfully 🙌")
		multi.Stop()

		pterm.Println()
		pterm.Info.Printfln("😎 View your app at: https://%s", appConfig.Url)

		pterm.Println()
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deployCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
