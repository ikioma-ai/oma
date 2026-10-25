package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/catalog"
)

func TestInstallEnvFormPrefillsDefaultsAndMasksSecrets(t *testing.T) {
	form := NewInstallEnvForm([]catalog.EnvVar{
		{Name: "DATABASE_URL", Required: true},
		{Name: "LOG_LEVEL", Default: "info"},
		{Name: "TOKEN", Secret: true, Default: "topsecret"},
		{Name: "APP_ENV", Value: "production"},
		{Name: "BASE_URL", From: "base_url"},
		{Name: "SECRET", Generate: "hex32"},
	})

	require.True(t, form.HasInputs())
	assert.Equal(t, 3, len(form.inputs))
	assert.Equal(t, "info", form.form.TextField(1).Value())
	assert.Equal(t, "topsecret", form.form.TextField(2).Value())
	assert.NotContains(t, form.form.TextField(2).View(), "topsecret")
}

func TestInstallEnvFormBlocksMissingRequiredValue(t *testing.T) {
	form := NewInstallEnvForm([]catalog.EnvVar{{Name: "DATABASE_URL", Required: true}})

	form, _ = form.Update(keyPressMsg("tab")) // submit
	form, _ = form.Update(keyPressMsg("enter"))

	assert.True(t, form.form.HasError())
}

func TestInstallEnvFormSubmitsValues(t *testing.T) {
	form := NewInstallEnvForm([]catalog.EnvVar{
		{Name: "DATABASE_URL", Required: true},
		{Name: "TOKEN", Secret: true},
	})

	form, _ = form.Update(keyPressMsg("d"))
	form, _ = form.Update(keyPressMsg("b"))
	form, _ = form.Update(keyPressMsg("tab"))
	form, _ = form.Update(keyPressMsg("s"))
	form, _ = form.Update(keyPressMsg("e"))
	form, _ = form.Update(keyPressMsg("c"))
	form, _ = form.Update(keyPressMsg("tab")) // submit
	form, cmd := form.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)

	msg := cmd()
	submit, ok := msg.(InstallEnvSubmitMsg)
	require.True(t, ok, "expected InstallEnvSubmitMsg, got %T", msg)
	assert.Equal(t, map[string]string{"DATABASE_URL": "db", "TOKEN": "sec"}, submit.Values)
}

func TestEnvInputsExcludeFixedGeneratedAndSystemValues(t *testing.T) {
	inputs := envInputs([]catalog.EnvVar{
		{Name: "REQUIRED", Required: true},
		{Name: "DEFAULT", Default: "x"},
		{Name: "FIXED", Value: "x"},
		{Name: "GENERATED", Generate: "hex32"},
		{Name: "SYSTEM", From: "base_url"},
	})

	require.Len(t, inputs, 2)
	assert.Equal(t, "REQUIRED", inputs[0].Name)
	assert.Equal(t, "DEFAULT", inputs[1].Name)
}
