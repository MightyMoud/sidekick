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

/*
This file is just a scratchpad for my thoughts.
Please ignore this file.
No I don't know how to use branches.
Send Help.
*/

var composeCmd = &cobra.Command{
	Use:   "compose",
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

		// keyAddSshCommand := exec.Command("sh", "-s", "-", viper.Get("serverAddress").(string))
		// keyAddSshCommand.Stdin = strings.NewReader(utils.SshKeysScript)
		// if sshAddErr := keyAddSshCommand.Run(); sshAddErr != nil {
		// 	panic(sshAddErr)
		// }

		if utils.FileExists("./docker-compose.yaml") {
			pterm.Info.Println("Docker compose file detected - scanning file for details")
		} else {
			pterm.Error.Println("No Docker compose files found in current directory.")
			os.Exit(1)
		}
		pterm.Info.Println("Analyzing Compose file...")
		res, err := os.ReadFile("./docker-compose.yaml")
		if err != nil {
			pterm.Error.Println("Unable to process your compose file")
		}
		var compose utils.DockerComposeFile
		err = yaml.Unmarshal(res, &compose)
		if err != nil {
			log.Fatalf("error unmarshalling yaml: %v", err)
		}

		composeServices := []string{}
		for name := range compose.Services {
			composeServices = append(composeServices, name)
		}
		httpServiceName, _ := pterm.DefaultInteractiveSelect.WithOptions(composeServices).WithDefaultText("Please select which service in your docker compose file will recieve http requests").Show()

		otherServices := []string{}
		for _, v := range composeServices {
			if v != httpServiceName {
				otherServices = append(otherServices, v)
			}
		}

		// attempt to get a port from dockerfile
		appPort := ""
		appPortTextInput := pterm.DefaultInteractiveTextInput.WithDefaultValue(appPort)
		appPortTextInput.DefaultText = "Please enter the port at which the app receives requests"
		appPort, _ = appPortTextInput.Show()
		if appPort == "" {
			pterm.Error.Println("You you have to enter a port to accept requests")
			os.Exit(0)
		}

		appName := ""
		appNameTextInput := pterm.DefaultInteractiveTextInput
		appNameTextInput.DefaultText = "Please enter your app url friendly app name"
		appName, _ = appNameTextInput.Show()
		if appName == "" || strings.Contains(appName, " ") {
			pterm.Error.Println("You have to enter url friendly app name")
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
			res := utils.HandleEnvFile(envFileName, envVariables, &dockerEnvProperty, &envFileChecksum)
			fmt.Println(res)
			fmt.Println(dockerEnvProperty)
			defer os.Remove("encrypted.env")
		} else {
			pterm.Info.Println("No env file detected - Skipping env parsing")
		}
		// make a docker service
		imageName := fmt.Sprintf("%s/%s", viper.Get("dockerUsername").(string), appName)
		currentService := compose.Services[httpServiceName]
		currentService.Image = imageName
		currentService.Labels = []string{
			"traefik.enable=true",
			fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", appName, appDomain),
			fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", appName, appPort),
			fmt.Sprintf("traefik.http.routers.%s.tls=true", appName),
			fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=default", appName),
			"traefik.docker.network=sidekick",
		}
		currentService.Environment = dockerEnvProperty
		currentService.Networks = []string{
			"sidekick",
		}
		currentService.DependsOn = otherServices
		compose.Services[httpServiceName] = currentService

		compose.Networks =
			map[string]utils.DockerNetwork{
				"sidekick": {
					External: true,
				},
			}

		dockerComposeFile, err := yaml.Marshal(&compose)
		if err != nil {
			fmt.Printf("Error marshalling YAML: %v\n", err)
			return
		}
		err = os.WriteFile("docker-compose.yaml", dockerComposeFile, 0644)
		if err != nil {
			fmt.Printf("Error writing file: %v\n", err)
			return
		}
		os.Exit(2)

		multi := pterm.DefaultMultiPrinter
		launchPb, _ := pterm.DefaultProgressbar.WithTotal(3).WithWriter(multi.NewWriter()).Start("Booting up app on VPS")
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
		launchPb.Increment()

		dockerBuildSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		cwd, _ := os.Getwd()
		dockerBuildCommd := exec.Command("sh", "-s", "-", appName, viper.Get("dockerUsername").(string), cwd)
		dockerBuildCommd.Stdin = strings.NewReader(utils.DockerBuildAndSaveScript)
		// better handle of errors -> Push it to another writer aside from os.stderr and then flush it when it panics
		if dockerBuildErr := dockerBuildCommd.Run(); dockerBuildErr != nil {
			log.Fatalln("Failed to run docker")
			os.Exit(1)
		}
		dockerBuildSpinner.Success("Successfully built and pushed docker image")
		launchPb.Increment()

		setupSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		_, _, sessionErr := utils.RunCommand(sshClient, fmt.Sprintf("mkdir %s", appName))
		if sessionErr != nil {
			panic(sessionErr)
		}
		rsync := exec.Command("rsync", "docker-compose.yaml", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appName)))
		rsync.Run()
		if hasEnvFile {
			encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s", appName)))
			encryptSync.Run()

			_, _, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && sops exec-env encrypted.env 'docker compose -p sidekick up -d'`, appName))
			if sessionErr1 != nil {
				fmt.Println("something went wrong")
			}
		} else {
			_, _, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s && docker compose -p sidekick up -d`, appName))
			if sessionErr1 != nil {
				panic(sessionErr1)
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
			Image:     fmt.Sprintf("%s/%s", viper.Get("dockerUsername"), appName),
			Port:      portNumber,
			Url:       appDomain,
			CreatedAt: time.Now().Format(time.UnixDate),
			Env:       envConfig,
		}
		ymlData, err := yaml.Marshal(&sidekickAppConfig)
		os.WriteFile("./sidekick.yml", ymlData, 0644)
		launchPb.Increment()

		setupSpinner.Success("🙌 App setup successfully 🙌")
		multi.Stop()

		pterm.Println()
		pterm.Info.Printfln("😎 Access your app at: https://%s", appDomain)
		pterm.Println()

	},
}

func init() {
}
