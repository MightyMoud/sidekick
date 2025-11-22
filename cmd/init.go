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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mightymoud/sidekick/render"
	"github.com/mightymoud/sidekick/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

func stage1LocalReqs(p *tea.Program) error {
	if _, err := exec.LookPath("sops"); err != nil {
		cmd := exec.Command("brew", "install", "sops")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install sops: %w", err)
		}
	}
	if _, err := exec.LookPath("age"); err != nil {
		cmd := exec.Command("brew", "install", "age")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install age: %w", err)
		}
	}
	return nil
}

func stage2Login(server string, p *tea.Program) (*ssh.Client, string, error) {
	users := []string{"root", "sidekick"}
	for _, user := range users {
		client, err := utils.Login(server, user)
		if err == nil {
			return client, user, nil
		}
	}
	return nil, "", fmt.Errorf("unable to establish SSH connection")
}

func stage3UserSetup(client *ssh.Client, loggedInUser string, p *tea.Program) error {
	hasSidekickUser := true
	outChan, _, err := utils.RunCommand(client, "id -u sidekick")
	if err != nil {
		hasSidekickUser = false
	} else {
		output := <-outChan
		if output == "" {
			hasSidekickUser = false
		}
	}

	if !hasSidekickUser && loggedInUser == "root" {
		if err := utils.RunStage(client, utils.UsersetupStage); err != nil {
			return err
		}
	}
	return nil
}

func stage4VPSSetup(client *ssh.Client, p *tea.Program) error {
	if err := utils.RunStage(client, utils.SetupStage); err != nil {
		return err
	}

	publicKey := viper.GetString("publicKey")
	secretKey := viper.GetString("secretKey")

	if publicKey == "" || secretKey == "" {
		cmd := exec.Command("age-keygen")
		output, err := cmd.Output()
		if err != nil {
			return err
		}
		outStr := string(output)
		lines := strings.Split(outStr, "\n")
		if len(lines) >= 3 {
			secretKey = lines[2]
			parts := strings.Split(lines[1], ":")
			if len(parts) > 1 {
				publicKey = strings.ReplaceAll(parts[1], " ", "")
			}
		}
		viper.Set("publicKey", publicKey)
		viper.Set("secretKey", secretKey)
	}
	return nil
}

func stage5Docker(client *ssh.Client, p *tea.Program) error {
	dockerReady := false
	outChan, _, err := utils.RunCommand(client, `command -v docker &> /dev/null && command -v docker compose &> /dev/null && echo "1" || echo "0"`)
	if err == nil {
		output := <-outChan
		if output == "1" {
			dockerReady = true
		}
	}

	if !dockerReady {
		if err := utils.RunStage(client, utils.DockerStage); err != nil {
			return err
		}
	}
	return nil
}

func stage6Traefik(client *ssh.Client, email string, p *tea.Program) error {
	traefikSetup := false
	outChan, _, err := utils.RunCommand(client, `[ -d "traefik" ] && echo "1" || echo "0"`)
	if err == nil {
		output := <-outChan
		if output == "1" {
			traefikSetup = true
		}
	}

	if !traefikSetup {
		traefikStage := utils.GetTraefikStage(email)
		if err := utils.RunStage(client, traefikStage); err != nil {
			return err
		}
	}
	return nil
}

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Init sidekick CLI and configure your VPS to host your apps",
	Long: `This command will run you through the setup steps to get sidekick loaded on your VPS.
		You wil need to provide your VPS IPv4 address and a registry to host your docker images.
		`,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()

		if configErr := utils.ViperInit(); configErr != nil {
			if errors.As(configErr, &viper.ConfigFileNotFoundError{}) {
				initConfig()
			} else {
				log.Fatalf("%s", configErr)
			}
		}

		skipPromptsFlag, _ := cmd.Flags().GetBool("yes")
		server, _ := cmd.Flags().GetString("server")
		certEmail, _ := cmd.Flags().GetString("email")

		if server == "" {
			server = render.GenerateTextQuestion("Please enter the IPv4 Address of your VPS", "", "")
			if !utils.IsValidIPAddress(server) {
				log.Fatalf("You entered an incorrect IP Address - %s", server)
			}
		}

		if certEmail == "" {
			certEmail = render.GenerateTextQuestion("Please enter an email for use with TLS certs", "", "")
			if certEmail == "" {
				log.Fatalf("An email is needed before you proceed")
			}
		}

		publicKey := viper.GetString("publicKey")
		if publicKey != "" && server != viper.GetString("serverAddress") && !skipPromptsFlag {
			confirm := render.GenerateTextQuestion("A server was previously setup with Sidekick. Would you like to override the settings? (y/n)", "n", "")
			if strings.ToLower(confirm) != "y" {
				fmt.Println("\nCurrently Sidekick only supports one server per setup")
				os.Exit(0)
			}
		}

		viper.Set("serverAddress", server)
		viper.Set("certEmail", certEmail)

		cmdStages := []render.Stage{
			render.MakeStage("Setting up your local env", "Installed local requirements successfully", false),
			render.MakeStage("Logging in to VPS", "Logged in successfully", false),
			render.MakeStage("Adding user Sidekick", "User Sidekick added successfully", false),
			render.MakeStage("Setting up VPS", "VPS setup successfully", false),
			render.MakeStage("Setting up Docker", "Docker setup successfully", false),
			render.MakeStage("Setting up Traefik", "Traefik setup successfully", false),
		}

		p := tea.NewProgram(render.TuiModel{
			Stages:      cmdStages,
			BannerMsg:   "Sidekick booting up! 🚀",
			ActiveIndex: 0,
			Quitting:    false,
			AllDone:     false,
		})

		go func() {
			if err := stage1LocalReqs(p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Local requirements check failed: %s", err)})
				return
			}
			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			sshClient, loggedInUser, err := stage2Login(server, p)
			if err != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Login failed: %s", err)})
				return
			}
			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			if err := stage3UserSetup(sshClient, loggedInUser, p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("User setup failed: %s", err)})
				return
			}

			sidekickClient, err := utils.Login(server, "sidekick")
			if err != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Failed to login as sidekick: %s", err)})
				return
			}
			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			if err := stage4VPSSetup(sidekickClient, p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("VPS setup failed: %s", err)})
				return
			}
			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			if err := stage5Docker(sidekickClient, p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Docker setup failed: %s", err)})
				return
			}
			time.Sleep(time.Millisecond * 100)
			p.Send(render.NextStageMsg{})

			if err := stage6Traefik(sidekickClient, certEmail, p); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Traefik setup failed: %s", err)})
				return
			}

			if err := viper.WriteConfig(); err != nil {
				p.Send(render.ErrorMsg{ErrorStr: fmt.Sprintf("Failed to write config: %s", err)})
				return
			}

			p.Send(render.AllDoneMsg{Message: "VPS Setup Done in " + time.Since(start).Round(time.Second).String() + "," + "\n" + "Your VPS is ready! You can now run Sidekick launch in your app folder"})
		}()

		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}

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
	InitCmd.Flags().BoolP("yes", "y", false, "Skip all validation prompts")
}
