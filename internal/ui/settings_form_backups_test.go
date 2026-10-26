package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
)

func TestSettingsFormBackupsShowsControllerDestinationForRemoteTarget(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithTargetName("vps"))
	require.NoError(t, err)

	app := docker.NewApplication(ns, docker.ApplicationSettings{Name: "remote"})
	form := NewSettingsFormBackups(app, nil)

	assert.Contains(t, form.StatusLine(), "controller")
}

func TestSettingsFormBackupsShowsTargetRunnerRequirement(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithTargetName("vps"))
	require.NoError(t, err)

	app := docker.NewApplication(ns, docker.ApplicationSettings{
		Name:   "remote",
		Backup: docker.BackupSettings{Destination: docker.BackupDestinationTarget, Path: "/backups"},
	})
	form := NewSettingsFormBackups(app, nil)

	assert.Contains(t, form.StatusLine(), "target runner is required")
}

func TestSettingsFormBackupsShowsObjectStorageRequirement(t *testing.T) {
	app := &docker.Application{Settings: docker.ApplicationSettings{
		Name:   "remote",
		Backup: docker.BackupSettings{Destination: docker.BackupDestinationObject},
	}}
	form := NewSettingsFormBackups(app, nil)

	assert.Contains(t, form.StatusLine(), "object storage")
}

func TestSettingsFormBackupsObjectDestinationSkipsPath(t *testing.T) {
	app := &docker.Application{Settings: docker.ApplicationSettings{
		Name:   "remote",
		Backup: docker.BackupSettings{Destination: docker.BackupDestinationObject},
	}}
	form := NewSettingsFormBackups(app, nil)
	assert.Equal(t, backupDestinationObject, form.form.SelectField(backupsDestinationField).Index())
}
