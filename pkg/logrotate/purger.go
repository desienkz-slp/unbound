package logrotate

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	unboundLogDir    = "/var/log/unbound"
	purgeGzPattern   = "unbound.log.*.gz"
	maxPurgePerCycle = 5 // Rate limit: max files deleted per cycle to avoid IO spike
	minRetainedFiles = 1 // Always keep at least 1 archive
)

// PurgeStatus tracks the state of the auto-purge subsystem.
type PurgeStatus struct {
	Active         bool      `json:"active"`
	Threshold      int       `json:"threshold_pct"`
	LastCheckAt    time.Time `json:"last_check_at,omitempty"`
	LastPurgeAt    time.Time `json:"last_purge_at,omitempty"`
	FilesPurged    int       `json:"files_purged"`
	BytesReclaimed int64     `json:"bytes_reclaimed"`
	CurrentDiskPct int       `json:"current_disk_pct"`
}

// purgeResult captures one cycle's outcome.
type purgeResult struct {
	FilesDeleted   int
	BytesReclaimed int64
	DiskPctAfter   int
}

// runPurgeCycle checks disk usage and deletes oldest .gz files if threshold is exceeded.
// Returns the purge result. Safe to call even when threshold is not reached (no-op).
func runPurgeCycle(threshold int) (purgeResult, error) {
	if threshold <= 0 || threshold > 100 {
		threshold = 75
	}

	diskPct, err := getDiskUsagePct(unboundLogDir)
	if err != nil {
		return purgeResult{}, fmt.Errorf("disk usage check: %w", err)
	}

	result := purgeResult{DiskPctAfter: diskPct}

	if diskPct < threshold {
		return result, nil
	}

	// Collect .gz files sorted by modification time (oldest first)
	archives, err := collectGzArchives(unboundLogDir)
	if err != nil {
		return result, fmt.Errorf("collecting archives: %w", err)
	}

	deleted := 0
	for _, a := range archives {
		// Safety: always keep at least minRetainedFiles
		remaining := len(archives) - deleted
		if remaining <= minRetainedFiles {
			break
		}
		// Rate limit per cycle
		if deleted >= maxPurgePerCycle {
			break
		}

		if err := os.Remove(a.path); err != nil {
			log.Printf("[auto-purge] failed to remove %s: %v", a.path, err)
			continue
		}
		log.Printf("[auto-purge] deleted %s (%s) — disk was %d%% >= %d%% threshold",
			filepath.Base(a.path), FormatBytes(a.size), diskPct, threshold)
		result.FilesDeleted++
		result.BytesReclaimed += a.size
		deleted++

		// Re-check disk after each deletion
		newPct, err := getDiskUsagePct(unboundLogDir)
		if err == nil {
			result.DiskPctAfter = newPct
			if newPct < threshold {
				break // Below threshold, stop purging
			}
		}
	}

	return result, nil
}

// archiveEntry represents a rotated .gz file with its metadata.
type archiveEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// collectGzArchives finds all .gz files in dir and returns them sorted oldest-first.
func collectGzArchives(dir string) ([]archiveEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var archives []archiveEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".gz") {
			continue
		}
		// Match unbound.log.*.gz pattern
		if !strings.HasPrefix(name, "unbound.log.") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		archives = append(archives, archiveEntry{
			path:    filepath.Join(dir, name),
			size:    fi.Size(),
			modTime: fi.ModTime(),
		})
	}

	// Sort oldest first (ascending modTime)
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].modTime.Before(archives[j].modTime)
	})

	return archives, nil
}
