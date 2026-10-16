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
	"sync"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/backup"
	"netora-dns/pkg/blocklist"
	"netora-dns/pkg/cluster"
	"netora-dns/pkg/configengine"
	"netora-dns/pkg/hermes"
	"netora-dns/pkg/logrotate"
	"netora-dns/pkg/model"
	"netora-dns/pkg/policy"
	"netora-dns/pkg/runtime"
	"netora-dns/pkg/storage"
	"netora-dns/pkg/telemetry"
)

// Server encapsulates the Netora REST API dependencies.
type Server struct {
	store                 storage.Store
	engine                *configengine.Engine
	policyEngine          *policy.PolicyEngine
	blocklistManager      *blocklist.Manager
	healthChecker         *runtime.DNSHealthChecker
	jwtSecret             string
	nodeID                string
	mgmtIP                string
	metrics               *telemetry.Metrics
	alertEngine           *telemetry.AlertEngine
	clusterManager        *cluster.ClusterManager
	investigator          *hermes.Investigator
	queryLogTailer        *telemetry.QueryLogTailer
	logrotateManager      *logrotate.Manager
	databaseBackupManager *backup.DatabaseManager

	updateMu       sync.Mutex
	isUpdating     bool
	lastUpdateLog  string
	lastUpdateErr  string
	lastUpdateTime time.Time
}

