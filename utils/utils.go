/*
Copyright © 2024 Mahmoud Mosua <m.mousa@hey.com>

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
	"github.com/skeema/knownhosts"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"os/user"

	"gopkg.in/yaml.v3"
)

type CommandsStage struct {
	Commands              []string
	SpinnerSuccessMessage string
	SpinnerFailMessage    string
}

func inspectServerPublicKey(key ssh.PublicKey) {
	sshKeyCmd := exec.Command("sh", "-s", "-", string(ssh.MarshalAuthorizedKey(key)))
	sshKeyCmd.Stdin = strings.NewReader(sshKeyScript)
	result, sshKeyCmdErr := sshKeyCmd.Output()
	if sshKeyCmdErr != nil {
		panic(sshKeyCmdErr)
	}
	resultLines := strings.Split(string(result), "\n")
	keyHash := resultLines[0]

	startColor := pterm.NewRGB(0, 255, 255)
	endColor := pterm.NewRGB(255, 0, 255)

	pterm.DefaultCenter.Print(keyHash)
	for i := 0; i < len(resultLines[1:]); i++ {
		fadeFactor := float32(i) / float32(20)
		currentColor := startColor.Fade(0, 1, fadeFactor, endColor)
		pterm.DefaultCenter.Print(currentColor.Sprint(resultLines[1:][i]))
	}

}

func GetSshClient(server string, sshUser string) (*ssh.Client, error) {
	sshPort := "22"
	// connect to local ssh-agent to grab all keys
	sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAgentSock == "" {
		log.Fatal("No SSH SOCK AVAILABLE")
		return nil, errors.New("Error happened connecting to ssh-agent")
	}
	// make a connection to SSH agent over unix protocl
	conn, err := net.Dial("unix", sshAgentSock)
	if err != nil {
		log.Fatalf("Failed to connect to SSH agent: %s", err)
		return nil, err
	}
	defer conn.Close()

	// make a ssh agent out of the connection
	agentClient := agent.NewClient(conn)

	// Check that we can get all the public keys added to the agent properly
	_, signersErr := agentClient.Signers()
	if signersErr != nil {
		log.Fatalf("Failed to get signers from SSH agent: %v", signersErr)
		return nil, err
	}

	cb := ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		currentUser, _ := user.Current()
		khPath := fmt.Sprintf("%s/.ssh/known_hosts", currentUser.HomeDir)
		kh, knErr := knownhosts.NewDB(khPath)
		if knErr != nil {
			log.Fatalf("Failed to read known_hosts: %s", err)
		}

		innerCallback := kh.HostKeyCallback()
		err := innerCallback(hostname, remote, key)
		if knownhosts.IsHostKeyChanged(err) {
			return fmt.Errorf("REMOTE HOST IDENTIFICATION HAS CHANGED for host %s! This may indicate a MitM attack.", hostname)
		} else if knownhosts.IsHostUnknown(err) {
			inspectServerPublicKey(key)
			prompt := pterm.DefaultInteractiveContinue

			pterm.DefaultCenter.Printf(pterm.FgYellow.Sprintf("This is the ASCII art and fingerprint of your VPS's public key at %s", hostname))
			pterm.DefaultCenter.Printf(pterm.FgYellow.Sprint("Please confirm you want to continue with the connection"))
			pterm.DefaultCenter.Printf(pterm.FgYellow.Sprint("Sidekick will add this host/key pair to known_hosts"))
			pterm.Println()

			prompt.DefaultText = "Would you like to proceed?"
			prompt.Options = []string{"yes", "no"}
			if result, _ := prompt.Show(); result != "yes" {
				pterm.Error.Println("In order to continue, you need to accept this.")
				os.Exit(0)
			}
			f, ferr := os.OpenFile(khPath, os.O_APPEND|os.O_WRONLY, 0600)
			if ferr == nil {
				defer f.Close()
				ferr = knownhosts.WriteKnownHost(f, hostname, remote, key)
			} else {
				log.Printf("Failed to add host %s to known_hosts: %v\n", hostname, ferr)
			}
			return nil
		}
		return err
	})
	// now that we have our key, we need to start ssh client sesssion
	config := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		HostKeyCallback: cb,
	}

	// create SSH client with the said config and connect to server
	client, sshClientErr := ssh.Dial("tcp", fmt.Sprintf("%s:%s", server, sshPort), config)
	if sshClientErr != nil {
		log.Fatalf("Failed to create ssh client to the server: %v", sshClientErr)
	}

	return client, nil
}

func Login(server string, user string) (*ssh.Client, error) {
	sshClient, err := GetSshClient(server, user)
	if err != nil {
		return nil, err
	}
	return sshClient, nil
}

func RunCommand(client *ssh.Client, cmd string) (chan string, error) {
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
		return nil, fmt.Errorf("error getting stdout reader: %s", err)
	}
	stderrReader, err := session.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("error getting stderr reader: %s", err)
	}

	// make a scanner of that reader that will read as we get new stuff
	stdoutScanner := bufio.NewScanner(stdoutReader)
	stderrScanner := bufio.NewScanner(stderrReader)

	// start separate go routines to read from the pipes and print out
	go func() {
		for stdoutScanner.Scan() {
			stdOutChannel <- stdoutScanner.Text()
			fmt.Printf("\033[34m[STDOUT]\033[0m %s\n", stdoutScanner.Text())
		}
	}()

	go func() {
		for stderrScanner.Scan() {
			errChannel <- stderrScanner.Text()
			fmt.Printf("\n\033[31m[STDERR]\033[0m %s\n", stderrScanner.Text())
		}
	}()

	// fmt.Printf("\033[35m Running the command: \033[0m %s\n", cmd)
	if err := session.Run(cmd); err != nil {
		session.Close()
		errString := <-errChannel
		return nil, fmt.Errorf("error running command - %s: - %s", cmd, errString)
	}

	time.Sleep(time.Millisecond * 500)
	// fmt.Println("Ran command successfully!")
	return stdOutChannel, nil
}

func RunCommands(client *ssh.Client, commands []string) error {
	for _, cmd := range commands {
		_, err := RunCommand(client, cmd)
		if err != nil {
			return err
		}

	}
	// fmt.Println("Ran all commands successfully")
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

	if re.MatchString(ip) {
		return true
	}

	return false
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
	viper.SetConfigName("default")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config/sidekick/")
	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("Fatal error config file: %w", err)
	}
	return nil
}

func LoadAppConfig() (SidekickAppConfig, error) {
	if !FileExists("./sidekick.yml") {
		log.Fatalln("Sidekick app config not found. Please run sidekick init first")
		os.Exit(1)
	}
	appConfigFile := SidekickAppConfig{}
	content, err := os.ReadFile("./sidekick.yml")
	if err != nil {
		fmt.Println(err)
		pterm.Error.Println("Unable to process your project config")
		os.Exit(1)
	}
	if err := yaml.Unmarshal(content, &appConfigFile); err != nil {
		panic(err)
	}

	return appConfigFile, nil
}

func HandleEnvFile(envFileName string, envVariables []string, dockerEnvProperty []string, envFileChecksum *string) error {
	envFileContent, envFileErr := os.ReadFile(fmt.Sprintf("./%s", envFileName))
	if envFileErr != nil {
		pterm.Error.Println("Unable to process your env file")
	}
	for _, line := range strings.Split(string(envFileContent), "\n") {
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		envVariables = append(envVariables, strings.Split(line, "=")[0])
	}

	for _, envVar := range envVariables {
		dockerEnvProperty = append(dockerEnvProperty, fmt.Sprintf("%s=${%s}", envVar, envVar))
	}
	// calculate and store the hash of env file to re-encrypt later on when changed
	*envFileChecksum = fmt.Sprintf("%x", md5.Sum(envFileContent))
	envCmd := exec.Command("sh", "-s", "-", viper.Get("publicKey").(string), fmt.Sprintf("./%s", envFileName))
	// encrypt and save/override encrypted.env
	envCmd.Stdin = strings.NewReader(EnvEncryptionScript)
	if envCmdErr := envCmd.Run(); envCmdErr != nil {
		return envCmdErr
	}
	return nil
}
