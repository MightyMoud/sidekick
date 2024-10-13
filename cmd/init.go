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
package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Init sidekick CLI and configure your VPS to host your apps",
	Long: `This command will run you through the setup steps to get sidekick loaded on your VPS.
		You wil need to provide your VPS IPv4 address and a registry to host your docker images.
		`,
	Run: func(cmd *cobra.Command, args []string) {
		pterm.DefaultBasicText.Println("Welcome to Sidekick. We need to collect some details from you first")

		render.RenderSidekickBig()

		if configErr := utils.ViperInit(); configErr != nil {
			if errors.As(configErr, &viper.ConfigFileNotFoundError{}) {
				initConfig()
			} else {
				pterm.Error.Printfln("%s", configErr)
				os.Exit(1)
			}
		}

		skipPromptsFlag, skipFlagErr := cmd.Flags().GetBool("yes")
		if skipFlagErr != nil {
			fmt.Println(skipFlagErr)
		}

		server, serverFlagErr := cmd.Flags().GetString("server")
		if serverFlagErr != nil {
			fmt.Println(serverFlagErr)
		}
		certEmail, emailFlagError := cmd.Flags().GetString("email")
		if emailFlagError != nil {
			fmt.Println(emailFlagError)
		}

		if server == "" {
			serverTextInput := pterm.DefaultInteractiveTextInput
			serverTextInput.DefaultText = "Please enter the IPv4 Address of your VPS"
			server, _ = serverTextInput.Show()
			if !utils.IsValidIPAddress(server) {
				pterm.Error.Printfln("You entered an incorrect IP Address - %s", server)
				os.Exit(0)
			}
		}

		if certEmail == "" {
			certEmailTextInput := pterm.DefaultInteractiveTextInput
			certEmailTextInput.DefaultText = "Please enter an email for use with TLS certs"
			certEmail, _ = certEmailTextInput.Show()
			if certEmail == "" {
				pterm.Error.Println("An email is needed before you proceed")
				os.Exit(0)
			}
		}

		// if public key exists -> a server is already setup
		publicKey := viper.GetString("publicKey")
		if publicKey != "" && server != viper.GetString("serverAddress") && !skipPromptsFlag {
			prompt := pterm.DefaultInteractiveConfirm
			prompt.DefaultText = "A server was previously setup with Sidekick. Would you like to override the settings?"
			result, showErr := prompt.Show()
			if showErr != nil {
				pterm.Error.Printfln("Something went wrong: %s", showErr)
				os.Exit(1)
			}
			if !result {
				pterm.Println()
				pterm.Error.Println("Currently Sidekick only supports one server per setup")
				pterm.Println()
				os.Exit(0)
			}
		}

		viper.Set("serverAddress", server)
		viper.Set("certEmail", certEmail)

		pterm.Println()
		pterm.DefaultHeader.WithFullWidth().Println("Sidekick booting up! 🚀")
		pterm.Println()

		var sshClient *ssh.Client
		var initSessionErr error
		var loggedInUser string
		users := []string{"root", "sidekick"}
		canConnect := false
		for _, user := range users {
			sshClient, initSessionErr = utils.Login(server, user)
			if initSessionErr != nil {
				continue
			}
			loggedInUser = user
			canConnect = true
			break
		}
		if !canConnect {
			pterm.Error.Println("Unable to establish SSH connection to the server")
			os.Exit(1)
		}

		multi := pterm.DefaultMultiPrinter
		localReqsChecks, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up your local env")
		rootLoginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging in with root")
		stage0Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Adding user Sidekick")
		sidekickLoginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging into with sidekick user")
		stage1Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up VPS")
		stage2Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up Docker")
		stage3Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up Traefik")
		pterm.Println()
		multi.Start()

		localReqsChecks.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		brewCheckCmd := exec.Command("brew", "-v")
		_, brewCheckCmdErr := brewCheckCmd.CombinedOutput()
		if brewCheckCmdErr != nil {
			log.Fatalf("Failed to run brew. Brew is required to use Sidekick: %s", brewCheckCmd)
			os.Exit(1)
		}

		installSopsCmd := exec.Command("brew", "install", "sops")
		_, installSopsCmdErr := installSopsCmd.CombinedOutput()
		if installSopsCmdErr != nil {
			log.Fatalf("Failed to install Sops. Sops is need to encrypt your local env: %s", installSopsCmd)
			os.Exit(1)
		}

		localReqsChecks.Success("Installed local requirements successfully")

		rootLoginSpinner.Success("Logged in successfully!")

		stage0Spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}

		hasSidekickUser := true
		outChan, _, sessionErr := utils.RunCommand(sshClient, "id -u sidekick")
		if sessionErr != nil {
			hasSidekickUser = false
		} else {
			select {
			case output := <-outChan:
				if output != "0" {
					hasSidekickUser = true
				}
			}
		}
		if !hasSidekickUser && loggedInUser == "root" {
			if err := utils.RunStage(sshClient, utils.UsersetupStage); err != nil {
				stage0Spinner.Fail(utils.UsersetupStage.SpinnerFailMessage)
				panic(err)
			}

		}
		stage0Spinner.Success(utils.UsersetupStage.SpinnerSuccessMessage)

		sidekickLoginSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		sidekickSshClient, err := utils.Login(server, "sidekick")
		if err != nil {
			sidekickLoginSpinner.Fail("Something went wrong logging in to your VPS")
			log.Fatal(err)
		}
		sidekickLoginSpinner.Success("Logged in successfully with new user!")

		stage1Spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		if err := utils.RunStage(sidekickSshClient, utils.SetupStage); err != nil {
			stage1Spinner.Fail(utils.SetupStage.SpinnerFailMessage)
			panic(err)
		}
		ageSetup := false
		outCh, _, sessionErr := utils.RunCommand(sshClient, `[ -f "$HOME/.config/sops/age/keys.txt" ] && echo "1" || echo "0"`)
		if sessionErr != nil {
			log.Fatal("issue with checking age")
		}

		select {
		case output := <-outCh:
			if output == "1" {
				ageSetup = true
			}
			break
		}
		if !ageSetup {
			ch, _, sessionErr := utils.RunCommand(sidekickSshClient, "mkdir -p $HOME/.config/sops/age/ && age-keygen -o $HOME/.config/sops/age/keys.txt 2>&1 ")
			if sessionErr != nil {
				stage1Spinner.Fail(utils.SetupStage.SpinnerFailMessage)
				panic(sessionErr)
			}
			select {
			case output := <-ch:
				if strings.HasPrefix(output, "Public key") {
					publicKey := strings.Split(output, " ")[2:3]
					viper.Set("publicKey", publicKey[0])
				}
			}

		}
		stage1Spinner.Success(utils.SetupStage.SpinnerSuccessMessage)

		stage2Spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		dockerReady := false
		dockOutCh, _, sessionErr := utils.RunCommand(sshClient, `command -v docker &> /dev/null && command -v docker compose &> /dev/null && echo "1" || echo "0"`)
		if sessionErr != nil {
			log.Fatal("Issue checking Docker")
		}

		select {
		case output := <-dockOutCh:
			if output == "1" {
				dockerReady = true
			}
			break
		}
		if !dockerReady {
			if err := utils.RunStage(sidekickSshClient, utils.DockerStage); err != nil {
				stage2Spinner.Fail(utils.DockerStage.SpinnerFailMessage)
				panic(err)
			}
		}
		stage2Spinner.Success(utils.DockerStage.SpinnerSuccessMessage)

		stage3Spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		traefikSetup := false
		trOutCh, _, sessionErr := utils.RunCommand(sshClient, `[ -d "sidekick-traefik" ] && echo "1" || echo "0"`)
		if sessionErr != nil {
			log.Fatal("Issue with checking folder traefik")
		}

		select {
		case output := <-trOutCh:
			if output == "1" {
				traefikSetup = true
			}
			break
		}
		traefikStage := utils.GetTraefikStage(certEmail)
		if !traefikSetup {
			if err := utils.RunStage(sidekickSshClient, traefikStage); err != nil {
				stage3Spinner.Fail(traefikStage.SpinnerFailMessage)
				panic(err)
			}
		}
		stage3Spinner.Success(traefikStage.SpinnerSuccessMessage)

		if err := viper.WriteConfig(); err != nil {
			panic(err)
		}
		multi.Stop()

		pterm.Println()
		pterm.Info.Println("Your VPS is ready! You can now run Sidekick launch in your app folder")
		pterm.Println()
	},
}

func initConfig() {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	configPath := fmt.Sprintf("%s/.config/sidekick", home)
	configFile := fmt.Sprintf("%s/default.yaml", configPath)

	makeDirErr := os.MkdirAll(configPath, os.ModePerm)
	if makeDirErr != nil {
		log.Fatalf("Error creating directory: %v\n", makeDirErr)
		os.Exit(1)
	}

	viper.AddConfigPath(configPath)
	viper.SetConfigType("yaml")
	viper.SetConfigName("default")
	file, fileCreateErr := os.Create(configFile)
	if fileCreateErr != nil {
		log.Fatalf("Error creating configFile: %v\n", fileCreateErr)
		os.Exit(1)
	}

	file.Close()
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringP("server", "s", "", "Set the IP address of your Server")
	initCmd.Flags().StringP("email", "e", "", "An email address to be used for SSL certs")
	initCmd.Flags().BoolP("yes", "y", false, "Skip all validation prompts")
}
