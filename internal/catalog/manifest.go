package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/distribution/reference"
	"go.yaml.in/yaml/v3"
)

const (
	CurrentSchemaVersion = 1
	KindWeb              = "web"

	DefaultPort          = 80
	DefaultHealthPath    = "/up"
	DefaultHealthTimeout = "5s"
	DefaultTLSMode       = "auto"
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	envNamePattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

var envFromSources = map[string]string{
	"base_url":      "BASE_URL",
	"disable_ssl":   "DISABLE_SSL",
	"num_cpus":      "NUM_CPUS",
	"smtp.server":   "SMTP_ADDRESS",
	"smtp.port":     "SMTP_PORT",
	"smtp.username": "SMTP_USERNAME",
	"smtp.password": "SMTP_PASSWORD",
	"smtp.from":     "MAILER_FROM_ADDRESS",
}

type Manifest struct {
	SchemaVersion int       `yaml:"schemaVersion"`
	Kind          string    `yaml:"kind"`
	ID            string    `yaml:"id"`
	Name          string    `yaml:"name"`
	Description   string    `yaml:"description"`
	Category      string    `yaml:"category"`
	Image         string    `yaml:"image"`
	Version       string    `yaml:"version"`
	Hostname      string    `yaml:"hostname"`
	Disabled      bool      `yaml:"disabled"`
	Web           Web       `yaml:"web"`
	Storage       []Storage `yaml:"storage"`
	Env           []EnvVar  `yaml:"env"`
	Hooks         Hooks     `yaml:"hooks"`
	Resources     Resources `yaml:"resources"`
}

type Web struct {
	Port          int    `yaml:"port"`
	HealthPath    string `yaml:"healthPath"`
	HealthPort    int    `yaml:"healthPort"`
	HealthTimeout string `yaml:"healthTimeout"`
	TLS           string `yaml:"tls"`
	Verify        *bool  `yaml:"verify"`
}

type Storage struct {
	Name    string `yaml:"name"`
	Path    string `yaml:"path"`
	Backup  bool   `yaml:"backup"`
	Primary bool   `yaml:"primary"`
}

type EnvVar struct {
	Name     string `yaml:"name"`
	Prompt   string `yaml:"prompt"`
	Default  string `yaml:"default"`
	Value    string `yaml:"value"`
	Generate string `yaml:"generate"`
	From     string `yaml:"from"`
	Required bool   `yaml:"required"`
	Secret   bool   `yaml:"secret"`
}

type Hooks struct {
	PreBackup   string `yaml:"preBackup"`
	PostRestore string `yaml:"postRestore"`
}

type Resources struct {
	CPUs     int `yaml:"cpus"`
	MemoryMB int `yaml:"memoryMB"`
}

func Parse(data []byte) (Manifest, error) {
	var manifest Manifest

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse: %w", err)
	}

	manifest.normalize()

	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func (m Manifest) Validate() error {
	schemaErrs := m.validateSchema()
	identityErrs := m.validateIdentity()

	if len(schemaErrs) > 0 || len(identityErrs) > 0 {
		return errors.Join(append(schemaErrs, identityErrs...)...)
	}

	if m.Disabled {
		return nil
	}

	var errs []error

	if strings.TrimSpace(m.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if strings.TrimSpace(m.Description) == "" {
		errs = append(errs, errors.New("description is required"))
	}
	if strings.TrimSpace(m.Category) == "" {
		errs = append(errs, errors.New("category is required"))
	}

	if m.Image == "" {
		errs = append(errs, errors.New("image is required"))
	} else if _, err := reference.ParseNormalizedNamed(m.Image); err != nil {
		errs = append(errs, fmt.Errorf("image: %w", err))
	}

	if m.Web.Port < 1 || m.Web.Port > 65535 {
		errs = append(errs, fmt.Errorf("web.port %d must be between 1 and 65535", m.Web.Port))
	}
	if m.Web.HealthPort < 1 || m.Web.HealthPort > 65535 {
		errs = append(errs, fmt.Errorf("web.healthPort %d must be between 1 and 65535", m.Web.HealthPort))
	}
	if !strings.HasPrefix(m.Web.HealthPath, "/") || strings.ContainsAny(m.Web.HealthPath, "?#") {
		errs = append(errs, fmt.Errorf("web.healthPath %q must be an absolute path without query or fragment", m.Web.HealthPath))
	}
	if timeout, err := time.ParseDuration(m.Web.HealthTimeout); err != nil {
		errs = append(errs, fmt.Errorf("web.healthTimeout: %w", err))
	} else if timeout <= 0 {
		errs = append(errs, errors.New("web.healthTimeout must be positive"))
	}
	if m.Web.TLS != "auto" && m.Web.TLS != "off" {
		errs = append(errs, fmt.Errorf("web.tls %q must be \"auto\" or \"off\"", m.Web.TLS))
	}

	errs = append(errs, m.validateStorage()...)
	errs = append(errs, m.validateEnv()...)

	for name, hook := range map[string]string{
		"hooks.preBackup":   m.Hooks.PreBackup,
		"hooks.postRestore": m.Hooks.PostRestore,
	} {
		if hook != "" && !validContainerPath(hook) {
			errs = append(errs, fmt.Errorf("%s %q must be an absolute container path", name, hook))
		}
	}

	if m.Resources.CPUs < 0 {
		errs = append(errs, errors.New("resources.cpus must not be negative"))
	}
	if m.Resources.MemoryMB < 0 {
		errs = append(errs, errors.New("resources.memoryMB must not be negative"))
	}

	return errors.Join(errs...)
}

func (m Manifest) VerifyEnabled() bool {
	return m.Web.Verify == nil || *m.Web.Verify
}

func (m Manifest) HealthTimeoutDuration() time.Duration {
	timeout, _ := time.ParseDuration(m.Web.HealthTimeout)
	return timeout
}

// Private

func (m Manifest) validateSchema() []error {
	var errs []error

	if m.SchemaVersion == 0 {
		errs = append(errs, errors.New("schemaVersion is required"))
	} else if m.SchemaVersion != CurrentSchemaVersion {
		errs = append(errs, fmt.Errorf("unsupported schemaVersion %d (want %d)", m.SchemaVersion, CurrentSchemaVersion))
	}

	if m.Kind != KindWeb {
		errs = append(errs, fmt.Errorf("unsupported kind %q (want %q)", m.Kind, KindWeb))
	}

	return errs
}

func (m Manifest) validateIdentity() []error {
	if !identifierPattern.MatchString(m.ID) {
		return []error{fmt.Errorf("id %q must match %s", m.ID, identifierPattern)}
	}
	return nil
}

func (m *Manifest) normalize() {
	if m.Kind == "" {
		m.Kind = KindWeb
	}
	if m.Web.Port == 0 {
		m.Web.Port = DefaultPort
	}
	if m.Web.HealthPath == "" {
		m.Web.HealthPath = DefaultHealthPath
	}
	if m.Web.HealthPort == 0 {
		m.Web.HealthPort = m.Web.Port
	}
	if m.Web.HealthTimeout == "" {
		m.Web.HealthTimeout = DefaultHealthTimeout
	}
	if m.Web.TLS == "" {
		m.Web.TLS = DefaultTLSMode
	}
}

func (m Manifest) validateStorage() []error {
	if len(m.Storage) == 0 {
		return nil
	}

	var errs []error

	volumeName := ""
	primaryIndex := 0
	primaryCount := 0
	paths := make(map[string]struct{}, len(m.Storage))

	for i, storage := range m.Storage {
		if !identifierPattern.MatchString(storage.Name) {
			errs = append(errs, fmt.Errorf("storage.name %q must match %s", storage.Name, identifierPattern))
		}
		if volumeName == "" {
			volumeName = storage.Name
		} else if storage.Name != volumeName {
			errs = append(errs, fmt.Errorf("storage must use one volume name, got %q and %q", volumeName, storage.Name))
		}

		if !validContainerPath(storage.Path) {
			errs = append(errs, fmt.Errorf("storage.path %q must be an absolute container path", storage.Path))
		}
		if _, exists := paths[storage.Path]; exists {
			errs = append(errs, fmt.Errorf("storage.path %q is mounted more than once", storage.Path))
		}
		paths[storage.Path] = struct{}{}

		if storage.Primary {
			primaryCount++
			primaryIndex = i
		}
	}

	if primaryCount > 1 {
		errs = append(errs, errors.New("storage has more than one primary mount target"))
	}

	for i, storage := range m.Storage {
		if storage.Backup && i != primaryIndex {
			errs = append(errs, fmt.Errorf("storage.path %q enables backup but is not the primary mount target", storage.Path))
		}
	}

	return errs
}

func (m Manifest) validateEnv() []error {
	seen := make(map[string]struct{}, len(m.Env))
	var errs []error

	for _, env := range m.Env {
		if !envNamePattern.MatchString(env.Name) {
			errs = append(errs, fmt.Errorf("env.name %q is not a valid environment variable name", env.Name))
			continue
		}
		if _, exists := seen[env.Name]; exists {
			errs = append(errs, fmt.Errorf("env.name %q is declared more than once", env.Name))
			continue
		}
		seen[env.Name] = struct{}{}

		sources := 0
		if env.Value != "" {
			sources++
		}
		if env.Generate != "" {
			sources++
		}
		if env.From != "" {
			sources++
		}
		if sources > 1 {
			errs = append(errs, fmt.Errorf("env %q must use only one of value, generate, or from", env.Name))
		}
		if env.Generate != "" && env.Generate != "hex32" {
			errs = append(errs, fmt.Errorf("env %q has unsupported generator %q", env.Name, env.Generate))
		}
		if env.From != "" {
			expectedName, ok := envFromSources[env.From]
			if !ok {
				errs = append(errs, fmt.Errorf("env %q has unsupported source %q", env.Name, env.From))
			} else if env.Name != expectedName {
				errs = append(errs, fmt.Errorf("env %q with source %q must be named %q", env.Name, env.From, expectedName))
			}
		}
	}

	return errs
}

// Helpers

func validContainerPath(p string) bool {
	if p == "" || !path.IsAbs(p) {
		return false
	}
	clean := path.Clean(p)
	return clean == p && clean != "/"
}
