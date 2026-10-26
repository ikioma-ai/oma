package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/docker"
	"github.com/ikioma-ai/oma/internal/metrics"
	"github.com/ikioma-ai/oma/internal/system"
	"github.com/ikioma-ai/oma/internal/target"
)

func TestTargetSwitcherListsTargets(t *testing.T) {
	config := &target.Config{
		Default: "local",
		Targets: map[string]target.Target{
			"local": {Name: "local", Local: true},
			"vps":   {Name: "vps", SSH: "deploy@example.com"},
		},
	}

	switcher := NewTargetSwitcher(config, "local", nil)
	view := switcher.View()

	assert.Contains(t, view, "Switch target")
	assert.Contains(t, view, "local (current)")
	assert.Contains(t, view, "vps")
	assert.Contains(t, view, "Cancel")
}

func TestTargetSwitcherSelectsTarget(t *testing.T) {
	config := &target.Config{
		Default: "local",
		Targets: map[string]target.Target{
			"local": {Name: "local", Local: true},
			"vps":   {Name: "vps", SSH: "deploy@example.com"},
		},
	}

	component, _ := NewTargetSwitcher(config, "local", nil).Update(keyPressMsg("down"))
	comp, cmd := component.Update(keyPressMsg("enter"))
	switcher := comp.(TargetSwitcher)
	require.NotNil(t, cmd)

	_, cmd = switcher.Update(cmd().(MenuSelectMsg))
	require.NotNil(t, cmd)

	msg := cmd()
	selected, ok := msg.(TargetSelectedMsg)
	require.True(t, ok, "expected TargetSelectedMsg, got %T", msg)
	assert.Equal(t, "vps", selected.Name)
}

func TestTargetSwitcherCurrentTargetCloses(t *testing.T) {
	config := &target.Config{
		Default: "local",
		Targets: map[string]target.Target{
			"local": {Name: "local", Local: true},
		},
	}

	component, _ := NewTargetSwitcher(config, "local", nil).Update(keyPressMsg("enter"))
	comp, cmd := component.Update(keyPressMsg("enter"))
	switcher := comp.(TargetSwitcher)
	require.NotNil(t, cmd)

	_, cmd = switcher.Update(cmd().(MenuSelectMsg))
	require.NotNil(t, cmd)
	_, ok := cmd().(TargetSwitcherCloseMsg)
	assert.True(t, ok)
}

func TestTargetSwitcherShowsError(t *testing.T) {
	switcher := NewTargetSwitcher(nil, "local", errors.New("SSH authentication failed"))

	assert.Contains(t, switcher.View(), "SSH authentication failed")
}

func TestDashboardTargetKey(t *testing.T) {
	dashboard := testDashboard(1)
	_, cmd := updateDashboard(dashboard, keyPressMsg("t"))
	require.NotNil(t, cmd)

	_, ok := cmd().(NavigateToTargetSwitcherMsg)
	assert.True(t, ok)
}

func TestDashboardShowsTargetName(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithTargetName("vps"))
	require.NoError(t, err)

	scraper := metrics.NewMetricsScraper(metrics.ScraperSettings{})
	systemScraper := system.NewScraper(system.ScraperSettings{BufferSize: 10})
	dashboard := NewDashboard(ns, nil, 0, scraper, &docker.Scraper{}, systemScraper, nil)
	dashboard, _ = updateDashboard(dashboard, tea.WindowSizeMsg{Width: 120, Height: 40})

	assert.Contains(t, dashboard.View(), "vps")
}

func TestTargetSwitchFailureKeepsCurrentTarget(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithTargetName("local"))
	require.NoError(t, err)

	app := &App{namespace: ns}
	app.Update(targetSwitchDoneMsg{err: errors.New("SSH authentication failed")})

	_, ok := app.currentScreen.(TargetSwitcher)
	assert.True(t, ok)
	assert.Equal(t, "local", app.namespace.TargetName())
}

func TestDashboardTargetKeyBlockedDuringOperation(t *testing.T) {
	dashboard := testDashboard(1)
	dashboard.toggling = true

	_, cmd := updateDashboard(dashboard, keyPressMsg("t"))
	assert.Nil(t, cmd)
}

func TestNewAppRemoteMarksTelemetryUnavailable(t *testing.T) {
	ns, err := docker.NewNamespace("test", docker.WithTargetName("remote"))
	require.NoError(t, err)

	app := NewApp(ns, "")
	defer app.shutdown()

	assert.False(t, app.systemScraper.Available())
	assert.False(t, app.scraper.Available())
}
