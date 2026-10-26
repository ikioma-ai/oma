package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ikioma-ai/oma/internal/catalog"
	"github.com/ikioma-ai/oma/internal/catalogapply"
	"github.com/ikioma-ai/oma/internal/docker"
)

type SettingsFormCatalog struct {
	settingsFormBase

	app      *docker.Application
	hasEntry bool
	changes  []catalogapply.Change
}

func NewSettingsFormCatalog(app *docker.Application) SettingsFormCatalog {
	status := "This application has no catalog entry."
	var entry catalog.Entry
	var changes []catalogapply.Change
	hasEntry := false

	if app.Settings.CatalogID != "" {
		store, err := catalog.Load()
		if err != nil {
			status = "Could not load catalog: " + err.Error()
		} else if found, ok := store.Get(app.Settings.CatalogID); ok {
			entry = found
			hasEntry = true
			changes = catalogapply.Diff(app.Settings, entry)
			status = fmt.Sprintf("Catalog entry: %s (%s)", entry.ID, entry.Source)
			if len(changes) == 0 {
				status += " — no changes"
			}
		} else {
			status = fmt.Sprintf("No catalog entry %q.", app.Settings.CatalogID)
		}
	}

	submitLabel := "Apply"
	if !hasEntry || len(changes) == 0 {
		submitLabel = "Close"
	}

	m := SettingsFormCatalog{
		title:      "Catalog",
		form:       NewForm(submitLabel),
		statusLine: func() string { return status },
		viewFn:     func(Form) string { return renderCatalogChanges(changes) },
		app:        app,
		hasEntry:   hasEntry,
		changes:    changes,
	}

	m.form.OnSubmit(func(f *Form) tea.Cmd {
		if !hasEntry || len(changes) == 0 {
			return func() tea.Msg { return SettingsSectionCancelMsg{} }
		}
		return func() tea.Msg {
			return SettingsSectionApplyMsg{
				Settings: catalogapply.Build(app.Settings, entry),
				Pull:     catalogapply.ImageChanged(app.Settings, entry),
			}
		}
	})
	m.form.OnCancel(func(f *Form) tea.Cmd {
		return func() tea.Msg { return SettingsSectionCancelMsg{} }
	})

	return m
}

func (m SettingsFormCatalog) Update(msg tea.Msg) (SettingsSection, tea.Cmd) {
	var cmd tea.Cmd
	m.settingsFormBase, cmd = m.update(msg)
	return m, cmd
}

// Helpers

func renderCatalogChanges(changes []catalogapply.Change) string {
	if len(changes) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Changes:\n\n")
	for _, change := range changes {
		fmt.Fprintf(&builder, "%-16s %s → %s\n", change.Field, change.Old, change.New)
	}
	return builder.String()
}
