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
	"os"
	"os/exec"

	"github.com/ms-mousa/sidekick/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type DockerService struct {
	Image   string   `yaml:"image"`
	Command string   `yaml:"command,omitempty"`
	Ports   []string `yaml:"ports,omitempty"`
	Volumes []string `yaml:"volumes,omitempty"`
	Labels  []string `yaml:"labels,omitempty"`
}

type DockerComposeFile struct {
	Services map[string]DockerService `yaml:"services"`
}

// launchCmd represents the launch command
var launchCmd = &cobra.Command{
	Use:   "launch",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		viper.SetConfigName("sidekick")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME/.config/sidekick/")
		err := viper.ReadInConfig() // Find and read the config file
		if err != nil {             // Handle errors reading the config file
			panic(fmt.Errorf("fatal error config file: %w", err))
		}
		adonis := DockerService{
			Image: "msmousa/adonis-test",
			Labels: []string{
				"traefik.enable=true",
				fmt.Sprintf("traefik.http.routers.frontend.rule=Host(`fe.%s.sslip.io`)", viper.Get("serverAddress")),
				"traefik.http.services.frontend.loadbalancer.server.port=3000",
				"traefik.http.routers.frontend.tls=true",
				"traefik.http.routers.frontend.tls.certresolver=default",
			},
		}
		newDockerCompose := DockerComposeFile{
			Services: map[string]DockerService{
				"adonis": adonis,
			},
		}
		dockerComposeFile, err := yaml.Marshal(&newDockerCompose)
		if err != nil {
			fmt.Printf("Error marshalling YAML: %v\n", err)
			return
		}
		err = os.WriteFile("docker-compose.yaml", dockerComposeFile, 0644)
		if err != nil {
			fmt.Printf("Error writing file: %v\n", err)
			return
		}

		multi := pterm.DefaultMultiPrinter
		launchPb, _ := pterm.DefaultProgressbar.WithTotal(3).WithWriter(multi.NewWriter()).Start("Booting up app on VPS")
		loginSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Logging into VPS")
		setupSpinner, _ := utils.GetSpinner().WithWriter(multi.NewWriter()).Start("Settin up application")

		multi.Start()

		sshClient, err := utils.LoginStage(viper.Get("serverAddress").(string), loginSpinner, launchPb)
		if err != nil {
			panic(err)
		}
		launchPb.Increment()

		setupSpinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
		sessionErr := utils.RunCommand(sshClient, "mkdir adonis")
		if sessionErr != nil {
			panic(sessionErr)
		}
		rsync := exec.Command("rsync", "docker-compose.yaml", fmt.Sprintf("%s@%s:%s", "root", viper.Get("serverAddress").(string), "./adonis"))
		rsync.Run()

		sessionErr1 := utils.RunCommand(sshClient, "cd adonis && docker compose -p sidekick up -d")
		if sessionErr1 != nil {
			panic(sessionErr1)
		}
		setupSpinner.Success("App setup successfully")
		launchPb.Increment()
		multi.Stop()
	},
}

func init() {
	rootCmd.AddCommand(launchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// launchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// launchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
