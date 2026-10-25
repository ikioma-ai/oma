package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/ikioma-ai/oma/internal/docker"
)

const (
	registryUsernameField = iota
	registryPasswordField
)

type SettingsFormRegistry struct {
	settingsFormBase
}

func NewSettingsFormRegistry(settings docker.ApplicationSettings) SettingsFormRegistry {
	host, _ := docker.RegistryHost(settings.Image)

	usernameField := NewTextField("(optional)")
	passwordField := NewTextField("(optional)")
	passwordField.SetEchoPassword()

	if settings.Registry.Host == host {
		usernameField.SetValue(settings.Registry.Username)
		passwordField.SetValue(settings.Registry.Password)
	}

	m := SettingsFormRegistry{
		title: "Registry",
		form: NewForm("Done",
			FormItem{Label: "Registry Username", Field: usernameField},
			FormItem{Label: "Registry Password", Field: passwordField},
		),
		statusLine: func() string {
			if host == "" {
				return ""
			}
			return "Credentials for " + host
		},
	}

	m.form.OnSubmit(func(f *Form) tea.Cmd {
		s := settings
		username := f.TextField(registryUsernameField).Value()
		password := f.TextField(registryPasswordField).Value()
		if username == "" && password == "" {
			s.Registry = docker.RegistrySettings{}
		} else {
			s.Registry = docker.RegistrySettings{Host: host, Username: username, Password: password}
		}
		return func() tea.Msg { return SettingsSectionSubmitMsg{Settings: s} }
	})
	m.form.OnCancel(func(f *Form) tea.Cmd {
		return func() tea.Msg { return SettingsSectionCancelMsg{} }
	})

	return m
}

func (m SettingsFormRegistry) Update(msg tea.Msg) (SettingsSection, tea.Cmd) {
	var cmd tea.Cmd
	m.settingsFormBase, cmd = m.update(msg)
	return m, cmd
}
