package command

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/background"
	"github.com/ikioma-ai/oma/internal/target"
)

type backgroundRunCommand struct {
	cmd        *cobra.Command
	backupRole string
}

func newBackgroundRunCommand() *backgroundRunCommand {
	b := &backgroundRunCommand{}
	b.cmd = &cobra.Command{
		Use:    "run",
		Short:  "Run background tasks (automatic backups and updates)",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE:   b.run,
	}
	b.cmd.Flags().StringVar(&b.backupRole, "backup-role", background.BackupRoleController, "backup role: controller or target")
	return b
}

// Private

func (b *backgroundRunCommand) run(cmd *cobra.Command, args []string) error {
	config, err := target.Load()
	if err != nil {
		return err
	}
	selected, err := config.Resolve(targetFlag(cmd), os.Getenv("OMA_TARGET"))
	if err != nil {
		return err
	}
	transport, err := selected.Transport()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	role, err := background.ValidateBackupRole(b.backupRole, selected.Name)
	if err != nil {
		return err
	}

	runner := background.NewRunnerForTargetWithRole(namespaceFor(cmd, selected), selected.Name, transport, role)
	return runner.Run(ctx)
}
