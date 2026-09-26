package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/blocklist"
	"netora-dns/pkg/cluster"
	"netora-dns/pkg/configengine"
	"netora-dns/pkg/hermes"
	"netora-dns/pkg/model"
	"netora-dns/pkg/policy"
	"netora-dns/pkg/runtime"
	"netora-dns/pkg/storage"
	"netora-dns/pkg/telemetry"
)

// Server encapsulates the Netora REST API dependencies.
type Server struct {
	store            storage.Store
	engine           *configengine.Engine
	policyEngine     *policy.PolicyEngine
	blocklistManager *blocklist.Manager
	healthChecker    *runtime.DNSHealthChecker
	jwtSecret        string
	nodeID           string
	metrics          *telemetry.Metrics
	alertEngine      *telemetry.AlertEngine
	clusterManager   *cluster.ClusterManager
	investigator     *hermes.Investigator
	queryLogTailer   *telemetry.QueryLogTailer
}

// NewServer initializes the API server components.
func NewServer(store storage.Store, engine *configengine.Engine, policyEngine *policy.PolicyEngine, jwtSecret, nodeID string) *Server {
	if policyEngine == nil {
		policyEngine = policy.NewPolicyEngine()
	}
	metrics := telemetry.DefaultMetrics
	alertEngine := telemetry.NewAlertEngine(nodeID, store, metrics)
	cm := cluster.NewClusterManager(store)
	cm.RegisterNode(&cluster.ClusterNode{
		ID:           nodeID,
		Hostname:     nodeID,
		ManagementIP: "172.18.20.145",
		Role:         cluster.RolePrimary,
		State:        cluster.StateOnline,
		SyncStatus:   cluster.SyncSynchronized,
	})

	bm := blocklist.NewManager(blocklist.ManagerOptions{
		BlocklistDir: "/etc/unbound/blocklist",
		ConfDDir:     "/etc/unbound/conf.d",
		Interval:     12 * time.Hour,
		Store:        store,
		Engine:       engine,
		NodeID:       nodeID,
		PolicyEngine: policyEngine,
	})

	hc := runtime.NewDNSHealthChecker(3 * time.Second)
	inv := hermes.NewInvestigator(nodeID, policyEngine, hc)

	tailer := telemetry.DefaultTailer
	if tailer == nil {
		tailer = telemetry.NewQueryLogTailer("/var/log/unbound/unbound.log")
	}

	return &Server{
		store:            store,
		engine:           engine,
		policyEngine:     policyEngine,
		blocklistManager: bm,
		healthChecker:    hc,
		jwtSecret:        jwtSecret,
		nodeID:           nodeID,
		metrics:          metrics,
		alertEngine:      alertEngine,
		clusterManager:   cm,
		investigator:     inv,
		queryLogTailer:   tailer,
	}
}

// RegisterRoutes registers all REST API endpoints on the multiplexer.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Health & Probe
	mux.HandleFunc("/api/v1/health", s.handleSystemHealth)
	mux.HandleFunc("/api/v1/system/version", s.handleSystemVersion)

	// Prometheus Telemetry
	mux.HandleFunc("/metrics", s.metrics.PrometheusHandler())

	// Operational Alerts
	mux.HandleFunc("/api/v1/alerts", s.requireAuth(s.handleAlerts))

	// Diagnostics & Explainability
	mux.HandleFunc("/api/v1/diagnose", s.requireAuth(s.handleDiagnose))

	// Hermes AI-NOC Deep Diagnostic Investigation
	mux.HandleFunc("/api/v1/hermes/investigate", s.requireAuth(s.handleHermesInvestigate))

	// Auth Endpoints
	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/api/v1/auth/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("/api/v1/auth/profile", s.requireAuth(s.handleUpdateProfile))

	// Node Management
	mux.HandleFunc("/api/v1/nodes", s.requireAuth(s.handleNodes))
	mux.HandleFunc("/api/v1/nodes/", s.requireAuth(s.handleNodeSubroutes))

	// Cluster & Multi-Node HA
	mux.HandleFunc("/api/v1/cluster/status", s.requireAuth(s.handleClusterStatus))
	mux.HandleFunc("/api/v1/cluster/heartbeat", s.handleClusterHeartbeat)
	mux.HandleFunc("/api/v1/cluster/sync", s.requireAuth(s.handleClusterSync))

	// Policies & Filtering
	mux.HandleFunc("/api/v1/policies", s.requireAuth(s.handlePolicies))
	mux.HandleFunc("/api/v1/filters", s.requireAuth(s.handleFilters))
	mux.HandleFunc("/api/v1/filters/", s.requireAuth(s.handleFilterSubroutes))
	mux.HandleFunc("/api/v1/blocklists/sync", s.requireAuth(s.handleSyncBlocklists))
	mux.HandleFunc("/api/v1/blocklists/auto-update-status", s.requireAuth(s.handleBlocklistAutoUpdateStatus))
	mux.HandleFunc("/api/v1/allowlist", s.requireAuth(s.handleAllowlist))
	mux.HandleFunc("/api/v1/allowlist/", s.requireAuth(s.handleAllowlistDelete))

	// Audit Logs
	mux.HandleFunc("/api/v1/audit", s.requireAuth(s.handleAuditLogs))

	// Static Web UI (SPA)
	webDir := "/var/www/netora-ui"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		webDir = "web/dist"
	}
	if _, err := os.Stat(webDir); err == nil {
		fileServer := http.FileServer(http.Dir(webDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			filePath := filepath.Join(webDir, filepath.Clean(r.URL.Path))
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
		})
	}
}

