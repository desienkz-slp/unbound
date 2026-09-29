package logrotate

import (
	"strings"
	"testing"

	"netora-dns/pkg/model"
)

func TestCalculateAIRecommendation_LowDisk(t *testing.T) {
	// 2 vCPU, 2GB RAM, 7GB total disk, 1GB free (similar to Server 145)
	rec := CalculateAIRecommendation(2, 2048, 7, 1, 150*1024*1024)

	if rec.ProfileName != "STORAGE_SAVER" {
		t.Fatalf("expected STORAGE_SAVER for low disk, got %s", rec.ProfileName)
	}
	if rec.RecommendedPolicy.Frequency != "daily" {
		t.Errorf("expected daily frequency, got %s", rec.RecommendedPolicy.Frequency)
	}
	if rec.RecommendedPolicy.MaxSize != "15M" && rec.RecommendedPolicy.MaxSize != "25M" {
		t.Errorf("expected aggressive maxSize (15M or 25M), got %s", rec.RecommendedPolicy.MaxSize)
	}
	if !rec.RecommendedPolicy.Compress {
		t.Errorf("expected compress to be true")
	}
	if !rec.RecommendedPolicy.DelayCompress {
		t.Errorf("expected delaycompress to be true for 2 cores")
	}
	if !strings.Contains(rec.Reasoning, "Ultra Storage Saver") {
		t.Errorf("expected reasoning to mention Ultra Storage Saver")
	}
}

func TestCalculateAIRecommendation_ISPAudit(t *testing.T) {
	// 16 vCPU, 32GB RAM, 100GB total disk, 80GB free
	rec := CalculateAIRecommendation(16, 32768, 100, 80, 50*1024*1024)

	if rec.ProfileName != "ISP_AUDIT" {
		t.Fatalf("expected ISP_AUDIT for enterprise disk, got %s", rec.ProfileName)
	}
	if rec.RecommendedPolicy.RotateCount < 14 {
		t.Errorf("expected high rotate count for audit, got %d", rec.RecommendedPolicy.RotateCount)
	}
}

func TestGenerateUnboundConfig(t *testing.T) {
	policy := model.LogrotateConfig{
		Enabled:       true,
		Frequency:     "daily",
		RotateCount:   5,
		MaxSize:       "30M",
		Compress:      true,
		DelayCompress: true,
	}

	conf := GenerateUnboundConfig(policy)

	if !strings.Contains(conf, "/var/log/unbound/unbound.log {") {
		t.Errorf("missing path header in generated config")
	}
	if !strings.Contains(conf, "daily") {
		t.Errorf("missing daily directive")
	}
	if !strings.Contains(conf, "rotate 5") {
		t.Errorf("missing rotate 5 directive")
	}
	if !strings.Contains(conf, "maxsize 30M") {
		t.Errorf("missing maxsize 30M directive")
	}
	if !strings.Contains(conf, "su unbound unbound") {
		t.Errorf("missing critical 'su unbound unbound' directive")
	}
	if !strings.Contains(conf, "/usr/sbin/unbound-control log_reopen") {
		t.Errorf("missing postrotate unbound-control log_reopen")
	}
}

func TestGenerateNetoraConfig(t *testing.T) {
	policy := model.LogrotateConfig{
		Enabled:       true,
		Frequency:     "weekly",
		RotateCount:   4,
		Compress:      true,
		DelayCompress: false,
	}

	conf := GenerateNetoraConfig(policy)

	if !strings.Contains(conf, "/var/log/netora-dns/*.log {") {
		t.Errorf("missing netora path header")
	}
	if !strings.Contains(conf, "copytruncate") {
		t.Errorf("missing copytruncate directive")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024 * 1024, "1.0 MB"},
		{150 * 1024 * 1024, "150.0 MB"},
		{5 * 1024 * 1024 * 1024, "5.0 GB"},
	}

	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if got != tt.expected {
			t.Errorf("FormatBytes(%d) = %s; want %s", tt.bytes, got, tt.expected)
		}
	}
}
