package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/catalog"
	"github.com/ikioma-ai/oma/internal/docker"
)

func TestSettingsFromCatalogEntry(t *testing.T) {
	verify := false
	entry := catalog.Entry{
		Manifest: catalog.Manifest{
			ID: "myapp",
			Web: catalog.Web{
				Port:          3000,
				HealthPath:    "/healthz",
				HealthPort:    3001,
				HealthTimeout: "10s",
				TLS:           "off",
				Verify:        &verify,
			},
			Storage: []catalog.Storage{
				{Name: "data", Path: "/data", Primary: true, Backup: true},
				{Name: "data", Path: "/storage"},
			},
		},
		Source: catalog.SourceUser,
	}

	settings := settingsFromCatalogEntry(docker.ApplicationSettings{
		Name:  "myapp",
		Image: "example.com/myapp:v1",
		Host:  "myapp.example.com",
	}, entry)

	assert.Equal(t, "myapp", settings.CatalogID)
	assert.Equal(t, "user", settings.CatalogSource)
	assert.Equal(t, 3000, settings.ServicePort)
	assert.Equal(t, "/healthz", settings.HealthCheckPath)
	assert.Equal(t, 3001, settings.HealthCheckPort)
	assert.Equal(t, "10s", settings.HealthCheckTimeout)
	assert.Equal(t, "off", settings.TLSMode)
	require.NotNil(t, settings.Verify)
	assert.False(t, *settings.Verify)
	assert.Equal(t, []docker.StorageMount{
		{Path: "/data", Primary: true, Backup: true},
		{Path: "/storage"},
	}, settings.Storage)
}

func TestApplicationSettingsUsesSelectedCatalogEntry(t *testing.T) {
	activity := &InstallActivity{
		imageRef:  "ghcr.io/basecamp/once-campfire",
		catalogID: "campfire",
	}

	settings := activity.applicationSettings("campfire", "chat.example.com")

	assert.Equal(t, "campfire", settings.CatalogID)
	assert.Equal(t, "builtin", settings.CatalogSource)
	assert.Equal(t, docker.DefaultServicePort, settings.ServicePort)
	assert.Equal(t, docker.HealthCheckPath, settings.HealthCheckPath)
	assert.True(t, settings.VerifyEnabled())
}
