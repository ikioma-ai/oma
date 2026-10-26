package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallScript_VerifiesChecksum(t *testing.T) {
	t.Run("matching checksum installs", func(t *testing.T) {
		f := newInstallFixture(t)
		binary := []byte("#!/bin/sh\necho \"background install\" >> \"$OMA_TEST_LOG\"\n")
		f.writeAsset(t, binary, sha256Hex(binary))

		code, output := f.run(t)
		require.Equal(t, 0, code, output)

		installed := filepath.Join(f.installDir, "oma")
		_, err := os.Stat(installed)
		require.NoError(t, err)
		assert.Contains(t, output, "Installed Oma")

		log, err := os.ReadFile(f.logPath)
		require.NoError(t, err)
		assert.Contains(t, string(log), "background install")
	})

	t.Run("mismatched checksum aborts", func(t *testing.T) {
		f := newInstallFixture(t)
		binary := []byte("#!/bin/sh\nexit 0\n")
		f.writeAsset(t, binary, strings.Repeat("0", 64))

		code, output := f.run(t)
		require.NotEqual(t, 0, code, output)
		assert.Contains(t, output, "Checksum verification failed")
		assertNoInstalledFile(t, f)
	})

	t.Run("missing checksum entry aborts", func(t *testing.T) {
		f := newInstallFixture(t)
		binary := []byte("#!/bin/sh\nexit 0\n")
		f.writeAsset(t, binary, sha256Hex(binary))
		require.NoError(t, os.WriteFile(f.checksumPath, []byte(strings.Repeat("a", 64)+"  other-binary\n"), 0o644))

		code, output := f.run(t)
		require.NotEqual(t, 0, code, output)
		assert.Contains(t, output, "Could not find a valid checksum")
		assertNoInstalledFile(t, f)
	})

	t.Run("duplicate checksum aborts", func(t *testing.T) {
		f := newInstallFixture(t)
		binary := []byte("#!/bin/sh\nexit 0\n")
		f.writeAsset(t, binary, sha256Hex(binary))
		data := sha256Hex(binary) + "  oma-linux-amd64\n" + sha256Hex(binary) + "  oma-linux-amd64\n"
		require.NoError(t, os.WriteFile(f.checksumPath, []byte(data), 0o644))

		code, output := f.run(t)
		require.NotEqual(t, 0, code, output)
		assert.Contains(t, output, "Could not find a valid checksum")
		assertNoInstalledFile(t, f)
	})

	t.Run("malformed checksum aborts", func(t *testing.T) {
		f := newInstallFixture(t)
		binary := []byte("#!/bin/sh\nexit 0\n")
		f.writeAsset(t, binary, sha256Hex(binary))
		require.NoError(t, os.WriteFile(f.checksumPath, []byte("not-a-digest  oma-linux-amd64\n"), 0o644))

		code, output := f.run(t)
		require.NotEqual(t, 0, code, output)
		assert.Contains(t, output, "Could not find a valid checksum")
		assertNoInstalledFile(t, f)
	})

	t.Run("missing checksum asset aborts", func(t *testing.T) {
		f := newInstallFixture(t)
		binary := []byte("#!/bin/sh\nexit 0\n")
		f.writeAsset(t, binary, sha256Hex(binary))
		require.NoError(t, os.Remove(f.checksumPath))
		f.writeRelease(t, false)

		code, output := f.run(t)
		require.NotEqual(t, 0, code, output)
		assert.Contains(t, output, "Release does not publish checksums.txt")
		assertNoInstalledFile(t, f)
	})
}

// Helpers

type installFixture struct {
	dir          string
	binDir       string
	installDir   string
	fixturesDir  string
	checksumPath string
	logPath      string
}

