package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"netora-dns/pkg/model"
	"netora-dns/pkg/storage"
)

// NodeRole indicates cluster authority.
type NodeRole string

const (
	RolePrimary   NodeRole = "PRIMARY"
	RoleSecondary NodeRole = "SECONDARY"
)

// NodeClusterState indicates operational readiness.
type NodeClusterState string

const (
	StateOnline    NodeClusterState = "ONLINE"
	StateSyncing   NodeClusterState = "SYNCING"
	StateDegraded  NodeClusterState = "DEGRADED"
	StateOffline   NodeClusterState = "OFFLINE"
)

// SyncStatus indicates config parity with desired cluster state.
type SyncStatus string

const (
	SyncSynchronized SyncStatus = "SYNCHRONIZED"
	SyncPending      SyncStatus = "PENDING"
	SyncFailed       SyncStatus = "FAILED"
)

// ClusterNode represents a managed DNS resolver in a multi-node topology.
type ClusterNode struct {
	ID                  string           `json:"id"`
	Hostname            string           `json:"hostname"`
	ManagementIP        string           `json:"management_ip"`
	Role                NodeRole         `json:"role"`
	State               NodeClusterState `json:"state"`
	ActiveConfigVersion string           `json:"active_config_version"`
	ActiveChecksum      string           `json:"active_checksum"`
	SyncStatus          SyncStatus       `json:"sync_status"`
	LastHeartbeat       time.Time        `json:"last_heartbeat"`
	LatencyMS           float64          `json:"latency_ms"`
	UnboundVersion      string           `json:"unbound_version"`
	CPUCores            int              `json:"cpu_cores"`
	RAMMB               int              `json:"ram_mb"`
}

// HeartbeatPayload represents a periodic liveness message sent by a node.
type HeartbeatPayload struct {
	NodeID              string  `json:"node_id"`
	ConfigVersion       string  `json:"config_version"`
	ConfigChecksum      string  `json:"config_checksum"`
	UnboundRunning      bool    `json:"unbound_running"`
	Port53Listening     bool    `json:"port53_listening"`
	LatencyMS           float64 `json:"latency_ms"`
	UnboundVersion      string  `json:"unbound_version,omitempty"`
	CPUCores            int     `json:"cpu_cores,omitempty"`
	RAMMB               int     `json:"ram_mb,omitempty"`
}

// ClusterManager coordinates multi-node discovery, heartbeats, and safe configuration sync.
type ClusterManager struct {
	mu            sync.RWMutex
	store         storage.Store
	nodes         map[string]*ClusterNode
	targetVersion string
	targetChecksum string
	degradedAfter time.Duration
	offlineAfter  time.Duration
}

// NewClusterManager creates a cluster management engine.
func NewClusterManager(store storage.Store) *ClusterManager {
	return &ClusterManager{
		store:         store,
		nodes:         make(map[string]*ClusterNode),
		degradedAfter: 20 * time.Second,
		offlineAfter:  45 * time.Second,
	}
}

// RegisterNode registers or updates a node in the cluster.
func (cm *ClusterManager) RegisterNode(node *ClusterNode) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if node.State == "" {
		node.State = StateOnline
	}
	if node.SyncStatus == "" {
		node.SyncStatus = SyncSynchronized
	}
	node.LastHeartbeat = time.Now().UTC()
	cm.nodes[node.ID] = node
}

// RecordHeartbeat processes a periodic liveness ping from a node.
func (cm *ClusterManager) RecordHeartbeat(hb *HeartbeatPayload) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[hb.NodeID]
	if !exists {
		// Auto-register new secondary node
		node = &ClusterNode{
			ID:             hb.NodeID,
			Hostname:       hb.NodeID,
			Role:           RoleSecondary,
			UnboundVersion: hb.UnboundVersion,
			CPUCores:       hb.CPUCores,
			RAMMB:          hb.RAMMB,
		}
		cm.nodes[hb.NodeID] = node
	}

	node.LastHeartbeat = time.Now().UTC()
	node.LatencyMS = hb.LatencyMS
	node.ActiveConfigVersion = hb.ConfigVersion
	node.ActiveChecksum = hb.ConfigChecksum

	// Evaluate state based on data plane health
	if !hb.UnboundRunning || !hb.Port53Listening {
		node.State = StateDegraded
	} else {
		node.State = StateOnline
	}

	// Evaluate synchronization status against target
	if cm.targetChecksum != "" && hb.ConfigChecksum != cm.targetChecksum {
		node.SyncStatus = SyncPending
	} else {
		node.SyncStatus = SyncSynchronized
	}

	return nil
}

