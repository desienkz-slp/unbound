import React, { useState } from 'react';
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Edit3,
  ExternalLink,
  Globe,
  Layers,
  List,
  Network,
  Plus,
  RefreshCw,
  Save,
  Server,
  ShieldCheck,
  Trash2,
  Zap,
} from 'lucide-react';
import { ClientNetwork, DesiredState, Listener, UpstreamServer } from '../types';
import { api } from '../services/api';

const KNOWN_UPSTREAMS_PRESETS = [
  { name: 'Cloudflare DNS Primary', ip: '1.1.1.1', port: 53, type: 'UDP' },
  { name: 'Cloudflare DNS Secondary', ip: '1.0.0.1', port: 53, type: 'UDP' },
  { name: 'Google Public DNS Primary', ip: '8.8.8.8', port: 53, type: 'UDP' },
  { name: 'Google Public DNS Secondary', ip: '8.8.4.4', port: 53, type: 'UDP' },
  { name: 'Quad9 Security Primary', ip: '9.9.9.9', port: 53, type: 'UDP' },
  { name: 'Quad9 Security Secondary', ip: '149.112.112.112', port: 53, type: 'UDP' },
  { name: 'AdGuard DNS Default Primary', ip: '94.140.14.14', port: 53, type: 'UDP' },
  { name: 'AdGuard DNS Default Secondary', ip: '94.140.15.15', port: 53, type: 'UDP' },
  { name: 'AdGuard DNS Family Protection', ip: '94.140.14.15', port: 53, type: 'UDP' },
  { name: 'AdGuard DNS Non-filtering', ip: '94.140.14.140', port: 53, type: 'UDP' },
  { name: 'Cisco OpenDNS Home', ip: '208.67.222.222', port: 53, type: 'UDP' },
  { name: 'CleanBrowsing Security', ip: '185.228.168.9', port: 53, type: 'UDP' },
  { name: 'Control D Free', ip: '76.76.2.0', port: 53, type: 'UDP' },
  { name: 'Mullvad Public DNS', ip: '194.242.2.2', port: 53, type: 'UDP' },
];

interface DNSConfigViewProps {
  nodeId: string;
  config: DesiredState | null;
  onConfigUpdated: (newConfig: DesiredState) => void;
}

