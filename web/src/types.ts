export type ListenerRole = 'INTERNET' | 'CLIENT' | 'MANAGEMENT' | 'INTERNAL';
export type ResolutionMode = 'RECURSIVE' | 'FORWARDING' | 'HYBRID';
export type DecisionAction = 'ALLOW' | 'BLOCKED' | 'REWRITE';

export interface Listener {
  id: string;
  name: string;
  ip: string;
  port: number;
  role: ListenerRole;
  is_ipv6: boolean;
  udp: boolean;
  tcp: boolean;
  enabled: boolean;
}

export interface ClientNetwork {
  id: string;
  cidr: string;
  description: string;
  action: string;
  policy_group: string;
  enabled: boolean;
}

export interface UpstreamServer {
  id: string;
  name: string;
  ip: string;
  port: number;
  type: string;
  enabled: boolean;
}

export interface DesiredState {
  version_id: string;
  node_id: string;
  resolution_mode: ResolutionMode;
  listeners: Listener[];
  client_networks: ClientNetwork[];
  upstreams: UpstreamServer[];
  cache: {
    msg_cache_size: string;
    rrset_cache_size: string;
    key_cache_size: string;
    neg_cache_size: string;
    min_ttl: number;
    max_ttl: number;
    infra_host_ttl: number;
  };
  performance: {
    profile: string;
    num_threads: number;
    msg_cache_slabs: number;
    rrset_cache_slabs: number;
    so_reuseport: boolean;
    so_rcvbuf: string;
    so_sndbuf: string;
    num_queries_per_thread: number;
    outgoing_range: number;
  };
  dnssec: {
    enabled: boolean;
    auto_trust_anchor_file: string;
    root_hints_file: string;
    val_permissive_mode: boolean;
    insecure_domains: string[];
  };
  features: {
    prefetch: boolean;
    prefetch_key: boolean;
    serve_expired: boolean;
    serve_expired_ttl: number;
    serve_expired_client_timeout: number;
    qname_minimisation: boolean;
    hide_identity: boolean;
    hide_version: boolean;
  };
  safe_search: {
    enabled: boolean;
    google_redirect_ipv4: string;
    bing_redirect_ipv4: string;
  };
  local_records: Array<{
    domain: string;
    type: string;
    value: string;
    ttl: number;
  }>;
  remote_control: {
    enabled: boolean;
    interface: string;
    port: number;
  };
}

export interface NodeRecord {
  id: string;
  hostname: string;
  management_ip: string;
  status: string;
  unbound_version: string;
  cpu_cores: number;
  ram_mb: number;
  last_seen_at: string;
}

export interface DiagnosisResult {
  domain: string;
  policy: string;
  action: DecisionAction;
  category?: string;
  source?: string;
  rule?: string;
  explanation: string;
  rewrite_value?: string;
  live_resolution: {
    healthy: boolean;
    latency: string;
    resolved_ips?: string[];
    error?: string;
  };
}

export interface ConfigVersion {
  version_id: string;
  node_id: string;
  checksum: string;
  actor: string;
  reason: string;
  status: string;
  applied_at?: string;
  created_at: string;
}

export interface AuditRecord {
  id: string;
  timestamp: string;
  actor: string;
  action: string;
  target: string;
  details?: string;
  client_ip?: string;
  status: string;
}

export interface AllowlistRecord {
  id: string;
  domain: string;
  category: string;
  reason: string;
  created_by: string;
  created_at: string;
}
