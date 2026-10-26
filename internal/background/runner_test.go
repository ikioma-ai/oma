package background

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
)

func TestRunnerShutdown(t *testing.T) {
	runner := NewRunner("test")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runner.Run(ctx)
	assert.NoError(t, err)
}

func TestCheckInterval(t *testing.T) {
	assert.Equal(t, 5*time.Minute, CheckInterval)
}

func TestBackupRoleValidation(t *testing.T) {
	role, err := NormalizeBackupRole("")
	require.NoError(t, err)
	assert.Equal(t, BackupRoleController, role)

	role, err = NormalizeBackupRole(BackupRoleTarget)
	require.NoError(t, err)
	assert.Equal(t, BackupRoleTarget, role)

	_, err = NormalizeBackupRole("other")
	require.Error(t, err)

	_, err = ValidateBackupRole(BackupRoleTarget, "vps")
	require.Error(t, err)

	role, err = ValidateBackupRole(BackupRoleTarget, docker.LocalTargetName)
	require.NoError(t, err)
	assert.Equal(t, BackupRoleTarget, role)
}

func TestRunnerBackupAppliesTo(t *testing.T) {
	controller := &Runner{backupRole: BackupRoleController}
	assert.True(t, controller.backupAppliesTo(docker.BackupDestinationController))
	assert.False(t, controller.backupAppliesTo(docker.BackupDestinationTarget))

	targetRunner := &Runner{backupRole: BackupRoleTarget}
	assert.False(t, targetRunner.backupAppliesTo(docker.BackupDestinationController))
	assert.True(t, targetRunner.backupAppliesTo(docker.BackupDestinationTarget))
}

func TestRunnerRejectsTargetRoleForRemoteTarget(t *testing.T) {
	runner := NewRunnerForTargetWithRole("test", "vps", nil, BackupRoleTarget)
	require.Error(t, runner.Run(context.Background()))
}

func TestControllerRunnerReportsTargetBackup(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	app := &docker.Application{Settings: docker.ApplicationSettings{
		Name: "app",
		Backup: docker.BackupSettings{
			AutoBackup:  true,
			Destination: docker.BackupDestinationTarget,
			Path:        "/backups",
		},
	}}

	runner := &Runner{backupRole: BackupRoleController}
	runner.checkBackup(context.Background(), app, &docker.State{})

	assert.Contains(t, logs.String(), "target runner required")
}
