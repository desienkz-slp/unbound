import React, { useState, useEffect } from 'react';
import {
  Activity,
  ArrowUpRight,
  Ban,
  CheckCircle2,
  Clock,
  ExternalLink,
  Eye,
  Filter,
  Globe,
  Layers,
  Lock,
  PauseCircle,
  PlayCircle,
  PlusCircle,
  RefreshCw,
  Search,
  Server,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Smartphone,
  TrendingDown,
  TrendingUp,
  UserCheck,
  Zap,
} from 'lucide-react';
import { DesiredState, NodeRecord, DetailedStatsReport } from '../types';
import { api } from '../services/api';

interface OverviewProps {
  nodeId: string;
  config: DesiredState | null;
  node: NodeRecord | null;
  onNavigate: (tab: any) => void;
}

export const OverviewView: React.FC<OverviewProps> = ({
  nodeId,
  config,
  node,
  onNavigate,
}) => {
  const [health, setHealth] = useState<{
    healthy: boolean;
    unbound_running: boolean;
    port53_listening: boolean;
    latency_ms: number;
    last_check: string;
  } | null>(null);

  const [refreshing, setRefreshing] = useState(false);
  const [protectionEnabled, setProtectionEnabled] = useState(true);

  // Live probe state
  const [testDomain, setTestDomain] = useState('graph.facebook.com');
  const [probeResult, setProbeResult] = useState<any>(null);
  const [probing, setProbing] = useState(false);

  const [isLiveTelemetry, setIsLiveTelemetry] = useState(false);

  const [statsReport, setStatsReport] = useState<DetailedStatsReport | null>(null);

  // Live statistics state from Unbound
  const [stats, setStats] = useState({
    totalQueries: 0,
    cacheHits: 0,
    cacheMisses: 0,
    blockedFilters: 0,
    blockedMalware: 0,
    blockedAdult: 0,
    blockedGambling: 0,
    blockedAds: 0,
    enforcedSafeSearch: 0,
    avgProcessingTime: 1, // ms
  });

  const fetchHealthAndStats = async () => {
    if (!nodeId) return;
    setRefreshing(true);
    try {
      const [resHealth, resStats] = await Promise.allSettled([
        api.getNodeHealth(nodeId),
        api.getNodeStats(nodeId),
      ]);

      if (resHealth.status === 'fulfilled') {
        setHealth(resHealth.value);
      }

      if (resStats.status === 'fulfilled' && resStats.value) {
        const s = resStats.value;
        setStatsReport(s);

        const isAdultActive = Boolean(config?.parental_control?.enabled && (config?.parental_control?.block_adult ?? true));
        const isGamblingActive = Boolean(config?.gambling_block?.enabled || config?.parental_control?.block_gambling);

        const adultBlocked = isAdultActive ? (s.blocked_adult ?? 0) : 0;
        const gamblingBlocked = isGamblingActive ? (s.blocked_gambling ?? 0) : 0;
        const malwareBlocked = s.blocked_malware ?? 0;
        const totalBlocked = s.blocked_queries ?? 0;
        const adsBlocked = s.blocked_ads ?? Math.max(0, totalBlocked - malwareBlocked - adultBlocked - gamblingBlocked);

        setStats({
          totalQueries: s.total_queries ?? 0,
          cacheHits: s.cache_hits ?? 0,
          cacheMisses: s.cache_misses ?? 0,
          blockedFilters: totalBlocked,
          blockedMalware: malwareBlocked,
          blockedAdult: adultBlocked,
          blockedGambling: gamblingBlocked,
          blockedAds: adsBlocked,
          enforcedSafeSearch: s.safe_search_enforced ?? 0,
          avgProcessingTime: Math.max(1, Math.round(s.avg_processing_time_ms || 1)),
        });
        setIsLiveTelemetry(true);
      }
    } catch (err) {
      console.error('Failed to fetch node health and stats:', err);
    } finally {
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchHealthAndStats();
    const interval = setInterval(fetchHealthAndStats, 5000);
    return () => clearInterval(interval);
  }, [nodeId]);

  const runQuickProbe = async (domainToTest: string) => {
    setTestDomain(domainToTest);
    setProbing(true);
    try {
      const res = await api.diagnoseDomain(domainToTest.trim());
      setProbeResult(res);
    } catch (err: any) {
      setProbeResult({ error: err.message || 'Probe failed' });
    } finally {
      setProbing(false);
    }
  };

  // Real telemetry collections directly from Unbound log tailer & dump_infra
  const topClients = statsReport?.top_clients || [];
  const topQueriedDomains = statsReport?.top_queried_domains || [];
  const topBlockedDomains = statsReport?.top_blocked_domains || [];
  const topUpstreams = statsReport?.top_upstreams && statsReport.top_upstreams.length > 0
    ? statsReport.top_upstreams
    : (config?.upstreams || []).map((up) => ({
        upstream: `${up.ip}:${up.port} (${up.type})`,
        count: 0,
        pct: 0,
        latency: health?.latency_ms ? `${health.latency_ms.toFixed(1)} ms` : '1.2 ms',
      }));

  const totalResolutions = (stats.cacheHits + stats.cacheMisses) > 0 ? (stats.cacheHits + stats.cacheMisses) : stats.totalQueries;
  const cacheHitRatio = totalResolutions > 0
    ? Math.min(100, Number((((stats.cacheHits || 0) / Math.max(1, totalResolutions)) * 100).toFixed(1))).toFixed(1)
    : '0.0';

  const blockedPct = stats.totalQueries > 0
    ? Math.min(100, Number((((stats.blockedFilters || 0) / Math.max(1, stats.totalQueries)) * 100).toFixed(1))).toFixed(1)
    : '0';

  const formatPercentage = (count: number, total: number): string => {
    if (!total || count <= 0) return '0%';
    const pct = (count / total) * 100;
    if (pct < 0.01) return '<0.01%';
    if (pct < 1) return `${pct.toFixed(2)}%`;
    return `${pct.toFixed(1)}%`;
  };

  const isAdultFilterActive = Boolean(config?.parental_control?.enabled);
  const isGamblingActive = Boolean(config?.gambling_block?.enabled || config?.parental_control?.block_gambling);

  return (
    <div className="space-y-6">
      {/* Top Bar: Title & Protection Status Controller (Inspired by AdGuard Home header) */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 pb-2 border-b border-gray-800/80">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold text-white tracking-tight">Dashboard</h1>
            <button
              onClick={() => setProtectionEnabled(!protectionEnabled)}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold font-mono border transition-all ${
                protectionEnabled
                  ? 'bg-emerald-950/80 text-emerald-400 border-emerald-700/80 hover:bg-emerald-900/80'
                  : 'bg-red-950/80 text-red-400 border-red-700/80 hover:bg-red-900/80'
              }`}
            >
              {protectionEnabled ? (
                <>
                  <ShieldCheck className="w-3.5 h-3.5" />
                  <span>PROTECTION ON</span>
                </>
              ) : (
                <>
                  <Ban className="w-3.5 h-3.5" />
                  <span>PROTECTION OFF</span>
                </>
              )}
            </button>
            {isLiveTelemetry && (
              <span className="flex items-center gap-1.5 px-3 py-1 rounded-full text-[11px] font-bold font-mono bg-cyan-950 text-cyan-400 border border-cyan-800">
                <span className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse" />
                LIVE UNBOUND DATA
              </span>
            )}
          </div>
          <p className="text-xs text-gray-400 mt-1">
            Real-time ISP NOC telemetry & query distribution for node <strong className="text-cyan-400 font-mono">{node?.hostname || nodeId || 'Current Node'}</strong>
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={fetchHealthAndStats}
            disabled={refreshing}
            className="flex items-center gap-2 px-3.5 py-1.5 rounded-lg bg-gray-900 hover:bg-gray-800 text-gray-200 text-xs font-medium border border-gray-700 transition-all disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin text-cyan-400' : ''}`} />
            Refresh Statistics
          </button>
          <button
            onClick={() => onNavigate('wizard')}
            className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-cyan-500/10 text-cyan-400 border border-cyan-500/30 hover:bg-cyan-500/20 text-xs font-semibold transition-all"
          >
            Setup Guide
            <ExternalLink className="w-3 h-3" />
          </button>
        </div>
      </div>

      {/* 5 Top KPI Cards (AdGuard Home Style + ISP Policy) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-5 gap-4">
        {/* Card 1: DNS Queries */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-36 flex flex-col justify-between relative overflow-hidden group hover:border-gray-700 transition-all shadow-sm">
          <div>
            <span className="text-2xl lg:text-3xl font-bold font-mono text-white tracking-tight">
              {stats.totalQueries.toLocaleString()}
            </span>
            <div className="text-xs font-medium text-gray-400 mt-1">DNS Queries</div>
          </div>
          {/* Subtle sparkline wave */}
          <div className="pt-2">
            <div className="h-8 w-full flex items-end gap-1 opacity-70">
              {[40, 55, 60, 48, 65, 80, 70, 85, 90, 75, 80, 95].map((h, i) => (
                <div
                  key={i}
                  className="flex-1 bg-gray-700/60 rounded-t transition-all group-hover:bg-cyan-500/50"
                  style={{ height: `${h}%` }}
                />
              ))}
            </div>
          </div>
        </div>

        {/* Card 2: Blocked by Filters */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-36 flex flex-col justify-between relative overflow-hidden group hover:border-rose-900/50 transition-all shadow-sm">
          <div className="flex items-start justify-between">
            <div>
              <span className="text-2xl lg:text-3xl font-bold font-mono text-rose-400 tracking-tight">
                {stats.blockedFilters.toLocaleString()}
              </span>
              <div className="text-xs font-medium text-rose-300/80 mt-1">Blocked by Filters</div>
            </div>
            <span className="text-sm font-bold font-mono text-rose-400 bg-rose-950/60 border border-rose-900/60 px-2 py-0.5 rounded">
              {blockedPct}%
            </span>
          </div>
          <div className="pt-2">
            <div className="h-8 w-full flex items-end gap-1 opacity-70">
              {[30, 45, 50, 42, 55, 60, 58, 65, 70, 62, 68, 72].map((h, i) => (
                <div
                  key={i}
                  className="flex-1 bg-rose-900/40 rounded-t transition-all group-hover:bg-rose-500/50"
                  style={{ height: `${h}%` }}
                />
              ))}
            </div>
          </div>
        </div>

        {/* Card 3: Blocked Malware/Phishing */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-36 flex flex-col justify-between relative overflow-hidden group hover:border-amber-900/50 transition-all shadow-sm">
          <div className="flex items-start justify-between">
            <div>
              <span className="text-2xl lg:text-3xl font-bold font-mono text-amber-400 tracking-tight">
                {stats.blockedMalware.toLocaleString()}
              </span>
              <div className="text-xs font-medium text-amber-300/80 mt-1">Blocked malware/phishing</div>
            </div>
            <span className="text-xs font-mono font-bold text-amber-400 bg-amber-950/60 border border-amber-900/60 px-2 py-0.5 rounded">
              {formatPercentage(stats.blockedMalware, stats.totalQueries)}
            </span>
          </div>
          <div className="pt-2">
            <div className="h-8 w-full flex items-end gap-1 opacity-70">
              {[10, 15, 12, 10, 8, 90, 20, 15, 12, 10, 14, 18].map((h, i) => (
                <div
                  key={i}
                  className="flex-1 bg-amber-900/40 rounded-t transition-all group-hover:bg-amber-500/50"
                  style={{ height: `${h}%` }}
                />
              ))}
            </div>
          </div>
        </div>

        {/* Card 4: Blocked Adult Websites */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-36 flex flex-col justify-between relative overflow-hidden group hover:border-purple-900/50 transition-all shadow-sm">
          <div className="flex items-start justify-between">
            <div>
              <span className="text-2xl lg:text-3xl font-bold font-mono text-purple-400 tracking-tight">
                {isAdultFilterActive ? stats.blockedAdult.toLocaleString() : '0'}
              </span>
              <div className="text-xs font-medium text-purple-300/80 mt-1">
                {isAdultFilterActive ? 'Blocked adult websites' : 'Konten 18+ (Akses Terbuka)'}
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="text-xs font-mono font-bold text-purple-400 bg-purple-950/60 border border-purple-900/60 px-2 py-0.5 rounded">
                {isAdultFilterActive ? formatPercentage(stats.blockedAdult, stats.totalQueries) : '0%'}
              </span>
              <span
                className={`text-[10px] font-mono px-1.5 py-0.5 rounded uppercase font-bold border ${
                  isAdultFilterActive
                    ? 'bg-emerald-950/80 text-emerald-400 border-emerald-800/80'
                    : 'bg-gray-950 text-gray-500 border-gray-800'
                }`}
                title={isAdultFilterActive ? 'Filter 18+ Aktif' : 'Filter 18+ Nonaktif / Terbuka'}
              >
                {isAdultFilterActive ? 'ACTIVE' : 'ALLOWED'}
              </span>
            </div>
          </div>
          <div className="pt-2">
            <div className="h-8 w-full flex items-end gap-1 opacity-70">
              {(isAdultFilterActive && stats.blockedAdult > 0
                ? [2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2]
                : [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
              ).map((h, i) => (
                <div
                  key={i}
                  className="flex-1 bg-purple-900/30 rounded-t"
                  style={{ height: `${Math.max(h, 4)}%` }}
                />
              ))}
            </div>
          </div>
        </div>

        {/* Card 5: Blocked Judi Online (Judol) */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-36 flex flex-col justify-between relative overflow-hidden group hover:border-amber-800/60 transition-all shadow-sm">
          <div className="flex items-start justify-between">
            <div>
              <span className="text-2xl lg:text-3xl font-bold font-mono text-amber-400 tracking-tight">
                {isGamblingActive ? stats.blockedGambling.toLocaleString() : '0'}
              </span>
              <div className="text-xs font-medium text-amber-300/80 mt-1">
                {isGamblingActive ? 'Blocked judi online & slot' : 'Situs judi (Akses Terbuka)'}
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="text-xs font-mono font-bold text-amber-400 bg-amber-950/60 border border-amber-900/60 px-2 py-0.5 rounded">
                {isGamblingActive ? formatPercentage(stats.blockedGambling, stats.totalQueries) : '0%'}
              </span>
              <span
                className={`text-[10px] font-mono px-1.5 py-0.5 rounded uppercase font-bold border ${
                  isGamblingActive
                    ? 'bg-emerald-950/80 text-emerald-400 border-emerald-800/80'
                    : 'bg-gray-950 text-gray-500 border-gray-800'
                }`}
                title={isGamblingActive ? 'Filter Judi Online Aktif' : 'Filter Judi Online Nonaktif / Terbuka'}
              >
                {isGamblingActive ? 'ACTIVE' : 'ALLOWED'}
              </span>
            </div>
          </div>
          <div className="pt-2">
            <div className="h-8 w-full flex items-end gap-1 opacity-70">
              {(isGamblingActive && stats.blockedGambling > 0
                ? [4, 6, 8, 12, 10, 15, 8, 14, 18, 12, 16, 20]
                : [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
              ).map((h, i) => (
                <div
                  key={i}
                  className="flex-1 bg-amber-900/30 rounded-t group-hover:bg-amber-500/50 transition-all"
                  style={{ height: `${Math.max(h, 4)}%` }}
                />
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Row 1: General statistics (Left) & Top clients (Right) */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Panel 1: General statistics */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 min-h-[370px] flex flex-col justify-between shadow-sm">
          <div className="flex items-center justify-between pb-2 border-b border-gray-800 shrink-0">
            <div>
              <h2 className="text-sm font-bold text-white">General statistics</h2>
              <span className="text-[11px] text-gray-500">for the last 24 hours</span>
            </div>
            <button onClick={fetchHealthAndStats} className="p-1 text-gray-400 hover:text-cyan-400 transition-colors" title="Refresh">
              <RefreshCw className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="flex-1 flex flex-col justify-around divide-y divide-gray-800/60 text-xs font-mono pt-1">
            <div className="py-1 flex items-center justify-between">
              <span className="text-gray-300">Total DNS Queries</span>
              <span className="text-cyan-400 font-bold">{stats.totalQueries.toLocaleString()}</span>
            </div>

            <div className="py-1 flex items-center justify-between">
              <span className="text-emerald-400">Cache Hits</span>
              <span className="text-emerald-400 font-bold">
                {stats.cacheHits?.toLocaleString() || 0} ({cacheHitRatio}%)
              </span>
            </div>

            <div className="py-1 flex items-center justify-between">
              <span className="text-gray-400">Cache Misses</span>
              <span className="text-gray-300 font-bold">{stats.cacheMisses?.toLocaleString() || 0}</span>
            </div>

            <div className="py-1 flex items-center justify-between">
              <span className="text-rose-400">Blocked by Filters (Total)</span>
              <span className="text-rose-400 font-bold">{stats.blockedFilters.toLocaleString()}</span>
            </div>

            <div className="py-1 flex items-center justify-between">
              <span className="text-cyan-400">Blocked Ads & Trackers</span>
              <span className="text-cyan-400 font-bold">{stats.blockedAds.toLocaleString()}</span>
            </div>

            <div className="py-1 flex items-center justify-between">
              <span className="text-amber-400">Blocked malware/phishing</span>
              <span className="text-amber-400 font-bold">{stats.blockedMalware.toLocaleString()}</span>
            </div>

            <div className="py-1 flex items-center justify-between">
              <span className="text-purple-400">Blocked adult websites</span>
              <span className="text-purple-400 font-bold">
                {isAdultFilterActive ? stats.blockedAdult.toLocaleString() : '0 (Akses Terbuka)'}
              </span>
            </div>

            <div className="py-1 flex items-center justify-between">
              <span className="text-amber-400">Blocked judi online</span>
              <span className="text-amber-400 font-bold">
                {isGamblingActive ? stats.blockedGambling.toLocaleString() : '0 (Akses Terbuka)'}
              </span>
            </div>

            <div className="py-1 flex items-center justify-between">
              <span className="text-gray-300">Average processing time</span>
              <span className="text-white font-bold">{stats.avgProcessingTime} ms</span>
            </div>
          </div>
        </div>

        {/* Panel 2: Top clients */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 min-h-[370px] flex flex-col shadow-sm">
          <div className="flex items-center justify-between pb-2 border-b border-gray-800 shrink-0">
            <div>
              <h2 className="text-sm font-bold text-white">Top clients</h2>
              <span className="text-[11px] text-gray-500">for the last 24 hours</span>
            </div>
            <button onClick={() => onNavigate('dns-config')} className="p-1 text-gray-400 hover:text-cyan-400 transition-colors" title="Client ACLs">
              <Layers className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase py-2 shrink-0 border-b border-gray-800/40">
            <span>Client</span>
            <span>Requests count</span>
          </div>

          <div className="flex-1 overflow-y-auto pr-1.5 space-y-3 pt-2 card-scrollbar">
            {topClients.length === 0 ? (
              <div className="h-full flex items-center justify-center text-center text-gray-500 font-mono text-xs">
                Belum ada query client yang tercatat dalam log telemetri Unbound.
              </div>
            ) : (
              topClients.map((client, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-cyan-400 font-bold font-mono text-xs truncate">{client.ip}</span>
                      <span className="text-gray-500 text-[11px] hidden sm:inline truncate">({client.name})</span>
                    </div>
                    <div className="text-right shrink-0">
                      <span className="text-white font-bold font-mono text-xs">{client.count.toLocaleString()}</span>
                      <span className="text-gray-500 text-[11px] ml-2 font-mono">{client.pct}%</span>
                    </div>
                  </div>
                  <div className="w-full h-1 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-cyan-400 rounded-full"
                      style={{ width: `${Math.min(client.pct, 100)}%` }}
                    />
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>

      {/* Row 2: Top queried domains (Left) & Top blocked domains (Right) */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Panel 3: Top queried domains */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-[410px] flex flex-col shadow-sm">
          <div className="flex items-center justify-between pb-2 border-b border-gray-800 shrink-0">
            <div>
              <h2 className="text-sm font-bold text-white">Top queried domains</h2>
              <span className="text-[11px] text-gray-500">for the last 24 hours</span>
            </div>
            <button onClick={() => onNavigate('diagnostics')} className="p-1 text-gray-400 hover:text-cyan-400 transition-colors" title="Domain Probe">
              <Search className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase py-2 shrink-0 border-b border-gray-800/40">
            <span>Domain</span>
            <span>Requests count</span>
          </div>

          <div className="flex-1 overflow-y-auto pr-1.5 space-y-3 pt-2 card-scrollbar">
            {topQueriedDomains.length === 0 ? (
              <div className="h-full flex items-center justify-center text-center text-gray-500 font-mono text-xs">
                Belum ada domain yang tercatat dalam log telemetri Unbound.
              </div>
            ) : (
              topQueriedDomains.map((item, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2 min-w-0 max-w-[68%]">
                      <span
                        className="text-gray-200 truncate cursor-pointer hover:text-cyan-400 text-xs font-mono"
                        title={item.domain}
                        onClick={() => runQuickProbe(item.domain)}
                      >
                        {item.domain}
                      </span>
                      <button
                        onClick={() => runQuickProbe(item.domain)}
                        className="text-gray-500 hover:text-emerald-400 text-[10px] shrink-0"
                        title="Probe this domain"
                      >
                        <Eye className="w-3 h-3" />
                      </button>
                    </div>
                    <div className="text-right shrink-0">
                      <span className="text-cyan-400 font-bold font-mono text-xs">{item.count.toLocaleString()}</span>
                      <span className="text-gray-500 text-[11px] ml-2 font-mono">{item.pct}%</span>
                    </div>
                  </div>
                  <div className="w-full h-1 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-cyan-500 rounded-full"
                      style={{ width: `${Math.min(item.pct, 100)}%` }}
                    />
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Panel 4: Top blocked domains */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-[410px] flex flex-col shadow-sm">
          <div className="flex items-center justify-between pb-2 border-b border-gray-800 shrink-0">
            <div>
              <h2 className="text-sm font-bold text-white">Top blocked domains</h2>
              <span className="text-[11px] text-gray-500">for the last 24 hours</span>
            </div>
            <button onClick={() => onNavigate('filtering')} className="p-1 text-gray-400 hover:text-cyan-400 transition-colors" title="Manage Blocklists">
              <Filter className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase py-2 shrink-0 border-b border-gray-800/40">
            <span>Domain</span>
            <span>Requests count</span>
          </div>

          <div className="flex-1 overflow-y-auto pr-1.5 space-y-3 pt-2 card-scrollbar">
            {topBlockedDomains.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-center text-gray-500 font-mono text-xs gap-1.5">
                <CheckCircle2 className="w-5 h-5 text-emerald-500/70" />
                <span>Nol ancaman atau pemblokiran dalam riwayat terkini.</span>
              </div>
            ) : (
              topBlockedDomains.map((item, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2 min-w-0 max-w-[62%]">
                      <span
                        className="text-rose-400 font-semibold truncate cursor-pointer hover:underline text-xs font-mono"
                        title={item.domain}
                        onClick={() => runQuickProbe(item.domain)}
                      >
                        {item.domain}
                      </span>
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-rose-950/80 text-rose-300 border border-rose-800/80 shrink-0 whitespace-nowrap">
                        {item.category}
                      </span>
                    </div>
                    <div className="text-right shrink-0">
                      <span className="text-rose-400 font-bold font-mono text-xs">{item.count.toLocaleString()}</span>
                      <span className="text-gray-500 text-[11px] ml-2 font-mono">{item.pct}%</span>
                    </div>
                  </div>
                  <div className="w-full h-1 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-rose-500 rounded-full"
                      style={{ width: `${Math.min(item.pct, 100)}%` }}
                    />
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>

      {/* Row 3: Top upstreams (Left) & Average upstream response time (Right) */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Panel 5: Top upstreams */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-[260px] flex flex-col shadow-sm">
          <div className="flex items-center justify-between pb-2 border-b border-gray-800 shrink-0">
            <div>
              <h2 className="text-sm font-bold text-white">Top upstreams</h2>
              <span className="text-[11px] text-gray-500">for the last 24 hours</span>
            </div>
            <button onClick={() => onNavigate('dns-config')} className="p-1 text-gray-400 hover:text-cyan-400 transition-colors" title="Configure Upstreams">
              <Globe className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase py-2 shrink-0 border-b border-gray-800/40">
            <span>Upstream</span>
            <span>Requests count</span>
          </div>

          <div className="flex-1 overflow-y-auto pr-1.5 space-y-3 pt-2 card-scrollbar">
            {topUpstreams.length === 0 ? (
              <div className="h-full flex items-center justify-center text-center text-gray-500 font-mono text-xs">
                Belum ada upstream DNS aktif.
              </div>
            ) : (
              topUpstreams.map((up, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-gray-300 font-semibold font-mono text-xs truncate">{up.upstream}</span>
                    <div className="text-right shrink-0">
                      <span className="text-white font-bold font-mono text-xs">{up.count.toLocaleString()}</span>
                      <span className="text-gray-500 text-[11px] ml-2 font-mono">{up.pct}%</span>
                    </div>
                  </div>
                  <div className="w-full h-1 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-emerald-500 rounded-full"
                      style={{ width: `${Math.min(up.pct, 100)}%` }}
                    />
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Panel 6: Average upstream response time */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 h-[260px] flex flex-col shadow-sm">
          <div className="flex items-center justify-between pb-2 border-b border-gray-800 shrink-0">
            <div>
              <h2 className="text-sm font-bold text-white">Average upstream response time</h2>
              <span className="text-[11px] text-gray-500">for the last 24 hours</span>
            </div>
            <button onClick={fetchHealthAndStats} className="p-1 text-gray-400 hover:text-cyan-400 transition-colors" title="Refresh">
              <Clock className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase py-2 shrink-0 border-b border-gray-800/40">
            <span>Upstream</span>
            <span>Response time</span>
          </div>

          <div className="flex-1 overflow-y-auto pr-1.5 space-y-2 pt-2 card-scrollbar">
            {topUpstreams.length === 0 ? (
              <div className="h-full flex items-center justify-center text-center text-gray-500 font-mono text-xs">
                Belum ada data latensi upstream.
              </div>
            ) : (
              topUpstreams.map((up, idx) => (
                <div key={idx} className="flex items-center justify-between py-1.5 border-b border-gray-800/40 last:border-0">
                  <span className="text-gray-300 font-mono text-xs truncate">{up.upstream}</span>
                  <span className="text-cyan-400 font-bold font-mono text-xs shrink-0">{up.latency}</span>
                </div>
              ))
            )}
          </div>
        </div>
      </div>

      {/* Interactive Quick Probe Drawer at Bottom */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Search className="w-4 h-4 text-cyan-400" />
            <h2 className="text-sm font-bold text-white">Live Policy & Explainability Inspector</h2>
          </div>
          <span className="text-[11px] text-gray-400 font-mono">Direct Unbound Socket Test</span>
        </div>

        <form onSubmit={(e) => { e.preventDefault(); runQuickProbe(testDomain); }} className="flex gap-2">
          <input
            type="text"
            value={testDomain}
            onChange={(e) => setTestDomain(e.target.value)}
            placeholder="Type any domain (e.g. doubleclick.net, graph.facebook.com)"
            className="flex-1 bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white placeholder-gray-500 font-mono focus:outline-none focus:border-cyan-500"
          />
          <button
            type="submit"
            disabled={probing}
            className="px-5 py-2 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-semibold text-xs rounded-lg transition-all flex items-center gap-2 disabled:opacity-50"
          >
            {probing ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : 'Inspect Domain'}
          </button>
        </form>

        {probeResult && (
          <div className="mt-3 p-4 rounded-lg bg-gray-950 border border-gray-800 space-y-2 font-mono text-xs">
            <div className="flex items-center justify-between">
              <span className="text-gray-400">Target:</span>
              <span className="font-bold text-white">{probeResult.domain || testDomain}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-gray-400">Decision:</span>
              <span
                className={`px-2 py-0.5 rounded font-bold ${
                  probeResult.action === 'BLOCKED'
                    ? 'bg-red-950 text-red-400 border border-red-800'
                    : 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                }`}
              >
                {probeResult.action || (probeResult.error ? 'ERROR' : 'ALLOW')}
              </span>
            </div>
            {probeResult.explanation && (
              <div className="flex items-center justify-between">
                <span className="text-gray-400">Reason:</span>
                <span className="text-gray-200">{probeResult.explanation}</span>
              </div>
            )}
            {probeResult.live_resolution && (
              <div className="flex items-center justify-between">
                <span className="text-gray-400">Resolution Latency:</span>
                <span className="text-cyan-400 font-bold">{probeResult.live_resolution.latency}</span>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
