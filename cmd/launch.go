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
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var launchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch a new application to host on your VPS with Sidekick",
	Long:  `This command will run you through the basic setup to add a new application to your VPS.`,
	Run: func(cmd *cobra.Command, args []string) {
		if configErr := utils.ViperInit(); configErr != nil {
			pterm.Error.Println("Sidekick config not found - Run sidekick init")
			os.Exit(1)
		}

		if utils.FileExists("./sidekick.yml") {
			pterm.Error.Println("Sidekick config exits in this project.")
			pterm.Info.Println("You can deploy a new version of your application with Sidekick deploy.")
			os.Exit(1)
		}

		if utils.FileExists("./Dockerfile") {
			pterm.Info.Println("Dockerfile detected - scanning file for details")
		} else {
			pterm.Error.Println("No dockerfiles found in current directory.")
			os.Exit(1)
		}
		pterm.Info.Println("Analyzing docker file...")
		res, err := os.ReadFile("./Dockerfile")
		if err != nil {
			pterm.Error.Println("Unable to process your dockerfile")
		}
		// attempt to get a port from dockerfile
		appPort := ""
		for _, line := range strings.Split(string(res), "\n") {
			if strings.HasPrefix(line, "EXPOSE") {
				appPort = line[len(line)-4:]
			}
		}

		appName := ""
		appNameTextInput := pterm.DefaultInteractiveTextInput
		appNameTextInput.DefaultText = "Please enter your app url friendly app name"
		appName, _ = appNameTextInput.Show()
		if appName == "" || strings.Contains(appName, " ") {
			pterm.Error.Println("You have to enter url friendly app name")
			os.Exit(0)
		}

		appPortTextInput := pterm.DefaultInteractiveTextInput.WithDefaultValue(appPort)
		appPortTextInput.DefaultText = "Please enter the port at which the app receives request"
		appPort, _ = appPortTextInput.Show()
		if appPort == "" {
			pterm.Error.Println("You you have to enter a port to accept requests")
			os.Exit(0)
		}

		appDomain := ""
		appDomainTextInput := pterm.DefaultInteractiveTextInput.WithDefaultValue(fmt.Sprintf("%s.%s.sslip.io", appName, viper.Get("serverAddress").(string)))
		appDomainTextInput.DefaultText = "Please enter the domain to point the app to"
		appDomain, _ = appDomainTextInput.Show()

		envFileName := ""
		envFileNameTextInput := pterm.DefaultInteractiveTextInput.WithDefaultValue(".env")
		envFileNameTextInput.DefaultText = "Please enter which env file you would like to load"
		envFileName, _ = envFileNameTextInput.Show()

		hasEnvFile := false
		envVariables := []string{}
		dockerEnvProperty := []string{}
		envFileChecksum := ""
		if utils.FileExists(fmt.Sprintf("./%s", envFileName)) {
			hasEnvFile = true
			pterm.Info.Printfln("Env file detected - Loading env vars from %s", envFileName)
			utils.HandleEnvFile(envFileName, envVariables, &dockerEnvProperty, &envFileChecksum)
			defer os.Remove("encrypted.env")
		} else {
			pterm.Info.Println("No env file detected - Skipping env parsing")
		}

		// make a docker service
		imageName := appName
		newService := utils.DockerService{
			Image: imageName,
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

		pterm.Println()
		pterm.DefaultHeader.WithFullWidth().Println("Let's launch your app! 🚀")
		pterm.Println()

		multi := pterm.DefaultMultiPrinter
		loginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging into VPS")
		dockerBuildSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Preparing docker image")
		setupSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up application")

		multi.Start()

		loginSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		sshClient, err := utils.Login(viper.Get("serverAddress").(string), "sidekick")
		if err != nil {
			loginSpinner.Fail("Something went wrong logging in to your VPS")
			panic(err)
		}
		loginSpinner.Success("Logged in successfully!")

		dockerBuildSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		cwd, _ := os.Getwd()
		dockerBuildCommd := exec.Command("sh", "-s", "-", appName, cwd)
		dockerBuildCommd.Stdin = strings.NewReader(utils.DockerBuildAndSaveScript)
		// better handle of errors -> Push it to another writer aside from os.stderr and then flush it when it panics
		if dockerBuildErr := dockerBuildCommd.Run(); dockerBuildErr != nil {
			log.Fatalln("Failed to run docker")
			os.Exit(1)
		}
		_, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("mkdir %s", appName))
		if sessionErr != nil {
			panic(sessionErr)
		}

		imgMoveCmd := exec.Command("sh", "-s", "-", appName, "sidekick", viper.GetString("serverAddress"))
		imgMoveCmd.Stdin = strings.NewReader(utils.ImageMoveScript)
		_, imgMoveErr := imgMoveCmd.Output()
		if imgMoveErr != nil {
			log.Fatalf("Issue occured with moving image to your VPS: %s", imgMoveErr)
			os.Exit(1)
		}
		if _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && docker load -i %s-latest.tar", appName, appName)); sessionErr != nil {
			log.Fatal("Issue happened loading docker image")
		}

		dockerBuildSpinner.Success("Successfully built and moved docker image to your VPS")

		setupSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		rsync := exec.Command("rsync", "docker-compose.yaml", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appName)))
		rsync.Run()
		if hasEnvFile {
			encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appName)))
			encryptSync.Run()

			_, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && sops exec-env encrypted.env 'docker compose -p sidekick up -d'`, appName))
			if sessionErr1 != nil {
				fmt.Println("something went wrong")
			}
		} else {
			_, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && docker compose -p sidekick up -d`, appName))
			if sessionErr1 != nil {
				panic(sessionErr1)
			}
		}
		if _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("cd %s && rm %s", appName, fmt.Sprintf("%s-latest.tar", appName))); sessionErr != nil {
			log.Fatal("Issue happened cleaning up the image file")
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
		ymlData, err := yaml.Marshal(&sidekickAppConfig)
		os.WriteFile("./sidekick.yml", ymlData, 0644)

		setupSpinner.Success("🙌 App setup successfully 🙌")
		multi.Stop()

		pterm.Println()
		pterm.Info.Printfln("😎 Access your app at: https://%s", appDomain)
		pterm.Println()

	},
}

func init() {
	rootCmd.AddCommand(launchCmd)
}
