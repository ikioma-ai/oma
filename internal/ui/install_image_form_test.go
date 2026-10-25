package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallImageForm_HidesCredentialFieldsByDefault(t *testing.T) {
	form := NewInstallImageForm()
	assert.False(t, form.ShowsCredentials())
	assert.NotContains(t, form.View(), "Registry Username")
}

func TestInstallImageForm_SubmitWithAlias(t *testing.T) {
	form := NewInstallImageForm()

	imageFormTypeText(&form, "campfire")
	imageFormTabTo(&form, 1) // to submit
	form, cmd := form.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)

	msg := cmd()
	submit, ok := msg.(InstallImageSubmitMsg)
	require.True(t, ok, "expected InstallImageSubmitMsg, got %T", msg)
	assert.Equal(t, "ghcr.io/basecamp/once-campfire", submit.ImageRef)
	assert.Equal(t, "campfire", submit.CatalogID)
	assert.Empty(t, submit.RegistryUsername)
	assert.Empty(t, submit.RegistryPassword)
}

func TestInstallImageForm_SubmitWithCustomImage(t *testing.T) {
	form := NewInstallImageForm()

	imageFormTypeText(&form, "ghcr.io/basecamp/once-campfire:latest")
	imageFormTabTo(&form, 1)
	form, cmd := form.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)

	msg := cmd()
	submit, ok := msg.(InstallImageSubmitMsg)
	require.True(t, ok, "expected InstallImageSubmitMsg, got %T", msg)
	assert.Equal(t, "ghcr.io/basecamp/once-campfire:latest", submit.ImageRef)
}

func TestInstallImageForm_SubmitWithRegistryCredentials(t *testing.T) {
	form := NewInstallImageFormWithCredentials("registry.example.com/app:latest")
	assert.True(t, form.ShowsCredentials())
	assert.Equal(t, "registry.example.com/app:latest", form.ImageRef())

	imageFormTabTo(&form, 1) // to username
	imageFormTypeText(&form, "user")
	imageFormTabTo(&form, 1)
	imageFormTypeText(&form, "pass")
	imageFormTabTo(&form, 1)
	form, cmd := form.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)

	msg := cmd()
	submit, ok := msg.(InstallImageSubmitMsg)
	require.True(t, ok, "expected InstallImageSubmitMsg, got %T", msg)
	assert.Equal(t, "registry.example.com/app:latest", submit.ImageRef)
	assert.Equal(t, "user", submit.RegistryUsername)
	assert.Equal(t, "pass", submit.RegistryPassword)
}

func TestInstallImageForm_Cancel(t *testing.T) {
	form := NewInstallImageForm()

	// Tab past the field and submit button to cancel
	imageFormTabTo(&form, 2)
	form, cmd := form.Update(keyPressMsg("enter"))
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(InstallImageBackMsg)
	assert.True(t, ok, "expected InstallImageBackMsg, got %T", msg)
}

func TestInstallImageForm_RequiresImage(t *testing.T) {
	form := NewInstallImageForm()

	// Tab to submit button, then press enter with empty image field
	imageFormTabTo(&form, 1)
	form, _ = form.Update(keyPressMsg("enter"))
	assert.True(t, form.form.HasError())
}

// Helpers

func imageFormTypeText(form *InstallImageForm, text string) {
	for _, r := range text {
		*form, _ = form.Update(keyPressMsg(string(r)))
	}
}

func imageFormTabTo(form *InstallImageForm, presses int) {
	for range presses {
		*form, _ = form.Update(keyPressMsg("tab"))
	}
}
