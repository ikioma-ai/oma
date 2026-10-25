package catalogapply

import (
	"fmt"
	"strconv"

	"github.com/ikioma-ai/oma/internal/catalog"
	"github.com/ikioma-ai/oma/internal/docker"
)

type Change struct {
	Field string
	Old   string
	New   string
}

func Diff(current docker.ApplicationSettings, entry catalog.Entry) []Change {
	var changes []Change

	add := func(field, old, next string) {
		if old != next {
			changes = append(changes, Change{Field: field, Old: old, New: next})
		}
	}

	add("Image", current.Image, entry.Image)
	add("Service port", strconv.Itoa(effectivePort(current)), strconv.Itoa(entry.Web.Port))
	add("Health path", effectiveHealthPath(current), entry.Web.HealthPath)
	add("Health port", strconv.Itoa(effectiveHealthPort(current)), strconv.Itoa(entry.Web.HealthPort))
	add("Health timeout", effectiveHealthTimeout(current), entry.Web.HealthTimeout)
	add("TLS mode", effectiveTLS(current), entry.Web.TLS)
	add("Verification", enabledLabel(current.VerifyEnabled()), enabledLabel(entry.VerifyEnabled()))
	add("CPU limit", strconv.Itoa(current.Resources.CPUs), strconv.Itoa(entry.Resources.CPUs))
	add("Memory limit", strconv.Itoa(current.Resources.MemoryMB), strconv.Itoa(entry.Resources.MemoryMB))

	existing := make(map[string]struct{})
	for _, mount := range effectiveStorage(current) {
		existing[mount.Path] = struct{}{}
	}
	for _, mount := range entry.Storage {
		if _, ok := existing[mount.Path]; ok {
			continue
		}
		changes = append(changes, Change{Field: "Mount target", Old: "—", New: mount.Path})
	}

	return changes
}

func Build(current docker.ApplicationSettings, entry catalog.Entry) docker.ApplicationSettings {
	next := current

	next.CatalogID = entry.ID
	next.CatalogSource = string(entry.Source)
	next.Image = entry.Image
	next.ServicePort = entry.Web.Port
	next.HealthCheckPath = entry.Web.HealthPath
	next.HealthCheckPort = entry.Web.HealthPort
	next.HealthCheckTimeout = entry.Web.HealthTimeout
	next.TLSMode = entry.Web.TLS
	next.DisableTLS = entry.Web.TLS == "off"

	if entry.Web.Verify != nil {
		verify := *entry.Web.Verify
		next.Verify = &verify
	} else {
		next.Verify = nil
	}

	next.Resources = docker.ContainerResources{
		CPUs:     entry.Resources.CPUs,
		MemoryMB: entry.Resources.MemoryMB,
	}

	if len(entry.Storage) > 0 {
		next.Storage = mergeStorage(effectiveStorage(current), entry.Storage)
	}

	return next
}

func ImageChanged(current docker.ApplicationSettings, entry catalog.Entry) bool {
	return current.Image != entry.Image
}

// Private

func effectivePort(settings docker.ApplicationSettings) int {
	if settings.ServicePort > 0 {
		return settings.ServicePort
	}
	return docker.DefaultServicePort
}

func effectiveHealthPath(settings docker.ApplicationSettings) string {
	if settings.HealthCheckPath != "" {
		return settings.HealthCheckPath
	}
	return docker.HealthCheckPath
}

func effectiveHealthPort(settings docker.ApplicationSettings) int {
	if settings.HealthCheckPort > 0 {
		return settings.HealthCheckPort
	}
	return effectivePort(settings)
}

func effectiveHealthTimeout(settings docker.ApplicationSettings) string {
	if settings.HealthCheckTimeout != "" {
		return settings.HealthCheckTimeout
	}
	return docker.DefaultHealthCheckTimeout
}

func effectiveTLS(settings docker.ApplicationSettings) string {
	if settings.TLSMode != "" {
		return settings.TLSMode
	}
	if settings.DisableTLS {
		return "off"
	}
	return "auto"
}

func effectiveStorage(settings docker.ApplicationSettings) []docker.StorageMount {
	if len(settings.Storage) > 0 {
		mounts := make([]docker.StorageMount, len(settings.Storage))
		copy(mounts, settings.Storage)
		return mounts
	}

	mounts := make([]docker.StorageMount, len(docker.AppVolumeMountTargets))
	for i, target := range docker.AppVolumeMountTargets {
		mounts[i] = docker.StorageMount{
			Path:    target,
			Primary: i == 0,
			Backup:  i == 0,
		}
	}
	return mounts
}

func mergeStorage(existing []docker.StorageMount, additions []catalog.Storage) []docker.StorageMount {
	merged := make([]docker.StorageMount, len(existing))
	copy(merged, existing)

	seen := make(map[string]struct{}, len(existing))
	for _, mount := range existing {
		seen[mount.Path] = struct{}{}
	}

	for _, addition := range additions {
		if _, ok := seen[addition.Path]; ok {
			continue
		}
		merged = append(merged, docker.StorageMount{Path: addition.Path})
		seen[addition.Path] = struct{}{}
	}

	return merged
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// Helpers

func FormatChange(change Change) string {
	return fmt.Sprintf("%s: %s -> %s", change.Field, change.Old, change.New)
}
