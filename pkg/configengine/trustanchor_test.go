package configengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureTrustAnchor(t *testing.T) {
	dir := t.TempDir()
	distro := filepath.Join(dir, "distro.key")
	if err := os.WriteFile(distro, []byte(". 3600 IN DNSKEY 257 3 8 AAAA\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Empty file (disk-full truncation) is restored.
	empty := filepath.Join(dir, "empty.key")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureTrustAnchor(empty, distro); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if fi, _ := os.Stat(empty); fi.Size() == 0 {
		t.Error("empty root.key was not restored")
	}

	// Missing file in a missing dir is created.
	missing := filepath.Join(dir, "sub", "missing.key")
	if err := ensureTrustAnchor(missing, distro); err != nil {
		t.Fatalf("missing: %v", err)
	}
	if _, err := os.Stat(missing); err != nil {
		t.Error("missing root.key was not created")
	}

	// A non-empty (possibly RFC 5011 managed) file is never overwritten.
	managed := filepath.Join(dir, "managed.key")
	if err := os.WriteFile(managed, []byte("managed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureTrustAnchor(managed, distro); err != nil {
		t.Fatalf("managed: %v", err)
	}
	if b, _ := os.ReadFile(managed); string(b) != "managed" {
		t.Errorf("non-empty root.key overwritten: %q", b)
	}

	// Empty path means DNSSEC anchor is not configured: no-op.
	if err := ensureTrustAnchor("", distro); err != nil {
		t.Errorf("empty path: %v", err)
	}
}
