package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/catalog"
)

func TestInstallAppList_SelectKnownApp(t *testing.T) {
	list := NewInstallAppList()

	// First item (Campfire) is selected by default; press enter
	list, cmd := list.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)

	// Menu emits MenuSelectMsg; feed it back to get the app list's own message
	_, cmd = list.Update(cmd().(MenuSelectMsg))
	require.NotNil(t, cmd)

	msg := cmd()
	selected, ok := msg.(InstallAppSelectedMsg)
	assert.True(t, ok, "expected InstallAppSelectedMsg, got %T", msg)
	assert.Equal(t, "ghcr.io/basecamp/once-campfire", selected.ImageRef)
	assert.Equal(t, "campfire", selected.CatalogID)
}

func TestInstallAppList_SelectOther(t *testing.T) {
	list := NewInstallAppList()

	// Wrap up from first item to reach last item ("Custom Docker image")
	list, _ = list.Update(keyPressMsg("up"))
	list, cmd := list.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)

	_, cmd = list.Update(cmd().(MenuSelectMsg))
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(InstallCustomSelectedMsg)
	assert.True(t, ok, "expected InstallCustomSelectedMsg, got %T", msg)
}

func TestInstallAppList_NavigateAndSelect(t *testing.T) {
	list := NewInstallAppList()

	// Navigate down to Fizzy
	list, _ = list.Update(keyPressMsg("down"))
	list, cmd := list.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)

	_, cmd = list.Update(cmd().(MenuSelectMsg))
	require.NotNil(t, cmd)

	msg := cmd()
	selected, ok := msg.(InstallAppSelectedMsg)
	require.True(t, ok, "expected InstallAppSelectedMsg, got %T", msg)
	assert.Equal(t, "ghcr.io/basecamp/fizzy", selected.ImageRef)
}

func TestInstallAppList_View(t *testing.T) {
	list := NewInstallAppList()
	view := list.View()

	assert.Contains(t, view, "Choose an application")
	assert.Contains(t, view, "Campfire")
	assert.Contains(t, view, "Group chat for teams")
	assert.Contains(t, view, "Communication")
	assert.Contains(t, view, "builtin")
	assert.Contains(t, view, "Fizzy")
	assert.Contains(t, view, "Writebook")
	assert.Contains(t, view, "Custom Docker image")
}

func TestInstallAppList_WarnsAboutInvalidCatalogEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	appDir := filepath.Join(dir, "oma", "apps")
	require.NoError(t, os.MkdirAll(appDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(appDir, "bad.yaml"),
		[]byte("schemaVersion: 1\nid: badapp\nkind: worker\n"),
		0o644,
	))

	list := NewInstallAppList()
	view := ansi.Strip(list.View())

	assert.Contains(t, view, "catalog entry skipped")
	assert.Contains(t, view, "Campfire")
}

func TestExpandAlias(t *testing.T) {
	ref, ok := expandAlias("campfire")
	assert.True(t, ok)
	assert.Equal(t, "ghcr.io/basecamp/once-campfire", ref)

	ref, ok = expandAlias("fizzy")
	assert.True(t, ok)
	assert.Equal(t, "ghcr.io/basecamp/fizzy", ref)

	ref, ok = expandAlias("writebook")
	assert.True(t, ok)
	assert.Equal(t, "ghcr.io/basecamp/writebook", ref)

	ref, ok = expandAlias("ghcr.io/basecamp/once-campfire:latest")
	assert.False(t, ok)
	assert.Equal(t, "ghcr.io/basecamp/once-campfire:latest", ref)
}

func TestFindCatalogEntryUsesUserOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	appDir := filepath.Join(dir, "oma", "apps")
	require.NoError(t, os.MkdirAll(appDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "campfire.yaml"), []byte(`schemaVersion: 1
id: campfire
name: Campfire Custom
description: A custom chat app
category: Communication
image: example.com/campfire:custom
`), 0o644))

	entry, ok := findCatalogEntry("campfire")
	require.True(t, ok)
	assert.Equal(t, "example.com/campfire:custom", entry.Image)
	assert.Equal(t, catalog.SourceUser, entry.Source)
}

func TestExpandAliasUserEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	appDir := filepath.Join(dir, "oma", "apps")
	require.NoError(t, os.MkdirAll(appDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "myapp.yaml"), []byte(`schemaVersion: 1
id: myapp
name: My App
description: A local app
category: Utilities
image: example.com/myapp:v1
`), 0o644))

	ref, ok := expandAlias("myapp")
	assert.True(t, ok)
	assert.Equal(t, "example.com/myapp:v1", ref)
}
