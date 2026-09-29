package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var safeFilenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+\.dump(\.gz)?$`)

// DatabaseBackupInfo captures metadata for a stored database snapshot.
type DatabaseBackupInfo struct {
	Filename    string    `json:"filename"`
	FilePath    string    `json:"file_path"`
	NodeID      string    `json:"node_id"`
	SizeMB      float64   `json:"size_mb"`
	SizeBytes   int64     `json:"size_bytes"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"created_at"`
	Format      string    `json:"format"`
	Description string    `json:"description,omitempty"`
}

// DatabaseManager handles PostgreSQL automated backups, listing, pruning, and restore.
type DatabaseManager struct {
	backupDir     string
	dbURL         string
	nodeID        string
	retentionDays int
	mu            sync.Mutex
}

// NewDatabaseManager initializes the backup manager with safe default directories.
func NewDatabaseManager(backupDir, dbURL, nodeID string) (*DatabaseManager, error) {
	if backupDir == "" {
		backupDir = "/var/backups/netora-dns/database"
	}
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		// Fallback to local data dir if /var/backups is not writable (e.g. running in user mode/windows tests)
		fallbackDir := filepath.Join(os.TempDir(), "netora-backups-database")
		_ = os.MkdirAll(fallbackDir, 0750)
		backupDir = fallbackDir
	}

	return &DatabaseManager{
		backupDir:     backupDir,
		dbURL:         dbURL,
		nodeID:        nodeID,
		retentionDays: 7,
	}, nil
}

// CreateBackup generates an atomic compressed PostgreSQL snapshot using pg_dump.
func (m *DatabaseManager) CreateBackup(ctx context.Context, description string) (*DatabaseBackupInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	timestamp := time.Now().UTC().Format("20060102_150405")
	filename := fmt.Sprintf("netora_%s_%s.dump", m.nodeID, timestamp)
	targetPath := filepath.Join(m.backupDir, filename)
	tempPath := targetPath + ".tmp"

	// Check if pg_dump is available
	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		// If pg_dump is not in PATH, return actionable error
		return nil, fmt.Errorf("pg_dump binary not found in system PATH: %w", err)
	}

	// Clean target temp before writing
	_ = os.Remove(tempPath)

	// Command: pg_dump -d <dbURL> -Fc -f <tempPath>
	// -Fc generates PostgreSQL custom archive format (compressed, supports parallel restore & selective tables)
	cmd := exec.CommandContext(ctx, pgDumpPath, "-d", m.dbURL, "-Fc", "-f", tempPath)
	cmd.Env = os.Environ()

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("pg_dump execution failed: %s (%w)", stderr.String(), err)
	}

	// Rename tempPath to targetPath atomically
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("failed to finalize backup file: %w", err)
	}

	// Compute checksum and size
	info, err := m.inspectBackupFile(targetPath)
	if err != nil {
		return nil, err
	}
	info.Description = description

	// Auto-prune old snapshots beyond retention period
	_, _ = m.pruneLocked()

	return info, nil
}

// ListBackups returns all available snapshots sorted by creation time descending (newest first).
func (m *DatabaseManager) ListBackups() ([]DatabaseBackupInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []DatabaseBackupInfo{}, nil
		}
		return nil, fmt.Errorf("failed reading backup directory: %w", err)
	}

	backups := make([]DatabaseBackupInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !safeFilenameRegex.MatchString(name) {
			continue
		}

		fullPath := filepath.Join(m.backupDir, name)
		info, err := m.inspectBackupFile(fullPath)
		if err != nil {
			continue
		}
		backups = append(backups, *info)
	}

	// Sort newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// GetBackup returns metadata for a single specific snapshot.
func (m *DatabaseManager) GetBackup(filename string) (*DatabaseBackupInfo, error) {
	if !safeFilenameRegex.MatchString(filename) {
		return nil, errors.New("invalid backup filename format")
	}

	fullPath := filepath.Join(m.backupDir, filepath.Clean(filename))
	if !strings.HasPrefix(fullPath, filepath.Clean(m.backupDir)) {
		return nil, errors.New("path traversal detected")
	}

	return m.inspectBackupFile(fullPath)
}

// DeleteBackup permanently removes a snapshot from storage.
func (m *DatabaseManager) DeleteBackup(filename string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !safeFilenameRegex.MatchString(filename) {
		return errors.New("invalid backup filename format")
	}

	fullPath := filepath.Join(m.backupDir, filepath.Clean(filename))
	if !strings.HasPrefix(fullPath, filepath.Clean(m.backupDir)) {
		return errors.New("path traversal detected")
	}

	return os.Remove(fullPath)
}

// RestoreBackup restores the database from a backup snapshot using pg_restore.
func (m *DatabaseManager) RestoreBackup(ctx context.Context, filename string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !safeFilenameRegex.MatchString(filename) {
		return errors.New("invalid backup filename format")
	}

	fullPath := filepath.Join(m.backupDir, filepath.Clean(filename))
	if !strings.HasPrefix(fullPath, filepath.Clean(m.backupDir)) {
		return errors.New("path traversal detected")
	}

	if _, err := os.Stat(fullPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	pgRestorePath, err := exec.LookPath("pg_restore")
	if err != nil {
		return fmt.Errorf("pg_restore binary not found in system PATH: %w", err)
	}

	// pg_restore --clean --if-exists -d <dbURL> <fullPath>
	cmd := exec.CommandContext(ctx, pgRestorePath, "--clean", "--if-exists", "-d", m.dbURL, fullPath)
	cmd.Env = os.Environ()

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// pg_restore may return exit code 1 with non-critical warnings (e.g. drop table if not exists)
		errMsg := stderr.String()
		if !strings.Contains(errMsg, "errors ignored on restore") && len(errMsg) > 0 {
			return fmt.Errorf("pg_restore execution failed: %s (%w)", errMsg, err)
		}
	}

	return nil
}

// pruneLocked cleans up backups older than retention period.
func (m *DatabaseManager) pruneLocked() (int, error) {
	cutoff := time.Now().AddDate(0, 0, -m.retentionDays)
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		return 0, err
	}

	pruned := 0
	for _, entry := range entries {
		if entry.IsDir() || !safeFilenameRegex.MatchString(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(m.backupDir, entry.Name()))
			pruned++
		}
	}
	return pruned, nil
}

// inspectBackupFile calculates sha256 checksum and metadata for a file.
func (m *DatabaseManager) inspectBackupFile(fullPath string) (*DatabaseBackupInfo, error) {
	fi, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed stating backup file: %w", err)
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed opening backup file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, fmt.Errorf("failed calculating checksum: %w", err)
	}

	hashStr := hex.EncodeToString(hasher.Sum(nil))
	sizeBytes := fi.Size()
	sizeMB := float64(sizeBytes) / (1024 * 1024)

	return &DatabaseBackupInfo{
		Filename:  filepath.Base(fullPath),
		FilePath:  fullPath,
		NodeID:    m.nodeID,
		SizeBytes: sizeBytes,
		Size:      sizeBytes,
		SizeMB:    float64(int(sizeMB*100)) / 100.0,
		SHA256:    hashStr,
		CreatedAt: fi.ModTime().UTC(),
		Format:    "PostgreSQL Custom Archive (pg_dump -Fc)",
	}, nil
}
