package configengine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"netora-dns/pkg/model"
)

// EngineOptions configures directory paths and dependencies for the Engine.
type EngineOptions struct {
	ConfigDir       string
	Generator       *Generator
	Validator       *Validator
	Supervisor      SupervisorInterface
	HealthChecker   HealthCheckerInterface
	SkipHealthCheck bool
}

// SupervisorInterface defines the process supervisor requirements.
type SupervisorInterface interface {
	Reload(ctx context.Context) error
	Restart(ctx context.Context) error
	IsRunning(ctx context.Context) (bool, error)
	Checkconf(ctx context.Context, confPath string) (string, error)
}

// HealthCheckerInterface defines the health checker requirements.
type HealthCheckerInterface interface {
	CheckResolution(ctx context.Context, serverIP string, serverPort int, protocol string, domain string) *HealthResult
}

// HealthResult abstracts the DNS probe outcome.
type HealthResult struct {
	Healthy      bool
	ErrorMessage string
	Latency      time.Duration
}

func syntaxCheckError(err error, output string) string {
	if output = strings.TrimSpace(output); output != "" {
		return fmt.Sprintf("syntax check failed: %s", output)
	}
	return fmt.Sprintf("syntax check failed: %v", err)
}

// Engine implements the atomic apply, validation, and rollback lifecycle.
type Engine struct {
	mu            sync.Mutex
	configDir     string
	stagingDir    string
	historyDir    string
	generator     *Generator
	validator     *Validator
	supervisor    SupervisorInterface
	healthChecker HealthCheckerInterface
	skipHealth    bool
}

// NewEngine creates and initializes an atomic configuration engine.
func NewEngine(opts EngineOptions) (*Engine, error) {
	if opts.ConfigDir == "" {
		opts.ConfigDir = "/etc/unbound"
	}

	if opts.Generator == nil {
		gen, err := NewGenerator()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize generator: %w", err)
		}
		opts.Generator = gen
	}

	if opts.Validator == nil {
		opts.Validator = NewValidator(nil)
	}

	stagingDir := filepath.Join(opts.ConfigDir, "staging")
	historyDir := filepath.Join(opts.ConfigDir, "history")

	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}
	// Ensure log directory exists and is accessible by unbound
	_ = os.MkdirAll("/var/log/unbound", 0777)
	_ = os.Chmod("/var/log/unbound", 0777)

	return &Engine{
		configDir:     opts.ConfigDir,
		stagingDir:    stagingDir,
		historyDir:    historyDir,
		generator:     opts.Generator,
		validator:     opts.Validator,
		supervisor:    opts.Supervisor,
		healthChecker: opts.HealthChecker,
		skipHealth:    opts.SkipHealthCheck,
	}, nil
}

