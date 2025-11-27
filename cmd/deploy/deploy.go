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
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

func prelude() utils.SidekickAppConfig {
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

	appConfig, loadError := utils.LoadAppConfig()
	if loadError != nil {
		panic(loadError)
	}
	return appConfig
}

func stage1Login() (*ssh.Client, error) {
	sshClient, err := utils.Login(viper.GetString("serverAddress"), "sidekick")
	return sshClient, err
}

func stage2EnvFile(appConfig utils.SidekickAppConfig, p *tea.Program) (bool, string, error) {
	defer os.Remove("encrypted.env")
	envFileChanged := false
	currentEnvFileHash := ""
	if appConfig.Env.File != "" {
		envFileContent, envFileErr := os.ReadFile(fmt.Sprintf("./%s", appConfig.Env.File))
		if envFileErr != nil {
			return false, "", fmt.Errorf("failed to read environment file: %w", envFileErr)
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
				return false, "", fmt.Errorf("failed to encrypt environment file: %w", envCmdErr)
			}
			encryptSyncCmd := exec.Command("rsync", "-v", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appConfig.Name)))
			encryptSyncCmdErrPipe, _ := encryptSyncCmd.StderrPipe()
			go render.SendLogsToTUI(encryptSyncCmdErrPipe, p)
			if encryptSyncCmdErr := encryptSyncCmd.Run(); encryptSyncCmdErr != nil {
				return false, "", fmt.Errorf("failed to sync encrypted environment file to server: %w", encryptSyncCmdErr)
			}
		}
	}
	return envFileChanged, currentEnvFileHash, nil
}

func stage3BuildDockerImage(appConfig utils.SidekickAppConfig, p *tea.Program) error {
	cwd, _ := os.Getwd()
	dockerPlatformId := viper.GetString("platformID")
	dockerBuildCmd := exec.Command("docker", "build", "--tag", appConfig.Name, "--progress=plain", fmt.Sprintf("--platform=%s", dockerPlatformId), cwd)
	dockerBuildCmdErrPipe, _ := dockerBuildCmd.StderrPipe()
	go render.SendLogsToTUI(dockerBuildCmdErrPipe, p)

	if dockerBuildErr := dockerBuildCmd.Run(); dockerBuildErr != nil {
		return fmt.Errorf("failed to build Docker image: %w", dockerBuildErr)
	}
	return nil
}

func stage4SaveDockerImage(appConfig utils.SidekickAppConfig, p *tea.Program) error {
	imgFileName := fmt.Sprintf("%s-latest.tar", appConfig.Name)
	imgSaveCmd := exec.Command("docker", "save", "-o", imgFileName, appConfig.Name)
	imgSaveCmdErrPipe, _ := imgSaveCmd.StderrPipe()
	go render.SendLogsToTUI(imgSaveCmdErrPipe, p)

	if imgSaveCmdErr := imgSaveCmd.Run(); imgSaveCmdErr != nil {
		return fmt.Errorf("failed to save Docker image: %w", imgSaveCmdErr)
	}
	return nil
}

func stage5MoveDockerImage(appConfig utils.SidekickAppConfig, p *tea.Program) error {
	imgFileName := fmt.Sprintf("%s-latest.tar", appConfig.Name)
	remoteDist := fmt.Sprintf("%s@%s:./%s", "sidekick", viper.GetString("serverAddress"), appConfig.Name)
	imgMoveCmd := exec.Command("scp", "-C", imgFileName, remoteDist)
	imgMoveCmdErrorPipe, _ := imgMoveCmd.StderrPipe()
	go render.SendLogsToTUI(imgMoveCmdErrorPipe, p)

	if imgMovCmdErr := imgMoveCmd.Run(); imgMovCmdErr != nil {
		return fmt.Errorf("failed to move Docker image to server: %w", imgMovCmdErr)
	}
	os.Remove(imgFileName)
	return nil
}

