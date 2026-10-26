package command

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/target"
	"github.com/ikioma-ai/oma/internal/version"
)

type targetAgentCommand struct {
	cmd *cobra.Command
}

func newTargetAgentCommand() *targetAgentCommand {
	t := &targetAgentCommand{}
	t.cmd = &cobra.Command{
		Use:   "agent",
		Short: "Manage the target-local Oma agent",
	}
	t.cmd.AddCommand(newTargetAgentInstallCommand().cmd)
	t.cmd.AddCommand(newTargetAgentDisableCommand().cmd)
	t.cmd.AddCommand(newTargetAgentStatusCommand().cmd)
	return t
}

type targetAgentInstallCommand struct {
	cmd *cobra.Command
}

func newTargetAgentInstallCommand() *targetAgentInstallCommand {
	t := &targetAgentInstallCommand{}
	t.cmd = &cobra.Command{
		Use:   "install <name>",
		Short: "Install or update the signed target-local agent",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	return t
}

func (t *targetAgentInstallCommand) run(cmd *cobra.Command, args []string) error {
	config, selected, err := resolveTargetConfig(args[0])
	if err != nil {
		return err
	}
	if selected.Local {
		return fmt.Errorf("the local target cannot have an agent")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ssh := target.SSHCommand{Destination: selected.SSH}
	osName, stderr, err := ssh.Run(ctx, "uname -s")
	if err != nil {
		return targetCommandError("detecting agent OS", stderr, err)
	}
	if strings.TrimSpace(osName) != "Linux" {
		return fmt.Errorf("target agent requires Linux, got %q", strings.TrimSpace(osName))
	}

	machine, stderr, err := ssh.Run(ctx, "uname -m")
	if err != nil {
		return targetCommandError("detecting agent architecture", stderr, err)
	}
	arch, err := agentArch(strings.TrimSpace(machine))
	if err != nil {
		return err
	}

	updater := version.NewUpdater()
	binary, releaseTag, err := updater.DownloadTargetBinary("linux", arch)
	if err != nil {
		return fmt.Errorf("downloading signed agent: %w", err)
	}
	digest := sha256.Sum256(binary)
	expectedHash := hex.EncodeToString(digest[:])

	const remoteTemp = "/tmp/oma-agent.bin"
	if _, stderr, err := ssh.RunInput(ctx, "cat > "+remoteTemp, bytes.NewReader(binary)); err != nil {
		return targetCommandError("uploading agent binary", stderr, err)
	}

	remoteHashOutput, stderr, err := ssh.Run(ctx, "sha256sum "+remoteTemp)
	if err != nil {
		return targetCommandError("hashing uploaded agent", stderr, err)
	}
	remoteHash := strings.Fields(remoteHashOutput)
	if len(remoteHash) == 0 || remoteHash[0] != expectedHash {
		return fmt.Errorf("uploaded agent checksum mismatch")
	}

	script := `set -e
bin=/usr/local/bin/oma
old=/usr/local/bin/oma.agent-old
new=/usr/local/bin/oma.agent-new
if [ -e "$bin" ]; then
  sudo -n cp -p "$bin" "$old"
fi
sudo -n install -m 755 ` + remoteTemp + ` "$new"
sudo -n mv -f "$new" "$bin"
rollback() {
  if [ -e "$old" ]; then
    sudo -n install -m 755 "$old" "$bin"
    sudo -n systemctl restart oma-background >/dev/null 2>&1 || true
  fi
}
sudo -n "$bin" background uninstall >/dev/null 2>&1 || true
if ! sudo -n "$bin" background install --backup-role target --agent; then
  rollback
  exit 1
fi
if ! sudo -n systemctl restart oma-background; then
  rollback
  exit 1
fi
rm -f ` + remoteTemp + `
sudo -n rm -f "$old"
`
	if _, stderr, err := ssh.Run(ctx, script); err != nil {
		return targetCommandError("installing agent service", stderr, err)
	}

	selected.AgentVersion = releaseTag
	selected.AutomationOwner = target.AutomationOwnerAgent
	config.Targets[selected.Name] = selected
	if err := config.Save(); err != nil {
		return fmt.Errorf("recording agent version: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Installed agent %s on %s\n", releaseTag, selected.Name)
	return nil
}

type targetAgentDisableCommand struct {
	cmd *cobra.Command
}

func newTargetAgentDisableCommand() *targetAgentDisableCommand {
	t := &targetAgentDisableCommand{}
	t.cmd = &cobra.Command{
		Use:   "disable <name>",
		Short: "Stop agent automation and return ownership to the controller",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	return t
}

func (t *targetAgentDisableCommand) run(cmd *cobra.Command, args []string) error {
	config, selected, err := resolveTargetConfig(args[0])
	if err != nil {
		return err
	}
	if selected.Local {
		return fmt.Errorf("the local target cannot have an agent")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ssh := target.SSHCommand{Destination: selected.SSH}
	if _, stderr, err := ssh.Run(ctx, "sudo -n systemctl disable --now oma-background >/dev/null 2>&1 || true"); err != nil {
		return targetCommandError("stopping agent service", stderr, err)
	}

	selected.AutomationOwner = target.AutomationOwnerController
	config.Targets[selected.Name] = selected
	if err := config.Save(); err != nil {
		return fmt.Errorf("recording automation owner: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Agent automation disabled on %s; controller owns automation\n", selected.Name)
	return nil
}

type targetAgentStatusCommand struct {
	cmd *cobra.Command
}

func newTargetAgentStatusCommand() *targetAgentStatusCommand {
	t := &targetAgentStatusCommand{}
	t.cmd = &cobra.Command{
		Use:   "status <name>",
		Short: "Show target-local agent status and recent logs",
		Args:  cobra.ExactArgs(1),
		RunE:  t.run,
	}
	return t
}

func (t *targetAgentStatusCommand) run(cmd *cobra.Command, args []string) error {
	_, selected, err := resolveTargetConfig(args[0])
	if err != nil {
		return err
	}
	if selected.Local {
		return fmt.Errorf("the local target cannot have an agent")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ssh := target.SSHCommand{Destination: selected.SSH}
	output, stderr, err := ssh.Run(ctx, "systemctl is-active oma-background 2>/dev/null || true\nsystemctl --no-pager --lines=15 status oma-background 2>&1 || true")
	if err != nil {
		return targetCommandError("reading agent status", stderr, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Target: %s\n", selected.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Agent version: %s\n", selected.AgentVersion)
	fmt.Fprintf(cmd.OutOrStdout(), "Automation owner: %s\n", selected.AutomationOwnerOrDefault())
	fmt.Fprint(cmd.OutOrStdout(), output)
	return nil
}

// Helpers

func resolveTargetConfig(name string) (*target.Config, target.Target, error) {
	config, err := target.Load()
	if err != nil {
		return nil, target.Target{}, err
	}
	selected, err := config.Resolve(name, "")
	if err != nil {
		return nil, target.Target{}, err
	}
	return config, selected, nil
}

func agentArch(machine string) (string, error) {
	switch machine {
	case "x86_64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported target architecture %q", machine)
	}
}

func targetCommandError(action, stderr string, err error) error {
	if strings.TrimSpace(stderr) == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, strings.TrimSpace(stderr))
}
