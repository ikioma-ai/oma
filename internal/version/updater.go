package version

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ikioma-ai/oma/internal/brand"
)

const (
	githubReleasesURL          = brand.GitHubReleasesURL
	checksumAssetName          = "checksums.txt"
	checksumSignatureAssetName = "checksums.txt.sig"
	maxReleaseAssetSize        = 128 << 20
	httpTimeout                = 30 * time.Second
	updateTempFile             = "." + brand.Name + "-update-tmp"
)

type Updater struct {
	currentVersion   string
	apiURL           string
	client           *http.Client
	githubToken      string
	execPath         string
	signingPublicKey []byte
}

func NewUpdater() *Updater {
	u := newUpdater(Version, githubReleasesURL, &http.Client{Timeout: httpTimeout})
	u.githubToken = os.Getenv("GITHUB_TOKEN")
	return u
}

func newUpdater(currentVersion, apiURL string, client *http.Client) *Updater {
	return &Updater{
		currentVersion:   currentVersion,
		apiURL:           apiURL,
		client:           client,
		signingPublicKey: []byte(releaseSigningPublicKey),
	}
}

func (u *Updater) UpdateBinary() error {
	rel, err := u.fetchRelease()
	if err != nil {
		return err
	}

	if rel.TagName == u.currentVersion {
		fmt.Printf("You already have the latest version (%s)\n", u.currentVersion)
		return nil
	}

	assetName := fmt.Sprintf("%s-%s-%s", brand.Name, runtime.GOOS, runtime.GOARCH)
	downloadURL := assetURL(rel, assetName)
	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s", assetName)
	}

	checksumURL := assetURL(rel, checksumAssetName)
	if checksumURL == "" {
		return fmt.Errorf("release does not publish %s", checksumAssetName)
	}
	signatureURL := assetURL(rel, checksumSignatureAssetName)
	if signatureURL == "" {
		return fmt.Errorf("release does not publish %s", checksumSignatureAssetName)
	}

	checksumData, err := u.fetchAssetBytes(checksumURL)
	if err != nil {
		return fmt.Errorf("fetching checksums: %w", err)
	}
	signature, err := u.fetchAssetBytes(signatureURL)
	if err != nil {
		return fmt.Errorf("fetching checksum signature: %w", err)
	}
	if err := u.verifyChecksumSignature(checksumData, signature); err != nil {
		return err
	}
	expectedChecksum, err := checksumForAsset(checksumData, assetName)
	if err != nil {
		return err
	}

	fmt.Printf("Updating %s from %s to %s...\n", brand.DisplayName, u.currentVersion, rel.TagName)

	execPath := u.execPath
	if execPath == "" {
		execPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("finding executable path: %w", err)
		}
	}

	tmpFile := filepath.Join(filepath.Dir(execPath), updateTempFile)
	if err := u.downloadBinary(downloadURL, tmpFile); err != nil {
		return err
	}

	if err := verifyFileChecksum(tmpFile, expectedChecksum); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("verifying %s: %w", assetName, err)
	}

	if err := u.replaceBinary(execPath, tmpFile); err != nil {
		return err
	}

	fmt.Println("Update complete.")
	return nil
}

