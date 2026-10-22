package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/model"
	"netora-dns/pkg/telemetry"
)

// Handler: Nodes List
func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.store.ListNodes(r.Context())
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, nodes)
}

// Handler: Node Subroutes (/api/v1/nodes/{id}/...)
func (s *Server) handleNodeSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		s.handleNodes(w, r)
		return
	}

	nodeID := parts[0]
	if len(parts) == 1 {
		node, err := s.store.GetNode(r.Context(), nodeID)
		if err != nil {
			s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
			return
		}
		s.writeJSON(w, http.StatusOK, node)
		return
	}

	action := parts[1]
	if action == "config" && len(parts) > 2 {
		action = parts[2]
	}

	switch action {
	case "health":
		res := s.healthChecker.CheckResolution(r.Context(), "127.0.0.1", 53, "udp", "google.com")
		s.writeJSON(w, http.StatusOK, res)

	case "stats":
		tailer := s.queryLogTailer
		if tailer == nil {
			tailer = telemetry.DefaultTailer
		}
		if tailer != nil {
			report := tailer.GetDetailedReport(r.Context())
			// Reconcile with active desired state so telemetry strictly matches the active filtering setup
			if s.store != nil {
				if cfg, err := s.store.GetDesiredState(r.Context(), nodeID); err == nil && cfg != nil {
					isAdultActive := cfg.ParentalControl.Enabled && cfg.ParentalControl.BlockAdult
					if !isAdultActive {
						if report.BlockedAdult > 0 {
							report.BlockedAds += report.BlockedAdult
							report.BlockedAdult = 0
						}
						report.AdultHistory = make([]int64, 24)
					}
					isGamblingActive := cfg.GamblingBlock.Enabled || (cfg.ParentalControl.Enabled && cfg.ParentalControl.BlockGambling)
					if !isGamblingActive {
						if report.BlockedGambling > 0 {
							report.BlockedAds += report.BlockedGambling
							report.BlockedGambling = 0
						}
						report.GamblingHistory = make([]int64, 24)
					}
					if !isAdultActive || !isGamblingActive {
						for i := range report.TopBlockedDomains {
							if !isAdultActive && report.TopBlockedDomains[i].Category == "18+ Adult" {
								report.TopBlockedDomains[i].Category = "DNS Filter Block"
							}
							if !isGamblingActive && report.TopBlockedDomains[i].Category == "Judi Online" {
								report.TopBlockedDomains[i].Category = "DNS Filter Block"
							}
						}
					}
				}
			}
			s.writeJSON(w, http.StatusOK, report)
			return
		}

		cmd := exec.CommandContext(r.Context(), "unbound-control", "stats_noreset")
		out, err := cmd.Output()
		var totalQueries, cacheHits, cacheMisses float64
		var avgTime float64
		if err == nil {
			parsed := telemetry.ParseStatsString(string(out))
			totalQueries = parsed["total.num.queries"]
			cacheHits = parsed["total.num.cachehits"]
			cacheMisses = parsed["total.num.cachemiss"]
			avgTime = parsed["total.recursion.time.avg"] * 1000 // to ms
		}
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"total_queries":          int64(totalQueries),
			"cache_hits":             int64(cacheHits),
			"cache_misses":           int64(cacheMisses),
			"avg_processing_time_ms": avgTime,
			"blocked_queries":        0,
			"safe_search_enforced":   0,
		})
		return

	case "config":
		if r.Method == http.MethodGet {
			cfg, err := s.store.GetDesiredState(r.Context(), nodeID)
			if err != nil {
				// Return default if not stored
				cfg = model.NewDefaultDesiredState(nodeID, 2, 2048)
			}
			var nodeIPs []string
			if nodeID == s.nodeID {
				nodeIPs = detectActiveIPv4Addresses()
			}
			if len(nodeIPs) == 0 && s.mgmtIP != "" && s.mgmtIP != "127.0.0.1" && nodeID == s.nodeID {
				nodeIPs = []string{s.mgmtIP}
			}
			cfg.EnsureNodeDnsRewrites(nodeID, nodeIPs)
			s.writeJSON(w, http.StatusOK, cfg)
			return
		}
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})

	case "apply":
		if r.Method != http.MethodPost {
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read request body"})
			return
		}

		var bodyMap map[string]json.RawMessage
		var state model.DesiredState
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil && bodyMap["state"] != nil {
			if err := json.Unmarshal(bodyMap["state"], &state); err != nil {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid state in body: " + err.Error()})
				return
			}
		} else {
			if err := json.Unmarshal(bodyBytes, &state); err != nil {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
				return
			}
		}

		if strings.TrimSpace(state.NodeID) == "" {
			state.NodeID = nodeID
		}
		if strings.TrimSpace(state.VersionID) == "" {
			state.VersionID = fmt.Sprintf("v-%d", time.Now().Unix())
		}
		if strings.TrimSpace(string(state.ResolutionMode)) == "" {
			state.ResolutionMode = model.ModeRecursive
		}

		// Ensure default node DNS rewrites are preserved during apply
		var applyIPs []string
		if nodeID == s.nodeID {
			applyIPs = detectActiveIPv4Addresses()
		}
		if len(applyIPs) == 0 && s.mgmtIP != "" && s.mgmtIP != "127.0.0.1" && nodeID == s.nodeID {
			applyIPs = []string{s.mgmtIP}
		}
		state.EnsureNodeDnsRewrites(nodeID, applyIPs)

		claims := r.Context().Value(userClaimsKey).(*auth.Claims)
		log.Printf("[apply] Node %s: starting apply for version %s (actor: %s)\n", nodeID, state.VersionID, claims.Username)
		res, err := s.engine.Apply(r.Context(), &state, claims.Username, "REST API Apply")
		if err != nil {
			log.Printf("[apply] Node %s: apply failed: %v\n", nodeID, err)
			_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
				Actor:    claims.Username,
				Action:   "CONFIG_APPLY",
				Target:   nodeID,
				Details:  err.Error(),
				ClientIP: r.RemoteAddr,
				Status:   "FAILURE",
			})
			s.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":         err.Error(),
				"error_message": err.Error(),
				"result":        res,
			})
			return
		}

		// Apply Host Logrotate Policy if configured
		if state.Logrotate.Enabled {
			if _, lErr := s.logrotateManager.Apply(r.Context(), state.Logrotate); lErr != nil {
				log.Printf("[apply] Node %s: Warning: Logrotate apply encountered issue: %v\n", nodeID, lErr)
			}
		}

		if err := s.store.SaveDesiredState(r.Context(), &state); err != nil {
			log.Printf("[apply] Node %s: SaveDesiredState failed: %v\n", nodeID, err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error": fmt.Sprintf("Applied to Unbound, but failed to save to database: %v", err),
			})
			return
		}

		// Record config version in history for NOC rollback
		now := time.Now().UTC()
		_ = s.store.SaveConfigVersion(r.Context(), &model.ConfigVersion{
			NodeID:    nodeID,
			VersionID: state.VersionID,
			Checksum:  fmt.Sprintf("sha256-%d", now.Unix()),
			Actor:     claims.Username,
			Reason:    "Applied via REST API / Web UI",
			Status:    model.StatusActive,
			AppliedAt: &now,
			CreatedAt: now,
		})

		// Sync core platform filters in database to keep status consistent with Master Cards
		if filters, err := s.store.ListFilters(r.Context()); err == nil {
			for _, f := range filters {
				switch f.ID {
				case "feed-adult":
					active := state.ParentalControl.Enabled && state.ParentalControl.BlockAdult
					if f.Enabled != active {
						f.Enabled = active
						_ = s.store.UpsertFilter(r.Context(), &f)
					}
				case "feed-gambling":
					active := state.GamblingBlock.Enabled || (state.ParentalControl.Enabled && state.ParentalControl.BlockGambling)
					if f.Enabled != active {
						f.Enabled = active
						_ = s.store.UpsertFilter(r.Context(), &f)
					}
				}
			}
		}

		_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
			Actor:    claims.Username,
			Action:   "CONFIG_APPLY",
			Target:   nodeID,
			Details:  fmt.Sprintf("Applied version %s in %d ms", res.VersionID, res.DurationMs),
			ClientIP: r.RemoteAddr,
			Status:   "SUCCESS",
		})
		s.writeJSON(w, http.StatusOK, res)

	case "rollback":
		if r.Method != http.MethodPost {
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req struct {
			TargetVersionID string `json:"target_version_id"`
			VersionID       string `json:"version_id"`
			Reason          string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}

		targetVer := req.TargetVersionID
		if targetVer == "" {
			targetVer = req.VersionID
		}

		claims := r.Context().Value(userClaimsKey).(*auth.Claims)
		res, err := s.engine.Rollback(r.Context(), targetVer, claims.Username, req.Reason)
		if err != nil {
			_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
				Actor:    claims.Username,
				Action:   "CONFIG_ROLLBACK",
				Target:   nodeID,
				Details:  err.Error(),
				ClientIP: r.RemoteAddr,
				Status:   "FAILURE",
			})
			s.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":         err.Error(),
				"error_message": err.Error(),
				"result":        res,
			})
			return
		}
		s.writeJSON(w, http.StatusOK, res)

	case "history", "versions":
		versions, err := s.store.ListConfigVersions(r.Context(), nodeID, 50)
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, versions)

	case "logrotate":
		s.handleLogrotate(w, r, nodeID, parts)
		return

	default:
		s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
	}
}

