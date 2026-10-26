package command

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
)

func TestApplyChanges(t *testing.T) {
	existing := docker.ApplicationSettings{
		Name:       "myapp",
		Image:      "myimage:latest",
		Host:       "app.example.com",
		DisableTLS: false,
		EnvVars:    map[string]string{"KEY": "value"},
		SMTP: docker.SMTPSettings{
			Server:   "smtp.example.com",
			Port:     "587",
			Username: "user",
			Password: "pass",
			From:     "noreply@example.com",
		},
		Resources: docker.ContainerResources{
			CPUs:     2,
			MemoryMB: 1024,
		},
		AutoUpdate: true,
		Backup: docker.BackupSettings{
			Path:       "/backups",
			AutoBackup: true,
		},
	}

	newCmd := func() (*cobra.Command, *settingsFlags) {
		cmd := &cobra.Command{}
		f := &settingsFlags{}
		f.register(cmd)
		return cmd, f
	}

	t.Run("no flags changed returns existing", func(t *testing.T) {
		cmd, f := newCmd()
		result, err := f.applyChanges(cmd, existing, existing.Image)
		require.NoError(t, err)
		assert.True(t, existing.Equal(result))
	})

	t.Run("single flag changed", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("memory", "2048"))

		result, err := f.applyChanges(cmd, existing, existing.Image)
		require.NoError(t, err)
		assert.Equal(t, 2048, result.Resources.MemoryMB)
		assert.Equal(t, existing.Resources.CPUs, result.Resources.CPUs)
		assert.Equal(t, existing.Host, result.Host)
		assert.Equal(t, existing.EnvVars, result.EnvVars)
	})

	t.Run("multiple flags changed", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("host", "new.example.com"))
		require.NoError(t, cmd.Flags().Set("cpus", "4"))
		require.NoError(t, cmd.Flags().Set("auto-backup", "false"))

		result, err := f.applyChanges(cmd, existing, existing.Image)
		require.NoError(t, err)
		assert.Equal(t, "new.example.com", result.Host)
		assert.Equal(t, 4, result.Resources.CPUs)
		assert.Equal(t, false, result.Backup.AutoBackup)
		// Unchanged fields preserved
		assert.Equal(t, existing.SMTP, result.SMTP)
		assert.Equal(t, existing.EnvVars, result.EnvVars)
		assert.Equal(t, existing.Resources.MemoryMB, result.Resources.MemoryMB)
	})

	t.Run("env replaces all vars", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("env", "NEW=val"))

		result, err := f.applyChanges(cmd, existing, existing.Image)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"NEW": "val"}, result.EnvVars)
	})

	t.Run("invalid env returns error", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("env", "INVALID"))

		_, err := f.applyChanges(cmd, existing, existing.Image)
		assert.ErrorContains(t, err, "must be in KEY=VALUE format")
	})

	t.Run("empty image", func(t *testing.T) {
		cmd, f := newCmd()

		_, err := f.applyChanges(cmd, existing, "")
		assert.ErrorIs(t, err, docker.ErrImageRequired)
	})

	t.Run("enable auto-backup with existing path", func(t *testing.T) {
		noAutoBackup := existing
		noAutoBackup.Backup.AutoBackup = false

		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("auto-backup", "true"))

		result, err := f.applyChanges(cmd, noAutoBackup, noAutoBackup.Image)
		require.NoError(t, err)
		assert.True(t, result.Backup.AutoBackup)
	})

	t.Run("enable auto-backup without path", func(t *testing.T) {
		noPath := existing
		noPath.Backup.Path = ""
		noPath.Backup.AutoBackup = false

		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("auto-backup", "true"))

		_, err := f.applyChanges(cmd, noPath, noPath.Image)
		assert.ErrorIs(t, err, docker.ErrAutoBackupWithoutPath)
	})

	t.Run("set custom TLS", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("tls-mode", docker.TLSModeCustom))
		require.NoError(t, cmd.Flags().Set("tls-certificate-path", "/certs/app.pem"))
		require.NoError(t, cmd.Flags().Set("tls-private-key-path", "/certs/app.key"))

		result, err := f.applyChanges(cmd, existing, existing.Image)
		require.NoError(t, err)
		assert.Equal(t, docker.TLSModeCustom, result.TLSMode)
		assert.Equal(t, "/certs/app.pem", result.TLSCertificatePath)
		assert.Equal(t, "/certs/app.key", result.TLSPrivateKeyPath)
	})

	t.Run("custom TLS without paths", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("tls-mode", docker.TLSModeCustom))

		_, err := f.applyChanges(cmd, existing, existing.Image)
		assert.ErrorIs(t, err, docker.ErrTLSCertificateRequired)
	})

	t.Run("set backup destination", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("backup-destination", docker.BackupDestinationTarget))

		result, err := f.applyChanges(cmd, existing, existing.Image)
		require.NoError(t, err)
		assert.Equal(t, docker.BackupDestinationTarget, result.Backup.Destination)
	})

	t.Run("invalid backup destination", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("backup-destination", "elsewhere"))

		_, err := f.applyChanges(cmd, existing, existing.Image)
		assert.ErrorIs(t, err, docker.ErrBackupDestinationInvalid)
	})

	t.Run("clear backup path with auto-backup enabled", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("backup-path", ""))

		_, err := f.applyChanges(cmd, existing, existing.Image)
		assert.ErrorIs(t, err, docker.ErrAutoBackupWithoutPath)
	})

	t.Run("registry credentials scope to the effective image's host", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("registry-username", "user"))
		require.NoError(t, cmd.Flags().Set("registry-password", "pass"))

		result, err := f.applyChanges(cmd, existing, "ghcr.io/otherimage:latest")
		require.NoError(t, err)
		assert.Equal(t, docker.RegistrySettings{Host: "ghcr.io", Username: "user", Password: "pass"}, result.Registry)
	})

	t.Run("registry password from stdin", func(t *testing.T) {
		withCreds := existing
		withCreds.Registry = docker.RegistrySettings{Host: "index.docker.io", Username: "user", Password: "old"}

		cmd, f := newCmd()
		cmd.SetIn(strings.NewReader("newpass\n"))
		require.NoError(t, cmd.Flags().Set("registry-username", "user"))
		require.NoError(t, cmd.Flags().Set("registry-password-stdin", "true"))

		result, err := f.applyChanges(cmd, withCreds, withCreds.Image)
		require.NoError(t, err)
		assert.Equal(t, "newpass", result.Registry.Password)
		assert.Equal(t, "user", result.Registry.Username)
	})

	t.Run("username alone is rejected even with stored credentials", func(t *testing.T) {
		// Otherwise a username change plus a new image host would re-scope
		// the stored password to a different registry.
		withCreds := existing
		withCreds.Registry = docker.RegistrySettings{Host: "index.docker.io", Username: "user", Password: "pass"}

		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("registry-username", "newuser"))

		_, err := f.applyChanges(cmd, withCreds, "ghcr.io/otherimage:latest")
		assert.ErrorIs(t, err, docker.ErrRegistryUsernameWithoutPassword)
	})

	t.Run("image change alone keeps existing registry scope", func(t *testing.T) {
		withCreds := existing
		withCreds.Registry = docker.RegistrySettings{Host: "index.docker.io", Username: "user", Password: "pass"}

		cmd, f := newCmd()
		result, err := f.applyChanges(cmd, withCreds, "otherimage:latest")
		require.NoError(t, err)
		assert.Equal(t, withCreds.Registry, result.Registry)
		assert.Equal(t, "otherimage:latest", result.Image)
	})

	t.Run("empty credentials clear the registry settings", func(t *testing.T) {
		withCreds := existing
		withCreds.Registry = docker.RegistrySettings{Host: "index.docker.io", Username: "user", Password: "pass"}

		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("registry-username", ""))
		require.NoError(t, cmd.Flags().Set("registry-password", ""))

		result, err := f.applyChanges(cmd, withCreds, withCreds.Image)
		require.NoError(t, err)
		assert.True(t, result.Registry.Empty())
	})

	t.Run("partial registry credentials return an error", func(t *testing.T) {
		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("registry-password", "pass"))

		_, err := f.applyChanges(cmd, existing, existing.Image)
		assert.ErrorIs(t, err, docker.ErrRegistryPasswordWithoutUsername)
	})

	t.Run("set both auto-backup and path", func(t *testing.T) {
		noBackup := existing
		noBackup.Backup = docker.BackupSettings{}

		cmd, f := newCmd()
		require.NoError(t, cmd.Flags().Set("auto-backup", "true"))
		require.NoError(t, cmd.Flags().Set("backup-path", "/backups"))

		result, err := f.applyChanges(cmd, noBackup, noBackup.Image)
		require.NoError(t, err)
		assert.True(t, result.Backup.AutoBackup)
		assert.Equal(t, "/backups", result.Backup.Path)
	})
}
