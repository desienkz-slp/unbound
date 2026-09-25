package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"netora-dns/pkg/auth"
	"netora-dns/pkg/model"
)

// PostgresStore implements the Store interface using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore connects to PostgreSQL and configures connection pooling.
func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(15 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	// Idempotent column check for node resolution_mode
	_, _ = db.Exec(`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS resolution_mode VARCHAR(32) DEFAULT 'RECURSIVE';`)

	return &PostgresStore{db: db}, nil
}

// Close closes the database connection pool.
func (p *PostgresStore) Close() error {
	return p.db.Close()
}

// CreateUser inserts a new user with non-destructive conflict handling.
func (p *PostgresStore) CreateUser(ctx context.Context, u *auth.User) error {
	query := `
		INSERT INTO users (id, username, password_hash, role, email, full_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (username) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			role = EXCLUDED.role,
			email = EXCLUDED.email,
			full_name = EXCLUDED.full_name,
			updated_at = NOW();
	`
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now

	_, err := p.db.ExecContext(ctx, query, u.ID, u.Username, u.PasswordHash, u.Role, u.Email, u.FullName, u.CreatedAt, u.UpdatedAt)
	return err
}

// GetUserByUsername retrieves a user by username.
func (p *PostgresStore) GetUserByUsername(ctx context.Context, username string) (*auth.User, error) {
	query := `SELECT id, username, password_hash, role, email, full_name, created_at, updated_at FROM users WHERE username = $1`
	row := p.db.QueryRowContext(ctx, query, username)

	var u auth.User
	var email, fullName sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &email, &fullName, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user '%s' not found", username)
		}
		return nil, err
	}
	u.Email = email.String
	u.FullName = fullName.String
	return &u, nil
}

