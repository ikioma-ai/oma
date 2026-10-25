package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ikioma-ai/oma/internal/docker"
)

func TestSettingsFormEnvironmentMasksSecretValues(t *testing.T) {
	form := NewSettingsFormEnvironment(docker.ApplicationSettings{
		EnvVars:   map[string]string{"TOKEN": "topsecret"},
		SecretEnv: []string{"TOKEN"},
	})

	assert.Equal(t, "topsecret", form.form.TextField(1).Value())
	assert.NotContains(t, form.form.TextField(1).View(), "topsecret")
}
