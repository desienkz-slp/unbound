-- ==============================================================================
-- Netora DNS — PostgreSQL Control Plane Schema Migration 001
-- ==============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Users & RBAC
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(64) PRIMARY KEY,
    username VARCHAR(64) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'READONLY',
    email VARCHAR(128),
    full_name VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roles_permissions (
    role VARCHAR(32) NOT NULL,
    permission VARCHAR(64) NOT NULL,
    PRIMARY KEY (role, permission)
);

-- 2. Nodes & Network Topology
CREATE TABLE IF NOT EXISTS nodes (
    id VARCHAR(64) PRIMARY KEY,
    hostname VARCHAR(128) NOT NULL,
    management_ip VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'HEALTHY',
    unbound_version VARCHAR(64),
    cpu_cores INT NOT NULL DEFAULT 1,
    ram_mb INT NOT NULL DEFAULT 1024,
    config_json JSONB,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS listeners (
    id VARCHAR(64) PRIMARY KEY,
    node_id VARCHAR(64) REFERENCES nodes(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    ip VARCHAR(64) NOT NULL,
    port INT NOT NULL DEFAULT 53,
    role VARCHAR(32) NOT NULL DEFAULT 'CLIENT', -- INTERNET, CLIENT, MANAGEMENT, INTERNAL
    is_ipv6 BOOLEAN NOT NULL DEFAULT FALSE,
    udp BOOLEAN NOT NULL DEFAULT TRUE,
    tcp BOOLEAN NOT NULL DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS client_networks (
    id VARCHAR(64) PRIMARY KEY,
    node_id VARCHAR(64) REFERENCES nodes(id) ON DELETE CASCADE,
    cidr VARCHAR(64) NOT NULL,
    description TEXT,
    action VARCHAR(32) NOT NULL DEFAULT 'allow', -- allow, refuse, deny, allow_snoop
    policy_group VARCHAR(64) DEFAULT 'DEFAULT',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS upstreams (
    id VARCHAR(64) PRIMARY KEY,
    node_id VARCHAR(64) REFERENCES nodes(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    ip VARCHAR(64) NOT NULL,
    port INT NOT NULL DEFAULT 53,
    type VARCHAR(16) NOT NULL DEFAULT 'UDP', -- UDP, TCP, DOT, DOH
    tls_name VARCHAR(128),
    weight INT NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Policy & Filtering
CREATE TABLE IF NOT EXISTS policy_profiles (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) UNIQUE NOT NULL,
    description TEXT,
    resolution_mode VARCHAR(32) NOT NULL DEFAULT 'RECURSIVE',
    safesearch_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    dnssec_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS filters (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(32) NOT NULL, -- ADS, MALWARE, PHISHING, ADULT, GAMBLING, CUSTOM
    source_url TEXT NOT NULL,
    local_path TEXT NOT NULL,
    rule_count INT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS allowlists (
    id VARCHAR(64) PRIMARY KEY,
    domain VARCHAR(255) UNIQUE NOT NULL,
    category VARCHAR(64) DEFAULT 'CUSTOM',
    reason TEXT,
    created_by VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rewrites (
    id VARCHAR(64) PRIMARY KEY,
    domain VARCHAR(255) NOT NULL,
    type VARCHAR(16) NOT NULL DEFAULT 'A',
    value VARCHAR(255) NOT NULL,
    ttl INT NOT NULL DEFAULT 300,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Versioning, Audit & Telemetry
CREATE TABLE IF NOT EXISTS config_versions (
    id VARCHAR(64) PRIMARY KEY,
    node_id VARCHAR(64) REFERENCES nodes(id) ON DELETE CASCADE,
    version_id VARCHAR(64) NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    actor VARCHAR(64) NOT NULL,
    reason TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'STAGED', -- STAGED, ACTIVE, SUPERSEDED, ROLLED_BACK, FAILED
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id VARCHAR(64) PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    target VARCHAR(128) NOT NULL,
    details TEXT,
    client_ip VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'SUCCESS'
);

CREATE TABLE IF NOT EXISTS alerts (
    id VARCHAR(64) PRIMARY KEY,
    node_id VARCHAR(64) REFERENCES nodes(id) ON DELETE CASCADE,
    severity VARCHAR(32) NOT NULL, -- INFO, WARNING, CRITICAL
    alert_type VARCHAR(64) NOT NULL,
    message TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, ACKNOWLEDGED, RESOLVED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

-- Default Role Permissions (RBAC)
INSERT INTO roles_permissions (role, permission) VALUES
('SUPER_ADMIN', 'all'),
('ADMIN', 'nodes:read'), ('ADMIN', 'nodes:write'),
('ADMIN', 'config:read'), ('ADMIN', 'config:write'), ('ADMIN', 'config:apply'), ('ADMIN', 'config:rollback'),
('ADMIN', 'policy:read'), ('ADMIN', 'policy:write'),
('ADMIN', 'audit:read'), ('ADMIN', 'users:read'), ('ADMIN', 'users:write'),
('OPERATOR', 'nodes:read'), ('OPERATOR', 'config:read'), ('OPERATOR', 'config:apply'), ('OPERATOR', 'config:rollback'),
('OPERATOR', 'policy:read'), ('OPERATOR', 'policy:write'), ('OPERATOR', 'audit:read'),
('NOC', 'nodes:read'), ('NOC', 'config:read'), ('NOC', 'audit:read'), ('NOC', 'diagnostics:execute'),
('AUDITOR', 'audit:read'), ('AUDITOR', 'config:read'),
('READONLY', 'nodes:read'), ('READONLY', 'config:read'), ('READONLY', 'policy:read')
ON CONFLICT (role, permission) DO NOTHING;

-- Grant table & sequence privileges to netora application role
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'netora') THEN
    GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO netora;
    GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO netora;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO netora;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO netora;
  END IF;
END $$;
