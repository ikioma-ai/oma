package docker

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"
)

type Keys struct {
	SecretKeyBase   string `json:"secretKeyBase,omitempty"`
	VAPIDPublicKey  string `json:"vapidPublicKey,omitempty"`
	VAPIDPrivateKey string `json:"vapidPrivateKey,omitempty"`
}

func (k Keys) Empty() bool {
	return k == Keys{}
}

func (k *Keys) SetVAPIDKey(privateKey string) error {
	key, err := parseVAPIDPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("invalid VAPID private key: %w", err)
	}

	k.VAPIDPublicKey = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	k.VAPIDPrivateKey = base64.RawURLEncoding.EncodeToString(key.Bytes())

	return nil
}

func (k *Keys) Regenerate(secretKeyBase, vapid bool) error {
	if secretKeyBase {
		skb, err := generateSecretKeyBase()
		if err != nil {
			return err
		}
		k.SecretKeyBase = skb
	}

	if vapid {
		pub, priv, err := generateVAPIDKeyPair()
		if err != nil {
			return err
		}
		k.VAPIDPublicKey = pub
		k.VAPIDPrivateKey = priv
	}

	return nil
}

type SMTPSettings struct {
	Server   string `json:"server,omitempty"`
	Port     string `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	From     string `json:"from,omitempty"`
}

func (s SMTPSettings) BuildEnv() []string {
	if s.Server == "" {
		return nil
	}
	return []string{
		"SMTP_ADDRESS=" + s.Server,
		"SMTP_PORT=" + s.Port,
		"SMTP_USERNAME=" + s.Username,
		"SMTP_PASSWORD=" + s.Password,
		"MAILER_FROM_ADDRESS=" + s.From,
	}
}

type RegistrySettings struct {
	Host     string `json:"host,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func (r RegistrySettings) Empty() bool {
	return r == RegistrySettings{}
}

type ContainerResources struct {
	CPUs     int `json:"cpus,omitempty"`
	MemoryMB int `json:"memoryMB,omitempty"`
}

type BackupSettings struct {
	Path       string `json:"path,omitempty"`
	AutoBackup bool   `json:"autoBackup,omitempty"`
}

type StorageMount struct {
	Path    string `json:"path"`
	Primary bool   `json:"primary,omitempty"`
	Backup  bool   `json:"backup,omitempty"`
}

type ApplicationSettings struct {
	Name               string             `json:"name"`
	Image              string             `json:"image"`
	Host               string             `json:"host"`
	DisableTLS         bool               `json:"disableTLS"`
	CatalogID          string             `json:"catalogID,omitempty"`
	CatalogSource      string             `json:"catalogSource,omitempty"`
	ServicePort        int                `json:"servicePort,omitempty"`
	HealthCheckPath    string             `json:"healthCheckPath,omitempty"`
	HealthCheckPort    int                `json:"healthCheckPort,omitempty"`
	HealthCheckTimeout string             `json:"healthCheckTimeout,omitempty"`
	TLSMode            string             `json:"tlsMode,omitempty"`
	Verify             *bool              `json:"verify,omitempty"`
	Storage            []StorageMount     `json:"storage,omitempty"`
	EnvVars            map[string]string  `json:"env"`
	SMTP               SMTPSettings       `json:"smtp"`
	Resources          ContainerResources `json:"resources"`
	AutoUpdate         bool               `json:"autoUpdate"`
	Backup             BackupSettings     `json:"backup"`
	Keys               Keys               `json:"keys"`
	Registry           RegistrySettings   `json:"registry"`
}

func UnmarshalApplicationSettings(s string) (ApplicationSettings, error) {
	var settings ApplicationSettings
	err := json.Unmarshal([]byte(s), &settings)
	return settings, err
}

