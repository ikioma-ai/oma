package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ikioma-ai/oma/internal/target"
)

var targetSwitcherCloseKey = WithHelp(NewKeyBinding("esc"), "esc", "close")

type TargetSwitcherCloseMsg struct{}

type TargetSelectedMsg struct {
	Name string
}

type TargetSwitcher struct {
	config  *target.Config
	current string
	err     error
	menu    Menu
	entries []target.Target
	help    Help
}

func NewTargetSwitcher(config *target.Config, current string, err error) TargetSwitcher {
	var entries []target.Target
	if config != nil {
		entries = config.SortedTargets()
	}

	items := make([]MenuItem, 0, len(entries)+1)
	for i, entry := range entries {
		label := entry.Name
		if entry.Name == current {
			label += " (current)"
		}
		if config != nil && entry.Name == config.Default && entry.Name != current {
			label += " (default)"
		}
		items = append(items, MenuItem{Label: label, Key: i})
	}
	items = append(items, MenuItem{Label: "Cancel", Key: -1})

	h := NewHelp()
	h.SetBindings([]key.Binding{targetSwitcherCloseKey})

	return TargetSwitcher{
		config:  config,
		current: current,
		err:     err,
		menu:    NewMenu(items...),
		entries: entries,
		help:    h,
	}
}

func (m TargetSwitcher) Init() tea.Cmd {
	return nil
}

func (m TargetSwitcher) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.help.SetWidth(msg.Width)

	case tea.KeyPressMsg:
		if key.Matches(msg, targetSwitcherCloseKey) {
			return m, func() tea.Msg { return TargetSwitcherCloseMsg{} }
		}

	case MenuSelectMsg:
		if msg.Key < 0 || msg.Key >= len(m.entries) {
			return m, func() tea.Msg { return TargetSwitcherCloseMsg{} }
		}
		entry := m.entries[msg.Key]
		if entry.Name == m.current {
			return m, func() tea.Msg { return TargetSwitcherCloseMsg{} }
		}
		return m, func() tea.Msg { return TargetSelectedMsg{Name: entry.Name} }
	}

	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

func (m TargetSwitcher) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(Colors.Primary).
		Render("Switch target")

	var parts []string
	parts = append(parts, title)

	if m.err != nil {
		parts = append(parts, "", lipgloss.NewStyle().Foreground(Colors.Error).Render(m.err.Error()))
	}

	parts = append(parts, "", m.menu.View(), "", m.help.View())

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Border).
		Padding(1, 4).
		Render(lipgloss.JoinVertical(lipgloss.Center, parts...))

	return box
}
