import React, { useState } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  Globe,
  Layers,
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
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div>
            <h2 className="text-sm font-bold text-white">Upstream Forwarders & Resolution Mode</h2>
            <p className="text-xs text-gray-400">
              Select resolution mode and configure primary and backup recursive/forwarding servers.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pb-4 border-b border-gray-800">
            {(['RECURSIVE', 'FORWARDING', 'HYBRID'] as const).map((m) => (
              <div
                key={m}
                onClick={() => setLocalState({ ...localState, resolution_mode: m })}
                className={`p-4 rounded-xl border cursor-pointer transition-all ${
                  localState.resolution_mode === m
                    ? 'bg-cyan-950/30 border-cyan-500 text-white'
                    : 'bg-gray-950/60 border-gray-800 text-gray-400 hover:border-gray-700'
                }`}
              >
                <div className="font-bold font-mono text-sm text-cyan-400">{m}</div>
                <p className="text-[11px] mt-1 text-gray-400">
                  {m === 'RECURSIVE' && 'Direct iterative root-hints resolution; no upstream forwarders used.'}
                  {m === 'FORWARDING' && 'Forward all queries to specified high-speed upstream DNS servers.'}
                  {m === 'HYBRID' && 'Forward primary queries with automatic iterative fallback on failure.'}
                </p>
              </div>
            ))}
          </div>

          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-gray-300">Configured Upstreams</h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {localState.upstreams.map((up) => (
                <div key={up.id} className="p-3 bg-gray-950 border border-gray-800 rounded-lg flex items-center justify-between font-mono text-xs">
                  <div>
                    <div className="text-white font-bold">{up.name}</div>
                    <div className="text-cyan-400">{up.ip}:{up.port} ({up.type})</div>
                  </div>
                  <span className="px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-800 text-[10px]">
                    ACTIVE
                  </span>
                </div>
              ))}
            </div>
          </div>
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
