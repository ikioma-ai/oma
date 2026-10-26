package target

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
)

// FileTransport provides read-only access to files on a target host.
type FileTransport interface {
	ListFiles(ctx context.Context, dir string) ([]string, error)
	OpenFile(ctx context.Context, path string) (io.ReadCloser, error)
}

func (LocalTransport) ListFiles(ctx context.Context, dir string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (LocalTransport) OpenFile(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.Open(name)
}

func (t SSHTransport) ListFiles(ctx context.Context, dir string) ([]string, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSSHNotFound, err)
	}

	command := fmt.Sprintf("for f in %s/*; do [ -f \"$f\" ] && printf '%%s\\0' \"$f\"; done; exit 0", shellQuote(dir))
	cmd := exec.CommandContext(ctx, sshPath, sshFileArgs(t.Destination, command)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, sshFileError("listing backups", t.Destination, stderr.String(), err)
	}

	var names []string
	for _, entry := range bytes.Split(output, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		names = append(names, path.Base(string(entry)))
	}
	sort.Strings(names)
	return names, nil
}

func (t SSHTransport) OpenFile(ctx context.Context, name string) (io.ReadCloser, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSSHNotFound, err)
	}

	cmd := exec.CommandContext(ctx, sshPath, sshFileArgs(t.Destination, "cat -- "+shellQuote(name))...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("opening remote file: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, sshFileError("opening backup", t.Destination, stderr.String(), err)
	}

	return &sshFile{
		stdout:      stdout,
		cmd:         cmd,
		stderr:      &stderr,
		destination: t.Destination,
		name:        name,
	}, nil
}

type sshFile struct {
	stdout      io.ReadCloser
	cmd         *exec.Cmd
	stderr      *bytes.Buffer
	destination string
	name        string
}

func (f *sshFile) Read(p []byte) (int, error) {
	return f.stdout.Read(p)
}

func (f *sshFile) Close() error {
	_ = f.stdout.Close()
	if err := f.cmd.Wait(); err != nil {
		return sshFileError("reading "+f.name, f.destination, f.stderr.String(), err)
	}
	return nil
}

// Helpers

func sshFileArgs(destination, command string) []string {
	return []string{
		"-T",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		destination,
		command,
	}
}

func sshFileError(action, destination, stderr string, err error) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return fmt.Errorf("%s on target %q: %w", action, destination, err)
	}
	return fmt.Errorf("%s on target %q: %w: %s", action, destination, err, stderr)
}

var _ FileTransport = LocalTransport{}
var _ FileTransport = SSHTransport{}
