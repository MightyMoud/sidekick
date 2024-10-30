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
package launch

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var LaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch a new application to host on your VPS with Sidekick",
	Long:  `This command will run you through the basic setup to add a new application to your VPS.`,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		if configErr := utils.ViperInit(); configErr != nil {
			render.GetLogger(log.Options{Prefix: "Sidekick Config"}).Fatalf("%s", configErr)
		}

		if viper.GetString("secretKey") == "" {
			render.GetLogger(log.Options{Prefix: "Backward Compat"}).Error("Recent changes to how Sidekick handles secrets prevents you from launcing a new application.")
			render.GetLogger(log.Options{Prefix: "Backward Compat"}).Info("To fix this, run `Sidekick init` with the same server address you have now.")
			render.GetLogger(log.Options{Prefix: "Backward Compat"}).Info("Learn more at www.sidekickdeploy.com/docs/design/encryption")
			os.Exit(1)
		}

		if utils.FileExists("./sidekick.yml") {
			render.GetLogger(log.Options{Prefix: "Sidekick Setup"}).Error("Sidekick config exits in this project.")
			render.GetLogger(log.Options{Prefix: "Sidekick Setup"}).Info("You can deploy a new version of your application with Sidekick deploy.")
			os.Exit(1)
		}

		if utils.FileExists("./Dockerfile") {
			render.GetLogger(log.Options{Prefix: "Dockerfile"}).Info("Detected - scanning file for details")
		} else {
			render.GetLogger(log.Options{Prefix: "Dockerfile"}).Fatal("No dockerfile found in current directory.")
		}

		res, err := os.ReadFile("./Dockerfile")
		if err != nil {
			render.GetLogger(log.Options{Prefix: "Dockerfile"}).Fatal("Unable to process your dockerfile")
		}

		appPort := ""
		for _, line := range strings.Split(string(res), "\n") {
			if strings.HasPrefix(line, "EXPOSE") {
				appPort = line[len(line)-4:]
			}
		}

		appName := ""
		appNameTextInput := render.GetDefaultTextInput("Please enter your app url friendly app name:", appName, "will identify your app containers")
		appName, appNameTextInputErr := appNameTextInput.RunPrompt()
		if appNameTextInputErr != nil {
			render.GetLogger(log.Options{Prefix: "Name Input"}).Fatalf(" %s", appNameTextInputErr)
		}

		appPortTextInput := render.GetDefaultTextInput("Please enter the port at which the app receives request:", appPort, "")
		appPort, appPortTextInputErr := appPortTextInput.RunPrompt()
		if appPortTextInputErr != nil {
			render.GetLogger(log.Options{Prefix: "Port Input"}).Fatalf(" %s", appPortTextInputErr)
		}

		appDomain := fmt.Sprintf("%s.%s.sslip.io", appName, viper.Get("serverAddress").(string))
		appDomainTextInput := render.GetDefaultTextInput("Please enter the domain to point the app to:", appDomain, "must point to your VPS ddress")
		appDomain, appDomainTextInputErr := appDomainTextInput.RunPrompt()
		if appDomainTextInputErr != nil {
			render.GetLogger(log.Options{Prefix: "Domain Input"}).Fatalf(" %s", appDomainTextInputErr)
		}

		envFileName := ".env"
		envFileNameTextInput := render.GetDefaultTextInput("Please enter which env file you would like to load", envFileName, "")
		envFileName, envFileNameTextInputErr := envFileNameTextInput.RunPrompt()
		if envFileNameTextInputErr != nil {
			render.GetLogger(log.Options{Prefix: "Env Input"}).Fatalf(" %s", envFileNameTextInputErr)
		}

		hasEnvFile := false
		envVariables := []string{}
		dockerEnvProperty := []string{}
		envFileChecksum := ""
		if utils.FileExists(fmt.Sprintf("./%s", envFileName)) {
			hasEnvFile = true
			render.GetLogger(log.Options{Prefix: "Env File"}).Infof("Detected - Loading env vars from %s", envFileName)
			envHandleErr := utils.HandleEnvFile(envFileName, envVariables, &dockerEnvProperty, &envFileChecksum)
			if envHandleErr != nil {
				render.GetLogger(log.Options{Prefix: "Env File"}).Fatalf("Something went wrong %s", envHandleErr)
			}
			defer os.Remove("encrypted.env")
		} else {
			render.GetLogger(log.Options{Prefix: "Env File"}).Info("Not Detected - Skipping env parsing")
		}

		// make a docker service
		imageName := appName
		newService := utils.DockerService{
			Image:   imageName,
			Restart: "unless-stopped",
			Labels: []string{
				"traefik.enable=true",
				fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", appName, appDomain),
				fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", appName, appPort),
				fmt.Sprintf("traefik.http.routers.%s.tls=true", appName),
				fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=default", appName),
				"traefik.docker.network=sidekick",
			},
			Environment: dockerEnvProperty,
			Networks: []string{
				"sidekick",
			},
		}
		newDockerCompose := utils.DockerComposeFile{
			Services: map[string]utils.DockerService{
				appName: newService,
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
		defer os.Remove("docker-compose.yaml")

		cmdStages := []render.Stage{
			render.MakeStage("Validating connection with VPS", "VPS is reachable", false),
			render.MakeStage("Building latest docker image of your app", "Latest docker image built", true),
			render.MakeStage("Saving docker image locally", "Image saved successfully", false),
			render.MakeStage("Moving image to your server", "Image moved and loaded successfully", false),
			render.MakeStage("Setting up your application", "Application setup successfully", false),
		}
		p := tea.NewProgram(render.TuiModel{
			Stages:      cmdStages,
			BannerMsg:   "Launching your application on your VPS 🚀",
			ActiveIndex: 0,
			Quitting:    false,
			AllDone:     false,
		})

		go func() {
			sshClient, err := utils.Login(viper.Get("serverAddress").(string), "sidekick")
			if err != nil {
				p.Send(render.ErrorMsg{ErrorStr: "Something went wrong logging in to your VPS"})
			}
			p.Send(render.NextStageMsg{})

			cwd, _ := os.Getwd()
			dockerBuildCmd := exec.Command("docker", "build", "--tag", appName, "--progress=plain", "--platform=linux/amd64", cwd)
			dockerBuildCmdErrPipe, _ := dockerBuildCmd.StderrPipe()
			go render.SendLogsToTUI(dockerBuildCmdErrPipe, p)

			if dockerBuildErr := dockerBuildCmd.Run(); dockerBuildErr != nil {
				p.Send(render.ErrorMsg{})
			}

			time.Sleep(time.Millisecond * 100)

			p.Send(render.NextStageMsg{})

			imgFileName := fmt.Sprintf("%s-latest.tar", appName)
			imgSaveCmd := exec.Command("docker", "save", "-o", imgFileName, appName)
			imgSaveCmdErrPipe, _ := imgSaveCmd.StderrPipe()
			go render.SendLogsToTUI(imgSaveCmdErrPipe, p)

			if imgSaveCmdErr := imgSaveCmd.Run(); imgSaveCmdErr != nil {
				p.Send(render.ErrorMsg{})
			}

			time.Sleep(time.Millisecond * 100)

			p.Send(render.NextStageMsg{})

			_, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("mkdir %s", appName))
			if sessionErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: sessionErr.Error()})
			}

			remoteDist := fmt.Sprintf("%s@%s:./%s", "sidekick", viper.GetString("serverAddress"), appName)
			imgMoveCmd := exec.Command("scp", "-C", imgFileName, remoteDist)
			imgMoveCmdErrorPipe, _ := imgMoveCmd.StderrPipe()
			go render.SendLogsToTUI(imgMoveCmdErrorPipe, p)

			if imgMovCmdErr := imgMoveCmd.Run(); imgMovCmdErr != nil {
				p.Send(render.ErrorMsg{})
			}
			defer os.Remove(imgFileName)

			time.Sleep(time.Millisecond * 200)

			dockerLoadOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s && rm %s", appName, imgFileName, imgFileName))
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

			rsyncCmd := exec.Command("rsync", "docker-compose.yaml", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appName)))
			rsyncCmErr := rsyncCmd.Run()
			if rsyncCmErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: rsyncCmErr.Error()})
			}

			if hasEnvFile {
				encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appName)))
				encryptSyncErr := encryptSync.Run()
				if encryptSyncErr != nil {
					p.Send(render.ErrorMsg{ErrorStr: encryptSyncErr.Error()})
				}

				runAppCmdOutChan, _, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && export SOPS_AGE_KEY=%s && sops exec-env encrypted.env 'docker compose -p sidekick up -d'`, appName, viper.GetString("secretKey")))
				go func() {
					p.Send(render.LogMsg{LogLine: <-runAppCmdOutChan + "\n"})
					time.Sleep(time.Millisecond * 50)
				}()
				if sessionErr1 != nil {
					p.Send(render.ErrorMsg{ErrorStr: sessionErr1.Error()})
				}
			} else {
				runAppCmdOutChan, _, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && docker compose -p sidekick up -d`, appName))
				go func() {
					p.Send(render.LogMsg{LogLine: <-runAppCmdOutChan + "\n"})
					time.Sleep(time.Millisecond * 50)
				}()
				if sessionErr1 != nil {
					p.Send(render.ErrorMsg{ErrorStr: sessionErr1.Error()})
				}
			}

			portNumber, err := strconv.ParseUint(appPort, 0, 64)
			if err != nil {
				panic(err)
			}
			envConfig := utils.SidekickAppEnvConfig{}
			if hasEnvFile {
				envConfig.File = envFileName
				envConfig.Hash = envFileChecksum
			}
			// save app config in same folder
			sidekickAppConfig := utils.SidekickAppConfig{
				Name:      appName,
				Version:   "V1",
				Port:      portNumber,
				Url:       appDomain,
				CreatedAt: time.Now().Format(time.UnixDate),
				Env:       envConfig,
			}
			ymlData, _ := yaml.Marshal(&sidekickAppConfig)
			os.WriteFile("./sidekick.yml", ymlData, 0644)
			p.Send(render.AllDoneMsg{Duration: time.Since(start).Round(time.Second), URL: appDomain})
		}()

		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}

	},
}
