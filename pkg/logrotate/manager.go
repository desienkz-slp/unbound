package logrotate

import (
	"context"
	"log"
	"sync"
	"time"

	"netora-dns/pkg/model"
)

const autoPurgeInterval = 5 * time.Minute

// Manager coordinates host log file inspection, rotation dry-runs, configuration application,
// and the NVR-style auto-purge background loop.
type Manager struct {
	mu         sync.RWMutex
	status     PurgeStatus
	cancelFunc context.CancelFunc
}

// NewManager creates a new logrotate manager instance.
func NewManager() *Manager {
	return &Manager{}
}

// GetReport gathers full system telemetry, active logrotate status, and hardware-aware AI recommendations.
func (m *Manager) GetReport(ctx context.Context, nodeID string, cores int, ramMB int, diskTotalGB int, diskFreeGB int) (*LogrotateReport, error) {
	report, err := inspectHostLogrotate(ctx, nodeID, cores, ramMB, diskTotalGB, diskFreeGB)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	report.AutoPurgeStatus = m.status
	m.mu.RUnlock()
	return report, nil
}

// Apply writes the logrotate configuration files, verifies them, and (re)starts auto-purge if enabled.
func (m *Manager) Apply(ctx context.Context, policy model.LogrotateConfig) (*ApplyResult, error) {
	res, err := applyHostLogrotate(ctx, policy)
	if err != nil {
		return nil, err
	}
	// (Re)configure auto-purge based on new policy
	m.ConfigureAutoPurge(policy)
	return res, err
}

// Test runs a dry-run test (logrotate -d) using the specified or active policy.
func (m *Manager) Test(ctx context.Context, policy *model.LogrotateConfig) (string, error) {
	return testHostLogrotate(ctx, policy)
}

// ForceRotate immediately triggers logrotate -f to free disk space immediately.
func (m *Manager) ForceRotate(ctx context.Context) (string, error) {
	return forceRotateHost(ctx)
}

// ConfigureAutoPurge starts or stops the auto-purge goroutine based on policy.
func (m *Manager) ConfigureAutoPurge(policy model.LogrotateConfig) {
	m.mu.Lock()
	// Stop existing goroutine if running
	if m.cancelFunc != nil {
		m.cancelFunc()
		m.cancelFunc = nil
	}
	m.mu.Unlock()

	if !policy.AutoPurge {
		m.mu.Lock()
		m.status = PurgeStatus{Active: false}
		m.mu.Unlock()
		log.Printf("[auto-purge] disabled")
		return
	}

	threshold := policy.AutoPurgeThreshold
	if threshold <= 0 || threshold > 100 {
		threshold = 75
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancelFunc = cancel
	m.status = PurgeStatus{
		Active:    true,
		Threshold: threshold,
	}
	m.mu.Unlock()

	log.Printf("[auto-purge] started — threshold %d%%, interval %v", threshold, autoPurgeInterval)

	go m.autoPurgeLoop(ctx, threshold)
}

// GetPurgeStatus returns the current auto-purge status.
func (m *Manager) GetPurgeStatus() PurgeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// autoPurgeLoop runs periodically, checking disk and purging archives as needed.
func (m *Manager) autoPurgeLoop(ctx context.Context, threshold int) {
	ticker := time.NewTicker(autoPurgeInterval)
	defer ticker.Stop()

	// Run once immediately on start
	m.executePurgeCycle(threshold)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[auto-purge] stopped")
			return
		case <-ticker.C:
			m.executePurgeCycle(threshold)
		}
	}
}

func (m *Manager) executePurgeCycle(threshold int) {
	result, err := runPurgeCycle(threshold)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.LastCheckAt = time.Now()
	m.status.CurrentDiskPct = result.DiskPctAfter

	if err != nil {
		log.Printf("[auto-purge] cycle error: %v", err)
		return
	}

	if result.FilesDeleted > 0 {
		m.status.LastPurgeAt = time.Now()
		m.status.FilesPurged += result.FilesDeleted
		m.status.BytesReclaimed += result.BytesReclaimed
		log.Printf("[auto-purge] purged %d files, reclaimed %s, disk now %d%%",
			result.FilesDeleted, FormatBytes(result.BytesReclaimed), result.DiskPctAfter)
	}
}

