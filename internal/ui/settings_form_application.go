package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/ikioma-ai/oma/internal/docker"
)

const (
	appImageField = iota
	appHostnameField
	appTLSField
	appTLSCustomField
	appTLSCertificateField
	appTLSPrivateKeyField
)

type SettingsFormApplication struct {
	settingsFormBase
}

func NewSettingsFormApplication(settings docker.ApplicationSettings) SettingsFormApplication {
	imageField := NewTextField("user/repo:tag")
	imageField.SetValue(settings.Image)

	hostnameField := NewTextField("app.example.com")
	hostnameField.SetValue(settings.Host)

	tlsField := NewCheckboxField("Enabled", settings.TLSMode != docker.TLSModeOff && !settings.DisableTLS)
	customTLSField := NewCheckboxField("Use custom certificate", settings.TLSMode == docker.TLSModeCustom)
	tlsField.SetDisabledWhen(func() (bool, string) {
		if !customTLSField.Checked() && docker.IsLocalhost(hostnameField.Value()) {
			return true, "Not available for localhost"
		}
		return false, ""
	})

	certificateField := NewTextField("/path/to/certificate.pem")
	certificateField.SetValue(settings.TLSCertificatePath)
	privateKeyField := NewTextField("/path/to/private-key.pem")
	privateKeyField.SetValue(settings.TLSPrivateKeyPath)

	m := SettingsFormApplication{
		title: "Application",
		form: NewForm("Done",
			FormItem{Label: "Image", Field: imageField, Required: true},
			FormItem{Label: "Hostname", Field: hostnameField, Required: true},
			FormItem{Label: "TLS", Field: tlsField},
			FormItem{Label: "Custom TLS", Field: customTLSField},
			FormItem{Label: "TLS certificate path", Field: certificateField},
			FormItem{Label: "TLS private key path", Field: privateKeyField},
		),
	}

	m.form.OnSubmit(func(f *Form) tea.Cmd {
		s := settings
		s.Image = f.TextField(appImageField).Value()
		s.Host = f.TextField(appHostnameField).Value()
		enabled := f.CheckboxField(appTLSField).Checked()
		custom := f.CheckboxField(appTLSCustomField).Checked()
		s.DisableTLS = !enabled
		switch {
		case !enabled:
			s.TLSMode = docker.TLSModeOff
		case custom:
			s.TLSMode = docker.TLSModeCustom
		default:
			s.TLSMode = docker.TLSModeAuto
		}
		s.TLSCertificatePath = f.TextField(appTLSCertificateField).Value()
		s.TLSPrivateKeyPath = f.TextField(appTLSPrivateKeyField).Value()
		return func() tea.Msg { return SettingsSectionSubmitMsg{Settings: s} }
	})
	m.form.OnCancel(func(f *Form) tea.Cmd {
		return func() tea.Msg { return SettingsSectionCancelMsg{} }
	})

	return m
}

func (m SettingsFormApplication) Update(msg tea.Msg) (SettingsSection, tea.Cmd) {
	var cmd tea.Cmd
	m.settingsFormBase, cmd = m.update(msg)
	return m, cmd
}
