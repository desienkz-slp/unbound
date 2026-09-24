package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/model"
)

// MemoryStore provides a thread-safe in-memory implementation of Store for tests.
type MemoryStore struct {
	mu           sync.RWMutex
	users        map[string]*auth.User
	nodes        map[string]*NodeRecord
	desiredState map[string]*model.DesiredState
	versions     map[string][]model.ConfigVersion
	auditLogs    []model.AuditRecord
	policies     map[string]*PolicyRecord
	filters      map[string]*FilterRecord
	allowlists   map[string]*AllowlistRecord
}

// NewMemoryStore initializes an in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:        make(map[string]*auth.User),
		nodes:        make(map[string]*NodeRecord),
		desiredState: make(map[string]*model.DesiredState),
		versions:     make(map[string][]model.ConfigVersion),
		policies:     make(map[string]*PolicyRecord),
		filters:      make(map[string]*FilterRecord),
		allowlists:   make(map[string]*AllowlistRecord),
	}
}

func (m *MemoryStore) Close() error {
	return nil
}

func (m *MemoryStore) CreateUser(ctx context.Context, u *auth.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.Username] = u
	return nil
}

func (m *MemoryStore) GetUserByUsername(ctx context.Context, username string) (*auth.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[username]
	if !ok {
		return nil, fmt.Errorf("user '%s' not found", username)
	}
	return u, nil
}

func (m *MemoryStore) ListUsers(ctx context.Context) ([]auth.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []auth.User
	for _, u := range m.users {
		list = append(list, *u)
	}
	return list, nil
}

func (m *MemoryStore) UpsertNode(ctx context.Context, n *model.NodeMetadata, status string, mgmtIP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[n.NodeID] = &NodeRecord{
		ID:             n.NodeID,
		Hostname:       n.Hostname,
		ManagementIP:   mgmtIP,
		Status:         status,
		UnboundVersion: n.UnboundVersion,
		CPUCores:       n.CPUCores,
		RAMMB:          n.TotalRAMMB,
		LastSeenAt:     n.LastSeenAt,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	return nil
}

func (m *MemoryStore) GetNode(ctx context.Context, nodeID string) (*NodeRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node '%s' not found", nodeID)
	}
	return n, nil
}

func (m *MemoryStore) ListNodes(ctx context.Context) ([]NodeRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []NodeRecord
	for _, n := range m.nodes {
		list = append(list, *n)
	}
	return list, nil
}

func (m *MemoryStore) SaveDesiredState(ctx context.Context, state *model.DesiredState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.desiredState[state.NodeID] = state
	return nil
}

func (m *MemoryStore) GetDesiredState(ctx context.Context, nodeID string) (*model.DesiredState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.desiredState[nodeID]
	if !ok {
		return nil, fmt.Errorf("desired state for node '%s' not found", nodeID)
	}
	return s, nil
}

func (m *MemoryStore) SaveConfigVersion(ctx context.Context, v *model.ConfigVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions[v.NodeID] = append([]model.ConfigVersion{*v}, m.versions[v.NodeID]...)
	return nil
}

func (m *MemoryStore) ListConfigVersions(ctx context.Context, nodeID string, limit int) ([]model.ConfigVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vers := m.versions[nodeID]
	if limit > 0 && len(vers) > limit {
		return vers[:limit], nil
	}
	return vers, nil
}

func (m *MemoryStore) LogAudit(ctx context.Context, a *model.AuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditLogs = append([]model.AuditRecord{*a}, m.auditLogs...)
	return nil
}

func (m *MemoryStore) ListAuditLogs(ctx context.Context, limit int) ([]model.AuditRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit > 0 && len(m.auditLogs) > limit {
		return m.auditLogs[:limit], nil
	}
	return m.auditLogs, nil
}

func (m *MemoryStore) ListPolicies(ctx context.Context) ([]PolicyRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []PolicyRecord
	for _, p := range m.policies {
		list = append(list, *p)
	}
	return list, nil
}

func (m *MemoryStore) UpsertPolicy(ctx context.Context, p *PolicyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies[p.Name] = p
	return nil
}

func (m *MemoryStore) ListFilters(ctx context.Context) ([]FilterRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []FilterRecord
	for _, f := range m.filters {
		list = append(list, *f)
	}
	return list, nil
}

func (m *MemoryStore) UpsertFilter(ctx context.Context, f *FilterRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filters[f.ID] = f
	return nil
}

func (m *MemoryStore) ListAllowlist(ctx context.Context) ([]AllowlistRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []AllowlistRecord
	for _, a := range m.allowlists {
		list = append(list, *a)
	}
	return list, nil
}

func (m *MemoryStore) AddAllowlist(ctx context.Context, domain, category, reason, createdBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowlists[domain] = &AllowlistRecord{
		ID:        fmt.Sprintf("al-%s", domain),
		Domain:    domain,
		Category:  category,
		Reason:    reason,
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC(),
	}
	return nil
}

func (m *MemoryStore) RemoveAllowlist(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.allowlists, domain)
	return nil
}
