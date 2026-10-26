package catalogapply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/catalog"
	"github.com/ikioma-ai/oma/internal/docker"
)

func TestDiffOnlyIncludesAllowedFields(t *testing.T) {
	verify := false
	nextVerify := true
	current := docker.ApplicationSettings{
		Name:               "app",
		Image:              "example.com/app:v1",
		Host:               "app.example.com",
		ServicePort:        3000,
		HealthCheckPath:    "/up",
		HealthCheckPort:    3000,
		HealthCheckTimeout: "5s",
		TLSMode:            "auto",
		Verify:             &verify,
		Resources:          docker.ContainerResources{CPUs: 1, MemoryMB: 256},
		Storage: []docker.StorageMount{
			{Path: "/storage", Primary: true, Backup: true},
			{Path: "/rails/storage"},
		},
		EnvVars:   map[string]string{"SECRET": "keep"},
		SecretEnv: []string{"SECRET"},
	}

	entry := catalog.Entry{
		ID: "app",
		Web: catalog.Web{
			Port:          3000,
			HealthPath:    "/healthz",
			HealthPort:    3000,
			HealthTimeout: "10s",
			TLS:           "off",
			Verify:        &nextVerify,
		},
		Resources: catalog.Resources{CPUs: 2, MemoryMB: 512},
		Storage: []catalog.Storage{
			{Name: "data", Path: "/storage", Primary: true, Backup: true},
			{Name: "data", Path: "/data"},
		},
		Env:    []catalog.EnvVar{{Name: "SECRET", Value: "replace"}},
		Source: catalog.SourceUser,
	}

	changes := Diff(current, entry)
	fields := make([]string, len(changes))
	for i, change := range changes {
		fields[i] = change.Field
	}

	assert.Equal(t, []string{
		"Image",
		"Health path",
		"Health timeout",
		"TLS mode",
		"Verification",
		"CPU limit",
		"Memory limit",
		"Mount target",
	}, fields)
	assert.Equal(t, "/data", changes[len(changes)-1].New)
}

func TestBuildPreservesSecretsAndEnvironment(t *testing.T) {
	nextVerify := true
	current := docker.ApplicationSettings{
		Name:       "app",
		Image:      "example.com/app:v1",
		Host:       "app.example.com",
		EnvVars:    map[string]string{"SECRET": "keep"},
		SecretEnv:  []string{"SECRET"},
		Keys:       docker.Keys{SecretKeyBase: "key", VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv"},
		SMTP:       docker.SMTPSettings{Server: "smtp.example.com"},
		Registry:   docker.RegistrySettings{Host: "example.com", Username: "user", Password: "pass"},
		Backup:     docker.BackupSettings{Path: "/backups", AutoBackup: true},
		AutoUpdate: true,
		Storage: []docker.StorageMount{
			{Path: "/storage", Primary: true, Backup: true},
		},
	}

	entry := catalog.Entry{
		ID:    "app",
		Image: "example.com/app:v1",
		Web: catalog.Web{
			Port:          3000,
			HealthPath:    "/healthz",
			HealthPort:    3001,
			HealthTimeout: "10s",
			TLS:           "off",
			Verify:        &nextVerify,
		},
		Resources: catalog.Resources{CPUs: 2, MemoryMB: 512},
		Storage: []catalog.Storage{
			{Name: "data", Path: "/storage", Primary: true},
			{Name: "data", Path: "/data"},
		},
		Source: catalog.SourceUser,
	}

	next := Build(current, entry)

	assert.Equal(t, "example.com/app:v1", next.Image)
	assert.Equal(t, 3000, next.ServicePort)
	assert.Equal(t, "/healthz", next.HealthCheckPath)
	assert.Equal(t, 3001, next.HealthCheckPort)
	assert.Equal(t, "10s", next.HealthCheckTimeout)
	assert.Equal(t, "off", next.TLSMode)
	assert.True(t, next.DisableTLS)
	require.NotNil(t, next.Verify)
	assert.True(t, *next.Verify)
	assert.Equal(t, docker.ContainerResources{CPUs: 2, MemoryMB: 512}, next.Resources)
	assert.Equal(t, []docker.StorageMount{
		{Path: "/storage", Primary: true, Backup: true},
		{Path: "/data"},
	}, next.Storage)

	assert.Equal(t, current.EnvVars, next.EnvVars)
	assert.Equal(t, current.SecretEnv, next.SecretEnv)
	assert.Equal(t, current.Keys, next.Keys)
	assert.Equal(t, current.SMTP, next.SMTP)
	assert.Equal(t, current.Registry, next.Registry)
	assert.Equal(t, current.Backup, next.Backup)
	assert.True(t, next.AutoUpdate)
}

func TestBuildDoesNotRemoveEnvironment(t *testing.T) {
	current := docker.ApplicationSettings{
		EnvVars:   map[string]string{"SECRET": "keep"},
		SecretEnv: []string{"SECRET"},
	}
	entry := catalog.Entry{ID: "app"}

	next := Build(current, entry)

	assert.Equal(t, map[string]string{"SECRET": "keep"}, next.EnvVars)
	assert.Equal(t, []string{"SECRET"}, next.SecretEnv)
}

func TestImageChanged(t *testing.T) {
	entry := catalog.Entry{Image: "example.com/app:v2"}

	assert.True(t, ImageChanged(docker.ApplicationSettings{Image: "example.com/app:v1"}, entry))
	assert.False(t, ImageChanged(docker.ApplicationSettings{Image: "example.com/app:v2"}, entry))
}
