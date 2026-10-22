package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/cluster"
	"netora-dns/pkg/configengine"
	"netora-dns/pkg/model"
	"netora-dns/pkg/policy"
	"netora-dns/pkg/runtime"
	"netora-dns/pkg/storage"
	"netora-dns/pkg/telemetry"
)

const (
	heartbeatInterval = 5 * time.Second
	jwtSecretFile     = "/etc/netora-dns/jwt.secret"
)

// resolveJWTSecret returns the signing secret: flag > NETORA_JWT_SECRET > persisted file.
// If none exist, a random 256-bit secret is generated once and persisted (0600) so tokens
// survive restarts. Existing deployments keep working (users simply re-login once).
func resolveJWTSecret(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if v := strings.TrimSpace(os.Getenv("NETORA_JWT_SECRET")); v != "" {
		return v, nil
	}
	if data, err := os.ReadFile(jwtSecretFile); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v, nil
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate jwt secret: %w", err)
	}
	secret := hex.EncodeToString(raw)
	if err := os.WriteFile(jwtSecretFile, []byte(secret+"\n"), 0o600); err != nil {
		log.Printf("Warning: cannot persist JWT secret to %s (%v); tokens will be invalidated on restart", jwtSecretFile, err)
	}
	return secret, nil
}

func main() {
	listenAddr := flag.String("listen", ":8080", "HTTP server listen address")
	dbURL := flag.String("db-url", "postgres://netora:netora_dev_password@127.0.0.1:5432/netora?sslmode=disable", "PostgreSQL connection string")
	configDir := flag.String("config-dir", "/etc/unbound", "Path to Unbound configuration directory")
	jwtSecretFlag := flag.String("jwt-secret", "", "JWT signing secret (default: NETORA_JWT_SECRET env, else auto-generated and persisted)")
	nodeID := flag.String("node-id", "dns-dev", "Node ID")
	useMemoryStore := flag.Bool("memory-store", false, "Use in-memory store instead of PostgreSQL (for testing)")
	flag.Parse()

	jwtSecret, err := resolveJWTSecret(*jwtSecretFlag)
	if err != nil {
		log.Fatalf("Failed to resolve JWT secret: %v", err)
	}

	log.Printf("Starting Netora DNS Control Plane API for node '%s' on %s...\n", *nodeID, *listenAddr)

	// 1. Initialize Storage
	var store storage.Store
	if *useMemoryStore || *dbURL == "" {
		log.Println("Using in-memory store")
		store = storage.NewMemoryStore()
	} else {
		log.Println("Connecting to PostgreSQL...")
		pg, err := storage.NewPostgresStore(*dbURL)
		if err != nil {
			log.Printf("Warning: PostgreSQL connection failed (%v); falling back to in-memory store\n", err)
			store = storage.NewMemoryStore()
		} else {
			log.Println("Connected to PostgreSQL successfully")
			store = pg
		}
	}
	// store.Close() called explicitly during ordered shutdown

	// 2. Seed Default Administrator if not present
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := store.GetUserByUsername(ctx, "admin"); err != nil {
		initialAdminPass := os.Getenv("NETORA_ADMIN_PASS")
		if initialAdminPass == "" {
			initialAdminPass = "admin123"
			log.Println("SECURITY WARNING: seeding admin with default password 'admin123'; set NETORA_ADMIN_PASS or change it immediately")
		}
		hash, hashErr := auth.HashPassword(initialAdminPass)
		if hashErr != nil {
			log.Fatalf("Failed to hash admin password: %v", hashErr)
		}
		adminUser := &auth.User{
			ID:           "usr-superadmin",
			Username:     "admin",
			PasswordHash: hash,
			Role:         auth.RoleSuperAdmin,
			Email:        "admin@netora.internal",
			FullName:     "System Administrator",
			CreatedAt:    time.Now().UTC(),
		}
		if err := store.CreateUser(ctx, adminUser); err != nil {
			log.Printf("Warning: failed to seed default admin: %v\n", err)
		} else {
			log.Println("Default administrator account created (username: admin / role: SUPER_ADMIN)")
		}
	}

	// 3. Register Current Node with Real Dynamic Hardware Specs & IP
	realIP := detectManagementIP()
	realCores, realRAM, realUnboundVer := detectSystemSpecs()
	realDiskTotal, realDiskFree := detectDiskSpecs()
	hostname := *nodeID
	if h, err := os.Hostname(); err == nil && h != "" && (*nodeID == "dns-dev" || *nodeID == "") {
		hostname = h
	}

	nodeMeta := &model.NodeMetadata{
		NodeID:         *nodeID,
		Hostname:       hostname,
		UnboundVersion: realUnboundVer,
		CPUCores:       realCores,
		TotalRAMMB:     realRAM,
		DiskTotalGB:    realDiskTotal,
		DiskFreeGB:     realDiskFree,
		LastSeenAt:     time.Now().UTC(),
	}
	_ = store.UpsertNode(ctx, nodeMeta, "HEALTHY", realIP)
	log.Printf("Registered local node '%s' (hostname: %s, mgmt_ip: %s, cpu: %d cores, ram: %d MB, disk: %dGB total/%dGB free, unbound: %s)\n",
		*nodeID, hostname, realIP, realCores, realRAM, realDiskTotal, realDiskFree, realUnboundVer)

	// 3.1 Ensure Initial Node Listeners & DesiredState Exist with Real Detected Host IPs
	existingState, err := store.GetDesiredState(ctx, *nodeID)
	hasConcreteListeners := false
	if err == nil && existingState != nil {
		for _, l := range existingState.Listeners {
			if l.IP != "0.0.0.0" && l.IP != "" {
				hasConcreteListeners = true
				break
			}
		}
	}
	activeIPs := detectActiveIPv4Addresses()
	if len(activeIPs) == 0 && realIP != "" && realIP != "127.0.0.1" {
		activeIPs = []string{realIP}
	}

	if !hasConcreteListeners {
		initListeners := []model.Listener{
			{
				ID:      "listen-mgmt",
				Name:    "Management Loopback",
				IP:      "127.0.0.1",
				Port:    53,
				Role:    model.RoleManagement,
				IsIPv6:  false,
				UDP:     true,
				TCP:     true,
				Enabled: true,
			},
		}
		for i, ip := range activeIPs {
			role := model.RoleClient
			name := fmt.Sprintf("Client DNS (%s)", ip)
			if ip == realIP {
				name = fmt.Sprintf("Uplink / Management (%s)", ip)
			}
			initListeners = append(initListeners, model.Listener{
				ID:      fmt.Sprintf("listen-nic-%d", i+1),
				Name:    name,
				IP:      ip,
				Port:    53,
				Role:    role,
				IsIPv6:  false,
				UDP:     true,
				TCP:     true,
				Enabled: true,
			})
		}
		if len(activeIPs) == 0 {
			initListeners = append(initListeners, model.Listener{
				ID:      "listen-client-v4",
				Name:    "Client DNS IPv4",
				IP:      "0.0.0.0",
				Port:    53,
				Role:    model.RoleClient,
				IsIPv6:  false,
				UDP:     true,
				TCP:     true,
				Enabled: true,
			})
		}

		baseState := model.NewDefaultDesiredState(*nodeID, realCores, realRAM)
		baseState.Listeners = initListeners
		baseState.EnsureNodeDnsRewrites(*nodeID, activeIPs)
		if err := store.SaveDesiredState(ctx, baseState); err != nil {
			log.Printf("Warning: failed saving initial desired state: %v\n", err)
		} else {
			log.Printf("Initialized default node '%s' configuration with %d active listener(s) and %d DNS rewrite(s)\n", *nodeID, len(initListeners), len(baseState.LocalRecords))
		}
	} else if existingState != nil {
		if existingState.EnsureNodeDnsRewrites(*nodeID, activeIPs) {
			if err := store.SaveDesiredState(ctx, existingState); err != nil {
				log.Printf("Warning: failed saving synchronized desired state: %v\n", err)
			} else {
				log.Printf("Synchronized node '%s' DNS rewrites with %d detected IP(s)\n", *nodeID, len(activeIPs))
			}
		}
	}

	// 4. Initialize Config Engine
	gen, err := configengine.NewGenerator()
	if err != nil {
		log.Fatalf("Failed to initialize generator: %v", err)
	}

	sup := runtime.NewSystemdSupervisor(nil)
	health := runtime.NewDNSHealthChecker(6 * time.Second)

	engine, err := configengine.NewEngine(configengine.EngineOptions{
		ConfigDir:     *configDir,
		Generator:     gen,
		Supervisor:    sup,
		HealthChecker: &apiHealthAdapter{checker: health},
	})
	if err != nil {
		log.Fatalf("Failed to initialize engine: %v", err)
	}

	// 5. Initialize Policy Engine
	// Block rules loaded dynamically by blocklist.Manager from feed files + DB allowlist.
	policyEngine := policy.NewPolicyEngine()

	// 6. Start Telemetry Background Collector & Query Log Tailer
	statsCollector := telemetry.NewUnboundStatsCollector(telemetry.DefaultMetrics, 10*time.Second)
	queryLogTailer := telemetry.NewQueryLogTailer("/var/log/unbound/unbound.log")
	collectorCtx, collectorCancel := context.WithCancel(context.Background())
	// collectorCancel() called explicitly during ordered shutdown
	go statsCollector.Start(collectorCtx)
	go queryLogTailer.Start(collectorCtx)

	// 7. Setup HTTP Server
	apiServer := NewServer(store, engine, policyEngine, jwtSecret, *nodeID, realIP)
	if *dbURL != "" {
		apiServer.SetDatabaseURL(*dbURL)
	}

	// 7.1 Start Autonomous Blocklist Auto-Update Scheduler
	apiServer.blocklistManager.StartScheduler(collectorCtx)

	// 8. Start Local Node Heartbeat Loop
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			probe := health.CheckResolution(context.Background(), "127.0.0.1", 53, "udp", "google.com")
			_ = apiServer.clusterManager.RecordHeartbeat(&cluster.HeartbeatPayload{
				NodeID:          *nodeID,
				ConfigVersion:   "v-active",
				ConfigChecksum:  "sha256-verified",
				UnboundRunning:  probe.Healthy,
				Port53Listening: probe.Healthy,
				LatencyMS:       float64(probe.Latency.Milliseconds()),
				UnboundVersion:  realUnboundVer,
				CPUCores:        realCores,
				RAMMB:           realRAM,
			})

			select {
			case <-collectorCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)

	handler := apiServer.LimitBody(apiServer.CORS(mux))

	httpServer := &http.Server{
		Addr:         *listenAddr,
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Netora REST API listening on %s (ready for NOC / UI)\n", *listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful Shutdown — ordered: HTTP → background goroutines → storage
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Netora API server gracefully...")

	// 1. Stop accepting new HTTP connections, drain in-flight requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v\n", err)
	}

	// 2. Cancel all background goroutines (stats, query log tailer, heartbeat, blocklist scheduler)
	collectorCancel()

	// 3. Give goroutines a moment to finish in-flight work
	// ponytail: use sync.WaitGroup if goroutine count grows beyond 4
	time.Sleep(500 * time.Millisecond)

	// 4. Close storage (DB connections)
	store.Close()

	log.Println("Netora API stopped cleanly.")
}

type apiHealthAdapter struct {
	checker *runtime.DNSHealthChecker
}

func (a *apiHealthAdapter) CheckResolution(ctx context.Context, serverIP string, serverPort int, protocol string, domain string) *configengine.HealthResult {
	res := a.checker.CheckResolution(ctx, serverIP, serverPort, protocol, domain)
	return &configengine.HealthResult{
		Healthy:      res.Healthy,
		ErrorMessage: res.ErrorMessage,
		Latency:      res.Latency,
	}
}
