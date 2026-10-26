package docker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectStorageConfigValidate(t *testing.T) {
	base := ObjectStorageConfig{
		Endpoint:  "https://s3.example.com",
		Region:    "us-east-1",
		Bucket:    "backups",
		Prefix:    "oma",
		AccessKey: "access",
		SecretKey: "secret",
	}
	require.NoError(t, base.Validate())

	missing := base
	missing.SecretKey = ""
	require.Error(t, missing.Validate())

	relative := base
	relative.Endpoint = "s3.example.com"
	require.Error(t, relative.Validate())
}

func TestObjectArchiveName(t *testing.T) {
	assert.True(t, isObjectArchiveName("chat-20261026-120000.tar.gz"))
	assert.False(t, isObjectArchiveName("../chat.tar.gz"))
	assert.False(t, isObjectArchiveName("prefix/chat.tar.gz"))
	assert.False(t, isObjectArchiveName(`chat\\..\\chat.tar.gz`))
	assert.False(t, isObjectArchiveName("chat.tar.gz\n"))
}

func TestObjectStorageObjectKey(t *testing.T) {
	config := ObjectStorageConfig{Prefix: "oma/backups"}
	key := config.ObjectKey("chat", time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
	assert.Equal(t, "oma/backups/chat-20261026-120000.tar.gz", key)

	config.Prefix = ""
	assert.Equal(t, "chat-20261026-120000.tar.gz", config.ObjectKey("chat", time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)))
}

func TestLoadObjectStorageConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "object.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"endpoint":"https://s3.example.com","bucket":"backups","accessKey":"access","secretKey":"secret"}`), 0o600))
	t.Setenv(ObjectStorageConfigEnv, path)

	config, err := LoadObjectStorageConfig()
	require.NoError(t, err)
	assert.Equal(t, "backups", config.Bucket)

	require.NoError(t, os.Chmod(path, 0o644))
	_, err = LoadObjectStorageConfig()
	require.Error(t, err)
}
