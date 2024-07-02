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

	"github.com/ms-mousa/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"
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

		server := ""
		textInput := pterm.DefaultInteractiveTextInput
		textInput.DefaultText = "Please enter the IPv4 Address of your VPS"
		server, _ = textInput.Show()
		if !utils.IsValidIPAddress(server) {
			pterm.Error.Printfln("You entered an incorrect IP Address - %s", server)
			os.Exit(0)
		}

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
