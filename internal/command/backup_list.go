package command

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/target"
)

type backupListCommand struct {
	cmd *cobra.Command
}

type targetArchive struct {
	Name string
	Path string
}

func newBackupListCommand() *backupListCommand {
	b := &backupListCommand{}
	b.cmd = &cobra.Command{
		Use:   "list",
		Short: "List target-side backups",
		Args:  cobra.NoArgs,
		RunE:  withTargetFiles(b.run),
	}
	return b
}

// Private

func (b *backupListCommand) run(ctx context.Context, ns *docker.Namespace, _ target.Target, files target.FileTransport, _ *cobra.Command, _ []string) error {
	archives, err := collectTargetArchives(ctx, ns.Applications(), files)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(archives))
	for name := range archives {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}

// Helpers

func collectTargetArchives(ctx context.Context, apps []*docker.Application, files target.FileTransport) (map[string]targetArchive, error) {
	archives := make(map[string]targetArchive)
	for _, app := range apps {
		if app.Settings.Backup.DestinationOrDefault() != docker.BackupDestinationTarget {
			continue
		}
		dir := app.Settings.Backup.Path
		if dir == "" {
			continue
		}

		names, err := files.ListFiles(ctx, dir)
		if err != nil {
			return nil, fmt.Errorf("listing backups for %s: %w", app.Settings.Name, err)
		}
		for _, name := range names {
			if !isBackupArchiveName(name) {
				continue
			}
			if existing, ok := archives[name]; ok {
				return nil, fmt.Errorf("multiple target-side archives named %q (%s and %s)", name, existing.Path, path.Join(dir, name))
			}
			archives[name] = targetArchive{Name: name, Path: path.Join(dir, name)}
		}
	}
	return archives, nil
}

func isBackupArchiveName(name string) bool {
	if name == "" || name == "." || name == ".." || name != path.Base(name) {
		return false
	}
	if strings.ContainsAny(name, "/\\\x00\n\r") {
		return false
	}
	return strings.HasSuffix(name, ".tar.gz")
}
