import React, { useState } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  Cpu,
  Database,
  Gauge,
  HardDrive,
  Info,
  MemoryStick,
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

// ── Preset profiles ─────────────────────────────────────────────────────────
const profiles = [
  {
    id: 'SMALL',
    name: 'Small Edge / Lab',
    desc: '1-2 vCPUs, ≤2 GB RAM. Minimal memory footprint for dev/test environments.',
    minCores: 1, maxCores: 2, minRAMGB: 0, minDiskGB: 4,
    threads: 2, slabs: 2, rcvbuf: '4m', sndbuf: '4m',
    queriesPerThread: 1024, outgoingRange: 4096,
    msgCache: '64m', rrsetCache: '128m',
  },
  {
    id: 'MEDIUM',
    name: 'Medium Regional Node',
    desc: '4 vCPUs, 4-8 GB RAM. Handles up to 30,000 QPS with balanced caching.',
    minCores: 3, maxCores: 4, minRAMGB: 4, minDiskGB: 10,
    threads: 4, slabs: 4, rcvbuf: '8m', sndbuf: '8m',
    queriesPerThread: 2048, outgoingRange: 8192,
    msgCache: '128m', rrsetCache: '256m',
  },
  {
    id: 'LARGE',
    name: 'Large Core Resolver',
    desc: '8 vCPUs, 16 GB RAM. Handles up to 80,000 QPS with deep cache retention.',
    minCores: 5, maxCores: 8, minRAMGB: 12, minDiskGB: 20,
    threads: 8, slabs: 8, rcvbuf: '16m', sndbuf: '16m',
    queriesPerThread: 4096, outgoingRange: 16384,
    msgCache: '512m', rrsetCache: '1024m',
  },
  {
    id: 'ISP',
    name: 'ISP Carrier Grade',
    desc: '16+ vCPUs, 32+ GB RAM. Handles 100k+ QPS with aggressive SO_REUSEPORT & kernel buffers.',
    minCores: 9, maxCores: 999, minRAMGB: 28, minDiskGB: 40,
    threads: 16, slabs: 16, rcvbuf: '32m', sndbuf: '32m',
    queriesPerThread: 4096, outgoingRange: 32768,
    msgCache: '1024m', rrsetCache: '2048m',
  },
];

// Determine recommended profile from CPU, RAM, and Disk
function getRecommendedProfile(cores: number, ramMB: number, diskGB: number): string {
  const ramGB = ramMB / 1024;
  // Disk is a hard lower bound — if disk is tiny, cap at SMALL/MEDIUM
  for (const p of [...profiles].reverse()) {
    if (cores >= p.minCores && ramGB >= p.minRAMGB && diskGB >= p.minDiskGB) {
      return p.id;
    }
  }
  return 'SMALL';
}

// Disk health thresholds
function diskStatus(freeGB: number, totalGB: number): { level: 'ok' | 'warn' | 'critical'; label: string; pct: number } {
  if (totalGB === 0) return { level: 'ok', label: 'Unknown', pct: 0 };
  const pct = Math.round(((totalGB - freeGB) / totalGB) * 100);
  if (pct >= 90) return { level: 'critical', label: `${pct}% used — CRITICAL`, pct };
  if (pct >= 75) return { level: 'warn',     label: `${pct}% used — Low Space`,  pct };
  return               { level: 'ok',       label: `${pct}% used — Healthy`,    pct };
}

