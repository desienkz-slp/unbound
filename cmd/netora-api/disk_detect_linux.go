//go:build linux
// +build linux

package main

import "syscall"

// detectDiskSpecs probes the root filesystem for total and available disk space (in GB).
// Uses syscall.Statfs on Linux. Protected by build tag.
func detectDiskSpecs() (totalGB int, freeGB int) {
	var stat syscall.Statfs_t
	// Primary: root filesystem
	if err := syscall.Statfs("/", &stat); err == nil {
		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		totalGB = int(totalBytes / (1024 * 1024 * 1024))
		freeGB = int(freeBytes / (1024 * 1024 * 1024))
		return totalGB, freeGB
	}
	// Fallback: check /opt (DNS data partition may be separate mount)
	if err := syscall.Statfs("/opt", &stat); err == nil {
		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		totalGB = int(totalBytes / (1024 * 1024 * 1024))
		freeGB = int(freeBytes / (1024 * 1024 * 1024))
	}
	return totalGB, freeGB
}
