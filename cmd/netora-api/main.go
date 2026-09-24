package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	listenAddr := flag.String("listen", ":8080", "HTTP server listen address")
	dbURL := flag.String("db-url", "postgres://netora:netora_dev_password@127.0.0.1:5432/netora?sslmode=disable", "PostgreSQL connection string")
	configDir := flag.String("config-dir", "/etc/unbound", "Path to Unbound configuration directory")
	jwtSecret := flag.String("jwt-secret", "netora-production-jwt-signing-secret-2026", "JWT signing secret")
	nodeID := flag.String("node-id", "dns-dev", "Node ID")
	useMemoryStore := flag.Bool("memory-store", false, "Use in-memory store instead of PostgreSQL (for testing)")
	flag.Parse()

	log.Printf("Starting Netora DNS Control Plane API for node '%s' on %s...\n", *nodeID, *listenAddr)

	// 1. Initialize Storage
	var store storage.Store
	if *useMemoryStore || *dbURL == "" {
		log.Println("Using in-memory store")
		store = storage.NewMemoryStore()
	} else {
		log.Printf("Connecting to PostgreSQL at %s...\n", *dbURL)
		pg, err := storage.NewPostgresStore(*dbURL)
		if err != nil {
			log.Printf("Warning: PostgreSQL connection failed (%v); falling back to in-memory store\n", err)
			store = storage.NewMemoryStore()
		} else {
			log.Println("Connected to PostgreSQL successfully")
			store = pg
		}
	}
	defer store.Close()

	// 2. Seed Default Administrator if not present
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := store.GetUserByUsername(ctx, "admin"); err != nil {
		hash, _ := auth.HashPassword("admin123")
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

	// 3. Register Current Node
	nodeMeta := &model.NodeMetadata{
		NodeID:         *nodeID,
		Hostname:       *nodeID,
		UnboundVersion: "1.13.1",
		CPUCores:       2,
		TotalRAMMB:     2048,
		LastSeenAt:     time.Now().UTC(),
	}
	_ = store.UpsertNode(ctx, nodeMeta, "HEALTHY", "172.18.20.145")

	// 4. Initialize Config Engine
	gen, err := configengine.NewGenerator()
	if err != nil {
		log.Fatalf("Failed to initialize generator: %v", err)
	}

	sup := runtime.NewSystemdSupervisor(nil)
	health := runtime.NewDNSHealthChecker(3 * time.Second)

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
	policyEngine := policy.NewPolicyEngine()
	policyEngine.AddBlockRule("doubleclick.net", "ADS", "Hagezi PopupAds", "||doubleclick.net^")
	policyEngine.AddBlockRule("googleadservices.com", "ADS", "Hagezi PopupAds", "||googleadservices.com^")
	policyEngine.AddBlockRule("malware-traffic-analysis.net", "MALWARE", "Hagezi Light", "||malware-traffic-analysis.net^")

	// 6. Start Telemetry Background Collector
	statsCollector := telemetry.NewUnboundStatsCollector(telemetry.DefaultMetrics, 10*time.Second)
	collectorCtx, collectorCancel := context.WithCancel(context.Background())
	defer collectorCancel()
	go statsCollector.Start(collectorCtx)

	// 7. Setup HTTP Server
	apiServer := NewServer(store, engine, policyEngine, *jwtSecret, *nodeID)

	// 8. Start Local Node Heartbeat Loop
	go func() {
		ticker := time.NewTicker(5 * time.Second)
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
				UnboundVersion:  "1.13.1",
				CPUCores:        2,
				RAMMB:           2048,
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

	handler := apiServer.CORS(mux)

	httpServer := &http.Server{
		Addr:         *listenAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Netora REST API listening on %s (ready for NOC / UI)\n", *listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Netora API server gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v\n", err)
	}
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
