package main

import (
	"embed"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/distribution/reference"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed apps.json
var appsJSON []byte

type InstallScriptArgs struct {
	InstallRef string
}

type CatalogApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Image       string `json:"image"`
	Hostname    string `json:"hostname"`
}

type Catalog struct {
	Apps []CatalogApp `json:"apps"`

	byID map[string]CatalogApp
}

var appIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func (c Catalog) Get(id string) (CatalogApp, bool) {
	app, ok := c.byID[id]
	return app, ok
}

func loadCatalog() (Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(appsJSON, &catalog); err != nil {
		return Catalog{}, err
	}

	catalog.byID = make(map[string]CatalogApp, len(catalog.Apps))
	for _, app := range catalog.Apps {
		if !appIDPattern.MatchString(app.ID) {
			return Catalog{}, &catalogError{id: app.ID, reason: "invalid app id"}
		}
		if _, exists := catalog.byID[app.ID]; exists {
			return Catalog{}, &catalogError{id: app.ID, reason: "duplicate app id"}
		}
		if _, err := reference.ParseNormalizedNamed(app.Image); err != nil {
			return Catalog{}, &catalogError{id: app.ID, reason: "invalid image reference"}
		}
		catalog.byID[app.ID] = app
	}

	return catalog, nil
}

type catalogError struct {
	id     string
	reason string
}

func (e *catalogError) Error() string {
	return "catalog app " + e.id + ": " + e.reason
}

func withLogging(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rl := &responseLogger{ResponseWriter: w, status: http.StatusOK}

		h.ServeHTTP(rl, r)

		duration := time.Since(start)
		clientIP := r.Header.Get("X-Forwarded-For")
		if clientIP == "" {
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				clientIP = host
			} else {
				clientIP = r.RemoteAddr
			}
		} else {
			clientIP = strings.Split(clientIP, ",")[0]
			clientIP = strings.TrimSpace(clientIP)
		}

		slog.Info("request",
			"ip", clientIP,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rl.status,
			"size", rl.size,
			"duration", duration,
		)
	}
}

type responseLogger struct {
	http.ResponseWriter
	status int
	size   int
}

func (rl *responseLogger) WriteHeader(status int) {
	rl.status = status
	rl.ResponseWriter.WriteHeader(status)
}

func (rl *responseLogger) Write(b []byte) (int, error) {
	n, err := rl.ResponseWriter.Write(b)
	rl.size += n
	return n, err
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func newAppsHandler(catalog Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(catalog); err != nil {
			slog.Error("Failed to encode catalog", "error", err)
		}
	}
}

func newInstallScriptHandler(template *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		imageRef := r.PathValue("image")
		if imageRef != "" {
			if _, err := reference.ParseNormalizedNamed(imageRef); err != nil {
				http.Error(w, "invalid image reference", http.StatusBadRequest)
				return
			}
		}

		renderInstallScript(w, template, InstallScriptArgs{InstallRef: imageRef})
	}
}

func newAppInstallScriptHandler(template *template.Template, catalog Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !appIDPattern.MatchString(id) {
			http.Error(w, "invalid app id", http.StatusBadRequest)
			return
		}

		app, ok := catalog.Get(id)
		if !ok {
			http.Error(w, "app not found", http.StatusNotFound)
			return
		}

		renderInstallScript(w, template, InstallScriptArgs{InstallRef: app.ID})
	}
}

func renderInstallScript(w http.ResponseWriter, template *template.Template, args InstallScriptArgs) {
	w.Header().Set("Content-Type", "text/x-shellscript")
	if err := template.ExecuteTemplate(w, "install.sh", args); err != nil {
		slog.Error("Failed to execute template", "error", err)
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	template, err := template.ParseFS(templateFS, "templates/*")
	if err != nil {
		panic(err)
	}

	catalog, err := loadCatalog()
	if err != nil {
		panic(err)
	}

	http.HandleFunc("GET /up", healthHandler)
	http.HandleFunc("GET /apps.json", withLogging(newAppsHandler(catalog)))
	http.HandleFunc("GET /apps/{id}", withLogging(newAppInstallScriptHandler(template, catalog)))
	http.HandleFunc("GET /{image...}", withLogging(newInstallScriptHandler(template)))

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "80"
	}

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