// Apply executes the complete atomic render -> validate -> stage -> apply -> health-check -> activate workflow.
func (e *Engine) Apply(ctx context.Context, state *model.DesiredState, actor, reason string) (*model.ApplyResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := &model.ApplyResult{
		VersionID: state.VersionID,
		NodeID:    state.NodeID,
		StartedAt: time.Now().UTC(),
	}

	// 1. Semantic Validation
	valResult := e.validator.ValidateState(state)
	if !valResult.Valid {
		result.CompletedAt = time.Now().UTC()
		result.DurationMs = time.Since(result.StartedAt).Milliseconds()
		result.SemanticValid = false
		result.ErrorMessage = fmt.Sprintf("semantic validation failed: %s", valResult.SemanticErrors[0])
		return result, fmt.Errorf("%s", result.ErrorMessage)
	}
	result.SemanticValid = true

	// 2. Render configuration to memory
	renderRes, err := e.generator.Generate(state, e.configDir)
	if err != nil {
		result.CompletedAt = time.Now().UTC()
		result.DurationMs = time.Since(result.StartedAt).Milliseconds()
		result.ErrorMessage = fmt.Sprintf("render error: %v", err)
		return result, err
	}

	// 3. Write to Staging Directory
	stageVersionDir := filepath.Join(e.stagingDir, state.VersionID)
	if err := os.MkdirAll(stageVersionDir, 0750); err != nil {
		return nil, fmt.Errorf("failed creating staging version dir: %w", err)
	}
	defer os.RemoveAll(stageVersionDir) // Clean up staging after apply

	if err := e.writeFiles(stageVersionDir, renderRes.Files); err != nil {
		result.CompletedAt = time.Now().UTC()
		result.DurationMs = time.Since(result.StartedAt).Milliseconds()
		result.ErrorMessage = fmt.Sprintf("failed writing staging files: %v", err)
		return result, err
	}

	// 4. Syntax Validation (unbound-checkconf on staged main config)
	if e.supervisor != nil && state.DNSSEC.Enabled {
		// Best effort: a failed repair surfaces through checkconf with its real error.
		if err := ensureTrustAnchor(state.DNSSEC.AutoTrustAnchorFile, distroRootKey); err != nil {
			result.ValidationOutput = err.Error()
		}
	}
	stagedMainConf := filepath.Join(stageVersionDir, "unbound.conf")
	if e.supervisor != nil {
		checkOut, err := e.supervisor.Checkconf(ctx, stagedMainConf)
		result.ValidationOutput = checkOut
		if err != nil {
			result.CompletedAt = time.Now().UTC()
			result.DurationMs = time.Since(result.StartedAt).Milliseconds()
			result.SyntaxValid = false
			result.ErrorMessage = syntaxCheckError(err, checkOut)
			return result, fmt.Errorf("%s", result.ErrorMessage)
		}
	}
	result.SyntaxValid = true

	// 5. Save Current Active State to History as "Last Known Good"
	currentActiveVersion, _ := e.GetActiveVersion()
	backupDir := filepath.Join(e.historyDir, fmt.Sprintf("backup-pre-%s", state.VersionID))
	_ = e.backupCurrentConfig(backupDir)

	// Determine if Unbound daemon restart is needed (e.g. interface listeners, ports changed)
	// Invariant: 'unbound-control reload' cannot dynamically rebind or unbind network interface sockets.
	oldConfPath := filepath.Join(e.configDir, "unbound.conf")
	restartNeeded := requiresDaemonRestart(oldConfPath, stagedMainConf)

	// 6. Atomic File Replacement
	if err := e.atomicCopyFiles(stageVersionDir, e.configDir, renderRes.Files); err != nil {
		result.CompletedAt = time.Now().UTC()
		result.DurationMs = time.Since(result.StartedAt).Milliseconds()
		result.ErrorMessage = fmt.Sprintf("atomic file replacement failed: %v", err)
		return result, err
	}

	// 7. Reload or Restart Service
	if e.supervisor != nil {
		var serviceErr error
		if restartNeeded {
			serviceErr = e.supervisor.Restart(ctx)
		} else {
			serviceErr = e.supervisor.Reload(ctx)
		}
		if serviceErr != nil {
			// RELOAD/RESTART FAILED -> IMMEDIATE ROLLBACK
			result.ReloadSuccess = false
			result.ErrorMessage = fmt.Sprintf("service reload/restart failed: %v", serviceErr)
			rollbackErr := e.executeRollback(ctx, backupDir, currentActiveVersion)
			result.RolledBack = true
			if currentActiveVersion != nil {
				result.RolledBackToVersion = currentActiveVersion.VersionID
			}
			result.CompletedAt = time.Now().UTC()
			result.DurationMs = time.Since(result.StartedAt).Milliseconds()
			return result, fmt.Errorf("service reload/restart failed (rolled back): %w (rollback error: %v)", serviceErr, rollbackErr)
		}
	}
	result.ReloadSuccess = true

	// 8. Health Check
	if !e.skipHealth && e.healthChecker != nil {
		// Collect candidate targets for probing
		type probeTarget struct {
			ip   string
			port int
		}
		var candidates []probeTarget

		// 1. Check if 127.0.0.1 or 0.0.0.0 is listened on
		hasWildcard := false
		hasLoopback := false
		for _, l := range state.Listeners {
			if !l.Enabled {
				continue
			}
			if l.IP == "0.0.0.0" || l.IP == "::" {
				hasWildcard = true
			}
			if l.IP == "127.0.0.1" || l.IP == "::1" {
				hasLoopback = true
			}
		}

		if hasWildcard || hasLoopback {
			candidates = append(candidates, probeTarget{ip: "127.0.0.1", port: 53})
		}

		// 2. Add Management role listeners
		for _, l := range state.Listeners {
			if l.Enabled && l.Role == model.RoleManagement {
				candidates = append(candidates, probeTarget{ip: l.IP, port: l.Port})
			}
		}

		// 3. Add any other enabled listeners
		for _, l := range state.Listeners {
			if l.Enabled {
				candidates = append(candidates, probeTarget{ip: l.IP, port: l.Port})
			}
		}

		// Fallback default if no candidates found
		if len(candidates) == 0 {
			candidates = append(candidates, probeTarget{ip: "127.0.0.1", port: 53})
		}

		// Probe across candidates with retries
		var health *HealthResult
		var lastErr string
		probeSuccess := false

		for attempt := 1; attempt <= 4; attempt++ {
			if attempt > 1 {
				time.Sleep(1000 * time.Millisecond)
			}
			for _, target := range candidates {
				h := e.healthChecker.CheckResolution(ctx, target.ip, target.port, "udp", "google.com")
				if h != nil {
					if h.Healthy {
						health = h
						probeSuccess = true
						break
					}
					lastErr = h.ErrorMessage
				}
			}
			if probeSuccess {
				break
			}
		}

		if !probeSuccess {
			// HEALTH CHECK FAILED -> AUTOMATIC ROLLBACK
			errMsg := "unknown health check failure"
			if health != nil && health.ErrorMessage != "" {
				errMsg = health.ErrorMessage
			} else if lastErr != "" {
				errMsg = lastErr
			}
			result.HealthCheckSuccess = false
			result.HealthOutput = errMsg
			result.ErrorMessage = fmt.Sprintf("health check failed after apply: %s", errMsg)

			rollbackErr := e.executeRollback(ctx, backupDir, currentActiveVersion)
			result.RolledBack = true
			if currentActiveVersion != nil {
				result.RolledBackToVersion = currentActiveVersion.VersionID
			}
			result.CompletedAt = time.Now().UTC()
			result.DurationMs = time.Since(result.StartedAt).Milliseconds()
			return result, fmt.Errorf("health check failed after apply (rolled back): %s (rollback error: %v)", errMsg, rollbackErr)
		}
		result.HealthOutput = fmt.Sprintf("healthy, latency=%v", health.Latency)
	}
	result.HealthCheckSuccess = true

	// 9. Mark Version Active & Record in History
	versionRecord := &model.ConfigVersion{
		VersionID:        state.VersionID,
		NodeID:           state.NodeID,
		CreatedAt:        result.StartedAt,
		Actor:            actor,
		Reason:           reason,
		Checksum:         renderRes.MainChecksum,
		Status:           model.StatusActive,
		ValidationReport: "Syntax & Semantic OK",
		AppliedAt:        &result.StartedAt,
		ActiveConfigPath: filepath.Join(e.configDir, "unbound.conf"),
	}

	if err := e.saveVersionRecord(versionRecord); err != nil {
		// Log warning, but apply itself succeeded
		fmt.Printf("warning: failed to save version record: %v\n", err)
	}

	// Archive version files in history
	versionArchiveDir := filepath.Join(e.historyDir, state.VersionID)
	_ = e.writeFiles(versionArchiveDir, renderRes.Files)
	e.pruneHistory(10)

	result.Success = true
	result.CompletedAt = time.Now().UTC()
	result.DurationMs = time.Since(result.StartedAt).Milliseconds()
	return result, nil
}

