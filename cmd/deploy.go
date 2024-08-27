/*
Copyright © 2024 Mahmoud Mosua <m.mousa@hey.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ms-mousa/sidekick/utils"
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

		multi := pterm.DefaultMultiPrinter
		setupProgressBar, _ := pterm.DefaultProgressbar.WithTotal(3).WithWriter(multi.NewWriter()).Start("Sidekick Booting up (2m estimated)  ")
		loginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging into VPS")
		dockerBuildStageSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Building latest docker image of your app")
		// check env hash if it has changed or not -> if it did, re-encrypt the file, send it over and then move on
		deployStageSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Deploying a new version of your application")

		multi.Start()

		appConfigFile, loadError := utils.LoadAppConfig()
		if loadError != nil {
			panic(loadError)
		}
		replacer := strings.NewReplacer(
			"$service_name", appConfigFile.App.Name,
			"$app_port", fmt.Sprint(appConfigFile.App.Port),
			"$docker_username", viper.Get("dockerUsername").(string),
		)

		sshClient, err := utils.LoginStage(viper.Get("serverAddress").(string), loginSpinner, setupProgressBar)
		if err != nil {
			panic(err)
		}

		dockerBuildStageSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		envFileChanged := false
		currentEnvFileHash := ""
		if appConfigFile.App.Env.File != "" {
			envFileContent, envFileErr := os.ReadFile(fmt.Sprintf("./%s", appConfigFile.App.Env.File))
			if envFileErr != nil {
				pterm.Error.Println("Unable to process your env file")
				os.Exit(1)
			}
			currentEnvFileHash = fmt.Sprintf("%x", md5.Sum(envFileContent))
			envFileChanged = appConfigFile.App.Env.Hash != currentEnvFileHash
			if envFileChanged {
				// encrypt new env file
				envCmd := exec.Command("sh", "-s", "-", viper.Get("publicKey").(string), fmt.Sprintf("./%s", appConfigFile.App.Env.File))
				envCmd.Stdin = strings.NewReader(utils.EnvEncryptionScript)
				envCmd.Stdout = os.Stdout
				envCmd.Stderr = os.Stderr
				if envCmdErr := envCmd.Run(); envCmdErr != nil {
					panic(envCmdErr)
				}
				encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "root", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appConfigFile.App.Name)))
				encryptSync.Run()
			}
		}

		cwd, _ := os.Getwd()
		dockerBuildCommd := exec.Command("sh", "-s", "-", appConfigFile.App.Name, viper.Get("dockerUsername").(string), cwd)
		dockerBuildCommd.Stdin = strings.NewReader(utils.DockerHandleScript)
		if dockerBuildErr := dockerBuildCommd.Run(); dockerBuildErr != nil {
			panic(dockerBuildErr)
		}
		dockerBuildStageSpinner.Success("Latest docker image built")

		deployStageSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		if appConfigFile.App.Env.File != "" {
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

		latestVersion := strings.Split(appConfigFile.App.Version, "")[1]
		latestVersionInt, _ := strconv.ParseInt(latestVersion, 0, 64)
		appConfigFile.App.Version = fmt.Sprintf("V%d", latestVersionInt+1)
		// env file changed ? -> update hash
		if envFileChanged {
			appConfigFile.App.Env.Hash = currentEnvFileHash
		}
		ymlData, err := yaml.Marshal(&appConfigFile)
		os.WriteFile("./sidekick.yml", ymlData, 0644)
		deployStageSpinner.Success("🙌 Deployed new version successfully 🙌")
		multi.Stop()

		pterm.Println()
		pterm.Info.Printfln("😎 View your app at: https://%s", appConfigFile.App.Url)

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
