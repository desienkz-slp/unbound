package logrotate

import (
	"time"

	"netora-dns/pkg/model"
)

// LogTelemetry captures real disk metrics and health indicators of log files on the host.
type LogTelemetry struct {
	UnboundLogSizeBytes   int64     `json:"unbound_log_size_bytes"`
	UnboundLogSizeHuman   string    `json:"unbound_log_size_human"`
	RotatedFilesCount     int       `json:"rotated_files_count"`
	RotatedFilesSizeBytes int64     `json:"rotated_files_size_bytes"`
	RotatedFilesSizeHuman string    `json:"rotated_files_size_human"`
	TotalLogSizeBytes     int64     `json:"total_log_size_bytes"`
	TotalLogSizeHuman     string    `json:"total_log_size_human"`
	NetoraLogSizeBytes    int64     `json:"netora_log_size_bytes"`
	NetoraLogSizeHuman    string    `json:"netora_log_size_human"`
	LastRotatedAt         time.Time `json:"last_rotated_at,omitempty"`
	ConfigPath            string    `json:"config_path"`
	LogrotateInstalled    bool      `json:"logrotate_installed"`
	HasSuDirective        bool      `json:"has_su_directive"`
	HealthStatus          string    `json:"health_status"` // "HEALTHY", "WARNING", "CRITICAL"
	Issues                []string  `json:"issues,omitempty"`
}

// AIRecommendation represents an intelligent hardware-tailored log rotation profile.
type AIRecommendation struct {
	ProfileName       string                `json:"profile_name"`     // "STORAGE_SAVER", "BALANCED", "ISP_AUDIT"
	Badge             string                `json:"badge"`            // "AI Storage Saver (Recommended)"
	Title             string                `json:"title"`
	Reasoning         string                `json:"reasoning"`        // Natural language rationale
	HardwareFactor    string                `json:"hardware_factor"`  // Description of driving factors (disk, ram, cpu)
	ProjectedSaving   string                `json:"projected_saving"` // Estimated disk space recovery
	RecommendedPolicy model.LogrotateConfig `json:"recommended_policy"`
}

// LogrotateReport provides the full diagnostic and operational status.
type LogrotateReport struct {
	NodeID           string                `json:"node_id"`
	ActivePolicy     model.LogrotateConfig `json:"active_policy"`
	ActiveConfigRaw  string                `json:"active_config_raw"`
	Telemetry        LogTelemetry          `json:"telemetry"`
	AIRecommendation AIRecommendation      `json:"ai_recommendation"`
	AutoPurgeStatus  PurgeStatus           `json:"auto_purge_status"`
}

// ApplyResult contains the outcome of applying and validating a logrotate policy.
type ApplyResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	DryRunOutput   string `json:"dry_run_output"`
	UnboundConfig  string `json:"unbound_config"`
	NetoraConfig   string `json:"netora_config,omitempty"`
}
