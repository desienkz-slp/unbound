package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"netora-dns/pkg/configengine"
)

// Supervisor controls the Unbound process lifecycle.
type Supervisor interface {
	Reload(ctx context.Context) error
	Restart(ctx context.Context) error
	IsRunning(ctx context.Context) (bool, error)
	Checkconf(ctx context.Context, confPath string) (string, error)
}

// SystemdSupervisor controls Unbound via systemctl and unbound-control.
type SystemdSupervisor struct {
	runner configengine.CommandRunner
}

// NewSystemdSupervisor initializes a supervisor using the provided runner.
func NewSystemdSupervisor(runner configengine.CommandRunner) *SystemdSupervisor {
	if runner == nil {
		runner = &configengine.LocalCommandRunner{}
	}
	return &SystemdSupervisor{runner: runner}
}

// Reload reloads configuration via unbound-control if available, otherwise systemctl reload.
func (s *SystemdSupervisor) Reload(ctx context.Context) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Try unbound-control reload first
	out, err := s.runner.Run(ctxTimeout, "unbound-control", "reload")
	if err == nil && (strings.Contains(out, "ok") || strings.TrimSpace(out) == "") {
		return nil
	}

	// Fallback to systemctl reload unbound, or restart if reload fails
	out, err = s.runner.Run(ctxTimeout, "systemctl", "reload", "unbound")
	if err != nil {
		out, err = s.runner.Run(ctxTimeout, "systemctl", "restart", "unbound")
		if err != nil {
			return fmt.Errorf("failed to reload/restart unbound: %w (output: %s)", err, out)
		}
	}
	return nil
}

// Restart restarts the Unbound daemon cleanly.
func (s *SystemdSupervisor) Restart(ctx context.Context) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	out, err := s.runner.Run(ctxTimeout, "systemctl", "restart", "unbound")
	if err != nil {
		return fmt.Errorf("failed to restart unbound: %w (output: %s)", err, out)
	}
	return nil
}

// IsRunning checks whether the Unbound service is active.
func (s *SystemdSupervisor) IsRunning(ctx context.Context) (bool, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := s.runner.Run(ctxTimeout, "systemctl", "is-active", "unbound")
	if err != nil {
		if strings.TrimSpace(out) == "inactive" || strings.TrimSpace(out) == "failed" {
			return false, nil
		}
		return false, fmt.Errorf("failed checking unbound status: %w (output: %s)", err, out)
	}
	return strings.TrimSpace(out) == "active", nil
}

// Checkconf tests a configuration file with unbound-checkconf.
func (s *SystemdSupervisor) Checkconf(ctx context.Context, confPath string) (string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return s.runner.Run(ctxTimeout, "unbound-checkconf", confPath)
}
