package docker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

type DescribedError interface {
	error
	Description() string
}

var (
	ErrProxyPortInUse = &describedError{
		msg:         "proxy port conflict",
		description: "Something else is using the web ports on this machine. You'll need to stop that service, and then try deploying again.",
	}
	ErrAppNotStarted = &describedError{
		msg:         "application did not start",
		description: "The application did not start within the time limit. Check the application logs for errors.",
	}
	ErrDockerPermissionDenied = &describedError{
		msg:         "permission denied connecting to Docker",
		description: "Permission denied when connecting to the Docker socket. Run with `sudo`, or add yourself to the `docker` group.",
	}
	ErrDockerNotRunning = &describedError{
		msg:         "cannot connect to Docker",
		description: "Could not connect to Docker. Make sure Docker is installed and the Docker daemon is running.",
	}
)

// ClassifyConnectionError maps a Docker connection failure to the shared
// permission-denied or daemon-not-running errors.
func ClassifyConnectionError(err error) error {
	return connectionError(err)
}

func ErrorMessage(err error) string {
	if de, ok := errors.AsType[DescribedError](err); ok {
		return de.Description()
	}
	return err.Error()
}

// Private

type describedError struct {
	msg         string
	description string
}

func (e *describedError) Error() string       { return e.msg }
func (e *describedError) Description() string { return e.description }

// Helpers

func wrapPullError(err error) error {
	if pullErrorMayNeedAuth(err) {
		return fmt.Errorf("%w: %w: %w", ErrPullMaybeUnauthorized, ErrPullFailed, err)
	}
	return fmt.Errorf("%w: %w", ErrPullFailed, err)
}

// pullErrorMayNeedAuth reports whether a pull failure looks like a missing or
// unauthorized image. Registries conflate the two cases (some return 404 for
// private images to hide their existence), so this is deliberately broad.
func pullErrorMayNeedAuth(err error) bool {
	if errdefs.IsUnauthorized(err) || errdefs.IsPermissionDenied(err) || errdefs.IsNotFound(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"unauthorized",
		"authentication required",
		"denied",
		"no basic auth credentials",
		"forbidden",
		"not found",
		"manifest unknown",
		"name unknown",
		"repository does not exist",
	} {
		if strings.Contains(msg, fragment) {
			return true
		}
	}
	return false
}

func isPortConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "bind: address already in use") ||
		strings.Contains(msg, "port is already allocated")
}

func connectionError(err error) error {
	switch {
	case err == nil:
		return nil
	case client.IsErrConnectionFailed(err):
		if strings.Contains(err.Error(), "permission denied") {
			return fmt.Errorf("%w: %w", ErrDockerPermissionDenied, err)
		}
		return fmt.Errorf("%w: %w", ErrDockerNotRunning, err)
	default:
		return err
	}
}
