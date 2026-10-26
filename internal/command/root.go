package command

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/brand"
	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/logging"
	"github.com/ikioma-ai/oma/internal/target"
	"github.com/ikioma-ai/oma/internal/ui"
)

type RootCommand struct {
	cmd        *cobra.Command
	namespace  string
	targetName string

	installImageRef string
}

func NewRootCommand() *RootCommand {
	r := &RootCommand{}
	r.cmd = &cobra.Command{
		Use:           brand.Name,
		Short:         "Manage web applications from Docker images",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		RunE: WithNamespace(func(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
			return logging.ToLogFile(func() error {
				return ui.Run(ns, r.installImageRef)
			})
		}),
	}
	r.cmd.PersistentFlags().StringVarP(&r.namespace, "namespace", "n", docker.DefaultNamespace, "namespace for containers")
	r.cmd.PersistentFlags().StringVar(&r.targetName, "target", "", "target to operate against")

	r.cmd.Flags().StringVar(&r.installImageRef, "install", "", "Path to Docker image to install")

	r.cmd.AddCommand(newBackgroundCommand().cmd)
	r.cmd.AddCommand(newBackupCommand().cmd)
	r.cmd.AddCommand(newCatalogCommand().cmd)
	r.cmd.AddCommand(newDeployCommand().cmd)
	r.cmd.AddCommand(newExecCommand().cmd)
	r.cmd.AddCommand(newKeysCommand().cmd)
	r.cmd.AddCommand(newListCommand().cmd)
	r.cmd.AddCommand(newRemoveCommand().cmd)
	r.cmd.AddCommand(newRestoreCommand().cmd)
	r.cmd.AddCommand(newStartCommand().cmd)
	r.cmd.AddCommand(newStopCommand().cmd)
	r.cmd.AddCommand(newTeardownCommand().cmd)
	r.cmd.AddCommand(newUpdateCommand().cmd)
	r.cmd.AddCommand(newSelfUpdateCommand().cmd)
	r.cmd.AddCommand(newTargetCommand().cmd)
	r.cmd.AddCommand(newVersionCommand().cmd)

	return r
}

func (r *RootCommand) Execute() error {
	return r.cmd.Execute()
}

// Helpers

type NamespaceRunE func(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error

func findApplication(ns *docker.Namespace, host string) (*docker.Application, error) {
	app := ns.ApplicationByHost(host)
	if app == nil {
		return nil, fmt.Errorf("no application found at host %q", host)
	}
	return app, nil
}

func withApplication(ns *docker.Namespace, host string, action string, fn func(*docker.Application) error) error {
	app, err := findApplication(ns, host)
	if err != nil {
		return err
	}

	if err := fn(app); err != nil {
		return fmt.Errorf("%s application: %w", action, err)
	}

	return nil
}

func WithNamespace(fn NamespaceRunE) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		config, err := target.Load()
		if err != nil {
			return fmt.Errorf("loading targets: %w", err)
		}

		selected, err := config.Resolve(targetFlag(cmd), os.Getenv("OMA_TARGET"))
		if err != nil {
			return err
		}

		transport, err := selected.Transport()
		if err != nil {
			return err
		}

		client, closer, err := transport.Connect(ctx)
		if err != nil {
			return fmt.Errorf("connecting to target %q: %w", selected.Name, err)
		}
		if closer != nil {
			defer closer.Close()
		}

		ns, err := docker.RestoreNamespace(ctx, namespaceFor(cmd, selected), docker.WithClient(client))
		if err != nil {
			return fmt.Errorf("restoring namespace: %w", err)
		}

		return fn(ctx, ns, cmd, args)
	}
}

func namespaceFlag(cmd *cobra.Command) string {
	namespace, _ := cmd.Root().PersistentFlags().GetString("namespace")
	return namespace
}

func targetFlag(cmd *cobra.Command) string {
	name, _ := cmd.Root().PersistentFlags().GetString("target")
	return name
}

func namespaceFor(cmd *cobra.Command, selected target.Target) string {
	if cmd.Root().PersistentFlags().Changed("namespace") {
		return namespaceFlag(cmd)
	}
	if selected.Namespace != "" {
		return selected.Namespace
	}
	return docker.DefaultNamespace
}

type exitCoder interface {
	error
	ExitCode() int
}

func ExitCode(err error) (int, bool) {
	if e, ok := errors.AsType[exitCoder](err); ok {
		return e.ExitCode(), true
	}
	return 0, false
}

func IsExitError(err error) bool {
	_, ok := ExitCode(err)
	return ok
}
