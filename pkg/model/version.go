package model

import "time"

// VersionStatus describes the lifecycle state of a configuration version.
type VersionStatus string

const (
	StatusStaged     VersionStatus = "STAGED"
	StatusActive     VersionStatus = "ACTIVE"
	StatusSuperseded VersionStatus = "SUPERSEDED"
	StatusRolledBack VersionStatus = "ROLLED_BACK"
	StatusFailed     VersionStatus = "FAILED"
)

// ConfigVersion tracks an immutable version snapshot of the desired state.
type ConfigVersion struct {
	VersionID        string        `json:"version_id"`
	NodeID           string        `json:"node_id"`
	CreatedAt        time.Time     `json:"created_at"`
	Actor            string        `json:"actor"`
	Reason           string        `json:"reason"`
	Checksum         string        `json:"checksum"`
	Status           VersionStatus `json:"status"`
	ValidationReport string        `json:"validation_report,omitempty"`
	AppliedAt        *time.Time    `json:"applied_at,omitempty"`
	ActiveConfigPath string        `json:"active_config_path,omitempty"`
}

// ApplyResult represents the outcome of an atomic apply or rollback attempt.
type ApplyResult struct {
	Success             bool          `json:"success"`
	VersionID           string        `json:"version_id"`
	NodeID              string        `json:"node_id"`
	StartedAt           time.Time     `json:"started_at"`
	CompletedAt         time.Time     `json:"completed_at"`
	DurationMs          int64         `json:"duration_ms"`
	SyntaxValid         bool          `json:"syntax_valid"`
	SemanticValid       bool          `json:"semantic_valid"`
	ReloadSuccess       bool          `json:"reload_success"`
	HealthCheckSuccess  bool          `json:"health_check_success"`
	RolledBack          bool          `json:"rolled_back"`
	RolledBackToVersion string        `json:"rolled_back_to_version,omitempty"`
	ErrorMessage        string        `json:"error_message,omitempty"`
	ValidationOutput    string        `json:"validation_output,omitempty"`
	HealthOutput        string        `json:"health_output,omitempty"`
}

// AuditRecord stores security and administrative events.
type AuditRecord struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"` // e.g. "CONFIG_APPLY", "CONFIG_ROLLBACK", "USER_LOGIN"
	Target    string    `json:"target"`
	Details   string    `json:"details"`
	ClientIP  string    `json:"client_ip"`
	Status    string    `json:"status"` // "SUCCESS", "FAILURE"
}