// Rollback restores a specific prior configuration from history.
func (e *Engine) Rollback(ctx context.Context, targetVersionID, actor, reason string) (*model.ApplyResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := &model.ApplyResult{
		VersionID:           targetVersionID,
		StartedAt:           time.Now().UTC(),
		RolledBack:          true,
		RolledBackToVersion: targetVersionID,
	}

	historyDir := filepath.Join(e.historyDir, targetVersionID)
	if _, err := os.Stat(historyDir); err != nil {
		result.CompletedAt = time.Now().UTC()
		result.ErrorMessage = fmt.Sprintf("target version '%s' not found in history", targetVersionID)
		return result, fmt.Errorf("%s", result.ErrorMessage)
	}

	oldConfPath := filepath.Join(e.configDir, "unbound.conf")
	targetConfPath := filepath.Join(historyDir, "unbound.conf")
	restartNeeded := requiresDaemonRestart(oldConfPath, targetConfPath)

	// Copy history files directly into config dir
	if err := e.copyDir(historyDir, e.configDir); err != nil {
		result.CompletedAt = time.Now().UTC()
		result.ErrorMessage = fmt.Sprintf("failed restoring history files: %v", err)
		return result, err
	}

	// Reload or restart supervisor
	if e.supervisor != nil {
		var serviceErr error
		if restartNeeded {
			serviceErr = e.supervisor.Restart(ctx)
		} else {
			serviceErr = e.supervisor.Reload(ctx)
		}
		if serviceErr != nil {
			result.CompletedAt = time.Now().UTC()
			result.ErrorMessage = fmt.Sprintf("failed reload/restart during rollback: %v", serviceErr)
			return result, serviceErr
		}
	}
	result.ReloadSuccess = true

	// Mark active
	versionRecord := &model.ConfigVersion{
		VersionID:        targetVersionID,
		CreatedAt:        result.StartedAt,
		Actor:            actor,
		Reason:           fmt.Sprintf("Rolled back to %s: %s", targetVersionID, reason),
		Status:           model.StatusActive,
		AppliedAt:        &result.StartedAt,
		ActiveConfigPath: filepath.Join(e.configDir, "unbound.conf"),
	}
	_ = e.saveVersionRecord(versionRecord)

	result.Success = true
	result.CompletedAt = time.Now().UTC()
	result.DurationMs = time.Since(result.StartedAt).Milliseconds()
	return result, nil
}

