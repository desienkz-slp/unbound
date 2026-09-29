//go:build linux
// +build linux

package logrotate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"netora-dns/pkg/model"
)

func inspectHostLogrotate(ctx context.Context, nodeID string, cores int, ramMB int, diskTotalGB int, diskFreeGB int) (*LogrotateReport, error) {
	telemetry := LogTelemetry{
		ConfigPath:   "/etc/logrotate.d/unbound",
		HealthStatus: "HEALTHY",
	}

	// 1. Check if logrotate binary is installed
	if _, err := exec.LookPath("logrotate"); err == nil {
		telemetry.LogrotateInstalled = true
	} else {
		telemetry.HealthStatus = "CRITICAL"
		telemetry.Issues = append(telemetry.Issues, "Paket 'logrotate' belum terpasang di sistem operasi.")
	}

	// 2. Telemetry: active Unbound log file
	unboundLogPath := "/var/log/unbound/unbound.log"
	if fi, err := os.Stat(unboundLogPath); err == nil {
		telemetry.UnboundLogSizeBytes = fi.Size()
		telemetry.UnboundLogSizeHuman = FormatBytes(fi.Size())
	} else {
		telemetry.UnboundLogSizeHuman = "0 B"
	}

	// 3. Telemetry: rotated Unbound logs in /var/log/unbound/
	entries, err := os.ReadDir("/var/log/unbound")
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, "unbound.log.") {
				if fi, err := e.Info(); err == nil {
					telemetry.RotatedFilesCount++
					telemetry.RotatedFilesSizeBytes += fi.Size()
				}
			}
		}
	}
	telemetry.RotatedFilesSizeHuman = FormatBytes(telemetry.RotatedFilesSizeBytes)
	telemetry.TotalLogSizeBytes = telemetry.UnboundLogSizeBytes + telemetry.RotatedFilesSizeBytes
	telemetry.TotalLogSizeHuman = FormatBytes(telemetry.TotalLogSizeBytes)

	// 4. Telemetry: Netora updater log
	netoraLogPath := "/var/log/netora-dns/update.log"
	if fi, err := os.Stat(netoraLogPath); err == nil {
		telemetry.NetoraLogSizeBytes = fi.Size()
		telemetry.NetoraLogSizeHuman = FormatBytes(fi.Size())
	} else {
		telemetry.NetoraLogSizeHuman = "0 B"
	}

	// 5. Read active /etc/logrotate.d/unbound
	var rawConfig string
	var activePolicy model.LogrotateConfig
	activePolicy.Enabled = true
	activePolicy.Frequency = "daily"
	activePolicy.RotateCount = 7
	activePolicy.MaxSize = "50M"
	activePolicy.Compress = true
	activePolicy.DelayCompress = true
	activePolicy.RotateNetora = true

	if cfgBytes, err := os.ReadFile("/etc/logrotate.d/unbound"); err == nil {
		rawConfig = string(cfgBytes)
		activePolicy = parseLogrotateConfig(rawConfig)

		if strings.Contains(rawConfig, "su ") {
			telemetry.HasSuDirective = true
		} else {
			telemetry.HasSuDirective = false
			telemetry.HealthStatus = "WARNING"
			telemetry.Issues = append(telemetry.Issues, "Directive 'su unbound unbound' tidak ada pada /etc/logrotate.d/unbound. Logrotate Linux akan menolak eksekusi!")
		}
	} else {
		telemetry.HealthStatus = "WARNING"
		telemetry.Issues = append(telemetry.Issues, "File konfigurasi /etc/logrotate.d/unbound tidak ditemukan.")
	}

	// 6. Inspect /var/lib/logrotate/status for last rotation timestamp
	if statusBytes, err := os.ReadFile("/var/lib/logrotate/status"); err == nil {
		lines := strings.Split(string(statusBytes), "\n")
		for _, line := range lines {
			if strings.Contains(line, "/var/log/unbound/unbound.log") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					dateStr := fields[len(fields)-1]
					// format usually 2026-9-28-0:0:0 or 2026-09-28
					if t, err := time.Parse("2006-1-2-15:4:5", dateStr); err == nil {
						telemetry.LastRotatedAt = t
					} else if t, err := time.Parse("2006-01-02", dateStr); err == nil {
						telemetry.LastRotatedAt = t
					}
				}
				break
			}
		}
	}

	// 7. Check log size thresholds against host disk capacity
	if telemetry.TotalLogSizeBytes > 100*1024*1024 {
		if telemetry.HealthStatus != "CRITICAL" {
			telemetry.HealthStatus = "WARNING"
		}
		telemetry.Issues = append(telemetry.Issues, fmt.Sprintf("Total file log Unbound sudah mencapai %s. Disarankan merotasi atau memadatkan retensi.", telemetry.TotalLogSizeHuman))
	}
	if diskFreeGB <= 1 && diskTotalGB > 0 {
		telemetry.HealthStatus = "CRITICAL"
		telemetry.Issues = append(telemetry.Issues, fmt.Sprintf("Sisa ruang disk host kritis (%dGB free). Rekomendasi rotasi agresif harus segera diterapkan.", diskFreeGB))
	}

	// 8. Generate hardware-aware AI Recommendation
	aiRec := CalculateAIRecommendation(cores, ramMB, diskTotalGB, diskFreeGB, telemetry.TotalLogSizeBytes)

	return &LogrotateReport{
		NodeID:           nodeID,
		ActivePolicy:     activePolicy,
		ActiveConfigRaw:  rawConfig,
		Telemetry:        telemetry,
		AIRecommendation: aiRec,
	}, nil
}

