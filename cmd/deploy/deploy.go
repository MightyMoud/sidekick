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
package deploy

import (
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	teaLog "github.com/charmbracelet/log"
	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var DeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a new version of your application to your VPS using Sidekick",
	Long: `This command deploys a new version of your application to your VPS. 
It assumes that your VPS is already configured and that your application is ready for deployment`,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		if configErr := utils.ViperInit(); configErr != nil {
			pterm.Error.Println("Sidekick config not found - Run sidekick init")
			os.Exit(1)
		}
		if !utils.FileExists("./sidekick.yml") {
			pterm.Error.Println(`Sidekick config not found in current directory Run sidekick launch`)
			os.Exit(1)
		}
		if viper.GetString("secretKey") == "" {
			render.GetLogger(teaLog.Options{Prefix: "Backward Compat"}).Error("Recent changes to how Sidekick handles secrets prevents you from launcing a new application.")
			render.GetLogger(teaLog.Options{Prefix: "Backward Compat"}).Info("To fix this, run `Sidekick init` with the same server address you have now.")
			render.GetLogger(teaLog.Options{Prefix: "Backward Compat"}).Info("Learn more at www.sidekickdeploy.com/docs/design/encryption")
			os.Exit(1)
		}

		cmdStages := []render.Stage{
			render.MakeStage("Validating connection with VPS", "VPS is reachable", false),
			render.MakeStage("Updating secrets if needed", "Env file check complete", false),
			render.MakeStage("Building latest docker image of your app", "Latest docker image built", true),
			render.MakeStage("Saving docker image locally", "Image saved successfully", false),
			render.MakeStage("Moving image to your server", "Image moved and loaded successfully", false),
			render.MakeStage("Deploying a new version of your application", "Deployed new version successfully", false),
		}
		p := tea.NewProgram(render.TuiModel{
			Stages:      cmdStages,
			BannerMsg:   "Deploying a new env of your app 😎",
			ActiveIndex: 0,
			Quitting:    false,
			AllDone:     false,
		})

		appConfig, loadError := utils.LoadAppConfig()
		if loadError != nil {
			panic(loadError)
		}
		replacer := strings.NewReplacer(
			"$service_name", appConfig.Name,
			"$app_port", fmt.Sprint(appConfig.Port),
			"$age_secret_key", viper.GetString("secretKey"),
		)

		go func() {
			sshClient, err := utils.Login(viper.GetString("serverAddress"), "sidekick")
			if err != nil {
				p.Send(render.ErrorMsg{})
			}
			p.Send(render.NextStageMsg{})

			envFileChanged := false
			currentEnvFileHash := ""
			if appConfig.Env.File != "" {
				envFileContent, envFileErr := os.ReadFile(fmt.Sprintf("./%s", appConfig.Env.File))
				if envFileErr != nil {
					p.Send(render.ErrorMsg{ErrorStr: envFileErr.Error()})
				}
				currentEnvFileHash = fmt.Sprintf("%x", md5.Sum(envFileContent))
				envFileChanged = appConfig.Env.Hash != currentEnvFileHash
				if envFileChanged {
					// encrypt new env file
					envCmd := exec.Command("sh", "-s", "-", viper.GetString("publicKey"), fmt.Sprintf("./%s", appConfig.Env.File))
					envCmd.Stdin = strings.NewReader(utils.EnvEncryptionScript)
					envCmdErrPipe, _ := envCmd.StderrPipe()
					go render.SendLogsToTUI(envCmdErrPipe, p)
					if envCmdErr := envCmd.Run(); envCmdErr != nil {
						p.Send(render.ErrorMsg{ErrorStr: envCmdErr.Error()})
					}
					encryptSyncCmd := exec.Command("rsync", "-v", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appConfig.Name)))
					encryptSyncCmdErrPipe, _ := encryptSyncCmd.StderrPipe()
					go render.SendLogsToTUI(encryptSyncCmdErrPipe, p)
					if encryptSyncCmdErr := encryptSyncCmd.Run(); encryptSyncCmdErr != nil {
						p.Send(render.ErrorMsg{ErrorStr: encryptSyncCmdErr.Error()})
						time.Sleep(time.Millisecond * 200)
					}
				}
			}
			defer os.Remove("encrypted.env")

			p.Send(render.NextStageMsg{})

			cwd, _ := os.Getwd()
			imgFileName := fmt.Sprintf("%s-latest.tar", appConfig.Name)
			dockerBuildCmd := exec.Command("docker", "build", "--tag", appConfig.Name, "--progress=plain", "--platform=linux/amd64", cwd)
			dockerBuildCmdErrPipe, _ := dockerBuildCmd.StderrPipe()
			go render.SendLogsToTUI(dockerBuildCmdErrPipe, p)

			if dockerBuildErr := dockerBuildCmd.Run(); dockerBuildErr != nil {
				p.Send(render.ErrorMsg{})
			}

			time.Sleep(time.Millisecond * 100)

			p.Send(render.NextStageMsg{})

			imgSaveCmd := exec.Command("docker", "save", "-o", imgFileName, appConfig.Name)
			imgSaveCmdErrPipe, _ := imgSaveCmd.StderrPipe()
			go render.SendLogsToTUI(imgSaveCmdErrPipe, p)

			if imgSaveCmdErr := imgSaveCmd.Run(); imgSaveCmdErr != nil {
				p.Send(render.ErrorMsg{})
			}

			time.Sleep(time.Millisecond * 200)

			p.Send(render.NextStageMsg{})

			remoteDist := fmt.Sprintf("%s@%s:./%s", "sidekick", viper.GetString("serverAddress"), appConfig.Name)
			imgMoveCmd := exec.Command("scp", "-C", imgFileName, remoteDist)
			imgMoveCmdErrorPipe, _ := imgMoveCmd.StderrPipe()
			go render.SendLogsToTUI(imgMoveCmdErrorPipe, p)

			if imgMovCmdErr := imgMoveCmd.Run(); imgMovCmdErr != nil {
				p.Send(render.ErrorMsg{})
			}
			defer os.Remove(imgFileName)

			time.Sleep(time.Millisecond * 200)
			p.Send(render.NextStageMsg{})

			dockerLoadOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s-latest.tar", appConfig.Name, appConfig.Name))
			if sessionErr != nil {
				log.Fatal("Issue happened loading docker image")
			}

			go func() {
				p.Send(render.LogMsg{LogLine: <-dockerLoadOutChan + "\n"})
				time.Sleep(time.Millisecond * 100)
			}()

			if appConfig.Env.File != "" {
				deployScript := replacer.Replace(utils.DeployAppWithEnvScript)
				_, runVersionOutChan, sessionErr := utils.RunCommand(sshClient, deployScript)
				if sessionErr != nil {
					panic(sessionErr)
				}
				go func() {
					p.Send(render.LogMsg{LogLine: <-runVersionOutChan + "\n"})
					time.Sleep(time.Millisecond * 100)
				}()
			} else {
				deployScript := replacer.Replace(utils.DeployAppScript)
				_, runOutChan, sessionErr := utils.RunCommand(sshClient, deployScript)
				if sessionErr != nil {
					panic(sessionErr)
				}
				go func() {
					p.Send(render.LogMsg{LogLine: <-runOutChan + "\n"})
				}()
				time.Sleep(time.Second * 2)
			}

			cleanOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && rm %s", appConfig.Name, fmt.Sprintf("%s-latest.tar", appConfig.Name)))
			if sessionErr != nil {
				log.Fatal("Issue happened cleaning up the image file")
			}
			go func() {
				p.Send(render.LogMsg{LogLine: <-cleanOutChan + "\n"})
				time.Sleep(time.Millisecond * 100)
			}()

			latestVersion := strings.Split(appConfig.Version, "")[1]
			latestVersionInt, _ := strconv.ParseInt(latestVersion, 0, 64)
			appConfig.Version = fmt.Sprintf("V%d", latestVersionInt+1)
			// env file changed ? -> update hash
			if envFileChanged {
				appConfig.Env.Hash = currentEnvFileHash
			}
			ymlData, _ := yaml.Marshal(&appConfig)
			os.WriteFile("./sidekick.yml", ymlData, 0644)

			time.Sleep(time.Millisecond * 500)
			p.Send(render.AllDoneMsg{Duration: time.Since(start).Round(time.Second), URL: appConfig.Url})
		}()

		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
	},
}