// executeRollback reverts the live config directory to the backup and reloads.
func (e *Engine) executeRollback(ctx context.Context, backupDir string, prevVersion *model.ConfigVersion) error {
	if _, err := os.Stat(backupDir); err != nil {
		return fmt.Errorf("no backup available for rollback: %w", err)
	}

	restartNeeded := requiresDaemonRestart(filepath.Join(e.configDir, "unbound.conf"), filepath.Join(backupDir, "unbound.conf"))

	if err := e.copyDir(backupDir, e.configDir); err != nil {
		return fmt.Errorf("failed copying backup to live config: %w", err)
	}

	if e.supervisor != nil {
		if restartNeeded {
			_ = e.supervisor.Restart(ctx)
		} else {
			_ = e.supervisor.Reload(ctx)
		}
	}

	if prevVersion != nil {
		prevVersion.Status = model.StatusRolledBack
		_ = e.saveVersionRecord(prevVersion)
	}

	return nil
}

// writeFiles writes a set of rendered files to a target directory.
func (e *Engine) writeFiles(targetDir string, files []RenderedFile) error {
	for _, f := range files {
		destPath := filepath.Join(targetDir, f.RelativePath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, f.Content, 0644); err != nil {
			return err
		}
	}
	return nil
}

// atomicCopyFiles copies staged files into the destination directory using atomic write-and-rename.
func (e *Engine) atomicCopyFiles(sourceDir, destDir string, files []RenderedFile) error {
	for _, f := range files {
		targetPath := filepath.Join(destDir, f.RelativePath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
			return err
		}

		tmpFile := targetPath + ".tmp"
		if err := os.WriteFile(tmpFile, f.Content, 0644); err != nil {
			return err
		}

		// Atomic replace on POSIX
		if err := os.Rename(tmpFile, targetPath); err != nil {
			// Fallback on Windows where os.Rename might fail if target exists
			_ = os.Remove(targetPath)
			if err := os.Rename(tmpFile, targetPath); err != nil {
				return fmt.Errorf("atomic rename failed for %s: %w", targetPath, err)
			}
		}
	}
	return nil
}