func stage6Deploy(sshClient *ssh.Client, appConfig utils.SidekickAppConfig, envFileChanged bool, currentEnvFileHash string, p *tea.Program) error {
	dockerLoadOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s-latest.tar", appConfig.Name, appConfig.Name))
	if sessionErr != nil {
		return fmt.Errorf("failed to load docker image on server: %w", sessionErr)
	}

	go func() {
		p.Send(render.LogMsg{LogLine: <-dockerLoadOutChan + "\n"})
		time.Sleep(time.Millisecond * 100)
	}()

	replacer := strings.NewReplacer(
		"$service_name", appConfig.Name,
		"$app_port", fmt.Sprint(appConfig.Port),
		"$age_secret_key", viper.GetString("secretKey"),
	)

	if appConfig.Env.File != "" {
		deployScript := replacer.Replace(utils.DeployAppWithEnvScript)
		_, runVersionOutChan, sessionErr := utils.RunCommand(sshClient, deployScript)
		if sessionErr != nil {
			return fmt.Errorf("failed to deploy application with environment file: %w", sessionErr)
		}
		go func() {
			p.Send(render.LogMsg{LogLine: <-runVersionOutChan + "\n"})
			time.Sleep(time.Millisecond * 100)
		}()
	} else {
		deployScript := replacer.Replace(utils.DeployApp)
		utils.RunCommandWithTUIHook(sshClient, deployScript, p)
		time.Sleep(time.Second * 2)
	}

	cleanOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && rm %s", appConfig.Name, fmt.Sprintf("%s-latest.tar", appConfig.Name)))
	if sessionErr != nil {
		return fmt.Errorf("failed to clean up image file on server: %w", sessionErr)
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

	return nil
}

var DeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a new version of your application to your VPS using Sidekick",
	Long: `This command deploys a new version of your application to your VPS. 
It assumes that your VPS is already configured and that your application is ready for deployment`,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		appConfig := prelude()

		cmdStages := []render.Stage{
			render.MakeStage("Validating connection with VPS", "VPS is reachable", false),
			render.MakeStage("Updating secrets if needed", "Env file check complete", false),
			render.MakeStage("Building latest docker image of your app", "Latest docker image built", true),
			render.MakeStage("Saving docker image locally", "Image saved successfully", false),
			render.MakeStage("Moving image to your server", "Image moved and loaded successfully", false),
			render.MakeStage("Deploying a new version of your application", "Deployed new version successfully", true),
		}
		p := tea.NewProgram(render.TuiModel{
			Stages:      cmdStages,
			BannerMsg:   "Deploying a new env of your app 😎",
			ActiveIndex: 0,
			Quitting:    false,
			AllDone:     false,
		})

		go func() {
			sshClient, err := stage1Login()
			if err != nil {
				p.Send(render.ErrorMsg{ErrorStr: "Failed to connect to VPS: " + err.Error()})
				return
			}
			p.Send(render.NextStageMsg{})

			envFileChanged, currentEnvFileHash, err := stage2EnvFile(appConfig, p)
			if err != nil {
				p.Send(render.ErrorMsg{ErrorStr: err.Error()})
				return
			}
			p.Send(render.NextStageMsg{})

			if err := stage3BuildDockerImage(appConfig, p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: err.Error()})
				return
			}
			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			if err := stage4SaveDockerImage(appConfig, p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: err.Error()})
				return
			}
			time.Sleep(time.Millisecond * 200)
			p.Send(render.NextStageMsg{})

			if err := stage5MoveDockerImage(appConfig, p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: err.Error()})
				return
			}
			time.Sleep(time.Millisecond * 200)
			p.Send(render.NextStageMsg{})

			if err := stage6Deploy(sshClient, appConfig, envFileChanged, currentEnvFileHash, p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: err.Error()})
				return
			}

			time.Sleep(time.Millisecond * 500)
			p.Send(render.AllDoneMsg{Message: "🚀 Deployed successfully in " + time.Since(start).Round(time.Second).String() + ".\n" + "😎 View your app at https://" + appConfig.Url})
		}()

		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
	},
}
