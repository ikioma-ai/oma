package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBuiltinCatalog(t *testing.T) {
	store, err := Load(WithUserDir(t.TempDir()))
	require.NoError(t, err)
	require.Empty(t, store.Issues())

	entries := store.List()
	require.Len(t, entries, 3)

	assert.Equal(t, []string{"campfire", "fizzy", "writebook"}, entryIDs(entries))

	for _, entry := range entries {
		assert.Equal(t, SourceBuiltin, entry.Source)
		assert.Equal(t, KindWeb, entry.Kind)
	}
}

func TestLoadUserCatalogAddsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "myapp.yaml", `schemaVersion: 1
id: myapp
name: My App
description: A local app
category: Utilities
image: example.com/myapp:v1
`)
	writeManifest(t, dir, "campfire.yaml", `schemaVersion: 1
id: campfire
name: Campfire Override
description: A custom chat app
category: Communication
image: example.com/campfire:custom
`)

	store, err := Load(WithUserDir(dir))
	require.NoError(t, err)
	require.Empty(t, store.Issues())

	entry, ok := store.Get("myapp")
	require.True(t, ok)
	assert.Equal(t, SourceUser, entry.Source)
	assert.Equal(t, "example.com/myapp:v1", entry.Image)

	entry, ok = store.Get("campfire")
	require.True(t, ok)
	assert.Equal(t, SourceUser, entry.Source)
	assert.Equal(t, "example.com/campfire:custom", entry.Image)
	assert.Equal(t, "Campfire Override", entry.Name)

	assert.Len(t, store.List(), 4)
}

func TestLoadSkipsInvalidUserManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "bad.yaml", `schemaVersion: 1
id: badapp
kind: worker
`)

	store, err := Load(WithUserDir(dir))
	require.NoError(t, err)
	require.Len(t, store.Issues(), 1)
	assert.Contains(t, store.Issues()[0].Error(), "bad.yaml")

	_, ok := store.Get("badapp")
	assert.False(t, ok)
	assert.Len(t, store.List(), 3)
}

func TestLoadRejectsDuplicateUserIDs(t *testing.T) {
	dir := t.TempDir()
	manifest := `schemaVersion: 1
id: duplicate
name: Duplicate
description: One of two
category: Utilities
image: example.com/duplicate:v1
`
	writeManifest(t, dir, "one.yaml", manifest)
	writeManifest(t, dir, "two.yaml", manifest)

	store, err := Load(WithUserDir(dir))
	require.NoError(t, err)
	require.Len(t, store.Issues(), 2)

	_, ok := store.Get("duplicate")
	assert.False(t, ok)
	assert.Len(t, store.List(), 3)
}

func TestLoadDuplicateUserEntriesDoNotRemoveBuiltin(t *testing.T) {
	dir := t.TempDir()
	manifest := `schemaVersion: 1
id: campfire
name: Campfire Override
description: One of two
category: Communication
image: example.com/campfire:custom
`
	writeManifest(t, dir, "one.yaml", manifest)
	writeManifest(t, dir, "two.yaml", manifest)

	store, err := Load(WithUserDir(dir))
	require.NoError(t, err)
	require.Len(t, store.Issues(), 2)

	entry, ok := store.Get("campfire")
	require.True(t, ok)
	assert.Equal(t, SourceBuiltin, entry.Source)
}

func TestLoadDisabledBuiltinIsHidden(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "fizzy.yaml", `schemaVersion: 1
id: fizzy
disabled: true
`)

	store, err := Load(WithUserDir(dir))
	require.NoError(t, err)
	require.Empty(t, store.Issues())

	_, ok := store.Get("fizzy")
	assert.False(t, ok)
	assert.Equal(t, []string{"campfire", "writebook"}, entryIDs(store.List()))
}

func TestLoadMissingUserDirIsEmpty(t *testing.T) {
	store, err := Load(WithUserDir(filepath.Join(t.TempDir(), "missing")))
	require.NoError(t, err)
	assert.Empty(t, store.Issues())
	assert.Len(t, store.List(), 3)
}

func TestUserDirUsesXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	got, err := UserDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "oma", "apps"), got)
}

func TestEmptyStore(t *testing.T) {
	store := Empty()
	assert.Empty(t, store.List())
	assert.Empty(t, store.Issues())
	_, ok := store.Get("campfire")
	assert.False(t, ok)
}

// Helpers

func writeManifest(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func entryIDs(entries []Entry) []string {
	ids := make([]string, len(entries))
	for i, entry := range entries {
		ids[i] = entry.ID
	}
	return ids
}
