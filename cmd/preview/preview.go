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
package preview

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	previewList "github.com/mightymoud/sidekick/cmd/preview/list"
	previewRemove "github.com/mightymoud/sidekick/cmd/preview/remove"
	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var PreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Deploy a preview environment for your application",
	Long:  `Sidekick allows you to deploy preview environment based on commit hash`,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		if configErr := utils.ViperInit(); configErr != nil {
			render.GetLogger(log.Options{Prefix: "Sidekick Config"}).Fatalf("%s", configErr)
		}
		appConfig, appConfigErr := utils.LoadAppConfig()
		if appConfigErr != nil {
			render.GetLogger(log.Options{Prefix: "Sidekick Setup"}).Error("Sidekick config exits in this project.")
			render.GetLogger(log.Options{Prefix: "Sidekick Setup"}).Info("You can deploy a new version of your application with Sidekick deploy.")
			os.Exit(1)
		}

		if viper.GetString("secretKey") == "" {
			render.GetLogger(log.Options{Prefix: "Backward Compat"}).Error("Recent changes to how Sidekick handles secrets prevents you from launcing a new application.")
			render.GetLogger(log.Options{Prefix: "Backward Compat"}).Info("To fix this, run `Sidekick init` with the same server address you have now.")
			render.GetLogger(log.Options{Prefix: "Backward Compat"}).Info("Learn more at www.sidekickdeploy.com/docs/design/encryption")
			os.Exit(1)
		}

		gitTreeCheck := exec.Command("sh", "-s", "-")
		gitTreeCheck.Stdin = strings.NewReader(utils.CheckGitTreeScript)
		output, _ := gitTreeCheck.Output()
		if string(output) != "all good\n" {
			render.GetLogger(log.Options{Prefix: "Preview Cmd"}).Error("Please commit any changes to git before deploying a preview environment")
			os.Exit(1)
		}

		gitShortHashCmd := exec.Command("sh", "-s", "-")
		gitShortHashCmd.Stdin = strings.NewReader("git rev-parse --short HEAD")
		hashOutput, hashErr := gitShortHashCmd.Output()
		if hashErr != nil {
			render.GetLogger(log.Options{Prefix: "Preview Cmd"}).Error("Issue occurred getting git commit hash: %s", hashErr)
			os.Exit(1)
		}
		deployHash := strings.TrimSuffix(string(hashOutput), "\n")

		cmdStages := []render.Stage{
			render.MakeStage("Validating connection with VPS", "VPS is reachable", false),
			render.MakeStage("Building latest docker image of your app", "Latest docker image built", true),
			render.MakeStage("Saving docker image locally", "Image saved successfully", false),
			render.MakeStage("Moving image to your server", "Image moved and loaded successfully", false),
			render.MakeStage("Deploying a preview env of your application", "Preview env setup successfully", false),
		}
		p := tea.NewProgram(render.TuiModel{
			Stages:      cmdStages,
			BannerMsg:   "Deploying a preview env of your app 😎",
			ActiveIndex: 0,
			Quitting:    false,
			AllDone:     false,
		})

		go func() {
			sshClient, err := utils.Login(viper.GetString("serverAddress"), "sidekick")
			if err != nil {
				p.Send(render.ErrorMsg{})
			}
			p.Send(render.NextStageMsg{})

			dockerEnvProperty := []string{}
			envFileChecksum := ""
			if appConfig.Env.File != "" {
				envErr := utils.HandleEnvFile(appConfig.Env.File, &dockerEnvProperty, &envFileChecksum)
				if envErr != nil {
					panic(envErr)
				}
			}

			imageName := fmt.Sprintf("%s:%s", appConfig.Name, deployHash)
			serviceName := fmt.Sprintf("%s-%s", appConfig.Name, deployHash)
			previewURL := fmt.Sprintf("%s.%s", deployHash, appConfig.Url)
			newService := utils.DockerService{
				Image: imageName,
				Labels: []string{
					"traefik.enable=true",
					fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", serviceName, previewURL),
					fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", serviceName, fmt.Sprint(appConfig.Port)),
					fmt.Sprintf("traefik.http.routers.%s.tls=true", serviceName),
					fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=default", serviceName),
					"traefik.docker.network=sidekick",
				},
				Environment: dockerEnvProperty,
				Networks: []string{
					"sidekick",
				},
			}
			newDockerCompose := utils.DockerComposeFile{
				Services: map[string]utils.DockerService{
					serviceName: newService,
				},
				Networks: map[string]utils.DockerNetwork{
					"sidekick": {
						External: true,
					},
				},
			}
			dockerComposeFile, err := yaml.Marshal(&newDockerCompose)
			if err != nil {
				fmt.Printf("Error marshalling YAML: %v\n", err)
				return
			}
			err = os.WriteFile("docker-compose.yaml", dockerComposeFile, 0644)
			if err != nil {
				fmt.Printf("Error writing file: %v\n", err)
				return
			}

			cwd, _ := os.Getwd()
			dockerImage := fmt.Sprintf("%s:%s", appConfig.Name, deployHash)
			dockerBuildCmd := exec.Command("docker", "build", "--tag", dockerImage, "--progress=plain", "--platform=linux/amd64", cwd)
			dockerBuildCmdErrPipe, _ := dockerBuildCmd.StderrPipe()
			go render.SendLogsToTUI(dockerBuildCmdErrPipe, p)

			if dockerBuildErr := dockerBuildCmd.Run(); dockerBuildErr != nil {
				p.Send(render.ErrorMsg{})
			}

			time.Sleep(time.Millisecond * 100)

			p.Send(render.NextStageMsg{})

			imgFileName := fmt.Sprintf("%s-%s.tar", appConfig.Name, deployHash)
			imgSaveCmd := exec.Command("docker", "save", "-o", imgFileName, dockerImage)
			imgSaveCmdErrPipe, _ := imgSaveCmd.StderrPipe()
			go render.SendLogsToTUI(imgSaveCmdErrPipe, p)

			if imgSaveCmdErr := imgSaveCmd.Run(); imgSaveCmdErr != nil {
				p.Send(render.ErrorMsg{})
			}

			time.Sleep(time.Millisecond * 100)

			p.Send(render.NextStageMsg{})

			_, _, sessionErr0 := utils.RunCommand(sshClient, fmt.Sprintf(`mkdir -p %s/preview/%s`, appConfig.Name, deployHash))
			if sessionErr0 != nil {
				p.Send(render.ErrorMsg{ErrorStr: sessionErr0.Error()})
			}

			remoteDist := fmt.Sprintf("%s@%s:./%s", "sidekick", viper.GetString("serverAddress"), appConfig.Name)
			imgMoveCmd := exec.Command("scp", "-C", imgFileName, remoteDist)
			imgMoveCmdErrorPipe, _ := imgMoveCmd.StderrPipe()
			go render.SendLogsToTUI(imgMoveCmdErrorPipe, p)

			if imgMovCmdErr := imgMoveCmd.Run(); imgMovCmdErr != nil {
				p.Send(render.ErrorMsg{})
			}

			time.Sleep(time.Millisecond * 200)

			dockerLoadOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s && rm %s", appConfig.Name, imgFileName, imgFileName))
			go func() {
				p.Send(render.LogMsg{LogLine: <-dockerLoadOutChan + "\n"})
				time.Sleep(time.Millisecond * 50)
			}()
			if sessionErr != nil {
				time.Sleep(time.Millisecond * 100)
				p.Send(render.ErrorMsg{ErrorStr: sessionErr.Error()})
			}

			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			previewFolder := fmt.Sprintf("./%s/preview/%s", appConfig.Name, deployHash)
			rsyncCmd := exec.Command("rsync", "docker-compose.yaml", fmt.Sprintf("%s@%s:%s", "sidekick", viper.GetString("serverAddress"), previewFolder))
			rsyncCmErr := rsyncCmd.Run()
			if rsyncCmErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: rsyncCmErr.Error()})
			}

			if appConfig.Env.File != "" {
				encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.GetString("serverAddress"), previewFolder))
				encryptSyncErrr := encryptSync.Run()
				if encryptSyncErrr != nil {
					p.Send(render.ErrorMsg{ErrorStr: encryptSyncErrr.Error()})
				}

				runAppCmdOutChan, _, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && export SOPS_AGE_KEY=%s && sops exec-env encrypted.env 'docker compose -p sidekick up -d'`, previewFolder, viper.GetString("secretKey")))
				go func() {
					p.Send(render.LogMsg{LogLine: <-runAppCmdOutChan + "\n"})
					time.Sleep(time.Millisecond * 50)
				}()
				if sessionErr1 != nil {
					p.Send(render.ErrorMsg{ErrorStr: sessionErr1.Error()})
				}
			} else {
				runAppCmdOutChan, _, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && docker compose -p sidekick up -d`, previewFolder))
				go func() {
					p.Send(render.LogMsg{LogLine: <-runAppCmdOutChan + "\n"})
					time.Sleep(time.Millisecond * 50)
				}()
				if sessionErr1 != nil {
					p.Send(render.ErrorMsg{ErrorStr: sessionErr1.Error()})
				}
			}
			previewEnvConfig := utils.SidekickPreview{
				Url:       fmt.Sprintf("https://%s", previewURL),
				Image:     imageName,
				CreatedAt: time.Now().Format(time.UnixDate),
			}
			if len(appConfig.PreviewEnvs) == 0 {
				appConfig.PreviewEnvs = map[string]utils.SidekickPreview{}
			}
			appConfig.PreviewEnvs[deployHash] = previewEnvConfig

			ymlData, _ := yaml.Marshal(&appConfig)
			os.WriteFile("./sidekick.yml", ymlData, 0644)

			os.Remove("docker-compose.yaml")
			os.Remove("encrypted.env")
			os.Remove(imgFileName)

			p.Send(render.AllDoneMsg{Message: "🚀 Deployed successfully in " + time.Since(start).Round(time.Second).String() + ".\n" + "😎 View your app at https://" + previewURL})

		}()

		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
	},
}

func init() {
	PreviewCmd.AddCommand(previewList.ListCmd)
	PreviewCmd.AddCommand(previewRemove.RemoveCmd)
}