// EvaluateClusterHealth evaluates stale heartbeats across all nodes.
func (cm *ClusterManager) EvaluateClusterHealth() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now().UTC()
	for _, node := range cm.nodes {
		since := now.Sub(node.LastHeartbeat)
		if since > cm.offlineAfter {
			node.State = StateOffline
			node.SyncStatus = SyncFailed
		} else if since > cm.degradedAfter && node.State != StateOffline {
			node.State = StateDegraded
		}
	}
}

// NodeApplyFunc defines the callback to apply a configuration onto a node.
type NodeApplyFunc func(ctx context.Context, nodeID string, state *model.DesiredState) error

// SyncClusterConfig distributes and applies a desired state across all online nodes.
// Isolation Invariant: A failure applying to one node does NOT abort or corrupt healthy nodes.
func (cm *ClusterManager) SyncClusterConfig(ctx context.Context, state *model.DesiredState, applyFn NodeApplyFunc) (map[string]error, error) {
	cm.mu.Lock()
	raw, _ := json.Marshal(state)
	sum := sha256.Sum256(raw)
	cm.targetVersion = state.VersionID
	cm.targetChecksum = hex.EncodeToString(sum[:])
	nodesToSync := make([]*ClusterNode, 0, len(cm.nodes))
	for _, n := range cm.nodes {
		nodesToSync = append(nodesToSync, n)
	}
	cm.mu.Unlock()

	if len(nodesToSync) == 0 {
		return nil, errors.New("no cluster nodes registered")
	}

	results := make(map[string]error)
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	for _, n := range nodesToSync {
		if n.State == StateOffline {
			resultsMu.Lock()
			results[n.ID] = fmt.Errorf("node %s is offline; skipped", n.ID)
			resultsMu.Unlock()
			continue
		}

		wg.Add(1)
		go func(target *ClusterNode) {
			defer wg.Done()

			cm.mu.Lock()
			target.SyncStatus = SyncPending
			cm.mu.Unlock()

			err := applyFn(ctx, target.ID, state)

			cm.mu.Lock()
			defer cm.mu.Unlock()
			resultsMu.Lock()
			defer resultsMu.Unlock()

			if err != nil {
				target.SyncStatus = SyncFailed
				target.State = StateDegraded
				results[target.ID] = err
			} else {
				target.SyncStatus = SyncSynchronized
				target.ActiveConfigVersion = state.VersionID
				target.ActiveChecksum = cm.targetChecksum
				results[target.ID] = nil
			}
		}(n)
	}

	wg.Wait()
	return results, nil
}

// GetClusterNodes returns a snapshot of all cluster nodes.
func (cm *ClusterManager) GetClusterNodes() []*ClusterNode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	list := make([]*ClusterNode, 0, len(cm.nodes))
	for _, n := range cm.nodes {
		list = append(list, n)
	}
	return list
}

// GetClusterSummary returns overall cluster state and quorum overview.
func (cm *ClusterManager) GetClusterSummary() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	total := len(cm.nodes)
	online := 0
	degraded := 0
	offline := 0
	synchronized := 0

	for _, n := range cm.nodes {
		switch n.State {
		case StateOnline:
			online++
		case StateDegraded:
			degraded++
		case StateOffline:
			offline++
		}
		if n.SyncStatus == SyncSynchronized {
			synchronized++
		}
	}

	return map[string]interface{}{
		"total_nodes":        total,
		"online_nodes":       online,
		"degraded_nodes":     degraded,
		"offline_nodes":      offline,
		"synchronized_nodes": synchronized,
		"target_version":     cm.targetVersion,
		"target_checksum":    cm.targetChecksum,
		"quorum_healthy":     online >= (total/2)+1,
	}
}
