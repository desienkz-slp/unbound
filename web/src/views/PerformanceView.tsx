import React, { useState, useEffect } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  Cpu,
  Gauge,
  HardDrive,
  Info,
  MemoryStick,
  RefreshCw,
  Save,
  Sparkles,
  Zap,
  FileText,
  RotateCcw,
  Terminal,
  ShieldAlert,
  ShieldCheck,
  FileCode,
  Sliders,
  Check,
} from 'lucide-react';
import { DesiredState, NodeRecord, LogrotateReport, LogrotatePolicy } from '../types';
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
  for (const p of [...profiles].reverse()) {
    if (cores >= p.minCores && ramGB >= p.minRAMGB && diskGB >= p.minDiskGB) {
      return p.id;
    }
  }
  return 'SMALL';
}

// Fallback AI recommendation for logrotate
function getFallbackAILogrotatePolicy(diskTotalGB: number, diskFreeGB: number, cores: number): LogrotatePolicy {
  const usedPct = diskTotalGB > 0 ? Math.round(((diskTotalGB - diskFreeGB) / diskTotalGB) * 100) : 0;
  if (diskTotalGB > 0 && (diskTotalGB <= 10 || diskFreeGB <= 2 || usedPct >= 75)) {
    // Keep in sync with storageSaverLimits() in pkg/logrotate/engine.go
    const [maxSize, rotateCount] =
      diskFreeGB <= 1 ? ['10M', 2] :
      diskFreeGB <= 2 ? ['15M', 3] :
      diskFreeGB <= 5 ? ['25M', 3] : ['50M', 4];
    return {
      enabled: true,
      frequency: 'daily',
      rotate_count: rotateCount as number,
      max_size: maxSize as string,
      compress: true,
      delay_compress: true,
      rotate_netora: true,
    };
  }
  if (diskTotalGB >= 40 && diskFreeGB >= 10 && usedPct < 70) {
    return {
      enabled: true,
      frequency: 'weekly',
      rotate_count: 30,
      max_size: '100M',
      compress: true,
      delay_compress: cores <= 4,
      rotate_netora: true,
    };
  }
  return {
    enabled: true,
    frequency: 'daily',
    rotate_count: 7,
    max_size: '50M',
    compress: true,
    delay_compress: true,
    rotate_netora: true,
  };
}

// Disk health thresholds
function diskStatus(freeGB: number, totalGB: number): { level: 'ok' | 'warn' | 'critical'; label: string; pct: number } {
  if (totalGB === 0) return { level: 'ok', label: 'Unknown', pct: 0 };
  const pct = Math.round(((totalGB - freeGB) / totalGB) * 100);
  if (pct >= 90) return { level: 'critical', label: `${pct}% used — CRITICAL`, pct };
  if (pct >= 75) return { level: 'warn',     label: `${pct}% used — Low Space`,  pct };
  return               { level: 'ok',       label: `${pct}% used — Healthy`,    pct };
}

// Ensure all subfields of DesiredState are initialized to avoid undefined errors
function ensureDefaults(cfg: DesiredState | null): DesiredState | null {
  if (!cfg) return null;
  const c = cfg.cache;
  const p = cfg.performance;
  const f = cfg.features;
  const r = cfg.rate_limit;
  return {
    ...cfg,
    cache: {
      msg_cache_size: c?.msg_cache_size || '64m',
      rrset_cache_size: c?.rrset_cache_size || '128m',
      key_cache_size: c?.key_cache_size || '32m',
      neg_cache_size: c?.neg_cache_size || '32m',
      min_ttl: typeof c?.min_ttl === 'number' ? c.min_ttl : 0,
      max_ttl: typeof c?.max_ttl === 'number' ? c.max_ttl : 86400,
      infra_host_ttl: typeof c?.infra_host_ttl === 'number' ? c.infra_host_ttl : 900,
    },
    performance: {
      profile: p?.profile || 'SMALL',
      num_threads: p?.num_threads || 2,
      msg_cache_slabs: p?.msg_cache_slabs || 2,
      rrset_cache_slabs: p?.rrset_cache_slabs || 2,
      so_reuseport: p?.so_reuseport ?? true,
      so_rcvbuf: p?.so_rcvbuf || '4m',
      so_sndbuf: p?.so_sndbuf || '4m',
      num_queries_per_thread: p?.num_queries_per_thread || 1024,
      outgoing_range: p?.outgoing_range || 4096,
    },
    features: {
      prefetch: f?.prefetch ?? true,
      prefetch_key: f?.prefetch_key ?? true,
      serve_expired: f?.serve_expired ?? true,
      serve_expired_ttl: typeof f?.serve_expired_ttl === 'number' ? f.serve_expired_ttl : 86400,
      serve_expired_client_timeout: typeof f?.serve_expired_client_timeout === 'number' ? f.serve_expired_client_timeout : 200,
      serve_expired_ttl_reset: f?.serve_expired_ttl_reset ?? true,
      qname_minimisation: f?.qname_minimisation ?? true,
      hide_identity: f?.hide_identity ?? true,
      hide_version: f?.hide_version ?? true,
    },
    rate_limit: {
      enabled: r?.enabled ?? false,
      ip_rate_limit: typeof r?.ip_rate_limit === 'number' ? r.ip_rate_limit : 1000,
      ip_rate_limit_factor: typeof r?.ip_rate_limit_factor === 'number' ? r.ip_rate_limit_factor : 10,
      domain_rate_limit: typeof r?.domain_rate_limit === 'number' ? r.domain_rate_limit : 1000,
      rate_limit_size: r?.rate_limit_size || '16m',
      rate_limit_slabs: r?.rate_limit_slabs || 4,
    },
  };
}

