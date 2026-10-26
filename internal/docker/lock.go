package docker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

const (
	operationLockTTL   = time.Hour
	operationLockLabel = "oma.lock"
	lockOwnerLabel     = "oma.lock.owner"
	lockOperationLabel = "oma.lock.operation"
	lockAcquiredLabel  = "oma.lock.acquired"
)

var ErrOperationLocked = errors.New("another operation is in progress")

type OperationLock struct {
	namespace *Namespace
	name      string
}

func (n *Namespace) AcquireOperation(ctx context.Context, operation string) (*OperationLock, error) {
	if n == nil || n.client == nil {
		return nil, errors.New("namespace is not connected")
	}

	name := n.name + "-lock"
	owner := operationOwner()

	for range 2 {
		acquiredAt := time.Now().UTC()
		_, err := n.client.ContainerCreate(ctx, client.ContainerCreateOptions{
			Name: name,
			Config: &container.Config{
				Image: ProxyImage,
				Labels: map[string]string{
					operationLockLabel: "true",
					lockOwnerLabel:     owner,
					lockOperationLabel: operation,
					lockAcquiredLabel:  acquiredAt.Format(time.RFC3339Nano),
				},
			},
		})
		if err == nil {
			return &OperationLock{namespace: n, name: name}, nil
		}
		if !errdefs.IsConflict(err) {
			return nil, fmt.Errorf("acquiring operation lock: %w", err)
		}

		info, inspectErr := n.client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
		if inspectErr != nil {
			if errdefs.IsNotFound(inspectErr) {
				continue
			}
			return nil, fmt.Errorf("inspecting operation lock: %w", inspectErr)
		}

		existingOwner := info.Container.Config.Labels[lockOwnerLabel]
		existingOperation := info.Container.Config.Labels[lockOperationLabel]
		acquired, parseErr := time.Parse(time.RFC3339Nano, info.Container.Config.Labels[lockAcquiredLabel])
		if parseErr != nil || time.Since(acquired) < operationLockTTL {
			return nil, fmt.Errorf("%w: %s is held by %s", ErrOperationLocked, existingOperation, existingOwner)
		}

		slog.Warn("Reclaiming stale operation lock", "app", n.name, "operation", existingOperation, "owner", existingOwner)
		if _, removeErr := n.client.ContainerRemove(ctx, name, client.ContainerRemoveOptions{Force: true}); removeErr != nil && !errdefs.IsNotFound(removeErr) {
			return nil, fmt.Errorf("removing stale operation lock: %w", removeErr)
		}
	}

	return nil, ErrOperationLocked
}

func (l *OperationLock) Release(ctx context.Context) error {
	if l == nil || l.namespace == nil || l.namespace.client == nil {
		return nil
	}
	_, err := l.namespace.client.ContainerRemove(ctx, l.name, client.ContainerRemoveOptions{Force: true})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

func operationOwner() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "oma"
	}
	return hostname + ":" + strconv.Itoa(os.Getpid())
}

func (a *Application) acquireOperation(ctx context.Context, operation string) (*OperationLock, error) {
	return a.namespace.AcquireOperation(ctx, operation)
}

func releaseOperation(lock *OperationLock) {
	if lock == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := lock.Release(ctx); err != nil {
		slog.Error("Failed to release operation lock", "error", err)
	}
}
