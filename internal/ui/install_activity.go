package ui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ikioma-ai/oma/internal/catalog"
	"github.com/ikioma-ai/oma/internal/docker"
)

type installStage int

const (
	stagePreparing installStage = iota
	stageDownloading
	stageStarting
	stageVerifying
)

type installProgressMsg struct {
	stage      installStage
	percentage int
}

type installDoneMsg struct {
	app *docker.Application
	err error
}

type InstallActivityDoneMsg struct {
	App *docker.Application
}

type InstallActivityFailedMsg struct {
	Err error
}

type InstallActivity struct {
	namespace     *docker.Namespace
	imageRef      string
	hostname      string
	catalogID     string
	envValues     map[string]string
	registry      docker.RegistrySettings
	width, height int
	stage         installStage
	percentage    int
	progress      Progress
	progressChan  chan installProgressMsg
	doneChan      chan installDoneMsg
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewInstallActivity(ns *docker.Namespace, imageRef, hostname, catalogID string, envValues map[string]string, registry docker.RegistrySettings) *InstallActivity {
	ctx, cancel := context.WithCancel(context.Background())
	return &InstallActivity{
		namespace:    ns,
		imageRef:     imageRef,
		hostname:     hostname,
		catalogID:    catalogID,
		envValues:    envValues,
		registry:     registry,
		stage:        stagePreparing,
		progress:     NewProgress(0, Colors.Primary),
		progressChan: make(chan installProgressMsg, 10),
		doneChan:     make(chan installDoneMsg, 1),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (m *InstallActivity) Init() tea.Cmd {
	return tea.Batch(m.progress.Init(), m.startInstall(), m.waitForProgress())
}

func (m *InstallActivity) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress = m.progress.SetWidth(min(m.width-4, 60))

	case installProgressMsg:
		m.stage = msg.stage
		m.percentage = msg.percentage
		switch msg.stage {
		case stageDownloading:
			m.progress = m.progress.SetPercent(msg.percentage)
		default:
			m.progress = m.progress.SetPercent(-1)
		}
		return m.waitForProgress()

	case installDoneMsg:
		if msg.err != nil {
			return func() tea.Msg { return InstallActivityFailedMsg{Err: msg.err} }
		}
		return func() tea.Msg { return InstallActivityDoneMsg{App: msg.app} }

	case ProgressTickMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return cmd
	}

	return nil
}

func (m *InstallActivity) View() string {
	var status string
	switch m.stage {
	case stagePreparing:
		status = "Preparing..."
	case stageDownloading:
		status = "Downloading..."
	case stageStarting:
		status = "Starting..."
	case stageVerifying:
		status = "Verifying..."
	}

	statusLine := Styles.CenteredLine(m.width, status)

	progressView := Styles.CenteredLine(m.width, m.progress.View())

	return lipgloss.JoinVertical(lipgloss.Left, statusLine, progressView)
}

func (m *InstallActivity) Cancel() {
	if m.cancel != nil {
		m.cancel()
	}
}

// Private

func (m *InstallActivity) startInstall() tea.Cmd {
	return func() tea.Msg {
		go m.runInstall(m.ctx)
		return nil
	}
}

func (m *InstallActivity) waitForProgress() tea.Cmd {
	return func() tea.Msg {
		select {
		case progress, ok := <-m.progressChan:
			if ok {
				return progress
			}
		case done := <-m.doneChan:
			return done
		}
		return nil
	}
}

func (m *InstallActivity) runInstall(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			m.doneChan <- installDoneMsg{err: fmt.Errorf("install panicked: %v", r)}
		}
	}()

	m.progressChan <- installProgressMsg{stage: stagePreparing}

	if err := m.namespace.Setup(ctx); err != nil {
		m.doneChan <- installDoneMsg{err: fmt.Errorf("%w: %w", docker.ErrSetupFailed, err)}
		return
	}

	m.progressChan <- installProgressMsg{stage: stageDownloading, percentage: 0}

	appName, err := m.namespace.UniqueName(docker.NameFromImageRef(m.imageRef))
	if err != nil {
		m.doneChan <- installDoneMsg{err: fmt.Errorf("generating app name: %w", err)}
		return
	}
	hostname := m.hostname

	settings, err := m.applicationSettings(appName, hostname)
	if err != nil {
		m.doneChan <- installDoneMsg{err: fmt.Errorf("configuring environment: %w", err)}
		return
	}

	app := docker.NewApplication(m.namespace, settings)

	progress := func(p docker.DeployProgress) {
		switch p.Stage {
		case docker.DeployStageDownloading:
			m.progressChan <- installProgressMsg{stage: stageDownloading, percentage: p.Percentage}
		case docker.DeployStageStarting:
			m.progressChan <- installProgressMsg{stage: stageStarting, percentage: 100}
		}
	}

	if err := app.Deploy(ctx, progress); err != nil {
		if cleanupErr := app.Destroy(context.Background(), true); cleanupErr != nil {
			slog.Error("Failed to clean up after deploy failure", "app", appName, "error", cleanupErr)
		}
		m.doneChan <- installDoneMsg{err: fmt.Errorf("%w: %w", docker.ErrDeployFailed, err)}
		return
	}

	m.progressChan <- installProgressMsg{stage: stageVerifying}

	if err := app.VerifyHTTPOrRemove(ctx); err != nil {
		m.doneChan <- installDoneMsg{err: err}
		return
	}

	m.doneChan <- installDoneMsg{app: app}
}

