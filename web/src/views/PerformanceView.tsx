import React, { useState } from 'react';
import {
  CheckCircle2,
  Cpu,
  Gauge,
  Info,
  RefreshCw,
  Save,
  Server,
  Sparkles,
  Zap,
} from 'lucide-react';
import { DesiredState, NodeRecord } from '../types';
import { api } from '../services/api';

interface PerformanceViewProps {
  config: DesiredState | null;
  node: NodeRecord | null;
  nodeId: string;
  onConfigUpdated: (newConfig: DesiredState) => void;
}

export const PerformanceView: React.FC<PerformanceViewProps> = ({
  config,
  node,
  nodeId,
  onConfigUpdated,
}) => {
  const [localConfig, setLocalConfig] = useState<DesiredState | null>(config);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  React.useEffect(() => {
    setLocalConfig(config);
  }, [config]);

  if (!localConfig) return null;

  // Preset profiles
  const profiles = [
    {
      id: 'SMALL',
      name: 'Small Edge / Lab',
      desc: '1-2 vCPUs, 1-2 GB RAM. Optimized for minimal memory footprint and dev environments.',
      threads: 2,
      slabs: 2,
      rcvbuf: '4m',
      sndbuf: '4m',
      queriesPerThread: 1024,
      outgoingRange: 4096,
      msgCache: '64m',
      rrsetCache: '128m',
    },
    {
      id: 'MEDIUM',
      name: 'Medium Regional Node',
      desc: '4 vCPUs, 4-8 GB RAM. Handles up to 30,000 QPS with balanced caching.',
      threads: 4,
      slabs: 4,
      rcvbuf: '8m',
      sndbuf: '8m',
      queriesPerThread: 2048,
      outgoingRange: 8192,
      msgCache: '128m',
      rrsetCache: '256m',
    },
    {
      id: 'LARGE',
      name: 'Large Core Resolver',
      desc: '8 vCPUs, 16 GB RAM. Handles up to 80,000 QPS with deep cache retention.',
      threads: 8,
      slabs: 8,
      rcvbuf: '16m',
      sndbuf: '16m',
      queriesPerThread: 4096,
      outgoingRange: 16384,
      msgCache: '512m',
      rrsetCache: '1024m',
    },
    {
      id: 'ISP',
      name: 'ISP Carrier Grade',
      desc: '16+ vCPUs, 32+ GB RAM. Handles 100k+ QPS with aggressive SO_REUSEPORT & kernel buffers.',
      threads: 16,
      slabs: 16,
      rcvbuf: '32m',
      sndbuf: '32m',
      queriesPerThread: 4096,
      outgoingRange: 32768,
      msgCache: '1024m',
      rrsetCache: '2048m',
    },
  ];

  const applyProfile = (p: typeof profiles[0]) => {
    setLocalConfig({
      ...localConfig,
      performance: {
        ...localConfig.performance,
        profile: p.id,
        num_threads: p.threads,
        msg_cache_slabs: p.slabs,
        rrset_cache_slabs: p.slabs,
        so_rcvbuf: p.rcvbuf,
        so_sndbuf: p.sndbuf,
        num_queries_per_thread: p.queriesPerThread,
        outgoing_range: p.outgoingRange,
        so_reuseport: true,
      },
      cache: {
        ...localConfig.cache,
        msg_cache_size: p.msgCache,
        rrset_cache_size: p.rrsetCache,
      },
    });
  };

  const handleSave = async () => {
    setSaving(true);
    setMsg(null);
    try {
      const res = await api.applyConfig(nodeId, localConfig, 'Tuned performance parameters');
      setMsg({ type: 'success', text: `Performance tuning applied! Version ID: ${res.version_id}` });
      onConfigUpdated(localConfig);
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to apply performance changes' });
    } finally {
      setSaving(false);
    }
  };

  // Node recommendation logic
  const detectedCores = node?.cpu_cores || 2;
  const recommendedId = detectedCores <= 2 ? 'SMALL' : detectedCores <= 4 ? 'MEDIUM' : detectedCores <= 8 ? 'LARGE' : 'ISP';

  return (
    <div className="space-y-6">
      {/* Title */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Cpu className="w-5 h-5 text-cyan-400" />
            Performance Engine & Kernel Socket Tuning
          </h1>
          <p className="text-xs text-gray-400 mt-1">
            Optimize Unbound thread concurrency, cache slabs (power of 2), kernel buffer sizes, and reuseport.
          </p>
        </div>

        <button
          onClick={handleSave}
          disabled={saving}
          className="px-5 py-2.5 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2 disabled:opacity-50"
        >
          {saving ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
          {saving ? 'Applying...' : 'Apply Tuning'}
        </button>
      </div>

      {msg && (
        <div
          className={`p-3 rounded-xl border text-xs flex items-center gap-2 ${
            msg.type === 'success'
              ? 'bg-emerald-950/50 border-emerald-800 text-emerald-300'
              : 'bg-red-950/50 border-red-800 text-red-300'
          }`}
        >
          <CheckCircle2 className="w-4 h-4 shrink-0" />
          <span>{msg.text}</span>
        </div>
      )}

      {/* Auto Recommendation Box */}
      <div className="bg-gradient-to-r from-cyan-950/40 via-blue-950/20 to-gray-900 border border-cyan-800/60 rounded-xl p-5 flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400 shrink-0">
            <Sparkles className="w-5 h-5" />
          </div>
          <div>
            <div className="text-sm font-bold text-white flex items-center gap-2">
              Hardware Recommendation
              <span className="text-[10px] px-2 py-0.5 rounded bg-cyan-950 text-cyan-400 font-mono border border-cyan-800">
                {detectedCores} Cores, {node?.ram_mb ? `${Math.round(node.ram_mb / 1024)}GB` : '2GB'} RAM
              </span>
            </div>
            <p className="text-xs text-gray-300 mt-0.5">
              Based on detected topology, profile <strong className="text-cyan-400 font-mono">{recommendedId}</strong> is recommended for zero buffer overrun and optimal CPU cache affinity.
            </p>
          </div>
        </div>

        <button
          onClick={() => {
            const p = profiles.find((x) => x.id === recommendedId);
            if (p) applyProfile(p);
          }}
          className="px-4 py-2 bg-cyan-500 hover:bg-cyan-400 text-black font-semibold text-xs rounded-lg transition-colors shrink-0"
        >
          Auto-Apply Recommended Profile
        </button>
      </div>

      {/* Profile Selector Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {profiles.map((p) => {
          const isSelected = localConfig.performance.profile === p.id;
          return (
            <div
              key={p.id}
              onClick={() => applyProfile(p)}
              className={`p-5 rounded-xl border cursor-pointer transition-all flex flex-col justify-between ${
                isSelected
                  ? 'bg-cyan-950/30 border-cyan-500 shadow-lg shadow-cyan-500/10'
                  : 'bg-gray-900/70 border-gray-800 hover:border-gray-700'
              }`}
            >
              <div>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-bold font-mono text-white">{p.name}</span>
                  {isSelected && (
                    <span className="text-[10px] px-2 py-0.5 rounded bg-cyan-900 text-cyan-300 font-mono">
                      ACTIVE
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-gray-400 mt-2 leading-relaxed">{p.desc}</p>
              </div>

              <div className="mt-4 pt-3 border-t border-gray-800 space-y-1.5 text-[11px] font-mono text-gray-300">
                <div className="flex justify-between">
                  <span className="text-gray-500">Threads:</span>
                  <span className="text-cyan-400">{p.threads}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Slabs:</span>
                  <span>{p.slabs}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Socket Buffers:</span>
                  <span>{p.rcvbuf}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Cache Allocation:</span>
                  <span>{p.rrsetCache}</span>
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Detailed Tuning Controls */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
        <h2 className="text-sm font-bold text-white flex items-center gap-2">
          <Gauge className="w-4 h-4 text-cyan-400" />
          Advanced Kernel & Concurrency Controls
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs font-mono">
          <div>
            <label className="block text-gray-400 mb-1">Worker Threads (num-threads)</label>
            <input
              type="number"
              value={localConfig.performance.num_threads}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  performance: {
                    ...localConfig.performance,
                    num_threads: parseInt(e.target.value) || 1,
                  },
                })
              }
              className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
            />
          </div>

          <div>
            <label className="block text-gray-400 mb-1">Cache Slabs (Power of 2)</label>
            <input
              type="number"
              value={localConfig.performance.msg_cache_slabs}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  performance: {
                    ...localConfig.performance,
                    msg_cache_slabs: parseInt(e.target.value) || 2,
                    rrset_cache_slabs: parseInt(e.target.value) || 2,
                  },
                })
              }
              className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
            />
          </div>

          <div>
            <label className="block text-gray-400 mb-1">Socket Receive Buffer (so-rcvbuf)</label>
            <input
              type="text"
              value={localConfig.performance.so_rcvbuf}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  performance: {
                    ...localConfig.performance,
                    so_rcvbuf: e.target.value,
                  },
                })
              }
              className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
            />
          </div>

          <div>
            <label className="block text-gray-400 mb-1">Socket Send Buffer (so-sndbuf)</label>
            <input
              type="text"
              value={localConfig.performance.so_sndbuf}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  performance: {
                    ...localConfig.performance,
                    so_sndbuf: e.target.value,
                  },
                })
              }
              className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
            />
          </div>

          <div>
            <label className="block text-gray-400 mb-1">Queries Per Thread</label>
            <input
              type="number"
              value={localConfig.performance.num_queries_per_thread}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  performance: {
                    ...localConfig.performance,
                    num_queries_per_thread: parseInt(e.target.value) || 1024,
                  },
                })
              }
              className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
            />
          </div>

          <div>
            <label className="block text-gray-400 mb-1">Outgoing Range</label>
            <input
              type="number"
              value={localConfig.performance.outgoing_range}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  performance: {
                    ...localConfig.performance,
                    outgoing_range: parseInt(e.target.value) || 4096,
                  },
                })
              }
              className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
            />
          </div>
        </div>

        <div className="pt-3 border-t border-gray-800">
          <label className="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              checked={localConfig.performance.so_reuseport}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  performance: {
                    ...localConfig.performance,
                    so_reuseport: e.target.checked,
                  },
                })
              }
              className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
            />
            <div>
              <span className="text-xs font-semibold text-white">Enable SO_REUSEPORT Multi-Socket Concurrency</span>
              <p className="text-[11px] text-gray-400">Allows each worker thread to have an independent UDP receive socket in the Linux kernel.</p>
            </div>
          </label>
        </div>
      </div>
    </div>
  );
};