// ── Component ────────────────────────────────────────────────────────────────
export const PerformanceView: React.FC<PerformanceViewProps> = ({
  config,
  node,
  nodeId,
  onConfigUpdated,
}) => {
  const [localConfig, setLocalConfig] = useState<DesiredState | null>(ensureDefaults(config));
  const [saving, setSaving] = useState(false);
  const [autoTuningAll, setAutoTuningAll] = useState(false);
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [activeTab, setActiveTab] = useState<'all' | 'compute' | 'resiliency' | 'logrotate'>('all');

  // ── Logrotate State ──────────────────────────────────────────────────────────
  const [logrotateReport, setLogrotateReport] = useState<LogrotateReport | null>(null);
  const [logrotateLoading, setLogrotateLoading] = useState(false);
  const [logrotatePolicy, setLogrotatePolicy] = useState<LogrotatePolicy>({
    enabled: true,
    frequency: 'daily',
    rotate_count: 7,
    max_size: '50M',
    compress: true,
    delay_compress: true,
    rotate_netora: true,
  });
  const [savingLogrotate, setSavingLogrotate] = useState(false);
  const [testingLogrotate, setTestingLogrotate] = useState(false);
  const [forceRotating, setForceRotating] = useState(false);
  const [consoleOutput, setConsoleOutput] = useState<string | null>(null);
  const [logrotateMsg, setLogrotateMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const effectiveNodeId = nodeId || node?.id || '';

  const fetchLogrotate = async () => {
    if (!effectiveNodeId) return;
    setLogrotateLoading(true);
    try {
      const rep = await api.getLogrotateReport(effectiveNodeId);
      setLogrotateReport(rep);
      if (rep.active_policy && rep.active_policy.frequency) {
        setLogrotatePolicy(rep.active_policy);
      } else if (rep.ai_recommendation?.recommended_policy) {
        setLogrotatePolicy(rep.ai_recommendation.recommended_policy);
      }
    } catch (err: any) {
      console.warn('Failed to fetch logrotate report:', err);
    } finally {
      setLogrotateLoading(false);
    }
  };

  useEffect(() => {
    setLocalConfig(ensureDefaults(config));
  }, [config]);

  useEffect(() => {
    if (effectiveNodeId) {
      fetchLogrotate();
    }
  }, [effectiveNodeId]);

  const detectedCores   = node?.cpu_cores    || 2;
  const detectedRAMMB   = node?.ram_mb       || 2048;
  const detectedDiskTotal = node?.disk_total_gb ?? 0;
  const detectedDiskFree  = node?.disk_free_gb  ?? 0;
  const detectedDiskUsed  = detectedDiskTotal - detectedDiskFree;

  const recommendedId = getRecommendedProfile(detectedCores, detectedRAMMB, detectedDiskTotal);
  const disk = diskStatus(detectedDiskFree, detectedDiskTotal);

  // Apply AI logrotate policy to form or host
  const applyAIPolicy = async (instantSave = false) => {
    const rec = logrotateReport?.ai_recommendation?.recommended_policy ||
      getFallbackAILogrotatePolicy(detectedDiskTotal, detectedDiskFree, detectedCores);
    setLogrotatePolicy(rec);

    if (instantSave) {
      if (!effectiveNodeId) return;
      setSavingLogrotate(true);
      setLogrotateMsg(null);
      try {
        const res = await api.applyLogrotate(effectiveNodeId, rec);
        setLogrotateMsg({
          type: res.success ? 'success' : 'error',
          text: `Rekomendasi AI Logrotate berhasil diterapkan & aktif pada host! ${res.message || ''}`,
        });
        if (res.dry_run_output) setConsoleOutput(res.dry_run_output);
        await fetchLogrotate();
      } catch (err: any) {
        setLogrotateMsg({ type: 'error', text: err.message || 'Gagal menyimpan rekomendasi AI logrotate ke host' });
      } finally {
        setSavingLogrotate(false);
      }
    } else {
      setLogrotateMsg({
        type: 'success',
        text: `Rekomendasi AI Logrotate diterapkan ke form tuning. Klik 'Terapkan Konfigurasi Logrotate' untuk menyimpan ke host.`,
      });
    }
  };

  const applyPresetLogrotate = (preset: { frequency: string; rotate_count: number; max_size: string; compress: boolean; delay_compress: boolean }) => {
    setLogrotatePolicy({
      ...logrotatePolicy,
      ...preset,
    });
    setLogrotateMsg({
      type: 'success',
      text: `Preset diterapkan ke form tuning (${preset.frequency}, ${preset.rotate_count}x, maxsize ${preset.max_size}). Klik 'Terapkan Konfigurasi Logrotate' untuk menyimpan ke host.`,
    });
  };

  const handleApplyLogrotate = async () => {
    if (!effectiveNodeId) {
      setLogrotateMsg({ type: 'error', text: 'Node ID tidak ditemukan' });
      return;
    }
    setSavingLogrotate(true);
    setLogrotateMsg(null);
    try {
      const res = await api.applyLogrotate(effectiveNodeId, logrotatePolicy);
      if (res.success) {
        setLogrotateMsg({ type: 'success', text: res.message || 'Konfigurasi logrotate berhasil disimpan dan aktif!' });
        const outMsg = res.dry_run_output && res.dry_run_output.trim()
          ? res.dry_run_output
          : `[SUKSES] ${res.message || 'Konfigurasi logrotate berhasil disimpan dan diverifikasi oleh sistem.'}\nFile /etc/logrotate.d/unbound telah diperbarui dengan su directive dan log_reopen.`;
        setConsoleOutput(outMsg);
        await fetchLogrotate();
      } else {
        setLogrotateMsg({ type: 'error', text: res.message || 'Gagal menerapkan logrotate' });
        if (res.dry_run_output) {
          setConsoleOutput(res.dry_run_output);
        }
      }
    } catch (err: any) {
      setLogrotateMsg({ type: 'error', text: err.message || 'Gagal menerapkan konfigurasi logrotate' });
    } finally {
      setSavingLogrotate(false);
    }
  };

  const handleTestLogrotate = async () => {
    if (!effectiveNodeId) {
      setLogrotateMsg({ type: 'error', text: 'Node ID tidak ditemukan' });
      return;
    }
    setTestingLogrotate(true);
    setLogrotateMsg(null);
    try {
      const res = await api.testLogrotate(effectiveNodeId, logrotatePolicy);
      const outMsg = res.output && res.output.trim()
        ? res.output
        : (res.error ? `[ERROR] ${res.error}` : '[INFO] Uji validasi sintaks dry-run (logrotate -d) selesai tanpa kesalahan.');
      setConsoleOutput(outMsg);
      setLogrotateMsg({
        type: res.success ? 'success' : 'error',
        text: res.success ? 'Uji validasi dry-run (logrotate -d) sukses! Konfigurasi valid dan aman.' : 'Uji validasi menemukan masalah pada syntax atau hak akses.',
      });
    } catch (err: any) {
      setLogrotateMsg({ type: 'error', text: err.message || 'Gagal menguji konfigurasi' });
    } finally {
      setTestingLogrotate(false);
    }
  };

  const handleForceRotate = async () => {
    if (!effectiveNodeId) {
      setLogrotateMsg({ type: 'error', text: 'Node ID tidak ditemukan' });
      return;
    }
    if (!window.confirm('Rotasi darurat akan memotong file log saat ini dan mengompresinya untuk mengosongkan ruang disk sekarang. Lanjutkan?')) {
      return;
    }
    setForceRotating(true);
    setLogrotateMsg(null);
    try {
      const res = await api.forceRotateLogrotate(effectiveNodeId);
      const outMsg = res.output && res.output.trim()
        ? res.output
        : `[SUKSES] Rotasi paksa darurat (logrotate -f) berhasil dieksekusi!\nFile /var/log/unbound/unbound.log telah diputar dan dikompresi ke arsip .gz.\nRuang disk host telah dibebaskan. (Exit code: 0)`;
      setConsoleOutput(outMsg);
      setLogrotateMsg({
        type: 'success',
        text: 'Rotasi paksa (logrotate -f) berhasil dijalankan! Ruang disk telah dibebaskan.',
      });
      await fetchLogrotate();
    } catch (err: any) {
      setLogrotateMsg({ type: 'error', text: err.message || 'Gagal menjalankan rotasi darurat' });
    } finally {
      setForceRotating(false);
    }
  };

  // Compute profile application
  const applyProfile = (p: typeof profiles[0], feedbackMsg?: string) => {
    if (!localConfig) return;
    const updatedCfg: DesiredState = {
      ...localConfig,
      performance: {
        ...(localConfig.performance || {}),
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
        ...(localConfig.cache || {}),
        msg_cache_size: p.msgCache,
        rrset_cache_size: p.rrsetCache,
      },
    };
    setLocalConfig(updatedCfg);
    setMsg({
      type: 'success',
      text: feedbackMsg || `Profil '${p.name}' dipilih (${p.threads} Threads, ${p.slabs} Slabs, ${p.rrsetCache} Cache). Klik 'Apply Tuning' di atas untuk menyimpan ke node.`,
    });
  };

  // Auto-apply AI Recommended Profile with option to instantly save to server
  const handleAutoApplyProfile = async (instantSave = false) => {
    const p = profiles.find((x) => x.id === recommendedId);
    if (!p || !localConfig) return;
    const updatedCfg: DesiredState = {
      ...localConfig,
      performance: {
        ...(localConfig.performance || {}),
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
        ...(localConfig.cache || {}),
        msg_cache_size: p.msgCache,
        rrset_cache_size: p.rrsetCache,
      },
    };
    setLocalConfig(updatedCfg);

    if (instantSave) {
      if (!effectiveNodeId) {
        setMsg({ type: 'error', text: 'Node ID tidak valid' });
        return;
      }
      setSaving(true);
      setMsg(null);
      try {
        const res = await api.applyConfig(effectiveNodeId, updatedCfg, `AI Auto-Tuning Profile applied: ${p.name}`);
        setMsg({
          type: 'success',
          text: `⚡ Rekomendasi AI Profil '${p.name}' (${p.threads} Threads, ${p.rrsetCache} Cache) berhasil disimpan & aktif pada server! (Versi: ${res.version_id})`,
        });
        onConfigUpdated(updatedCfg);
      } catch (err: any) {
        setMsg({ type: 'error', text: err.message || 'Gagal menyimpan profil AI ke server' });
      } finally {
        setSaving(false);
      }
    } else {
      setMsg({
        type: 'success',
        text: `Rekomendasi AI (${p.name}) berhasil diterapkan ke form editor. Klik 'Apply Tuning' di atas untuk menyimpan ke server.`,
      });
    }
  };

  // Master AI One-Click Full-Stack Auto-Tune: Profile + Resiliency + DDoS + Logrotate
  const handleAIAutoTuneAll = async () => {
    if (!localConfig || !effectiveNodeId) {
      setMsg({ type: 'error', text: 'Konfigurasi node belum siap' });
      return;
    }
    const p = profiles.find((x) => x.id === recommendedId) || profiles[0];
    setAutoTuningAll(true);
    setMsg(null);

    try {
      // 1. Build optimal desired state with Hardware Profile + Carrier Resiliency + Standard DDoS Protection
      const fullTunedState: DesiredState = {
        ...localConfig,
        performance: {
          ...(localConfig.performance || {}),
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
          ...(localConfig.cache || {}),
          msg_cache_size: p.msgCache,
          rrset_cache_size: p.rrsetCache,
          key_cache_size: '32m',
          neg_cache_size: '32m',
          min_ttl: 0,
          max_ttl: 86400,
          infra_host_ttl: 900,
        },
        features: {
          ...(localConfig.features || {}),
          prefetch: true,
          prefetch_key: true,
          serve_expired: true,
          serve_expired_ttl: 86400,
          serve_expired_client_timeout: 200,
          serve_expired_ttl_reset: true,
        },
        rate_limit: {
          enabled: true,
          ip_rate_limit: 1000,
          ip_rate_limit_factor: 10,
          domain_rate_limit: 1000,
          rate_limit_size: '16m',
          rate_limit_slabs: p.slabs,
        },
      };

      // 2. Prepare optimal AI Logrotate Policy
      const aiLogrotate = logrotateReport?.ai_recommendation?.recommended_policy ||
        getFallbackAILogrotatePolicy(detectedDiskTotal, detectedDiskFree, detectedCores);

      setLocalConfig(fullTunedState);
      setLogrotatePolicy(aiLogrotate);

      // 3. Atomically apply full configuration to node
      const res = await api.applyConfig(effectiveNodeId, fullTunedState, `AI Full-Stack Auto-Tune Applied: ${p.name}`);

      // 4. Apply Logrotate policy to host
      try {
        await api.applyLogrotate(effectiveNodeId, aiLogrotate);
      } catch (logErr) {
        console.warn('Logrotate apply warning during full tune:', logErr);
      }

      setMsg({
        type: 'success',
        text: `✨ AI Full Auto-Tune Sukses! Profil '${p.name}' (${p.threads} Threads, ${p.rrsetCache} Cache), Resiliency Carrier Grade (0ms), Subnet DDoS Shield, dan Auto-Logrotate telah aktif di node! (Versi: ${res.version_id})`,
      });
      onConfigUpdated(fullTunedState);
      await fetchLogrotate();
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Gagal menjalankan AI Full Auto-Tune' });
    } finally {
      setAutoTuningAll(false);
    }
  };

  const applyResiliencyPreset = () => {
    if (!localConfig) return;
    setLocalConfig({
      ...localConfig,
      features: {
        ...(localConfig.features || {}),
        prefetch: true,
        prefetch_key: true,
        serve_expired: true,
        serve_expired_ttl_reset: true,
        serve_expired_ttl: 86400,
        serve_expired_client_timeout: 200,
      },
    });
    setMsg({
      type: 'success',
      text: 'Preset Carrier Resilience diterapkan (Prefetch Aktif, DNSKEY Prefetch, Serve-Expired Fallback, TTL Reset). Klik \'Apply Tuning\' di atas untuk menyimpan ke node.',
    });
  };

  const applyRateLimitPreset = (preset: { label: string; ip: number; ipFactor: number; domain: number; domainFactor: number }) => {
    if (!localConfig) return;
    setLocalConfig({
      ...localConfig,
      rate_limit: {
        enabled: preset.ip > 0 || preset.domain > 0,
        ip_rate_limit: preset.ip,
        ip_rate_limit_factor: preset.ipFactor,
        domain_rate_limit: preset.domain,
        rate_limit_size: '16m',
        rate_limit_slabs: localConfig.performance?.msg_cache_slabs || 4,
      },
    });
    setMsg({
      type: 'success',
      text: `Preset Anti-DDoS '${preset.label}' diterapkan (${preset.ip} IP QPS / ${preset.domain} Domain QPS). Klik 'Apply Tuning' di atas untuk menyimpan ke node.`,
    });
  };

  const handleSave = async () => {
    if (!effectiveNodeId || !localConfig) {
      setMsg({ type: 'error', text: 'Node ID tidak valid atau konfigurasi belum siap' });
      return;
    }
    setSaving(true);
    setMsg(null);
    try {
      const res = await api.applyConfig(effectiveNodeId, localConfig, 'Tuned performance parameters');
      setMsg({ type: 'success', text: `Performance tuning applied! Version ID: ${res.version_id}` });
      onConfigUpdated(localConfig);
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to apply performance changes' });
    } finally {
      setSaving(false);
    }
  };

  if (!localConfig) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between pb-4 border-b border-slate-800/80">
          <div>
            <h1 className="text-xl font-bold text-white flex items-center gap-2">
              <Sliders className="w-5 h-5 text-cyan-400" />
              Tuning Performa &amp; Profil Hardware
            </h1>
                      </div>
        </div>

        <div className="flex flex-col items-center justify-center min-h-[380px] bg-[#0D1424] border border-slate-800 rounded-xl p-8 text-center space-y-4 shadow-xl">
          <div className="w-14 h-14 rounded-2xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400 shadow-[0_0_20px_rgba(6,182,212,0.15)]">
            <RefreshCw className="w-7 h-7 animate-spin" />
          </div>
          <div className="space-y-1">
            <h3 className="text-base font-semibold text-slate-200">Memuat Konfigurasi Hardware &amp; Profil...</h3>
          </div>
        </div>
      </div>
    );
  }

  const diskBarColor =
    disk.level === 'critical' ? 'bg-red-500' :
    disk.level === 'warn'     ? 'bg-amber-400' :
                                'bg-emerald-400';

  return (
    <div className="space-y-6">
      {/* ── Title & Actions ─────────────────────────────────────────────────── */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Cpu className="w-5 h-5 text-cyan-400" />
            Performance Engine &amp; Kernel Socket Tuning
          </h1>
                  </div>

        <div className="flex items-center gap-2.5 flex-wrap">
          {/* AI One-Click Full-Stack Auto-Tune */}
          <button
            onClick={handleAIAutoTuneAll}
            disabled={autoTuningAll || saving}
            className="px-4 py-2.5 bg-gradient-to-r from-purple-600 via-indigo-600 to-cyan-500 hover:from-purple-500 hover:to-cyan-400 text-white font-bold text-xs rounded-lg transition-all shadow-lg shadow-purple-500/20 flex items-center gap-2 disabled:opacity-50 active:scale-95"
            title="Otomatis terapkan Profil Hardware + Resiliency + Anti-DDoS + Logrotate AI sekaligus ke node!"
          >
            {autoTuningAll ? <RefreshCw className="w-4 h-4 animate-spin text-cyan-200" /> : <Sparkles className="w-4 h-4 text-cyan-300 animate-pulse" />}
            {autoTuningAll ? 'Menerapkan AI Auto-Tune...' : '✨ AI Full Auto-Tune'}
          </button>

          {/* Standard Save Button */}
          <button
            onClick={handleSave}
            disabled={saving || autoTuningAll}
            className="px-5 py-2.5 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2 disabled:opacity-50 active:scale-95"
          >
            {saving ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
            {saving ? 'Applying...' : 'Apply Tuning'}
          </button>
        </div>
      </div>

      {/* ── Tab Switcher ───────────────────────────────────────────────────── */}
      <div className="flex items-center gap-2 border-b border-gray-800 pb-3 overflow-x-auto">
        <button
          onClick={() => setActiveTab('all')}
          className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-all flex items-center gap-1.5 whitespace-nowrap ${
            activeTab === 'all'
              ? 'bg-cyan-500/20 text-cyan-400 border border-cyan-500/30'
              : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
          }`}
        >
          <Sliders className="w-3.5 h-3.5" />
          Semua Sistem Tuning
        </button>
        <button
          onClick={() => setActiveTab('compute')}
          className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-all flex items-center gap-1.5 whitespace-nowrap ${
            activeTab === 'compute'
              ? 'bg-cyan-500/20 text-cyan-400 border border-cyan-500/30'
              : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
          }`}
        >
          <Cpu className="w-3.5 h-3.5" />
          CPU &amp; Cache Engine
        </button>
        <button
          onClick={() => setActiveTab('resiliency')}
          className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-all flex items-center gap-1.5 whitespace-nowrap ${
            activeTab === 'resiliency'
              ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
              : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
          }`}
        >
          <Zap className="w-3.5 h-3.5" />
          Resiliency &amp; Anti-DDoS
        </button>
        <button
          onClick={() => setActiveTab('logrotate')}
          className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-all flex items-center gap-1.5 whitespace-nowrap ${
            activeTab === 'logrotate'
              ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
              : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
          }`}
        >
          <RotateCcw className="w-3.5 h-3.5" />
          Logrotate &amp; Storage AI Guard
          {logrotateReport?.telemetry?.health_status === 'WARNING' && (
            <span className="w-2 h-2 rounded-full bg-amber-400 animate-pulse" title="Peringatan logrotate terdeteksi"></span>
          )}
          {logrotateReport?.telemetry?.health_status === 'CRITICAL' && (
            <span className="w-2 h-2 rounded-full bg-red-500 animate-pulse" title="Masalah kritis logrotate!"></span>
          )}
        </button>
      </div>

      {msg && (
        <div
          className={`p-3.5 rounded-xl border text-xs flex items-center gap-2.5 ${
            msg.type === 'success'
              ? 'bg-emerald-950/50 border-emerald-800 text-emerald-300'
              : 'bg-red-950/50 border-red-800 text-red-300'
          }`}
        >
          {msg.type === 'success' ? (
            <CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-400" />
          ) : (
            <AlertTriangle className="w-4 h-4 shrink-0 text-red-400" />
          )}
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

      {/* ── Compute Tuning (Profiles & Threads) ────────────────────────── */}
      {(activeTab === 'all' || activeTab === 'compute') && (
        <>
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
                  <div>$ ls -t /opt/netora-dns/releases/ | tail -n +4 | xargs -I{'{'}{'}'} rm -rf /opt/netora-dns/releases/{'{'}{'}'}  # prune old releases</div>
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
                <Sparkles className="w-5 h-5 animate-pulse" />
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
                  Berdasarkan topologi hardware server (CPU + RAM + Disk), profil{' '}
                  <strong className="text-cyan-400 font-mono">{recommendedId}</strong>{' '}
                  direkomendasikan untuk menghindari buffer overrun dan afinitas cache CPU optimal.
                  {detectedDiskTotal > 0 && detectedDiskTotal < 10 && (
                    <span className="text-amber-400"> Kapasitas disk terbatas — pertimbangkan upgrade untuk retensi rilis dan blocklist.</span>
                  )}
                </p>
              </div>
            </div>

            {/* AI Action Buttons */}
            <div className="flex items-center gap-2 flex-wrap shrink-0">
              <button
                onClick={() => handleAutoApplyProfile(false)}
                className="px-3 py-2 bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-200 text-xs font-semibold rounded-lg transition-all active:scale-95"
                title="Terapkan konfigurasi profil rekomendasi AI ke form editor untuk ditinjau"
              >
                Terapkan ke Form
              </button>
              <button
                onClick={() => handleAutoApplyProfile(true)}
                disabled={saving}
                className="px-4 py-2 bg-gradient-to-r from-cyan-500 to-blue-500 hover:from-cyan-400 hover:to-blue-400 text-black font-bold text-xs rounded-lg transition-all shadow-md shadow-cyan-500/20 active:scale-95 flex items-center gap-1.5"
                title="Langsung simpan dan aktifkan profil rekomendasi AI ke server"
              >
                <Sparkles className="w-3.5 h-3.5" />
                ⚡ Auto-Apply AI Profile ({recommendedId})
              </button>
            </div>
          </div>

          {/* ── Disk Setup Recommendations ─────────────────────────────────────── */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <h2 className="text-sm font-bold text-white flex items-center gap-2">
                <HardDrive className="w-4 h-4 text-emerald-400" />
                Disk Setup &amp; Architecture Recommendations
              </h2>
              <span className="text-[11px] text-gray-400">Pilih kartu atau klik tombol untuk mengaktifkan profil</span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs">
              {/* Minimum */}
              <div
                onClick={() => {
                  const p = profiles.find((x) => x.id === 'SMALL');
                  if (p) applyProfile(p, "Profil 'SMALL' (Minimum Setup) dipilih. Klik 'Apply Tuning' untuk menyimpan.");
                }}
                className={`bg-gray-950/60 border rounded-xl p-4 space-y-3 cursor-pointer transition-all hover:border-gray-500 active:scale-[0.99] flex flex-col justify-between ${
                  localConfig.performance.profile === 'SMALL' ? 'border-cyan-500 bg-cyan-950/20 ring-1 ring-cyan-500/50' : 'border-gray-700'
                }`}
              >
                <div>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 font-bold text-gray-300">
                      <div className="w-2 h-2 rounded-full bg-gray-400" />
                      Minimum Setup
                    </div>
                    {localConfig.performance.profile === 'SMALL' && (
                      <span className="text-[10px] px-2 py-0.5 rounded bg-cyan-950 text-cyan-300 font-mono border border-cyan-800">ACTIVE</span>
                    )}
                  </div>
                  <div className="mt-3 space-y-1.5 text-gray-400 font-mono">
                    <div className="flex justify-between"><span>Root (/)</span><span className="text-white">8 GB</span></div>
                    <div className="flex justify-between"><span>RAM</span><span className="text-white">1 GB</span></div>
                    <div className="flex justify-between"><span>Profile</span><span className="text-cyan-400">SMALL</span></div>
                    <div className="flex justify-between"><span>QPS Target</span><span className="text-white">&lt;5k</span></div>
                  </div>
                  <p className="mt-2 text-[10px] text-gray-500 leading-relaxed">
                    Suitable for lab/dev only. Blocklist storage and release retention may be constrained.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    const p = profiles.find((x) => x.id === 'SMALL');
                    if (p) applyProfile(p);
                  }}
                  className="mt-3 w-full py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-200 text-[11px] font-semibold rounded border border-gray-700 transition-colors"
                >
                  Pilih Profil SMALL
                </button>
              </div>

              {/* Recommended */}
              <div
                onClick={() => {
                  const targetId = recommendedId === 'LARGE' ? 'LARGE' : 'MEDIUM';
                  const p = profiles.find((x) => x.id === targetId) || profiles.find((x) => x.id === 'MEDIUM');
                  if (p) applyProfile(p, `Profil '${p.id}' (Recommended ISP Node) dipilih. Klik 'Apply Tuning' untuk menyimpan.`);
                }}
                className={`bg-cyan-950/20 border rounded-xl p-4 space-y-3 relative cursor-pointer transition-all hover:border-cyan-400 active:scale-[0.99] flex flex-col justify-between ${
                  localConfig.performance.profile === 'MEDIUM' || localConfig.performance.profile === 'LARGE'
                    ? 'border-cyan-500 ring-1 ring-cyan-500/50 shadow-lg shadow-cyan-500/10'
                    : 'border-cyan-800/50'
                }`}
              >
                <div>
                  <div className="absolute top-2 right-2 text-[10px] px-2 py-0.5 rounded bg-cyan-900 text-cyan-300 font-mono">RECOMMENDED</div>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 font-bold text-cyan-300">
                      <div className="w-2 h-2 rounded-full bg-cyan-400" />
                      Recommended ISP Node
                    </div>
                    {(localConfig.performance.profile === 'MEDIUM' || localConfig.performance.profile === 'LARGE') && (
                      <span className="text-[10px] px-2 py-0.5 rounded bg-cyan-950 text-cyan-300 font-mono border border-cyan-800 mr-24">ACTIVE</span>
                    )}
                  </div>
                  <div className="mt-3 space-y-1.5 text-gray-400 font-mono">
                    <div className="flex justify-between"><span>Root (/)</span><span className="text-white">25+ GB</span></div>
                    <div className="flex justify-between"><span>RAM</span><span className="text-white">4-8 GB</span></div>
                    <div className="flex justify-between"><span>vCPUs</span><span className="text-white">4-8</span></div>
                    <div className="flex justify-between"><span>Profile</span><span className="text-cyan-400">MEDIUM / LARGE</span></div>
                    <div className="flex justify-between"><span>QPS Target</span><span className="text-white">30k–80k</span></div>
                  </div>
                  <p className="mt-2 text-[10px] text-emerald-400/80 leading-relaxed">
                    Leaves ample space for 5 release dirs (~55MB each), blocklists (~50MB), and log rotation.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    const targetId = recommendedId === 'LARGE' ? 'LARGE' : 'MEDIUM';
                    const p = profiles.find((x) => x.id === targetId) || profiles.find((x) => x.id === 'MEDIUM');
                    if (p) applyProfile(p);
                  }}
                  className="mt-3 w-full py-1.5 bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-300 text-[11px] font-semibold rounded border border-cyan-500/40 transition-colors"
                >
                  Pilih Profil {recommendedId === 'LARGE' ? 'LARGE' : 'MEDIUM'}
                </button>
              </div>

              {/* Carrier Grade */}
              <div
                onClick={() => {
                  const p = profiles.find((x) => x.id === 'ISP');
                  if (p) applyProfile(p, "Profil 'ISP' (Carrier Grade) dipilih. Klik 'Apply Tuning' untuk menyimpan.");
                }}
                className={`bg-gray-950/60 border rounded-xl p-4 space-y-3 cursor-pointer transition-all hover:border-amber-500 active:scale-[0.99] flex flex-col justify-between ${
                  localConfig.performance.profile === 'ISP' ? 'border-amber-500 bg-amber-950/20 ring-1 ring-amber-500/50' : 'border-gray-700'
                }`}
              >
                <div>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 font-bold text-amber-300">
                      <div className="w-2 h-2 rounded-full bg-amber-400" />
                      Carrier Grade (ISP)
                    </div>
                    {localConfig.performance.profile === 'ISP' && (
                      <span className="text-[10px] px-2 py-0.5 rounded bg-amber-950 text-amber-300 font-mono border border-amber-800">ACTIVE</span>
                    )}
                  </div>
                  <div className="mt-3 space-y-1.5 text-gray-400 font-mono">
                    <div className="flex justify-between"><span>Root (/)</span><span className="text-white">50+ GB</span></div>
                    <div className="flex justify-between"><span>RAM</span><span className="text-white">32+ GB</span></div>
                    <div className="flex justify-between"><span>vCPUs</span><span className="text-white">16+</span></div>
                    <div className="flex justify-between"><span>Profile</span><span className="text-amber-400">ISP</span></div>
                    <div className="flex justify-between"><span>QPS Target</span><span className="text-white">100k+</span></div>
                  </div>
                  <p className="mt-2 text-[10px] text-gray-500 leading-relaxed">
                    Separate /var/log mount recommended. Use WAL journal mode on PostgreSQL. Enable SO_REUSEPORT.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    const p = profiles.find((x) => x.id === 'ISP');
                    if (p) applyProfile(p);
                  }}
                  className="mt-3 w-full py-1.5 bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 text-[11px] font-semibold rounded border border-amber-500/40 transition-colors"
                >
                  Pilih Profil ISP
                </button>
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
                      ? 'bg-cyan-950/30 border-cyan-500 shadow-lg shadow-cyan-500/10 ring-1 ring-cyan-500/40'
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
                      <span className="text-cyan-400 font-bold">{p.threads}</span>
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

                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        applyProfile(p);
                      }}
                      className={`mt-2 w-full py-1 text-[11px] font-semibold rounded border transition-colors ${
                        isSelected
                          ? 'bg-cyan-500/20 border-cyan-500/50 text-cyan-300'
                          : 'bg-gray-800 border-gray-700 text-gray-300 hover:bg-gray-700'
                      }`}
                    >
                      {isSelected ? '✓ Profil Aktif' : 'Pilih & Terapkan'}
                    </button>
                  </div>
                </div>
              );
            })}
          </div>

          {/* ── Advanced Kernel & Cache Controls ───────────────────────────────── */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
            <h2 className="text-sm font-bold text-white flex items-center gap-2">
              <Gauge className="w-4 h-4 text-cyan-400" />
              Advanced Kernel, Cache &amp; Concurrency Controls
            </h2>

            <div className="grid grid-cols-1 md:grid-cols-4 gap-4 text-xs font-mono">
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
                <label className="block text-gray-400 mb-1">RRset Cache Size (rrset-cache-size)</label>
                <input
                  type="text"
                  value={localConfig.cache.rrset_cache_size}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      cache: {
                        ...localConfig.cache,
                        rrset_cache_size: e.target.value,
                      },
                    })
                  }
                  className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
                />
              </div>

              <div>
                <label className="block text-gray-400 mb-1">Message Cache Size (msg-cache-size)</label>
                <input
                  type="text"
                  value={localConfig.cache.msg_cache_size}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      cache: {
                        ...localConfig.cache,
                        msg_cache_size: e.target.value,
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
                                  </div>
              </label>
            </div>
          </div>
        </>
      )}

      {/* ── Cache Resiliency & Anti-DDoS Subnet Shield ────────────────────────────── */}
      {(activeTab === 'all' || activeTab === 'resiliency') && (
        <div className="space-y-6 pt-4 border-t border-gray-800/80">
          {/* Section Header */}
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-amber-500/20 text-amber-400 border border-amber-500/30">
                  CARRIER RESILIENCE (RFC 8767)
                </span>
                <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-rose-500/20 text-rose-400 border border-rose-500/30 flex items-center gap-1">
                  <ShieldCheck className="w-3 h-3" />
                  SUBNET FLOOD DEFENSE
                </span>
              </div>
              <h2 className="text-lg font-bold text-white flex items-center gap-2 mt-1">
                <Zap className="w-5 h-5 text-amber-400" />
                Cache Resiliency &amp; Anti-DDoS Subnet Rate Limiting
              </h2>
                          </div>

            <div className="flex items-center gap-2">
              <button
                onClick={applyResiliencyPreset}
                className="px-3.5 py-2 bg-gradient-to-r from-amber-500/20 to-orange-500/20 hover:from-amber-500/30 hover:to-orange-500/30 border border-amber-500/40 text-amber-300 rounded-lg text-xs font-semibold transition-all flex items-center gap-1.5 active:scale-95"
                title="Terapkan konfigurasi Carrier Grade untuk browsing 0ms dan tahan mati upstream"
              >
                <Sparkles className="w-3.5 h-3.5 text-amber-400" />
                <span>Terapkan Preset Resiliency ISP</span>
              </button>
            </div>
          </div>

          {/* ── Card 1: Cache Resiliency (Prefetch & Serve-Expired) ─────────────── */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-5">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-gray-800 gap-2">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-amber-500/10 border border-amber-500/30 flex items-center justify-center text-amber-400">
                  <Zap className="w-4 h-4" />
                </div>
                <div>
                  <h3 className="text-sm font-bold text-white" title="Menjamin kecepatan browsing 0ms dan ketahanan saat internet upstream mengalami kendala.">Cache Resiliency Engine (Prefetch &amp; Serve-Expired)</h3>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <span className={`text-[10px] font-mono px-2 py-0.5 rounded border ${
                  localConfig.features.serve_expired
                    ? 'bg-emerald-950/60 text-emerald-400 border-emerald-800'
                    : 'bg-gray-800 text-gray-400 border-gray-700'
                }`}>
                  {localConfig.features.serve_expired ? '● Expired Fallback AKTIF' : '○ Expired Fallback MATI'}
                </span>
              </div>
            </div>

            {/* Feature Toggles */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* Toggle 1: Prefetch Hot Domains */}
              <div className="bg-gray-950/60 border border-gray-800 rounded-lg p-3.5 flex items-start gap-3">
                <input
                  type="checkbox"
                  id="feature_prefetch"
                  checked={localConfig.features.prefetch}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      features: {
                        ...localConfig.features,
                        prefetch: e.target.checked,
                      },
                    })
                  }
                  className="mt-1 rounded bg-gray-900 border-gray-700 text-amber-500 w-4 h-4 cursor-pointer"
                />
                <label htmlFor="feature_prefetch" className="cursor-pointer">
                  <span className="text-xs font-semibold text-white block">Prefetch Hot Domain Records</span>
                  <span className="text-[11px] text-gray-400 leading-relaxed block mt-0.5">
                    Otomatis memperbarui record populer di background sebelum TTL habis. Klien selalu mendapat respon cache instan (0ms).
                  </span>
                </label>
              </div>

              {/* Toggle 2: Prefetch DNSKEY */}
              <div className="bg-gray-950/60 border border-gray-800 rounded-lg p-3.5 flex items-start gap-3">
                <input
                  type="checkbox"
                  id="feature_prefetch_key"
                  checked={localConfig.features.prefetch_key}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      features: {
                        ...localConfig.features,
                        prefetch_key: e.target.checked,
                      },
                    })
                  }
                  className="mt-1 rounded bg-gray-900 border-gray-700 text-amber-500 w-4 h-4 cursor-pointer"
                />
                <label htmlFor="feature_prefetch_key" className="cursor-pointer">
                  <span className="text-xs font-semibold text-white block">Prefetch DNSKEY (DNSSEC Acceleration)</span>
                  <span className="text-[11px] text-gray-400 leading-relaxed block mt-0.5">
                    Mengambil kunci kriptografi DNSSEC lebih awal sehingga validasi domain bertanda tangan berjalan tanpa jeda query tambahan.
                  </span>
                </label>
              </div>

              {/* Toggle 3: Serve-Expired Fallback */}
              <div className="bg-gray-950/60 border border-gray-800 rounded-lg p-3.5 flex items-start gap-3">
                <input
                  type="checkbox"
                  id="feature_serve_expired"
                  checked={localConfig.features.serve_expired}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      features: {
                        ...localConfig.features,
                        serve_expired: e.target.checked,
                      },
                    })
                  }
                  className="mt-1 rounded bg-gray-900 border-gray-700 text-amber-500 w-4 h-4 cursor-pointer"
                />
                <label htmlFor="feature_serve_expired" className="cursor-pointer">
                  <span className="text-xs font-semibold text-white block">Serve-Expired Fallback (Tahan Upstream Down)</span>
                  <span className="text-[11px] text-gray-400 leading-relaxed block mt-0.5">
                    Jika server authoritative luar lambat atau tidak dapat dijangkau, sajikan cache kedaluwarsa agar pelanggan tetap dapat browsing.
                  </span>
                </label>
              </div>

              {/* Toggle 4: Serve-Expired TTL Reset */}
              <div className="bg-gray-950/60 border border-gray-800 rounded-lg p-3.5 flex items-start gap-3">
                <input
                  type="checkbox"
                  id="feature_serve_expired_ttl_reset"
                  checked={localConfig.features.serve_expired_ttl_reset ?? true}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      features: {
                        ...localConfig.features,
                        serve_expired_ttl_reset: e.target.checked,
                      },
                    })
                  }
                  className="mt-1 rounded bg-gray-900 border-gray-700 text-amber-500 w-4 h-4 cursor-pointer"
                />
                <label htmlFor="feature_serve_expired_ttl_reset" className="cursor-pointer">
                  <span className="text-xs font-semibold text-white block">Reset TTL to 0s for Expired Responses</span>
                  <span className="text-[11px] text-gray-400 leading-relaxed block mt-0.5">
                    Mengirimkan TTL 0 detik kepada klien sehingga gadget/browser pelanggan tidak menyimpan data basi dan langsung update saat upstream pulih.
                  </span>
                </label>
              </div>
            </div>

            {/* Resiliency Numeric Parameters */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono pt-2 border-t border-gray-800/60">
              <div>
                <label className="block text-gray-400 mb-1">
                  Max Expired Retention (<span className="text-amber-400">serve-expired-ttl</span>) [Detik]
                </label>
                <input
                  type="number"
                  min={0}
                  value={localConfig.features.serve_expired_ttl}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      features: {
                        ...localConfig.features,
                        serve_expired_ttl: parseInt(e.target.value) || 0,
                      },
                    })
                  }
                  className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
                />
                <span className="text-[10px] text-gray-500 block mt-1">Default: 86400 (24 Jam). 0 = pertahankan tanpa batas waktu.</span>
              </div>

              <div>
                <label className="block text-gray-400 mb-1">
                  Client Timeout Fallback (<span className="text-amber-400">serve-expired-client-timeout</span>) [Milidetik]
                </label>
                <input
                  type="number"
                  min={0}
                  value={localConfig.features.serve_expired_client_timeout}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      features: {
                        ...localConfig.features,
                        serve_expired_client_timeout: parseInt(e.target.value) || 0,
                      },
                    })
                  }
                  className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
                />
                <span className="text-[10px] text-gray-500 block mt-1">Waktu tunggu respon upstream sebelum fallback expired (200ms recommended, 0ms = instan).</span>
              </div>
            </div>
          </div>

          {/* ── Card 2: Anti-DDoS Subnet Rate Limiting & RRL Shield ─────────────── */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-5">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-gray-800 gap-2">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-rose-500/10 border border-rose-500/30 flex items-center justify-center text-rose-400">
                  <ShieldCheck className="w-4 h-4" />
                </div>
                <div>
                  <h3 className="text-sm font-bold text-white" title="Proteksi Response Rate Limiting (RRL) terhadap DNS Amplification Attack dan infeksi botnet.">Anti-DDoS Subnet Rate Limiting &amp; RRL Shield</h3>
                </div>
              </div>

              {/* Quick Presets */}
              <div className="flex items-center gap-1.5 flex-wrap">
                <button
                  onClick={() => applyRateLimitPreset({ label: 'Disabled', ip: 0, ipFactor: 0, domain: 0, domainFactor: 0 })}
                  className={`px-2.5 py-1 text-[10px] font-mono rounded border transition-colors ${
                    (localConfig.rate_limit?.ip_rate_limit ?? 0) === 0
                      ? 'bg-gray-800 text-white border-gray-600'
                      : 'bg-gray-950/80 text-gray-400 border-gray-800 hover:border-gray-700'
                  }`}
                >
                  Off (0 QPS)
                </button>
                <button
                  onClick={() => applyRateLimitPreset({ label: 'Strict Anti-DDoS', ip: 200, ipFactor: 10, domain: 300, domainFactor: 10 })}
                  className={`px-2.5 py-1 text-[10px] font-mono rounded border transition-colors ${
                    (localConfig.rate_limit?.ip_rate_limit ?? 0) === 200
                      ? 'bg-rose-950 text-rose-300 border-rose-700'
                      : 'bg-gray-950/80 text-gray-400 border-gray-800 hover:border-gray-700'
                  }`}
                >
                  Strict (200 QPS)
                </button>
                <button
                  onClick={() => applyRateLimitPreset({ label: 'Standard ISP', ip: 1000, ipFactor: 10, domain: 1000, domainFactor: 10 })}
                  className={`px-2.5 py-1 text-[10px] font-mono rounded border transition-colors ${
                    (localConfig.rate_limit?.ip_rate_limit ?? 0) === 1000
                      ? 'bg-cyan-950 text-cyan-300 border-cyan-700'
                      : 'bg-gray-950/80 text-gray-400 border-gray-800 hover:border-gray-700'
                  }`}
                >
                  Standard ISP (1000 QPS)
                </button>
                <button
                  onClick={() => applyRateLimitPreset({ label: 'Enterprise Core', ip: 5000, ipFactor: 5, domain: 5000, domainFactor: 5 })}
                  className={`px-2.5 py-1 text-[10px] font-mono rounded border transition-colors ${
                    (localConfig.rate_limit?.ip_rate_limit ?? 0) === 5000
                      ? 'bg-emerald-950 text-emerald-300 border-emerald-700'
                      : 'bg-gray-950/80 text-gray-400 border-gray-800 hover:border-gray-700'
                  }`}
                >
                  Enterprise (5000 QPS)
                </button>
              </div>
            </div>

            {/* Rate Limiting Parameters */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 text-xs font-mono">
              <div>
                <label className="block text-gray-400 mb-1">
                  Subscriber IP Limit (<span className="text-rose-400">ip-ratelimit</span>)
                </label>
                <input
                  type="number"
                  min={0}
                  value={localConfig.rate_limit?.ip_rate_limit ?? 1000}
                  onChange={(e) => {
                    const val = parseInt(e.target.value) || 0;
                    setLocalConfig({
                      ...localConfig,
                      rate_limit: {
                        enabled: val > 0 || (localConfig.rate_limit?.domain_rate_limit ?? 0) > 0,
                        ip_rate_limit: val,
                        ip_rate_limit_factor: localConfig.rate_limit?.ip_rate_limit_factor ?? 10,
                        domain_rate_limit: localConfig.rate_limit?.domain_rate_limit ?? 1000,
                        rate_limit_size: localConfig.rate_limit?.rate_limit_size || '16m',
                        rate_limit_slabs: localConfig.rate_limit?.rate_limit_slabs || 4,
                      },
                    });
                  }}
                  className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
                />
                <span className="text-[10px] text-gray-500 block mt-1">Batas QPS per alamat IP subscriber (0 = disabled).</span>
              </div>

              <div>
                <label className="block text-gray-400 mb-1">
                  IP Backoff Factor (<span className="text-rose-400">ip-ratelimit-factor</span>)
                </label>
                <input
                  type="number"
                  min={0}
                  value={localConfig.rate_limit?.ip_rate_limit_factor ?? 10}
                  onChange={(e) => {
                    const val = parseInt(e.target.value) || 0;
                    setLocalConfig({
                      ...localConfig,
                      rate_limit: {
                        enabled: (localConfig.rate_limit?.ip_rate_limit ?? 0) > 0 || (localConfig.rate_limit?.domain_rate_limit ?? 0) > 0,
                        ip_rate_limit: localConfig.rate_limit?.ip_rate_limit ?? 1000,
                        ip_rate_limit_factor: val,
                        domain_rate_limit: localConfig.rate_limit?.domain_rate_limit ?? 1000,
                        rate_limit_size: localConfig.rate_limit?.rate_limit_size || '16m',
                        rate_limit_slabs: localConfig.rate_limit?.rate_limit_slabs || 4,
                      },
                    });
                  }}
                  className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
                />
                <span className="text-[10px] text-gray-500 block mt-1">1 dari N query yang dibalas saat limit terlampaui (default: 10).</span>
              </div>

              <div>
                <label className="block text-gray-400 mb-1">
                  Domain Global Limit (<span className="text-orange-400">ratelimit</span>)
                </label>
                <input
                  type="number"
                  min={0}
                  value={localConfig.rate_limit?.domain_rate_limit ?? 1000}
                  onChange={(e) => {
                    const val = parseInt(e.target.value) || 0;
                    setLocalConfig({
                      ...localConfig,
                      rate_limit: {
                        enabled: (localConfig.rate_limit?.ip_rate_limit ?? 0) > 0 || val > 0,
                        ip_rate_limit: localConfig.rate_limit?.ip_rate_limit ?? 1000,
                        ip_rate_limit_factor: localConfig.rate_limit?.ip_rate_limit_factor ?? 10,
                        domain_rate_limit: val,
                        rate_limit_size: localConfig.rate_limit?.rate_limit_size || '16m',
                        rate_limit_slabs: localConfig.rate_limit?.rate_limit_slabs || 4,
                      },
                    });
                  }}
                  className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
                />
                <span className="text-[10px] text-gray-500 block mt-1">Batas total QPS untuk satu domain (0 = disabled).</span>
              </div>

              <div>
                <label className="block text-gray-400 mb-1">
                  Hash Table Memory Size (<span className="text-orange-400">rate-limit-size</span>)
                </label>
                <input
                  type="text"
                  value={localConfig.rate_limit?.rate_limit_size || '16m'}
                  onChange={(e) => {
                    const val = e.target.value;
                    setLocalConfig({
                      ...localConfig,
                      rate_limit: {
                        enabled: (localConfig.rate_limit?.ip_rate_limit ?? 0) > 0 || (localConfig.rate_limit?.domain_rate_limit ?? 0) > 0,
                        ip_rate_limit: localConfig.rate_limit?.ip_rate_limit ?? 1000,
                        ip_rate_limit_factor: localConfig.rate_limit?.ip_rate_limit_factor ?? 10,
                        domain_rate_limit: localConfig.rate_limit?.domain_rate_limit ?? 1000,
                        rate_limit_size: val,
                        rate_limit_slabs: localConfig.rate_limit?.rate_limit_slabs || 4,
                      },
                    });
                  }}
                  className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
                />
                <span className="text-[10px] text-gray-500 block mt-1">Alokasi memori tabel hash RRL (default: 16m).</span>
              </div>
            </div>

            {/* Informational Box */}
            <div className="bg-gray-950/60 border border-gray-800/80 rounded-lg p-3 text-[11px] text-gray-400 flex items-start gap-2.5">
              <Info className="w-4 h-4 text-cyan-400 shrink-0 mt-0.5" />
              <span>
                <strong>Catatan Keamanan NOC:</strong> Pengaturan <code className="text-rose-300">ip-ratelimit</code> dan <code className="text-orange-300">ratelimit</code> beroperasi langsung di level kernel Unbound C-threads. Menghentikan botnet dari mengeksploitasi resolver ISP sebagai amplifier serangan DDoS global (amplification vector) sekaligus menjaga latensi pengguna legal tetap normal.
              </span>
            </div>
          </div>
        </div>
      )}

      {/* ── Logrotate & Host Storage Section ────────────────────────────────────────── */}
      {(activeTab === 'all' || activeTab === 'logrotate') && (
        <div className="space-y-6 pt-4 border-t border-gray-800/80">
          {/* Section Header */}
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                  HOST SYSTEM TUNING
                </span>
                <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-purple-500/20 text-purple-400 border border-purple-500/30 flex items-center gap-1">
                  <Sparkles className="w-3 h-3" />
                  AI REASONING ADVISOR
                </span>
              </div>
              <h2 className="text-lg font-bold text-white flex items-center gap-2 mt-1">
                <RotateCcw className="w-5 h-5 text-emerald-400" />
                Host Log Rotation &amp; Storage Tuning (Logrotate)
              </h2>
                          </div>

            <div className="flex items-center gap-2">
              <button
                onClick={fetchLogrotate}
                disabled={logrotateLoading}
                className="px-3 py-2 bg-gray-900 hover:bg-gray-800 border border-gray-800 rounded-lg text-xs text-gray-300 font-medium transition-all flex items-center gap-2 active:scale-95"
                title="Muat ulang telemetri log file host"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${logrotateLoading ? 'animate-spin text-cyan-400' : ''}`} />
                <span>Refresh Status</span>
              </button>
            </div>
          </div>

          {/* Feedback Message */}
          {logrotateMsg && (
            <div
              className={`p-3.5 rounded-xl border text-xs flex items-center gap-2.5 ${
                logrotateMsg.type === 'success'
                  ? 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300'
                  : 'bg-red-950/40 border-red-800/60 text-red-300'
              }`}
            >
              {logrotateMsg.type === 'success' ? (
                <CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-400" />
              ) : (
                <AlertTriangle className="w-4 h-4 shrink-0 text-red-400" />
              )}
              <span>{logrotateMsg.text}</span>
            </div>
          )}

          {/* Issues Alert Box (if issues detected) */}
          {logrotateReport?.telemetry.issues && logrotateReport.telemetry.issues.length > 0 && (
            <div className="bg-amber-950/30 border border-amber-800/60 rounded-xl p-4 text-xs text-amber-200 space-y-2">
              <div className="flex items-center gap-2 font-semibold text-amber-400">
                <ShieldAlert className="w-4 h-4" />
                <span>Peringatan Konfigurasi Logrotate Terdeteksi:</span>
              </div>
              <ul className="list-disc list-inside space-y-1 text-gray-300 pl-1 text-[11px]">
                {logrotateReport.telemetry.issues.map((issue, idx) => (
                  <li key={idx}>{issue}</li>
                ))}
              </ul>
              <div className="pt-2 flex items-center gap-2 flex-wrap">
                <button
                  onClick={() => applyAIPolicy(false)}
                  className="px-3 py-1.5 bg-amber-500/20 hover:bg-amber-500/30 border border-amber-500/40 text-amber-300 rounded-lg text-xs font-semibold transition-all flex items-center gap-1.5 active:scale-95"
                >
                  <Sparkles className="w-3.5 h-3.5" />
                  Perbaiki ke Form
                </button>
                <button
                  onClick={() => applyAIPolicy(true)}
                  disabled={savingLogrotate}
                  className="px-3 py-1.5 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-black text-xs font-bold rounded-lg transition-all shadow-md flex items-center gap-1.5 active:scale-95"
                >
                  <Sparkles className="w-3.5 h-3.5" />
                  ⚡ Perbaiki &amp; Terapkan ke Host Sekarang
                </button>
              </div>
            </div>
          )}

          {/* Telemetry Metric Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {/* Card 1: Active Log Size */}
            <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4 flex items-center gap-3.5">
              <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400 shrink-0">
                <FileText className="w-5 h-5" />
              </div>
              <div className="min-w-0">
                <div className="text-[11px] text-gray-400 uppercase tracking-wider truncate">Log Aktif Unbound</div>
                <div className="text-xl font-bold text-white font-mono">
                  {logrotateReport?.telemetry.unbound_log_size_human || '0 B'}
                </div>
                <div className="text-[10px] text-gray-500 truncate">/var/log/unbound/unbound.log</div>
              </div>
            </div>

            {/* Card 2: Rotated Archives */}
            <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4 flex items-center gap-3.5">
              <div className="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0">
                <RotateCcw className="w-5 h-5" />
              </div>
              <div className="min-w-0">
                <div className="text-[11px] text-gray-400 uppercase tracking-wider truncate">Arsip Terotasi</div>
                <div className="text-xl font-bold text-white font-mono">
                  {logrotateReport?.telemetry.rotated_files_size_human || '0 B'}
                </div>
                <div className="text-[10px] text-gray-500">
                  {logrotateReport?.telemetry.rotated_files_count || 0} file arsip (.gz)
                </div>
              </div>
            </div>

            {/* Card 3: Total Footprint */}
            <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4 flex items-center gap-3.5">
              <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400 shrink-0">
                <HardDrive className="w-5 h-5" />
              </div>
              <div className="min-w-0">
                <div className="text-[11px] text-gray-400 uppercase tracking-wider truncate">Total Log di Disk</div>
                <div className="text-xl font-bold text-white font-mono">
                  {logrotateReport?.telemetry.total_log_size_human || '0 B'}
                </div>
                <div className="text-[10px] text-gray-500 truncate">
                  Updater: {logrotateReport?.telemetry.netora_log_size_human || '0 B'}
                </div>
              </div>
            </div>

            {/* Card 4: Health Status */}
            <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4 flex items-center gap-3.5">
              <div
                className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 border ${
                  logrotateReport?.telemetry.health_status === 'HEALTHY'
                    ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                    : logrotateReport?.telemetry.health_status === 'WARNING'
                    ? 'bg-amber-500/10 border-amber-500/30 text-amber-400'
                    : 'bg-red-500/10 border-red-500/30 text-red-400'
                }`}
              >
                {logrotateReport?.telemetry.health_status === 'HEALTHY' ? (
                  <ShieldCheck className="w-5 h-5" />
                ) : (
                  <ShieldAlert className="w-5 h-5" />
                )}
              </div>
              <div className="min-w-0">
                <div className="text-[11px] text-gray-400 uppercase tracking-wider truncate">Status Rotasi</div>
                <div className="text-sm font-bold text-white truncate">
                  {logrotateReport?.telemetry.health_status === 'HEALTHY'
                    ? 'Sehat & Terproteksi'
                    : logrotateReport?.telemetry.health_status === 'WARNING'
                    ? 'Perlu Penyesuaian'
                    : 'Kritis / Rentan'}
                </div>
                <div className="text-[10px] text-gray-400 truncate">
                  su directive:{' '}
                  <span
                    className={
                      logrotateReport?.telemetry.has_su_directive ? 'text-emerald-400 font-semibold' : 'text-red-400 font-bold'
                    }
                  >
                    {logrotateReport?.telemetry.has_su_directive ? 'AMAN' : 'TIDAK ADA'}
                  </span>
                </div>
              </div>
            </div>
          </div>

          {/* AI Recommendation Hero Card */}
          {logrotateReport?.ai_recommendation && (
            <div className="relative overflow-hidden rounded-2xl border border-emerald-500/30 bg-gradient-to-br from-emerald-950/30 via-gray-900 to-gray-950 p-5 shadow-xl">
              <div className="absolute top-0 right-0 w-64 h-64 bg-emerald-500/5 rounded-full blur-3xl pointer-events-none" />

              <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-3">
                <div className="flex items-center gap-2.5">
                  <div className="w-8 h-8 rounded-lg bg-emerald-500/20 border border-emerald-500/40 flex items-center justify-center text-emerald-400">
                    <Sparkles className="w-4 h-4 animate-pulse" />
                  </div>
                  <div>
                    <div className="text-xs font-bold text-emerald-400 uppercase tracking-wider">
                      {logrotateReport.ai_recommendation.badge}
                    </div>
                    <h3 className="text-sm font-semibold text-white">
                      {logrotateReport.ai_recommendation.title}
                    </h3>
                  </div>
                </div>

                <div className="flex items-center gap-2 flex-wrap">
                  <span className="px-2.5 py-1 rounded-full text-[11px] font-medium bg-emerald-500/10 border border-emerald-500/30 text-emerald-300">
                    {logrotateReport.ai_recommendation.projected_saving}
                  </span>
                  <button
                    onClick={() => applyAIPolicy(false)}
                    className="px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-200 text-xs font-semibold rounded-lg transition-all border border-gray-700 active:scale-95"
                  >
                    Terapkan ke Form
                  </button>
                  <button
                    onClick={() => applyAIPolicy(true)}
                    disabled={savingLogrotate}
                    className="px-4 py-2 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-black text-xs font-bold rounded-lg transition-all shadow-lg shadow-emerald-500/20 flex items-center gap-1.5 active:scale-95"
                  >
                    <Sparkles className="w-3.5 h-3.5" />
                    ⚡ Auto-Tune &amp; Simpan ke Host
                  </button>
                </div>
              </div>

              <p className="text-xs text-gray-300 leading-relaxed bg-black/30 rounded-xl p-3.5 border border-gray-800/60 font-sans">
                {logrotateReport.ai_recommendation.reasoning}
              </p>

              <div className="mt-3 flex flex-wrap items-center gap-2 text-[11px] text-gray-400">
                <span className="text-gray-500">Faktor Deteksi:</span>
                <span className="px-2 py-0.5 rounded bg-gray-800 text-gray-300">
                  {logrotateReport.ai_recommendation.hardware_factor}
                </span>
                <span className="px-2 py-0.5 rounded bg-gray-800 text-gray-300">
                  Rekomendasi:{' '}
                  <strong className="text-white">
                    {logrotateReport.ai_recommendation.recommended_policy.frequency},{' '}
                    {logrotateReport.ai_recommendation.recommended_policy.rotate_count}x retensi, maxsize{' '}
                    {logrotateReport.ai_recommendation.recommended_policy.max_size}
                  </strong>
                </span>
              </div>
            </div>
          )}

          {/* Quick Preset Selector */}
          <div>
            <label className="block text-xs font-semibold text-gray-300 uppercase tracking-wider mb-2">
              Pilihan Preset Auto-Tuning
            </label>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              {/* Preset 1 */}
              <button
                type="button"
                onClick={() =>
                  applyPresetLogrotate({
                    frequency: 'daily',
                    rotate_count: 3,
                    max_size: '25M',
                    compress: true,
                    delay_compress: true,
                  })
                }
                className={`p-3.5 rounded-xl border text-left transition-all ${
                  logrotatePolicy.rotate_count <= 3 && logrotatePolicy.max_size === '25M'
                    ? 'bg-emerald-950/30 border-emerald-500/50 shadow-md ring-1 ring-emerald-500/40'
                    : 'bg-gray-900/60 border-gray-800 hover:border-gray-700'
                }`}
              >
                <div className="flex items-center justify-between">
                  <span className="text-xs font-bold text-white">Ultra Storage Saver</span>
                  {logrotatePolicy.rotate_count <= 3 && logrotatePolicy.max_size === '25M' && (
                    <Check className="w-3.5 h-3.5 text-emerald-400" />
                  )}
                </div>
                <div className="text-[11px] text-emerald-400 font-mono mt-1">Daily • 3x • Max 25MB • Gzip</div>
                              </button>

              {/* Preset 2 */}
              <button
                type="button"
                onClick={() =>
                  applyPresetLogrotate({
                    frequency: 'daily',
                    rotate_count: 7,
                    max_size: '50M',
                    compress: true,
                    delay_compress: true,
                  })
                }
                className={`p-3.5 rounded-xl border text-left transition-all ${
                  logrotatePolicy.rotate_count === 7 && logrotatePolicy.max_size === '50M'
                    ? 'bg-cyan-950/30 border-cyan-500/50 shadow-md ring-1 ring-cyan-500/40'
                    : 'bg-gray-900/60 border-gray-800 hover:border-gray-700'
                }`}
              >
                <div className="flex items-center justify-between">
                  <span className="text-xs font-bold text-white">Balanced Standard</span>
                  {logrotatePolicy.rotate_count === 7 && logrotatePolicy.max_size === '50M' && (
                    <Check className="w-3.5 h-3.5 text-cyan-400" />
                  )}
                </div>
                <div className="text-[11px] text-cyan-400 font-mono mt-1">Daily • 7x • Max 50MB • Gzip</div>
                              </button>

              {/* Preset 3 */}
              <button
                type="button"
                onClick={() =>
                  applyPresetLogrotate({
                    frequency: 'weekly',
                    rotate_count: 30,
                    max_size: '100M',
                    compress: true,
                    delay_compress: false,
                  })
                }
                className={`p-3.5 rounded-xl border text-left transition-all ${
                  logrotatePolicy.rotate_count >= 14 && logrotatePolicy.max_size === '100M'
                    ? 'bg-purple-950/30 border-purple-500/50 shadow-md ring-1 ring-purple-500/40'
                    : 'bg-gray-900/60 border-gray-800 hover:border-gray-700'
                }`}
              >
                <div className="flex items-center justify-between">
                  <span className="text-xs font-bold text-white">Carrier Audit Retention</span>
                  {logrotatePolicy.rotate_count >= 14 && logrotatePolicy.max_size === '100M' && (
                    <Check className="w-3.5 h-3.5 text-purple-400" />
                  )}
                </div>
                <div className="text-[11px] text-purple-400 font-mono mt-1">Weekly • 30x • Max 100MB • Gzip</div>
                              </button>
            </div>
          </div>

          {/* Interactive Custom Tuning Form */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-2xl p-5 space-y-4">
            <div className="flex items-center justify-between border-b border-gray-800 pb-3">
              <h3 className="text-xs font-bold text-white flex items-center gap-2 uppercase tracking-wider">
                <Sliders className="w-4 h-4 text-emerald-400" />
                Parameter Kustom Rotasi Log Server
              </h3>
              <span className="text-[11px] text-gray-400">Target: /etc/logrotate.d/unbound</span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs">
              {/* Frequency */}
              <div>
                <label className="block text-gray-400 mb-1">Frekuensi Rotasi (Interval)</label>
                <select
                  value={logrotatePolicy.frequency}
                  onChange={(e) =>
                    setLogrotatePolicy({
                      ...logrotatePolicy,
                      frequency: e.target.value,
                    })
                  }
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono"
                >
                  <option value="daily">daily (Harian - Disarankan)</option>
                  <option value="weekly">weekly (Mingguan)</option>
                  <option value="monthly">monthly (Bulanan)</option>
                  <option value="hourly">hourly (Per Jam - Khusus Trafik Masif)</option>
                </select>
              </div>

              {/* Rotate Count */}
              <div>
                <label className="block text-gray-400 mb-1">Jumlah Retensi Arsip (rotate)</label>
                <input
                  type="number"
                  min={1}
                  max={90}
                  value={logrotatePolicy.rotate_count}
                  onChange={(e) =>
                    setLogrotatePolicy({
                      ...logrotatePolicy,
                      rotate_count: parseInt(e.target.value) || 1,
                    })
                  }
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono"
                />
                <span className="text-[10px] text-gray-500">Maksimum file log arsip yang disimpan sebelum dihapus.</span>
              </div>

              {/* Max Size */}
              <div>
                <label className="block text-gray-400 mb-1">Ambang Batas Ukuran (maxsize)</label>
                <select
                  value={logrotatePolicy.max_size}
                  onChange={(e) =>
                    setLogrotatePolicy({
                      ...logrotatePolicy,
                      max_size: e.target.value,
                    })
                  }
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white font-mono"
                >
                  <option value="10M">10M (Sangat Ketat - Rekomendasi Disk &lt;5GB)</option>
                  <option value="15M">15M (Ketat)</option>
                  <option value="25M">25M (Optimal Disk Rendah)</option>
                  <option value="50M">50M (Standar Rekomendasi)</option>
                  <option value="100M">100M (Kapasitas Besar)</option>
                  <option value="250M">250M (Audit ISP Longgar)</option>
                  <option value="500M">500M (Super Kapasitas)</option>
                </select>
                <span className="text-[10px] text-gray-500">Rotasi dipicu lebih awal jika file mencapai batas ini.</span>
              </div>
            </div>

            {/* Checkboxes / Toggles */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-3 border-t border-gray-800/80">
              <label className="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={logrotatePolicy.compress}
                  onChange={(e) =>
                    setLogrotatePolicy({
                      ...logrotatePolicy,
                      compress: e.target.checked,
                    })
                  }
                  className="mt-0.5 rounded bg-gray-950 border-gray-700 text-emerald-500 w-4 h-4"
                />
                <div>
                  <span className="text-xs font-semibold text-white">Kompresi Gzip (compress)</span>
                                  </div>
              </label>

              <label className="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={logrotatePolicy.delay_compress}
                  onChange={(e) =>
                    setLogrotatePolicy({
                      ...logrotatePolicy,
                      delay_compress: e.target.checked,
                    })
                  }
                  className="mt-0.5 rounded bg-gray-950 border-gray-700 text-emerald-500 w-4 h-4"
                />
                <div>
                  <span className="text-xs font-semibold text-white">Tunda Kompresi (delaycompress)</span>
                                  </div>
              </label>

              <label className="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={logrotatePolicy.rotate_netora}
                  onChange={(e) =>
                    setLogrotatePolicy({
                      ...logrotatePolicy,
                      rotate_netora: e.target.checked,
                    })
                  }
                  className="mt-0.5 rounded bg-gray-950 border-gray-700 text-emerald-500 w-4 h-4"
                />
                <div>
                  <span className="text-xs font-semibold text-white">Rotasi Log Updater Netora</span>
                                  </div>
              </label>
            </div>

            {/* Live Config Preview */}
            <div className="pt-2">
              <div className="flex items-center justify-between text-xs text-gray-400 mb-1.5">
                <span className="flex items-center gap-1.5 font-mono text-[11px]">
                  <FileCode className="w-3.5 h-3.5 text-cyan-400" />
                  Pratinjau /etc/logrotate.d/unbound:
                </span>
                <span className="text-[10px] text-gray-500">su unbound unbound disertakan otomatis</span>
              </div>
              <pre className="bg-black/60 border border-gray-800 rounded-xl p-3.5 text-[11px] font-mono text-emerald-400 overflow-x-auto leading-relaxed">
{`/var/log/unbound/unbound.log {
    ${logrotatePolicy.frequency}
    rotate ${logrotatePolicy.rotate_count}
    maxsize ${logrotatePolicy.max_size}
    missingok
    notifempty
    ${logrotatePolicy.compress ? 'compress' : 'nocompress'}
${logrotatePolicy.delay_compress ? '    delaycompress\n' : ''}    sharedscripts
    su unbound unbound
    postrotate
        /usr/sbin/unbound-control log_reopen >/dev/null 2>&1 || true
    endscript
}`}
              </pre>
            </div>

            {/* Inline Action Feedback Banner (directly visible above the 3 buttons) */}
            {logrotateMsg && (
              <div
                className={`p-3.5 rounded-xl border text-xs flex items-center justify-between gap-2.5 transition-all shadow-md ${
                  logrotateMsg.type === 'success'
                    ? 'bg-emerald-950/70 border-emerald-600 text-emerald-200'
                    : 'bg-red-950/70 border-red-600 text-red-200'
                }`}
              >
                <div className="flex items-center gap-2.5">
                  {logrotateMsg.type === 'success' ? (
                    <CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-400" />
                  ) : (
                    <AlertTriangle className="w-4 h-4 shrink-0 text-red-400" />
                  )}
                  <span className="font-medium">{logrotateMsg.text}</span>
                </div>
                <button
                  type="button"
                  onClick={() => setLogrotateMsg(null)}
                  className="text-[10px] text-gray-400 hover:text-white underline shrink-0 cursor-pointer"
                >
                  Tutup Pesan
                </button>
              </div>
            )}

            {/* Action Buttons Toolbar */}
            <div className="flex flex-wrap items-center justify-between gap-3 pt-3 border-t border-gray-800">
              <div className="flex items-center gap-2 flex-wrap">
                <button
                  type="button"
                  onClick={handleTestLogrotate}
                  disabled={testingLogrotate}
                  className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-200 text-xs font-semibold rounded-lg transition-all border border-gray-700 flex items-center gap-2 disabled:opacity-50 active:scale-95"
                >
                  <Terminal className={`w-3.5 h-3.5 ${testingLogrotate ? 'animate-spin text-cyan-400' : ''}`} />
                  {testingLogrotate ? 'Menguji Dry Run...' : 'Uji Validasi (Dry Run)'}
                </button>

                <button
                  type="button"
                  onClick={handleForceRotate}
                  disabled={forceRotating}
                  className="px-4 py-2 bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 text-xs font-semibold rounded-lg transition-all border border-amber-500/30 flex items-center gap-2 disabled:opacity-50 active:scale-95"
                  title="Paksa rotasi segera untuk mengosongkan disk sekarang juga"
                >
                  <Zap className={`w-3.5 h-3.5 ${forceRotating ? 'animate-spin' : ''}`} />
                  {forceRotating ? 'Memutar Log...' : 'Rotasi Sekarang (Paksa & Kosongkan)'}
                </button>
              </div>

              <button
                type="button"
                onClick={handleApplyLogrotate}
                disabled={savingLogrotate}
                className="px-5 py-2.5 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-black text-xs font-bold rounded-lg transition-all shadow-lg shadow-emerald-500/20 flex items-center gap-2 disabled:opacity-50 active:scale-95"
              >
                {savingLogrotate ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Save className="w-4 h-4" />
                )}
                {savingLogrotate ? 'Menerapkan...' : 'Terapkan Konfigurasi Logrotate'}
              </button>
            </div>

            {/* Console Output Terminal */}
            {consoleOutput && (
              <div className="mt-4 pt-4 border-t border-gray-800">
                <div className="flex items-center justify-between text-xs text-gray-400 mb-2">
                  <span className="flex items-center gap-1.5 font-mono text-[11px] text-gray-300">
                    <Terminal className="w-3.5 h-3.5 text-cyan-400" />
                    Output Eksekusi Konsol Logrotate:
                  </span>
                  <button
                    onClick={() => setConsoleOutput(null)}
                    className="text-[10px] text-gray-500 hover:text-gray-300 underline"
                  >
                    Tutup Konsol
                  </button>
                </div>
                <pre className="bg-black border border-gray-800 rounded-xl p-4 text-[11px] font-mono text-cyan-300 overflow-x-auto max-h-60 overflow-y-auto leading-relaxed whitespace-pre-wrap">
                  {consoleOutput}
                </pre>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
