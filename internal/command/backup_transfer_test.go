package command

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/target"
)

func TestIsBackupArchiveName(t *testing.T) {
	assert.True(t, isBackupArchiveName("app-20250101-120000.tar.gz"))
	assert.False(t, isBackupArchiveName("app.tar.gz/evil"))
	assert.False(t, isBackupArchiveName("../app.tar.gz"))
	assert.False(t, isBackupArchiveName(`app\evil.tar.gz`))
	assert.False(t, isBackupArchiveName("app.txt"))
	assert.False(t, isBackupArchiveName(""))
	assert.False(t, isBackupArchiveName("app.tar.gz\n"))
}

func TestCollectTargetArchives(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app-20250101-120000.tar.gz"), []byte("archive"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0o600))

	apps := []*docker.Application{
		{Settings: docker.ApplicationSettings{
			Name:   "app",
			Backup: docker.BackupSettings{Destination: docker.BackupDestinationTarget, Path: dir},
		}},
		{Settings: docker.ApplicationSettings{
			Name:   "controller-app",
			Backup: docker.BackupSettings{Path: dir},
		}},
	}

	archives, err := collectTargetArchives(context.Background(), apps, target.LocalTransport{})
	require.NoError(t, err)
	require.Len(t, archives, 1)
	assert.Equal(t, filepath.Join(dir, "app-20250101-120000.tar.gz"), archives["app-20250101-120000.tar.gz"].Path)
}

func TestCollectTargetArchivesRejectsDuplicateNames(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	name := "app-20250101-120000.tar.gz"
	require.NoError(t, os.WriteFile(filepath.Join(first, name), []byte("first"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(second, name), []byte("second"), 0o600))

	apps := []*docker.Application{
		{Settings: docker.ApplicationSettings{Name: "one", Backup: docker.BackupSettings{Destination: docker.BackupDestinationTarget, Path: first}}},
		{Settings: docker.ApplicationSettings{Name: "two", Backup: docker.BackupSettings{Destination: docker.BackupDestinationTarget, Path: second}}},
	}

	_, err := collectTargetArchives(context.Background(), apps, target.LocalTransport{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple target-side archives")
}

func TestDownloadTargetArchive(t *testing.T) {
	sourceDir := t.TempDir()
	content := []byte("backup data")
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "app.tar.gz"), content, 0o600))

	destination := filepath.Join(t.TempDir(), "downloaded.tar.gz")
	archive := targetArchive{Name: "app.tar.gz", Path: filepath.Join(sourceDir, "app.tar.gz")}
	require.NoError(t, downloadTargetArchive(context.Background(), target.LocalTransport{}, archive, destination))

	got, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestDownloadTargetArchiveDoesNotOverwrite(t *testing.T) {
	sourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "app.tar.gz"), []byte("new"), 0o600))
	destination := filepath.Join(t.TempDir(), "downloaded.tar.gz")
	require.NoError(t, os.WriteFile(destination, []byte("old"), 0o600))

	archive := targetArchive{Name: "app.tar.gz", Path: filepath.Join(sourceDir, "app.tar.gz")}
	err := downloadTargetArchive(context.Background(), target.LocalTransport{}, archive, destination)
	require.ErrorIs(t, err, docker.ErrBackupFileExists)

	got, readErr := os.ReadFile(destination)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("old"), got)
}

func TestDownloadTargetArchiveRemovesPartialFile(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "downloaded.tar.gz")
	archive := targetArchive{Name: "app.tar.gz", Path: "/remote/app.tar.gz"}

	err := downloadTargetArchive(context.Background(), failingFileTransport{}, archive, destination)
	require.Error(t, err)

	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))
}

func TestDownloadTargetArchiveRemovesFileWhenOpenFails(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "downloaded.tar.gz")
	archive := targetArchive{Name: "app.tar.gz", Path: "/remote/app.tar.gz"}

	err := downloadTargetArchive(context.Background(), openFailTransport{}, archive, destination)
	require.Error(t, err)

	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))
}

// Helpers

type failingFileTransport struct{}

func (failingFileTransport) ListFiles(context.Context, string) ([]string, error) {
	return nil, nil
}

func (failingFileTransport) OpenFile(context.Context, string) (io.ReadCloser, error) {
	return failingReader{}, nil
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReader) Close() error             { return nil }

type openFailTransport struct{}

func (openFailTransport) ListFiles(context.Context, string) ([]string, error) {
	return nil, nil
}

func (openFailTransport) OpenFile(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("open failed")
}
