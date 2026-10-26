package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/fsutil"
	"github.com/ikioma-ai/oma/internal/target"
)

type targetBackupsCommand struct {
	cmd *cobra.Command
}

func newTargetBackupsCommand() *targetBackupsCommand {
	t := &targetBackupsCommand{}
	t.cmd = &cobra.Command{
		Use:   "backups",
		Short: "Manage object-storage backups on a target",
	}
	t.cmd.AddCommand(newTargetBackupsListCommand().cmd)
	t.cmd.AddCommand(newTargetBackupsDownloadCommand().cmd)
	t.cmd.AddCommand(newTargetBackupsRestoreCommand().cmd)
	return t
}

type targetBackupsListCommand struct {
	cmd *cobra.Command
}

func newTargetBackupsListCommand() *targetBackupsListCommand {
	t := &targetBackupsListCommand{}
	t.cmd = &cobra.Command{
		Use:   "list <name>",
		Short: "List object-storage backups on a target",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	return t
}

func (t *targetBackupsListCommand) run(cmd *cobra.Command, args []string) error {
	_, selected, err := resolveTargetConfig(args[0])
	if err != nil {
		return err
	}
	if selected.Local {
		return fmt.Errorf("object storage backups require a remote target")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	output, stderr, err := target.SSHCommand{Destination: selected.SSH}.Run(ctx, "sudo -n /usr/local/bin/oma backup object list")
	if err != nil {
		return targetCommandError("listing object backups", stderr, err)
	}

	var archives []docker.ObjectArchive
	if err := json.Unmarshal([]byte(output), &archives); err != nil {
		return fmt.Errorf("parsing object backup list: %w", err)
	}
	if len(archives) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No object backups found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tMODIFIED")
	for _, archive := range archives {
		fmt.Fprintf(w, "%s\t%d\t%s\n", archive.Name, archive.Size, archive.Modified.Local().Format(time.RFC3339))
	}
	return w.Flush()
}

type targetBackupsDownloadCommand struct {
	cmd *cobra.Command
}

func newTargetBackupsDownloadCommand() *targetBackupsDownloadCommand {
	t := &targetBackupsDownloadCommand{}
	t.cmd = &cobra.Command{
		Use:   "download <name> <archive> <destination>",
		Short: "Download an object-storage backup from a target",
		Args:  cobra.ExactArgs(3),
		RunE:  t.run,
	}
	return t
}

func (t *targetBackupsDownloadCommand) run(cmd *cobra.Command, args []string) error {
	_, selected, err := resolveTargetConfig(args[0])
	if err != nil {
		return err
	}
	if selected.Local {
		return fmt.Errorf("object storage backups require a remote target")
	}
	if !isBackupArchiveName(args[1]) {
		return fmt.Errorf("invalid backup archive name %q", args[1])
	}

	destination, err := filepath.Abs(args[2])
	if err != nil {
		return fmt.Errorf("resolving destination: %w", err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()
	if err := downloadObjectArchive(ctx, selected, args[1], destination); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %s to %s\n", args[1], destination)
	return nil
}

type targetBackupsRestoreCommand struct {
	cmd *cobra.Command
}

func newTargetBackupsRestoreCommand() *targetBackupsRestoreCommand {
	t := &targetBackupsRestoreCommand{}
	t.cmd = &cobra.Command{
		Use:   "restore <name> <archive>",
		Short: "Restore a target from an object-storage backup",
		Args:  cobra.ExactArgs(2),
		RunE:  t.run,
	}
	return t
}

func (t *targetBackupsRestoreCommand) run(cmd *cobra.Command, args []string) error {
	_, selected, err := resolveTargetConfig(args[0])
	if err != nil {
		return err
	}
	if selected.Local {
		return fmt.Errorf("object storage backups require a remote target")
	}
	if !isBackupArchiveName(args[1]) {
		return fmt.Errorf("invalid backup archive name %q", args[1])
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
	defer cancel()
	transport, err := selected.Transport()
	if err != nil {
		return err
	}
	client, closer, err := transport.Connect(ctx)
	if err != nil {
		return fmt.Errorf("connecting to target %q: %w", selected.Name, err)
	}
	defer closer.Close()
	ns, err := docker.RestoreNamespace(ctx, namespaceFor(cmd, selected), docker.WithClient(client), docker.WithTargetName(selected.Name))
	if err != nil {
		return fmt.Errorf("restoring namespace: %w", err)
	}
	defer ns.Close()

	file, err := os.CreateTemp("", "oma-object-restore-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temporary restore file: %w", err)
	}
	filename := file.Name()
	defer os.Remove(filename)
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing temporary restore file: %w", err)
	}
	os.Remove(filename)

	if err := downloadObjectArchive(ctx, selected, args[1], filename); err != nil {
		return err
	}
	backup, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("opening downloaded backup: %w", err)
	}
	defer backup.Close()

	app, err := ns.Restore(ctx, backup)
	if err != nil {
		return fmt.Errorf("restoring application: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Restored %s from %s\n", app.Settings.Name, args[1])
	return nil
}

func downloadObjectArchive(ctx context.Context, selected target.Target, archive, destination string) error {
	file, err := fsutil.CreateFile(destination)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer file.Close()

	complete := false
	defer func() {
		if !complete {
			os.Remove(destination)
		}
	}()

	command := "sudo -n /usr/local/bin/oma backup object download " + target.ShellQuote(archive)
	if err := (target.SSHCommand{Destination: selected.SSH}).RunTo(ctx, command, file); err != nil {
		return targetCommandError("downloading object backup", "", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing destination file: %w", err)
	}
	complete = true
	return nil
}
