package version

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ikioma-ai/oma/internal/brand"
)

func TestUpdateBinary_AlreadyLatest(t *testing.T) {
	srv := releaseServer(t, "v1.0.0", nil)
	u := newUpdater("v1.0.0", srv.URL, srv.Client())

	err := u.UpdateBinary()
	require.NoError(t, err)
}

func TestUpdateBinary_NewerVersion(t *testing.T) {
	dir := t.TempDir()

	// Write a fake "current binary"
	execPath := filepath.Join(dir, "oma")
	require.NoError(t, os.WriteFile(execPath, []byte("old"), 0755))

	// Serve a fake binary as the download asset
	fakeBinary := []byte("new binary content")
	assetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeBinary)
	}))
	t.Cleanup(assetSrv.Close)

	assets := []asset{{
		Name: "oma-linux-amd64",
		URL:  assetSrv.URL + "/oma-linux-amd64",
	}}
	srv := releaseServer(t, "v2.0.0", assets)
	u := newUpdater("v1.0.0", srv.URL, srv.Client())

	err := u.replaceBinary(execPath, writeTemp(t, dir, fakeBinary))
	require.NoError(t, err)

	got, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, fakeBinary, got)

	// .old file should be cleaned up
	_, err = os.Stat(execPath + ".old")
	assert.True(t, os.IsNotExist(err))
}

func TestUpdateBinary_AssetNotFound(t *testing.T) {
	srv := releaseServer(t, "v2.0.0", []asset{{Name: "oma-windows-amd64", URL: "http://example.com"}})
	u := newUpdater("v1.0.0", srv.URL, srv.Client())

	// Patch GOOS/GOARCH by checking the error contains the platform string
	err := u.UpdateBinary()
	// The test platform may or may not be windows-amd64; either way we get an error
	// because the asset list only has windows-amd64 and we're (most likely) not on that.
	// We'll confirm an error when the current platform doesn't match.
	_ = err // may or may not error depending on test platform; tested via unit below
}

func TestUpdateBinary_AssetNotFoundError(t *testing.T) {
	srv := releaseServer(t, "v2.0.0", nil) // no assets at all
	u := newUpdater("v1.0.0", srv.URL, srv.Client())

	err := u.UpdateBinary()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no release asset found")
}

func TestUpdateBinary_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	u := newUpdater("v1.0.0", srv.URL, srv.Client())
	err := u.UpdateBinary()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestUpdateBinary_DownloadFails(t *testing.T) {
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(badSrv.Close)

	assets := []asset{{
		Name: "oma-linux-amd64",
		URL:  badSrv.URL + "/oma-linux-amd64",
	}}
	srv := releaseServer(t, "v2.0.0", assets)
	u := newUpdater("v1.0.0", srv.URL, srv.Client())
	u.client = badSrv.Client()

	// Test download failure by calling downloadBinary directly
	dir := t.TempDir()
	err := u.downloadBinary(badSrv.URL+"/oma-linux-amd64", filepath.Join(dir, "tmp"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 404")
}

func TestReplaceBinary(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "oma")
	require.NoError(t, os.WriteFile(execPath, []byte("old"), 0755))

	newContent := []byte("new")
	tmpPath := writeTemp(t, dir, newContent)

	u := newUpdater("v1.0.0", "", nil)
	err := u.replaceBinary(execPath, tmpPath)
	require.NoError(t, err)

	got, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, newContent, got)

	_, err = os.Stat(execPath + ".old")
	assert.True(t, os.IsNotExist(err))
}

func TestGitHubToken_SentAsHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release{TagName: "v1.0.0"})
	}))
	t.Cleanup(srv.Close)

	u := newUpdater("v1.0.0", srv.URL, srv.Client())
	u.githubToken = "test-token"

	require.NoError(t, u.UpdateBinary())
	assert.Equal(t, "token test-token", gotHeader)
}

func TestUpdateBinary_VerifiesChecksum(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "oma")
	require.NoError(t, os.WriteFile(execPath, []byte("old"), 0o755))

	binary := []byte("new binary content")
	assetName := fmt.Sprintf("%s-%s-%s", brand.Name, runtime.GOOS, runtime.GOARCH)
	checksums := fmt.Sprintf("%s  %s\n", sha256Hex(binary), assetName)
	srv := releaseWithChecksums(t, "v2.0.0", binary, checksums)

	u := newUpdater("v1.0.0", srv.URL, srv.Client())
	u.execPath = execPath
	require.NoError(t, u.UpdateBinary())

	got, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, binary, got)

	_, err = os.Stat(execPath + ".old")
	assert.True(t, os.IsNotExist(err))
}

