package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMissingConfigUsesLocal(t *testing.T) {
	config, err := LoadFrom(filepath.Join(t.TempDir(), "config.yaml"))
	require.NoError(t, err)

	assert.Equal(t, LocalName, config.Default)
	entry, ok := config.Targets[LocalName]
	require.True(t, ok)
	assert.True(t, entry.Local)
	assert.Equal(t, DefaultNamespace, entry.Namespace)

	resolved, err := config.Resolve("", "")
	require.NoError(t, err)
	assert.Equal(t, LocalName, resolved.Name)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	config, err := LoadFrom(path)
	require.NoError(t, err)

	require.NoError(t, config.Add(Target{
		Name:         "vps",
		SSH:          "deploy@example.com",
		DockerSocket: "/run/docker.sock",
		Namespace:    "apps",
	}))
	require.NoError(t, config.SetDefault("vps"))
	require.NoError(t, config.Save())

	loaded, err := LoadFrom(path)
	require.NoError(t, err)
	assert.Equal(t, "vps", loaded.Default)

	entry, ok := loaded.Targets["vps"]
	require.True(t, ok)
	assert.Equal(t, "deploy@example.com", entry.SSH)
	assert.Equal(t, "/run/docker.sock", entry.DockerSocket)
	assert.Equal(t, "apps", entry.Namespace)
	assert.False(t, entry.Local)
}

func TestResolvePrecedence(t *testing.T) {
	config := defaultConfig("")
	config.Targets["vps"] = Target{Name: "vps", SSH: "root@example.com", Namespace: "oma"}

	resolved, err := config.Resolve("vps", LocalName)
	require.NoError(t, err)
	assert.Equal(t, "vps", resolved.Name)

	resolved, err = config.Resolve("", "vps")
	require.NoError(t, err)
	assert.Equal(t, "vps", resolved.Name)

	resolved, err = config.Resolve("", "")
	require.NoError(t, err)
	assert.Equal(t, LocalName, resolved.Name)
}

func TestUnknownTargetListsAvailable(t *testing.T) {
	config := defaultConfig("")
	config.Targets["vps"] = Target{Name: "vps", SSH: "root@example.com"}

	_, err := config.Resolve("missing", "")
	require.ErrorIs(t, err, ErrUnknownTarget)
	assert.Contains(t, err.Error(), "local")
	assert.Contains(t, err.Error(), "vps")
}

func TestAddRejectsLocalAndDuplicates(t *testing.T) {
	config := defaultConfig("")

	require.ErrorIs(t, config.Add(Target{Name: LocalName, SSH: "root@example.com"}), ErrLocalTarget)
	require.Error(t, config.Add(Target{Name: "vps"}))

	require.NoError(t, config.Add(Target{Name: "vps", SSH: "root@example.com"}))
	require.ErrorIs(t, config.Add(Target{Name: "vps", SSH: "root@example.com"}), ErrTargetExists)
}

func TestAddRejectsInvalidNamespace(t *testing.T) {
	config := defaultConfig("")

	err := config.Add(Target{Name: "vps", SSH: "root@example.com", Namespace: "Bad Namespace"})
	require.Error(t, err)
}

func TestAddRejectsInvalidSSHDestination(t *testing.T) {
	config := defaultConfig("")

	for _, destination := range []string{"", "-oProxyCommand=bad", "root@example.com extra"} {
		assert.Error(t, config.Add(Target{Name: "vps", SSH: destination}), destination)
	}
}

func TestAddRejectsRelativeDockerSocket(t *testing.T) {
	config := defaultConfig("")

	err := config.Add(Target{Name: "vps", SSH: "root@example.com", DockerSocket: "docker.sock"})
	require.Error(t, err)
}

func TestRemoveDefaultRequiresNewDefault(t *testing.T) {
	config := defaultConfig("")
	require.NoError(t, config.Add(Target{Name: "vps", SSH: "root@example.com"}))
	require.NoError(t, config.SetDefault("vps"))

	require.Error(t, config.Remove("vps", ""))
	require.Error(t, config.Remove("vps", "missing"))

	require.NoError(t, config.Remove("vps", LocalName))
	assert.Equal(t, LocalName, config.Default)
	_, ok := config.Targets["vps"]
	assert.False(t, ok)
}

func TestRemoveLocalRejected(t *testing.T) {
	config := defaultConfig("")
	require.ErrorIs(t, config.Remove(LocalName, ""), ErrLocalTarget)
}

func TestSetDefaultUnknown(t *testing.T) {
	config := defaultConfig("")
	require.ErrorIs(t, config.SetDefault("missing"), ErrUnknownTarget)
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "default missing", content: "default: missing\n"},
		{name: "missing ssh", content: "targets:\n  vps:\n    namespace: oma\n"},
		{name: "invalid ssh", content: "targets:\n  vps:\n    ssh: 'root@example.com extra'\n"},
		{name: "relative socket", content: "targets:\n  vps:\n    ssh: root@example.com\n    dockerSocket: docker.sock\n"},
		{name: "invalid target name", content: "targets:\n  Bad_Name:\n    ssh: root@example.com\n"},
		{name: "local with ssh", content: "targets:\n  local:\n    ssh: root@example.com\n"},
		{name: "invalid namespace", content: "targets:\n  vps:\n    ssh: root@example.com\n    namespace: Bad Namespace\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(test.content), 0o644))
			_, err := LoadFrom(path)
			assert.Error(t, err)
		})
	}
}

func TestSortedTargetsPutsLocalFirst(t *testing.T) {
	config := defaultConfig("")
	config.Targets["zeta"] = Target{Name: "zeta", SSH: "root@example.com"}
	config.Targets["alpha"] = Target{Name: "alpha", SSH: "root@example.com"}

	names := make([]string, 0, 3)
	for _, entry := range config.SortedTargets() {
		names = append(names, entry.Name)
	}
	assert.Equal(t, []string{LocalName, "alpha", "zeta"}, names)
}

func TestConfigPathUsesXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := ConfigPath()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "oma", "config.yaml"), path)
}

func TestSSHTransportSelected(t *testing.T) {
	transport, err := (Target{
		Name:         "vps",
		SSH:          "root@example.com",
		DockerSocket: "/var/run/docker.sock",
	}).Transport()
	require.NoError(t, err)
	assert.IsType(t, SSHTransport{}, transport)
}