// Private

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func assetURL(rel *release, name string) string {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

func (u *Updater) fetchAssetBytes(url string) ([]byte, error) {
	return u.fetchAssetBytesLimit(url, 1<<20)
}

func (u *Updater) fetchAssetBytesLimit(url string, limit int64) ([]byte, error) {
	resp, err := u.getAsset(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (u *Updater) DownloadTargetBinary(goos, goarch string) ([]byte, string, error) {
	rel, err := u.fetchRelease()
	if err != nil {
		return nil, "", err
	}

	assetName := fmt.Sprintf("%s-%s-%s", brand.Name, goos, goarch)
	binaryURL := assetURL(rel, assetName)
	if binaryURL == "" {
		return nil, "", fmt.Errorf("no release asset found for %s", assetName)
	}
	checksumURL := assetURL(rel, checksumAssetName)
	if checksumURL == "" {
		return nil, "", fmt.Errorf("release does not publish %s", checksumAssetName)
	}
	signatureURL := assetURL(rel, checksumSignatureAssetName)
	if signatureURL == "" {
		return nil, "", fmt.Errorf("release does not publish %s", checksumSignatureAssetName)
	}

	checksumData, err := u.fetchAssetBytes(checksumURL)
	if err != nil {
		return nil, "", fmt.Errorf("fetching checksums: %w", err)
	}
	signature, err := u.fetchAssetBytes(signatureURL)
	if err != nil {
		return nil, "", fmt.Errorf("fetching checksum signature: %w", err)
	}
	if err := u.verifyChecksumSignature(checksumData, signature); err != nil {
		return nil, "", err
	}
	expected, err := checksumForAsset(checksumData, assetName)
	if err != nil {
		return nil, "", err
	}

	binary, err := u.fetchAssetBytesLimit(binaryURL, maxReleaseAssetSize)
	if err != nil {
		return nil, "", fmt.Errorf("downloading %s: %w", assetName, err)
	}
	digest := sha256.Sum256(binary)
	if hex.EncodeToString(digest[:]) != expected {
		return nil, "", fmt.Errorf("checksum mismatch for %s", assetName)
	}

	return binary, rel.TagName, nil
}

func (u *Updater) verifyChecksumSignature(data, signature []byte) error {
	if len(u.signingPublicKey) == 0 {
		return fmt.Errorf("release signing public key is not configured")
	}
	return verifySignatureWithPublicKey(data, signature, u.signingPublicKey)
}

func checksumForAsset(data []byte, assetName string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var expected string
	found := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return "", fmt.Errorf("malformed checksum file")
		}

		digest := fields[0]
		name := strings.TrimPrefix(fields[1], "*")
		if len(digest) != sha256.Size*2 || !isLowerHex(digest) {
			return "", fmt.Errorf("malformed checksum for %s", name)
		}
		if name != assetName {
			continue
		}
		if found {
			return "", fmt.Errorf("duplicate checksum entry for %s", assetName)
		}

		expected = digest
		found = true
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading checksums: %w", err)
	}
	if !found {
		return "", fmt.Errorf("checksum for %s not found", assetName)
	}

	return expected, nil
}

func isLowerHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func verifyFileChecksum(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening downloaded file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hashing downloaded file: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func (u *Updater) get(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if u.githubToken != "" {
		req.Header.Set("Authorization", "token "+u.githubToken)
	}
	return u.client.Do(req)
}

func (u *Updater) fetchRelease() (*release, error) {
	resp, err := u.get(u.apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching release: unexpected status %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}

	return &rel, nil
}

func (u *Updater) getAsset(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	if u.githubToken != "" {
		req.Header.Set("Authorization", "token "+u.githubToken)
	}
	return u.client.Do(req)
}

func (u *Updater) downloadBinary(url, dest string) error {
	resp, err := u.getAsset(url)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading binary: unexpected status %d", resp.StatusCode)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer f.Close()

	pr := &progressReader{r: resp.Body, total: resp.ContentLength}
	if _, err := io.Copy(f, pr); err != nil {
		fmt.Println()
		os.Remove(dest)
		return fmt.Errorf("writing binary: %w", err)
	}
	fmt.Println()

	return nil
}

type progressReader struct {
	r     io.Reader
	total int64 // -1 if unknown
	read  int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.total > 0 {
		fmt.Printf("\r  %.1f / %.1f MB (%d%%)", float64(p.read)/1e6, float64(p.total)/1e6, p.read*100/p.total)
	} else {
		fmt.Printf("\r  %.1f MB", float64(p.read)/1e6)
	}
	return n, err
}

func (u *Updater) replaceBinary(execPath, newPath string) error {
	oldPath := execPath + ".old"

	if err := os.Rename(execPath, oldPath); err != nil {
		return fmt.Errorf("backing up current binary: %w", err)
	}

	if err := os.Rename(newPath, execPath); err != nil {
		// Attempt to restore from backup
		os.Rename(oldPath, execPath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	os.Remove(oldPath)
	return nil
}
