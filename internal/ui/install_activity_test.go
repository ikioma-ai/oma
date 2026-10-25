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

	settings, err := settingsFromCatalogEntry(docker.ApplicationSettings{
		Name:  "myapp",
		Image: "example.com/myapp:v1",
		Host:  "myapp.example.com",
	}, entry, nil)
	require.NoError(t, err)

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

	settings, err := activity.applicationSettings("campfire", "chat.example.com")
	require.NoError(t, err)

	assert.Equal(t, "campfire", settings.CatalogID)
	assert.Equal(t, "builtin", settings.CatalogSource)
	assert.Equal(t, docker.DefaultServicePort, settings.ServicePort)
	assert.Equal(t, docker.HealthCheckPath, settings.HealthCheckPath)
	assert.True(t, settings.VerifyEnabled())
}

func TestSettingsFromCatalogEntryEnvironment(t *testing.T) {
	entry := catalog.Entry{
		Manifest: catalog.Manifest{
			ID: "envapp",
			Env: []catalog.EnvVar{
				{Name: "DATABASE_URL", Required: true, Secret: true},
				{Name: "LOG_LEVEL", Default: "info"},
				{Name: "APP_ENV", Value: "production"},
				{Name: "SECRET_TOKEN", Generate: "hex32", Secret: true},
				{Name: "BASE_URL", From: "base_url"},
			},
		},
		Source: catalog.SourceUser,
	}

	settings, err := settingsFromCatalogEntry(docker.ApplicationSettings{
		Name:  "envapp",
		Image: "example.com/envapp:v1",
		Host:  "envapp.example.com",
	}, entry, map[string]string{
		"DATABASE_URL": "postgres://localhost/envapp",
	})
	require.NoError(t, err)

	assert.Equal(t, "postgres://localhost/envapp", settings.EnvVars["DATABASE_URL"])
	assert.Equal(t, "info", settings.EnvVars["LOG_LEVEL"])
	assert.Equal(t, "production", settings.EnvVars["APP_ENV"])
	assert.Len(t, settings.EnvVars["SECRET_TOKEN"], 64)
	assert.NotContains(t, settings.EnvVars, "BASE_URL")
	assert.Equal(t, []string{"DATABASE_URL", "SECRET_TOKEN"}, settings.SecretEnv)

	restored, err := docker.UnmarshalApplicationSettings(settings.Marshal())
	require.NoError(t, err)
	assert.Equal(t, settings.EnvVars["SECRET_TOKEN"], restored.EnvVars["SECRET_TOKEN"])
	assert.Equal(t, settings.SecretEnv, restored.SecretEnv)
}

func TestSettingsFromCatalogEntryRequiresEnv(t *testing.T) {
	entry := catalog.Entry{
		Manifest: catalog.Manifest{
			ID:  "envapp",
			Env: []catalog.EnvVar{{Name: "DATABASE_URL", Required: true}},
		},
	}

	_, err := settingsFromCatalogEntry(docker.ApplicationSettings{Name: "envapp"}, entry, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}