// NewServer initializes the API server components.
func NewServer(store storage.Store, engine *configengine.Engine, policyEngine *policy.PolicyEngine, jwtSecret, nodeID string, optionalMgmtIP ...string) *Server {
	if policyEngine == nil {
		policyEngine = policy.NewPolicyEngine()
	}
	mgmtIP := detectManagementIP()
	if len(optionalMgmtIP) > 0 && optionalMgmtIP[0] != "" {
		mgmtIP = optionalMgmtIP[0]
	}

	metrics := telemetry.DefaultMetrics
	alertEngine := telemetry.NewAlertEngine(nodeID, store, metrics)
	cm := cluster.NewClusterManager(store)
	cm.RegisterNode(&cluster.ClusterNode{
		ID:           nodeID,
		Hostname:     nodeID,
		ManagementIP: mgmtIP,
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

	dbBackupMgr, _ := backup.NewDatabaseManager("/var/backups/netora-dns/database", "", nodeID)

	return &Server{
		store:                 store,
		engine:                engine,
		policyEngine:          policyEngine,
		blocklistManager:      bm,
		healthChecker:         hc,
		jwtSecret:             jwtSecret,
		nodeID:                nodeID,
		mgmtIP:                mgmtIP,
		metrics:               metrics,
		alertEngine:           alertEngine,
		clusterManager:        cm,
		investigator:          inv,
		queryLogTailer:        tailer,
		logrotateManager:      logrotate.NewManager(),
		databaseBackupManager: dbBackupMgr,
	}
}

// SetDatabaseURL configures the real PostgreSQL database connection string for disaster recovery backups.
func (s *Server) SetDatabaseURL(dbURL string) {
	if dbURL != "" {
		mgr, err := backup.NewDatabaseManager("/var/backups/netora-dns/database", dbURL, s.nodeID)
		if err == nil {
			s.databaseBackupManager = mgr
		}
	}
}

// RegisterRoutes registers all REST API endpoints on the multiplexer.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Health & Probe
	mux.HandleFunc("/api/v1/health", s.handleSystemHealth)
	mux.HandleFunc("/api/v1/system/version", s.handleSystemVersion)
	mux.HandleFunc("/api/v1/system/update", s.requireAuth(s.handleSystemUpdate))
	mux.HandleFunc("/api/v1/system/update/status", s.handleSystemUpdateStatus)

	// Database Disaster Recovery & Snapshots
	mux.HandleFunc("/api/v1/system/backups/database", s.requireAuth(s.handleDatabaseBackups))
	mux.HandleFunc("/api/v1/system/backups/database/", s.requireAuth(s.handleDatabaseBackupOperations))

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
	mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)
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
		tokenStr := ""
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else if qToken := r.URL.Query().Get("token"); qToken != "" {
			tokenStr = qToken
		}

		if tokenStr == "" {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid authorization header"})
			return
		}

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

// Handler: System Update Status (/api/v1/system/update/status)
func (s *Server) handleSystemUpdateStatus(w http.ResponseWriter, r *http.Request) {
	s.updateMu.Lock()
	isUpdating := s.isUpdating
	lastErr := s.lastUpdateErr
	lastTime := s.lastUpdateTime
	s.updateMu.Unlock()

	statusInfo := map[string]interface{}{
		"version":          "v1.0.48",
		"commit_count":     48,
		"build":            48,
		"git_commit":       "bc2988f",
		"git_branch":       "main",
		"node_id":          s.nodeID,
		"is_updating":      isUpdating,
		"last_error":       lastErr,
		"update_available": false,
	}

	if !lastTime.IsZero() {
		statusInfo["last_update_time"] = lastTime.Format(time.RFC3339)
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
					statusInfo[k] = v
				}
				break
			}
		}
	}

	if data, err := os.ReadFile("/var/lib/netora-dns/update-state.json"); err == nil {
		var updateState struct {
			CurrentRevision string `json:"current_revision"`
			Channel         string `json:"channel"`
			UpdateTime      string `json:"update_time"`
			UpdateStatus    string `json:"update_status"`
			LastError       string `json:"last_error"`
		}
		if err := json.Unmarshal(data, &updateState); err == nil {
			if updateState.CurrentRevision != "" {
				statusInfo["git_commit"] = updateState.CurrentRevision
				if len(updateState.CurrentRevision) >= 7 {
					statusInfo["short_commit"] = updateState.CurrentRevision[:7]
				}
			}
			if updateState.UpdateStatus != "" {
				statusInfo["last_status"] = updateState.UpdateStatus
			}
			if updateState.UpdateTime != "" {
				statusInfo["last_update_time"] = updateState.UpdateTime
			}
			if updateState.LastError != "" && updateState.LastError != "none" {
				statusInfo["last_error"] = updateState.LastError
			}
		}
	}

	// Fast remote check via git ls-remote if requested
	if r.URL.Query().Get("check_remote") == "true" {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		repoURL := buildAuthenticatedRepoURL("https://github.com/desienkz-slp/unbound.git", "/etc/netora-dns/github.token")
		cmd := exec.CommandContext(ctx, "git", "ls-remote", repoURL, "refs/heads/main")
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.Output(); err == nil {
			fields := strings.Fields(string(out))
			if len(fields) > 0 {
				remoteSHA := fields[0]
				statusInfo["remote_commit"] = remoteSHA
				currSHA, _ := statusInfo["git_commit"].(string)
				if currSHA != "" && remoteSHA != "" {
					if !strings.HasPrefix(remoteSHA, currSHA) && !strings.HasPrefix(currSHA, remoteSHA) {
						statusInfo["update_available"] = true
					}
				}
			}
		}
	}

	s.writeJSON(w, http.StatusOK, statusInfo)
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
			log.Printf("[UPDATER] Update failed: %v, output: %s", err, string(out))
		} else {
			s.lastUpdateErr = ""
			log.Printf("[UPDATER] Update completed successfully: %s", string(out))
		}
		s.updateMu.Unlock()
	}()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "IN_PROGRESS",
		"message": "Autonomous system update initiated. Node pulling from GitHub...",
		"node_id": s.nodeID,
	})
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

// Handler: Logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	reason := "MANUAL"
	if req.Reason != "" {
		reason = req.Reason
	}

	tokenStr := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
	} else if qToken := r.URL.Query().Get("token"); qToken != "" {
		tokenStr = qToken
	}

	if tokenStr != "" {
		if claims, err := auth.ValidateToken(tokenStr, s.jwtSecret); err == nil && claims != nil {
			_ = s.store.LogAudit(r.Context(), &model.AuditRecord{
				Actor:    claims.Username,
				Action:   "USER_LOGOUT",
				Target:   claims.UserID,
				ClientIP: r.RemoteAddr,
				Status:   reason,
			})
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
		"reason":  reason,
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
		claims, _ := r.Context().Value("user_claims").(*auth.Claims)
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
		claims, _ := r.Context().Value("user_claims").(*auth.Claims)
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
		if al == nil {
			al = []storage.AllowlistRecord{}
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

		claims, _ := r.Context().Value("user_claims").(*auth.Claims)
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

		claims, _ := r.Context().Value("user_claims").(*auth.Claims)
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

			claims, _ := r.Context().Value("user_claims").(*auth.Claims)
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