// ListUsers lists all registered operators.
func (p *PostgresStore) ListUsers(ctx context.Context) ([]auth.User, error) {
	query := `SELECT id, username, role, email, full_name, created_at, updated_at FROM users ORDER BY username ASC`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []auth.User
	for rows.Next() {
		var u auth.User
		var email, fullName sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &email, &fullName, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.Email = email.String
		u.FullName = fullName.String
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpsertNode records node metadata using non-destructive UPSERT.
func (p *PostgresStore) UpsertNode(ctx context.Context, n *model.NodeMetadata, status string, mgmtIP string) error {
	query := `
		INSERT INTO nodes (id, hostname, management_ip, status, unbound_version, cpu_cores, ram_mb, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			management_ip = EXCLUDED.management_ip,
			status = EXCLUDED.status,
			unbound_version = EXCLUDED.unbound_version,
			cpu_cores = EXCLUDED.cpu_cores,
			ram_mb = EXCLUDED.ram_mb,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = NOW();
	`
	_, err := p.db.ExecContext(ctx, query, n.NodeID, n.Hostname, mgmtIP, status, n.UnboundVersion, n.CPUCores, n.TotalRAMMB, n.LastSeenAt)
	return err
}

// GetNode gets a single node by ID.
func (p *PostgresStore) GetNode(ctx context.Context, nodeID string) (*NodeRecord, error) {
	query := `SELECT id, hostname, management_ip, status, unbound_version, cpu_cores, ram_mb, last_seen_at, created_at, updated_at FROM nodes WHERE id = $1`
	row := p.db.QueryRowContext(ctx, query, nodeID)

	var n NodeRecord
	var lastSeen sql.NullTime
	var ver sql.NullString
	if err := row.Scan(&n.ID, &n.Hostname, &n.ManagementIP, &n.Status, &ver, &n.CPUCores, &n.RAMMB, &lastSeen, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return nil, err
	}
	n.UnboundVersion = ver.String
	n.LastSeenAt = lastSeen.Time
	return &n, nil
}

// ListNodes lists all managed DNS nodes.
func (p *PostgresStore) ListNodes(ctx context.Context) ([]NodeRecord, error) {
	query := `SELECT id, hostname, management_ip, status, unbound_version, cpu_cores, ram_mb, last_seen_at, created_at, updated_at FROM nodes ORDER BY hostname ASC`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []NodeRecord
	for rows.Next() {
		var n NodeRecord
		var lastSeen sql.NullTime
		var ver sql.NullString
		if err := rows.Scan(&n.ID, &n.Hostname, &n.ManagementIP, &n.Status, &ver, &n.CPUCores, &n.RAMMB, &lastSeen, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.UnboundVersion = ver.String
		n.LastSeenAt = lastSeen.Time
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// SaveDesiredState synchronizes a node's full desired state transactionally.
func (p *PostgresStore) SaveDesiredState(ctx context.Context, state *model.DesiredState) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 0. Update resolution_mode on node if set
	if state.ResolutionMode != "" {
		_, _ = tx.ExecContext(ctx, `UPDATE nodes SET resolution_mode = $1, updated_at = NOW() WHERE id = $2`, string(state.ResolutionMode), state.NodeID)
	}

	// 1. Sync Listeners (non-destructive UPSERT)
	if len(state.Listeners) > 0 {
		var ids []string
		for _, l := range state.Listeners {
			ids = append(ids, fmt.Sprintf("'%s'", strings.ReplaceAll(l.ID, "'", "''")))
		}
		delQuery := fmt.Sprintf(`DELETE FROM listeners WHERE node_id = $1 AND id NOT IN (%s)`, strings.Join(ids, ","))
		if _, err := tx.ExecContext(ctx, delQuery, state.NodeID); err != nil {
			return fmt.Errorf("failed pruning listeners: %w", err)
		}
	}
	for _, l := range state.Listeners {
		query := `
			INSERT INTO listeners (id, node_id, name, ip, port, role, is_ipv6, udp, tcp, enabled, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				ip = EXCLUDED.ip,
				port = EXCLUDED.port,
				role = EXCLUDED.role,
				is_ipv6 = EXCLUDED.is_ipv6,
				udp = EXCLUDED.udp,
				tcp = EXCLUDED.tcp,
				enabled = EXCLUDED.enabled;
		`
		if _, err := tx.ExecContext(ctx, query, l.ID, state.NodeID, l.Name, l.IP, l.Port, string(l.Role), l.IsIPv6, l.UDP, l.TCP, l.Enabled); err != nil {
			return fmt.Errorf("failed saving listener %s: %w", l.ID, err)
		}
	}

	// 2. Sync Client Networks
	if len(state.ClientNetworks) > 0 {
		var ids []string
		for _, n := range state.ClientNetworks {
			ids = append(ids, fmt.Sprintf("'%s'", strings.ReplaceAll(n.ID, "'", "''")))
		}
		delQuery := fmt.Sprintf(`DELETE FROM client_networks WHERE node_id = $1 AND id NOT IN (%s)`, strings.Join(ids, ","))
		if _, err := tx.ExecContext(ctx, delQuery, state.NodeID); err != nil {
			return fmt.Errorf("failed pruning client networks: %w", err)
		}
	}
	for _, n := range state.ClientNetworks {
		query := `
			INSERT INTO client_networks (id, node_id, cidr, description, action, policy_group, enabled, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			ON CONFLICT (id) DO UPDATE SET
				cidr = EXCLUDED.cidr,
				description = EXCLUDED.description,
				action = EXCLUDED.action,
				policy_group = EXCLUDED.policy_group,
				enabled = EXCLUDED.enabled;
		`
		if _, err := tx.ExecContext(ctx, query, n.ID, state.NodeID, n.CIDR, n.Description, n.Action, n.PolicyGroup, n.Enabled); err != nil {
			return fmt.Errorf("failed saving client network %s: %w", n.ID, err)
		}
	}

	// 3. Sync Upstreams
	if len(state.Upstreams) > 0 {
		var ids []string
		for _, u := range state.Upstreams {
			ids = append(ids, fmt.Sprintf("'%s'", strings.ReplaceAll(u.ID, "'", "''")))
		}
		delQuery := fmt.Sprintf(`DELETE FROM upstreams WHERE node_id = $1 AND id NOT IN (%s)`, strings.Join(ids, ","))
		if _, err := tx.ExecContext(ctx, delQuery, state.NodeID); err != nil {
			return fmt.Errorf("failed pruning upstreams: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM upstreams WHERE node_id = $1`, state.NodeID); err != nil {
			return fmt.Errorf("failed clearing upstreams: %w", err)
		}
	}
	for _, u := range state.Upstreams {
		query := `
			INSERT INTO upstreams (id, node_id, name, ip, port, type, tls_name, weight, enabled, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				ip = EXCLUDED.ip,
				port = EXCLUDED.port,
				type = EXCLUDED.type,
				tls_name = EXCLUDED.tls_name,
				weight = EXCLUDED.weight,
				enabled = EXCLUDED.enabled;
		`
		if _, err := tx.ExecContext(ctx, query, u.ID, state.NodeID, u.Name, u.IP, u.Port, string(u.Type), u.TLSName, u.Weight, u.Enabled); err != nil {
			return fmt.Errorf("failed saving upstream %s: %w", u.ID, err)
		}
	}

	return tx.Commit()
}

// GetDesiredState reconstructs the DesiredState from PostgreSQL.
func (p *PostgresStore) GetDesiredState(ctx context.Context, nodeID string) (*model.DesiredState, error) {
	node, err := p.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed fetching node: %w", err)
	}

	state := model.NewDefaultDesiredState(nodeID, node.CPUCores, node.RAMMB)

	// Fetch resolution_mode if stored on node
	var resMode sql.NullString
	if err := p.db.QueryRowContext(ctx, `SELECT resolution_mode FROM nodes WHERE id = $1`, nodeID).Scan(&resMode); err == nil && resMode.Valid && resMode.String != "" {
		state.ResolutionMode = model.ResolutionMode(resMode.String)
	}

	// Fetch Listeners
	lRows, err := p.db.QueryContext(ctx, `SELECT id, name, ip, port, role, is_ipv6, udp, tcp, enabled FROM listeners WHERE node_id = $1 ORDER BY created_at ASC`, nodeID)
	if err == nil {
		defer lRows.Close()
		var listeners []model.Listener
		for lRows.Next() {
			var l model.Listener
			var roleStr string
			if err := lRows.Scan(&l.ID, &l.Name, &l.IP, &l.Port, &roleStr, &l.IsIPv6, &l.UDP, &l.TCP, &l.Enabled); err == nil {
				l.Role = model.ListenerRole(roleStr)
				listeners = append(listeners, l)
			}
		}
		if err := lRows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating listeners: %w", err)
		}
		if len(listeners) > 0 {
			state.Listeners = listeners
		}
	}

	// Fetch Client Networks
	cRows, err := p.db.QueryContext(ctx, `SELECT id, cidr, description, action, policy_group, enabled FROM client_networks WHERE node_id = $1 ORDER BY created_at ASC`, nodeID)
	if err == nil {
		defer cRows.Close()
		var networks []model.ClientNetwork
		for cRows.Next() {
			var n model.ClientNetwork
			var desc, grp sql.NullString
			if err := cRows.Scan(&n.ID, &n.CIDR, &desc, &n.Action, &grp, &n.Enabled); err == nil {
				n.Description = desc.String
				n.PolicyGroup = grp.String
				networks = append(networks, n)
			}
		}
		if err := cRows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating client networks: %w", err)
		}
		if len(networks) > 0 {
			state.ClientNetworks = networks
		}
	}

	// Fetch Upstreams
	uRows, err := p.db.QueryContext(ctx, `SELECT id, name, ip, port, type, COALESCE(tls_name, ''), COALESCE(weight, 100), enabled FROM upstreams WHERE node_id = $1 ORDER BY created_at ASC`, nodeID)
	if err == nil {
		defer uRows.Close()
		var upstreams []model.UpstreamServer
		for uRows.Next() {
			var u model.UpstreamServer
			var uType string
			if err := uRows.Scan(&u.ID, &u.Name, &u.IP, &u.Port, &uType, &u.TLSName, &u.Weight, &u.Enabled); err == nil {
				u.Type = model.UpstreamType(uType)
				upstreams = append(upstreams, u)
			}
		}
		if err := uRows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating upstreams: %w", err)
		}
		if len(upstreams) > 0 {
			state.Upstreams = upstreams
		}
	}

	return state, nil
}

// SaveConfigVersion records a configuration version in history.
func (p *PostgresStore) SaveConfigVersion(ctx context.Context, v *model.ConfigVersion) error {
	query := `
		INSERT INTO config_versions (id, node_id, version_id, checksum, actor, reason, status, applied_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			applied_at = EXCLUDED.applied_at;
	`
	id := fmt.Sprintf("%s-%s", v.NodeID, v.VersionID)
	_, err := p.db.ExecContext(ctx, query, id, v.NodeID, v.VersionID, v.Checksum, v.Actor, v.Reason, string(v.Status), v.AppliedAt, v.CreatedAt)
	return err
}

// ListConfigVersions lists versions for a node.
func (p *PostgresStore) ListConfigVersions(ctx context.Context, nodeID string, limit int) ([]model.ConfigVersion, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT version_id, node_id, checksum, actor, reason, status, applied_at, created_at FROM config_versions WHERE node_id = $1 ORDER BY created_at DESC LIMIT $2`
	rows, err := p.db.QueryContext(ctx, query, nodeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []model.ConfigVersion
	for rows.Next() {
		var v model.ConfigVersion
		var applied sql.NullTime
		var reason sql.NullString
		var status string
		if err := rows.Scan(&v.VersionID, &v.NodeID, &v.Checksum, &v.Actor, &reason, &status, &applied, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Reason = reason.String
		v.Status = model.VersionStatus(status)
		if applied.Valid {
			v.AppliedAt = &applied.Time
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// LogAudit persists an audit trail event.
func (p *PostgresStore) LogAudit(ctx context.Context, a *model.AuditRecord) error {
	query := `
		INSERT INTO audit_logs (id, timestamp, actor, action, target, details, client_ip, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
	`
	if a.ID == "" {
		a.ID = fmt.Sprintf("aud-%d", time.Now().UnixNano())
	}
	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now().UTC()
	}
	_, err := p.db.ExecContext(ctx, query, a.ID, a.Timestamp, a.Actor, a.Action, a.Target, a.Details, a.ClientIP, a.Status)
	return err
}

// ListAuditLogs retrieves audit logs ordered by timestamp.
func (p *PostgresStore) ListAuditLogs(ctx context.Context, limit int) ([]model.AuditRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT id, timestamp, actor, action, target, details, client_ip, status FROM audit_logs ORDER BY timestamp DESC LIMIT $1`
	rows, err := p.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.AuditRecord
	for rows.Next() {
		var a model.AuditRecord
		var details, clientIP sql.NullString
		if err := rows.Scan(&a.ID, &a.Timestamp, &a.Actor, &a.Action, &a.Target, &details, &clientIP, &a.Status); err != nil {
			return nil, err
		}
		a.Details = details.String
		a.ClientIP = clientIP.String
		logs = append(logs, a)
	}
	return logs, rows.Err()
}

// ListPolicies lists all policy profiles.
func (p *PostgresStore) ListPolicies(ctx context.Context) ([]PolicyRecord, error) {
	query := `SELECT id, name, description, resolution_mode, safesearch_enabled, dnssec_enabled, created_at, updated_at FROM policy_profiles ORDER BY name ASC`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []PolicyRecord
	for rows.Next() {
		var pol PolicyRecord
		var desc sql.NullString
		if err := rows.Scan(&pol.ID, &pol.Name, &desc, &pol.ResolutionMode, &pol.SafeSearchEnabled, &pol.DNSSECEnabled, &pol.CreatedAt, &pol.UpdatedAt); err != nil {
			return nil, err
		}
		pol.Description = desc.String
		policies = append(policies, pol)
	}
	return policies, rows.Err()
}

// UpsertPolicy adds or updates a policy profile.
func (p *PostgresStore) UpsertPolicy(ctx context.Context, pol *PolicyRecord) error {
	query := `
		INSERT INTO policy_profiles (id, name, description, resolution_mode, safesearch_enabled, dnssec_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (name) DO UPDATE SET
			description = EXCLUDED.description,
			resolution_mode = EXCLUDED.resolution_mode,
			safesearch_enabled = EXCLUDED.safesearch_enabled,
			dnssec_enabled = EXCLUDED.dnssec_enabled,
			updated_at = NOW();
	`
	_, err := p.db.ExecContext(ctx, query, pol.ID, pol.Name, pol.Description, pol.ResolutionMode, pol.SafeSearchEnabled, pol.DNSSECEnabled)
	return err
}

// ListFilters lists all configured blocklist feeds.
func (p *PostgresStore) ListFilters(ctx context.Context) ([]FilterRecord, error) {
	query := `SELECT id, name, category, source_url, local_path, rule_count, enabled, last_updated_at, created_at FROM filters ORDER BY name ASC`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filters []FilterRecord
	for rows.Next() {
		var f FilterRecord
		var lastUp sql.NullTime
		if err := rows.Scan(&f.ID, &f.Name, &f.Category, &f.SourceURL, &f.LocalPath, &f.RuleCount, &f.Enabled, &lastUp, &f.CreatedAt); err != nil {
			return nil, err
		}
		if lastUp.Valid {
			f.LastUpdatedAt = &lastUp.Time
		}
		filters = append(filters, f)
	}
	return filters, rows.Err()
}

// UpsertFilter adds or updates a filter entry.
func (p *PostgresStore) UpsertFilter(ctx context.Context, f *FilterRecord) error {
	query := `
		INSERT INTO filters (id, name, category, source_url, local_path, rule_count, enabled, last_updated_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			category = EXCLUDED.category,
			source_url = EXCLUDED.source_url,
			local_path = EXCLUDED.local_path,
			rule_count = EXCLUDED.rule_count,
			enabled = EXCLUDED.enabled,
			last_updated_at = EXCLUDED.last_updated_at;
	`
	_, err := p.db.ExecContext(ctx, query, f.ID, f.Name, f.Category, f.SourceURL, f.LocalPath, f.RuleCount, f.Enabled, f.LastUpdatedAt)
	return err
}

// DeleteFilter removes a filter feed by ID.
func (p *PostgresStore) DeleteFilter(ctx context.Context, id string) error {
	query := `DELETE FROM filters WHERE id = $1`
	_, err := p.db.ExecContext(ctx, query, id)
	return err
}

// ListAllowlist retrieves all allowlist entries.
func (p *PostgresStore) ListAllowlist(ctx context.Context) ([]AllowlistRecord, error) {
	query := `SELECT id, domain, category, reason, created_by, created_at FROM allowlists ORDER BY domain ASC`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []AllowlistRecord
	for rows.Next() {
		var a AllowlistRecord
		var cat, rsn sql.NullString
		if err := rows.Scan(&a.ID, &a.Domain, &cat, &rsn, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Category = cat.String
		a.Reason = rsn.String
		records = append(records, a)
	}
	return records, rows.Err()
}

// AddAllowlist inserts a domain allowlist entry.
func (p *PostgresStore) AddAllowlist(ctx context.Context, domain, category, reason, createdBy string) error {
	query := `
		INSERT INTO allowlists (id, domain, category, reason, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (domain) DO UPDATE SET
			category = EXCLUDED.category,
			reason = EXCLUDED.reason;
	`
	id := fmt.Sprintf("al-%s", domain)
	_, err := p.db.ExecContext(ctx, query, id, domain, category, reason, createdBy)
	return err
}

// RemoveAllowlist deletes a domain from the allowlist.
func (p *PostgresStore) RemoveAllowlist(ctx context.Context, domain string) error {
	query := `DELETE FROM allowlists WHERE domain = $1`
	_, err := p.db.ExecContext(ctx, query, domain)
	return err
}

var _ = json.Marshal // suppress unused imports
