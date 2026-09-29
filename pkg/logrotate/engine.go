package logrotate

import (
	"fmt"
	"strings"

	"netora-dns/pkg/model"
)

// GenerateUnboundConfig renders standard /etc/logrotate.d/unbound configuration.
func GenerateUnboundConfig(policy model.LogrotateConfig) string {
	freq := policy.Frequency
	if freq == "" {
		freq = "daily"
	}
	rotate := policy.RotateCount
	if rotate <= 0 {
		rotate = 7
	}
	maxSize := strings.TrimSpace(policy.MaxSize)
	if maxSize == "" {
		maxSize = "50M"
	}

	compressStr := "compress"
	if !policy.Compress {
		compressStr = "nocompress"
	}

	delayCompressStr := ""
	if policy.DelayCompress && policy.Compress {
		delayCompressStr = "    delaycompress\n"
	}

	return fmt.Sprintf(`/var/log/unbound/unbound.log {
    %s
    rotate %d
    maxsize %s
    missingok
    notifempty
    %s
%s    sharedscripts
    su unbound unbound
    postrotate
        /usr/sbin/unbound-control log_reopen >/dev/null 2>&1 || true
    endscript
}
`, freq, rotate, maxSize, compressStr, delayCompressStr)
}

// GenerateNetoraConfig renders /etc/logrotate.d/netora-dns for system updates and telemetry logs.
func GenerateNetoraConfig(policy model.LogrotateConfig) string {
	freq := policy.Frequency
	if freq == "" {
		freq = "daily"
	}
	rotate := policy.RotateCount
	if rotate <= 0 {
		rotate = 4
	}

	compressStr := "compress"
	if !policy.Compress {
		compressStr = "nocompress"
	}

	delayCompressStr := ""
	if policy.DelayCompress && policy.Compress {
		delayCompressStr = "    delaycompress\n"
	}

	return fmt.Sprintf(`/var/log/netora-dns/*.log {
    %s
    rotate %d
    maxsize 25M
    missingok
    notifempty
    %s
%s    copytruncate
}
`, freq, rotate, compressStr, delayCompressStr)
}