// Middleware: CORS
func (s *Server) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Middleware: Authentication
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := auth.ValidateToken(tokenStr, s.jwtSecret)
		if err != nil {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": fmt.Sprintf("unauthorized: %v", err)})
			return
		}

		// Inject user claims into request context
		ctx := context.WithValue(r.Context(), "user_claims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

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

// Handler: System Version
func (s *Server) handleSystemVersion(w http.ResponseWriter, r *http.Request) {
	versionInfo := map[string]interface{}{
		"version":      "v1.0.44",
		"commit_count": 44,
		"build":        44,
		"git_commit":   "de28649",
		"git_branch":   "main",
		"build_time":   time.Now().UTC().Format(time.RFC3339),
		"channel":      "production",
		"node_id":      s.nodeID,
		"status":       "SUCCESS",
	}

	versionCandidates := []string{
		"/var/www/netora-ui/version.json",
		"/opt/netora-dns/current/version.json",
		"/var/lib/netora-dns/version.json",
		"version.json",
		"web/src/version.json",
	}

	for _, cand := range versionCandidates {
		if data, err := os.ReadFile(cand); err == nil {
			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err == nil {
				for k, v := range parsed {
					versionInfo[k] = v
				}
				break
			}
		}
	}

	// Also check /var/lib/netora-dns/update-state.json for live node deployment state
	if data, err := os.ReadFile("/var/lib/netora-dns/update-state.json"); err == nil {
		var updateState struct {
			CurrentRevision string `json:"current_revision"`
			Channel         string `json:"channel"`
			UpdateTime      string `json:"update_time"`
			UpdateStatus    string `json:"update_status"`
		}
		if err := json.Unmarshal(data, &updateState); err == nil && updateState.CurrentRevision != "" {
			versionInfo["git_commit"] = updateState.CurrentRevision
			if len(updateState.CurrentRevision) >= 7 {
				versionInfo["short_commit"] = updateState.CurrentRevision[:7]
			}
			if updateState.Channel != "" {
				versionInfo["channel"] = updateState.Channel
			}
			if updateState.UpdateTime != "" {
				versionInfo["update_time"] = updateState.UpdateTime
			}
		}
	}

	s.writeJSON(w, http.StatusOK, versionInfo)
}

// Handler: Login
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	user, err := s.store.GetUserByUsername(r.Context(), req.Username)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}

	token, err := auth.GenerateToken(user, s.jwtSecret, 24*time.Hour)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
		Actor:    user.Username,
		Action:   "USER_LOGIN",
		Target:   user.ID,
		ClientIP: r.RemoteAddr,
		Status:   "SUCCESS",
	})

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"id":        user.ID,
			"username":  user.Username,
			"role":      user.Role,
			"full_name": user.FullName,
		},
	})
}

// Handler: Me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("user_claims").(*auth.Claims)
	if !ok {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.writeJSON(w, http.StatusOK, claims)
}

