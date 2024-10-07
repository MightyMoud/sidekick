/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// teaCmd represents the tea command
var teaCmd = &cobra.Command{
	Use:   "tea",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		cmdStages := []stage{
			makeStage("Logging in to your server", "Logged in successfully", true),
			makeStage("Doing something", "We did it", true),
		}

		p := tea.NewProgram(newModel(cmdStages))

		go func() {
			// imgMoveCmd := exec.Command("scp", "-C", "-v", "go.sum", "sidekick@178.156.139.11:./app", "|", "grep", "-v", "debug")
			imgMoveCmd := exec.Command("sftp", "-v", "-b", "-", "sidekick@178.156.139.11", "<<< 'put go.sum ./app'")
			// imgMoveCmd.Stdin = strings.NewReader(utils.ImageMoveScript)
			// var out bytes.Buffer
			// imgMoveCmd.Stdout = &out
			// imgMoveCmd.Stderr = &out
			secCommand := exec.Command("pwd")
			// fmt.Printf("File copied successfully!\nOutput: %s\n", out.String())

			e, _ := imgMoveCmd.StderrPipe()
			y, _ := secCommand.StdoutPipe()

			go func() {
				reader := bufio.NewReader(e)
				for {
					line, err := reader.ReadString('\n')

					if err != nil {
						if err == io.EOF {
							break
						}
					}
					if line != "\n" {
						p.Send(logMsg{LogLine: line})
					}
				}
			}()

			go func() {
				reader := bufio.NewReader(y)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							break
						}
					}
					p.Send(logMsg{LogLine: line})
				}
			}()
			if dockerBuildErr := imgMoveCmd.Run(); dockerBuildErr != nil {
				// pterm.Error.Printfln("Failed to build docker image with the following error: \n%s", stdErrBuff.String())
				// os.Exit(1)
			}
			time.Sleep(time.Millisecond * 2000)
			p.Send(setStateMsg{ActiveIndex: 1})
			time.Sleep(time.Millisecond * 2000)
			if secCommandErr := secCommand.Run(); secCommandErr != nil {
				// pterm.Error.Printfln("Failed to build docker image with the following error: \n%s", stdErrBuff.String())
				// os.Exit(1)
			}

		}()

		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}

	},
}

func init() {
	rootCmd.AddCommand(teaCmd)

}
