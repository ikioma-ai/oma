package target

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
)

type SSHCommand struct {
	Destination string
}

func (c SSHCommand) Run(ctx context.Context, script string) (string, string, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrSSHNotFound, err)
	}

	cmd := exec.CommandContext(ctx, sshPath,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		c.Destination,
		"sh -s",
	)
	cmd.Stdin = strings.NewReader(script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("SSH command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), stderr.String(), nil
}

func (c SSHCommand) RunInput(ctx context.Context, command string, input io.Reader) (string, string, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrSSHNotFound, err)
	}

	cmd := exec.CommandContext(ctx, sshPath,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		c.Destination,
		command,
	)
	cmd.Stdin = input

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("SSH command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), stderr.String(), nil
}

type SetupReport struct {
	OS           string
	Docker       string
	GroupUpdated bool
	Installed    bool
	DNS          []string
	Ports        map[int]bool
}

func Setup(ctx context.Context, target Target, emit func(string)) (SetupReport, error) {
	if target.Local {
		return SetupReport{}, fmt.Errorf("target setup is only for remote targets")
	}
	if !validSSHDestination(target.SSH) {
		return SetupReport{}, fmt.Errorf("target %q has an invalid SSH destination", target.Name)
	}
	if !pathpkg.IsAbs(target.DockerSocket) {
		return SetupReport{}, fmt.Errorf("target %q requires an absolute Docker socket path", target.Name)
	}

	report := SetupReport{Ports: make(map[int]bool)}
	ssh := SSHCommand{Destination: target.SSH}

	osName, stderr, err := ssh.Run(ctx, "uname -s")
	if err != nil {
		return report, setupError("detecting remote OS", stderr, err)
	}
	report.OS = strings.TrimSpace(osName)
	if report.OS != "Linux" {
		return report, fmt.Errorf("automated setup supports Linux targets only, got %q", report.OS)
	}
	emit("SSH: ok")
	emit("OS: " + report.OS)

	user, stderr, err := ssh.Run(ctx, "id -un")
	if err != nil {
		return report, setupError("reading remote user", stderr, err)
	}
	user = strings.TrimSpace(user)

	root := false
	if uid, _, err := ssh.Run(ctx, "id -u"); err == nil {
		root = strings.TrimSpace(uid) == "0"
	}

	if !root {
		if _, _, err := ssh.Run(ctx, "sudo -n true 2>/dev/null"); err != nil {
			return report, fmt.Errorf("passwordless sudo or root is required for automated setup\n\nRun manually on the target:\n%s", manualSetupCommands(user))
		}
	}

	dockerReady := false
	if _, _, err := ssh.Run(ctx, "docker info >/dev/null"); err == nil {
		dockerReady = true
	}

	if !dockerReady {
		if _, _, err := ssh.Run(ctx, "command -v docker >/dev/null 2>&1"); err != nil {
			emit("Docker: installing")
			installScript := "set -e\ncurl -fsSL https://get.docker.com | sh\n"
			if !root {
				installScript = "set -e\nsudo -n sh -c 'curl -fsSL https://get.docker.com | sh'\n"
			}
			if _, stderr, err := ssh.Run(ctx, installScript); err != nil {
				return report, setupError("installing Docker", stderr, err)
			}
			report.Installed = true
		}

		if _, stderr, err := ssh.Run(ctx, "docker info >/dev/null"); err != nil {
			if !strings.Contains(strings.ToLower(stderr), "permission denied") {
				return report, setupError("checking Docker after installation", stderr, err)
			}
			if root {
				return report, fmt.Errorf("docker is installed but the Docker socket is not accessible")
			}

			emit("Docker group: updating")
			if _, stderr, err := ssh.Run(ctx, "sudo -n usermod -aG docker "+shellQuote(user)); err != nil {
				return report, setupError("adding the user to the Docker group", stderr, err)
			}
			report.GroupUpdated = true
		}
	}

	version, stderr, err := ssh.Run(ctx, "docker info --format '{{.ServerVersion}}'")
	if err != nil {
		return report, setupError("verifying Docker access", stderr, err)
	}
	report.Docker = strings.TrimSpace(version)
	if report.Docker == "" {
		report.Docker = "ok"
	}
	emit("Docker: ok (" + report.Docker + ")")

	report.DNS = lookupTargetHost(ctx, target.SSH)
	if len(report.DNS) > 0 {
		emit("DNS: " + strings.Join(report.DNS, ", "))
	} else {
		emit("DNS: unresolved")
	}

	for _, port := range []int{80, 443} {
		report.Ports[port] = probePort(ctx, target.SSH, port)
		state := "closed"
		if report.Ports[port] {
			state = "open"
		}
		emit(fmt.Sprintf("Port %d: %s", port, state))
	}
	emit("Firewall and DNS were not modified; ensure ports 80/443 are allowed before deploying.")

	return report, nil
}

func manualSetupCommands(user string) string {
	return fmt.Sprintf("curl -fsSL https://get.docker.com | sudo sh\nsudo usermod -aG docker %s", shellQuote(user))
}

// Helpers

func setupError(action, stderr string, err error) error {
	if strings.TrimSpace(stderr) == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, strings.TrimSpace(stderr))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func lookupTargetHost(ctx context.Context, destination string) []string {
	host := sshHost(destination)
	if host == "" {
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var resolver net.Resolver
	addresses, err := resolver.LookupHost(lookupCtx, host)
	if err != nil {
		return nil
	}
	return addresses
}

func probePort(ctx context.Context, destination string, port int) bool {
	host := sshHost(destination)
	if host == "" {
		return false
	}
	dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func sshHost(destination string) string {
	host := destination
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if strings.HasPrefix(host, "[") {
		if end := strings.Index(host, "]"); end >= 0 {
			return host[1:end]
		}
	}
	if colon := strings.LastIndex(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	return host
}
