package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDatabaseManager_ListingAndPruning(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "netora-backup-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr, err := NewDatabaseManager(tempDir, "postgres://test", "node-test")
	if err != nil {
		t.Fatalf("failed creating manager: %v", err)
	}

	// Create dummy backup files
	f1 := filepath.Join(tempDir, "netora_node-test_20260928_100000.dump")
	f2 := filepath.Join(tempDir, "netora_node-test_20260929_100000.dump")
	invalidFile := filepath.Join(tempDir, "ignore_me.txt")

	if err := os.WriteFile(f1, []byte("dummy backup content 1"), 0640); err != nil {
		t.Fatalf("failed writing f1: %v", err)
	}
	if err := os.WriteFile(f2, []byte("dummy backup content 2 with more bytes"), 0640); err != nil {
		t.Fatalf("failed writing f2: %v", err)
	}
	if err := os.WriteFile(invalidFile, []byte("not a dump"), 0640); err != nil {
		t.Fatalf("failed writing invalidFile: %v", err)
	}

	// List backups
	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups error: %v", err)
	}

	if len(backups) != 2 {
		t.Fatalf("expected 2 valid backups, got %d", len(backups))
	}

	// Check metadata
	b := backups[0]
	if b.SHA256 == "" {
		t.Errorf("expected non-empty sha256 checksum")
	}
	if b.SizeBytes <= 0 {
		t.Errorf("expected positive size_bytes")
	}

	// Test GetBackup
	single, err := mgr.GetBackup("netora_node-test_20260929_100000.dump")
	if err != nil {
		t.Fatalf("GetBackup error: %v", err)
	}
	if single.Filename != "netora_node-test_20260929_100000.dump" {
		t.Errorf("expected filename match, got %s", single.Filename)
	}

	// Test Path Traversal security
	_, err = mgr.GetBackup("../../../etc/passwd")
	if err == nil {
		t.Errorf("expected path traversal detection error")
	}

	// Test DeleteBackup
	if err := mgr.DeleteBackup("netora_node-test_20260928_100000.dump"); err != nil {
		t.Fatalf("DeleteBackup error: %v", err)
	}

	backupsAfterDelete, _ := mgr.ListBackups()
	if len(backupsAfterDelete) != 1 {
		t.Errorf("expected 1 backup after delete, got %d", len(backupsAfterDelete))
	}

	// Test PruneOldBackups
	// Modify timestamp of remaining file to 10 days ago
	tenDaysAgo := time.Now().AddDate(0, 0, -10)
	_ = os.Chtimes(f2, tenDaysAgo, tenDaysAgo)

	pruned, err := mgr.pruneLocked()
	if err != nil {
		t.Fatalf("pruneLocked error: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 file pruned, got %d", pruned)
	}

	finalBackups, _ := mgr.ListBackups()
	if len(finalBackups) != 0 {
		t.Errorf("expected 0 backups after pruning old files, got %d", len(finalBackups))
	}
}
