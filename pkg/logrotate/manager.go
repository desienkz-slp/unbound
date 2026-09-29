package logrotate

import (
	"context"

	"netora-dns/pkg/model"
)

// Manager coordinates host log file inspection, rotation dry-runs, and configuration application.
type Manager struct{}

// NewManager creates a new logrotate manager instance.
func NewManager() *Manager {
	return &Manager{}
}

// GetReport gathers full system telemetry, active logrotate status, and hardware-aware AI recommendations.
func (m *Manager) GetReport(ctx context.Context, nodeID string, cores int, ramMB int, diskTotalGB int, diskFreeGB int) (*LogrotateReport, error) {
	return inspectHostLogrotate(ctx, nodeID, cores, ramMB, diskTotalGB, diskFreeGB)
}

// Apply writes the logrotate configuration files and verifies them via logrotate -d.
func (m *Manager) Apply(ctx context.Context, policy model.LogrotateConfig) (*ApplyResult, error) {
	return applyHostLogrotate(ctx, policy)
}

// Test runs a dry-run test (logrotate -d) using the specified or active policy.
func (m *Manager) Test(ctx context.Context, policy *model.LogrotateConfig) (string, error) {
	return testHostLogrotate(ctx, policy)
}

// ForceRotate immediately triggers logrotate -f to free disk space immediately.
func (m *Manager) ForceRotate(ctx context.Context) (string, error) {
	return forceRotateHost(ctx)
}
