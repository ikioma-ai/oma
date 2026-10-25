package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryAuthFor(t *testing.T) {
	t.Run("invalid image string", func(t *testing.T) {
		isolateDockerConfig(t)
		assert.Equal(t, "", authForImage(":::bad"))
	})

	t.Run("no docker config present", func(t *testing.T) {
		isolateDockerConfig(t)
		assert.Equal(t, "", authForImage("ghcr.io/ikioma-ai/oma:main"))
	})

	t.Run("reads config from DOCKER_CONFIG directory", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		encoded := base64.StdEncoding.EncodeToString([]byte("myuser:mypass"))
		writeDockerConfig(t, dir, map[string]string{"ghcr.io": encoded}, nil, "")

		token := authForImage("ghcr.io/ikioma-ai/oma:main")
		require.NotEmpty(t, token)
		ac := decodeAuthToken(t, token)
		assert.Equal(t, "myuser", ac.Username)
		assert.Equal(t, "mypass", ac.Password)
	})

	t.Run("malformed config.json falls back to anonymous", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json}"), 0600))

		assert.Equal(t, "", authForImage("ghcr.io/ikioma-ai/oma:main"))
	})

	t.Run("config has credHelpers for host", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		installFakeCredHelper(t, "myhelper", credHelperScript("helper-user", "helper-pass"))
		writeDockerConfig(t, dir, nil, map[string]string{"ghcr.io": "myhelper"}, "")

		token := authForImage("ghcr.io/ikioma-ai/oma:main")
		require.NotEmpty(t, token)
		ac := decodeAuthToken(t, token)
		assert.Equal(t, "helper-user", ac.Username)
		assert.Equal(t, "helper-pass", ac.Password)
	})

	t.Run("config has credsStore only", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		installFakeCredHelper(t, "mystore", credHelperScript("store-user", "store-pass"))
		writeDockerConfig(t, dir, nil, nil, "mystore")

		token := authForImage("ghcr.io/ikioma-ai/oma:main")
		require.NotEmpty(t, token)
		ac := decodeAuthToken(t, token)
		assert.Equal(t, "store-user", ac.Username)
		assert.Equal(t, "store-pass", ac.Password)
	})

	t.Run("credHelpers wins over credsStore", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		installFakeCredHelper(t, "specific-helper", credHelperScript("helper-user", "helper-pass"))
		// Real credential stores are indexed by hostname, so they don't return
		// credentials when given a full repo path like "ghcr.io/ikioma-ai/oma".
		// Simulate that by outputting the "credentials not found" message (the
		// docker-credential-helpers protocol for "no credentials for this server").
		installFakeCredHelper(t, "global-store", `#!/bin/sh
input=$(cat)
if echo "$input" | grep -q '/'; then
  echo "credentials not found in native keychain"; exit 1
fi
echo '{"ServerURL":"","Username":"store-user","Secret":"store-pass"}'
`)
		writeDockerConfig(t, dir, nil, map[string]string{"ghcr.io": "specific-helper"}, "global-store")

		token := authForImage("ghcr.io/ikioma-ai/oma:main")
		require.NotEmpty(t, token)
		ac := decodeAuthToken(t, token)
		assert.Equal(t, "helper-user", ac.Username)
		assert.Equal(t, "helper-pass", ac.Password)
	})

	t.Run("config has inline auths entry", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		encoded := base64.StdEncoding.EncodeToString([]byte("inline-user:inline-pass"))
		writeDockerConfig(t, dir, map[string]string{"ghcr.io": encoded}, nil, "")

		token := authForImage("ghcr.io/ikioma-ai/oma:main")
		require.NotEmpty(t, token)
		ac := decodeAuthToken(t, token)
		assert.Equal(t, "inline-user", ac.Username)
		assert.Equal(t, "inline-pass", ac.Password)
	})

	t.Run("credHelpers entry but helper fails - no fallback", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		installFakeCredHelper(t, "failing-helper", "#!/bin/sh\nexit 1\n")
		writeDockerConfig(t, dir, nil, map[string]string{"ghcr.io": "failing-helper"}, "")

		assert.Equal(t, "", authForImage("ghcr.io/ikioma-ai/oma:main"))
	})

	t.Run("no matching entry for host", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		encoded := base64.StdEncoding.EncodeToString([]byte("user:pass"))
		writeDockerConfig(t, dir, map[string]string{"docker.io": encoded}, nil, "")

		assert.Equal(t, "", authForImage("ghcr.io/ikioma-ai/oma:main"))
	})
}

