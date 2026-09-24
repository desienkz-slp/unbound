package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"netora-dns/pkg/model"
)

func TestClusterNodeRegistrationAndHeartbeat(t *testing.T) {
	cm := NewClusterManager(nil)

	node1 := &ClusterNode{
		ID:           "dns-01",
		Hostname:     "dns-01.isp.net",
		ManagementIP: "10.0.0.1",
		Role:         RolePrimary,
	}
	cm.RegisterNode(node1)

	nodes := cm.GetClusterNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node registered, got %d", len(nodes))
	}
	if nodes[0].State != StateOnline {
		t.Errorf("expected state ONLINE, got %s", nodes[0].State)
	}

	// Send successful heartbeat
	hb := &HeartbeatPayload{
		NodeID:          "dns-01",
		ConfigVersion:   "v1.0.0",
		ConfigChecksum:  "abc123sha",
		UnboundRunning:  true,
		Port53Listening: true,
		LatencyMS:       0.8,
	}
	if err := cm.RecordHeartbeat(hb); err != nil {
		t.Fatalf("record heartbeat failed: %v", err)
	}

	nodes = cm.GetClusterNodes()
	if nodes[0].ActiveConfigVersion != "v1.0.0" {
		t.Errorf("expected config version v1.0.0, got %s", nodes[0].ActiveConfigVersion)
	}
	if nodes[0].State != StateOnline {
		t.Errorf("expected state ONLINE, got %s", nodes[0].State)
	}

	// Send degraded heartbeat (Unbound crashed on dns-01)
	hbDown := &HeartbeatPayload{
		NodeID:          "dns-01",
		ConfigVersion:   "v1.0.0",
		ConfigChecksum:  "abc123sha",
		UnboundRunning:  false,
		Port53Listening: false,
		LatencyMS:       0.0,
	}
	_ = cm.RecordHeartbeat(hbDown)

	nodes = cm.GetClusterNodes()
	if nodes[0].State != StateDegraded {
		t.Errorf("expected state DEGRADED after data plane crash, got %s", nodes[0].State)
	}
}

func TestClusterHeartbeatTimeoutEvaluation(t *testing.T) {
	cm := NewClusterManager(nil)
	cm.degradedAfter = 50 * time.Millisecond
	cm.offlineAfter = 100 * time.Millisecond

	node := &ClusterNode{
		ID:           "dns-edge-02",
		Hostname:     "edge-02",
		ManagementIP: "10.0.0.2",
		Role:         RoleSecondary,
	}
	cm.RegisterNode(node)

	// Simulate time passing beyond degraded threshold
	time.Sleep(60 * time.Millisecond)
	cm.EvaluateClusterHealth()

	nodes := cm.GetClusterNodes()
	if nodes[0].State != StateDegraded {
		t.Errorf("expected state DEGRADED after 60ms, got %s", nodes[0].State)
	}

	// Simulate time passing beyond offline threshold
	time.Sleep(50 * time.Millisecond)
	cm.EvaluateClusterHealth()

	nodes = cm.GetClusterNodes()
	if nodes[0].State != StateOffline {
		t.Errorf("expected state OFFLINE after 110ms, got %s", nodes[0].State)
	}
}

func TestClusterConfigSync_FailureIsolation(t *testing.T) {
	cm := NewClusterManager(nil)

	nodeA := &ClusterNode{ID: "node-a", Hostname: "node-a", Role: RolePrimary, State: StateOnline}
	nodeB := &ClusterNode{ID: "node-b", Hostname: "node-b", Role: RoleSecondary, State: StateOnline}
	nodeC := &ClusterNode{ID: "node-c", Hostname: "node-c", Role: RoleSecondary, State: StateOnline}

	cm.RegisterNode(nodeA)
	cm.RegisterNode(nodeB)
	cm.RegisterNode(nodeC)

	targetState := &model.DesiredState{
		VersionID: "v-cluster-42",
		NodeID:    "cluster",
	}

	// Mock apply function: node-a and node-b succeed, node-c fails
	mockApply := func(ctx context.Context, nodeID string, state *model.DesiredState) error {
		if nodeID == "node-c" {
			return errors.New("unbound-checkconf syntax error on node-c")
		}
		return nil
	}

	results, err := cm.SyncClusterConfig(context.Background(), targetState, mockApply)
	if err != nil {
		t.Fatalf("sync cluster config returned unexpected error: %v", err)
	}

	if results["node-a"] != nil {
		t.Errorf("expected node-a to succeed, got %v", results["node-a"])
	}
	if results["node-b"] != nil {
		t.Errorf("expected node-b to succeed, got %v", results["node-b"])
	}
	if results["node-c"] == nil {
		t.Errorf("expected node-c to fail")
	}

	// Verify isolation invariant: node-a & node-b are SYNCHRONIZED, node-c is FAILED/DEGRADED
	nodeMap := make(map[string]*ClusterNode)
	for _, n := range cm.GetClusterNodes() {
		nodeMap[n.ID] = n
	}

	if nodeMap["node-a"].SyncStatus != SyncSynchronized || nodeMap["node-a"].State != StateOnline {
		t.Errorf("node-a should remain ONLINE & SYNCHRONIZED, got status=%s state=%s",
			nodeMap["node-a"].SyncStatus, nodeMap["node-a"].State)
	}
	if nodeMap["node-b"].SyncStatus != SyncSynchronized || nodeMap["node-b"].State != StateOnline {
		t.Errorf("node-b should remain ONLINE & SYNCHRONIZED, got status=%s state=%s",
			nodeMap["node-b"].SyncStatus, nodeMap["node-b"].State)
	}
	if nodeMap["node-c"].SyncStatus != SyncFailed || nodeMap["node-c"].State != StateDegraded {
		t.Errorf("node-c should be DEGRADED & FAILED, got status=%s state=%s",
			nodeMap["node-c"].SyncStatus, nodeMap["node-c"].State)
	}
}
