package ui

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	configDir, err := os.MkdirTemp("", "oma-ui-config")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", configDir); err != nil {
		panic(err)
	}

	code := m.Run()

	os.RemoveAll(configDir)
	os.Exit(code)
}
