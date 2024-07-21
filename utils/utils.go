package utils

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type CommandsStage struct {
	Commands              []string
	SpinnerSuccessMessage string
	SpinnerFailMessage    string
}

func GetSshClient(server string) (*ssh.Client, error) {
	sshPort := "22"
	// connect to local ssh-agent to grab all keys
	sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAgentSock == "" {
		log.Fatal("No SSH SOCK AVAIBALEB")
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

	// now that we have our key, we need to start ssh client sesssion
	// ƒirst we make some config we pass later
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			// passing the public keys to callback to get the auth methods
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		// FIX BEFORE PROD
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// create SSH client with the said config and connect to server
	client, sshClientErr := ssh.Dial("tcp", fmt.Sprintf("%s:%s", server, sshPort), config)
	if sshClientErr != nil {
		log.Fatalf("Failed to create ssh client to the server: %v", sshClientErr)
	}

	return client, nil
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
			// fmt.Printf("\033[34m[STDOUT]\033[0m %s\n", stdoutScanner.Text())
		}
	}()

	go func() {
		for stderrScanner.Scan() {
			errChannel <- stderrScanner.Text()
			// fmt.Printf("\n\033[31m[STDERR]\033[0m %s\n", stderrScanner.Text())
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

func RunStage(client *ssh.Client, stage CommandsStage, spinner *pterm.SpinnerPrinter, progressBar *pterm.ProgressbarPrinter) error {
	spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
	if err := RunCommands(client, stage.Commands); err != nil {
		spinner.Fail(stage.SpinnerFailMessage)
		return err
	}
	spinner.Success(stage.SpinnerSuccessMessage)
	progressBar.Increment()
	return nil
}

func LoginStage(server string, spinner *pterm.SpinnerPrinter, progressBar *pterm.ProgressbarPrinter) (*ssh.Client, error) {
	spinner.Sequence = []string{"▀ ", " ▀", " ▄", "▄ "}
	sshClient, err := GetSshClient(server)
	if err != nil {
		spinner.Fail("Something went wrong logging in to your VPS")
		return nil, err
	}
	spinner.Success("Logged in successfully!")
	progressBar.Increment()
	return sshClient, nil
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
	viper.SetConfigName("sidekick")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config/sidekick/")
	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("Fatal error config file: %w", err)
	}
	return nil
}