func TestRegistryAuthForScopedCredentials(t *testing.T) {
	settings := func(image string, registry RegistrySettings) ApplicationSettings {
		return ApplicationSettings{Image: image, Registry: registry}
	}

	t.Run("scoped credentials win over the keychain", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		encoded := base64.StdEncoding.EncodeToString([]byte("keychain-user:keychain-pass"))
		writeDockerConfig(t, dir, map[string]string{"ghcr.io": encoded}, nil, "")

		token := registryAuthFor(settings("ghcr.io/ikioma-ai/oma:main", RegistrySettings{
			Host:     "ghcr.io",
			Username: "scoped-user",
			Password: "scoped-pass",
		}))
		require.NotEmpty(t, token)
		ac := decodeAuthToken(t, token)
		assert.Equal(t, "scoped-user", ac.Username)
		assert.Equal(t, "scoped-pass", ac.Password)
	})

	t.Run("credentials apply to any image on the same host", func(t *testing.T) {
		isolateDockerConfig(t)

		token := registryAuthFor(settings("ghcr.io/other/app:v2", RegistrySettings{
			Host:     "ghcr.io",
			Username: "scoped-user",
			Password: "scoped-pass",
		}))
		require.NotEmpty(t, token)
		ac := decodeAuthToken(t, token)
		assert.Equal(t, "scoped-user", ac.Username)
		assert.Equal(t, "scoped-pass", ac.Password)
	})

	t.Run("credentials for a different host fall back to the keychain", func(t *testing.T) {
		dir := isolateDockerConfig(t)
		encoded := base64.StdEncoding.EncodeToString([]byte("keychain-user:keychain-pass"))
		writeDockerConfig(t, dir, map[string]string{"ghcr.io": encoded}, nil, "")

		token := registryAuthFor(settings("ghcr.io/ikioma-ai/oma:main", RegistrySettings{
			Host:     "registry.example.com",
			Username: "scoped-user",
			Password: "scoped-pass",
		}))
		require.NotEmpty(t, token)
		ac := decodeAuthToken(t, token)
		assert.Equal(t, "keychain-user", ac.Username)
		assert.Equal(t, "keychain-pass", ac.Password)
	})

	t.Run("credentials for a different host with no keychain entry", func(t *testing.T) {
		isolateDockerConfig(t)

		token := registryAuthFor(settings("ghcr.io/ikioma-ai/oma:main", RegistrySettings{
			Host:     "registry.example.com",
			Username: "scoped-user",
			Password: "scoped-pass",
		}))
		assert.Equal(t, "", token)
	})
}

func TestRegistryHost(t *testing.T) {
	host, err := RegistryHost("ghcr.io/ikioma-ai/oma:v1")
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io", host)

	host, err = RegistryHost("nginx")
	require.NoError(t, err)
	assert.Equal(t, "index.docker.io", host)

	host, err = RegistryHost("registry.example.com:5000/app@sha256:1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	assert.Equal(t, "registry.example.com:5000", host)

	_, err = RegistryHost(":::bad")
	assert.Error(t, err)
}

// Helpers

func authForImage(image string) string {
	return registryAuthFor(ApplicationSettings{Image: image})
}

// isolateDockerConfig sets DOCKER_CONFIG to a fresh temp dir so tests don't
// touch the real Docker config. Returns the temp dir path.
func isolateDockerConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	return dir
}

// writeDockerConfig writes a Docker config.json into dir (which should be the
// value of $DOCKER_CONFIG). auths maps registry hostnames to base64 auth strings.
func writeDockerConfig(t *testing.T, dir string, auths map[string]string, credHelpers map[string]string, credsStore string) {
	t.Helper()
	type authEntry struct {
		Auth string `json:"auth"`
	}
	cfg := struct {
		Auths       map[string]authEntry `json:"auths,omitempty"`
		CredHelpers map[string]string    `json:"credHelpers,omitempty"`
		CredsStore  string               `json:"credsStore,omitempty"`
	}{
		CredHelpers: credHelpers,
		CredsStore:  credsStore,
	}
	if len(auths) > 0 {
		cfg.Auths = make(map[string]authEntry, len(auths))
		for k, v := range auths {
			cfg.Auths[k] = authEntry{Auth: v}
		}
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), data, 0600))
}

func installFakeCredHelper(t *testing.T, helperName, script string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docker-credential-"+helperName), []byte(script), 0755))
}

func credHelperScript(username, secret string) string {
	payload, _ := json.Marshal(struct {
		ServerURL string `json:"ServerURL"`
		Username  string `json:"Username"`
		Secret    string `json:"Secret"`
	}{Username: username, Secret: secret})
	return fmt.Sprintf("#!/bin/sh\necho '%s'\n", payload)
}

type authTokenPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func decodeAuthToken(t *testing.T, token string) authTokenPayload {
	t.Helper()
	data, err := base64.URLEncoding.DecodeString(token)
	require.NoError(t, err)
	var ac authTokenPayload
	require.NoError(t, json.Unmarshal(data, &ac))
	return ac
}
