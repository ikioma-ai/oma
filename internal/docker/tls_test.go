package docker

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTLSMaterial(t *testing.T) {
	now := time.Now()
	host := "app.local"

	t.Run("valid certificate", func(t *testing.T) {
		certificatePath, privateKeyPath := writeTLSMaterial(t, host, now.Add(-time.Hour), now.Add(time.Hour), nil)
		settings := ApplicationSettings{
			Host:               host,
			TLSCertificatePath: certificatePath,
			TLSPrivateKeyPath:  privateKeyPath,
		}

		certificate, privateKey, err := loadTLSMaterial(settings, now)
		require.NoError(t, err)
		assert.NotEmpty(t, certificate)
		assert.NotEmpty(t, privateKey)
	})

	t.Run("wrong hostname", func(t *testing.T) {
		certificatePath, privateKeyPath := writeTLSMaterial(t, "other.local", now.Add(-time.Hour), now.Add(time.Hour), nil)
		settings := ApplicationSettings{
			Host:               host,
			TLSCertificatePath: certificatePath,
			TLSPrivateKeyPath:  privateKeyPath,
		}

		_, _, err := loadTLSMaterial(settings, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not cover")
	})

	t.Run("expired", func(t *testing.T) {
		certificatePath, privateKeyPath := writeTLSMaterial(t, host, now.Add(-2*time.Hour), now.Add(-time.Hour), nil)
		settings := ApplicationSettings{
			Host:               host,
			TLSCertificatePath: certificatePath,
			TLSPrivateKeyPath:  privateKeyPath,
		}

		_, _, err := loadTLSMaterial(settings, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("not yet valid", func(t *testing.T) {
		certificatePath, privateKeyPath := writeTLSMaterial(t, host, now.Add(time.Hour), now.Add(2*time.Hour), nil)
		settings := ApplicationSettings{
			Host:               host,
			TLSCertificatePath: certificatePath,
			TLSPrivateKeyPath:  privateKeyPath,
		}

		_, _, err := loadTLSMaterial(settings, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not valid yet")
	})

	t.Run("mismatched key", func(t *testing.T) {
		certificatePath, _ := writeTLSMaterial(t, host, now.Add(-time.Hour), now.Add(time.Hour), nil)
		_, otherKeyPath := writeTLSMaterial(t, host, now.Add(-time.Hour), now.Add(time.Hour), nil)
		settings := ApplicationSettings{
			Host:               host,
			TLSCertificatePath: certificatePath,
			TLSPrivateKeyPath:  otherKeyPath,
		}

		_, _, err := loadTLSMaterial(settings, now)
		require.Error(t, err)
	})

	t.Run("missing certificate", func(t *testing.T) {
		_, privateKeyPath := writeTLSMaterial(t, host, now.Add(-time.Hour), now.Add(time.Hour), nil)
		settings := ApplicationSettings{
			Host:               host,
			TLSCertificatePath: filepath.Join(t.TempDir(), "missing.pem"),
			TLSPrivateKeyPath:  privateKeyPath,
		}

		_, _, err := loadTLSMaterial(settings, now)
		require.Error(t, err)
	})
}

func TestApplicationSettingsCustomTLS(t *testing.T) {
	settings := ApplicationSettings{
		Image:              "img:latest",
		Host:               "app.local",
		TLSMode:            TLSModeCustom,
		TLSCertificatePath: "/certs/app.pem",
		TLSPrivateKeyPath:  "/certs/app.key",
	}

	assert.NoError(t, settings.Validate())
	assert.True(t, settings.TLSEnabled())

	settings.TLSCertificatePath = ""
	assert.ErrorIs(t, settings.Validate(), ErrTLSCertificateRequired)

	settings.TLSCertificatePath = "/certs/app.pem"
	settings.TLSPrivateKeyPath = ""
	assert.ErrorIs(t, settings.Validate(), ErrTLSPrivateKeyRequired)

	settings.TLSPrivateKeyPath = "/certs/app.key"
	settings.TLSMode = "sometimes"
	assert.ErrorIs(t, settings.Validate(), ErrTLSModeInvalid)
}

func TestApplicationSettingsTLSModeCustomLocalhost(t *testing.T) {
	assert.True(t, ApplicationSettings{Host: "app.localhost", TLSMode: TLSModeCustom}.TLSEnabled())
	assert.False(t, ApplicationSettings{Host: "app.localhost", TLSMode: TLSModeCustom, DisableTLS: true}.TLSEnabled())
	assert.False(t, ApplicationSettings{Host: "app.localhost", TLSMode: TLSModeOff}.TLSEnabled())
}

func TestApplicationSettingsEqualIncludesTLSPaths(t *testing.T) {
	base := ApplicationSettings{Name: "app", Image: "img:latest", TLSCertificatePath: "/one", TLSPrivateKeyPath: "/two"}

	changed := base
	changed.TLSCertificatePath = "/different"
	assert.False(t, base.Equal(changed))

	changed = base
	changed.TLSPrivateKeyPath = "/different"
	assert.False(t, base.Equal(changed))
}

// Helpers

func writeTLSMaterial(t *testing.T, host string, notBefore, notAfter time.Time, key *ecdsa.PrivateKey) (certificatePath, privateKeyPath string) {
	t.Helper()

	if key == nil {
		var err error
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	dir := t.TempDir()
	certificatePath = filepath.Join(dir, "certificate.pem")
	privateKeyPath = filepath.Join(dir, "private-key.pem")
	require.NoError(t, os.WriteFile(certificatePath, certificatePEM, 0o600))
	require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))

	return certificatePath, privateKeyPath
}