// backupCurrentConfig creates a timestamped copy of existing configuration files.
func (e *Engine) backupCurrentConfig(destDir string) error {
	return e.copyDir(e.configDir, destDir)
}

// copyDir recursively copies files from src to dst (skipping staging and history).
func (e *Engine) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip staging and history folders
		if rel == "staging" || rel == "history" || strings.HasPrefix(rel, "staging"+string(filepath.Separator)) || strings.HasPrefix(rel, "history"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0750)
		}

		return copyFile(path, targetPath)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func (e *Engine) saveVersionRecord(v *model.ConfigVersion) error {
	metaPath := filepath.Join(e.configDir, "active_version.json")
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

// GetActiveVersion reads the currently active version metadata.
func (e *Engine) GetActiveVersion() (*model.ConfigVersion, error) {
	metaPath := filepath.Join(e.configDir, "active_version.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var v model.ConfigVersion
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// extractRestartDirectives reads directives that require a daemon restart when modified
// (e.g. network interface listeners, port, threads, system buffers).
func extractRestartDirectives(confPath string) ([]string, error) {
	f, err := os.Open(confPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var directives []string
	restartPrefixes := []string{
		"interface:",
		"port:",
		"num-threads:",
		"so-rcvbuf:",
		"so-sndbuf:",
		"ip-transparent:",
		"ip-freebind:",
		"do-ip4:",
		"do-ip6:",
		"forward-tls-upstream:",
		"forward-zone:",
		"forward-addr:",
		"tls-cert-bundle:",
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		for _, prefix := range restartPrefixes {
			if strings.HasPrefix(line, prefix) {
				directives = append(directives, line)
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(directives)
	return directives, nil
}

// requiresDaemonRestart compares socket-level directives between two configuration files.
// Unbound's 'unbound-control reload' cannot dynamically rebind or unbind network interface sockets.
// When interface listeners, ports, or thread counts change, a daemon restart is required.
func requiresDaemonRestart(oldConfPath, newConfPath string) bool {
	oldDirectives, err := extractRestartDirectives(oldConfPath)
	if err != nil {
		// If old config does not exist or cannot be read, restart daemon to ensure clean binding
		return true
	}
	newDirectives, err := extractRestartDirectives(newConfPath)
	if err != nil {
		return true
	}
	if len(oldDirectives) != len(newDirectives) {
		return true
	}
	for i := range oldDirectives {
		if oldDirectives[i] != newDirectives[i] {
			return true
		}
	}
	return false
}

// pruneHistory removes older version backups and archives, keeping only the most recent `keep` directories.
func (e *Engine) pruneHistory(keep int) {
	if keep <= 0 {
		keep = 10
	}
	entries, err := os.ReadDir(e.historyDir)
	if err != nil {
		return
	}

	type dirEntry struct {
		name    string
		modTime time.Time
	}
	var dirs []dirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			if fi, err := entry.Info(); err == nil {
				dirs = append(dirs, dirEntry{name: entry.Name(), modTime: fi.ModTime()})
			}
		}
	}

	if len(dirs) <= keep {
		return
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].modTime.After(dirs[j].modTime)
	})

	for _, d := range dirs[keep:] {
		_ = os.RemoveAll(filepath.Join(e.historyDir, d.name))
	}
}
