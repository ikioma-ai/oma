package target

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalFileTransport(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.tar.gz"), []byte("b"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.tar.gz"), []byte("a"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))

	files := LocalTransport{}
	names, err := files.ListFiles(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.tar.gz", "b.tar.gz"}, names)

	reader, err := files.OpenFile(context.Background(), filepath.Join(dir, "a.tar.gz"))
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, []byte("a"), data)
}

func TestLocalFileTransportMissingDirectory(t *testing.T) {
	names, err := (LocalTransport{}).ListFiles(context.Background(), filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestSSHFileTransport(t *testing.T) {
	parseSSHCommand(t)
	t.Setenv("FAKE_SSH_FAIL", "")
	t.Setenv("PATH", fakeSSHDir(t)+":"+os.Getenv("PATH"))

	files := SSHTransport{Destination: "fake"}

	names, err := files.ListFiles(context.Background(), "/backups")
	require.NoError(t, err)
	assert.Equal(t, []string{"a.tar.gz", "b.tar.gz"}, names)

	reader, err := files.OpenFile(context.Background(), "/backups/a.tar.gz")
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Equal(t, []byte("backup-data"), data)
}

func TestSSHFileTransportError(t *testing.T) {
	parseSSHCommand(t)
	t.Setenv("FAKE_SSH_FAIL", "1")
	t.Setenv("PATH", fakeSSHDir(t)+":"+os.Getenv("PATH"))

	_, err := (SSHTransport{Destination: "fake"}).ListFiles(context.Background(), "/backups")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing backups")
}

// Helpers

func fakeSSHDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
if [ -n "${FAKE_SSH_FAIL:-}" ]; then
  echo "ssh failed" >&2
  exit 1
fi
for last do :; done
case "$last" in
*"cat --"*) printf 'backup-data' ;;
*"for f in"*) printf '/backups/a.tar.gz\0/backups/b.tar.gz\0' ;;
*) exit 2 ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755))
	return dir
}

func parseSSHCommand(t *testing.T) {
	t.Helper()
	// Keep the fake ssh script isolated from any real SSH configuration.
	t.Setenv("HOME", t.TempDir())
}
