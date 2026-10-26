package target

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/moby/moby/client"
	"go.yaml.in/yaml/v3"
)

const (
	AutomationOwnerController = "controller"
	AutomationOwnerAgent      = "agent"

	LocalName           = "local"
	DefaultNamespace    = "oma"
	DefaultDockerSocket = "/var/run/docker.sock"
)

var (
	ErrUnknownTarget = errors.New("unknown target")
	ErrTargetExists  = errors.New("target already exists")
	ErrLocalTarget   = errors.New("the local target is implicit and cannot be modified")
)

var targetNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Target struct {
	Name            string
	SSH             string
	DockerSocket    string
	Namespace       string
	AgentVersion    string
	AutomationOwner string
	Local           bool
}

type Config struct {
	Default string
	Targets map[string]Target

	path string
}

type fileConfig struct {
	Default string                `yaml:"default"`
	Targets map[string]fileTarget `yaml:"targets"`
}

type fileTarget struct {
	SSH             string `yaml:"ssh,omitempty"`
	DockerSocket    string `yaml:"dockerSocket,omitempty"`
	Namespace       string `yaml:"namespace,omitempty"`
	AgentVersion    string `yaml:"agentVersion,omitempty"`
	AutomationOwner string `yaml:"automationOwner,omitempty"`
}

func ConfigPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "oma", "config.yaml"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "oma", "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(path), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading target config: %w", err)
	}

	var file fileConfig
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("parsing target config: %w", err)
		}
	}

	config := defaultConfig(path)
	if file.Default != "" {
		config.Default = file.Default
	}

	for name, entry := range file.Targets {
		if !targetNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid target name %q", name)
		}

		target := Target{
			Name:            name,
			SSH:             strings.TrimSpace(entry.SSH),
			DockerSocket:    entry.DockerSocket,
			Namespace:       entry.Namespace,
			AgentVersion:    entry.AgentVersion,
			AutomationOwner: entry.AutomationOwner,
		}
		if name == LocalName {
			if target.SSH != "" {
				return nil, fmt.Errorf("target %q cannot define SSH", name)
			}
			target.Local = true
			if target.DockerSocket != "" {
				return nil, fmt.Errorf("target %q cannot define a Docker socket", name)
			}
			if target.Namespace == "" {
				target.Namespace = DefaultNamespace
			}
		} else {
			if !validSSHDestination(target.SSH) {
				return nil, fmt.Errorf("target %q has an invalid SSH destination", name)
			}
			if target.DockerSocket == "" {
				target.DockerSocket = DefaultDockerSocket
			}
			if !pathpkg.IsAbs(target.DockerSocket) {
				return nil, fmt.Errorf("target %q Docker socket must be an absolute path", name)
			}
			if target.Namespace == "" {
				target.Namespace = DefaultNamespace
			}
		}
		if target.Namespace != "" && !targetNamePattern.MatchString(target.Namespace) {
			return nil, fmt.Errorf("target %q has invalid namespace %q", name, target.Namespace)
		}
		if target.AutomationOwner != "" && target.AutomationOwner != AutomationOwnerController && target.AutomationOwner != AutomationOwnerAgent {
			return nil, fmt.Errorf("target %q has invalid automation owner %q", name, target.AutomationOwner)
		}

		config.Targets[name] = target
	}

	if config.Default == "" {
		config.Default = LocalName
	}
	if _, ok := config.Targets[config.Default]; !ok {
		return nil, fmt.Errorf("default target %q is not defined", config.Default)
	}

	return config, nil
}

func (c *Config) Save() error {
	if c.path == "" {
		return errors.New("target config has no path")
	}
	return c.SaveTo(c.path)
}

func (c *Config) SaveTo(path string) error {
	file := fileConfig{
		Default: c.Default,
		Targets: make(map[string]fileTarget),
	}

	for name, target := range c.Targets {
		if target.Local {
			continue
		}
		file.Targets[name] = fileTarget{
			SSH:             target.SSH,
			DockerSocket:    target.DockerSocket,
			Namespace:       target.Namespace,
			AgentVersion:    target.AgentVersion,
			AutomationOwner: target.AutomationOwner,
		}
	}

	data, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("marshaling target config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing target config: %w", err)
	}
	return nil
}