// CalculateAIRecommendation analyzes hardware metrics and log velocity to determine the optimal rotation strategy.
func CalculateAIRecommendation(cores int, ramMB int, diskTotalGB int, diskFreeGB int, currentLogBytes int64) AIRecommendation {
	// Fallbacks
	if cores < 1 {
		cores = 1
	}
	if ramMB < 512 {
		ramMB = 1024
	}

	var diskUsedPct int
	if diskTotalGB > 0 {
		usedGB := diskTotalGB - diskFreeGB
		if usedGB < 0 {
			usedGB = 0
		}
		diskUsedPct = int((float64(usedGB) / float64(diskTotalGB)) * 100)
	}

	currentLogMB := currentLogBytes / (1024 * 1024)

	// Strategy 1: Ultra Storage Saver (Low Disk Space, VPS, or Disk Usage >= 75%)
	if diskTotalGB > 0 && (diskTotalGB <= 10 || diskFreeGB <= 2 || diskUsedPct >= 75) {
		maxSize := "25M"
		if diskFreeGB <= 1 {
			maxSize = "15M"
		}
		rotateCount := 3
		if diskFreeGB <= 1 {
			rotateCount = 2
		}

		factor := fmt.Sprintf("Kapasitas Disk Sangat Kritis (%dGB total, %dGB sisa, %d%% terpakai)", diskTotalGB, diskFreeGB, diskUsedPct)
		if diskTotalGB == 0 {
			factor = "Kapasitas Disk Terbatas / VPS Spec"
		}

		reasoning := fmt.Sprintf(
			"Terdeteksi ruang penyimpanan tersisa hanya %dGB dari total %dGB (%d%% terpakai). Log Unbound saat ini memakan %d MB. " +
				"Untuk mencegah kegagalan fatal Unbound akibat disk penuh (ENOSPC), AI merekomendasikan profil 'Ultra Storage Saver': " +
				"rotasi harian dengan threshold batas ukuran ketat %s, retensi %d file terkompresi, dan delaycompress untuk melindungi %d vCPU dari lonjakan beban gzip.",
			diskFreeGB, diskTotalGB, diskUsedPct, currentLogMB, maxSize, rotateCount, cores,
		)

		projected := fmt.Sprintf("Menghemat hingga ~80%% ruang penyimpanan log (reklamasi ~%d MB)", (currentLogMB * 4) / 5)
		if currentLogMB < 20 {
			projected = "Membatasi jejak log Unbound maksimum ~50 MB di disk"
		}

		return AIRecommendation{
			ProfileName:    "STORAGE_SAVER",
			Badge:          "🤖 AI: Ultra Storage Saver",
			Title:          "Konservasi Ruang Penyimpanan Kritis",
			Reasoning:      reasoning,
			HardwareFactor: factor,
			ProjectedSaving: projected,
			RecommendedPolicy: model.LogrotateConfig{
				Enabled:       true,
				Frequency:     "daily",
				RotateCount:   rotateCount,
				MaxSize:       maxSize,
				Compress:      true,
				DelayCompress: true,
				RotateNetora:  true,
			},
		}
	}

	// Strategy 2: ISP Carrier Compliance & High Retention (Large Storage > 40GB and Free > 10GB)
	if diskTotalGB >= 40 && diskFreeGB >= 10 && diskUsedPct < 70 {
		factor := fmt.Sprintf("Penyimpanan Enterprise Longgar (%dGB total, %dGB sisa, %d%% terpakai)", diskTotalGB, diskFreeGB, diskUsedPct)
		delayComp := cores <= 4

		reasoning := fmt.Sprintf(
			"Server memiliki kapasitas penyimpanan sangat longgar (%dGB total, %dGB bebas) dengan prosesor tangguh (%d cores, %d MB RAM). " +
				"AI merekomendasikan profil 'Carrier Compliance & Audit' untuk mempertahankan riwayat DNS hingga 30 hari. " +
				"Rotasi mingguan dengan ambang batas 100M dan kompresi gzip aktif memastikan jejak log tetap efisien tanpa membebani disk.",
			diskTotalGB, diskFreeGB, cores, ramMB,
		)

		return AIRecommendation{
			ProfileName:    "ISP_AUDIT",
			Badge:          "🤖 AI: Carrier Compliance & Audit",
			Title:          "Retensi Audit Jangka Panjang (ISP Standard)",
			Reasoning:      reasoning,
			HardwareFactor: factor,
			ProjectedSaving: "Menyimpan 30 hari jejak query audit dengan footprint terkendali di bawah 1.5 GB",
			RecommendedPolicy: model.LogrotateConfig{
				Enabled:       true,
				Frequency:     "weekly",
				RotateCount:   30,
				MaxSize:       "100M",
				Compress:      true,
				DelayCompress: delayComp,
				RotateNetora:  true,
			},
		}
	}

	// Strategy 3: Balanced Standard Performance (Default for 10-40GB disk)
	factor := fmt.Sprintf("Kapasitas Server Seimbang (%dGB total, %dGB sisa, %d vCPU)", diskTotalGB, diskFreeGB, cores)
	reasoning := fmt.Sprintf(
		"Server memiliki spesifikasi seimbang (%dGB disk, %d vCPU, %d MB RAM). " +
			"AI merekomendasikan rotasi harian dengan batas ukuran 50M dan retensi 7 hari dengan kompresi gzip aktif. " +
			"Pengaturan ini memberikan visibilitas diagnostik satu minggu penuh sambil menjaga konsumsi disk stabil di bawah 250 MB.",
		diskTotalGB, cores, ramMB,
	)

	return AIRecommendation{
		ProfileName:    "BALANCED",
		Badge:          "🤖 AI: Balanced Performance",
		Title:          "Keseimbangan Performa & Riwayat Diagnostik",
		Reasoning:      reasoning,
		HardwareFactor: factor,
		ProjectedSaving: "Stabilisasi jejak log DNS di bawah 250 MB sepanjang waktu",
		RecommendedPolicy: model.LogrotateConfig{
			Enabled:       true,
			Frequency:     "daily",
			RotateCount:   7,
			MaxSize:       "50M",
			Compress:      true,
			DelayCompress: true,
			RotateNetora:  true,
		},
	}
}

// FormatBytes formats byte counts into human-readable strings (e.g. "24.5 MB").
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
