/*
Copyright © 2024 Mahmoud Mosua <m.mousa@hey.com>

Licensed under the GNU AGPL License, Version 3.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.gnu.org/licenses/agpl-3.0.en.html

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Init sidekick CLI and configure your VPS to host your apps",
	Long: `This command will run you throgh the setup steps to get sidekick loaded on your VPS.
		You wil neede to provide your VPS IPv4 address and a registery to host your docker images.
		`,
	Run: func(cmd *cobra.Command, args []string) {
		pterm.DefaultBasicText.Println("Welcome to Sidekick. We need to collect some details from you first")

		render.RenderSidekickBig()

		// get server address
		server := ""
		serverTextInput := pterm.DefaultInteractiveTextInput
		serverTextInput.DefaultText = "Please enter the IPv4 Address of your VPS"
		server, _ = serverTextInput.Show()
		if !utils.IsValidIPAddress(server) {
			pterm.Error.Printfln("You entered an incorrect IP Address - %s", server)
			os.Exit(0)
		}

		certEmail := ""
		certEmailTextInput := pterm.DefaultInteractiveTextInput
		certEmailTextInput.DefaultText = "Please enter an email for use with TLS certs"
		certEmail, _ = certEmailTextInput.Show()
		if certEmail == "" {
			pterm.Error.Println("An email is needed befoer you proceed")
			os.Exit(0)
		}

		dockerRegistery := ""
		dockerRegisteryTextInput := pterm.DefaultInteractiveTextInput.WithDefaultValue("docker.io")
		dockerRegisteryTextInput.DefaultText = "Please enter your docker registery"
		dockerRegistery, _ = dockerRegisteryTextInput.Show()

		dockerUsername := ""
		dokerUsernameTextInput := pterm.DefaultInteractiveTextInput
		dokerUsernameTextInput.DefaultText = "Please enter your docker username for the registery"
		dockerUsername, _ = dokerUsernameTextInput.Show()
		if dockerUsername == "" {
			pterm.Error.Println("You have to enter your docker username")
			os.Exit(0)
		}

		prompt := pterm.DefaultInteractiveContinue
		prompt.DefaultText = "Are you logged in to the docker registery?"
		prompt.Options = []string{"yes", "no"}
		if result, _ := prompt.Show(); result != "yes" {
			pterm.Println()
			pterm.Error.Printfln("You need to login to your docker registery %s", dockerRegistery)
			pterm.Info.Printfln("You can do so by running `docker login %s`", dockerRegistery)
			pterm.Println()
			os.Exit(0)
		}

		// init the sidekick system config && add public key to known_hosts
		preludeCmd := exec.Command("sh", "-s", "-")
		preludeCmd.Stdin = strings.NewReader(utils.PreludeScript)
		if preludeCmdErr := preludeCmd.Run(); preludeCmdErr != nil {
			panic(preludeCmdErr)
		}

		// setup viper for config
		viper.SetConfigName("default")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME/.config/sidekick/")
		viper.Set("serverAddress", server)
		viper.Set("dockerRegistery", dockerRegistery)
		viper.Set("dockerUsername", dockerUsername)
		viper.Set("certEmail", certEmail)

		pterm.Println()
		pterm.DefaultHeader.WithFullWidth().Println("Sidekick booting up! 🚀")
		pterm.Println()

		// init login with checking handshake
		rootSshClient, err := utils.Login(server, "root")
		if err != nil {
			log.Fatalf("Unable to login using 'root' user: %s", err)
			os.Exit(1)
		}

		multi := pterm.DefaultMultiPrinter
		setupProgressBar, _ := pterm.DefaultProgressbar.WithTotal(6).WithWriter(multi.NewWriter()).Start("Sidekick Booting up (2m estimated)  ")
		rootLoginSpinner, _ := pterm.DefaultSpinner.Start("Logging in with root")
		stage0Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Adding user Sidekick")
		sidekickLoginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging into with sidekick user")
		stage1Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up VPS")
		stage2Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up Docker")
		stage3Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up Traefik")
		pterm.Println()
		multi.Start()

		rootLoginSpinner.Success("Logged in successfully!")
		setupProgressBar.Increment()

		stage0Spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		if err := utils.RunStage(rootSshClient, utils.UsersetupStage); err != nil {
			stage0Spinner.Fail(utils.UsersetupStage.SpinnerFailMessage)
			panic(err)
		}
		stage0Spinner.Success(utils.UsersetupStage.SpinnerSuccessMessage)
		setupProgressBar.Increment()

		sidekickLoginSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		sidekickSshClient, err := utils.Login(server, "sidekick")
		if err != nil {
			sidekickLoginSpinner.Fail("Something went wrong logging in to your VPS")
			panic(err)
		}
		sidekickLoginSpinner.Success("Logged in successfully with new user!")
		setupProgressBar.Increment()

		stage1Spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		if err := utils.RunStage(sidekickSshClient, utils.SetupStage); err != nil {
			stage1Spinner.Fail(utils.SetupStage.SpinnerFailMessage)
			panic(err)
		}
		ch, sessionErr := utils.RunCommand(sidekickSshClient, "mkdir -p $HOME/.config/sops/age/ && age-keygen -o $HOME/.config/sops/age/keys.txt 2>&1 ")
		if sessionErr != nil {
			stage1Spinner.Fail(utils.SetupStage.SpinnerFailMessage)
			panic(sessionErr)
		}
		select {
		case output := <-ch:
			if strings.HasPrefix(output, "Public key") {
				publicKey := strings.Split(output, " ")[2:3]
				viper.Set("publicKey", publicKey[0])
				viper.WriteConfig()
			}
		}
		stage1Spinner.Success(utils.SetupStage.SpinnerSuccessMessage)
		setupProgressBar.Increment()

		stage2Spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		if err := utils.RunStage(sidekickSshClient, utils.DockerStage); err != nil {
			stage2Spinner.Fail(utils.DockerStage.SpinnerFailMessage)
			panic(err)
		}
		stage2Spinner.Success(utils.DockerStage.SpinnerSuccessMessage)
		setupProgressBar.Increment()

		stage3Spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		traefikStage := utils.GetTraefikStage(certEmail)
		if err := utils.RunStage(sidekickSshClient, traefikStage); err != nil {
			stage3Spinner.Fail(traefikStage.SpinnerFailMessage)
			panic(err)
		}
		stage3Spinner.Success(traefikStage.SpinnerSuccessMessage)
		setupProgressBar.Increment()

		if err := viper.WriteConfig(); err != nil {
			panic(err)
		}

		multi.Stop()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// initCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