func (c *Config) Add(target Target) error {
	if target.Name == LocalName {
		return ErrLocalTarget
	}
	if !targetNamePattern.MatchString(target.Name) {
		return fmt.Errorf("invalid target name %q", target.Name)
	}
	if !validSSHDestination(target.SSH) {
		return fmt.Errorf("target %q requires a valid SSH destination", target.Name)
	}
	if target.DockerSocket == "" {
		target.DockerSocket = DefaultDockerSocket
	}
	if !pathpkg.IsAbs(target.DockerSocket) {
		return fmt.Errorf("target %q Docker socket must be an absolute path", target.Name)
	}
	if target.Namespace == "" {
		target.Namespace = DefaultNamespace
	}
	if !targetNamePattern.MatchString(target.Namespace) {
		return fmt.Errorf("target %q has invalid namespace %q", target.Name, target.Namespace)
	}
	if _, ok := c.Targets[target.Name]; ok {
		return fmt.Errorf("%w: %s", ErrTargetExists, target.Name)
	}
	c.Targets[target.Name] = target
	return nil
}

func (c *Config) Remove(name, newDefault string) error {
	if name == LocalName {
		return ErrLocalTarget
	}
	if _, ok := c.Targets[name]; !ok {
		return fmt.Errorf("%w: %s", ErrUnknownTarget, name)
	}
	if name == c.Default {
		if newDefault == "" {
			return fmt.Errorf("target %q is the default; choose a new default", name)
		}
		if newDefault == name {
			return fmt.Errorf("new default must differ from the removed target")
		}
		if _, ok := c.Targets[newDefault]; !ok {
			return fmt.Errorf("%w: %s", ErrUnknownTarget, newDefault)
		}
		c.Default = newDefault
	}
	delete(c.Targets, name)
	return nil
}

func (c *Config) SetDefault(name string) error {
	if _, ok := c.Targets[name]; !ok {
		return fmt.Errorf("%w: %s", ErrUnknownTarget, name)
	}
	c.Default = name
	return nil
}

func (c *Config) Resolve(flagValue, envValue string) (Target, error) {
	name := flagValue
	if name == "" {
		name = envValue
	}
	if name == "" {
		name = c.Default
	}
	if name == "" {
		name = LocalName
	}

	target, ok := c.Targets[name]
	if !ok {
		available := make([]string, 0, len(c.Targets))
		for targetName := range c.Targets {
			available = append(available, targetName)
		}
		sort.Strings(available)
		return Target{}, fmt.Errorf("%w %q (available: %s)", ErrUnknownTarget, name, strings.Join(available, ", "))
	}

	return target, nil
}

func (c *Config) SortedTargets() []Target {
	targets := make([]Target, 0, len(c.Targets))
	for _, target := range c.Targets {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Local != targets[j].Local {
			return targets[i].Local
		}
		return targets[i].Name < targets[j].Name
	})
	return targets
}

func (t Target) AutomationOwnerOrDefault() string {
	if t.AutomationOwner == "" {
		return AutomationOwnerController
	}
	return t.AutomationOwner
}

func (t Target) Transport() (Transport, error) {
	if t.Local {
		return LocalTransport{}, nil
	}
	if !validSSHDestination(t.SSH) {
		return nil, fmt.Errorf("target %q requires a valid SSH destination", t.Name)
	}
	if !pathpkg.IsAbs(t.DockerSocket) {
		return nil, fmt.Errorf("target %q requires an absolute Docker socket path", t.Name)
	}
	return SSHTransport{Destination: t.SSH, DockerSocket: t.DockerSocket}, nil
}

type Transport interface {
	Connect(ctx context.Context) (*client.Client, io.Closer, error)
}

type LocalTransport struct{}

func (LocalTransport) Connect(ctx context.Context) (*client.Client, io.Closer, error) {
	client, err := client.New(client.FromEnv)
	if err != nil {
		return nil, nil, err
	}
	return client, client, nil
}

// Helpers

func validSSHDestination(destination string) bool {
	if destination == "" || strings.HasPrefix(destination, "-") {
		return false
	}
	return !strings.ContainsAny(destination, " \t\r\n")
}

func defaultConfig(path string) *Config {
	return &Config{
		Default: LocalName,
		Targets: map[string]Target{
			LocalName: {Name: LocalName, Local: true, Namespace: DefaultNamespace},
		},
		path: path,
	}
}
