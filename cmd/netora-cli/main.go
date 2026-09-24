package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"netora-dns/pkg/configengine"
	"netora-dns/pkg/model"
	"netora-dns/pkg/runtime"
)

func main() {
	mode := flag.String("mode", "render", "Action: render, apply, rollback")
	configDir := flag.String("config-dir", "/etc/unbound", "Path to Unbound configuration directory")
	nodeID := flag.String("node-id", "dns-dev", "Node ID")
	resMode := flag.String("res-mode", "RECURSIVE", "Resolution mode: RECURSIVE, FORWARDING, HYBRID")
	cores := flag.Int("cores", 2, "CPU cores")
	ramMB := flag.Int("ram-mb", 2048, "RAM in MB")
	writeDir := flag.String("write-dir", "", "Optional directory to write rendered files")
	flag.Parse()

	state := model.NewDefaultDesiredState(*nodeID, *cores, *ramMB)
	switch *resMode {
	case "FORWARDING":
		state.ResolutionMode = model.ModeForwarding
	case "HYBRID":
		state.ResolutionMode = model.ModeHybrid
	default:
		state.ResolutionMode = model.ModeRecursive
	}

	gen, err := configengine.NewGenerator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
		os.Exit(1)
	}

	switch *mode {
	case "render":
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

	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *mode)
		os.Exit(1)
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