// ── Component ────────────────────────────────────────────────────────────────
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

  const detectedCores   = node?.cpu_cores    || 2;
  const detectedRAMMB   = node?.ram_mb       || 2048;
  const detectedDiskTotal = node?.disk_total_gb ?? 0;
  const detectedDiskFree  = node?.disk_free_gb  ?? 0;
  const detectedDiskUsed  = detectedDiskTotal - detectedDiskFree;

  const recommendedId = getRecommendedProfile(detectedCores, detectedRAMMB, detectedDiskTotal);
  const disk = diskStatus(detectedDiskFree, detectedDiskTotal);

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

  const diskBarColor =
    disk.level === 'critical' ? 'bg-red-500' :
    disk.level === 'warn'     ? 'bg-amber-400' :
                                'bg-emerald-400';

  return (
    <div className="space-y-6">
      {/* ── Title ─────────────────────────────────────────────────────────── */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Cpu className="w-5 h-5 text-cyan-400" />
            Performance Engine &amp; Kernel Socket Tuning
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

      {/* ── Hardware Topology Cards ────────────────────────────────────────── */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* CPU */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4 flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400 shrink-0">
            <Cpu className="w-5 h-5" />
          </div>
          <div>
            <div className="text-[11px] text-gray-500 uppercase tracking-wider">CPU Cores</div>
            <div className="text-2xl font-bold text-white font-mono">{detectedCores}</div>
            <div className="text-[11px] text-gray-400">vCPU / Physical</div>
          </div>
        </div>

        {/* RAM */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4 flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-violet-500/10 border border-violet-500/30 flex items-center justify-center text-violet-400 shrink-0">
            <MemoryStick className="w-5 h-5" />
          </div>
          <div>
            <div className="text-[11px] text-gray-500 uppercase tracking-wider">System RAM</div>
            <div className="text-2xl font-bold text-white font-mono">
              {detectedRAMMB >= 1024 ? `${Math.round(detectedRAMMB / 1024)}GB` : `${detectedRAMMB}MB`}
            </div>
            <div className="text-[11px] text-gray-400">Physical Memory</div>
          </div>
        </div>

        {/* Disk */}
        <div className={`bg-gray-900/80 border rounded-xl p-4 flex items-center gap-4 ${
          disk.level === 'critical' ? 'border-red-700/70' :
          disk.level === 'warn'     ? 'border-amber-700/70' :
                                      'border-gray-800'
        }`}>
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 border ${
            disk.level === 'critical' ? 'bg-red-500/10 border-red-500/30 text-red-400' :
            disk.level === 'warn'     ? 'bg-amber-500/10 border-amber-500/30 text-amber-400' :
                                        'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
          }`}>
            <HardDrive className="w-5 h-5" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-[11px] text-gray-500 uppercase tracking-wider">Disk (root fs)</div>
            <div className="text-2xl font-bold text-white font-mono">
              {detectedDiskTotal > 0 ? `${detectedDiskTotal}GB` : 'N/A'}
            </div>
            {detectedDiskTotal > 0 ? (
              <>
                <div className="text-[11px] text-gray-400 mt-0.5">
                  {detectedDiskFree}GB free · {detectedDiskUsed}GB used
                </div>
                {/* Disk usage bar */}
                <div className="mt-2 h-1.5 rounded-full bg-gray-800 overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-all ${diskBarColor}`}
                    style={{ width: `${Math.min(disk.pct, 100)}%` }}
                  />
                </div>
                <div className={`text-[10px] mt-1 font-mono ${
                  disk.level === 'critical' ? 'text-red-400' :
                  disk.level === 'warn'     ? 'text-amber-400' :
                                              'text-emerald-400'
                }`}>
                  {disk.level === 'critical' && '⚠ '}
                  {disk.level === 'warn' && '⚡ '}
                  {disk.label}
                </div>
              </>
            ) : (
              <div className="text-[11px] text-gray-500">Available on Linux only</div>
            )}
          </div>
        </div>
      </div>

      {/* ── Disk Critical Alert ────────────────────────────────────────────── */}
      {disk.level === 'critical' && detectedDiskTotal > 0 && (
        <div className="bg-red-950/40 border border-red-700 rounded-xl p-4 flex items-start gap-3">
          <AlertTriangle className="w-5 h-5 text-red-400 shrink-0 mt-0.5" />
          <div>
            <div className="text-sm font-bold text-red-300">Disk Critically Full — {disk.pct}% Used</div>
            <p className="text-xs text-red-400/80 mt-1">
              The root filesystem is nearly full. Auto-updates will fail with &quot;No space left on device&quot;.
              Prune old releases in <span className="font-mono">/opt/netora-dns/releases/</span>,
              rotate <span className="font-mono">/var/log/unbound/unbound.log</span>, and check
              <span className="font-mono"> /opt/netora-dns/staging/</span> for leftover clone artifacts.
            </p>
            <div className="mt-2 text-[11px] font-mono bg-red-950/60 rounded p-2 text-red-300 space-y-0.5">
              <div>$ ls -t /opt/netora-dns/releases/ | tail -n +4 | xargs -I{'{'}{'}'}  rm -rf /opt/netora-dns/releases/{'{'}{'}'}  # prune old releases</div>
              <div>$ truncate -s 0 /var/log/unbound/unbound.log  # rotate log</div>
              <div>$ rm -rf /opt/netora-dns/staging/*  # clear staging</div>
            </div>
          </div>
        </div>
      )}

      {disk.level === 'warn' && detectedDiskTotal > 0 && (
        <div className="bg-amber-950/30 border border-amber-700/60 rounded-xl p-3 flex items-center gap-3">
          <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0" />
          <p className="text-xs text-amber-300">
            Disk usage is above 75%. Consider pruning old releases and rotating logs to prevent update failures.
          </p>
        </div>
      )}

      {/* ── Auto Recommendation Box ────────────────────────────────────────── */}
      <div className="bg-gradient-to-r from-cyan-950/40 via-blue-950/20 to-gray-900 border border-cyan-800/60 rounded-xl p-5 flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400 shrink-0">
            <Sparkles className="w-5 h-5" />
          </div>
          <div>
            <div className="text-sm font-bold text-white flex items-center gap-2 flex-wrap">
              Hardware Recommendation
              <span className="text-[10px] px-2 py-0.5 rounded bg-cyan-950 text-cyan-400 font-mono border border-cyan-800">
                {detectedCores} Cores
              </span>
              <span className="text-[10px] px-2 py-0.5 rounded bg-violet-950 text-violet-400 font-mono border border-violet-800">
                {detectedRAMMB >= 1024 ? `${Math.round(detectedRAMMB / 1024)}GB` : `${detectedRAMMB}MB`} RAM
              </span>
              {detectedDiskTotal > 0 && (
                <span className={`text-[10px] px-2 py-0.5 rounded font-mono border ${
                  disk.level === 'critical' ? 'bg-red-950 text-red-400 border-red-800' :
                  disk.level === 'warn'     ? 'bg-amber-950 text-amber-400 border-amber-800' :
                                              'bg-emerald-950 text-emerald-400 border-emerald-800'
                }`}>
                  {detectedDiskTotal}GB Disk
                </span>
              )}
            </div>
            <p className="text-xs text-gray-300 mt-0.5">
              Based on detected hardware topology (CPU + RAM + Disk), profile{' '}
              <strong className="text-cyan-400 font-mono">{recommendedId}</strong>{' '}
              is recommended for zero buffer overrun and optimal CPU cache affinity.
              {detectedDiskTotal > 0 && detectedDiskTotal < 10 && (
                <span className="text-amber-400"> Disk is small — consider upgrading for blocklist storage.</span>
              )}
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

      {/* ── Disk Setup Recommendations ─────────────────────────────────────── */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
        <h2 className="text-sm font-bold text-white flex items-center gap-2">
          <HardDrive className="w-4 h-4 text-emerald-400" />
          Disk Setup Recommendations for DNS Servers
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs">
          {/* Minimum */}
          <div className="bg-gray-950/60 border border-gray-700 rounded-xl p-4 space-y-3">
            <div className="flex items-center gap-2 font-bold text-gray-300">
              <div className="w-2 h-2 rounded-full bg-gray-400" />
              Minimum Setup
            </div>
            <div className="space-y-1.5 text-gray-400 font-mono">
              <div className="flex justify-between"><span>Root (/)</span><span className="text-white">8 GB</span></div>
              <div className="flex justify-between"><span>RAM</span><span className="text-white">1 GB</span></div>
              <div className="flex justify-between"><span>Profile</span><span className="text-cyan-400">SMALL</span></div>
              <div className="flex justify-between"><span>QPS Target</span><span className="text-white">&lt;5k</span></div>
            </div>
            <p className="text-[10px] text-gray-500 leading-relaxed">
              Suitable for lab/dev only. Blocklist storage and release retention may be constrained.
            </p>
          </div>

          {/* Recommended */}
          <div className="bg-cyan-950/20 border border-cyan-800/50 rounded-xl p-4 space-y-3 relative">
            <div className="absolute top-2 right-2 text-[10px] px-2 py-0.5 rounded bg-cyan-900 text-cyan-300 font-mono">RECOMMENDED</div>
            <div className="flex items-center gap-2 font-bold text-cyan-300">
              <div className="w-2 h-2 rounded-full bg-cyan-400" />
              Recommended ISP Node
            </div>
            <div className="space-y-1.5 text-gray-400 font-mono">
              <div className="flex justify-between"><span>Root (/)</span><span className="text-white">25+ GB</span></div>
              <div className="flex justify-between"><span>RAM</span><span className="text-white">4-8 GB</span></div>
              <div className="flex justify-between"><span>vCPUs</span><span className="text-white">4-8</span></div>
              <div className="flex justify-between"><span>Profile</span><span className="text-cyan-400">MEDIUM / LARGE</span></div>
              <div className="flex justify-between"><span>QPS Target</span><span className="text-white">30k–80k</span></div>
            </div>
            <p className="text-[10px] text-emerald-400/80 leading-relaxed">
              Leaves ample space for 5 release dirs (~55MB each), blocklists (~50MB), and log rotation.
            </p>
          </div>

          {/* Carrier Grade */}
          <div className="bg-gray-950/60 border border-gray-700 rounded-xl p-4 space-y-3">
            <div className="flex items-center gap-2 font-bold text-amber-300">
              <div className="w-2 h-2 rounded-full bg-amber-400" />
              Carrier Grade (ISP)
            </div>
            <div className="space-y-1.5 text-gray-400 font-mono">
              <div className="flex justify-between"><span>Root (/)</span><span className="text-white">50+ GB</span></div>
              <div className="flex justify-between"><span>RAM</span><span className="text-white">32+ GB</span></div>
              <div className="flex justify-between"><span>vCPUs</span><span className="text-white">16+</span></div>
              <div className="flex justify-between"><span>Profile</span><span className="text-amber-400">ISP</span></div>
              <div className="flex justify-between"><span>QPS Target</span><span className="text-white">100k+</span></div>
            </div>
            <p className="text-[10px] text-gray-500 leading-relaxed">
              Separate /var/log mount recommended. Use WAL journal mode on PostgreSQL. Enable SO_REUSEPORT.
            </p>
          </div>
        </div>

        {/* Disk allocation breakdown */}
        <div className="pt-3 border-t border-gray-800 space-y-2">
          <div className="text-xs font-semibold text-gray-400">Disk Space Breakdown (Typical)</div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-2 text-[11px]">
            {[
              { label: 'OS + Netora API', size: '~2 GB', color: 'text-blue-400' },
              { label: 'Release Dirs (×5)', size: '~275 MB', color: 'text-violet-400' },
              { label: 'Blocklists + Conf', size: '~200 MB', color: 'text-amber-400' },
              { label: 'Logs (rotated)', size: '~500 MB', color: 'text-emerald-400' },
            ].map((item) => (
              <div key={item.label} className="bg-gray-950/60 border border-gray-800 rounded-lg p-2.5">
                <div className={`font-mono font-bold ${item.color}`}>{item.size}</div>
                <div className="text-gray-500 mt-0.5">{item.label}</div>
              </div>
            ))}
          </div>
          <p className="text-[11px] text-gray-500">
            <Info className="inline w-3 h-3 mr-1" />
            The release pruning system keeps only the last {' '}
            <span className="font-mono text-cyan-400">BACKUP_RETENTION=5</span> releases automatically.
            Disk usage beyond 75% triggers <span className="text-amber-400">warnings</span>;
            beyond 90% causes update failures.
          </p>
        </div>
      </div>

      {/* ── Profile Selector Cards ─────────────────────────────────────────── */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {profiles.map((p) => {
          const isSelected = localConfig.performance.profile === p.id;
          const isRecommended = p.id === recommendedId;
          return (
            <div
              key={p.id}
              onClick={() => applyProfile(p)}
              className={`p-5 rounded-xl border cursor-pointer transition-all flex flex-col justify-between relative ${
                isSelected
                  ? 'bg-cyan-950/30 border-cyan-500 shadow-lg shadow-cyan-500/10'
                  : 'bg-gray-900/70 border-gray-800 hover:border-gray-700'
              }`}
            >
              {isRecommended && !isSelected && (
                <div className="absolute top-2 right-2 text-[9px] px-1.5 py-0.5 rounded bg-emerald-900/80 text-emerald-300 font-mono border border-emerald-700">
                  ★ FIT
                </div>
              )}
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
                  <span className="text-gray-500">Cache Alloc:</span>
                  <span>{p.rrsetCache}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Min Disk:</span>
                  <span className={detectedDiskTotal > 0 && detectedDiskTotal < p.minDiskGB ? 'text-red-400' : 'text-gray-300'}>
                    {p.minDiskGB}GB
                  </span>
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* ── Advanced Kernel Controls ───────────────────────────────────────── */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
        <h2 className="text-sm font-bold text-white flex items-center gap-2">
          <Gauge className="w-4 h-4 text-cyan-400" />
          Advanced Kernel &amp; Concurrency Controls
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
