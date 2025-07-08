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
package utils

import (
	"bufio"
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

type CommandsStage struct {
	Commands              []string
	SpinnerSuccessMessage string
	SpinnerFailMessage    string
}

func RunCommand(client *ssh.Client, cmd string) (chan string, chan string, error) {
	session, err := client.NewSession()
	errChannel := make(chan string)
	stdOutChannel := make(chan string)
	if err != nil {
		log.Fatalf("Failed to create session: %s", err)
	}
	defer session.Close()
	// Need to hook into the pipe of output coming from that session
	stdoutReader, err := session.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting stdout reader: %s", err)
	}
	stderrReader, err := session.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting stderr reader: %s", err)
	}

	// make a scanner of that reader that will read as we get new stuff
	stdoutScanner := bufio.NewScanner(stdoutReader)
	stderrScanner := bufio.NewScanner(stderrReader)

	// start separate go routines to read from the pipes and print out
	go func() {
		for stdoutScanner.Scan() {
			stdOutChannel <- stdoutScanner.Text()
			// fmt.Printf("\033[34m[STDOUT]\033[0m %s\n", stdoutScanner.Text())
		}
	}()

	go func() {
		for stderrScanner.Scan() {
			errChannel <- stderrScanner.Text()
			// fmt.Printf("\n\033[31m[STDERR]\033[0m %s\n", stderrScanner.Text())
		}
	}()

	if err := session.Run(cmd); err != nil {
		defer session.Close()
		errString := <-errChannel
		return nil, nil, fmt.Errorf("error running command - %s: - %s", cmd, errString)
	}

	time.Sleep(time.Millisecond * 500)
	return stdOutChannel, errChannel, nil
}

func RunCommands(client *ssh.Client, commands []string) error {
	for _, cmd := range commands {
		_, _, err := RunCommand(client, cmd)
		if err != nil {
			return err
		}

	}
	return nil
}

func RunStage(client *ssh.Client, stage CommandsStage) error {
	if err := RunCommands(client, stage.Commands); err != nil {
		return err
	}
	return nil
}

func IsValidIPAddress(ip string) bool {
	const ipPattern = `\b(?:\d{1,3}\.){3}\d{1,3}\b`

	re := regexp.MustCompile(ipPattern)

	return re.MatchString(ip)

}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func ViperInit() error {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	configPath := fmt.Sprintf("%s/.config/sidekick", home)

	viper.AddConfigPath(configPath)
	viper.SetConfigType("yaml")
	viper.SetConfigName("default")
	err = viper.ReadInConfig()
	if err != nil {
		return err
	}
	return nil
}

func LoadAppConfig() (SidekickAppConfig, error) {
	if !FileExists("./sidekick.yml") {
		return SidekickAppConfig{}, errors.New("Sidekick app config not found. Please run sidekick launch first")
	}
	appConfigFile := SidekickAppConfig{}
	content, err := os.ReadFile("./sidekick.yml")
	if err != nil {
		pterm.Error.Println("Unable to process your project config")
		os.Exit(1)
	}
	if err := yaml.Unmarshal(content, &appConfigFile); err != nil {
		panic(err)
	}

	return appConfigFile, nil
}

func HandleEnvFile(envFileName string, dockerEnvProperty *[]string, envFileChecksum *string) error {
	envFile, envFileErr := os.Open(fmt.Sprintf("./%s", envFileName))
	if envFileErr != nil {
		return envFileErr
	}
	envMap, envParseErr := godotenv.Parse(envFile)
	if envParseErr != nil {
		return envParseErr
	}

	for key := range envMap {
		if strings.HasPrefix(key, "_") {
			continue
		}
		*dockerEnvProperty = append(*dockerEnvProperty, fmt.Sprintf("%s=${%s}", key, key))
	}
	// calculate and store the hash of env file to re-encrypt later on when changed
	envFileContent, _ := godotenv.Marshal(envMap)
	*envFileChecksum = fmt.Sprintf("%x", md5.Sum([]byte(envFileContent)))
	envCmd := exec.Command("sh", "-s", "-", viper.GetString("publicKey"), fmt.Sprintf("./%s", envFileName))
	// encrypt and save/override encrypted.env
	envCmd.Stdin = strings.NewReader(EnvEncryptionScript)
	if envCmdErr := envCmd.Run(); envCmdErr != nil {
		return envCmdErr
	}
	return nil
}

func WriteEnvFile(filename string, env map[string]string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error openign file: %w", err)
	}
	defer f.Close()

	for key, value := range env {
		// Check if the value contains spaces or special characters
		// and quote it if needed.  This is important for robustness.
		if strings.ContainsAny(value, " \t\n\r\"") {
			value = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\\\"")) // Escape inner quotes
		}
		if _, err := f.WriteString(fmt.Sprintf("%s=%s\n", key, value)); err != nil {
			return fmt.Errorf("error writing to file: %w", err)
		}
	}
	return nil
}

// DetectComposeFile checks for docker-compose.yml or compose.yml and returns the filename if found
func DetectComposeFile() (string, bool) {
	composeFiles := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	for _, file := range composeFiles {
		if FileExists(file) {
			return file, true
		}
	}
	return "", false
}

// ParseComposeFile reads and parses a docker-compose file
func ParseComposeFile(filename string) (*DockerComposeFile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	var compose DockerComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	return &compose, nil
}

// GetServicesWithBuildContext returns services that have a build context defined
func GetServicesWithBuildContext(compose *DockerComposeFile) map[string]DockerService {
	servicesWithBuild := make(map[string]DockerService)
	for name, service := range compose.Services {
		if service.Build != nil {
			servicesWithBuild[name] = service
		}
	}
	return servicesWithBuild
}

// GetServicesWithPorts returns all services that expose ports
func GetServicesWithPorts(compose *DockerComposeFile) []string {
	var services []string
	for name, service := range compose.Services {
		if len(service.Ports) > 0 {
			services = append(services, name)
		}
	}
	return services
}

// ExtractPortFromService extracts the container port from a service's port mapping
func ExtractPortFromService(service *DockerService) string {
	if len(service.Ports) == 0 {
		return ""
	}
	
	// Parse first port mapping (format: "host:container" or just "container")
	portMapping := service.Ports[0]
	parts := strings.Split(portMapping, ":")
	if len(parts) >= 2 {
		return parts[len(parts)-1] // Return container port
	}
	return parts[0] // Return the only part if no colon
}
