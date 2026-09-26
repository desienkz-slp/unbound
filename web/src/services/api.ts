import {
  AuditRecord,
  ConfigVersion,
  DesiredState,
  DiagnosisResult,
  NodeRecord,
  AllowlistRecord,
  FilterRecord,
  AutoUpdateStatus,
  SyncBlocklistResult,
  DetailedStatsReport,
  SystemVersion,
} from '../types';

const API_BASE = '/api/v1';

class ApiClient {
  private token: string | null = null;

  constructor() {
    this.token = localStorage.getItem('netora_token');
  }

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem('netora_token', token);
    } else {
      localStorage.removeItem('netora_token');
    }
  }

  getToken(): string | null {
    return this.token;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });

    if (response.status === 401) {
      this.setToken(null);
      throw new Error('Unauthorized');
    }

    if (!response.ok) {
      let errMsg = `Request failed: ${response.statusText}`;
      try {
        const errJson = await response.json();
        if (errJson.error) errMsg = errJson.error;
      } catch {
        // ignore
      }
      throw new Error(errMsg);
    }

    return response.json();
  }

  // Auth
  async login(username: string, password: string): Promise<{ token: string; user: { id: string; username: string; role: string } }> {
    const res = await this.request<{ token: string; user: { id: string; username: string; role: string } }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    this.setToken(res.token);
    return res;
  }

  async logout(): Promise<void> {
    try {
      await this.request('/auth/logout', { method: 'POST' });
    } finally {
      this.setToken(null);
    }
  }

  async updateProfile(data: {
    username?: string;
    current_password: string;
    new_password?: string;
  }): Promise<{ success: boolean; message: string; token: string; user: { id: string; username: string; role: string; full_name?: string } }> {
    const res = await this.request<{ success: boolean; message: string; token: string; user: { id: string; username: string; role: string; full_name?: string } }>('/auth/profile', {
      method: 'POST',
      body: JSON.stringify(data),
    });
    if (res.token) {
      this.setToken(res.token);
    }
    return res;
  }

  // Nodes
  async getNodes(): Promise<NodeRecord[]> {
    return this.request<NodeRecord[]>('/nodes');
  }

  async getNodeHealth(nodeId: string): Promise<{ healthy: boolean; unbound_running: boolean; port53_listening: boolean; last_check: string; latency_ms: number }> {
    return this.request(`/nodes/${nodeId}/health`);
  }

  async getNodeStats(nodeId: string): Promise<DetailedStatsReport> {
    return this.request<DetailedStatsReport>(`/nodes/${nodeId}/stats`);
  }

  // Desired State / Config
  async getDesiredState(nodeId: string): Promise<DesiredState> {
    return this.request<DesiredState>(`/nodes/${nodeId}/config`);
  }

  async applyConfig(nodeId: string, state: DesiredState, reason: string): Promise<{ success: boolean; version_id: string; checksum: string; message: string }> {
    return this.request(`/nodes/${nodeId}/config/apply`, {
      method: 'POST',
      body: JSON.stringify({ state, reason }),
    });
  }

  async rollbackConfig(nodeId: string, versionId: string): Promise<{ success: boolean; rolled_back_to: string; message: string }> {
    return this.request(`/nodes/${nodeId}/config/rollback`, {
      method: 'POST',
      body: JSON.stringify({ version_id: versionId }),
    });
  }

  async getConfigVersions(nodeId: string): Promise<ConfigVersion[]> {
    return this.request<ConfigVersion[]>(`/nodes/${nodeId}/config/versions`);
  }

  // Diagnostics ("Why Blocked?")
  async diagnoseDomain(domain: string, clientIp?: string, policy?: string): Promise<DiagnosisResult> {
    return this.request<DiagnosisResult>('/diagnose', {
      method: 'POST',
      body: JSON.stringify({ domain, client_ip: clientIp, policy: policy || 'DEFAULT' }),
    });
  }

  // Filter Feeds (Blocklists / Hosts files)
  async getFilters(): Promise<FilterRecord[]> {
    return this.request<FilterRecord[]>('/filters');
  }

  async addFilter(name: string, category: string, source_url: string, local_path?: string): Promise<FilterRecord> {
    return this.request<FilterRecord>('/filters', {
      method: 'POST',
      body: JSON.stringify({ name, category, source_url, local_path }),
    });
  }

  async deleteFilter(id: string): Promise<void> {
    return this.request(`/filters/${id}`, {
      method: 'DELETE',
    });
  }

  async toggleFilter(id: string, enabled: boolean): Promise<FilterRecord> {
    return this.request<FilterRecord>(`/filters/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    });
  }

  async updateFilter(id: string, patch: Partial<FilterRecord>): Promise<FilterRecord> {
    return this.request<FilterRecord>(`/filters/${id}`, {
      method: 'PUT',
      body: JSON.stringify(patch),
    });
  }

  async syncBlocklists(): Promise<SyncBlocklistResult> {
    return this.request<SyncBlocklistResult>('/blocklists/sync', {
      method: 'POST',
    });
  }

  // Alias for backward compatibility
  async syncBlocklistsFromGitHub(): Promise<SyncBlocklistResult> {
    return this.syncBlocklists();
  }

  async getAutoUpdateStatus(): Promise<AutoUpdateStatus> {
    return this.request<AutoUpdateStatus>('/blocklists/auto-update-status');
  }

  // Allowlist
  async getAllowlist(): Promise<AllowlistRecord[]> {
    return this.request<AllowlistRecord[]>('/allowlist');
  }

  async addAllowlist(domain: string, category: string, reason: string): Promise<AllowlistRecord> {
    return this.request<AllowlistRecord>('/allowlist', {
      method: 'POST',
      body: JSON.stringify({ domain, category, reason }),
    });
  }

  async deleteAllowlist(id: string): Promise<void> {
    return this.request(`/allowlist/${id}`, {
      method: 'DELETE',
    });
  }

  // Audit Logs
  async getAuditLogs(): Promise<AuditRecord[]> {
    return this.request<AuditRecord[]>('/audit');
  }

  // System Version
  async getSystemVersion(): Promise<SystemVersion> {
    try {
      const response = await fetch(`${API_BASE}/system/version`);
      if (response.ok) {
        return await response.json();
      }
    } catch {
      // Fallback below
    }

    try {
      const res = await fetch('/version.json');
      if (res.ok) {
        return await res.json();
      }
    } catch {
      // Static fallback
    }

    return {
      version: 'v1.0.44',
      build: 44,
      commit_count: 44,
      git_commit: 'de28649',
      git_branch: 'main',
      build_time: new Date().toISOString(),
    };
  }
}

export const api = new ApiClient();
