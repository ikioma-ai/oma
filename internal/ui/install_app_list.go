package ui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ikioma-ai/oma/internal/catalog"
)

const installAppListOtherKey = -1

type (
	InstallAppSelectedMsg struct {
		ImageRef  string
		CatalogID string
	}
	InstallCustomSelectedMsg struct{}
)

type InstallAppList struct {
	menu   Menu
	apps   []catalog.Entry
	issues []catalog.Issue
}

func NewInstallAppList() InstallAppList {
	store, err := catalog.Load()
	if err != nil {
		slog.Error("Failed to load app catalog", "error", err)
		store = catalog.Empty()
	}

	apps := store.List()
	items := make([]MenuItem, 0, len(apps)+1)
	for i, app := range apps {
		items = append(items, MenuItem{Label: appLabel(app), Key: i})
	}
	items = append(items, MenuItem{Label: "Custom Docker image", Key: installAppListOtherKey})

	return InstallAppList{
		menu:   NewMenu(items...),
		apps:   apps,
		issues: store.Issues(),
	}
}

func (m InstallAppList) Init() tea.Cmd {
	return nil
}

func (m InstallAppList) Update(msg tea.Msg) (InstallAppList, tea.Cmd) {
	switch msg := msg.(type) {
	case MenuSelectMsg:
		if msg.Key == installAppListOtherKey {
			return m, func() tea.Msg { return InstallCustomSelectedMsg{} }
		}
		if msg.Key < 0 || msg.Key >= len(m.apps) {
			return m, nil
		}
		app := m.apps[msg.Key]
		return m, func() tea.Msg {
			return InstallAppSelectedMsg{ImageRef: app.Image, CatalogID: app.ID}
		}
	}

	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

func (m InstallAppList) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Border).
		Padding(1, 4)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Colors.Primary).
		MarginBottom(1)

	title := titleStyle.Render("Choose an application")
	m.menu.SetWidth(lipgloss.Width(title))
	menuView := m.menu.View()

	content := lipgloss.JoinVertical(lipgloss.Left, title, menuView)

	if len(m.issues) > 0 {
		warning := lipgloss.NewStyle().
			Foreground(Colors.Error).
			Render(fmt.Sprintf("%d catalog entry skipped; run `oma catalog validate` for details", len(m.issues)))
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", warning)
	}

	return boxStyle.Render(content)
}

// Helpers

func appLabel(app catalog.Entry) string {
	return strings.Join([]string{app.Name, app.Description, app.Category, string(app.Source)}, " · ")
}

func findCatalogEntry(s string) (catalog.Entry, bool) {
	store, err := catalog.Load()
	if err != nil {
		return catalog.Entry{}, false
	}
	return store.Get(s)
}

func expandAlias(s string) (string, bool) {
	entry, ok := findCatalogEntry(s)
	if !ok {
		return s, false
	}
	return entry.Image, true
}