func (m *InstallActivity) applicationSettings(appName, hostname string) (docker.ApplicationSettings, error) {
	settings := docker.ApplicationSettings{
		Name:       appName,
		Image:      m.imageRef,
		Host:       hostname,
		AutoUpdate: true,
		Registry:   m.registry,
	}

	if m.catalogID == "" {
		return settings, nil
	}

	store, err := catalog.Load()
	if err != nil {
		return settings, fmt.Errorf("loading catalog: %w", err)
	}

	entry, ok := store.Get(m.catalogID)
	if !ok {
		return settings, fmt.Errorf("catalog entry %q not found", m.catalogID)
	}

	return settingsFromCatalogEntry(settings, entry, m.envValues)
}

// Helpers

func settingsFromCatalogEntry(settings docker.ApplicationSettings, entry catalog.Entry, values map[string]string) (docker.ApplicationSettings, error) {
	settings.CatalogID = entry.ID
	settings.CatalogSource = string(entry.Source)
	settings.ServicePort = entry.Web.Port
	settings.HealthCheckPath = entry.Web.HealthPath
	settings.HealthCheckPort = entry.Web.HealthPort
	settings.HealthCheckTimeout = entry.Web.HealthTimeout
	settings.TLSMode = entry.Web.TLS

	if entry.Web.Verify != nil {
		verify := *entry.Web.Verify
		settings.Verify = &verify
	}

	if len(entry.Storage) > 0 {
		settings.Storage = make([]docker.StorageMount, len(entry.Storage))
		for i, storage := range entry.Storage {
			settings.Storage[i] = docker.StorageMount{
				Path:    storage.Path,
				Primary: storage.Primary,
				Backup:  storage.Backup,
			}
		}
	}

	if err := applyCatalogEnv(&settings, entry.Env, values); err != nil {
		return settings, err
	}

	return settings, nil
}

func applyCatalogEnv(settings *docker.ApplicationSettings, env []catalog.EnvVar, values map[string]string) error {
	if len(env) == 0 {
		return nil
	}

	vars := make(map[string]string, len(settings.EnvVars)+len(env))
	for name, value := range settings.EnvVars {
		vars[name] = value
	}

	var secrets []string

	for _, variable := range env {
		switch {
		case variable.Value != "":
			vars[variable.Name] = variable.Value
		case variable.From != "":
			// BuildEnv derives standard system values from the app settings.
		case variable.Generate != "":
			value, err := generateEnvValue(variable.Generate)
			if err != nil {
				return fmt.Errorf("generating %s: %w", variable.Name, err)
			}
			vars[variable.Name] = value
		case values[variable.Name] != "":
			vars[variable.Name] = values[variable.Name]
		case variable.Default != "":
			vars[variable.Name] = variable.Default
		case variable.Required:
			return fmt.Errorf("environment variable %s is required", variable.Name)
		}

		if variable.Secret {
			secrets = append(secrets, variable.Name)
		}
	}

	if len(vars) > 0 {
		settings.EnvVars = vars
	}
	sort.Strings(secrets)
	settings.SecretEnv = secrets

	return nil
}

func generateEnvValue(kind string) (string, error) {
	switch kind {
	case "hex32":
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		return hex.EncodeToString(bytes), nil
	default:
		return "", fmt.Errorf("unsupported generator %q", kind)
	}
}
