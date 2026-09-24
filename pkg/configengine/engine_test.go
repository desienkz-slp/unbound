package configengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"netora-dns/pkg/model"
)

// MockSupervisor implements SupervisorInterface for testing.
type MockSupervisor struct {
	ReloadFail   bool
	CheckFail    bool
	RestartFail  bool
	ReloadCalled int
}

func (m *MockSupervisor) Reload(ctx context.Context) error {
	m.ReloadCalled++
	if m.ReloadFail {
		return fmt.Errorf("simulated reload failure")
	}
	return nil
}

func (m *MockSupervisor) Restart(ctx context.Context) error {
	if m.RestartFail {
		return fmt.Errorf("simulated restart failure")
	}
	return nil
}

func (m *MockSupervisor) IsRunning(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockSupervisor) Checkconf(ctx context.Context, confPath string) (string, error) {
	if m.CheckFail {
		return "syntax error on line 42", fmt.Errorf("checkconf failure")
	}
	return "unbound-checkconf: no errors in " + confPath, nil
}

// MockHealthChecker implements HealthCheckerInterface for testing.
type MockHealthChecker struct {
	ShouldFail bool
}

func (m *MockHealthChecker) CheckResolution(ctx context.Context, serverIP string, serverPort int, protocol string, domain string) *HealthResult {
	if m.ShouldFail {
		return &HealthResult{
			Healthy:      false,
			ErrorMessage: "connection refused / SERVFAIL",
			Latency:      100 * time.Millisecond,
		}
	}
	return &HealthResult{
		Healthy: true,
		Latency: 5 * time.Millisecond,
	}
}

func TestEngine_HappyPathApply(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "netora-engine-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sup := &MockSupervisor{}
	health := &MockHealthChecker{ShouldFail: false}

	engine, err := NewEngine(EngineOptions{
		ConfigDir:     tempDir,
		Supervisor:    sup,
		HealthChecker: health,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.VersionID = "v-100"

	result, err := engine.Apply(context.Background(), state, "admin", "Initial deployment")
	if err != nil {
		t.Fatalf("apply failed unexpectedly: %v", err)
	}

	if !result.Success {
		t.Errorf("expected result.Success == true")
	}
	if !result.SyntaxValid || !result.SemanticValid || !result.ReloadSuccess || !result.HealthCheckSuccess {
		t.Errorf("all validation and health flags must be true, got %+v", result)
	}

	// Verify main conf file was created in configDir
	mainConfPath := filepath.Join(tempDir, "unbound.conf")
	if _, err := os.Stat(mainConfPath); err != nil {
		t.Errorf("expected unbound.conf in %s, got: %v", tempDir, err)
	}

	// Verify active_version.json
	activeVersion, err := engine.GetActiveVersion()
	if err != nil {
		t.Fatalf("failed to get active version: %v", err)
	}
	if activeVersion.VersionID != "v-100" {
		t.Errorf("expected active version v-100, got %s", activeVersion.VersionID)
	}
}

func TestEngine_RollbackOnReloadFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "netora-engine-rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sup := &MockSupervisor{}
	health := &MockHealthChecker{ShouldFail: false}

	engine, err := NewEngine(EngineOptions{
		ConfigDir:     tempDir,
		Supervisor:    sup,
		HealthChecker: health,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// 1. Apply good version v-1
	state1 := model.NewDefaultDesiredState("node-01", 2, 2048)
	state1.VersionID = "v-1"
	_, err = engine.Apply(context.Background(), state1, "admin", "v1 good")
	if err != nil {
		t.Fatalf("v1 apply failed: %v", err)
	}

	// 2. Trigger reload failure on v-2
	sup.ReloadFail = true
	state2 := model.NewDefaultDesiredState("node-01", 2, 2048)
	state2.VersionID = "v-2"

	res2, err := engine.Apply(context.Background(), state2, "admin", "v2 bad reload")
	if err == nil {
		t.Fatalf("expected error on reload failure, got nil")
	}

	if !res2.RolledBack {
		t.Errorf("expected res2.RolledBack == true")
	}
	if res2.RolledBackToVersion != "v-1" {
		t.Errorf("expected rolled back to v-1, got: %s", res2.RolledBackToVersion)
	}

	// Verify live config reverted to v-1 contents
	activeConf, err := os.ReadFile(filepath.Join(tempDir, "unbound.conf"))
	if err != nil {
		t.Fatalf("failed reading active config: %v", err)
	}
	if !strings.Contains(string(activeConf), "Version ID: v-1") {
		t.Errorf("expected live config to contain 'Version ID: v-1', got: %s", string(activeConf))
	}
}

func TestEngine_RollbackOnHealthCheckFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "netora-engine-health-rollback-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sup := &MockSupervisor{}
	health := &MockHealthChecker{ShouldFail: false}

	engine, err := NewEngine(EngineOptions{
		ConfigDir:     tempDir,
		Supervisor:    sup,
		HealthChecker: health,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// 1. Good v1
	state1 := model.NewDefaultDesiredState("node-01", 2, 2048)
	state1.VersionID = "v-10"
	_, _ = engine.Apply(context.Background(), state1, "admin", "v10 good")

	// 2. Apply v2 where health check fails
	health.ShouldFail = true
	state2 := model.NewDefaultDesiredState("node-01", 2, 2048)
	state2.VersionID = "v-20"

	res2, err := engine.Apply(context.Background(), state2, "admin", "v20 health fails")
	if err == nil {
		t.Fatalf("expected error on health check failure, got nil")
	}

	if !res2.RolledBack {
		t.Errorf("expected automated rollback after health check failure")
	}
	if res2.RolledBackToVersion != "v-10" {
		t.Errorf("expected rolled back to v-10, got %s", res2.RolledBackToVersion)
	}
}
