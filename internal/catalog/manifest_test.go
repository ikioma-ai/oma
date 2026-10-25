package catalog

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAppliesDefaults(t *testing.T) {
	manifest, err := Parse([]byte(validManifest()))
	require.NoError(t, err)

	assert.Equal(t, KindWeb, manifest.Kind)
	assert.Equal(t, DefaultPort, manifest.Web.Port)
	assert.Equal(t, DefaultHealthPath, manifest.Web.HealthPath)
	assert.Equal(t, DefaultPort, manifest.Web.HealthPort)
	assert.Equal(t, DefaultHealthTimeout, manifest.Web.HealthTimeout)
	assert.Equal(t, DefaultTLSMode, manifest.Web.TLS)
	assert.True(t, manifest.VerifyEnabled())
	assert.Equal(t, 5*time.Second, manifest.HealthTimeoutDuration())
}

func TestParseVerifyFalse(t *testing.T) {
	data := strings.Replace(validManifest(), "  verify: true", "  verify: false", 1)
	manifest, err := Parse([]byte(data))
	require.NoError(t, err)
	assert.False(t, manifest.VerifyEnabled())
}

func TestParseDisabledEntryRequiresOnlyIdentity(t *testing.T) {
	manifest, err := Parse([]byte(`
schemaVersion: 1
id: fizzy
disabled: true
`))
	require.NoError(t, err)
	assert.True(t, manifest.Disabled)
	assert.Equal(t, KindWeb, manifest.Kind)
}

func TestParseRejectsInvalidManifests(t *testing.T) {
	valid := validManifest()

	tests := []struct {
		name string
		data string
	}{
		{name: "missing schema version", data: strings.Replace(valid, "schemaVersion: 1\n", "", 1)},
		{name: "newer schema version", data: strings.Replace(valid, "schemaVersion: 1", "schemaVersion: 2", 1)},
		{name: "unsupported kind", data: strings.Replace(valid, "kind: web", "kind: worker", 1)},
		{name: "invalid id", data: strings.Replace(valid, "id: myapp", "id: My_App", 1)},
		{name: "missing name", data: strings.Replace(valid, "name: My App\n", "", 1)},
		{name: "missing description", data: strings.Replace(valid, "description: Does things\n", "", 1)},
		{name: "missing category", data: strings.Replace(valid, "category: Tools\n", "", 1)},
		{name: "invalid image", data: strings.Replace(valid, "image: example.com/myapp", "image: not an image", 1)},
		{name: "unknown field", data: valid + "unknown: true\n"},
		{name: "port out of range", data: strings.Replace(valid, "  port: 80", "  port: 70000", 1)},
		{name: "relative health path", data: strings.Replace(valid, "  healthPath: /up", "  healthPath: up", 1)},
		{name: "invalid health timeout", data: strings.Replace(valid, "  healthTimeout: 5s", "  healthTimeout: soon", 1)},
		{name: "invalid tls mode", data: strings.Replace(valid, "  tls: auto", "  tls: sometimes", 1)},
		{name: "negative resources", data: valid + "resources:\n  cpus: -1\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.data))
			assert.Error(t, err)
		})
	}
}

func TestParseRejectsInvalidStorage(t *testing.T) {
	tests := []struct {
		name    string
		storage string
	}{
		{
			name: "multiple volume names",
			storage: `  - name: data
    path: /data
  - name: other
    path: /other
`,
		},
		{
			name: "relative path",
			storage: `  - name: data
    path: data
`,
		},
		{
			name: "duplicate path",
			storage: `  - name: data
    path: /data
  - name: data
    path: /data
`,
		},
		{
			name: "multiple primaries",
			storage: `  - name: data
    path: /data
    primary: true
  - name: data
    path: /other
    primary: true
`,
		},
		{
			name: "backup on secondary",
			storage: `  - name: data
    path: /data
    primary: true
  - name: data
    path: /other
    backup: true
`,
		},
		{
			name: "root path",
			storage: `  - name: data
    path: /
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(validManifest() + "storage:\n" + test.storage))
			assert.Error(t, err)
		})
	}
}

func TestParseRejectsInvalidEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{
			name: "invalid name",
			env: `  - name: "bad-name"
    value: x
`,
		},
		{
			name: "duplicate name",
			env: `  - name: TOKEN
    value: one
  - name: TOKEN
    value: two
`,
		},
		{
			name: "multiple sources",
			env: `  - name: TOKEN
    value: one
    generate: hex32
`,
		},
		{
			name: "unsupported generator",
			env: `  - name: TOKEN
    generate: base64
`,
		},
		{
			name: "unsupported source",
			env: `  - name: BASE_URL
    from: somewhere
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(validManifest() + "env:\n" + test.env))
			assert.Error(t, err)
		})
	}
}

// Helpers

func validManifest() string {
	return `schemaVersion: 1
kind: web
id: myapp
name: My App
description: Does things
category: Tools
image: example.com/myapp
web:
  port: 80
  healthPath: /up
  healthTimeout: 5s
  tls: auto
  verify: true
`
}
