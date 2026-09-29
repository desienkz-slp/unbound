package blocklist

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"netora-dns/pkg/storage"
)

func TestBlocklistManager_SanitizationAndWhitelist(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "netora-blocklist-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confDir := filepath.Join(tmpDir, "conf.d")
	_ = os.MkdirAll(confDir, 0755)

	// Create test raw files
	adsRaw := strings.Join([]string{
		"# Comment line",
		"! ABP comment",
		"||doubleclick.net^",
		"0.0.0.0 adservice.google.com",
		"127.0.0.1 localhost",
		"255.255.255.255 broadcasthost",
		"bca.co.id", // Should be whitelisted
		"valid-ad-tracker.com",
	}, "\n")

	whitelistRaw := strings.Join([]string{
		"@@||bca.co.id^",
		"wa.me",
	}, "\n")

	_ = os.WriteFile(filepath.Join(tmpDir, "ads.txt"), []byte(adsRaw), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "whitelist.txt"), []byte(whitelistRaw), 0644)

	store := storage.NewMemoryStore()

	// Use an offline HTTP client to force local disk reading for deterministic testing
	offlineClient := &http.Client{
		Transport: &mockTransport{statusCode: 404},
	}

	mgr := NewManager(ManagerOptions{
		BlocklistDir: tmpDir,
		ConfDDir:     confDir,
		Interval:     1 * time.Hour,
		Store:        store,
		NodeID:       "test-node",
		HTTPClient:   offlineClient,
	})

	report, err := mgr.SyncAll(context.Background(), "Unit Test")
	if err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}

	if !report.Success {
		t.Errorf("expected report.Success to be true")
	}

	// Verify ads.txt on disk
	adsCleaned, err := os.ReadFile(filepath.Join(tmpDir, "ads.txt"))
	if err != nil {
		t.Fatalf("failed to read cleaned ads.txt: %v", err)
	}
	adsContent := string(adsCleaned)

	// doubleclick.net and valid-ad-tracker.com must be present
	if !strings.Contains(adsContent, "doubleclick.net") {
		t.Errorf("expected doubleclick.net in cleaned ads.txt")
	}
	if !strings.Contains(adsContent, "valid-ad-tracker.com") {
		t.Errorf("expected valid-ad-tracker.com in cleaned ads.txt")
	}

	// bca.co.id must be EXCLUDED because of whitelist
	if strings.Contains(adsContent, "bca.co.id") {
		t.Errorf("bca.co.id should have been excluded by whitelist, but was found in ads.txt")
	}

	// localhost and 255.255.255.255 must be stripped
	if strings.Contains(adsContent, "localhost") {
		t.Errorf("localhost should have been stripped")
	}
	if strings.Contains(adsContent, "255.255.255.255") {
		t.Errorf("255.255.255.255 should have been stripped")
	}

	// Verify status
	status := mgr.GetStatus()
	if status.LastStatus != "SUCCESS" {
		t.Errorf("expected LastStatus SUCCESS, got %s", status.LastStatus)
	}
	if status.LastSyncTime == nil {
		t.Errorf("expected LastSyncTime to be set")
	}
}

type mockTransport struct {
	statusCode int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

// TestBlocklistManager_CrossFeedDeduplication verifies that a domain appearing in
// multiple feed files is only written to the highest-priority feed's output file and
// excluded from all lower-priority files, preventing duplicate local-zone warnings.
func TestBlocklistManager_CrossFeedDeduplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "netora-dedup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confDir := filepath.Join(tmpDir, "conf.d")
	_ = os.MkdirAll(confDir, 0755)

	// "overlap.com" appears in BOTH malware and ads feeds
	_ = os.WriteFile(filepath.Join(tmpDir, "malware.txt"), []byte("overlap.com\nmalware-only.com\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "ads.txt"), []byte("overlap.com\nads-only.com\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "whitelist.txt"), []byte(""), 0644)

	offlineClient := &http.Client{
		Transport: &mockTransport{statusCode: 404},
	}

	mgr := NewManager(ManagerOptions{
		BlocklistDir: tmpDir,
		ConfDDir:     confDir,
		Interval:     1 * time.Hour,
		Store:        storage.NewMemoryStore(),
		NodeID:       "dedup-test-node",
		HTTPClient:   offlineClient,
	})

	if _, err := mgr.SyncAll(context.Background(), "Dedup Test"); err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}

	malwareCleaned, _ := os.ReadFile(filepath.Join(tmpDir, "malware.txt"))
	adsCleaned, _ := os.ReadFile(filepath.Join(tmpDir, "ads.txt"))

	malwareOut := string(malwareCleaned)
	adsOut := string(adsCleaned)

	// "overlap.com" must be in malware (higher priority)
	if !strings.Contains(malwareOut, "overlap.com") {
		t.Errorf("overlap.com should be present in malware.txt (higher priority)")
	}
	// "overlap.com" must NOT be in ads (lower priority — deduped out)
	if strings.Contains(adsOut, "overlap.com") {
		t.Errorf("overlap.com should NOT be in ads.txt — it was already claimed by malware feed")
	}
	// unique domains must still be present
	if !strings.Contains(malwareOut, "malware-only.com") {
		t.Errorf("malware-only.com should be present in malware.txt")
	}
	if !strings.Contains(adsOut, "ads-only.com") {
		t.Errorf("ads-only.com should be present in ads.txt")
	}
}
