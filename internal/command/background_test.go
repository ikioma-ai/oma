package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackgroundServiceName(t *testing.T) {
	assert.Equal(t, "oma-background", backgroundServiceName("oma", "local"))
	assert.Equal(t, "oma-vps-background", backgroundServiceName("oma", "vps"))
	assert.Equal(t, "apps-vps-background", backgroundServiceName("apps", "vps"))
}

func TestBackgroundCommandsExposeBackupRole(t *testing.T) {
	install := newBackgroundInstallCommand()
	assert.NotNil(t, install.cmd.Flags().Lookup("backup-role"))

	run := newBackgroundRunCommand()
	assert.NotNil(t, run.cmd.Flags().Lookup("backup-role"))
}
