package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
)

func TestSettingsFormCatalogShowsChanges(t *testing.T) {
	app := &docker.Application{Settings: docker.ApplicationSettings{
		Name:            "campfire",
		Host:            "chat.localhost",
		CatalogID:       "campfire",
		Image:           "example.com/campfire:old",
		HealthCheckPath: "/old",
	}}

	form := NewSettingsFormCatalog(app)
	require.True(t, form.hasEntry)
	require.NotEmpty(t, form.changes)
	assert.Contains(t, form.View(), "Image")
	assert.Contains(t, form.View(), "Health path")

	section, cmd := form.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)
	_, ok := section.(SettingsFormCatalog)
	require.True(t, ok)

	msg := cmd()
	apply, ok := msg.(SettingsSectionApplyMsg)
	require.True(t, ok, "expected SettingsSectionApplyMsg, got %T", msg)
	assert.True(t, apply.Pull, "image changed, so the image must be pulled")
	assert.Equal(t, "ghcr.io/basecamp/once-campfire", apply.Settings.Image)
}

func TestSettingsFormCatalogNoChangesCloses(t *testing.T) {
	app := &docker.Application{Settings: docker.ApplicationSettings{
		Name:      "campfire",
		Host:      "chat.localhost",
		CatalogID: "campfire",
		Image:     "ghcr.io/basecamp/once-campfire",
	}}

	form := NewSettingsFormCatalog(app)
	require.True(t, form.hasEntry)
	assert.Empty(t, form.changes)

	_, cmd := form.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)
	_, ok := cmd().(SettingsSectionCancelMsg)
	assert.True(t, ok)
}

func TestSettingsFormCatalogWithoutEntry(t *testing.T) {
	app := &docker.Application{Settings: docker.ApplicationSettings{
		Name: "custom",
		Host: "custom.localhost",
	}}

	form := NewSettingsFormCatalog(app)
	assert.False(t, form.hasEntry)
	assert.Contains(t, form.StatusLine(), "no catalog entry")

	_, cmd := form.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)
	_, ok := cmd().(SettingsSectionCancelMsg)
	assert.True(t, ok)
}
