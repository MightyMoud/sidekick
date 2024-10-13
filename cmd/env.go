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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mightymoud/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// envCmd represents the env command
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Prepare env variable secrets by encrypting them before deployment",
	Long: `This command allows you to encrypt sensitive environment variables, such as API keys and database credentials, before deploying your application. 
These encrypted secrets will be securely stored and used by your application during runtime, ensuring that your sensitive information is not exposed.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.ViperInit()
		envFile, envFileErr := os.ReadFile("./.env")
		if envFileErr != nil {
			pterm.Error.Println("Unable to process your env file")
		}
		pterm.Info.Println("Detected the following env variables in your project")
		for _, line := range strings.Split(string(envFile), "\n") {
			fmt.Println(strings.Split(line, "=")[0])
		}
		pterm.Info.Println("-----------")
		pterm.Info.Println("Encrypting env vars using VPS public key")
		envCmd := exec.Command("sh", "-s", "-", viper.Get("publicKey").(string))
		envCmd.Stdin = strings.NewReader(utils.EnvEncryptionScript)
		if envCmdErr := envCmd.Run(); envCmdErr != nil {
			panic(envCmdErr)
		}
	},
}

func init() {
	launchCmd.AddCommand(envCmd)
}
