package tmux

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Client handles tmux operations, optionally on a remote host
type Client struct {
	Remote string // "user@host" or empty for local
}

// NewClient creates a new tmux client
func NewClient(remote string) *Client {
	return &Client{Remote: remote}
}

// DefaultClient is the default local client
var DefaultClient = NewClient("")

// Run executes a tmux command
func (c *Client) Run(args ...string) (string, error) {
	if c.Remote == "" {
		return runLocal(args...)
	}

	// Remote execution via ssh
	// We need to handle escaping if args contain spaces?
	// ssh passes args concatenated.
	// For simple tmux commands it's usually fine.
	sshArgs := append([]string{c.Remote, "tmux"}, args...)
	return runSSH(sshArgs...)
}

// runLocal executes a tmux command locally
func runLocal(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// runSSH executes an ssh command and returns stdout
func runSSH(args ...string) (string, error) {
	cmd := exec.Command("ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("ssh %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunSilent executes a tmux command ignoring output
func (c *Client) RunSilent(args ...string) error {
	_, err := c.Run(args...)
	return err
}

// IsInstalled checks if tmux is available on the target host
func (c *Client) IsInstalled() bool {
	if c.Remote == "" {
		_, err := exec.LookPath("tmux")
		return err == nil
	}
	// Check remote
	err := c.RunSilent("-V")
	return err == nil
}
