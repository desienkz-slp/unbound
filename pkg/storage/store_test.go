package storage

import (
	"context"
	"testing"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/model"
)

func TestStorage_MemoryStoreOperations(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 1. User
	user := &auth.User{
		ID:           "usr-1",
		Username:     "netora_admin",
		PasswordHash: "hashed_pw",
		Role:         auth.RoleAdmin,
		Email:        "admin@netora.isp",
	}
	if err := store.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	fetchedUser, err := store.GetUserByUsername(ctx, "netora_admin")
	if err != nil {
		t.Fatalf("failed fetching user: %v", err)
	}
	if fetchedUser.Username != "netora_admin" || fetchedUser.Role != auth.RoleAdmin {
		t.Errorf("user mismatch: %+v", fetchedUser)
	}

	// 2. Node
	nodeMeta := &model.NodeMetadata{
		NodeID:         "node-01",
		Hostname:       "dns01.isp.net",
		UnboundVersion: "1.13.1",
		CPUCores:       4,
		TotalRAMMB:     4096,
		LastSeenAt:     time.Now().UTC(),
	}
	if err := store.UpsertNode(ctx, nodeMeta, "HEALTHY", "172.18.20.145"); err != nil {
		t.Fatalf("failed to upsert node: %v", err)
	}

	node, err := store.GetNode(ctx, "node-01")
	if err != nil {
		t.Fatalf("failed fetching node: %v", err)
	}
	if node.Hostname != "dns01.isp.net" || node.CPUCores != 4 {
		t.Errorf("node mismatch: %+v", node)
	}

	// 3. Desired State
	state := model.NewDefaultDesiredState("node-01", 4, 4096)
	state.VersionID = "v-555"
	if err := store.SaveDesiredState(ctx, state); err != nil {
		t.Fatalf("failed saving desired state: %v", err)
	}

	fetchedState, err := store.GetDesiredState(ctx, "node-01")
	if err != nil {
		t.Fatalf("failed getting desired state: %v", err)
	}
	if fetchedState.VersionID != "v-555" {
		t.Errorf("expected version v-555, got: %s", fetchedState.VersionID)
	}

	// 4. Audit Log
	audit := &model.AuditRecord{
		ID:        "aud-1",
		Timestamp: time.Now().UTC(),
		Actor:     "admin",
		Action:    "CONFIG_APPLY",
		Target:    "node-01",
		Status:    "SUCCESS",
	}
	if err := store.LogAudit(ctx, audit); err != nil {
		t.Fatalf("failed to log audit: %v", err)
	}

	logs, err := store.ListAuditLogs(ctx, 10)
	if err != nil || len(logs) != 1 {
		t.Errorf("expected 1 audit log, got: %d (err: %v)", len(logs), err)
	}

	// 5. Allowlist
	if err := store.AddAllowlist(ctx, "example.com", "AD_BYPASS", "Partner request", "admin"); err != nil {
		t.Fatalf("failed adding allowlist: %v", err)
	}
	al, err := store.ListAllowlist(ctx)
	if err != nil || len(al) != 1 {
		t.Errorf("expected 1 allowlist record, got: %d", len(al))
	}
}
