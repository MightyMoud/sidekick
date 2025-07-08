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

// Package launch implements Docker Compose support for Sidekick
// Author: madebycm (https://github.com/madebycm)
package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func launchCompose(composeFile string) {
	start := time.Now()
	
	// Parse the compose file
	compose, err := utils.ParseComposeFile(composeFile)
	if err != nil {
		render.GetLogger(log.Options{Prefix: "Docker Compose"}).Fatalf("Failed to parse compose file: %s", err)
	}

	// Get services with build contexts
	buildServices := utils.GetServicesWithBuildContext(compose)
	if len(buildServices) == 0 {
		render.GetLogger(log.Options{Prefix: "Docker Compose"}).Info("No services with build contexts found. All services use pre-built images.")
	}

	// Get services with exposed ports
	servicesWithPorts := utils.GetServicesWithPorts(compose)
	if len(servicesWithPorts) == 0 {
		render.GetLogger(log.Options{Prefix: "Docker Compose"}).Fatal("No services with exposed ports found in compose file")
	}

	// Let user select main service
	var mainServiceName string
	if len(servicesWithPorts) == 1 {
		mainServiceName = servicesWithPorts[0]
		render.GetLogger(log.Options{Prefix: "Docker Compose"}).Infof("Using service '%s' as the main web service", mainServiceName)
	} else {
		mainServiceName = render.GenerateServiceSelection(servicesWithPorts, "")
	}
	
	selectedService := compose.Services[mainServiceName]
	mainService := &selectedService

	// Get app details
	appName := render.GenerateTextQuestion("Please enter your app url friendly app name", mainServiceName, "will identify your app containers")
	
	// Ask for port - similar to Dockerfile approach
	suggestedPort := utils.ExtractPortFromService(mainService)
	if suggestedPort == "" {
		suggestedPort = "3000" // Default fallback
	}
	appPort := render.GenerateTextQuestion("Please enter the port at which the app receives requests", suggestedPort, "")

	appDomain := render.GenerateTextQuestion("Please enter the domain to point the app to", fmt.Sprintf("%s.%s.sslip.io", appName, viper.Get("serverAddress").(string)), "must point to your VPS address")
	envFileName := render.GenerateTextQuestion("Please enter which env file you would like to load", ".env", "")

	// Handle environment file
	hasEnvFile := false
	envFileChecksum := ""
	if utils.FileExists(fmt.Sprintf("./%s", envFileName)) {
		hasEnvFile = true
		render.GetLogger(log.Options{Prefix: "Env File"}).Infof("Detected - Loading env vars from %s", envFileName)
		dockerEnvProperty := []string{}
		envHandleErr := utils.HandleEnvFile(envFileName, &dockerEnvProperty, &envFileChecksum)
		if envHandleErr != nil {
			render.GetLogger(log.Options{Prefix: "Env File"}).Fatalf("Something went wrong %s", envHandleErr)
		}
		defer os.Remove("encrypted.env")
	} else {
		render.GetLogger(log.Options{Prefix: "Env File"}).Info("Not Detected - Skipping env parsing")
	}

	// Add Traefik labels to main service if not present
	if !hasTraefikLabels(mainService) {
		if mainService.Labels == nil {
			mainService.Labels = []string{}
		}
		mainService.Labels = append(mainService.Labels,
			"traefik.enable=true",
			fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", appName, appDomain),
			fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", appName, appPort),
			fmt.Sprintf("traefik.http.routers.%s.tls=true", appName),
			fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=default", appName),
			"traefik.docker.network=sidekick",
		)
	}

	// Ensure sidekick network is added
	if compose.Networks == nil {
		compose.Networks = make(map[string]utils.DockerNetwork)
	}
	compose.Networks["sidekick"] = utils.DockerNetwork{External: true}

	// Add sidekick network to all services
	for name, service := range compose.Services {
		if service.Networks == nil {
			service.Networks = []string{}
		}
		if !contains(service.Networks, "sidekick") {
			service.Networks = append(service.Networks, "sidekick")
		}
		compose.Services[name] = service
	}

	// Write updated compose file
	updatedComposeFile := fmt.Sprintf("sidekick-%s", composeFile)
	composeData, err := yaml.Marshal(&compose)
	if err != nil {
		render.GetLogger(log.Options{Prefix: "Docker Compose"}).Fatalf("Failed to marshal compose file: %s", err)
	}
	err = os.WriteFile(updatedComposeFile, composeData, 0644)
	if err != nil {
		render.GetLogger(log.Options{Prefix: "Docker Compose"}).Fatalf("Failed to write updated compose file: %s", err)
	}
	defer os.Remove(updatedComposeFile)

	// Prepare stages for TUI
	stages := []render.Stage{
		render.MakeStage("Validating connection with VPS", "VPS is reachable", false),
	}

	// Add build stages for each service with build context
	imageMap := make(map[string]string)
	for serviceName := range buildServices {
		imageName := fmt.Sprintf("%s-%s", appName, serviceName)
		imageMap[serviceName] = imageName
		stages = append(stages, render.MakeStage(
			fmt.Sprintf("Building docker image for service: %s", serviceName),
			fmt.Sprintf("Image built for %s", serviceName),
			true,
		))
	}

	stages = append(stages,
		render.MakeStage("Saving docker images locally", "Images saved successfully", false),
		render.MakeStage("Moving images to your server", "Images moved and loaded successfully", false),
		render.MakeStage("Setting up your application", "Application setup successfully", false),
	)

	p := tea.NewProgram(render.TuiModel{
		Stages:      stages,
		BannerMsg:   "Launching your docker-compose application on your VPS 🚀",
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

		// Build each service with build context
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

			// Build the image
			dockerBuildCmd := exec.Command("docker", append([]string{
				"build",
				"--tag", imageName,
				"--progress=plain",
				"--platform=linux/amd64",
				"-f", filepath.Join(buildContext, dockerfile),
			}, append(buildArgs, buildContext)...)...)
			
			dockerBuildCmdErrPipe, _ := dockerBuildCmd.StderrPipe()
			go render.SendLogsToTUI(dockerBuildCmdErrPipe, p)

			if dockerBuildErr := dockerBuildCmd.Run(); dockerBuildErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Failed to build %s: %v", serviceName, dockerBuildErr)})
			}

			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})
			stageIndex++
		}

		// Update compose file with built image names
		for serviceName, imageName := range imageMap {
			if service, exists := compose.Services[serviceName]; exists {
				service.Image = imageName
				service.Build = nil // Remove build section since we're using pre-built images
				compose.Services[serviceName] = service
			}
		}

		// Write final compose file for deployment
		finalComposeData, _ := yaml.Marshal(&compose)
		os.WriteFile(updatedComposeFile, finalComposeData, 0644)

		// Save images
		if len(imageMap) > 0 {
			imgFileName := fmt.Sprintf("%s-images.tar", appName)
			
			// Build docker save command with all images
			var imageNames []string
			for _, imageName := range imageMap {
				imageNames = append(imageNames, imageName)
			}
			
			imgSaveCmd := exec.Command("docker", append([]string{"save", "-o", imgFileName}, imageNames...)...)
			imgSaveCmdErrPipe, _ := imgSaveCmd.StderrPipe()
			go render.SendLogsToTUI(imgSaveCmdErrPipe, p)

			if imgSaveCmdErr := imgSaveCmd.Run(); imgSaveCmdErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Failed to save images: %v", imgSaveCmdErr)})
			}
			defer os.Remove(imgFileName)

			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			// Transfer images
			_, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("mkdir -p %s", appName))
			if sessionErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: sessionErr.Error()})
			}

			remoteDist := fmt.Sprintf("%s@%s:./%s", "sidekick", viper.GetString("serverAddress"), appName)
			imgMoveCmd := exec.Command("scp", "-C", imgFileName, remoteDist)
			imgMoveCmdErrorPipe, _ := imgMoveCmd.StderrPipe()
			go render.SendLogsToTUI(imgMoveCmdErrorPipe, p)

			if imgMovCmdErr := imgMoveCmd.Run(); imgMovCmdErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Failed to transfer images: %v", imgMovCmdErr)})
			}

			// Load images on server
			dockerLoadOutChan, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s && rm %s", appName, imgFileName, imgFileName))
			go func() {
				for line := range dockerLoadOutChan {
					p.Send(render.LogMsg{LogLine: line + "\n"})
					time.Sleep(time.Millisecond * 50)
				}
			}()
			if sessionErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: sessionErr.Error()})
			}
		} else {
			// Skip to next stage if no images to build
			p.Send(render.NextStageMsg{})
		}

		time.Sleep(time.Millisecond * 100)
		p.Send(render.NextStageMsg{})

		// Transfer compose file
		rsyncCmd := exec.Command("rsync", updatedComposeFile, fmt.Sprintf("%s@%s:%s/%s", "sidekick", viper.GetString("serverAddress"), appName, "docker-compose.yml"))
		if rsyncCmErr := rsyncCmd.Run(); rsyncCmErr != nil {
			p.Send(render.ErrorMsg{ErrorStr: rsyncCmErr.Error()})
		}

		// Transfer env file if exists
		if hasEnvFile {
			encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.GetString("serverAddress"), fmt.Sprintf("./%s", appName)))
			if encryptSyncErr := encryptSync.Run(); encryptSyncErr != nil {
				p.Send(render.ErrorMsg{ErrorStr: encryptSyncErr.Error()})
			}

			// Run with encrypted env
			runAppCmdOutChan, _, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && export SOPS_AGE_KEY=%s && sops exec-env encrypted.env 'docker compose -p %s up -d'`, appName, viper.GetString("secretKey"), appName))
			go func() {
				for line := range runAppCmdOutChan {
					p.Send(render.LogMsg{LogLine: line + "\n"})
					time.Sleep(time.Millisecond * 50)
				}
			}()
			if sessionErr1 != nil {
				p.Send(render.ErrorMsg{ErrorStr: sessionErr1.Error()})
			}
		} else {
			// Run without env
			runAppCmdOutChan, _, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && docker compose -p %s up -d`, appName, appName))
			go func() {
				for line := range runAppCmdOutChan {
					p.Send(render.LogMsg{LogLine: line + "\n"})
					time.Sleep(time.Millisecond * 50)
				}
			}()
			if sessionErr1 != nil {
				p.Send(render.ErrorMsg{ErrorStr: sessionErr1.Error()})
			}
		}

		// Save config
		portNumber, _ := strconv.ParseUint(appPort, 0, 64)
		envConfig := utils.SidekickAppEnvConfig{}
		if hasEnvFile {
			envConfig.File = envFileName
			envConfig.Hash = envFileChecksum
		}

		sidekickAppConfig := utils.SidekickAppConfig{
			Name:           appName,
			Version:        "V1",
			Port:           portNumber,
			Url:            appDomain,
			CreatedAt:      time.Now().Format(time.UnixDate),
			DeploymentType: "compose",
			ComposeFile:    composeFile,
			MainService:    mainServiceName,
			Env:            envConfig,
		}
		ymlData, _ := yaml.Marshal(&sidekickAppConfig)
		os.WriteFile("./sidekick.yml", ymlData, 0644)
		
		p.Send(render.AllDoneMsg{Duration: time.Since(start).Round(time.Second), URL: appDomain})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func hasTraefikLabels(service *utils.DockerService) bool {
	if service.Labels == nil {
		return false
	}
	for _, label := range service.Labels {
		if strings.HasPrefix(label, "traefik.") {
			return true
		}
	}
	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}