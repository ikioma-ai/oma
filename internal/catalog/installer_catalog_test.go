package catalog

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type installerCatalog struct {
	Apps []installerApp `json:"apps"`
}

type installerApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Image       string `json:"image"`
	Hostname    string `json:"hostname"`
}

func TestInstallerCatalogMatchesEmbeddedCatalog(t *testing.T) {
	data, err := os.ReadFile("../../installer/apps.json")
	require.NoError(t, err)

	var installer installerCatalog
	require.NoError(t, json.Unmarshal(data, &installer))

	store, err := Load(WithUserDir(t.TempDir()))
	require.NoError(t, err)
	require.Len(t, installer.Apps, len(store.List()))

	for _, app := range installer.Apps {
		entry, ok := store.Get(app.ID)
		require.True(t, ok, "missing embedded app %q", app.ID)
		assert.Equal(t, entry.Name, app.Name)
		assert.Equal(t, entry.Description, app.Description)
		assert.Equal(t, entry.Category, app.Category)
		assert.Equal(t, entry.Image, app.Image)
		assert.Equal(t, entry.Hostname, app.Hostname)
	}
}