func (s ApplicationSettings) Marshal() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (s ApplicationSettings) Validate() error {
	if s.Image == "" {
		return ErrImageRequired
	}
	if s.Backup.AutoBackup && s.Backup.Path == "" {
		return ErrAutoBackupWithoutPath
	}
	if s.Registry.Password != "" && s.Registry.Username == "" {
		return ErrRegistryPasswordWithoutUsername
	}
	if s.Registry.Username != "" && s.Registry.Password == "" {
		return ErrRegistryUsernameWithoutPassword
	}
	if s.ServicePort != 0 && (s.ServicePort < 1 || s.ServicePort > 65535) {
		return fmt.Errorf("service port %d must be between 1 and 65535", s.ServicePort)
	}
	if s.HealthCheckPort != 0 && (s.HealthCheckPort < 1 || s.HealthCheckPort > 65535) {
		return fmt.Errorf("health check port %d must be between 1 and 65535", s.HealthCheckPort)
	}
	if s.HealthCheckPath != "" && (!strings.HasPrefix(s.HealthCheckPath, "/") || strings.ContainsAny(s.HealthCheckPath, "?#")) {
		return fmt.Errorf("health check path %q must be an absolute path without query or fragment", s.HealthCheckPath)
	}
	if s.HealthCheckTimeout != "" {
		timeout, err := time.ParseDuration(s.HealthCheckTimeout)
		if err != nil || timeout <= 0 {
			return fmt.Errorf("health check timeout %q must be a positive duration", s.HealthCheckTimeout)
		}
	}
	if s.TLSMode != "" && s.TLSMode != "auto" && s.TLSMode != "off" {
		return fmt.Errorf("TLS mode %q must be \"auto\" or \"off\"", s.TLSMode)
	}
	if err := validateStorageMounts(s.Storage); err != nil {
		return err
	}
	return nil
}

func (s ApplicationSettings) TLSEnabled() bool {
	if s.TLSMode == "off" {
		return false
	}
	return s.Host != "" && !s.DisableTLS && !IsLocalhost(s.Host)
}

func (s ApplicationSettings) VerifyEnabled() bool {
	return s.Verify == nil || *s.Verify
}

func (s ApplicationSettings) Equal(other ApplicationSettings) bool {
	if s.Name != other.Name || s.Image != other.Image || s.Host != other.Host || s.DisableTLS != other.DisableTLS {
		return false
	}
	if s.CatalogID != other.CatalogID || s.CatalogSource != other.CatalogSource {
		return false
	}
	if s.ServicePort != other.ServicePort || s.HealthCheckPath != other.HealthCheckPath ||
		s.HealthCheckPort != other.HealthCheckPort || s.HealthCheckTimeout != other.HealthCheckTimeout {
		return false
	}
	if s.TLSMode != other.TLSMode {
		return false
	}
	if (s.Verify == nil) != (other.Verify == nil) {
		return false
	}
	if s.Verify != nil && *s.Verify != *other.Verify {
		return false
	}
	if !slices.Equal(s.Storage, other.Storage) {
		return false
	}
	if s.Resources != other.Resources {
		return false
	}
	if s.SMTP != other.SMTP {
		return false
	}
	if s.AutoUpdate != other.AutoUpdate {
		return false
	}
	if s.Backup != other.Backup {
		return false
	}
	if s.Keys != other.Keys {
		return false
	}
	if s.Registry != other.Registry {
		return false
	}
	if len(s.EnvVars) != len(other.EnvVars) {
		return false
	}
	for k, v := range s.EnvVars {
		if other.EnvVars[k] != v {
			return false
		}
	}
	return true
}

// Helpers

func validateStorageMounts(mounts []StorageMount) error {
	if len(mounts) == 0 {
		return nil
	}

	primaryIndex := 0
	primaryCount := 0
	paths := make(map[string]struct{}, len(mounts))

	for i, mount := range mounts {
		if !path.IsAbs(mount.Path) || path.Clean(mount.Path) != mount.Path || mount.Path == "/" {
			return fmt.Errorf("storage path %q must be an absolute container path", mount.Path)
		}
		if _, exists := paths[mount.Path]; exists {
			return fmt.Errorf("storage path %q is mounted more than once", mount.Path)
		}
		paths[mount.Path] = struct{}{}

		if mount.Primary {
			primaryCount++
			primaryIndex = i
		}
	}

	if primaryCount > 1 {
		return fmt.Errorf("storage has more than one primary mount target")
	}

	for i, mount := range mounts {
		if mount.Backup && i != primaryIndex {
			return fmt.Errorf("storage path %q enables backup but is not the primary mount target", mount.Path)
		}
	}

	return nil
}

func generateSecretKeyBase() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generateVAPIDKeyPair() (publicKey, privateKey string, err error) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}

	privateKey = base64.RawURLEncoding.EncodeToString(key.Bytes())
	publicKey = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())

	return publicKey, privateKey, nil
}

// parseVAPIDPrivateKey accepts any common base64 dialect (url-safe or
// standard, padded or not) of a raw P-256 private key.
func parseVAPIDPrivateKey(s string) (*ecdh.PrivateKey, error) {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("+", "-", "/", "_").Replace(s)
	s = strings.TrimRight(s, "=")

	bytes, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	return ecdh.P256().NewPrivateKey(bytes)
}
