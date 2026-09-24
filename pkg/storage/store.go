package storage

import (
	"context"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/model"
)

// NodeRecord represents a cluster DNS node stored in PostgreSQL.
type NodeRecord struct {
	ID             string    `json:"id"`
	Hostname       string    `json:"hostname"`
	ManagementIP   string    `json:"management_ip"`
	Status         string    `json:"status"` // HEALTHY, DEGRADED, OFFLINE
	UnboundVersion string    `json:"unbound_version"`
	CPUCores       int       `json:"cpu_cores"`
	RAMMB          int       `json:"ram_mb"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PolicyRecord represents a customer/tenant policy profile.
type PolicyRecord struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	ResolutionMode    string    `json:"resolution_mode"`
	SafeSearchEnabled bool      `json:"safesearch_enabled"`
	DNSSECEnabled     bool      `json:"dnssec_enabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// FilterRecord represents an external or custom blocklist feed.
type FilterRecord struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Category      string     `json:"category"`
	SourceURL     string     `json:"source_url"`
	LocalPath     string     `json:"local_path"`
	RuleCount     int        `json:"rule_count"`
	Enabled       bool       `json:"enabled"`
	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// AllowlistRecord represents an explicit domain bypass rule.
type AllowlistRecord struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	Category  string    `json:"category"`
	Reason    string    `json:"reason"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// Store defines persistence operations for the control plane.
type Store interface {
	// Users & Auth
	CreateUser(ctx context.Context, u *auth.User) error
	GetUserByUsername(ctx context.Context, username string) (*auth.User, error)
	ListUsers(ctx context.Context) ([]auth.User, error)

	// Nodes
	UpsertNode(ctx context.Context, n *model.NodeMetadata, status string, mgmtIP string) error
	GetNode(ctx context.Context, nodeID string) (*NodeRecord, error)
	ListNodes(ctx context.Context) ([]NodeRecord, error)

	// DesiredState
	SaveDesiredState(ctx context.Context, state *model.DesiredState) error
	GetDesiredState(ctx context.Context, nodeID string) (*model.DesiredState, error)

	// ConfigVersions
	SaveConfigVersion(ctx context.Context, v *model.ConfigVersion) error
	ListConfigVersions(ctx context.Context, nodeID string, limit int) ([]model.ConfigVersion, error)

	// Audit
	LogAudit(ctx context.Context, a *model.AuditRecord) error
	ListAuditLogs(ctx context.Context, limit int) ([]model.AuditRecord, error)

	// Policies & Filters
	ListPolicies(ctx context.Context) ([]PolicyRecord, error)
	UpsertPolicy(ctx context.Context, p *PolicyRecord) error
	ListFilters(ctx context.Context) ([]FilterRecord, error)
	UpsertFilter(ctx context.Context, f *FilterRecord) error
	ListAllowlist(ctx context.Context) ([]AllowlistRecord, error)
	AddAllowlist(ctx context.Context, domain, category, reason, createdBy string) error
	RemoveAllowlist(ctx context.Context, domain string) error

	Close() error
}
