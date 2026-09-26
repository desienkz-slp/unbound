package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/configengine"
	"netora-dns/pkg/model"
	"netora-dns/pkg/runtime"
	"netora-dns/pkg/storage"
)

func main() {
	mode := flag.String("mode", "render", "Action: render, apply, reset-password, list-users, create-user")
	configDir := flag.String("config-dir", "/etc/unbound", "Path to Unbound configuration directory")
	nodeID := flag.String("node-id", "dns-dev", "Node ID")
	resMode := flag.String("res-mode", "RECURSIVE", "Resolution mode: RECURSIVE, FORWARDING, HYBRID")
	cores := flag.Int("cores", 2, "CPU cores")
	ramMB := flag.Int("ram-mb", 2048, "RAM in MB")
	writeDir := flag.String("write-dir", "", "Optional directory to write rendered files")

	// User management flags
	defaultDB := os.Getenv("DATABASE_URL")
	if defaultDB == "" {
		defaultDB = "postgres://netora:netora_dev_password@127.0.0.1:5432/netora?sslmode=disable"
	}
	dbURL := flag.String("db-url", defaultDB, "PostgreSQL database connection URL")
	username := flag.String("username", "admin", "Username for user operations")
	newPassword := flag.String("new-password", "", "New password to set for reset-password or create-user")
	userRole := flag.String("role", auth.RoleAdmin, "Role for create-user (SUPER_ADMIN, ADMIN, OPERATOR, NOC)")

	flag.Parse()

	switch *mode {
	case "render":
		state := model.NewDefaultDesiredState(*nodeID, *cores, *ramMB)
		setResMode(state, *resMode)

		gen, err := configengine.NewGenerator()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
			os.Exit(1)
		}

		targetDir := *configDir
		if *writeDir != "" {
			targetDir = *writeDir
		}
		res, err := gen.Generate(state, targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully generated %d files for %s (Checksum: %s)\n", len(res.Files), state.VersionID, res.MainChecksum)
		if *writeDir != "" {
			for _, f := range res.Files {
				p := filepath.Join(*writeDir, f.RelativePath)
				_ = os.MkdirAll(filepath.Dir(p), 0750)
				_ = os.WriteFile(p, f.Content, 0640)
				fmt.Printf("Wrote: %s (%d bytes)\n", p, len(f.Content))
			}
		} else {
			for _, f := range res.Files {
				fmt.Printf("\n--- [%s] ---\n", f.RelativePath)
				fmt.Println(string(f.Content))
			}
		}

	case "apply":
		state := model.NewDefaultDesiredState(*nodeID, *cores, *ramMB)
		setResMode(state, *resMode)

		gen, err := configengine.NewGenerator()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
			os.Exit(1)
		}

		sup := runtime.NewSystemdSupervisor(nil)
		health := runtime.NewDNSHealthChecker(0)

		engine, err := configengine.NewEngine(configengine.EngineOptions{
			ConfigDir:     *configDir,
			Generator:     gen,
			Supervisor:    sup,
			HealthChecker: &healthAdapter{checker: health},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating engine: %v\n", err)
			os.Exit(1)
		}

		res, err := engine.Apply(context.Background(), state, "cli-admin", "Manual CLI apply")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Apply failed: %v (Rolled back: %v)\n", err, res.RolledBack)
			os.Exit(1)
		}
		fmt.Printf("Apply SUCCESS! Version: %s in %d ms (Health: %s)\n", res.VersionID, res.DurationMs, res.HealthOutput)

	case "reset-password":
		targetUser := strings.TrimSpace(*username)
		if targetUser == "" {
			fmt.Fprintln(os.Stderr, "Error: -username flag is required (e.g. -username admin)")
			os.Exit(1)
		}

		pass := *newPassword
		if pass == "" {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("Enter new password for user '%s': ", targetUser)
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}
			pass = strings.TrimSpace(input)
		} else {
			pass = strings.TrimSpace(pass)
		}

		if len(pass) < 6 {
			fmt.Fprintln(os.Stderr, "Error: new password must be at least 6 characters")
			os.Exit(1)
		}

		store, err := storage.NewPostgresStore(*dbURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to database (%s): %v\n", *dbURL, err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		user, err := store.GetUserByUsername(ctx, targetUser)
		if err != nil {
			if targetUser == "admin" {
				hash, hErr := auth.HashPassword(pass)
				if hErr != nil {
					fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", hErr)
					os.Exit(1)
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
				if cErr := store.CreateUser(ctx, adminUser); cErr != nil {
					fmt.Fprintf(os.Stderr, "Error creating admin user: %v\n", cErr)
					os.Exit(1)
				}
				user = adminUser
			} else {
				fmt.Fprintf(os.Stderr, "Error: user '%s' not found in database: %v\n", targetUser, err)
				os.Exit(1)
			}
		} else {
			hash, err := auth.HashPassword(pass)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
				os.Exit(1)
			}

			if err := store.UpdateUserCredentials(ctx, user.ID, user.Username, hash); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating user in PostgreSQL: %v\n", err)
				os.Exit(1)
			}
		}

		_ = store.LogAudit(ctx, &model.AuditRecord{
			Actor:    "CLI_ROOT",
			Action:   "CLI_PASSWORD_RESET",
			Target:   user.ID,
			Details:  fmt.Sprintf("Password for user '%s' was reset via server CLI", user.Username),
			ClientIP: "127.0.0.1",
			Status:   "SUCCESS",
		})

		fmt.Println("\n========================================================")
		fmt.Println("  Netora DNS - Password Reset Successful")
		fmt.Println("========================================================")
		fmt.Printf("  User ID   : %s\n", user.ID)
		fmt.Printf("  Username  : %s\n", user.Username)
		fmt.Printf("  Role      : %s\n", user.Role)
		fmt.Printf("  Status    : Password successfully updated in PostgreSQL\n")
		fmt.Printf("  Timestamp : %s\n", time.Now().UTC().Format(time.RFC3339))
		fmt.Println("========================================================")
		fmt.Println("You can now sign in to the Netora NOC Web UI with your new password.")

	case "list-users":
		store, err := storage.NewPostgresStore(*dbURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to database (%s): %v\n", *dbURL, err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		users, err := store.ListUsers(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing users from database: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n=========================================================================================")
		fmt.Println("  Netora DNS - Registered Operators")
		fmt.Println("=========================================================================================")
		fmt.Printf("  %-16s %-16s %-14s %-22s\n", "USER ID", "USERNAME", "ROLE", "LAST UPDATED")
		fmt.Println("-----------------------------------------------------------------------------------------")
		for _, u := range users {
			fmt.Printf("  %-16s %-16s %-14s %-22s\n", u.ID, u.Username, u.Role, u.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println("=========================================================================================")

	case "create-user":
		targetUser := strings.TrimSpace(*username)
		if targetUser == "" {
			fmt.Fprintln(os.Stderr, "Error: -username flag is required (e.g. -username operator1)")
			os.Exit(1)
		}

		pass := strings.TrimSpace(*newPassword)
		if len(pass) < 6 {
			fmt.Fprintln(os.Stderr, "Error: -new-password must be at least 6 characters")
			os.Exit(1)
		}

		store, err := storage.NewPostgresStore(*dbURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to database (%s): %v\n", *dbURL, err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		hash, err := auth.HashPassword(pass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
			os.Exit(1)
		}

		newUser := &auth.User{
			ID:           fmt.Sprintf("usr-%d", time.Now().UnixNano()),
			Username:     targetUser,
			PasswordHash: hash,
			Role:         *userRole,
			Email:        targetUser + "@netora.internal",
			FullName:     targetUser,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}

		if err := store.CreateUser(ctx, newUser); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating user in PostgreSQL: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Operator user '%s' (%s) successfully created in database.\n", targetUser, *userRole)

	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (Available: render, apply, reset-password, list-users, create-user)\n", *mode)
		os.Exit(1)
	}
}

func setResMode(state *model.DesiredState, mode string) {
	switch mode {
	case "FORWARDING":
		state.ResolutionMode = model.ModeForwarding
	case "HYBRID":
		state.ResolutionMode = model.ModeHybrid
	default:
		state.ResolutionMode = model.ModeRecursive
	}
}

type healthAdapter struct {
	checker *runtime.DNSHealthChecker
}

func (h *healthAdapter) CheckResolution(ctx context.Context, serverIP string, serverPort int, protocol string, domain string) *configengine.HealthResult {
	res := h.checker.CheckResolution(ctx, serverIP, serverPort, protocol, domain)
	return &configengine.HealthResult{
		Healthy:      res.Healthy,
		ErrorMessage: res.ErrorMessage,
		Latency:      res.Latency,
	}
}
