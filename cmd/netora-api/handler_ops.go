package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/cluster"
	"netora-dns/pkg/model"
)

// Handler: System Health
func (s *Server) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	dnsHealth := s.healthChecker.CheckResolution(r.Context(), "127.0.0.1", 53, "udp", "google.com")
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "UP",
		"timestamp":   time.Now().UTC(),
		"node_id":     s.nodeID,
		"dns_healthy": dnsHealth.Healthy,
		"dns_latency": dnsHealth.Latency.String(),
	})
}

// Handler: Trigger System Update (/api/v1/system/update)
func (s *Server) handleSystemUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	s.updateMu.Lock()
	if s.isUpdating {
		s.updateMu.Unlock()
		s.writeJSON(w, http.StatusConflict, map[string]interface{}{
			"status":  "CONFLICT",
			"message": "Update process is already running in background",
		})
		return
	}
	s.isUpdating = true
	s.lastUpdateErr = ""
	s.lastUpdateTime = time.Now()
	s.updateMu.Unlock()

	go func() {
		defer func() {
			s.updateMu.Lock()
			s.isUpdating = false
			s.updateMu.Unlock()
		}()

		// Locate updater script
		updaterPath := "/usr/local/sbin/netora-dns-update"
		if _, err := os.Stat(updaterPath); os.IsNotExist(err) {
			updaterPath = "/opt/netora-dns/current/scripts/netora-dns-update.sh"
		}
		if _, err := os.Stat(updaterPath); os.IsNotExist(err) {
			s.updateMu.Lock()
			s.lastUpdateErr = "updater script not found on system"
			s.updateMu.Unlock()
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, updaterPath, "update", "--force")
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		s.updateMu.Lock()
		s.lastUpdateLog = string(out)
		if err != nil {
			s.lastUpdateErr = fmt.Sprintf("update failed: %v", err)
		} else {
			s.lastUpdateErr = ""
		}
		s.updateMu.Unlock()
	}()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "IN_PROGRESS",
		"message": "Autonomous system update initiated. Node pulling from GitHub...",
		"node_id": s.nodeID,
	})
}

// Handler: Audit Logs
func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.ListAuditLogs(r.Context(), 100)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if logs == nil {
		logs = []model.AuditRecord{}
	}
	s.writeJSON(w, http.StatusOK, logs)
}

// Handler: Domain Diagnostics & Explainability (/api/v1/diagnose)
func (s *Server) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Domain   string `json:"domain"`
		ClientIP string `json:"client_ip,omitempty"`
		Policy   string `json:"policy,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.Domain == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain is required"})
		return
	}

	// 1. Evaluate in Policy Engine
	decision := s.policyEngine.Evaluate(req.Domain, req.Policy)

	// Record telemetry
	s.metrics.IncQueries("UDP", false)
	s.metrics.RecordDecision(string(decision.Action), decision.Category)

	// 2. Perform Live DNS Resolution Probe
	probe := s.healthChecker.CheckResolution(r.Context(), "127.0.0.1", 53, "udp", req.Domain)
	if probe.Latency > 0 {
		s.metrics.SetUpstreamLatency("127.0.0.1", float64(probe.Latency.Milliseconds()))
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"domain":        req.Domain,
		"policy":        decision.Policy,
		"action":        decision.Action,
		"category":      decision.Category,
		"source":        decision.Source,
		"rule":          decision.Rule,
		"explanation":   decision.Explanation,
		"rewrite_value": decision.RewriteValue,
		"live_resolution": map[string]interface{}{
			"healthy":      probe.Healthy,
			"latency":      probe.Latency.String(),
			"resolved_ips": probe.ResolvedIPs,
			"error":        probe.ErrorMessage,
		},
	})
}

// Handler: Operational Alerts (/api/v1/alerts)
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	probe := s.healthChecker.CheckResolution(r.Context(), "127.0.0.1", 53, "udp", "google.com")
	_ = s.alertEngine.Evaluate(r.Context(), probe.Healthy)
	alerts := s.alertEngine.GetActiveAlerts()
	s.writeJSON(w, http.StatusOK, alerts)
}

// Handler: Cluster Status (/api/v1/cluster/status)
func (s *Server) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	s.clusterManager.EvaluateClusterHealth()
	nodes := s.clusterManager.GetClusterNodes()
	summary := s.clusterManager.GetClusterSummary()
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary": summary,
		"nodes":   nodes,
	})
}

// Handler: Cluster Heartbeat (/api/v1/cluster/heartbeat)
func (s *Server) handleClusterHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var hb cluster.HeartbeatPayload
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := s.clusterManager.RecordHeartbeat(&hb); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// Handler: Cluster Sync (/api/v1/cluster/sync)
func (s *Server) handleClusterSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		State  *model.DesiredState `json:"state"`
		Reason string              `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.State == nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid desired state payload"})
		return
	}

	results, err := s.clusterManager.SyncClusterConfig(r.Context(), req.State, func(ctx context.Context, nodeID string, state *model.DesiredState) error {
		if nodeID == s.nodeID {
			_, err := s.engine.Apply(ctx, state, "cluster-sync", req.Reason)
			return err
		}
		// Remote node push would be forwarded here via mTLS/SSH/HTTP
		return nil
	})

	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Cluster configuration synchronization complete",
		"results": results,
		"summary": s.clusterManager.GetClusterSummary(),
	})
}

