package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/fsutil"
	"github.com/ikioma-ai/oma/internal/target"
)

type backupDownloadCommand struct {
	cmd *cobra.Command
}

func newBackupDownloadCommand() *backupDownloadCommand {
	b := &backupDownloadCommand{}
	b.cmd = &cobra.Command{
		Use:   "download <archive> <destination>",
		Short: "Download a target-side backup",
		Args:  cobra.ExactArgs(2),
		RunE:  withTargetFiles(b.run),
	}
	return b
}

// Private

func (b *backupDownloadCommand) run(ctx context.Context, ns *docker.Namespace, _ target.Target, files target.FileTransport, _ *cobra.Command, args []string) error {
	archiveName := args[0]
	if !isBackupArchiveName(archiveName) {
		return fmt.Errorf("invalid backup archive name %q", archiveName)
	}

	archives, err := collectTargetArchives(ctx, ns.Applications(), files)
	if err != nil {
		return err
	}
	archive, ok := archives[archiveName]
	if !ok {
		return fmt.Errorf("no target-side backup named %q", archiveName)
	}

	destination, err := filepath.Abs(args[1])
	if err != nil {
		return fmt.Errorf("resolving destination: %w", err)
	}

	if err := downloadTargetArchive(ctx, files, archive, destination); err != nil {
		return err
	}

	fmt.Printf("Downloaded %s to %s\n", archiveName, destination)
	return nil
}

// Helpers

func downloadTargetArchive(ctx context.Context, files target.FileTransport, archive targetArchive, destination string) error {
	file, err := fsutil.CreateFile(destination)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %s (backups never overwrite an existing file)", docker.ErrBackupFileExists, destination)
	}
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer file.Close()

	downloaded := false
	defer func() {
		if !downloaded {
			os.Remove(destination)
		}
	}()

	source, err := files.OpenFile(ctx, archive.Path)
	if err != nil {
		return fmt.Errorf("opening target backup: %w", err)
	}

	_, copyErr := io.Copy(file, source)
	closeErr := source.Close()
	if copyErr != nil {
		return fmt.Errorf("downloading target backup: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("reading target backup: %w", closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing destination file: %w", err)
	}

	downloaded = true
	return nil
}
