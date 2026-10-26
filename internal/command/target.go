package command

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/moby/moby/client"
	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/docker"
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
	t.cmd.AddCommand(newTargetAgentCommand().cmd)
	t.cmd.AddCommand(newTargetCheckCommand().cmd)
	t.cmd.AddCommand(newTargetDefaultCommand().cmd)
	t.cmd.AddCommand(newTargetSetupCommand().cmd)
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

type targetCheckCommand struct {
	cmd *cobra.Command
}

func newTargetCheckCommand() *targetCheckCommand {
	t := &targetCheckCommand{}
	t.cmd = &cobra.Command{
		Use:   "check [name]",
		Short: "Check target connectivity and Docker access",
		Args:  cobra.MaximumNArgs(1),
		RunE:  t.run,
	}
	return t
}

type targetSetupCommand struct {
	cmd          *cobra.Command
	ssh          string
	dockerSocket string
	namespace    string
	yes          bool
}

func newTargetSetupCommand() *targetSetupCommand {
	t := &targetSetupCommand{}
	t.cmd = &cobra.Command{
		Use:   "setup <name>",
		Short: "Prepare a remote target for Docker access",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	t.cmd.Flags().StringVar(&t.ssh, "ssh", "", "SSH destination for a new target")
	t.cmd.Flags().StringVar(&t.dockerSocket, "docker-socket", "", "remote Docker socket path")
	t.cmd.Flags().StringVar(&t.namespace, "namespace", "", "namespace to use on the target")
	t.cmd.Flags().BoolVar(&t.yes, "yes", false, "run without confirmation")
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

func (t *targetCheckCommand) run(cmd *cobra.Command, args []string) error {
	config, err := target.Load()
	if err != nil {
		return err
	}

	name := targetFlag(cmd)
	if len(args) > 0 {
		name = args[0]
	}
	selected, err := config.Resolve(name, os.Getenv("OMA_TARGET"))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out := cmd.OutOrStdout()
	if selected.Local {
		fmt.Fprintln(out, "SSH: not required")
	} else {
		fmt.Fprintf(out, "SSH: %s\n", selected.SSH)
	}

	transport, err := selected.Transport()
	if err != nil {
		return err
	}

	dockerClient, closer, err := transport.Connect(ctx)
	if err != nil {
		if selected.Local {
			fmt.Fprintf(out, "Docker: failed: %s\n", docker.ErrorMessage(err))
		} else {
			fmt.Fprintf(out, "SSH: failed: %s\n", err)
		}
		return err
	}
	defer closer.Close()

	if !selected.Local {
		fmt.Fprintln(out, "SSH: ok")
	}

	ping, err := dockerClient.Ping(ctx, client.PingOptions{})
	if err != nil {
		err = docker.ClassifyConnectionError(err)
		fmt.Fprintf(out, "Docker: failed: %s\n", docker.ErrorMessage(err))
		return err
	}

	fmt.Fprintf(out, "Docker: ok (API %s, OS %s)\n", ping.APIVersion, ping.OSType)
	return nil
}

func (t *targetSetupCommand) run(cmd *cobra.Command, args []string) error {
	config, err := target.Load()
	if err != nil {
		return err
	}

	name := args[0]
	selected, err := config.Resolve(name, "")
	isNew := false
	if err != nil {
		if t.ssh == "" {
			return err
		}
		isNew = true
		selected = target.Target{
			Name:         name,
			SSH:          t.ssh,
			DockerSocket: t.dockerSocket,
			Namespace:    t.namespace,
		}
		if selected.DockerSocket == "" {
			selected.DockerSocket = target.DefaultDockerSocket
		}
		if selected.Namespace == "" {
			selected.Namespace = target.DefaultNamespace
		}
	}

	if selected.Local {
		return fmt.Errorf("target setup is only for remote targets")
	}
	if !isNew && t.ssh != "" && selected.SSH != t.ssh {
		return fmt.Errorf("target %q already exists with SSH destination %q", name, selected.SSH)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Target: %s (%s)\n", selected.Name, selected.SSH)
	if !t.yes {
		confirmed, err := confirmTargetSetup(cmd)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(out, "Setup cancelled.")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if _, err := target.Setup(ctx, selected, func(line string) {
		fmt.Fprintln(out, line)
	}); err != nil {
		return err
	}

	transport, err := selected.Transport()
	if err != nil {
		return err
	}
	dockerClient, closer, err := transport.Connect(ctx)
	if err != nil {
		return fmt.Errorf("verifying Docker socket: %w", err)
	}
	defer closer.Close()
	ping, err := dockerClient.Ping(ctx, client.PingOptions{})
	if err != nil {
		err = docker.ClassifyConnectionError(err)
		return fmt.Errorf("verifying Docker socket: %s", docker.ErrorMessage(err))
	}
	fmt.Fprintf(out, "Docker socket: ok (API %s)\n", ping.APIVersion)

	if isNew {
		if err := config.Add(selected); err != nil {
			return err
		}
		if err := config.Save(); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "Target %s is ready.\n", selected.Name)
	return nil
}

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

func confirmTargetSetup(cmd *cobra.Command) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), "Prepare this target now? [y/N]: ")
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false, fmt.Errorf("reading confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
