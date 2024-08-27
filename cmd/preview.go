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
		utils.ViperInit()
		appConfig, appConfigErr := utils.LoadAppConfig()
		if appConfigErr != nil {
			log.Fatalln("Unable to load your config file. Might be corrupted")
			os.Exit(1)
		}
		fmt.Println(appConfig)
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
		fmt.Println(string(hashOutput))
		// if dockerBuildErr := gitTreeCheck.Run(); dockerBuildErr != nil {
		// 	log.Fatalln("Failed to run docker")
		// 	os.Exit(1)
		// }
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