func newInstallFixture(t *testing.T) *installFixture {
	t.Helper()

	dir := t.TempDir()
	f := &installFixture{
		dir:          dir,
		binDir:       filepath.Join(dir, "bin"),
		installDir:   filepath.Join(dir, "install"),
		fixturesDir:  filepath.Join(dir, "fixtures"),
		checksumPath: filepath.Join(dir, "fixtures", "checksums.txt"),
		logPath:      filepath.Join(dir, "background.log"),
	}
	require.NoError(t, os.MkdirAll(f.binDir, 0o755))
	require.NoError(t, os.MkdirAll(f.installDir, 0o755))
	require.NoError(t, os.MkdirAll(f.fixturesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(f.fixturesDir, "binary"), []byte("#!/bin/sh\nexit 0\n"), 0o644))

	f.writeExecutable(t, "uname", `#!/bin/sh
case "$1" in
-s) echo Linux ;;
-m) echo x86_64 ;;
*) exit 1 ;;
esac
`)
	f.writeExecutable(t, "docker", "#!/bin/sh\nexit 0\n")
	f.writeExecutable(t, "sudo", "#!/bin/sh\nexec \"$@\"\n")
	f.writeExecutable(t, "curl", `#!/bin/sh
out=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
  -o) out="$2"; shift 2 ;;
  -H|--header) shift 2 ;;
  -*) shift ;;
  *) url="$1"; shift ;;
  esac
done
case "$url" in
*/releases/latest) body="$FIXTURES/release.json" ;;
*/assets/binary) body="$FIXTURES/binary" ;;
*/assets/checksums) body="$FIXTURES/checksums.txt" ;;
*) exit 22 ;;
esac
if [ "$out" = "-" ] || [ -z "$out" ]; then
  cat "$body"
else
  cat "$body" > "$out"
fi
`)

	f.writeRelease(t, true)
	return f
}

func (f *installFixture) writeAsset(t *testing.T, binary []byte, checksum string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.fixturesDir, "binary"), binary, 0o644))
	require.NoError(t, os.WriteFile(f.checksumPath, []byte(fmt.Sprintf("%s  oma-linux-amd64\n", checksum)), 0o644))
}

func (f *installFixture) writeRelease(t *testing.T, withChecksums bool) {
	t.Helper()
	checksumAsset := ""
	if withChecksums {
		checksumAsset = `,
    {"url": "https://api.github.com/repos/ikioma-ai/oma/releases/assets/checksums", "name": "checksums.txt"}`
	}
	release := fmt.Sprintf(`{
  "tag_name": "v9.9.9",
  "assets": [
    {"url": "https://api.github.com/repos/ikioma-ai/oma/releases/assets/binary", "name": "oma-linux-amd64"}%s
  ]
}`, checksumAsset)
	require.NoError(t, os.WriteFile(filepath.Join(f.fixturesDir, "release.json"), []byte(release), 0o644))
}

func (f *installFixture) run(t *testing.T) (int, string) {
	t.Helper()

	var script bytes.Buffer
	require.NoError(t, testTemplate().ExecuteTemplate(&script, "install.sh", InstallScriptArgs{}))
	scriptPath := filepath.Join(f.dir, "install.sh")
	require.NoError(t, os.WriteFile(scriptPath, script.Bytes(), 0o755))

	cmd := exec.Command("sh", scriptPath)
	cmd.Env = append(os.Environ(),
		"PATH="+f.binDir+":/usr/bin:/bin",
		"OMA_INSTALL_DIR="+f.installDir,
		"OMA_INTERACTIVE=false",
		"OMA_TEST_LOG="+f.logPath,
		"FIXTURES="+f.fixturesDir,
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(output)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), string(output)
	}
	t.Fatalf("running installer: %v", err)
	return -1, ""
}

func (f *installFixture) writeExecutable(t *testing.T, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.binDir, name), []byte(content), 0o755))
}

func assertNoInstalledFile(t *testing.T, f *installFixture) {
	t.Helper()
	_, err := os.Stat(filepath.Join(f.installDir, "oma"))
	assert.True(t, os.IsNotExist(err))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
