package utils

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strings"

	"github.com/mightymoud/sidekick/render"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func getKeyFilesAuth() ([]ssh.AuthMethod, error) {
	user, err := user.Current()
	if err != nil {
		return nil, err
	}
	sshDir := path.Join(user.HomeDir, ".ssh")
	keyFiles := []string{
		"id_rsa",
		"id_ecdsa",
		"id_ed25519",
	}

	var authMethods []ssh.AuthMethod

	for _, keyFile := range keyFiles {
		keyPath := path.Join(sshDir, keyFile)
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			continue
		}

		privateKey, err := os.ReadFile(keyPath)
		if err != nil {
			continue
		}

		signer, err := ssh.ParsePrivateKey(privateKey)
		if err != nil {
			continue
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	return authMethods, nil
}

func inspectServerPublicKey(key ssh.PublicKey, hostname string) {
	sshKeyCmd := exec.Command("sh", "-s", "-", string(ssh.MarshalAuthorizedKey(key)))
	sshKeyCmd.Stdin = strings.NewReader(sshKeyScript)
	result, sshKeyCmdErr := sshKeyCmd.Output()
	if sshKeyCmdErr != nil {
		panic(sshKeyCmdErr)
	}
	resultLines := strings.Split(string(result), "\n")
	keyHash := resultLines[0]

	render.RenderKeyValidation(resultLines, keyHash, hostname)

}

func GetSshClient(server string, sshUser string) (*ssh.Client, error) {
	sshPort := "22"
	sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAgentSock == "" {
		log.Fatal("No SSH SOCK AVAILABLE")
		return nil, errors.New("Error happened connecting to ssh-agent")
	}

	conn, err := net.Dial("unix", sshAgentSock)
	if err != nil {
		log.Fatalf("Failed to connect to SSH agent: %s", err)
		return nil, err
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)

	// Get auth of standard keys not in agent
	authMethods, _ := getKeyFilesAuth()

	authMethods = append(authMethods, ssh.PublicKeysCallback(agentClient.Signers))

	cb := ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		currentUser, _ := user.Current()
		khPath := fmt.Sprintf("%s/.ssh/known_hosts", currentUser.HomeDir)
		kh, knErr := knownhosts.NewDB(khPath)
		if knErr != nil {
			log.Fatalf("Failed to read known_hosts: %s", err)
			os.Exit(1)
		}

		innerCallback := kh.HostKeyCallback()
		err := innerCallback(hostname, remote, key)
		if knownhosts.IsHostKeyChanged(err) {
			return fmt.Errorf("REMOTE HOST IDENTIFICATION HAS CHANGED for host %s! This may indicate a MitM attack.", hostname)
		} else if knownhosts.IsHostUnknown(err) {
			inspectServerPublicKey(key, hostname)
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
		User:            sshUser,
		Auth:            authMethods,
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