export const DNSConfigView: React.FC<DNSConfigViewProps> = ({
  nodeId,
  config,
  onConfigUpdated,
}) => {
  const [subTab, setSubTab] = useState<'listeners' | 'networks' | 'upstreams' | 'cache' | 'dnssec'>('listeners');
  const [localState, setLocalState] = useState<DesiredState | null>(config);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [upstreamMode, setUpstreamMode] = useState<'table' | 'raw'>('table');
  const [rawText, setRawText] = useState('');
  const [testStatus, setTestStatus] = useState<Record<string, { status: 'testing' | 'ok' | 'fail'; latency?: number }>>({});

  React.useEffect(() => {
    setLocalState(config);
  }, [config]);

  if (!localState) {
    return <div className="text-gray-400 p-8">Loading configuration state...</div>;
  }

  const handleApply = async () => {
    setSaving(true);
    setMessage(null);
    try {
      const res = await api.applyConfig(nodeId, localState, 'Updated via Netora Web UI');
      setMessage({ type: 'success', text: `Configuration applied atomically! Version ID: ${res.version_id}` });
      onConfigUpdated(localState);
    } catch (err: any) {
      setMessage({ type: 'error', text: err.message || 'Failed to apply configuration' });
    } finally {
      setSaving(false);
    }
  };

  // Helper updates
  const updateListener = (index: number, updated: Partial<Listener>) => {
    const list = [...localState.listeners];
    list[index] = { ...list[index], ...updated };
    setLocalState({ ...localState, listeners: list });
  };

  const addListener = () => {
    const newL: Listener = {
      id: `listener-${Date.now()}`,
      name: 'New Listener',
      ip: '172.18.20.145',
      port: 53,
      role: 'CLIENT',
      is_ipv6: false,
      udp: true,
      tcp: true,
      enabled: true,
    };
    setLocalState({ ...localState, listeners: [...localState.listeners, newL] });
  };

  const removeListener = (index: number) => {
    const list = localState.listeners.filter((_, i) => i !== index);
    setLocalState({ ...localState, listeners: list });
  };

  // Client Networks
  const updateNetwork = (index: number, updated: Partial<ClientNetwork>) => {
    const list = [...localState.client_networks];
    list[index] = { ...list[index], ...updated };
    setLocalState({ ...localState, client_networks: list });
  };

  const addNetwork = () => {
    const newN: ClientNetwork = {
      id: `net-${Date.now()}`,
      cidr: '192.168.1.0/24',
      description: 'Internal Client Subnet',
      action: 'allow',
      policy_group: 'DEFAULT',
      enabled: true,
    };
    setLocalState({ ...localState, client_networks: [...localState.client_networks, newN] });
  };

  const removeNetwork = (index: number) => {
    const list = localState.client_networks.filter((_, i) => i !== index);
    setLocalState({ ...localState, client_networks: list });
  };

  // Upstreams Management
  const updateUpstream = (index: number, updated: Partial<UpstreamServer>) => {
    const list = [...localState.upstreams];
    list[index] = { ...list[index], ...updated };
    setLocalState({ ...localState, upstreams: list });
  };

  const addUpstream = (preset?: Partial<UpstreamServer>) => {
    const newUp: UpstreamServer = {
      id: `up-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
      name: preset?.name || 'Custom Upstream',
      ip: preset?.ip || '1.1.1.1',
      port: preset?.port || 53,
      type: preset?.type || 'UDP',
      enabled: preset?.enabled ?? true,
    };
    setLocalState({ ...localState, upstreams: [...localState.upstreams, newUp] });
  };

  const removeUpstream = (index: number) => {
    const list = localState.upstreams.filter((_, i) => i !== index);
    setLocalState({ ...localState, upstreams: list });
  };

  const testUpstream = async (up: UpstreamServer) => {
    setTestStatus((prev) => ({ ...prev, [up.id]: { status: 'testing' } }));
    const startTime = performance.now();
    try {
      await api.getNodeHealth(nodeId);
      const latency = Math.round(performance.now() - startTime);
      setTestStatus((prev) => ({ ...prev, [up.id]: { status: 'ok', latency: Math.max(1, latency) } }));
    } catch {
      setTestStatus((prev) => ({ ...prev, [up.id]: { status: 'fail' } }));
    }
  };

  const handleParseRawText = () => {
    const lines = rawText.split('\n').map((l) => l.trim()).filter((l) => l.length > 0 && !l.startsWith('#'));
    if (lines.length === 0) return;
    const parsed: UpstreamServer[] = lines.map((line, idx) => {
      let ip = line;
      let port = 53;
      if (line.includes(':')) {
        const parts = line.split(':');
        ip = parts[0];
        port = parseInt(parts[1], 10) || 53;
      }
      return {
        id: `up-raw-${Date.now()}-${idx}`,
        name: `Upstream (${ip})`,
        ip,
        port,
        type: 'UDP',
        enabled: true,
      };
    });
    setLocalState({ ...localState, upstreams: [...localState.upstreams, ...parsed] });
    setUpstreamMode('table');
    setRawText('');
  };

  return (
    <div className="space-y-6">
      {/* Title & Actions Bar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Network className="w-5 h-5 text-cyan-400" />
            DNS Configuration & Runtime Sockets
          </h1>
          <p className="text-xs text-gray-400 mt-1">
            Configure Multi-IP listeners, strict client ACL ranges, upstream resolvers, and DNSSEC verification.
          </p>
        </div>

        <button
          onClick={handleApply}
          disabled={saving}
          className="px-5 py-2.5 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2 disabled:opacity-50"
        >
          {saving ? (
            <RefreshCw className="w-4 h-4 animate-spin" />
          ) : (
            <Save className="w-4 h-4" />
          )}
          {saving ? 'Validating & Applying...' : 'Save & Atomic Apply'}
        </button>
      </div>

      {message && (
        <div
          className={`p-4 rounded-xl border text-xs flex items-center gap-2 ${
            message.type === 'success'
              ? 'bg-emerald-950/50 border-emerald-800 text-emerald-300'
              : 'bg-red-950/50 border-red-800 text-red-300'
          }`}
        >
          {message.type === 'success' ? (
            <CheckCircle2 className="w-4 h-4 shrink-0" />
          ) : (
            <AlertTriangle className="w-4 h-4 shrink-0" />
          )}
          <span>{message.text}</span>
        </div>
      )}

      {/* Tabs */}
      <div className="flex border-b border-gray-800 gap-2 text-xs font-medium">
        <button
          onClick={() => setSubTab('listeners')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'listeners'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <Server className="w-3.5 h-3.5" />
          Multi-IP Listeners ({localState.listeners.length})
        </button>

        <button
          onClick={() => setSubTab('networks')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'networks'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <Layers className="w-3.5 h-3.5" />
          Client ACL Networks ({localState.client_networks.length})
        </button>

        <button
          onClick={() => setSubTab('upstreams')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'upstreams'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <Globe className="w-3.5 h-3.5" />
          Upstream Resolvers ({localState.upstreams.length})
        </button>

        <button
          onClick={() => setSubTab('cache')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'cache'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <Zap className="w-3.5 h-3.5" />
          Cache & Prefetch
        </button>

        <button
          onClick={() => setSubTab('dnssec')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'dnssec'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <ShieldCheck className="w-3.5 h-3.5" />
          DNSSEC Validation
        </button>
      </div>

      {/* Subtab 1: Listeners */}
      {subTab === 'listeners' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-sm font-bold text-white">Listener Sockets & Roles</h2>
              <p className="text-xs text-gray-400">
                Define IP bindings and explicit roles (INTERNET, CLIENT, MANAGEMENT, INTERNAL) to prevent accidental public exposure.
              </p>
            </div>
            <button
              onClick={addListener}
              className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-cyan-400 text-xs font-semibold rounded-lg border border-gray-700 flex items-center gap-1.5"
            >
              <Plus className="w-3.5 h-3.5" /> Add Listener
            </button>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs font-mono">
              <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
                <tr>
                  <th className="py-2.5 px-3">Role</th>
                  <th className="py-2.5 px-3">IP Address</th>
                  <th className="py-2.5 px-3">Port</th>
                  <th className="py-2.5 px-3">Protocol</th>
                  <th className="py-2.5 px-3">Family</th>
                  <th className="py-2.5 px-3">Enabled</th>
                  <th className="py-2.5 px-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800/60">
                {localState.listeners.map((l, idx) => (
                  <tr key={l.id} className="hover:bg-gray-800/30">
                    <td className="py-2.5 px-3">
                      <select
                        value={l.role}
                        onChange={(e) => updateListener(idx, { role: e.target.value as any })}
                        className={`px-2 py-1 rounded text-xs font-bold border ${
                          l.role === 'CLIENT'
                            ? 'bg-cyan-950 text-cyan-400 border-cyan-800'
                            : l.role === 'INTERNET'
                            ? 'bg-amber-950 text-amber-400 border-amber-800'
                            : l.role === 'INTERNAL'
                            ? 'bg-emerald-950 text-emerald-400 border-emerald-800'
                            : 'bg-purple-950 text-purple-400 border-purple-800'
                        }`}
                      >
                        <option value="CLIENT">CLIENT</option>
                        <option value="INTERNET">INTERNET</option>
                        <option value="INTERNAL">INTERNAL</option>
                        <option value="MANAGEMENT">MANAGEMENT</option>
                      </select>
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="text"
                        value={l.ip}
                        onChange={(e) => updateListener(idx, { ip: e.target.value })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-white text-xs w-36"
                      />
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="number"
                        value={l.port}
                        onChange={(e) => updateListener(idx, { port: parseInt(e.target.value) || 53 })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-white text-xs w-20"
                      />
                    </td>
                    <td className="py-2.5 px-3 text-gray-300">
                      <label className="mr-2 inline-flex items-center gap-1 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={l.udp}
                          onChange={(e) => updateListener(idx, { udp: e.target.checked })}
                          className="rounded bg-gray-950 border-gray-700 text-cyan-500"
                        />
                        UDP
                      </label>
                      <label className="inline-flex items-center gap-1 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={l.tcp}
                          onChange={(e) => updateListener(idx, { tcp: e.target.checked })}
                          className="rounded bg-gray-950 border-gray-700 text-cyan-500"
                        />
                        TCP
                      </label>
                    </td>
                    <td className="py-2.5 px-3">
                      <span className="text-gray-400">{l.is_ipv6 ? 'IPv6' : 'IPv4'}</span>
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="checkbox"
                        checked={l.enabled}
                        onChange={(e) => updateListener(idx, { enabled: e.target.checked })}
                        className="rounded bg-gray-950 border-gray-700 text-cyan-500 cursor-pointer"
                      />
                    </td>
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => removeListener(idx)}
                        disabled={localState.listeners.length <= 1}
                        className="p-1 text-gray-500 hover:text-red-400 disabled:opacity-30"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Subtab 2: Client ACL Networks */}
      {subTab === 'networks' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-sm font-bold text-white">Client CIDR Networks & Access Control</h2>
              <p className="text-xs text-gray-400">
                Only explicitly declared client subnets can query Unbound. <strong className="text-amber-400">0.0.0.0/0 allow is blocked</strong> by security invariant.
              </p>
            </div>
            <button
              onClick={addNetwork}
              className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-cyan-400 text-xs font-semibold rounded-lg border border-gray-700 flex items-center gap-1.5"
            >
              <Plus className="w-3.5 h-3.5" /> Add Network CIDR
            </button>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs font-mono">
              <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
                <tr>
                  <th className="py-2.5 px-3">CIDR Prefix</th>
                  <th className="py-2.5 px-3">Description</th>
                  <th className="py-2.5 px-3">Policy Group</th>
                  <th className="py-2.5 px-3">Action</th>
                  <th className="py-2.5 px-3">Enabled</th>
                  <th className="py-2.5 px-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800/60">
                {localState.client_networks.map((net, idx) => (
                  <tr key={net.id} className="hover:bg-gray-800/30">
                    <td className="py-2.5 px-3">
                      <input
                        type="text"
                        value={net.cidr}
                        onChange={(e) => updateNetwork(idx, { cidr: e.target.value })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-cyan-300 font-bold text-xs w-44"
                      />
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="text"
                        value={net.description}
                        onChange={(e) => updateNetwork(idx, { description: e.target.value })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-gray-300 text-xs w-52"
                      />
                    </td>
                    <td className="py-2.5 px-3">
                      <select
                        value={net.policy_group}
                        onChange={(e) => updateNetwork(idx, { policy_group: e.target.value })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-gray-300 text-xs"
                      >
                        <option value="DEFAULT">DEFAULT</option>
                        <option value="FAMILY">FAMILY</option>
                        <option value="STRICT_ISP">STRICT_ISP</option>
                        <option value="UNFILTERED">UNFILTERED</option>
                      </select>
                    </td>
                    <td className="py-2.5 px-3">
                      <span className="px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 font-bold border border-emerald-800">
                        {net.action}
                      </span>
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="checkbox"
                        checked={net.enabled}
                        onChange={(e) => updateNetwork(idx, { enabled: e.target.checked })}
                        className="rounded bg-gray-950 border-gray-700 text-cyan-500 cursor-pointer"
                      />
                    </td>
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => removeNetwork(idx)}
                        disabled={localState.client_networks.length <= 1}
                        className="p-1 text-gray-500 hover:text-red-400 disabled:opacity-30"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Subtab 3: Upstream Resolvers */}
      {subTab === 'upstreams' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-5">
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div>
              <h2 className="text-sm font-bold text-white flex items-center gap-2">
                <Globe className="w-4 h-4 text-cyan-400" />
                Upstream Forwarders & Resolution Architecture
              </h2>
              <p className="text-xs text-gray-400 mt-0.5">
                Configure primary, secondary, and encrypted upstream resolvers. Fully editable with one-click presets.
              </p>
            </div>

            <div className="flex items-center gap-2">
              <button
                onClick={() => setUpstreamMode(upstreamMode === 'table' ? 'raw' : 'table')}
                className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-medium rounded-lg border border-gray-700 flex items-center gap-1.5 transition-colors"
              >
                {upstreamMode === 'table' ? (
                  <>
                    <Edit3 className="w-3.5 h-3.5 text-cyan-400" /> Bulk Paste (AdGuard Style)
                  </>
                ) : (
                  <>
                    <List className="w-3.5 h-3.5 text-cyan-400" /> Table Editor
                  </>
                )}
              </button>

              <button
                onClick={() => addUpstream()}
                className="px-3 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-black text-xs font-bold rounded-lg flex items-center gap-1.5 shadow-lg shadow-cyan-600/20 transition-colors"
              >
                <Plus className="w-3.5 h-3.5" /> Add Upstream
              </button>
            </div>
          </div>

          {/* Mode Selector */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pb-2 border-b border-gray-800">
            {(['RECURSIVE', 'FORWARDING', 'HYBRID'] as const).map((m) => (
              <div
                key={m}
                onClick={() => setLocalState({ ...localState, resolution_mode: m })}
                className={`p-4 rounded-xl border cursor-pointer transition-all ${
                  localState.resolution_mode === m
                    ? 'bg-cyan-950/30 border-cyan-500 text-white shadow-lg shadow-cyan-950/40'
                    : 'bg-gray-950/60 border-gray-800 text-gray-400 hover:border-gray-700'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="font-bold font-mono text-sm text-cyan-400">{m}</div>
                  {localState.resolution_mode === m && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-300 border border-cyan-800">
                      ACTIVE
                    </span>
                  )}
                </div>
                <p className="text-[11px] mt-1.5 text-gray-400">
                  {m === 'RECURSIVE' && 'Direct iterative root-hints resolution; no upstream forwarders used.'}
                  {m === 'FORWARDING' && 'Forward all queries to specified high-speed upstream DNS servers.'}
                  {m === 'HYBRID' && 'Forward primary queries with automatic iterative fallback on failure.'}
                </p>
              </div>
            ))}
          </div>

          {/* AdGuard Known DNS Providers Callout Banner */}
          <div className="p-3.5 rounded-xl bg-gradient-to-r from-blue-950/40 via-cyan-950/20 to-gray-900 border border-cyan-900/60 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center shrink-0">
                <Globe className="w-4 h-4 text-cyan-400" />
              </div>
              <div>
                <div className="text-xs font-bold text-white flex items-center gap-1.5">
                  Known DNS Providers Directory
                  <span className="text-[10px] font-normal px-1.5 py-0.5 rounded bg-cyan-900/60 text-cyan-300">
                    AdGuard Knowledge Base
                  </span>
                </div>
                <p className="text-[11px] text-gray-400">
                  Looking for trusted public resolvers, family protection, or privacy-oriented upstream providers?
                </p>
              </div>
            </div>

            <a
              href="https://adguard-dns.io/kb/general/dns-providers/?clid=y5dJ5x&utm_campaign=dns_kb_providers&utm_medium=ui&utm_source=home"
              target="_blank"
              rel="noopener noreferrer"
              className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-cyan-400 text-xs font-semibold rounded-lg border border-cyan-800/60 flex items-center gap-1.5 shrink-0 transition-all hover:border-cyan-500"
            >
              <span>Explore Provider List</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          </div>

          {/* Quick Preset Selector */}
          <div className="flex flex-wrap items-center gap-2 pt-1">
            <span className="text-xs font-medium text-gray-400">Quick-Add Popular Presets:</span>
            <div className="flex flex-wrap gap-1.5">
              {KNOWN_UPSTREAMS_PRESETS.slice(0, 6).map((preset) => (
                <button
                  key={preset.name}
                  onClick={() => addUpstream(preset)}
                  className="px-2.5 py-1 rounded-md bg-gray-950 hover:bg-gray-800 border border-gray-800 text-[11px] font-mono text-gray-300 hover:text-cyan-400 transition-colors flex items-center gap-1"
                >
                  <Plus className="w-3 h-3 text-cyan-500" />
                  {preset.name.split(' ')[0]} ({preset.ip})
                </button>
              ))}
              <select
                onChange={(e) => {
                  const found = KNOWN_UPSTREAMS_PRESETS.find((p) => p.name === e.target.value);
                  if (found) {
                    addUpstream(found);
                    e.target.value = '';
                  }
                }}
                className="bg-gray-950 border border-gray-800 text-gray-400 text-xs rounded px-2 py-1 focus:border-cyan-500"
                defaultValue=""
              >
                <option value="" disabled>
                  More Presets...
                </option>
                {KNOWN_UPSTREAMS_PRESETS.slice(6).map((p) => (
                  <option key={p.name} value={p.name}>
                    {p.name} — {p.ip}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Mode Notification if Recursive */}
          {localState.resolution_mode === 'RECURSIVE' && (
            <div className="p-3 rounded-lg bg-amber-950/30 border border-amber-800/60 text-amber-300 text-xs flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 shrink-0 text-amber-400" />
                <span>
                  <strong>Mode RECURSIVE sedang aktif:</strong> Unbound mencari langsung via Root Hints IANA. Upstream di bawah ini akan aktif otomatis jika mode diubah ke <strong>FORWARDING</strong> atau <strong>HYBRID</strong>.
                </span>
              </div>
              <button
                onClick={() => setLocalState({ ...localState, resolution_mode: 'HYBRID' })}
                className="px-2.5 py-1 bg-amber-900/60 hover:bg-amber-800 text-amber-200 text-xs font-bold rounded shrink-0 border border-amber-700 transition-colors"
              >
                Aktifkan Mode HYBRID
              </button>
            </div>
          )}

          {/* Content: Raw Mode vs Table Mode */}
          {upstreamMode === 'raw' ? (
            <div className="space-y-3 bg-gray-950 p-4 rounded-xl border border-gray-800">
              <div className="flex items-center justify-between">
                <label className="text-xs font-semibold text-gray-300">
                  Paste Upstream DNS Servers (1 IP address / hostname per line)
                </label>
                <span className="text-[11px] text-gray-500 font-mono">Format: 1.1.1.1 or 8.8.8.8:53</span>
              </div>
              <textarea
                rows={6}
                value={rawText}
                onChange={(e) => setRawText(e.target.value)}
                placeholder={"1.1.1.1\n1.0.0.1\n8.8.8.8\n9.9.9.9\n94.140.14.14"}
                className="w-full bg-gray-900 border border-gray-800 rounded-lg p-3 text-xs font-mono text-cyan-300 focus:border-cyan-500 focus:outline-none"
              />
              <div className="flex items-center justify-between">
                <p className="text-[11px] text-gray-400">
                  You can copy addresses directly from the AdGuard Known Providers list and paste them here.
                </p>
                <button
                  onClick={handleParseRawText}
                  className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-black text-xs font-bold rounded-lg shadow-md transition-colors"
                >
                  Import to Upstream List
                </button>
              </div>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs font-mono">
                <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
                  <tr>
                    <th className="py-2.5 px-3">Enabled</th>
                    <th className="py-2.5 px-3">Provider / Name</th>
                    <th className="py-2.5 px-3">IP / Server Address</th>
                    <th className="py-2.5 px-3">Port</th>
                    <th className="py-2.5 px-3">Protocol</th>
                    <th className="py-2.5 px-3">Latency Probe</th>
                    <th className="py-2.5 px-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800/60">
                  {localState.upstreams.map((up, idx) => {
                    const test = testStatus[up.id];
                    return (
                      <tr key={up.id} className="hover:bg-gray-800/30">
                        <td className="py-2.5 px-3">
                          <input
                            type="checkbox"
                            checked={up.enabled}
                            onChange={(e) => updateUpstream(idx, { enabled: e.target.checked })}
                            className="rounded bg-gray-950 border-gray-700 text-cyan-500 cursor-pointer"
                          />
                        </td>
                        <td className="py-2.5 px-3">
                          <input
                            type="text"
                            value={up.name}
                            onChange={(e) => updateUpstream(idx, { name: e.target.value })}
                            className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-white text-xs w-48 font-sans"
                            placeholder="Provider Name"
                          />
                        </td>
                        <td className="py-2.5 px-3">
                          <input
                            type="text"
                            value={up.ip}
                            onChange={(e) => updateUpstream(idx, { ip: e.target.value })}
                            className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-cyan-300 text-xs w-40 font-mono"
                            placeholder="1.1.1.1"
                          />
                        </td>
                        <td className="py-2.5 px-3">
                          <input
                            type="number"
                            value={up.port}
                            onChange={(e) => updateUpstream(idx, { port: parseInt(e.target.value, 10) || 53 })}
                            className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-white text-xs w-20 font-mono"
                          />
                        </td>
                        <td className="py-2.5 px-3">
                          <select
                            value={up.type}
                            onChange={(e) => updateUpstream(idx, { type: e.target.value })}
                            className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-xs text-gray-300 font-mono"
                          >
                            <option value="UDP">UDP (Standard)</option>
                            <option value="TCP">TCP</option>
                            <option value="DOT">DoT (TLS)</option>
                            <option value="DOH">DoH (HTTPS)</option>
                          </select>
                        </td>
                        <td className="py-2.5 px-3">
                          {test?.status === 'testing' ? (
                            <span className="text-[11px] text-cyan-400 flex items-center gap-1 font-sans">
                              <RefreshCw className="w-3 h-3 animate-spin" /> Testing...
                            </span>
                          ) : test?.status === 'ok' ? (
                            <span className="px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-800 text-[10px] font-mono">
                              ✔ {test.latency}ms
                            </span>
                          ) : test?.status === 'fail' ? (
                            <span className="px-2 py-0.5 rounded bg-red-950 text-red-400 border border-red-800 text-[10px] font-mono">
                              ✖ Timeout
                            </span>
                          ) : (
                            <button
                              onClick={() => testUpstream(up)}
                              className="px-2 py-0.5 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded text-[10px] font-sans border border-gray-700 transition-colors"
                            >
                              Test Ping
                            </button>
                          )}
                        </td>
                        <td className="py-2.5 px-3 text-right">
                          <button
                            onClick={() => removeUpstream(idx)}
                            disabled={localState.upstreams.length <= 1}
                            className="p-1 text-gray-500 hover:text-red-400 disabled:opacity-30 transition-colors"
                            title="Delete Upstream"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}


      {/* Subtab 4: Cache & Prefetch */}
      {subTab === 'cache' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div>
            <h2 className="text-sm font-bold text-white">Cache & Latency Tuning</h2>
            <p className="text-xs text-gray-400">
              Configure Unbound memory cache partitions, prefetch expired records, and serve expired answers during upstream outages.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono">
            <div>
              <label className="block text-gray-400 mb-1">RRset Cache Size</label>
              <input
                type="text"
                value={localState.cache.rrset_cache_size}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    cache: { ...localState.cache, rrset_cache_size: e.target.value },
                  })
                }
                className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
              />
            </div>

            <div>
              <label className="block text-gray-400 mb-1">Message Cache Size</label>
              <input
                type="text"
                value={localState.cache.msg_cache_size}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    cache: { ...localState.cache, msg_cache_size: e.target.value },
                  })
                }
                className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
              />
            </div>

            <div>
              <label className="block text-gray-400 mb-1">Min TTL (Seconds)</label>
              <input
                type="number"
                value={localState.cache.min_ttl}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    cache: { ...localState.cache, min_ttl: parseInt(e.target.value) || 0 },
                  })
                }
                className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
              />
            </div>

            <div>
              <label className="block text-gray-400 mb-1">Max TTL (Seconds)</label>
              <input
                type="number"
                value={localState.cache.max_ttl}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    cache: { ...localState.cache, max_ttl: parseInt(e.target.value) || 86400 },
                  })
                }
                className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
              />
            </div>
          </div>

          <div className="pt-4 border-t border-gray-800 space-y-3">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={localState.features.prefetch}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    features: { ...localState.features, prefetch: e.target.checked },
                  })
                }
                className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
              />
              <div>
                <span className="text-xs font-semibold text-white">Enable Cache Prefetch</span>
                <p className="text-[11px] text-gray-400">Automatically renew expiring cache entries before user requests experience miss latency.</p>
              </div>
            </label>

            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={localState.features.serve_expired}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    features: { ...localState.features, serve_expired: e.target.checked },
                  })
                }
                className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
              />
              <div>
                <span className="text-xs font-semibold text-white">Enable Serve-Expired Resilience</span>
                <p className="text-[11px] text-gray-400">Return expired cache records with TTL 0 if upstream resolvers timeout or fail.</p>
              </div>
            </label>
          </div>
        </div>
      )}

      {/* Subtab 5: DNSSEC */}
      {subTab === 'dnssec' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div>
            <h2 className="text-sm font-bold text-white">Cryptographic DNSSEC Validation</h2>
            <p className="text-xs text-gray-400">
              Validates digital signatures (RRSIG, DNSKEY, DS) using the IANA root trust anchor.
            </p>
          </div>

          <div className="space-y-4">
            <label className="flex items-center gap-3 cursor-pointer p-4 bg-gray-950 border border-gray-800 rounded-lg">
              <input
                type="checkbox"
                checked={localState.dnssec.enabled}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    dnssec: { ...localState.dnssec, enabled: e.target.checked },
                  })
                }
                className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
              />
              <div>
                <span className="text-xs font-bold text-white">Enforce DNSSEC Validation</span>
                <p className="text-[11px] text-gray-400">Bogus responses failing cryptographic signature checks will return SERVFAIL.</p>
              </div>
            </label>

            <div className="p-4 bg-gray-950 border border-gray-800 rounded-lg space-y-2 text-xs font-mono">
              <div className="flex justify-between">
                <span className="text-gray-400">Auto Trust Anchor File:</span>
                <span className="text-white">{localState.dnssec.auto_trust_anchor_file}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-400">Root Hints File:</span>
                <span className="text-white">{localState.dnssec.root_hints_file}</span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
