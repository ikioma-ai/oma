package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/target"
)

func TestTargetListShowsLocal(t *testing.T) {
	setTargetConfigHome(t)

	c := newTargetListCommand()
	var out bytes.Buffer
	c.cmd.SetOut(&out)

	require.NoError(t, c.run(c.cmd, nil))
	assert.Contains(t, out.String(), "local")
	assert.Contains(t, out.String(), "*")
	assert.Contains(t, out.String(), "(env)")
}

func TestTargetAddAndList(t *testing.T) {
	setTargetConfigHome(t)

	add := newTargetAddCommand()
	add.ssh = "deploy@example.com"
	var addOut bytes.Buffer
	add.cmd.SetOut(&addOut)
	require.NoError(t, add.run(add.cmd, []string{"vps"}))
	assert.Contains(t, addOut.String(), "Added target vps")

	list := newTargetListCommand()
	var listOut bytes.Buffer
	list.cmd.SetOut(&listOut)
	require.NoError(t, list.run(list.cmd, nil))
	assert.Contains(t, listOut.String(), "vps")
	assert.Contains(t, listOut.String(), "deploy@example.com")
}

func TestTargetAddRequiresSSH(t *testing.T) {
	setTargetConfigHome(t)

	c := newTargetAddCommand()
	require.Error(t, c.run(c.cmd, []string{"vps"}))
}

func TestTargetDefaultAndRemoveDefault(t *testing.T) {
	setTargetConfigHome(t)

	add := newTargetAddCommand()
	add.ssh = "deploy@example.com"
	require.NoError(t, add.run(add.cmd, []string{"vps"}))

	def := newTargetDefaultCommand()
	require.NoError(t, def.run(def.cmd, []string{"vps"}))

	remove := newTargetRemoveCommand()
	require.Error(t, remove.run(remove.cmd, []string{"vps"}))

	remove = newTargetRemoveCommand()
	remove.newDefault = "local"
	require.NoError(t, remove.run(remove.cmd, []string{"vps"}))

	configData, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "oma", "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configData), "default: local")
}

func TestTargetRemoveLocalRejected(t *testing.T) {
	setTargetConfigHome(t)

	c := newTargetRemoveCommand()
	require.Error(t, c.run(c.cmd, []string{"local"}))
}

func TestWithNamespaceUnknownTargetFailsBeforeDocker(t *testing.T) {
	setTargetConfigHome(t)
	t.Setenv("OMA_TARGET", "missing")

	cmd := &cobra.Command{}
	err := WithNamespace(func(context.Context, *docker.Namespace, *cobra.Command, []string) error {
		t.Fatal("callback should not run for an unknown target")
		return nil
	})(cmd, nil)

	require.ErrorIs(t, err, target.ErrUnknownTarget)
}

func TestRootRegistersTargetCommand(t *testing.T) {
	root := NewRootCommand()

	cmd, _, err := root.cmd.Find([]string{"target"})
	require.NoError(t, err)
	assert.Equal(t, "target", cmd.Name())

	cmd, _, err = root.cmd.Find([]string{"target", "add"})
	require.NoError(t, err)
	assert.Equal(t, "add", cmd.Name())
}

// Helpers

func setTargetConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestTargetCheckUnknownTarget(t *testing.T) {
	setTargetConfigHome(t)
	t.Setenv("OMA_TARGET", "missing")

	c := newTargetCheckCommand()
	var out bytes.Buffer
	c.cmd.SetOut(&out)

	err := c.run(c.cmd, nil)
	require.ErrorIs(t, err, target.ErrUnknownTarget)
}

func TestRootRegistersTargetCheckCommand(t *testing.T) {
	root := NewRootCommand()

	cmd, _, err := root.cmd.Find([]string{"target", "check"})
	require.NoError(t, err)
	assert.Equal(t, "check", cmd.Name())
}

func TestTargetSetupUnknownRequiresSSH(t *testing.T) {
	setTargetConfigHome(t)

	c := newTargetSetupCommand()
	err := c.run(c.cmd, []string{"missing"})
	require.Error(t, err)
}

func TestTargetSetupRejectsLocal(t *testing.T) {
	setTargetConfigHome(t)

	c := newTargetSetupCommand()
	c.yes = true
	err := c.run(c.cmd, []string{"local"})
	require.Error(t, err)
}

func TestConfirmTargetSetup(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("y\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	ok, err := confirmTargetSetup(cmd)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, out.String(), "Prepare this target")
}
