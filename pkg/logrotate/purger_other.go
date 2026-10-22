//go:build !linux
// +build !linux

package logrotate

// getDiskUsagePct returns a simulated disk usage on non-Linux platforms.
func getDiskUsagePct(_ string) (int, error) {
	// ponytail: simulated 40% for dev/test; upgrade to platform-specific syscall if needed
	return 40, nil
}
