package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
)

func TestSettingsFormBackupsLabelsControllerPathForRemoteTarget(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithTargetName("vps"))
	require.NoError(t, err)

	app := docker.NewApplication(ns, docker.ApplicationSettings{Name: "remote"})
	form := NewSettingsFormBackups(app, nil)

	assert.Contains(t, form.View(), "on this controller")
}
