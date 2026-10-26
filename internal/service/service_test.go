package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemdUnitIncludesBackupRole(t *testing.T) {
	unit := fmt.Sprintf(unitTemplate, "oma", "local", "/usr/local/bin/oma", "oma", "local", "target")
	assert.Contains(t, unit, "--backup-role target")
}

func TestLaunchdPlistIncludesBackupRole(t *testing.T) {
	plist := fmt.Sprintf(plistTemplate, "com.example.oma", "/usr/local/bin/oma", "oma", "local", "target")
	assert.Contains(t, plist, "<string>--backup-role</string>")
	assert.Contains(t, plist, "<string>target</string>")
}