// handleLogrotate manages host log rotation inspection, testing, and application.
func (s *Server) handleLogrotate(w http.ResponseWriter, r *http.Request, nodeID string, parts []string) {
	subAction := ""
	if len(parts) > 2 {
		subAction = parts[2]
	}

	switch subAction {
	case "apply":
		if r.Method != http.MethodPost {
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var policy model.LogrotateConfig
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
			return
		}
		res, err := s.logrotateManager.Apply(r.Context(), policy)
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		// Save into current desired state if present
		if state, err := s.store.GetDesiredState(r.Context(), nodeID); err == nil && state != nil {
			state.Logrotate = policy
			_ = s.store.SaveDesiredState(r.Context(), state)
		}
		claims, _ := r.Context().Value(userClaimsKey).(*auth.Claims)
		actor := "system"
		if claims != nil {
			actor = claims.Username
		}
		_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
			Actor:    actor,
			Action:   "LOGROTATE_APPLY",
			Target:   nodeID,
			Details:  fmt.Sprintf("Applied logrotate: %s, rotate %d, maxsize %s", policy.Frequency, policy.RotateCount, policy.MaxSize),
			ClientIP: r.RemoteAddr,
			Status:   "SUCCESS",
		})
		s.writeJSON(w, http.StatusOK, res)

	case "test":
		if r.Method != http.MethodPost {
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var policy *model.LogrotateConfig
		var bodyPolicy model.LogrotateConfig
		if err := json.NewDecoder(r.Body).Decode(&bodyPolicy); err == nil && bodyPolicy.Frequency != "" {
			policy = &bodyPolicy
		}
		out, err := s.logrotateManager.Test(r.Context(), policy)
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": err == nil,
			"output":  out,
			"error":   errStr,
		})

	case "force-rotate":
		if r.Method != http.MethodPost {
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		out, err := s.logrotateManager.ForceRotate(r.Context())
		claims, _ := r.Context().Value(userClaimsKey).(*auth.Claims)
		actor := "system"
		if claims != nil {
			actor = claims.Username
		}
		_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
			Actor:    actor,
			Action:   "LOGROTATE_FORCE",
			Target:   nodeID,
			Details:  "Triggered immediate manual logrotate",
			ClientIP: r.RemoteAddr,
			Status:   "SUCCESS",
		})
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": err == nil,
			"output":  out,
			"error":   errStr,
			"message": "Rotasi manual logrotate selesai dijalankan",
		})

	default: // GET report
		if r.Method != http.MethodGet {
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		// Retrieve hardware specs of node
		node, _ := s.store.GetNode(r.Context(), nodeID)
		cores := 2
		ramMB := 2048
		diskTotalGB := 0
		diskFreeGB := 0
		if node != nil {
			cores = node.CPUCores
			ramMB = node.RAMMB
			diskTotalGB = node.DiskTotalGB
			diskFreeGB = node.DiskFreeGB
		}
		// The DB value is captured once at startup; for the local node use the live root fs
		// so the AI policy reflects the real disk state on every request.
		if nodeID == s.nodeID {
			if liveTotal, liveFree := detectDiskSpecs(); liveTotal > 0 {
				diskTotalGB, diskFreeGB = liveTotal, liveFree
			}
		}
		report, err := s.logrotateManager.GetReport(r.Context(), nodeID, cores, ramMB, diskTotalGB, diskFreeGB)
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, report)
	}
}
