package docker

import (
	"archive/tar"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyContainerName(t *testing.T) {
	ns := &Namespace{name: "oma"}
	proxy := NewProxy(ns)
	assert.Equal(t, "oma-proxy", proxy.containerName())

	ns2 := &Namespace{name: "staging"}
	proxy2 := NewProxy(ns2)
	assert.Equal(t, "staging-proxy", proxy2.containerName())
}

func TestDeployArgs(t *testing.T) {
	proxy := &Proxy{}

	t.Run("basic deploy includes timeout", func(t *testing.T) {
		args := proxy.deployArgs(DeployOptions{AppName: "chat", Target: "localhost:3000"})

		assert.Equal(t, []string{
			"kamal-proxy", "deploy", "chat",
			"--target", "localhost:3000",
			"--deploy-timeout", "120s",
		}, args)
	})

	t.Run("with host", func(t *testing.T) {
		args := proxy.deployArgs(DeployOptions{AppName: "chat", Target: "localhost:3000", Host: "chat.example.com"})

		assert.Contains(t, args, "--host")
		assert.Contains(t, args, "chat.example.com")
	})

	t.Run("with TLS", func(t *testing.T) {
		args := proxy.deployArgs(DeployOptions{AppName: "chat", Target: "localhost:3000", TLS: true})

		assert.Contains(t, args, "--tls")
	})

	t.Run("with custom TLS certificates", func(t *testing.T) {
		args := proxy.deployArgs(DeployOptions{
			AppName:            "chat",
			Target:             "localhost:3000",
			Host:               "chat.local",
			TLS:                true,
			TLSCertificatePath: "/certs/chat.pem",
			TLSPrivateKeyPath:  "/certs/chat.key",
		})

		assert.Contains(t, args, "--tls")
		assert.Contains(t, args, "--tls-certificate-path")
		assert.Contains(t, args, "/certs/chat.pem")
		assert.Contains(t, args, "--tls-private-key-path")
		assert.Contains(t, args, "/certs/chat.key")
	})

	t.Run("with health check settings", func(t *testing.T) {
		args := proxy.deployArgs(DeployOptions{
			AppName:            "chat",
			Target:             "localhost:3000",
			Host:               "chat.example.com",
			TLS:                true,
			HealthCheckPath:    "/healthz",
			HealthCheckPort:    3001,
			HealthCheckTimeout: "10s",
		})

		assert.Equal(t, []string{
			"kamal-proxy", "deploy", "chat",
			"--target", "localhost:3000",
			"--deploy-timeout", "120s",
			"--health-check-path", "/healthz",
			"--health-check-port", "3001",
			"--health-check-timeout", "10s",
			"--host", "chat.example.com",
			"--tls",
		}, args)
	})

	t.Run("with host and TLS", func(t *testing.T) {
		args := proxy.deployArgs(DeployOptions{
			AppName: "chat",
			Target:  "localhost:3000",
			Host:    "chat.example.com",
			TLS:     true,
		})

		assert.Equal(t, []string{
			"kamal-proxy", "deploy", "chat",
			"--target", "localhost:3000",
			"--deploy-timeout", "120s",
			"--host", "chat.example.com",
			"--tls",
		}, args)
	})
}

func TestTLSArchive(t *testing.T) {
	archive, err := tlsArchive("app.pem", "app.key", []byte("certificate"), []byte("private key"))
	require.NoError(t, err)

	reader := tar.NewReader(archive)
	entries := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		entries[header.Name] = string(data)
	}

	assert.Equal(t, "certificate", entries["app.pem"])
	assert.Equal(t, "private key", entries["app.key"])
}
