package command

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ikioma-ai/oma/internal/catalog"
)

type catalogCommand struct {
	cmd *cobra.Command
}

func newCatalogCommand() *catalogCommand {
	c := &catalogCommand{}
	c.cmd = &cobra.Command{
		Use:   "catalog",
		Short: "Manage the application catalog",
	}

	c.cmd.AddCommand(newCatalogAddCommand().cmd)
	c.cmd.AddCommand(newCatalogApplyCommand().cmd)
	c.cmd.AddCommand(newCatalogListCommand().cmd)
	c.cmd.AddCommand(newCatalogShowCommand().cmd)
	c.cmd.AddCommand(newCatalogValidateCommand().cmd)

	return c
}

type catalogAddCommand struct {
	cmd   *cobra.Command
	force bool
}

func newCatalogAddCommand() *catalogAddCommand {
	c := &catalogAddCommand{}
	c.cmd = &cobra.Command{
		Use:   "add <file>",
		Short: "Add or update a catalog entry from a manifest file",
		Args:  cobra.ExactArgs(1),
		RunE:  c.run,
	}
	c.cmd.Flags().BoolVar(&c.force, "force", false, "replace an existing catalog entry")
	return c
}

type catalogListCommand struct {
	cmd *cobra.Command
}

func newCatalogListCommand() *catalogListCommand {
	c := &catalogListCommand{}
	c.cmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List catalog entries",
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}
	return c
}

type catalogShowCommand struct {
	cmd *cobra.Command
}

func newCatalogShowCommand() *catalogShowCommand {
	c := &catalogShowCommand{}
	c.cmd = &cobra.Command{
		Use:   "show <id>",
		Short: "Show a catalog entry",
		Args:  cobra.ExactArgs(1),
		RunE:  c.run,
	}
	return c
}

type catalogValidateCommand struct {
	cmd *cobra.Command
}

func newCatalogValidateCommand() *catalogValidateCommand {
	c := &catalogValidateCommand{}
	c.cmd = &cobra.Command{
		Use:   "validate",
		Short: "Validate the embedded and user catalogs",
		Args:  cobra.NoArgs,
		RunE:  c.run,
	}
	return c
}

// Private

func (c *catalogAddCommand) run(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	manifest, err := catalog.Parse(data)
	if err != nil {
		return fmt.Errorf("validating manifest %s: %w", args[0], err)
	}

	dir, err := catalog.UserDir()
	if err != nil {
		return fmt.Errorf("resolving user catalog directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating user catalog directory: %w", err)
	}

	destination := filepath.Join(dir, manifest.ID+".yaml")
	if !c.force {
		if _, err := os.Stat(destination); err == nil {
			return fmt.Errorf("catalog entry %q already exists at %s (use --force to replace it)", manifest.ID, destination)
		}
	}

	if err := os.WriteFile(destination, data, 0o644); err != nil {
		return fmt.Errorf("writing catalog entry: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added %s to %s\n", manifest.ID, destination)
	return nil
}

func (c *catalogListCommand) run(cmd *cobra.Command, args []string) error {
	store, err := loadCatalog(cmd)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION\tCATEGORY\tSOURCE")

	for _, entry := range store.List() {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", entry.ID, entry.Name, entry.Description, entry.Category, entry.Source)
	}

	return w.Flush()
}

func (c *catalogShowCommand) run(cmd *cobra.Command, args []string) error {
	store, err := loadCatalog(cmd)
	if err != nil {
		return err
	}

	entry, ok := store.Get(args[0])
	if !ok {
		return fmt.Errorf("catalog entry %q not found", args[0])
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "ID: %s\n", entry.ID)
	fmt.Fprintf(out, "Name: %s\n", entry.Name)
	fmt.Fprintf(out, "Description: %s\n", entry.Description)
	fmt.Fprintf(out, "Category: %s\n", entry.Category)
	fmt.Fprintf(out, "Source: %s\n", entry.Source)
	fmt.Fprintf(out, "Image: %s\n", entry.Image)
	if entry.Version != "" {
		fmt.Fprintf(out, "Version: %s\n", entry.Version)
	}
	if entry.Hostname != "" {
		fmt.Fprintf(out, "Hostname: %s\n", entry.Hostname)
	}
	fmt.Fprintf(out, "Web: port=%d health=%s tls=%s verify=%t\n", entry.Web.Port, entry.Web.HealthPath, entry.Web.TLS, entry.VerifyEnabled())

	if len(entry.Storage) > 0 {
		fmt.Fprintln(out, "Storage:")
		for _, storage := range entry.Storage {
			fmt.Fprintf(out, "  %s -> %s", storage.Name, storage.Path)
			if storage.Primary {
				fmt.Fprint(out, " (primary)")
			}
			if storage.Backup {
				fmt.Fprint(out, " (backup)")
			}
			fmt.Fprintln(out)
		}
	}

	if len(entry.Env) > 0 {
		fmt.Fprintln(out, "Environment:")
		for _, env := range entry.Env {
			fmt.Fprintf(out, "  %s", env.Name)
			if env.Required {
				fmt.Fprint(out, " (required)")
			}
			if env.Secret {
				fmt.Fprint(out, " (secret)")
			}
			fmt.Fprintln(out)
		}
	}

	return nil
}

func (c *catalogValidateCommand) run(cmd *cobra.Command, args []string) error {
	store, err := catalog.Load()
	if err != nil {
		return err
	}

	issues := store.Issues()
	if len(issues) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Catalog is valid.")
		return nil
	}

	for _, issue := range issues {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid: %v\n", issue)
	}

	if len(issues) == 1 {
		return fmt.Errorf("1 catalog entry is invalid")
	}
	return fmt.Errorf("%d catalog entries are invalid", len(issues))
}

// Helpers

func loadCatalog(cmd *cobra.Command) (*catalog.Store, error) {
	store, err := catalog.Load()
	if err != nil {
		return nil, err
	}

	for _, issue := range store.Issues() {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", issue)
	}

	return store, nil
}
