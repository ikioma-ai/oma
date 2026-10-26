package target

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupVerifiesAnAlreadyPreparedTarget(t *testing.T) {
	installFakeSSH(t, `#!/bin/sh
script=$(cat)
case "$script" in
  *"uname -s"*) echo Linux ;;
  *"id -un"*) echo deploy ;;
  *"id -u"*) echo 1000 ;;
  *"sudo -n true"*) exit 0 ;;
  *"docker info >/dev/null"*) exit 0 ;;
  *"docker info --format"*) echo 24.0.0 ;;
  *) exit 0 ;;
esac
`)

	var messages []string
	report, err := Setup(context.Background(), Target{
		Name:         "vps",
		SSH:          "deploy@localhost",
		DockerSocket: "/var/run/docker.sock",
	}, func(message string) { messages = append(messages, message) })
	require.NoError(t, err)

	assert.Equal(t, "Linux", report.OS)
	assert.Equal(t, "24.0.0", report.Docker)
	assert.False(t, report.Installed)
	assert.False(t, report.GroupUpdated)
	assert.Contains(t, strings.Join(messages, "\n"), "Docker: ok")
}

func TestSetupWithoutSudoExplainsManualSteps(t *testing.T) {
	installFakeSSH(t, `#!/bin/sh
script=$(cat)
case "$script" in
  *"uname -s"*) echo Linux ;;
  *"id -un"*) echo deploy ;;
  *"id -u"*) echo 1000 ;;
  *"sudo -n true"*) exit 1 ;;
  *) exit 0 ;;
esac
`)

	_, err := Setup(context.Background(), Target{
		Name:         "vps",
		SSH:          "deploy@localhost",
		DockerSocket: "/var/run/docker.sock",
	}, func(string) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get.docker.com")
	assert.Contains(t, err.Error(), "usermod")
}

func TestSetupRejectsNonLinux(t *testing.T) {
	installFakeSSH(t, `#!/bin/sh
script=$(cat)
case "$script" in
  *"uname -s"*) echo Darwin ;;
  *) exit 0 ;;
esac
`)

	_, err := Setup(context.Background(), Target{
		Name:         "vps",
		SSH:          "deploy@localhost",
		DockerSocket: "/var/run/docker.sock",
	}, func(string) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Linux")
}
