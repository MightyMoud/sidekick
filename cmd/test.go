/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		cwd, _ := os.Getwd()
		// var stdErrBuff bytes.Buffer
		dockerBuildCommd := exec.Command("sh", "-s", "-", "app", cwd, "latest")
		dockerBuildCommd.Stdin = strings.NewReader(utils.DockerBuildAndSaveScript)
		// dockerBuildCommd.Stderr = &stdErrBuff
		// p, _ := dockerBuildCommd.StdoutPipe()
		e, _ := dockerBuildCommd.StderrPipe()
		lastLines := make([]string, 0, 5)
		ch := make(chan string, 5)

		updates := make(chan [2]string)

		fullArea := pterm.DefaultArea.WithCenter()
		fullArea.Start()

		// Define area printers
		area1 := pterm.AreaPrinter{}
		area2 := pterm.AreaPrinter{}

		// Goroutine to handle updating the panel in the loop
		go func() {
			// Set initial values
			area1.Update("TWO")
			area2.Update("ONE")

			for {
				// Select to listen for updates or continue looping
				select {
				case updateData := <-updates:
					// Apply the updates received from the channel
					area1.Update(updateData[0])
					area2.Update(updateData[1])
				default:
					// Get updated content from the areas
					area1Content := area1.GetContent()
					area2Content := area2.GetContent()

					// Create panels using the updated content
					panels := pterm.Panels{
						{
							{Data: area1Content},
						},
						{
							{Data: area2Content},
						},
					}

					// Render the panels with a padding of 5 and update the full area
					s, _ := pterm.DefaultPanel.WithPanels(panels).WithPadding(5).Srender()
					fullArea.Update(s)

					// Sleep to prevent tight loop
					time.Sleep(time.Millisecond * 500)
				}
			}
		}()

		// Simulate updates from outside the loop
		time.Sleep(time.Second * 2)
		updates <- [2]string{pterm.DefaultBox.WithTitle("dhdh").Sprint("jklfdjslk"), "TWO"} // Send the first update
		time.Sleep(time.Second * 2)
		updates <- [2]string{pterm.DefaultBox.WithTitle("second").Sprint("third"), "TWO"} // Send the first update
		time.Sleep(time.Second * 2)
		updates <- [2]string{pterm.DefaultBox.WithTitle("helloWorld").Sprint("hjklfdhjkfha"), "UPDATED TWO"} // Send the second update

		pterm.Print("hjhhhh")
		// Keep the program running
		select {} // b.Update("LOLOLO")
		// dockerBuildStageSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Building latest docker image of your app")
		// deployStageSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Deploying a preview env of your application")

		// title := pterm.LightRed("I'm a box!")
		// pterm.DefaultBox.WithTitle(title).Println()

		// multi.Start()

		// spinnerInfo, _ := pterm.DefaultSpinner.Start("Some informational action...")

		// area, _ := pterm.DefaultArea.WithRemoveWhenDone().Start()
		// area.Update(s)
		// time.Sleep(time.Second * 2) // Simulate 3 seconds of processing something.
		// fmt.Println("Single line cursor area demo")
		// fmt.Println("----------------------------")

		// area := cursor.NewArea()

		// header := "CONTENT-----"
		// area.Update(header)
		// for i := 1; i < 6; i++ {
		// 	time.Sleep(1 * time.Second)
		// 	area.Update(fmt.Sprintf("%s: %d", header, i))
		// }

		// header = "CONTENT 222222 --------"
		// area.Update(header)
		// for i := 1; i < 6; i++ {
		// 	time.Sleep(1 * time.Second)
		// 	area.Update(fmt.Sprintf("%s: %d\n", header, i))
		// }

		// time.Sleep(1 * time.Second)
		// fmt.Println("\n--- DONE")
		// loginSpinner.Success("yay")

		go func() {
			reader := bufio.NewReader(e)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						break
					}
					log.Fatalf("Error reading from stderr pipe: %v", err)
				}
				ch <- line
			}

		}()

		go func() {
			for i := range ch {
				lastLines = append(lastLines, i)
				n := len(lastLines)

				start := n - 5
				if start < 0 {
					start = 0
				}

				// lastFive := lastLines[start:]
				// toPrint := strings.Join(lastFive, "\n")
				// area.Update(lastFive)
			}
			// area.Stop()
			time.Sleep(time.Millisecond * 2000)
			// spinnerInfo.Info()
		}()
		if dockerBuildErr := dockerBuildCommd.Run(); dockerBuildErr != nil {
			// pterm.Error.Printfln("Failed to build docker image with the following error: \n%s", stdErrBuff.String())
			// os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(testCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// testCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// testCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
