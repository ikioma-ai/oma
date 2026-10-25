package command

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogListShowsBuiltinEntries(t *testing.T) {
	setCatalogConfigHome(t)

	c := newCatalogListCommand()
	var out bytes.Buffer
	c.cmd.SetOut(&out)

	require.NoError(t, c.run(c.cmd, nil))
	assert.Contains(t, out.String(), "campfire")
	assert.Contains(t, out.String(), "builtin")
	assert.Contains(t, out.String(), "Group chat for teams")
}

func TestCatalogShowBuiltinEntry(t *testing.T) {
	setCatalogConfigHome(t)

	c := newCatalogShowCommand()
	var out bytes.Buffer
	c.cmd.SetOut(&out)

	require.NoError(t, c.run(c.cmd, []string{"campfire"}))
	assert.Contains(t, out.String(), "ID: campfire")
	assert.Contains(t, out.String(), "Source: builtin")
	assert.Contains(t, out.String(), "Image: ghcr.io/basecamp/once-campfire")
}

func TestCatalogListWarnsAboutInvalidEntries(t *testing.T) {
	userDir := setCatalogConfigHome(t)
	writeCatalogFile(t, userDir, "bad.yaml", "schemaVersion: 1\nid: badapp\nkind: worker\n")

	c := newCatalogListCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	c.cmd.SetOut(&out)
	c.cmd.SetErr(&errOut)

	require.NoError(t, c.run(c.cmd, nil))
	assert.Contains(t, out.String(), "campfire")
	assert.Contains(t, errOut.String(), "warning:")
	assert.Contains(t, errOut.String(), "bad.yaml")
}

func TestCatalogValidateValid(t *testing.T) {
	setCatalogConfigHome(t)

	c := newCatalogValidateCommand()
	var out bytes.Buffer
	c.cmd.SetOut(&out)

	require.NoError(t, c.run(c.cmd, nil))
	assert.Contains(t, out.String(), "Catalog is valid.")
}

func TestCatalogValidateInvalid(t *testing.T) {
	userDir := setCatalogConfigHome(t)
	writeCatalogFile(t, userDir, "bad.yaml", "schemaVersion: 1\nid: badapp\nkind: worker\n")

	c := newCatalogValidateCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	c.cmd.SetOut(&out)
	c.cmd.SetErr(&errOut)

	err := c.run(c.cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 catalog entry is invalid")
	assert.Contains(t, errOut.String(), "invalid:")
	assert.Contains(t, errOut.String(), "bad.yaml")
}

func TestRootRegistersCatalogCommand(t *testing.T) {
	root := NewRootCommand()

	cmd, _, err := root.cmd.Find([]string{"catalog"})
	require.NoError(t, err)
	assert.Equal(t, "catalog", cmd.Name())
}

// Helpers

func setCatalogConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "oma", "apps")
}

func writeCatalogFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func TestCatalogAddWritesUserManifest(t *testing.T) {
	userDir := setCatalogConfigHome(t)
	source := writeSourceManifest(t, `schemaVersion: 1
id: myapp
name: My App
description: A local app
category: Utilities
image: example.com/myapp:v1
`)

	c := newCatalogAddCommand()
	var out bytes.Buffer
	c.cmd.SetOut(&out)

	require.NoError(t, c.run(c.cmd, []string{source}))
	assert.Contains(t, out.String(), "Added myapp")

	written, err := os.ReadFile(filepath.Join(userDir, "myapp.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(written), "id: myapp")

	list := newCatalogListCommand()
	var listOut bytes.Buffer
	list.cmd.SetOut(&listOut)
	require.NoError(t, list.run(list.cmd, nil))
	assert.Contains(t, listOut.String(), "myapp")
	assert.Contains(t, listOut.String(), "user")
}

func TestCatalogAddRejectsInvalidManifest(t *testing.T) {
	userDir := setCatalogConfigHome(t)
	source := writeSourceManifest(t, `schemaVersion: 1
id: badapp
kind: worker
`)

	c := newCatalogAddCommand()
	err := c.run(c.cmd, []string{source})
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(userDir, "badapp.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestCatalogAddRequiresForceToReplace(t *testing.T) {
	setCatalogConfigHome(t)
	source := writeSourceManifest(t, `schemaVersion: 1
id: myapp
name: My App
description: A local app
category: Utilities
image: example.com/myapp:v1
`)

	first := newCatalogAddCommand()
	require.NoError(t, first.run(first.cmd, []string{source}))

	second := newCatalogAddCommand()
	err := second.run(second.cmd, []string{source})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	third := newCatalogAddCommand()
	require.NoError(t, third.cmd.Flags().Set("force", "true"))
	require.NoError(t, third.run(third.cmd, []string{source}))
}

// Helpers

func writeSourceManifest(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
