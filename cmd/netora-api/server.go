package main

import (
	"context"
	"log"
	"net/http"
	"os"
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
// Authorization: requirePerm(read, write, h) enforces `read` on GET/HEAD and `write` otherwise.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Health & Probe
	mux.HandleFunc("/api/v1/health", s.handleSystemHealth)
	mux.HandleFunc("/api/v1/system/version", s.handleSystemVersion)
	mux.HandleFunc("/api/v1/system/update", s.requirePerm("nodes:write", "nodes:write", s.handleSystemUpdate))
	mux.HandleFunc("/api/v1/system/update/status", s.handleSystemUpdateStatus)

	// Database Disaster Recovery & Snapshots (admin-level: restore/delete are destructive)
	mux.HandleFunc("/api/v1/system/backups/database", s.requirePerm("nodes:write", "nodes:write", s.handleDatabaseBackups))
	mux.HandleFunc("/api/v1/system/backups/database/", s.requirePerm("nodes:write", "nodes:write", s.handleDatabaseBackupOperations))

	// Prometheus Telemetry
	mux.HandleFunc("/metrics", s.metrics.PrometheusHandler())

	// Operational Alerts
	mux.HandleFunc("/api/v1/alerts", s.requirePerm("nodes:read", "nodes:read", s.handleAlerts))

	// Diagnostics & Explainability
	mux.HandleFunc("/api/v1/diagnose", s.requirePerm("diagnostics:execute", "diagnostics:execute", s.handleDiagnose))

	// Hermes AI-NOC Deep Diagnostic Investigation
	mux.HandleFunc("/api/v1/hermes/investigate", s.requirePerm("diagnostics:execute", "diagnostics:execute", s.handleHermesInvestigate))

	// Auth Endpoints (any authenticated user may manage own session/profile)
	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("/api/v1/auth/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("/api/v1/auth/profile", s.requireAuth(s.handleUpdateProfile))

	// Node Management (apply/rollback/logrotate are checked per-action inside the handler)
	mux.HandleFunc("/api/v1/nodes", s.requirePerm("nodes:read", "nodes:write", s.handleNodes))
	mux.HandleFunc("/api/v1/nodes/", s.requirePerm("nodes:read", "config:apply", s.handleNodeSubroutes))

	// Cluster & Multi-Node HA
	mux.HandleFunc("/api/v1/cluster/status", s.requirePerm("nodes:read", "nodes:read", s.handleClusterStatus))
	mux.HandleFunc("/api/v1/cluster/heartbeat", s.handleClusterHeartbeat)
	mux.HandleFunc("/api/v1/cluster/sync", s.requirePerm("config:apply", "config:apply", s.handleClusterSync))

	// Policies & Filtering
	mux.HandleFunc("/api/v1/policies", s.requirePerm("policy:read", "policy:write", s.handlePolicies))
	mux.HandleFunc("/api/v1/filters", s.requirePerm("policy:read", "policy:write", s.handleFilters))
	mux.HandleFunc("/api/v1/filters/", s.requirePerm("policy:read", "policy:write", s.handleFilterSubroutes))
	mux.HandleFunc("/api/v1/blocklists/sync", s.requirePerm("policy:write", "policy:write", s.handleSyncBlocklists))
	mux.HandleFunc("/api/v1/blocklists/auto-update-status", s.requirePerm("policy:read", "policy:read", s.handleBlocklistAutoUpdateStatus))
	mux.HandleFunc("/api/v1/allowlist", s.requirePerm("policy:read", "policy:write", s.handleAllowlist))
	mux.HandleFunc("/api/v1/allowlist/", s.requirePerm("policy:write", "policy:write", s.handleAllowlistDelete))

	// Audit Logs
	mux.HandleFunc("/api/v1/audit", s.requirePerm("audit:read", "audit:read", s.handleAuditLogs))

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

// ctxKey is a private type so context keys can never collide with other packages.
type ctxKey struct{ name string }

var userClaimsKey = ctxKey{"user_claims"}

// dummyPasswordHash is a valid bcrypt hash (same cost as real ones) compared against
// when a login names an unknown user, so both paths take equal time.
var dummyPasswordHash, _ = auth.HashPassword("netora-timing-equalizer")

// maxRequestBodyBytes caps every request body (config apply payloads are well below this).
const maxRequestBodyBytes = 2 << 20 // 2 MiB

// LimitBody caps request body size for all routes.
func (s *Server) LimitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// allowedCORSOrigins parses NETORA_CORS_ORIGINS (comma-separated).
// Default is empty: the SPA is served by this same server (same-origin), so no CORS is needed.
// Set e.g. NETORA_CORS_ORIGINS=http://localhost:5173 for the Vite dev server.
func allowedCORSOrigins() map[string]bool {
	allowed := map[string]bool{}
	for _, o := range strings.Split(os.Getenv("NETORA_CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	return allowed
}

// Middleware: CORS (explicit origin allowlist; never wildcard)
func (s *Server) CORS(next http.Handler) http.Handler {
	allowed := allowedCORSOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
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
			log.Printf("[auth] token rejected from %s: %v", r.RemoteAddr, err)
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		// Inject user claims into request context
		ctx := context.WithValue(r.Context(), userClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// requirePerm authenticates, then enforces `readPerm` for GET/HEAD and `writePerm` for other methods.
func (s *Server) requirePerm(readPerm, writePerm string, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(userClaimsKey).(*auth.Claims)
		if !ok {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		perm := writePerm
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			perm = readPerm
		}
		if !auth.HasPermission(claims.Role, perm) {
			s.writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
			return
		}
		next(w, r)
	})
}
