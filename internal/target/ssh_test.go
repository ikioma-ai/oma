package target

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSHArgs(t *testing.T) {
	args := sshArgs("deploy@example.com", "/tmp/oma.sock", "/var/run/docker.sock")

	assert.Contains(t, args, "-N")
	assert.Contains(t, args, "-L")
	assert.Contains(t, args, "/tmp/oma.sock:/var/run/docker.sock")
	assert.Contains(t, args, "deploy@example.com")
	assert.Contains(t, args, "BatchMode=yes")
	assert.Contains(t, args, "ExitOnForwardFailure=yes")
}

func TestClassifySSHError(t *testing.T) {
	tests := []struct {
		stderr string
		want   error
	}{
		{stderr: "Host key verification failed.", want: ErrSSHHostKey},
		{stderr: "Permission denied (publickey).", want: ErrSSHAuth},
		{stderr: "ssh: Could not resolve hostname example.com", want: ErrSSHUnreachable},
		{stderr: "remote port forwarding failed for listen path", want: ErrSSHForward},
	}

	for _, test := range tests {
		assert.ErrorIs(t, classifySSHError(test.stderr), test.want, test.stderr)
	}
}

func TestSSHTransportMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, _, err := (SSHTransport{
		Destination:  "deploy@example.com",
		DockerSocket: "/var/run/docker.sock",
	}).Connect(context.Background())

	require.ErrorIs(t, err, ErrSSHNotFound)
}

func TestSSHTransportReportsAuthenticationFailure(t *testing.T) {
	installFakeSSH(t, `#!/bin/sh
echo "Permission denied (publickey)." >&2
exit 255
`)

	_, _, err := (SSHTransport{
		Destination:  "deploy@example.com",
		DockerSocket: "/var/run/docker.sock",
	}).Connect(context.Background())

	require.ErrorIs(t, err, ErrSSHAuth)
	assert.Contains(t, err.Error(), "deploy@example.com")
	assert.Contains(t, err.Error(), "Permission denied")
}

func TestSSHTransportBuildsForwardingCommand(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	installFakeSSH(t, `#!/bin/sh
printf '%s\n' "$@" > "$ARGS_FILE"
exit 255
`)
	t.Setenv("ARGS_FILE", argsFile)

	_, _, err := (SSHTransport{
		Destination:  "deploy@example.com",
		DockerSocket: "/var/run/docker.sock",
	}).Connect(context.Background())
	require.Error(t, err)

	data, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	args := string(data)
	assert.Contains(t, args, "-N")
	assert.Contains(t, args, "-L")
	assert.Contains(t, args, "/docker.sock:/var/run/docker.sock")
	assert.Contains(t, args, "deploy@example.com")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(args), "deploy@example.com"))
}

// Helpers

func installFakeSSH(t *testing.T, script string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "ssh")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	t.Setenv("PATH", dir)
}
