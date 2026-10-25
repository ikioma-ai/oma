package catalog

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed apps/*.yaml
var embeddedFS embed.FS

type Source string

const (
	SourceBuiltin Source = "builtin"
	SourceUser    Source = "user"
)

type Entry struct {
	Manifest
	Source Source
	Path   string
}

type Issue struct {
	Path string
	Err  error
}

func (i Issue) Error() string {
	if i.Path == "" {
		return i.Err.Error()
	}
	return fmt.Sprintf("%s: %v", i.Path, i.Err)
}

func (i Issue) Unwrap() error {
	return i.Err
}

type Store struct {
	entries []Entry
	byID    map[string]Entry
	issues  []Issue
}

type Option func(*loadConfig)

type loadConfig struct {
	userDir    string
	userDirSet bool
}

func WithUserDir(dir string) Option {
	return func(c *loadConfig) {
		c.userDir = dir
		c.userDirSet = true
	}
}

func Load(opts ...Option) (*Store, error) {
	var config loadConfig
	for _, opt := range opts {
		opt(&config)
	}

	userDir := config.userDir
	if !config.userDirSet {
		var err error
		userDir, err = UserDir()
		if err != nil {
			return nil, fmt.Errorf("resolving user catalog directory: %w", err)
		}
	}

	builtins, err := loadEmbedded()
	if err != nil {
		return nil, err
	}

	byID := make(map[string]Entry, len(builtins))
	for _, entry := range builtins {
		byID[entry.ID] = entry
	}

	users, issues, err := loadUser(userDir)
	if err != nil {
		return nil, err
	}

	userIssues, validUsers := rejectDuplicateIDs(users)
	issues = append(issues, userIssues...)

	for _, entry := range validUsers {
		if entry.Disabled {
			delete(byID, entry.ID)
			continue
		}
		byID[entry.ID] = entry
	}

	entries := make([]Entry, 0, len(byID))
	for _, entry := range byID {
		entries = append(entries, entry)
	}
	sortEntries(entries)

	return &Store{
		entries: entries,
		byID:    byID,
		issues:  issues,
	}, nil
}

func Empty() *Store {
	return &Store{byID: make(map[string]Entry)}
}

func (s *Store) List() []Entry {
	if s == nil {
		return nil
	}
	entries := make([]Entry, len(s.entries))
	copy(entries, s.entries)
	return entries
}

func (s *Store) Get(id string) (Entry, bool) {
	if s == nil {
		return Entry{}, false
	}
	entry, ok := s.byID[id]
	return entry, ok
}

func (s *Store) Issues() []Issue {
	if s == nil {
		return nil
	}
	issues := make([]Issue, len(s.issues))
	copy(issues, s.issues)
	return issues
}

func UserDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "oma", "apps"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "oma", "apps"), nil
}

// Private

func loadEmbedded() ([]Entry, error) {
	files, err := fs.ReadDir(embeddedFS, "apps")
	if err != nil {
		return nil, fmt.Errorf("reading embedded catalog: %w", err)
	}

	entries := make([]Entry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !isManifestFile(file.Name()) {
			continue
		}

		name := path.Join("apps", file.Name())
		data, err := embeddedFS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("reading embedded catalog %s: %w", file.Name(), err)
		}

		manifest, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("embedded catalog %s: %w", file.Name(), err)
		}

		entries = append(entries, Entry{
			Manifest: manifest,
			Source:   SourceBuiltin,
			Path:     "embedded:" + name,
		})
	}

	return entries, nil
}

func loadUser(dir string) ([]Entry, []Issue, error) {
	files, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading user catalog %s: %w", dir, err)
	}

	var entries []Entry
	var issues []Issue

	for _, file := range files {
		if file.IsDir() || !isManifestFile(file.Name()) {
			continue
		}

		filePath := filepath.Join(dir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			issues = append(issues, Issue{Path: filePath, Err: err})
			continue
		}

		manifest, err := Parse(data)
		if err != nil {
			issues = append(issues, Issue{Path: filePath, Err: err})
			continue
		}

		entries = append(entries, Entry{
			Manifest: manifest,
			Source:   SourceUser,
			Path:     filePath,
		})
	}

	return entries, issues, nil
}

func rejectDuplicateIDs(entries []Entry) ([]Issue, []Entry) {
	byID := make(map[string][]Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = append(byID[entry.ID], entry)
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var issues []Issue
	for _, id := range ids {
		duplicates := byID[id]
		if len(duplicates) < 2 {
			continue
		}
		for _, entry := range duplicates {
			issues = append(issues, Issue{
				Path: entry.Path,
				Err:  fmt.Errorf("duplicate catalog id %q", id),
			})
		}
	}

	valid := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if len(byID[entry.ID]) == 1 {
			valid = append(valid, entry)
		}
	}

	return issues, valid
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]

		if compared := strings.Compare(strings.ToLower(left.Category), strings.ToLower(right.Category)); compared != 0 {
			return compared < 0
		}
		if compared := strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name)); compared != 0 {
			return compared < 0
		}
		return left.ID < right.ID
	})
}

// Helpers

func isManifestFile(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}
