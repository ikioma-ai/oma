package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/target"
)

type targetStorageCommand struct {
	cmd *cobra.Command
}

func newTargetStorageCommand() *targetStorageCommand {
	t := &targetStorageCommand{}
	t.cmd = &cobra.Command{
		Use:   "storage",
		Short: "Manage target object storage",
	}
	t.cmd.AddCommand(newTargetStorageConfigureCommand().cmd)
	t.cmd.AddCommand(newTargetStorageStatusCommand().cmd)
	return t
}

type targetStorageConfigureCommand struct {
	cmd            *cobra.Command
	endpoint       string
	region         string
	bucket         string
	prefix         string
	accessKey      string
	secretKeyStdin bool
}

func newTargetStorageConfigureCommand() *targetStorageConfigureCommand {
	t := &targetStorageConfigureCommand{}
	t.cmd = &cobra.Command{
		Use:   "configure <name>",
		Short: "Configure S3-compatible object storage on a target",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	t.cmd.Flags().StringVar(&t.endpoint, "endpoint", "", "S3-compatible endpoint URL")
	t.cmd.Flags().StringVar(&t.region, "region", "", "object storage region")
	t.cmd.Flags().StringVar(&t.bucket, "bucket", "", "object storage bucket")
	t.cmd.Flags().StringVar(&t.prefix, "prefix", "", "object key prefix")
	t.cmd.Flags().StringVar(&t.accessKey, "access-key", "", "object storage access key")
	t.cmd.Flags().BoolVar(&t.secretKeyStdin, "secret-key-stdin", false, "read the secret key from stdin")
	return t
}

func readSecret(cmd *cobra.Command) (string, error) {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", fmt.Errorf("reading secret: %w", err)
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

func (t *targetStorageConfigureCommand) run(cmd *cobra.Command, args []string) error {
	_, selected, err := resolveTargetConfig(args[0])
	if err != nil {
		return err
	}
	if selected.Local {
		return fmt.Errorf("object storage configuration requires a remote target")
	}
	if !t.secretKeyStdin {
		return fmt.Errorf("--secret-key-stdin is required so the secret is not passed in process arguments")
	}
	secret, err := readSecret(cmd)
	if err != nil {
		return err
	}

	config := docker.ObjectStorageConfig{
		Endpoint:  t.endpoint,
		Region:    t.region,
		Bucket:    t.bucket,
		Prefix:    t.prefix,
		AccessKey: t.accessKey,
		SecretKey: secret,
	}
	if err := config.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encoding object storage config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ssh := target.SSHCommand{Destination: selected.SSH}
	if _, stderr, err := ssh.RunInput(ctx, "sudo -n tee /etc/oma/object-storage.json >/dev/null", bytes.NewReader(data)); err != nil {
		return targetCommandError("writing object storage config", stderr, err)
	}
	if _, stderr, err := ssh.Run(ctx, "sudo -n chmod 600 /etc/oma/object-storage.json"); err != nil {
		return targetCommandError("securing object storage config", stderr, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Configured object storage on %s (bucket %s, prefix %s)\n", selected.Name, config.Bucket, config.Prefix)
	return nil
}

type targetStorageStatusCommand struct {
	cmd *cobra.Command
}

func newTargetStorageStatusCommand() *targetStorageStatusCommand {
	t := &targetStorageStatusCommand{}
	t.cmd = &cobra.Command{
		Use:   "status <name>",
		Short: "Show target object storage configuration status",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	return t
}

func (t *targetStorageStatusCommand) run(cmd *cobra.Command, args []string) error {
	_, selected, err := resolveTargetConfig(args[0])
	if err != nil {
		return err
	}
	if selected.Local {
		return fmt.Errorf("object storage configuration requires a remote target")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ssh := target.SSHCommand{Destination: selected.SSH}
	output, stderr, err := ssh.Run(ctx, "sudo -n test -f /etc/oma/object-storage.json && echo configured || echo missing")
	if err != nil {
		return targetCommandError("reading object storage status", stderr, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Object storage: %s\n", strings.TrimSpace(output))
	return nil
}
