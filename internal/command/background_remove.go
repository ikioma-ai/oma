package command

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/service"
	"github.com/ikioma-ai/oma/internal/target"
)

type backgroundUninstallCommand struct {
	cmd *cobra.Command
}

func newBackgroundUninstallCommand() *backgroundUninstallCommand {
	b := &backgroundUninstallCommand{}
	b.cmd = &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the background tasks system service",
		Args:  cobra.NoArgs,
		RunE:  b.run,
	}
	return b
}

// Private

func (b *backgroundUninstallCommand) run(cmd *cobra.Command, args []string) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("must be run as root")
	}

	config, err := target.Load()
	if err != nil {
		return err
	}
	selected, err := config.Resolve(targetFlag(cmd), os.Getenv("OMA_TARGET"))
	if err != nil {
		return err
	}
	namespace := namespaceFor(cmd, selected)

	ctx := context.Background()

	svc, err := service.New()
	if err != nil {
		return err
	}

	serviceName := backgroundServiceName(namespace, selected.Name)

	if !svc.IsInstalled(serviceName) {
		fmt.Printf("Service %s is not installed\n", svc.ServiceName(serviceName))
		return nil
	}

	if err := svc.Remove(ctx, serviceName); err != nil {
		return fmt.Errorf("removing service: %w", err)
	}

	fmt.Printf("Uninstalled %s\n", svc.ServiceName(serviceName))
	return nil
}