// Handler: Hermes Deep Diagnostic Investigation (/api/v1/hermes/investigate)
func (s *Server) handleHermesInvestigate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Domain      string `json:"domain"`
		ClientIP    string `json:"client_ip,omitempty"`
		PolicyGroup string `json:"policy_group,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	state, _ := s.store.GetDesiredState(r.Context(), s.nodeID)
	report := s.investigator.Investigate(r.Context(), req.Domain, req.ClientIP, req.PolicyGroup, state)
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// buildAuthenticatedRepoURL reads a GitHub token from tokenFile (must be mode 600/400)
// and injects it into the HTTPS clone URL in-memory only. Token is never logged or stored.
// If the token file is missing or the URL is not HTTPS, the original URL is returned unchanged.
func buildAuthenticatedRepoURL(repoURL, tokenFile string) string {
	if !strings.HasPrefix(repoURL, "https://") {
		return repoURL
	}
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return repoURL
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return repoURL
	}
	cleanURL := strings.TrimPrefix(repoURL, "https://")
	return "https://" + token + "@" + cleanURL
}

// Handler: Database Disaster Recovery Backups (/api/v1/system/backups/database)
func (s *Server) handleDatabaseBackups(w http.ResponseWriter, r *http.Request) {
	if s.databaseBackupManager == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Database backup manager not initialized"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		backups, err := s.databaseBackupManager.ListBackups()
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to list database backups: %v", err)})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"backups": backups,
			"total":   len(backups),
			"node_id": s.nodeID,
		})

	case http.MethodPost:
		var req struct {
			Description string `json:"description"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		info, err := s.databaseBackupManager.CreateBackup(r.Context(), req.Description)
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to create database backup: %v", err)})
			return
		}

		claims, _ := r.Context().Value(userClaimsKey).(*auth.Claims)
		actor := "system"
		if claims != nil {
			actor = claims.Username
		}
		_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
			Actor:    actor,
			Action:   "DB_BACKUP_CREATE",
			Target:   s.nodeID,
			Details:  fmt.Sprintf("Created database snapshot %s (%.2f MB)", info.Filename, info.SizeMB),
			ClientIP: r.RemoteAddr,
			Status:   "SUCCESS",
		})

		s.writeJSON(w, http.StatusCreated, map[string]interface{}{
			"message": "Database backup created successfully",
			"backup":  info,
		})

	default:
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// Handler: Database Disaster Recovery Operations (/api/v1/system/backups/database/{filename}/[download|restore])
func (s *Server) handleDatabaseBackupOperations(w http.ResponseWriter, r *http.Request) {
	if s.databaseBackupManager == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Database backup manager not initialized"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/system/backups/database/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Backup filename is required"})
		return
	}

	filename := parts[0]

	if len(parts) == 2 && parts[1] == "download" {
		if r.Method != http.MethodGet {
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		info, err := s.databaseBackupManager.GetBackup(filename)
		if err != nil {
			s.writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("Backup file not found: %v", err)})
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Filename))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, info.FilePath)
		return
	}

	if len(parts) == 2 && parts[1] == "restore" {
		if r.Method != http.MethodPost {
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if err := s.databaseBackupManager.RestoreBackup(r.Context(), filename); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to restore database: %v", err)})
			return
		}

		claims, _ := r.Context().Value(userClaimsKey).(*auth.Claims)
		actor := "system"
		if claims != nil {
			actor = claims.Username
		}
		_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
			Actor:    actor,
			Action:   "DB_BACKUP_RESTORE",
			Target:   s.nodeID,
			Details:  fmt.Sprintf("Restored database from snapshot %s", filename),
			ClientIP: r.RemoteAddr,
			Status:   "SUCCESS",
		})

		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":  "Database restored successfully from snapshot",
			"filename": filename,
		})
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			info, err := s.databaseBackupManager.GetBackup(filename)
			if err != nil {
				s.writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("Backup not found: %v", err)})
				return
			}
			s.writeJSON(w, http.StatusOK, info)

		case http.MethodDelete:
			if err := s.databaseBackupManager.DeleteBackup(filename); err != nil {
				s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to delete backup: %v", err)})
				return
			}

			claims, _ := r.Context().Value(userClaimsKey).(*auth.Claims)
			actor := "system"
			if claims != nil {
				actor = claims.Username
			}
			_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
				Actor:    actor,
				Action:   "DB_BACKUP_DELETE",
				Target:   s.nodeID,
				Details:  fmt.Sprintf("Deleted database snapshot %s", filename),
				ClientIP: r.RemoteAddr,
				Status:   "SUCCESS",
			})

			s.writeJSON(w, http.StatusOK, map[string]interface{}{
				"message":  "Database snapshot deleted successfully",
				"filename": filename,
			})

		default:
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
		return
	}

	s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
}