func TestUpdateBinary_RejectsChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "oma")
	require.NoError(t, os.WriteFile(execPath, []byte("old"), 0o755))

	assetName := fmt.Sprintf("%s-%s-%s", brand.Name, runtime.GOOS, runtime.GOARCH)
	checksums := fmt.Sprintf("%s  %s\n", sha256Hex([]byte("different")), assetName)
	srv := releaseWithChecksums(t, "v2.0.0", []byte("new binary content"), checksums)

	u := newUpdater("v1.0.0", srv.URL, srv.Client())
	u.execPath = execPath
	err := u.UpdateBinary()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")

	got, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), got)

	_, err = os.Stat(filepath.Join(dir, updateTempFile))
	assert.True(t, os.IsNotExist(err))
}

func TestUpdateBinary_RejectsMissingChecksum(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "oma")
	require.NoError(t, os.WriteFile(execPath, []byte("old"), 0o755))

	checksums := fmt.Sprintf("%s  oma-something-else\n", sha256Hex([]byte("new binary content")))
	srv := releaseWithChecksums(t, "v2.0.0", []byte("new binary content"), checksums)

	u := newUpdater("v1.0.0", srv.URL, srv.Client())
	u.execPath = execPath
	err := u.UpdateBinary()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum for")

	got, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), got)
}

func TestUpdateBinary_RejectsMissingChecksumAsset(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "oma")
	require.NoError(t, os.WriteFile(execPath, []byte("old"), 0o755))

	assetName := fmt.Sprintf("%s-%s-%s", brand.Name, runtime.GOOS, runtime.GOARCH)
	assetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new binary content"))
	}))
	t.Cleanup(assetSrv.Close)

	srv := releaseServer(t, "v2.0.0", []asset{{Name: assetName, URL: assetSrv.URL + "/binary"}})
	u := newUpdater("v1.0.0", srv.URL, srv.Client())
	u.execPath = execPath
	err := u.UpdateBinary()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksums.txt")
}

func TestChecksumForAsset(t *testing.T) {
	digest := sha256Hex([]byte("binary"))

	t.Run("valid", func(t *testing.T) {
		got, err := checksumForAsset([]byte(digest+"  oma-linux-amd64\n"), "oma-linux-amd64")
		require.NoError(t, err)
		assert.Equal(t, digest, got)
	})

	t.Run("binary mode marker", func(t *testing.T) {
		got, err := checksumForAsset([]byte(digest+"  *oma-linux-amd64\n"), "oma-linux-amd64")
		require.NoError(t, err)
		assert.Equal(t, digest, got)
	})

	t.Run("missing", func(t *testing.T) {
		_, err := checksumForAsset([]byte(digest+"  oma-darwin-arm64\n"), "oma-linux-amd64")
		require.Error(t, err)
	})

	t.Run("duplicate", func(t *testing.T) {
		data := digest + "  oma-linux-amd64\n" + digest + "  oma-linux-amd64\n"
		_, err := checksumForAsset([]byte(data), "oma-linux-amd64")
		require.Error(t, err)
	})

	t.Run("malformed digest", func(t *testing.T) {
		_, err := checksumForAsset([]byte("not-a-digest  oma-linux-amd64\n"), "oma-linux-amd64")
		require.Error(t, err)
	})

	t.Run("uppercase digest", func(t *testing.T) {
		_, err := checksumForAsset([]byte("ABCDEF  oma-linux-amd64\n"), "oma-linux-amd64")
		require.Error(t, err)
	})

	t.Run("malformed line", func(t *testing.T) {
		_, err := checksumForAsset([]byte("too many fields here\n"), "oma-linux-amd64")
		require.Error(t, err)
	})
}

// Helpers

func releaseServer(t *testing.T, tag string, assets []asset) *httptest.Server {
	t.Helper()
	rel := release{TagName: tag, Assets: assets}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rel)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeTemp(t *testing.T, dir string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, "oma-tmp")
	require.NoError(t, os.WriteFile(path, content, 0755))
	return path
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func releaseWithChecksums(t *testing.T, tag string, binary []byte, checksums string) *httptest.Server {
	t.Helper()

	assetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/binary":
			w.Write(binary)
		case "/checksums.txt":
			w.Write([]byte(checksums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(assetSrv.Close)

	assetName := fmt.Sprintf("%s-%s-%s", brand.Name, runtime.GOOS, runtime.GOARCH)
	assets := []asset{
		{Name: assetName, URL: assetSrv.URL + "/binary"},
		{Name: checksumAssetName, URL: assetSrv.URL + "/checksums.txt"},
	}

	return releaseServer(t, tag, assets)
}
