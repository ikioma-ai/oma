package target

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

const (
	sshConnectTimeout = 15 * time.Second
	sshSocketName     = "docker.sock"
)

var (
	ErrSSHNotFound    = errors.New("ssh client not found")
	ErrSSHHostKey     = errors.New("ssh host key verification failed")
	ErrSSHAuth        = errors.New("ssh authentication failed")
	ErrSSHUnreachable = errors.New("ssh target is unreachable")
	ErrSSHForward     = errors.New("docker socket forwarding failed")
)

type SSHTransport struct {
	Destination  string
	DockerSocket string
}

func (t SSHTransport) Connect(ctx context.Context) (*client.Client, io.Closer, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrSSHNotFound, err)
	}

	dir, err := shortTempDir()
	if err != nil {
		return nil, nil, fmt.Errorf("creating SSH socket directory: %w", err)
	}

	socketPath := filepath.Join(dir, sshSocketName)
	cmd := exec.CommandContext(ctx, sshPath, sshArgs(t.Destination, socketPath, t.DockerSocket)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, nil, fmt.Errorf("starting SSH: %w", err)
	}

	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
	}()

	processExited, err := waitForSocket(ctx, socketPath, exited)
	if err != nil {
		if !processExited {
			stopProcess(cmd, exited)
		}
		os.RemoveAll(dir)
		return nil, nil, sshConnectError(t.Destination, stderr.String(), err)
	}

	dockerClient, err := client.New(client.WithHost("unix://" + socketPath))
	if err != nil {
		stopProcess(cmd, exited)
		os.RemoveAll(dir)
		return nil, nil, fmt.Errorf("creating Docker client for target %q: %w", t.Destination, err)
	}

	return dockerClient, &sshConnection{
		client: dockerClient,
		cmd:    cmd,
		exited: exited,
		dir:    dir,
	}, nil
}

type sshConnection struct {
	client *client.Client
	cmd    *exec.Cmd
	exited chan error
	dir    string
}

func (c *sshConnection) Close() error {
	var errs []error

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	if c.exited != nil {
		select {
		case <-c.exited:
		case <-time.After(2 * time.Second):
		}
	}
	if c.dir != "" {
		if err := os.RemoveAll(c.dir); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Helpers

func sshArgs(destination, socketPath, dockerSocket string) []string {
	return []string{
		"-N",
		"-o", "BatchMode=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "StreamLocalBindUnlink=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-L", socketPath + ":" + dockerSocket,
		destination,
	}
}

func waitForSocket(ctx context.Context, socketPath string, exited <-chan error) (bool, error) {
	deadline := time.NewTimer(sshConnectTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-exited:
			return true, fmt.Errorf("SSH exited before the Docker socket was ready: %w", err)
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, fmt.Errorf("timed out waiting for the SSH Docker socket")
		case <-ticker.C:
			conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
			if err == nil {
				conn.Close()
				return false, nil
			}
		}
	}
}

func stopProcess(cmd *exec.Cmd, exited <-chan error) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
	}
}

func sshConnectError(destination, stderr string, err error) error {
	stderr = strings.TrimSpace(stderr)
	kind := classifySSHError(stderr)

	if stderr == "" {
		return fmt.Errorf("%w: %s: %v", kind, destination, err)
	}
	return fmt.Errorf("%w: %s: %s", kind, destination, stderr)
}

func classifySSHError(stderr string) error {
	message := strings.ToLower(stderr)

	switch {
	case strings.Contains(message, "host key verification failed"),
		strings.Contains(message, "remote host identification has changed"):
		return ErrSSHHostKey
	case strings.Contains(message, "permission denied"),
		strings.Contains(message, "authentication failed"),
		strings.Contains(message, "no supported authentication methods"):
		return ErrSSHAuth
	case strings.Contains(message, "could not resolve hostname"),
		strings.Contains(message, "connection refused"),
		strings.Contains(message, "connection timed out"),
		strings.Contains(message, "no route to host"):
		return ErrSSHUnreachable
	case strings.Contains(message, "remote port forwarding failed"),
		strings.Contains(message, "cannot listen to port"),
		strings.Contains(message, "streamlocal"):
		return ErrSSHForward
	default:
		return ErrSSHUnreachable
	}
}

func shortTempDir() (string, error) {
	dir, err := os.MkdirTemp("/tmp", "oma-")
	if err == nil {
		return dir, nil
	}
	return os.MkdirTemp("", "oma-")
}
