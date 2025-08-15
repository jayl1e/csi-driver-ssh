package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"golang.org/x/crypto/ssh"
)

type Executer interface {
	ExecuteCommand(cmd string, env map[string]string) ([]byte, error)
}

var _ Executer = &SshExecuter{}

type SshExecuter struct {
	SshServer string
	SshUser   string
	SshKey    string
}

func (config *SshExecuter) ExecuteCommand(cmd string, env map[string]string) ([]byte, error) {
	privateKey, err := ssh.ParsePrivateKey([]byte(config.SshKey))
	if err != nil {
		slog.Error("Failed to parse private key", "err", err)
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	client, err := ssh.Dial("tcp", config.SshServer,
		&ssh.ClientConfig{
			User:            config.SshUser,
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(privateKey)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()
	newCmd := generateCmdWithEnv(cmd, env)
	result, err := session.CombinedOutput(newCmd)
	slog.Debug("SSH command executed", "cmd", newCmd, "output", string(result))
	return result, err
}

func generateCmdWithEnv(cmd string, env map[string]string) string {
	newCmd := "set -e;"
	for k, v := range env {
		newCmd += fmt.Sprintf("export %s=%s;", k, v)
	}
	newCmd += cmd
	return newCmd
}

type LocalExecuter struct {
}

func (e *LocalExecuter) ExecuteCommand(cmd string, env map[string]string) ([]byte, error) {
	command := exec.Command("bash", "-ec", generateCmdWithEnv(cmd, env))
	command.Env = os.Environ()
	return command.CombinedOutput()
}
