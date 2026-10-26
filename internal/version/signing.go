package version

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

const releaseSigningPublicKey = `-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAua8ohkTHTlanRR3sbo5I
iCuTbYafEng6K0fHkfGPsBIcN63fWclmBAkOT87e1N8cby/5zi24kxfASgEHHOgR
BvKp+Rff1zBk9iCvirSmmgCNwkqjWbLpGSH6wxc/7na3gxyNzaRwjxej3gXAoM7D
R9HKzvVuPiBrY3D0sYI1oRFSrgaggKgouR0Xa3kzdcjIf7nK723Coft16snoLq7Q
mzi+Pbg0dz3Kuhj1QEgn1hgj8V9QkZYkV4UgsDalOmxbpbmCbd63nt/Ly46UtwK2
lwvO79FICVBB7ksDKxxTMsxruu/QZJe9hFGO8aNi6Xci8MxJO2lU741OXhpyIGmo
jOQot6nIe+N6drdW1HVv7yyoRKbGurf1R2uGlCBNfdl7SfEO7EgLSxG8LAFl/K4k
XsGcCvDi4NJJSj78DX8Uz4cP/UU/5xeFukKxS4uCadwr/RCtox6V+c10F7P74Wui
axsGzzJBEgTCAgErfsbjmssrP3DIBqeDRmy0cv0jPsIfr5jbvjrCPT/rW+Gkm/QM
OE7NRKkdnD+smE2eFIXiqesMbNtpZIKu98arK4/mluyLfHduZeMHHt3zb/ECklOg
1C3HDSa92/xfI2O/wK1dTZLDS8baHwQJyX+Xs+YGCjLrjDmh1nUVydX0bC3udblf
ZjbIOA4ffDzYSxLU44kkOZcCAwEAAQ==
-----END PUBLIC KEY-----`

func verifySignatureWithPublicKey(data, signature, publicKeyPEM []byte) error {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return fmt.Errorf("release signing public key is malformed")
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parsing release signing public key: %w", err)
	}

	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("release signing public key is not RSA")
	}

	digest := sha256.Sum256(data)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return fmt.Errorf("release signature verification failed: %w", err)
	}
	return nil
}
