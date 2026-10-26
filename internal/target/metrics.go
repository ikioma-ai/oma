package target

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"time"
)

func (t SSHTransport) ForwardMetrics(ctx context.Context, remotePort int) (string, io.Closer, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrSSHNotFound, err)
	}

	localPort, err := reserveLocalPort()
	if err != nil {
		return "", nil, fmt.Errorf("reserving local metrics port: %w", err)
	}

	cmd := exec.CommandContext(ctx, sshPath, sshMetricsArgs(t.Destination, localPort, remotePort)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("starting SSH metrics forward: %w", err)
	}

	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
	}()

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort))
	processExited, err := waitForTCP(ctx, address, exited)
	if err != nil {
		if !processExited {
			stopProcess(cmd, exited)
		}
		return "", nil, sshConnectError(t.Destination, stderr.String(), err)
	}

	return "http://" + address + "/metrics", &sshConnection{
		cmd:    cmd,
		exited: exited,
	}, nil
}

// Helpers

func sshMetricsArgs(destination string, localPort, remotePort int) []string {
	return []string{
		"-N",
		"-o", "BatchMode=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-L", fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", localPort, remotePort),
		destination,
	}
}

func reserveLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForTCP(ctx context.Context, address string, exited <-chan error) (bool, error) {
	deadline := time.NewTimer(sshConnectTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-exited:
			return true, fmt.Errorf("SSH exited before the metrics port was ready: %w", err)
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, fmt.Errorf("timed out waiting for the SSH metrics port")
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if err == nil {
				conn.Close()
				return false, nil
			}
		}
	}
}
