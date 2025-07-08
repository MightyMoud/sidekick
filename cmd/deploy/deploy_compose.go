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

// Package deploy implements Docker Compose deployment support for Sidekick
// Author: madebycm (https://github.com/madebycm)
package deploy

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	
	"github.com/joho/godotenv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func deployCompose(appConfig utils.SidekickAppConfig) {
	start := time.Now()

	// Parse the compose file
	compose, err := utils.ParseComposeFile(appConfig.ComposeFile)
	if err != nil {
		pterm.Error.Printf("Failed to parse compose file: %s", err)
		os.Exit(1)
	}

	// Get services with build contexts
	buildServices := utils.GetServicesWithBuildContext(compose)
	
	// Get main service
	mainService := compose.Services[appConfig.MainService]

	// Update Traefik labels on main service
	if mainService.Labels == nil {
		mainService.Labels = []string{}
	}
	
	// Update labels with new domain if changed
	var updatedLabels []string
	for _, label := range mainService.Labels {
		if strings.HasPrefix(label, "traefik.http.routers.") && strings.Contains(label, ".rule=Host") {
			updatedLabels = append(updatedLabels, fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", appConfig.Name, appConfig.Url))
		} else {
			updatedLabels = append(updatedLabels, label)
		}
	}
	mainService.Labels = updatedLabels
	compose.Services[appConfig.MainService] = mainService

	// Check env file changes
	shouldUpdateEnv := false
	if appConfig.Env.File != "" && utils.FileExists(appConfig.Env.File) {
		// Calculate current env file hash
		envFile, _ := os.Open(appConfig.Env.File)
		envMap, _ := godotenv.Parse(envFile)
		envFileContent, _ := godotenv.Marshal(envMap)
		currentHash := fmt.Sprintf("%x", md5.Sum([]byte(envFileContent)))
		
		if currentHash != appConfig.Env.Hash {
			pterm.Info.Println("Detected changes to environment file - Will re-encrypt")
			shouldUpdateEnv = true
			appConfig.Env.Hash = currentHash
		}
	}

	// Prepare stages
	stages := []render.Stage{
		render.MakeStage("Checking connection with VPS", "VPS is reachable", false),
	}

	// Add build stages for services with build contexts
	imageMap := make(map[string]string)
	for serviceName := range buildServices {
		imageName := fmt.Sprintf("%s-%s:v%s", appConfig.Name, serviceName, appConfig.Version)
		imageMap[serviceName] = imageName
		stages = append(stages, render.MakeStage(
			fmt.Sprintf("Building image for service: %s", serviceName),
			fmt.Sprintf("Image built for %s", serviceName),
			true,
		))
	}

	if len(imageMap) > 0 {
		stages = append(stages,
			render.MakeStage("Preparing to deploy", "Images saved", false),
			render.MakeStage("Deploying new version", "Images pushed to server", false),
		)
	}
	
	stages = append(stages,
		render.MakeStage("Running new version", "New version is up", false),
	)

	p := tea.NewProgram(render.TuiModel{
		Stages:      stages,
		BannerMsg:   fmt.Sprintf("Deploying version %s of %s 🚀", appConfig.Version, appConfig.Name),
		ActiveIndex: 0,
		Quitting:    false,
		AllDone:     false,
	})

	go func() {
		// SSH connection
		sshClient, err := utils.Login(viper.GetString("serverAddress"), "sidekick")
		if err != nil {
			p.Send(render.ErrorMsg{ErrorStr: "Something went wrong logging in to your VPS"})
		}
		p.Send(render.NextStageMsg{})

		// Build services with build contexts
		stageIndex := 1
		for serviceName := range buildServices {
			service := buildServices[serviceName]
			imageName := imageMap[serviceName]
			
			// Determine build context
			buildContext := "."
			dockerfile := "Dockerfile"
			buildArgs := []string{}
			
			if service.Build != nil {
				if service.Build.Context != "" {
					buildContext = service.Build.Context
				}
				if service.Build.Dockerfile != "" {
					dockerfile = service.Build.Dockerfile
				}
				for key, value := range service.Build.Args {
					buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%v", key, value))
				}
			}

			// Build with cache
			dockerBuildCmd := exec.Command("docker", append([]string{
				"build",
				"--tag", imageName,
				"--progress=plain",
				"--platform=linux/amd64",
				"--cache-from", fmt.Sprintf("%s-%s:v%d", appConfig.Name, serviceName, getVersionNumber(appConfig.Version)-1),
				"-f", filepath.Join(buildContext, dockerfile),
			}, append(buildArgs, buildContext)...)...)
			
			dockerBuildCmdErrPipe, _ := dockerBuildCmd.StderrPipe()
			go render.SendLogsToTUI(dockerBuildCmdErrPipe, p)

			if dockerBuildErr := dockerBuildCmd.Run(); dockerBuildErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Failed to build %s", serviceName)})
			}

			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})
			stageIndex++
		}

		// Update compose file with new image tags
		for serviceName, imageName := range imageMap {
			if service, exists := compose.Services[serviceName]; exists {
				service.Image = imageName
				service.Build = nil
				compose.Services[serviceName] = service
			}
		}

		// Write updated compose file
		updatedComposeFile := fmt.Sprintf("sidekick-%s", appConfig.ComposeFile)
		composeData, _ := yaml.Marshal(&compose)
		os.WriteFile(updatedComposeFile, composeData, 0644)
		defer os.Remove(updatedComposeFile)

		// Save and transfer images if any were built
		if len(imageMap) > 0 {
			imgFileName := fmt.Sprintf("%s-v%s.tar", appConfig.Name, appConfig.Version)
			
			var imageNames []string
			for _, imageName := range imageMap {
				imageNames = append(imageNames, imageName)
			}
			
			imgSaveCmd := exec.Command("docker", append([]string{"save", "-o", imgFileName}, imageNames...)...)
			imgSaveCmdErrPipe, _ := imgSaveCmd.StderrPipe()
			go render.SendLogsToTUI(imgSaveCmdErrPipe, p)

			if imgSaveCmdErr := imgSaveCmd.Run(); imgSaveCmdErr != nil {
				p.Send(render.ErrorMsg{})
			}
			defer os.Remove(imgFileName)

			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			// Transfer
			remoteDist := fmt.Sprintf("%s@%s:./%s", "sidekick", viper.GetString("serverAddress"), appConfig.Name)
			imgMoveCmd := exec.Command("scp", "-C", imgFileName, remoteDist)
			imgMoveCmdErrorPipe, _ := imgMoveCmd.StderrPipe()
			go render.SendLogsToTUI(imgMoveCmdErrorPipe, p)

			if imgMovCmdErr := imgMoveCmd.Run(); imgMovCmdErr != nil {
				p.Send(render.ErrorMsg{})
			}

			// Load on server
			dockerLoadOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s && rm %s", appConfig.Name, imgFileName, imgFileName))
			go func() {
				for line := range dockerLoadOutChan {
					p.Send(render.LogMsg{LogLine: line + "\n"})
				}
			}()
			if sessionErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: sessionErr.Error()})
			}

			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})
		}

		// Transfer updated compose file
		rsyncCmd := exec.Command("rsync", updatedComposeFile, fmt.Sprintf("%s@%s:%s/%s", "sidekick", viper.GetString("serverAddress"), appConfig.Name, "docker-compose.yml"))
		if rsyncCmErr := rsyncCmd.Run(); rsyncCmErr != nil {
			p.Send(render.ErrorMsg{ErrorStr: rsyncCmErr.Error()})
		}

		// Handle env update if needed
		if shouldUpdateEnv {
			dockerEnvProperty := []string{}
			utils.HandleEnvFile(appConfig.Env.File, &dockerEnvProperty, &appConfig.Env.Hash)
			
			encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.GetString("serverAddress"), appConfig.Name))
			encryptSync.Run()
			os.Remove("encrypted.env")
		}

		// Deploy with zero downtime
		var deployCmd string
		if appConfig.Env.File != "" {
			deployCmd = fmt.Sprintf(`cd %s && export SOPS_AGE_KEY=%s && sops exec-env encrypted.env 'docker compose -p %s up -d'`, 
				appConfig.Name, viper.GetString("secretKey"), appConfig.Name)
		} else {
			deployCmd = fmt.Sprintf(`cd %s && docker compose -p %s up -d`, appConfig.Name, appConfig.Name)
		}

		deployOutChan, _, sessionErr := utils.RunCommand(sshClient, deployCmd)
		go func() {
			for line := range deployOutChan {
				p.Send(render.LogMsg{LogLine: line + "\n"})
			}
		}()
		if sessionErr != nil {
			p.Send(render.ErrorMsg{ErrorStr: sessionErr.Error()})
		}

		// Update version
		appConfig.Version = fmt.Sprintf("V%d", getVersionNumber(appConfig.Version)+1)
		ymlData, _ := yaml.Marshal(&appConfig)
		os.WriteFile("./sidekick.yml", ymlData, 0644)

		p.Send(render.AllDoneMsg{Duration: time.Since(start).Round(time.Second), URL: appConfig.Url})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func getVersionNumber(version string) int {
	var v int
	fmt.Sscanf(version, "V%d", &v)
	return v
}