type updateProfileRequest struct {
	Username        string `json:"username"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Handler: Update Profile / Credentials (Username & Password)
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	claims, ok := r.Context().Value("user_claims").(*auth.Claims)
	if !ok {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	// 1. Fetch current user record
	user, err := s.store.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		user, err = s.store.GetUserByUsername(r.Context(), claims.Username)
		if err != nil {
			s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "user record not found"})
			return
		}
	}

	// 2. Validate current password
	if strings.TrimSpace(req.CurrentPassword) == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Password saat ini wajib diisi untuk verifikasi keamanan"})
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Password saat ini salah"})
		return
	}

	// 3. Validate new username if provided
	targetUsername := strings.TrimSpace(req.Username)
	if targetUsername == "" {
		targetUsername = user.Username
	}
	if len(targetUsername) < 3 {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Username minimal harus 3 karakter"})
		return
	}
	if targetUsername != user.Username {
		existing, _ := s.store.GetUserByUsername(r.Context(), targetUsername)
		if existing != nil && existing.ID != user.ID {
			s.writeJSON(w, http.StatusConflict, map[string]string{"error": "Username '" + targetUsername + "' sudah digunakan oleh akun lain"})
			return
		}
	}

	// 4. Validate new password if provided
	targetPasswordHash := user.PasswordHash
	passwordChanged := false
	if strings.TrimSpace(req.NewPassword) != "" {
		if len(req.NewPassword) < 6 {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Password baru minimal harus 6 karakter"})
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mengenkripsi password baru"})
			return
		}
		targetPasswordHash = hash
		passwordChanged = true
	}

	// 5. Update user credentials in database
	if err := s.store.UpdateUserCredentials(r.Context(), user.ID, targetUsername, targetPasswordHash); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal memperbarui akun: " + err.Error()})
		return
	}

	// 6. Record Audit Log
	details := fmt.Sprintf("username updated to '%s'", targetUsername)
	if passwordChanged {
		details += ", password changed"
	}
	_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
		Actor:    user.Username,
		Action:   "USER_CREDENTIALS_UPDATED",
		Target:   user.ID,
		Details:  details,
		ClientIP: r.RemoteAddr,
		Status:   "SUCCESS",
	})

	// 7. Issue refreshed token with new username
	user.Username = targetUsername
	user.PasswordHash = targetPasswordHash
	newToken, _ := auth.GenerateToken(user, s.jwtSecret, 24*time.Hour)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pengaturan akun berhasil disimpan",
		"token":   newToken,
		"user": map[string]string{
			"id":        user.ID,
			"username":  user.Username,
			"role":      user.Role,
			"full_name": user.FullName,
		},
	})
}

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

		claims := r.Context().Value("user_claims").(*auth.Claims)
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
			s.writeJSON(w, http.StatusInternalServerError, res)
			return
		}

		log.Printf("[apply] Node %s: apply succeeded! Version %s active in %dms\n", nodeID, res.VersionID, res.DurationMs)
		_ = s.store.SaveDesiredState(r.Context(), &state)

		// Sync core platform filters in database to keep status consistent with Master Cards
		if filters, err := s.store.ListFilters(r.Context()); err == nil {
			for _, f := range filters {
				if f.ID == "feed-adult" {
					active := state.ParentalControl.Enabled && state.ParentalControl.BlockAdult
					if f.Enabled != active {
						f.Enabled = active
						_ = s.store.UpsertFilter(r.Context(), &f)
					}
				} else if f.ID == "feed-gambling" {
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

		claims := r.Context().Value("user_claims").(*auth.Claims)
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
			s.writeJSON(w, http.StatusInternalServerError, res)
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


	default:
		s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
	}
}

// Handler: Policies
func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		policies, err := s.store.ListPolicies(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, policies)
		return
	}
	if r.Method == http.MethodPost {
		var p storage.PolicyRecord
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("pol-%d", time.Now().UnixNano())
		}
		if err := s.store.UpsertPolicy(r.Context(), &p); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, p)
		return
	}
	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

// Handler: Filters
func (s *Server) handleFilters(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		filters, err := s.store.ListFilters(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if filters == nil {
			filters = []storage.FilterRecord{}
		}
		canonicalFeeds := []storage.FilterRecord{
			{
				ID:        "feed-adult",
				Name:      "Parental Control (18+ Dewasa)",
				Category:  "ADULT",
				SourceURL: "adult-domains.txt",
				LocalPath: "/etc/unbound/blocklist/adult-domains.txt",
				RuleCount: 46466,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-gambling",
				Name:      "Judi Online & Slot Gacor",
				Category:  "GAMBLING",
				SourceURL: "gambling-domains.txt",
				LocalPath: "/etc/unbound/blocklist/gambling-domains.txt",
				RuleCount: 83549,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-ads",
				Name:      "Iklan, Pop-up & Pelacak",
				Category:  "ADS_TRACKING",
				SourceURL: "ads.txt",
				LocalPath: "/etc/unbound/blocklist/ads.txt",
				RuleCount: 52100,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-malware",
				Name:      "Malware, Phishing & Botnet Shield",
				Category:  "MALWARE_ADS",
				SourceURL: "malware.txt",
				LocalPath: "/etc/unbound/blocklist/malware.txt",
				RuleCount: 71800,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-custom",
				Name:      "Custom ISP Blacklist",
				Category:  "CUSTOM",
				SourceURL: "custom.txt",
				LocalPath: "/etc/unbound/blocklist/custom.txt",
				RuleCount: 15,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-hagezi",
				Name:      "Hagezi Multi-PRO Threat Feed",
				Category:  "MALWARE_ADS",
				SourceURL: "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/adblock/pro.txt",
				LocalPath: "/var/lib/netora/filters/hagezi-pro.txt",
				RuleCount: 142300,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-adguard",
				Name:      "AdGuard Base DNS Filter",
				Category:  "ADS_TRACKING",
				SourceURL: "https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt",
				LocalPath: "/var/lib/netora/filters/adguard.txt",
				RuleCount: 48900,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-stevenblack",
				Name:      "StevenBlack Unified Hosts File",
				Category:  "HOSTS_BLOCK",
				SourceURL: "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
				LocalPath: "/etc/unbound/blocklists/stevenblack.hosts",
				RuleCount: 73200,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
		}

		existingIDs := make(map[string]bool)
		for _, f := range filters {
			existingIDs[f.ID] = true
		}

		hasNew := false
		for i := range canonicalFeeds {
			if !existingIDs[canonicalFeeds[i].ID] {
				_ = s.store.UpsertFilter(r.Context(), &canonicalFeeds[i])
				filters = append(filters, canonicalFeeds[i])
				hasNew = true
			}
		}
		if hasNew {
			if refetched, err := s.store.ListFilters(r.Context()); err == nil {
				filters = refetched
			}
		}
		s.writeJSON(w, http.StatusOK, filters)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Name      string `json:"name"`
			Category  string `json:"category"`
			SourceURL string `json:"source_url"`
			LocalPath string `json:"local_path"`
			Enabled   *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Name == "" || (req.SourceURL == "" && req.LocalPath == "") {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and source_url or local_path are required"})
			return
		}
		id := fmt.Sprintf("filter-%d", time.Now().UnixNano())
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		f := &storage.FilterRecord{
			ID:        id,
			Name:      req.Name,
			Category:  req.Category,
			SourceURL: req.SourceURL,
			LocalPath: req.LocalPath,
			RuleCount: 0,
			Enabled:   enabled,
			CreatedAt: time.Now().UTC(),
		}
		if err := s.store.UpsertFilter(r.Context(), f); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, f)
		return
	}
	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

// Handler: Filter Subroutes (/api/v1/filters/{id})
func (s *Server) handleFilterSubroutes(w http.ResponseWriter, r *http.Request) {
	subPath := strings.TrimPrefix(r.URL.Path, "/api/v1/filters/")
	parts := strings.Split(subPath, "/")
	id := parts[0]
	if id == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if r.Method == http.MethodDelete {
		if err := s.store.DeleteFilter(r.Context(), id); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED", "id": id})
		return
	}

	if r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodPost {
		filters, err := s.store.ListFilters(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		var target *storage.FilterRecord
		for _, f := range filters {
			if f.ID == id {
				cp := f
				target = &cp
				break
			}
		}

		var req struct {
			Enabled   *bool   `json:"enabled"`
			Name      *string `json:"name"`
			Category  *string `json:"category"`
			SourceURL *string `json:"source_url"`
			LocalPath *string `json:"local_path"`
			RuleCount *int    `json:"rule_count"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if target == nil {
			target = &storage.FilterRecord{
				ID:        id,
				Name:      id,
				Category:  "BLOCKLIST",
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			}
		}

		if req.Enabled != nil {
			target.Enabled = *req.Enabled
		} else if len(parts) > 1 && parts[1] == "toggle" {
			target.Enabled = !target.Enabled
		}

		if req.Name != nil && *req.Name != "" {
			target.Name = *req.Name
		}
		if req.Category != nil && *req.Category != "" {
			target.Category = *req.Category
		}
		if req.SourceURL != nil {
			target.SourceURL = *req.SourceURL
		}
		if req.LocalPath != nil {
			target.LocalPath = *req.LocalPath
		}
		if req.RuleCount != nil {
			target.RuleCount = *req.RuleCount
		}

		now := time.Now().UTC()
		target.LastUpdatedAt = &now

		if err := s.store.UpsertFilter(r.Context(), target); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Dynamically re-apply filter configuration to Unbound
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			s.reapplyFilterState(bgCtx, "Filter Toggled: "+target.Name)
		}()

		s.writeJSON(w, http.StatusOK, target)
		return
	}

	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

