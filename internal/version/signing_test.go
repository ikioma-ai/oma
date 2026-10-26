package version

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyReleaseSignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKey})

	data := []byte("checksums")
	digest := sha256.Sum256(data)
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)

	require.NoError(t, verifySignatureWithPublicKey(data, signature, publicKeyPEM))
	require.Error(t, verifySignatureWithPublicKey([]byte("tampered"), signature, publicKeyPEM))

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	otherPublicKey, err := x509.MarshalPKIXPublicKey(&otherKey.PublicKey)
	require.NoError(t, err)
	otherPublicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: otherPublicKey})
	require.Error(t, verifySignatureWithPublicKey(data, signature, otherPublicKeyPEM))

	assert.Error(t, verifySignatureWithPublicKey(data, signature, []byte("not a key")))
}
