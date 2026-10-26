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

const (
	backupDestinationController = iota
	backupDestinationTarget
	backupDestinationObject
)

type SettingsFormBackups struct {
	settingsFormBase
}

func NewSettingsFormBackups(app *docker.Application, lastResult *docker.OperationResult) SettingsFormBackups {
	pathField := NewTextField("/path/to/backups")
	pathField.SetValue(app.Settings.Backup.Path)

	autoBackupField := NewCheckboxField("Automatically create backups", app.Settings.Backup.AutoBackup)
	destinationField := NewSelectField("Controller", "Target", "Object storage")
	switch app.Settings.Backup.DestinationOrDefault() {
	case docker.BackupDestinationTarget:
		destinationField.SetIndex(backupDestinationTarget)
	case docker.BackupDestinationObject:
		destinationField.SetIndex(backupDestinationObject)
	}

	m := SettingsFormBackups{
		title: "Backups",
		form: NewForm("Done",
			FormItem{Label: "Backup location", Field: pathField},
			FormItem{Label: "Backups", Field: autoBackupField},
			FormItem{Label: "Destination", Field: destinationField},
		),
		statusLine: func() string {
			line := "Backups stored on the controller"
			switch destinationField.Index() {
			case backupDestinationTarget:
				line = "Backups stored on the target; a target runner is required"
			case backupDestinationObject:
				line = "Backups stored in object storage; configure credentials on the target and use the target agent"
			}
			if status := formatOperationStatus("backup", lastResult); status != "" {
				return status + " · " + line
			}
			return line
		},
	}

	m.form.SetActionButton("Run backup now", func() tea.Msg {
		return settingsRunActionMsg{action: func() (string, error) {
			if destinationField.Index() != backupDestinationController {
				return "", errors.New("target and object backups run on the target agent")
			}
			return "Backup complete", runBackup(app, pathField.Value())
		}}
	})
	m.form.OnSubmit(func(f *Form) tea.Cmd {
		s := app.Settings
		s.Backup.Path = f.TextField(backupsPathField).Value()
		s.Backup.AutoBackup = f.CheckboxField(backupsAutoBackupField).Checked()
		switch f.SelectField(backupsDestinationField).Index() {
		case backupDestinationTarget:
			s.Backup.Destination = docker.BackupDestinationTarget
		case backupDestinationObject:
			s.Backup.Destination = docker.BackupDestinationObject
			s.Backup.Path = ""
		default:
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
