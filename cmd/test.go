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
	"time"

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
		// Create a multi printer. This allows multiple spinners to print simultaneously.
		multi := pterm.DefaultMultiPrinter

		// Create and start spinner 1 with a new writer from the multi printer.
		// The spinner will display the message "Spinner 1".
		spinner1, _ := pterm.DefaultSpinner.WithWriter(multi.NewWriter()).Start("Logging into VPS")

		// Create and start spinner 2 with a new writer from the multi printer.
		// The spinner will display the message "Spinner 2".
		spinner2, _ := pterm.DefaultSpinner.WithWriter(multi.NewWriter()).Start("Updating and settin up VPS")

		// Create and start spinner 3 with a new writer from the multi printer.
		// The spinner will display the message "Spinner 3".
		spinner3, _ := pterm.DefaultSpinner.WithWriter(multi.NewWriter()).Start("Setting up reverse Proxy")

		// Start the multi printer. This will start printing all the spinners.
		multi.Start()

		// Wait for 1 second.
		time.Sleep(time.Millisecond * 1000)

		// Stop spinner 1 with a success message.
		spinner1.Success("Logged in successfully")

		// Wait for 750 milliseconds.
		time.Sleep(time.Millisecond * 750)

		// Stop spinner 2 with a failure message.
		spinner2.Fail("Spinner 2 failed!")

		// Wait for 500 milliseconds.
		time.Sleep(time.Millisecond * 500)

		// Stop spinner 3 with a warning message.
		spinner3.Warning("Spinner 3 has a warning!")

		// Stop the multi printer. This will stop printing all the spinners.
		multi.Stop()

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
