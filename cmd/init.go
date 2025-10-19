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

var InitCmd = &cobra.Command{
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

		port, portFlagErr := cmd.Flags().GetString("ssh-port")
		if portFlagErr != nil {
			fmt.Println(portFlagErr)
			port = "22"
		}

		certEmail, emailFlagError := cmd.Flags().GetString("email")
		if emailFlagError != nil {
			fmt.Println(emailFlagError)
		}
		sshProvider, sshProviderFlagErr := cmd.Flags().GetString("ssh-provider")
		if sshProviderFlagErr != nil {
			fmt.Println(sshProviderFlagErr)
			sshProvider = "openssh"
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

		wasPortProvided := cmd.Flags().Changed("ssh-port")
		if !wasPortProvided {
			portTextInput := pterm.DefaultInteractiveTextInput
			portTextInput.DefaultText = "Please enter the SSH port of your VPS (default: 22)"
			userPort, _ := portTextInput.Show()
			if userPort != "" {
				port = userPort
			}
		}
		pterm.Info.Printfln("Using SSH port: %s", port)

		if certEmail == "" {
			certEmailTextInput := pterm.DefaultInteractiveTextInput
			certEmailTextInput.DefaultText = "Please enter an email for use with TLS certs"
			certEmail, _ = certEmailTextInput.Show()
			if certEmail == "" {
				pterm.Error.Println("An email is needed before you proceed")
				os.Exit(0)
			}
		}

		wasProviderProvided := cmd.Flags().Changed("ssh-provider")
		if !wasProviderProvided {
			sshProviderTextInput := pterm.DefaultInteractiveTextInput
			sshProviderTextInput.DefaultText = "Please enter the SSH provider you want to use (default: openSSH, options: 1password, openssh)"
			userProvider, _ := sshProviderTextInput.Show()
			if userProvider != "" {
				sshProvider = userProvider
			}
		}
		pterm.Info.Printfln("Using SSH provider: %s", sshProvider)

		// if keys exist -> a server is already setup
		publicKey := viper.GetString("publicKey")
		secretKey := viper.GetString("secretKey")
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
		viper.Set("sshProvider", sshProvider)
		viper.Set("sshPort", port)

		pterm.Println()
		pterm.DefaultHeader.WithFullWidth().Println("Sidekick booting up! 🚀")
		pterm.Println()

		var sshClient *ssh.Client
		var initSessionErr error
		var loggedInUser string
		users := []string{"root", "sidekick"}
		canConnect := false
		for _, user := range users {
			sshClient, initSessionErr = utils.Login(server, user, sshProvider, port)
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

		_, sopsErr := exec.LookPath("sops")
		if sopsErr != nil {
			_, err := exec.LookPath("brew")
			if err != nil {
				log.Fatalf("Failed to run brew. Brew is required to use Sidekick: %s", err)
			}
			installSopsCmd := exec.Command("brew", "install", "sops")
			_, installSopsCmdErr := installSopsCmd.CombinedOutput()
			if installSopsCmdErr != nil {
				log.Fatalf("Failed to install Sops. Sops is needed to encrypt your local env: %s", installSopsCmdErr)
			}
		}
		_, ageErr := exec.LookPath("age")
		if ageErr != nil {
			// log.Println("Age not found, installing Age")
			_, err := exec.LookPath("brew")
			if err != nil {
				log.Fatalf("Failed to run brew. Brew is required to use Sidekick: %s", err)
			}
			installAgeCmd := exec.Command("brew", "install", "age")
			_, installAgeCmdErr := installAgeCmd.CombinedOutput()
			if installAgeCmdErr != nil {
				log.Fatalf("Failed to install Age. Age is needed to encrypt your local env: %s", installAgeCmd)
			}
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
		sidekickSshClient, err := utils.Login(server, "sidekick", sshProvider, port)
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

		if publicKey == "" || secretKey == "" {
			ageKeygenCmd := exec.Command("age-keygen")
			cmdOutput, ageKeygenCmdErr := ageKeygenCmd.Output()
			if ageKeygenCmdErr != nil {
				log.Fatalf("Failed to run brew. Brew is required to use Sidekick: %s", ageKeygenCmd)
				os.Exit(1)
			}
			ageKeyOut := string(cmdOutput)
			outputSlice := strings.Split(ageKeyOut, "\n")

			secretKey = outputSlice[2]
			publicKey = strings.ReplaceAll(strings.Split(outputSlice[1], ":")[1], " ", "")
		}
		viper.Set("publicKey", publicKey)
		viper.Set("secretKey", secretKey)

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
	rootCmd.AddCommand(InitCmd)

	InitCmd.Flags().StringP("server", "s", "", "Set the IP address of your Server")
	InitCmd.Flags().StringP("email", "e", "", "An email address to be used for SSL certs")
	InitCmd.Flags().StringP("ssh-port", "p", "22", "SSH port for connecting to the server")
	InitCmd.Flags().String("ssh-provider", "openssh", "SSH provider to use (openssh or 1password)")
	InitCmd.Flags().BoolP("yes", "y", false, "Skip all validation prompts")
}
