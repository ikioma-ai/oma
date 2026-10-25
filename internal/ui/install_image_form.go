package ui

import (
	tea "charm.land/bubbletea/v2"
)

const installImageField = 0

type InstallImageSubmitMsg struct {
	ImageRef         string
	CatalogID        string
	RegistryUsername string
	RegistryPassword string
}
type InstallImageBackMsg struct{}

type InstallImageForm struct {
	form            Form
	showCredentials bool
}

func NewInstallImageForm() InstallImageForm {
	return newInstallImageForm(false, "")
}

// NewInstallImageFormWithCredentials builds the form with the registry
// username/password fields revealed, prefilled with the given image ref.
func NewInstallImageFormWithCredentials(imageRef string) InstallImageForm {
	return newInstallImageForm(true, imageRef)
}

func (m InstallImageForm) ShowsCredentials() bool {
	return m.showCredentials
}

func (m InstallImageForm) ImageRef() string {
	return m.form.TextField(installImageField).Value()
}

func (m InstallImageForm) Init() tea.Cmd {
	return m.form.Init()
}

func (m InstallImageForm) Update(msg tea.Msg) (InstallImageForm, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

func (m InstallImageForm) View() string {
	return m.form.View()
}

// Helpers

func newInstallImageForm(showCredentials bool, imageRef string) InstallImageForm {
	imageField := NewTextField("user/repo:tag")
	imageField.SetValue(imageRef)

	items := []FormItem{{Label: "Image", Field: imageField, Required: true}}

	var usernameField, passwordField *TextField
	if showCredentials {
		usernameField = NewTextField("(optional)")
		passwordField = NewTextField("(optional)")
		passwordField.SetEchoPassword()
		items = append(items,
			FormItem{Label: "Registry Username", Field: usernameField},
			FormItem{Label: "Registry Password", Field: passwordField},
		)
	}

	m := InstallImageForm{
		form:            NewForm("Next", items...),
		showCredentials: showCredentials,
	}

	m.form.OnSubmit(func(f *Form) tea.Cmd {
		ref := imageField.Value()
		catalogID := ""
		if entry, ok := findCatalogEntry(ref); ok {
			ref = entry.Image
			catalogID = entry.ID
		}
		msg := InstallImageSubmitMsg{ImageRef: ref, CatalogID: catalogID}
		if showCredentials {
			msg.RegistryUsername = usernameField.Value()
			msg.RegistryPassword = passwordField.Value()
		}
		return func() tea.Msg { return msg }
	})
	m.form.OnCancel(func(f *Form) tea.Cmd {
		return func() tea.Msg { return InstallImageBackMsg{} }
	})

	return m
}
