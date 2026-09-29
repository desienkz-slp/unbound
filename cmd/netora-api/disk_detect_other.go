//go:build !linux
// +build !linux

package main

// detectDiskSpecs is a stub for non-Linux platforms.
// Returns 0,0 — disk info only available on Linux (production target).
func detectDiskSpecs() (totalGB int, freeGB int) {
	return 0, 0
}
