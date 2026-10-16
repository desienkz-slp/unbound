package configengine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// distroRootKey is the DNSSEC root trust anchor shipped by the dns-root-data package.
const distroRootKey = "/usr/share/dns/root.key"

// ensureTrustAnchor repairs a missing or empty auto-trust-anchor-file. A 0-byte root.key
// (e.g. truncated when the disk filled up) makes unbound-checkconf fail with
// "error reading auto-trust-anchor-file", which blocks every Apply. It restores the file
// from the distro copy, or from unbound-anchor as a fallback. It never touches a
// non-empty file, so an RFC 5011 managed anchor is left intact.
func ensureTrustAnchor(path, distroCopy string) error {
	if path == "" {
		return nil
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
		return nil
	}
	if data, err := os.ReadFile(distroCopy); err == nil && len(data) > 0 {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create trust anchor dir: %w", err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("restore trust anchor from %s: %w", distroCopy, err)
		}
		chownUnbound(path)
		return nil
	}
	// unbound-anchor exits 1 when it had to bootstrap the file, so judge by file content.
	_ = exec.Command("unbound-anchor", "-a", path).Run()
	if fi, err := os.Stat(path); err != nil || fi.Size() == 0 {
		return fmt.Errorf("trust anchor %s is empty and could not be restored", path)
	}
	chownUnbound(path)
	return nil
}

// chownUnbound hands the file to the unbound user so the daemon can update it (RFC 5011).
func chownUnbound(path string) {
	_ = exec.Command("chown", "unbound:unbound", path).Run()
}
