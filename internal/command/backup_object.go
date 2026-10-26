package command

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/docker"
)

type backupObjectCommand struct {
	cmd *cobra.Command
}

func newBackupObjectCommand() *backupObjectCommand {
	b := &backupObjectCommand{}
	b.cmd = &cobra.Command{
		Use:    "object",
		Short:  "Manage object-storage backups on this target",
		Hidden: true,
	}
	b.cmd.AddCommand(newBackupObjectListCommand().cmd)
	b.cmd.AddCommand(newBackupObjectDownloadCommand().cmd)
	return b
}

type backupObjectListCommand struct {
	cmd *cobra.Command
}

func newBackupObjectListCommand() *backupObjectListCommand {
	b := &backupObjectListCommand{}
	b.cmd = &cobra.Command{
		Use:   "list",
		Short: "List object-storage backups",
		Args:  cobra.NoArgs,
		RunE:  b.run,
	}
	return b
}

func (b *backupObjectListCommand) run(cmd *cobra.Command, _ []string) error {
	store, err := objectStoreForTarget()
	if err != nil {
		return err
	}
	archives, err := store.List(cmd.Context())
	if err != nil {
		return err
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(archives)
}

type backupObjectDownloadCommand struct {
	cmd *cobra.Command
}

func newBackupObjectDownloadCommand() *backupObjectDownloadCommand {
	b := &backupObjectDownloadCommand{}
	b.cmd = &cobra.Command{
		Use:   "download <archive>",
		Short: "Download an object-storage backup",
		Args:  cobra.ExactArgs(1),
		RunE:  b.run,
	}
	return b
}

func (b *backupObjectDownloadCommand) run(cmd *cobra.Command, args []string) error {
	if !isBackupArchiveName(args[0]) {
		return fmt.Errorf("invalid backup archive name %q", args[0])
	}
	store, err := objectStoreForTarget()
	if err != nil {
		return err
	}
	return store.Download(cmd.Context(), args[0], cmd.OutOrStdout())
}

func objectStoreForTarget() (*docker.ObjectStore, error) {
	config, err := docker.LoadObjectStorageConfig()
	if err != nil {
		return nil, err
	}
	return config.Store()
}
