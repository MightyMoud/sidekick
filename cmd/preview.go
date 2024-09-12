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
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/ms-mousa/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// previewCmd represents the preview command
var previewCmd = &cobra.Command{
	Use:   "preview",
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
		appConfig, appConfigErr := utils.LoadAppConfig()
		if appConfigErr != nil {
			log.Fatalln("Unable to load your config file. Might be corrupted")
			os.Exit(1)
		}

		gitTreeCheck := exec.Command("sh", "-s", "-")
		gitTreeCheck.Stdin = strings.NewReader(utils.CheckGitTreeScript)
		output, _ := gitTreeCheck.Output()
		if string(output) != "all good\n" {
			fmt.Println(string(output))
			pterm.Error.Println("Please commit any changes to git before deploying a preview environment")
			os.Exit(1)
		}

		gitShortHashCmd := exec.Command("sh", "-s", "-")
		gitShortHashCmd.Stdin = strings.NewReader("git rev-parse --short HEAD")
		hashOutput, hashErr := gitShortHashCmd.Output()
		if hashErr != nil {
			panic(hashErr)
		}
		deployHash := string(hashOutput)
		deployHash = strings.TrimSuffix(deployHash, "\n")

		multi := pterm.DefaultMultiPrinter
		setupProgressBar, _ := pterm.DefaultProgressbar.WithTotal(3).WithWriter(multi.NewWriter()).Start("Deploying your preview env")
		loginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging into VPS")
		dockerBuildStageSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Building latest docker image of your app")
		deployStageSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Deploying a preview env of your application")

		multi.Start()

		loginSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		sshClient, err := utils.Login(viper.Get("serverAddress").(string), "sidekick")
		if err != nil {
			loginSpinner.Fail("Something went wrong logging in to your VPS")
			panic(err)
		}
		loginSpinner.Success("Logged in successfully!")
		setupProgressBar.Increment()

		dockerBuildStageSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}

		envVariables := []string{}
		dockerEnvProperty := []string{}
		envFileChecksum := ""
		if appConfig.Env.File != "" {
			envErr := utils.HandleEnvFile(appConfig.Env.File, envVariables, dockerEnvProperty, &envFileChecksum)
			if envErr != nil {
				panic(envErr)
			}
		}
		defer os.Remove("encrypted.env")

		imageName := fmt.Sprintf("%s/%s:%s", viper.Get("dockerUsername").(string), appConfig.Name, deployHash)
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
		defer os.Remove("docker-compose.yaml")

		cwd, _ := os.Getwd()
		dockerBuildCommd := exec.Command("sh", "-s", "-", appConfig.Name, viper.Get("dockerUsername").(string), cwd, deployHash)
		dockerBuildCommd.Stdin = strings.NewReader(utils.DockerHandleScript)
		// better handle of errors -> Push it to another writer aside from os.stderr and then flush it when it panics
		if dockerBuildErr := dockerBuildCommd.Run(); dockerBuildErr != nil {
			log.Fatalln("Failed to run docker")
			os.Exit(1)
		}
		dockerBuildStageSpinner.Success("Successfully built and pushed docker image")

		deployStageSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		_, sessionErr0 := utils.RunCommand(sshClient, fmt.Sprintf(`mkdir -p %s/preview/%s`, appConfig.Name, deployHash))
		if sessionErr0 != nil {
			panic(sessionErr0)
		}
		rsync := exec.Command("rsync", "docker-compose.yaml", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s/preview/%s", appConfig.Name, deployHash)))
		rsync.Run()
		if appConfig.Env.File != "" {
			encryptSync := exec.Command("rsync", "encrypted.env", fmt.Sprintf("%s@%s:%s", "sidekick", viper.Get("serverAddress").(string), fmt.Sprintf("./%s/preview/%s", appConfig.Name, deployHash)))
			encryptSync.Run()

			_, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s/preview/%s && sops exec-env encrypted.env 'docker compose -p sidekick up -d'`, appConfig.Name, deployHash))
			if sessionErr1 != nil {
				panic(sessionErr1)
			}
		} else {
			_, sessionErr1 := utils.RunCommand(sshClient, fmt.Sprintf(`cd %s/preview/%s && docker compose -p sidekick up -d`, appConfig.Name, deployHash))
			if sessionErr1 != nil {
				panic(sessionErr1)
			}
		}
		previewEnvConfig := utils.SidekickPreview{
			Name:  serviceName,
			Url:   fmt.Sprintf("https://%s", previewURL),
			Image: imageName,
		}
		appConfig.PreviewEnvs = map[string]utils.SidekickPreview{
			deployHash: previewEnvConfig,
		}

		ymlData, err := yaml.Marshal(&appConfig)
		os.WriteFile("./sidekick.yml", ymlData, 0644)

		deployStageSpinner.Success("Successfully built and pushed docker image")
		multi.Stop()

		pterm.Println()
		pterm.Info.Printfln("😎 Access your app at: https://%s.%s", deployHash, appConfig.Url)
		pterm.Println()

	},
}

func init() {
	deployCmd.AddCommand(previewCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// previewCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// previewCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
