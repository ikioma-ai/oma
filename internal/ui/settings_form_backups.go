package ui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/ikioma-ai/oma/internal/docker"
)

const (
	backupsPathField = iota
	backupsAutoBackupField
	backupsDestinationField
)

type SettingsFormBackups struct {
	settingsFormBase
}

func NewSettingsFormBackups(app *docker.Application, lastResult *docker.OperationResult) SettingsFormBackups {
	pathField := NewTextField("/path/to/backups")
	pathField.SetValue(app.Settings.Backup.Path)

	autoBackupField := NewCheckboxField("Automatically create backups", app.Settings.Backup.AutoBackup)
	destinationField := NewCheckboxField("Store backups on the target", app.Settings.Backup.DestinationOrDefault() == docker.BackupDestinationTarget)

	m := SettingsFormBackups{
		title: "Backups",
		form: NewForm("Done",
			FormItem{Label: "Backup location", Field: pathField},
			FormItem{Label: "Backups", Field: autoBackupField},
			FormItem{Label: "Destination", Field: destinationField},
		),
		statusLine: func() string {
			line := "Backups stored on the controller"
			if destinationField.Checked() {
				line = "Backups stored on the target; a target runner is required"
			}
			if status := formatOperationStatus("backup", lastResult); status != "" {
				return status + " · " + line
			}
			return line
		},
	}

	m.form.SetActionButton("Run backup now", func() tea.Msg {
		return settingsRunActionMsg{action: func() (string, error) {
			if destinationField.Checked() {
				return "", errors.New("target-side backups run on the target runner")
			}
			return "Backup complete", runBackup(app, pathField.Value())
		}}
	})
	m.form.OnSubmit(func(f *Form) tea.Cmd {
		s := app.Settings
		s.Backup.Path = f.TextField(backupsPathField).Value()
		s.Backup.AutoBackup = f.CheckboxField(backupsAutoBackupField).Checked()
		if f.CheckboxField(backupsDestinationField).Checked() {
			s.Backup.Destination = docker.BackupDestinationTarget
		} else {
			s.Backup.Destination = docker.BackupDestinationController
		}
		return func() tea.Msg { return SettingsSectionSubmitMsg{Settings: s} }
	})
	m.form.OnCancel(func(f *Form) tea.Cmd {
		return func() tea.Msg { return SettingsSectionCancelMsg{} }
	})

	return m
}

func (m SettingsFormBackups) Update(msg tea.Msg) (SettingsSection, tea.Cmd) {
	var cmd tea.Cmd
	m.settingsFormBase, cmd = m.update(msg)
	return m, cmd
}

// Helpers

func runBackup(app *docker.Application, dir string) error {
	return app.BackupToFile(context.Background(), dir, app.BackupName())
}
