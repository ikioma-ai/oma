package background

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ikioma-ai/oma/internal/brand"
	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/target"
	"github.com/ikioma-ai/oma/internal/userstats"
	"github.com/ikioma-ai/oma/internal/version"
)

const CheckInterval = 5 * time.Minute

type Runner struct {
	namespace  string
	targetName string
	transport  target.Transport
	userStats  bool
}

func NewRunner(namespace string) *Runner {
	return NewRunnerForTarget(namespace, docker.LocalTargetName, target.LocalTransport{})
}

func NewRunnerForTarget(namespace, targetName string, transport target.Transport) *Runner {
	return &Runner{
		namespace:  namespace,
		targetName: targetName,
		transport:  transport,
		userStats:  targetName == docker.LocalTargetName,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	slog.Info("Starting background runner", "namespace", r.namespace, "target", r.targetName,
		"backup_destination", "controller", "check_interval", CheckInterval)

	if r.userStats {
		scraper := userstats.NewScraper(r.namespace)
		go scraper.Run(ctx)
	}

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down")
			return nil
		case <-ticker.C:
			if updated, err := r.check(ctx); err != nil {
				slog.Error("Check failed", "error", err)
			} else if updated {
				return nil
			}
		}
	}
}

// Private

func (r *Runner) check(ctx context.Context) (bool, error) {
	client, closer, err := r.transport.Connect(ctx)
	if err != nil {
		return false, fmt.Errorf("connecting to target %q: %w", r.targetName, err)
	}

	options := []docker.NamespaceOption{
		docker.WithClient(client),
		docker.WithTargetName(r.targetName),
	}
	if closer != nil {
		options = append(options, docker.WithCloser(closer))
	}

	ns, err := docker.RestoreNamespace(ctx, r.namespace, options...)
	if err != nil {
		if closer != nil {
			_ = closer.Close()
		} else {
			_ = client.Close()
		}
		return false, fmt.Errorf("restoring namespace: %w", err)
	}
	defer ns.Close()

	state, err := ns.LoadState(ctx)
	if err != nil {
		return false, fmt.Errorf("loading state: %w", err)
	}

	if r.checkSelfUpdate(ctx, ns, state) {
		return true, nil
	}

	for _, app := range ns.Applications() {
		if !app.Running {
			continue
		}

		r.checkUpdate(ctx, app, state)
		r.checkBackup(ctx, app, state)
	}

	return false, nil
}

func (r *Runner) checkSelfUpdate(ctx context.Context, ns *docker.Namespace, state *docker.State) bool {
	if os.Getenv(brand.NoSelfUpdateEnv) != "" {
		return false
	}

	if !state.SelfUpdateDue() {
		return false
	}

	slog.Info("Checking for " + brand.DisplayName + " update")

	err := version.NewUpdater().UpdateBinary()
	state.RecordSelfUpdate(err)
	if saveErr := ns.SaveState(ctx, state); saveErr != nil {
		slog.Error("Failed to save state after self-update check", "error", saveErr)
	}

	if err != nil {
		slog.Error("Self-update failed", "error", err)
		return false
	}

	slog.Info("Self-update complete, restarting")
	return true
}

func (r *Runner) checkUpdate(ctx context.Context, app *docker.Application, state *docker.State) {
	if !app.Settings.AutoUpdate {
		return
	}
	if !state.UpdateDue(app.Settings.Name) {
		return
	}

	slog.Info("Running auto-update", "app", app.Settings.Name)

	changed, err := app.Update(ctx, nil)
	if err != nil {
		slog.Error("Auto-update failed", "app", app.Settings.Name, "error", err)
	} else if changed {
		slog.Info("Auto-update completed", "app", app.Settings.Name)
	} else {
		slog.Info("Already up to date", "app", app.Settings.Name)
	}
}

func (r *Runner) checkBackup(ctx context.Context, app *docker.Application, state *docker.State) {
	if !app.Settings.Backup.AutoBackup {
		return
	}
	if !state.BackupDue(app.Settings.Name) {
		return
	}

	slog.Info("Running auto-backup", "app", app.Settings.Name)

	if err := app.Backup(ctx); err != nil {
		slog.Error("Auto-backup failed", "app", app.Settings.Name, "error", err)
	} else {
		slog.Info("Auto-backup completed", "app", app.Settings.Name)
	}

	if err := app.TrimBackups(); err != nil {
		slog.Error("Backup trim failed", "app", app.Settings.Name, "error", err)
	}
}
