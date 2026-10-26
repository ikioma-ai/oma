package docker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"
)

func (a *Application) prepareTLS(ctx context.Context) (certificatePath, privateKeyPath string, err error) {
	if a.Settings.TLSMode != TLSModeCustom || !a.Settings.TLSEnabled() {
		return "", "", nil
	}

	certificate, privateKey, err := loadTLSMaterial(a.Settings, time.Now())
	if err != nil {
		return "", "", err
	}

	return a.namespace.Proxy().StoreTLSMaterial(ctx, a.Settings.Name, certificate, privateKey)
}

func loadTLSMaterial(settings ApplicationSettings, now time.Time) (certificate, privateKey []byte, err error) {
	certificate, err = os.ReadFile(settings.TLSCertificatePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading TLS certificate: %w", err)
	}

	privateKey, err = os.ReadFile(settings.TLSPrivateKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading TLS private key: %w", err)
	}

	pair, err := tls.X509KeyPair(certificate, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("loading TLS certificate and key: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return nil, nil, fmt.Errorf("TLS certificate chain is empty")
	}

	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, nil, fmt.Errorf("parsing TLS certificate: %w", err)
	}
	if now.Before(leaf.NotBefore) {
		return nil, nil, fmt.Errorf("TLS certificate is not valid yet")
	}
	if now.After(leaf.NotAfter) {
		return nil, nil, fmt.Errorf("TLS certificate has expired")
	}
	if err := leaf.VerifyHostname(settings.Host); err != nil {
		return nil, nil, fmt.Errorf("TLS certificate does not cover %s: %w", settings.Host, err)
	}

	return certificate, privateKey, nil
}
