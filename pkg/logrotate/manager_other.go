//go:build !linux
// +build !linux

package logrotate

import (
	"context"
	"time"

	"netora-dns/pkg/model"
)

func inspectHostLogrotate(ctx context.Context, nodeID string, cores int, ramMB int, diskTotalGB int, diskFreeGB int) (*LogrotateReport, error) {
	telemetry := LogTelemetry{
		UnboundLogSizeBytes:   24 * 1024 * 1024,
		UnboundLogSizeHuman:   "24.0 MB",
		RotatedFilesCount:     4,
		RotatedFilesSizeBytes: 128 * 1024 * 1024,
		RotatedFilesSizeHuman: "128.0 MB",
		TotalLogSizeBytes:     152 * 1024 * 1024,
		TotalLogSizeHuman:     "152.0 MB",
		NetoraLogSizeBytes:    370 * 1024,
		NetoraLogSizeHuman:    "370 KB",
		LastRotatedAt:         time.Now().Add(-24 * time.Hour),
		ConfigPath:            "/etc/logrotate.d/unbound (Simulated)",
		LogrotateInstalled:    true,
		HasSuDirective:        true,
		HealthStatus:          "HEALTHY",
	}

	activePolicy := model.LogrotateConfig{
		Enabled:       true,
		Frequency:     "daily",
		RotateCount:   7,
		MaxSize:       "50M",
		Compress:      true,
		DelayCompress: true,
		RotateNetora:  true,
	}

	rawConfig := GenerateUnboundConfig(activePolicy)
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
	unboundConf := GenerateUnboundConfig(policy)
	netoraConf := ""
	if policy.RotateNetora {
		netoraConf = GenerateNetoraConfig(policy)
	}

	return &ApplyResult{
		Success:       true,
		Message:       "Konfigurasi logrotate berhasil divalidasi (Simulasi non-Linux).",
		DryRunOutput:  "reading config file /etc/logrotate.d/unbound\nHandling 1 logs\nrotating pattern: /var/log/unbound/unbound.log  after 1 days (7 rotations)\nlog needs rotating",
		UnboundConfig: unboundConf,
		NetoraConfig:  netoraConf,
	}, nil
}

func testHostLogrotate(ctx context.Context, policy *model.LogrotateConfig) (string, error) {
	p := model.LogrotateConfig{
		Enabled:       true,
		Frequency:     "daily",
		RotateCount:   7,
		MaxSize:       "50M",
		Compress:      true,
		DelayCompress: true,
	}
	if policy != nil {
		p = *policy
	}
	conf := GenerateUnboundConfig(p)
	return "Simulated dry-run test (logrotate -d):\n" + conf, nil
}

func forceRotateHost(ctx context.Context) (string, error) {
	return "Simulated force rotation completed: /var/log/unbound/unbound.log rotated.", nil
}