// Handler: Sync All Blocklists (/api/v1/blocklists/sync)
func (s *Server) handleSyncBlocklists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	actor := "Manual NOC Trigger"
	if claims, ok := r.Context().Value("user_claims").(*auth.Claims); ok && claims != nil {
		actor = claims.Username
	}

	report, err := s.blocklistManager.SyncAll(r.Context(), actor)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	adultCount := report.CategoryCounts["ADULT"]
	gamblingCount := report.CategoryCounts["GAMBLING"]

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":                report.Success,
		"adult_domains_count":    adultCount,
		"gambling_domains_count": gamblingCount,
		"category_counts":        report.CategoryCounts,
		"total_domains":          report.TotalDomains,
		"duration_ms":            report.DurationMs,
		"timestamp":              report.Timestamp,
		"reloaded":               report.Reloaded,
		"message":                report.Message,
	})
}

// Handler: Auto-Update Status (/api/v1/blocklists/auto-update-status)
func (s *Server) handleBlocklistAutoUpdateStatus(w http.ResponseWriter, r *http.Request) {
	status := s.blocklistManager.GetStatus()
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) reapplyFilterState(ctx context.Context, reason string) {
	if s.engine == nil || s.store == nil {
		return
	}
	cfg, err := s.store.GetDesiredState(ctx, s.nodeID)
	if err != nil || cfg == nil {
		return
	}

	filters, err := s.store.ListFilters(ctx)
	if err == nil {
		for _, f := range filters {
			switch f.ID {
			case "feed-adult":
				cfg.ParentalControl.Enabled = f.Enabled
				cfg.ParentalControl.BlockAdult = f.Enabled
			case "feed-gambling":
				cfg.GamblingBlock.Enabled = f.Enabled
			case "feed-ads", "feed-adguard":
				cfg.AdBlock.Enabled = f.Enabled
			case "feed-malware", "feed-hagezi":
				cfg.MalwareBlock.Enabled = f.Enabled
			case "feed-custom":
				cfg.CustomBlock.Enabled = f.Enabled
			}
		}
	}

	cfg.VersionID = fmt.Sprintf("v-filter-toggle-%d", time.Now().Unix())
	if _, err := s.engine.Apply(ctx, cfg, "NOC Admin", reason); err == nil {
		_ = s.store.SaveDesiredState(ctx, cfg)
	}
}

// Handler: Allowlist
func (s *Server) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		al, err := s.store.ListAllowlist(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, al)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Domain   string `json:"domain"`
			Category string `json:"category"`
			Reason   string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		claims := r.Context().Value("user_claims").(*auth.Claims)
		if err := s.store.AddAllowlist(r.Context(), req.Domain, req.Category, req.Reason, claims.Username); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "ADDED", "domain": req.Domain})
		return
	}
	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

// Handler: Allowlist Delete
func (s *Server) handleAllowlistDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	domain := strings.TrimPrefix(r.URL.Path, "/api/v1/allowlist/")
	if domain == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain is required"})
		return
	}
	if err := s.store.RemoveAllowlist(r.Context(), domain); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED", "domain": domain})
}

// Handler: Audit Logs
func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.ListAuditLogs(r.Context(), 100)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
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
