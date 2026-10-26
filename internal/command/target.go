package command

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/target"
)

type targetCommand struct {
	cmd *cobra.Command
}

func newTargetCommand() *targetCommand {
	t := &targetCommand{}
	t.cmd = &cobra.Command{
		Use:   "target",
		Short: "Manage deployment targets",
	}

	t.cmd.AddCommand(newTargetAddCommand().cmd)
	t.cmd.AddCommand(newTargetDefaultCommand().cmd)
	t.cmd.AddCommand(newTargetListCommand().cmd)
	t.cmd.AddCommand(newTargetRemoveCommand().cmd)

	return t
}

type targetAddCommand struct {
	cmd          *cobra.Command
	ssh          string
	dockerSocket string
	namespace    string
}

func newTargetAddCommand() *targetAddCommand {
	t := &targetAddCommand{}
	t.cmd = &cobra.Command{
		Use:   "add <name>",
		Short: "Add a named target",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	t.cmd.Flags().StringVar(&t.ssh, "ssh", "", "SSH destination, such as user@host")
	t.cmd.Flags().StringVar(&t.dockerSocket, "docker-socket", "", "remote Docker socket path")
	t.cmd.Flags().StringVar(&t.namespace, "namespace", "", "namespace to use on the target")
	return t
}

type targetListCommand struct {
	cmd *cobra.Command
}

func newTargetListCommand() *targetListCommand {
	t := &targetListCommand{}
	t.cmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List targets",
		Args:    cobra.NoArgs,
		RunE:    t.run,
	}
	return t
}

type targetRemoveCommand struct {
	cmd        *cobra.Command
	newDefault string
}

func newTargetRemoveCommand() *targetRemoveCommand {
	t := &targetRemoveCommand{}
	t.cmd = &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a target",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	t.cmd.Flags().StringVar(&t.newDefault, "new-default", "", "new default target when removing the current default")
	return t
}

type targetDefaultCommand struct {
	cmd *cobra.Command
}

func newTargetDefaultCommand() *targetDefaultCommand {
	t := &targetDefaultCommand{}
	t.cmd = &cobra.Command{
		Use:   "default <name>",
		Short: "Set the default target",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	return t
}

// Private

func (t *targetAddCommand) run(cmd *cobra.Command, args []string) error {
	config, err := target.Load()
	if err != nil {
		return err
	}

	entry := target.Target{
		Name:         args[0],
		SSH:          t.ssh,
		DockerSocket: t.dockerSocket,
		Namespace:    t.namespace,
	}
	if err := config.Add(entry); err != nil {
		return err
	}
	if err := config.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added target %s\n", entry.Name)
	return nil
}

func (t *targetListCommand) run(cmd *cobra.Command, args []string) error {
	config, err := target.Load()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDEFAULT\tSSH\tDOCKER SOCKET\tNAMESPACE")

	for _, entry := range config.SortedTargets() {
		ssh := entry.SSH
		if ssh == "" {
			ssh = "-"
		}
		socket := entry.DockerSocket
		if entry.Local {
			socket = "(env)"
		}
		marker := ""
		if entry.Name == config.Default {
			marker = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", entry.Name, marker, ssh, socket, entry.Namespace)
	}

	return w.Flush()
}

func (t *targetRemoveCommand) run(cmd *cobra.Command, args []string) error {
	config, err := target.Load()
	if err != nil {
		return err
	}
	if err := config.Remove(args[0], t.newDefault); err != nil {
		return err
	}
	if err := config.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed target %s\n", args[0])
	return nil
}

func (t *targetDefaultCommand) run(cmd *cobra.Command, args []string) error {
	config, err := target.Load()
	if err != nil {
		return err
	}
	if err := config.SetDefault(args[0]); err != nil {
		return err
	}
	if err := config.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Default target is now %s\n", args[0])
	return nil
}
