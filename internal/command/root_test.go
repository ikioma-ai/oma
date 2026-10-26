package command

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
)

func TestWithApplicationFound(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithApplications(
		docker.ApplicationSettings{Name: "myapp", Host: "myapp.localhost"},
	))
	require.NoError(t, err)

	var called bool
	err = withApplication(ns, "myapp.localhost", "testing", func(app *docker.Application) error {
		called = true
		assert.Equal(t, "myapp", app.Settings.Name)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
}

func TestWithApplicationNotFound(t *testing.T) {
	ns, err := docker.NewNamespace("test")
	require.NoError(t, err)

	err = withApplication(ns, "missing.localhost", "testing", func(app *docker.Application) error {
		t.Fatal("should not be called")
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `no application found at host "missing.localhost"`)
}

func TestWithApplicationError(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithApplications(
		docker.ApplicationSettings{Name: "myapp", Host: "myapp.localhost"},
	))
	require.NoError(t, err)

	err = withApplication(ns, "myapp.localhost", "starting", func(app *docker.Application) error {
		return assert.AnError
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "starting application")
}

func TestExitCodeIgnoresOSExecErrors(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 7").Run()
	require.Error(t, err)
	_, ok := ExitCode(err)
	assert.False(t, ok)
}