func applyHostLogrotate(ctx context.Context, policy model.LogrotateConfig) (*ApplyResult, error) {
	// 1. Ensure log directories exist with proper permissions
	_ = os.MkdirAll("/var/log/unbound", 0755)
	_ = os.MkdirAll("/var/log/netora-dns", 0755)

	// 2. Generate and write Unbound logrotate config
	unboundConf := GenerateUnboundConfig(policy)
	if err := os.WriteFile("/etc/logrotate.d/unbound", []byte(unboundConf), 0644); err != nil {
		return nil, fmt.Errorf("failed to write /etc/logrotate.d/unbound: %w", err)
	}

	// 3. Generate and write Netora logrotate config if enabled
	var netoraConf string
	if policy.RotateNetora {
		netoraConf = GenerateNetoraConfig(policy)
		if err := os.WriteFile("/etc/logrotate.d/netora-dns", []byte(netoraConf), 0644); err != nil {
			return nil, fmt.Errorf("failed to write /etc/logrotate.d/netora-dns: %w", err)
		}
	}

	// 4. Validate syntax via logrotate -d
	dryRunOut := ""
	cmd := exec.CommandContext(ctx, "logrotate", "-d", "/etc/logrotate.d/unbound")
	out, err := cmd.CombinedOutput()
	dryRunOut = string(out)

	// If dry-run produced errors (e.g. syntax error), return warning
	if err != nil && !strings.Contains(dryRunOut, "considering log") {
		return &ApplyResult{
			Success:       false,
			Message:       "Konfigurasi ditulis namun pengujian logrotate menghasilkan error: " + err.Error(),
			DryRunOutput:  dryRunOut,
			UnboundConfig: unboundConf,
			NetoraConfig:  netoraConf,
		}, nil
	}

	return &ApplyResult{
		Success:       true,
		Message:       "Konfigurasi logrotate berhasil diterapkan dan diverifikasi oleh sistem.",
		DryRunOutput:  dryRunOut,
		UnboundConfig: unboundConf,
		NetoraConfig:  netoraConf,
	}, nil
}

func testHostLogrotate(ctx context.Context, policy *model.LogrotateConfig) (string, error) {
	targetFile := "/etc/logrotate.d/unbound"
	// If a custom policy is tested without saving, write to temporary file
	if policy != nil {
		tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("netora-test-logrotate-%d.conf", time.Now().UnixNano()))
		conf := GenerateUnboundConfig(*policy)
		if err := os.WriteFile(tmpPath, []byte(conf), 0644); err == nil {
			defer os.Remove(tmpPath)
			targetFile = tmpPath
		}
	}

	cmd := exec.CommandContext(ctx, "logrotate", "-d", targetFile)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func forceRotateHost(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "logrotate", "-f", "/etc/logrotate.d/unbound")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// parseLogrotateConfig extracts basic policy attributes from active configuration file.
func parseLogrotateConfig(content string) model.LogrotateConfig {
	p := model.LogrotateConfig{
		Enabled:       true,
		Frequency:     "daily",
		RotateCount:   7,
		MaxSize:       "50M",
		Compress:      true,
		DelayCompress: true,
		RotateNetora:  true,
	}

	lines := strings.Split(content, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if line == "daily" || line == "weekly" || line == "monthly" || line == "hourly" {
			p.Frequency = line
		} else if strings.HasPrefix(line, "rotate ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if n, err := strconv.Atoi(fields[1]); err == nil {
					p.RotateCount = n
				}
			}
		} else if strings.HasPrefix(line, "maxsize ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				p.MaxSize = fields[1]
			}
		} else if strings.HasPrefix(line, "size ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				p.MaxSize = fields[1]
			}
		} else if line == "nocompress" {
			p.Compress = false
		} else if line == "compress" {
			p.Compress = true
		} else if line == "delaycompress" {
			p.DelayCompress = true
		}
	}
	return p
}
