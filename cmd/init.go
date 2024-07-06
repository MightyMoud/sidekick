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
	"os"
	"os/exec"

	"github.com/ms-mousa/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Init sidekick CLI and configure your VPS to host your apps",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		pterm.Println()

		s, _ := pterm.DefaultBigText.WithLetters(
			putils.LettersFromStringWithStyle("Side", pterm.FgCyan.ToStyle()),
			putils.LettersFromStringWithStyle("kick", pterm.FgLightMagenta.ToStyle())).Srender()
		pterm.DefaultCenter.Println(s)
		pterm.DefaultBasicText.Println("Welcome to Sidekick. We need to collect some details from you first")

		// get server address
		server := ""
		serverTextInput := pterm.DefaultInteractiveTextInput
		serverTextInput.DefaultText = "Please enter the IPv4 Address of your VPS"
		server, _ = serverTextInput.Show()
		if !utils.IsValidIPAddress(server) {
			pterm.Error.Printfln("You entered an incorrect IP Address - %s", server)
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

		keyAddSshCommand := exec.Command("./prelude.sh", server)
		if sshAddErr := keyAddSshCommand.Run(); sshAddErr != nil {
			panic(sshAddErr)
		}

		// setup viper for config
		viper.SetConfigName("sidekick")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME/.config/sidekick/")
		viper.Set("serverAddress", server)
		viper.Set("dockerRegistery", dockerRegistery)
		viper.Set("dockerUsername", dockerUsername)

		multi := pterm.DefaultMultiPrinter
		setupProgressBar, _ := pterm.DefaultProgressbar.WithTotal(4).WithWriter(multi.NewWriter()).Start("Sidekick Booting up (2m estimated)  ")
		loginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging into VPS")
		stage1Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up VPS")
		stage2Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up Docker")
		stage3Spinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Setting up Traefik")
		multi.Start()

		sshClient, err := utils.LoginStage(server, loginSpinner, setupProgressBar)
		if err != nil {
			panic(err)
		}

		if err := utils.RunStage(sshClient, utils.SetupStage, stage1Spinner, setupProgressBar); err != nil {
			panic(err)
		}

		if err := utils.RunStage(sshClient, utils.DockerStage, stage2Spinner, setupProgressBar); err != nil {
			panic(err)
		}

		if err := utils.RunStage(sshClient, utils.GetTraefikStage(server), stage3Spinner, setupProgressBar); err != nil {
			panic(err)
		}

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
