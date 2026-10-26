package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/catalog"
	"github.com/ikioma-ai/oma/internal/catalogapply"
	"github.com/ikioma-ai/oma/internal/docker"
)

type catalogApplyCommand struct {
	cmd      *cobra.Command
	dryRun   bool
	yes      bool
	noVerify bool
}

func newCatalogApplyCommand() *catalogApplyCommand {
	c := &catalogApplyCommand{}
	c.cmd = &cobra.Command{
		Use:   "apply <host>",
		Short: "Apply catalog changes to an installed application",
		Args:  cobra.ExactArgs(1),
		RunE:  WithNamespace(c.run),
	}
	c.cmd.Flags().BoolVar(&c.dryRun, "dry-run", false, "show the diff without applying it")
	c.cmd.Flags().BoolVar(&c.yes, "yes", false, "apply without confirmation")
	c.cmd.Flags().BoolVar(&c.noVerify, "no-verify", false, "skip post-apply HTTP verification")
	return c
}

// Private

func (c *catalogApplyCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
	host := args[0]

	app, err := findApplication(ns, host)
	if err != nil {
		return err
	}

	if app.Settings.CatalogID == "" {
		return fmt.Errorf("application %q is not managed by a catalog entry", host)
	}

	store, err := catalog.Load()
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	entry, ok := store.Get(app.Settings.CatalogID)
	if !ok {
		fmt.Fprintf(cmd.OutOrStdout(), "No catalog entry %q; nothing to apply.\n", app.Settings.CatalogID)
		return nil
	}

	changes := catalogapply.Diff(app.Settings, entry)
	if len(changes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Catalog is already applied; no changes.")
		return nil
	}

	printCatalogChanges(cmd.OutOrStdout(), changes)

	if c.dryRun {
		return nil
	}

	if !c.yes {
		confirmed, err := confirmCatalogApply(cmd)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(cmd.OutOrStdout(), "Apply cancelled.")
			return nil
		}
	}

	settings := catalogapply.Build(app.Settings, entry)
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("validating catalog settings: %w", err)
	}

	if err := ns.Setup(ctx); err != nil {
		return fmt.Errorf("%w: %w", docker.ErrSetupFailed, err)
	}

	pull := catalogapply.ImageChanged(app.Settings, entry)

	return runWithProgress("Applying "+host, func(progress docker.DeployProgressCallback) error {
		if err := app.ApplySettings(ctx, settings, pull, progress); err != nil {
			return fmt.Errorf("%w: %w", docker.ErrDeployFailed, err)
		}
		if !c.noVerify {
			if err := app.VerifyHTTPOrRemove(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

// Helpers

func printCatalogChanges(w io.Writer, changes []catalogapply.Change) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "FIELD\tOLD\tNEW")
	for _, change := range changes {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", change.Field, change.Old, change.New)
	}
	tw.Flush()
}

func confirmCatalogApply(cmd *cobra.Command) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), "Apply these changes? [y/N]: ")

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
