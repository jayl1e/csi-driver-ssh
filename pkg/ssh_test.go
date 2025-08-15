package pkg

import (
	"os"
	"testing"
)

func newSshConfig() *SshExecuter {
	return &SshExecuter{
		SshServer: os.Getenv("SSH_SERVER"),
		SshUser:   os.Getenv("SSH_USER"),
		SshKey:    os.Getenv("SSH_KEY"),
	}
}

func TestExecSSHCommand(t *testing.T) {
	if os.Getenv("SSH_SERVER") == "" {
		t.Skip("SSH_SERVER environment variable is not set")
	}
	config := newSshConfig()
	env := map[string]string{
		"VOLUME_ID": "test-volume-id",
	}
	stdout, err := config.ExecuteCommand("echo -n $VOLUME_ID", env)
	if err != nil {
		t.Fatalf("ExecSSHCommand failed: %v", err)
	}
	print(string(stdout))
	expected := "test-volume-id"
	if string(stdout) != expected {
		t.Errorf("unexpected output: got %q, want %q", stdout, expected)
	}
}
