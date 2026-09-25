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
import { DesiredState, NodeRecord } from '../types';
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

  // AdGuard Home inspired statistics state
  const [stats, setStats] = useState({
    totalQueries: 14391677,
    blockedFilters: 6187788,
    blockedMalware: 350,
    blockedAdult: 0,
    enforcedSafeSearch: 265325,
    avgProcessingTime: 30, // ms
  });

  const topClients = [
    { ip: '172.18.20.30', name: 'Core Router / NAT', count: 854858, pct: 5.94 },
    { ip: '172.18.20.1', name: 'Gateway / NOC', count: 852073, pct: 5.92 },
    { ip: '10.22.31.234', name: 'DHCP Pool A', count: 166234, pct: 1.16 },
    { ip: '10.22.30.161', name: 'PPPoE Client BRAS', count: 86154, pct: 0.60 },
    { ip: '10.22.31.31', name: 'OLT Management', count: 77094, pct: 0.54 },
  ];

  const topQueriedDomains = [
    { domain: 'graph.facebook.com', count: 230017, pct: 1.60, blocked: false },
    { domain: 'api.whatsapp.net', count: 122034, pct: 0.85, blocked: false },
    { domain: 'g.whatsapp.net', count: 110114, pct: 0.77, blocked: false },
    { domain: 'play.googleapis.com', count: 93072, pct: 0.65, blocked: false },
    { domain: 'test-gateway.instagram.com', count: 89830, pct: 0.62, blocked: false },
  ];

  const topBlockedDomains = [
    { domain: 'thirdplat.189cube.com', count: 1853256, pct: 28.72, category: 'Malware/Tracker' },
    { domain: 'nosudp7.189cube.com', count: 810396, pct: 12.56, category: 'Ad/Telemetry' },
    { domain: 'tr069-avs.firstmedia.com', count: 276757, pct: 4.29, category: 'Unused TR069' },
    { domain: 'mediation-sg2-log.pangle.io', count: 253366, pct: 3.93, category: 'Mobile Ads' },
    { domain: 'doubleclick.net', count: 242765, pct: 3.76, category: 'Advertising' },
  ];

  const topUpstreams = [
    { upstream: 'https://1.0.0.1:443/dns-query', count: 56562, pct: 40.9, latency: '258 ms' },
    { upstream: 'https://1.1.1.1:443/dns-query', count: 53999, pct: 39.04, latency: '255 ms' },
    { upstream: 'https://8.8.4.4:443/dns-query', count: 13983, pct: 10.11, latency: '167 ms' },
    { upstream: 'https://8.8.8.8:443/dns-query', count: 13760, pct: 9.95, latency: '163 ms' },
  ];

  const fetchHealth = async () => {
    if (!nodeId) return;
    setRefreshing(true);
    try {
      const res = await api.getNodeHealth(nodeId);
      setHealth(res);
    } catch (err) {
      console.error('Failed to fetch node health:', err);
    } finally {
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchHealth();
    const interval = setInterval(fetchHealth, 10000);
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

  const blockedPct = ((stats.blockedFilters / stats.totalQueries) * 100).toFixed(0);

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
          </div>
          <p className="text-xs text-gray-400 mt-1">
            Real-time ISP NOC telemetry & query distribution for node <strong className="text-cyan-400 font-mono">{node?.hostname || 'dns-dev'}</strong>
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={fetchHealth}
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

      {/* 4 Top KPI Cards (AdGuard Home Style) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Card 1: DNS Queries */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 flex flex-col justify-between relative overflow-hidden group hover:border-gray-700 transition-all">
          <div>
            <span className="text-2xl lg:text-3xl font-bold font-mono text-white tracking-tight">
              {stats.totalQueries.toLocaleString()}
            </span>
            <div className="text-xs font-medium text-gray-400 mt-1">DNS Queries</div>
          </div>
          {/* Subtle sparkline wave */}
          <div className="mt-4 pt-2">
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
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 flex flex-col justify-between relative overflow-hidden group hover:border-rose-900/50 transition-all">
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
          <div className="mt-4 pt-2">
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
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 flex flex-col justify-between relative overflow-hidden group hover:border-amber-900/50 transition-all">
          <div className="flex items-start justify-between">
            <div>
              <span className="text-2xl lg:text-3xl font-bold font-mono text-amber-400 tracking-tight">
                {stats.blockedMalware.toLocaleString()}
              </span>
              <div className="text-xs font-medium text-amber-300/80 mt-1">Blocked malware/phishing</div>
            </div>
            <span className="text-xs font-mono text-gray-500 px-2 py-0.5 rounded bg-gray-950">
              &lt;0.01%
            </span>
          </div>
          <div className="mt-4 pt-2">
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
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 flex flex-col justify-between relative overflow-hidden group hover:border-purple-900/50 transition-all">
          <div className="flex items-start justify-between">
            <div>
              <span className="text-2xl lg:text-3xl font-bold font-mono text-purple-400 tracking-tight">
                {stats.blockedAdult.toLocaleString()}
              </span>
              <div className="text-xs font-medium text-purple-300/80 mt-1">Blocked adult websites</div>
            </div>
            <span className="text-xs font-mono text-gray-500 px-2 py-0.5 rounded bg-gray-950">
              0%
            </span>
          </div>
          <div className="mt-4 pt-2">
            <div className="h-8 w-full flex items-end gap-1 opacity-70">
              {[2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2].map((h, i) => (
                <div
                  key={i}
                  className="flex-1 bg-purple-900/30 rounded-t"
                  style={{ height: `${h}%` }}
                />
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Main 2-Column Analytical Grid (Matching AdGuard Home Layout) */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left Column: General stats + Top queried domains + Top upstreams */}
        <div className="space-y-6">
          {/* Panel 1: General statistics for the last 24 hours */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-gray-800">
              <div>
                <h2 className="text-sm font-bold text-white">General statistics</h2>
                <span className="text-[11px] text-gray-500">for the last 24 hours</span>
              </div>
              <button onClick={fetchHealth} className="p-1 text-gray-400 hover:text-cyan-400">
                <RefreshCw className="w-3.5 h-3.5" />
              </button>
            </div>

            <div className="divide-y divide-gray-800/60 text-xs font-mono">
              <div className="py-2.5 flex items-center justify-between">
                <span className="text-gray-300">DNS Queries</span>
                <span className="text-cyan-400 font-bold">{stats.totalQueries.toLocaleString()}</span>
              </div>

              <div className="py-2.5 flex items-center justify-between">
                <span className="text-rose-400">Blocked by Filters</span>
                <span className="text-rose-400 font-bold">{stats.blockedFilters.toLocaleString()}</span>
              </div>

              <div className="py-2.5 flex items-center justify-between">
                <span className="text-amber-400">Blocked malware/phishing</span>
                <span className="text-amber-400 font-bold">{stats.blockedMalware}</span>
              </div>

              <div className="py-2.5 flex items-center justify-between">
                <span className="text-purple-400">Blocked adult websites</span>
                <span className="text-purple-400 font-bold">{stats.blockedAdult}</span>
              </div>

              <div className="py-2.5 flex items-center justify-between">
                <span className="text-emerald-400">Enforced safe search</span>
                <span className="text-emerald-400 font-bold">{stats.enforcedSafeSearch.toLocaleString()}</span>
              </div>

              <div className="py-2.5 flex items-center justify-between">
                <span className="text-gray-300">Average processing time</span>
                <span className="text-white font-bold">{stats.avgProcessingTime} ms</span>
              </div>
            </div>
          </div>

          {/* Panel 2: Top queried domains */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-gray-800">
              <div>
                <h2 className="text-sm font-bold text-white">Top queried domains</h2>
                <span className="text-[11px] text-gray-500">for the last 24 hours</span>
              </div>
              <button onClick={() => onNavigate('diagnostics')} className="p-1 text-gray-400 hover:text-cyan-400" title="Domain Probe">
                <Search className="w-3.5 h-3.5" />
              </button>
            </div>

            <div className="space-y-3 text-xs font-mono pt-1">
              <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase">
                <span>Domain</span>
                <span>Requests count</span>
              </div>

              {topQueriedDomains.map((item, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 max-w-[70%]">
                      <span className="text-gray-200 truncate cursor-pointer hover:text-cyan-400" onClick={() => runQuickProbe(item.domain)}>
                        {item.domain}
                      </span>
                      <button
                        onClick={() => runQuickProbe(item.domain)}
                        className="text-gray-500 hover:text-emerald-400 text-[10px]"
                        title="Probe this domain"
                      >
                        <Eye className="w-3 h-3" />
                      </button>
                    </div>
                    <div className="text-right">
                      <span className="text-cyan-400 font-bold">{item.count.toLocaleString()}</span>
                      <span className="text-gray-500 text-[11px] ml-2">{item.pct}%</span>
                    </div>
                  </div>
                  {/* Progress Bar */}
                  <div className="w-full h-1 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-cyan-500 rounded-full"
                      style={{ width: `${Math.min(item.pct * 25, 100)}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Panel 3: Top upstreams */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-gray-800">
              <div>
                <h2 className="text-sm font-bold text-white">Top upstreams</h2>
                <span className="text-[11px] text-gray-500">for the last 24 hours</span>
              </div>
              <button onClick={() => onNavigate('dns-config')} className="p-1 text-gray-400 hover:text-cyan-400" title="Configure Upstreams">
                <Globe className="w-3.5 h-3.5" />
              </button>
            </div>

            <div className="space-y-3 text-xs font-mono pt-1">
              <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase">
                <span>Upstream</span>
                <span>Requests count</span>
              </div>

              {topUpstreams.map((up, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center justify-between">
                    <span className="text-gray-300 font-semibold">{up.upstream}</span>
                    <div>
                      <span className="text-white font-bold">{up.count.toLocaleString()}</span>
                      <span className="text-gray-500 text-[11px] ml-2">{up.pct}%</span>
                    </div>
                  </div>
                  <div className="w-full h-1 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-emerald-500 rounded-full"
                      style={{ width: `${up.pct}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Right Column: Top clients + Top blocked domains + Upstream latency */}
        <div className="space-y-6">
          {/* Panel 4: Top clients for the last 24 hours */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-gray-800">
              <div>
                <h2 className="text-sm font-bold text-white">Top clients</h2>
                <span className="text-[11px] text-gray-500">for the last 24 hours</span>
              </div>
              <button onClick={() => onNavigate('dns-config')} className="p-1 text-gray-400 hover:text-cyan-400" title="Client ACLs">
                <Layers className="w-3.5 h-3.5" />
              </button>
            </div>

            <div className="space-y-3 text-xs font-mono pt-1">
              <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase">
                <span>Client</span>
                <span>Requests count</span>
              </div>

              {topClients.map((client, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="text-cyan-400 font-bold">{client.ip}</span>
                      <span className="text-gray-500 text-[11px] hidden sm:inline">({client.name})</span>
                    </div>
                    <div>
                      <span className="text-white font-bold">{client.count.toLocaleString()}</span>
                      <span className="text-gray-500 text-[11px] ml-2">{client.pct}%</span>
                    </div>
                  </div>
                  <div className="w-full h-1 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-cyan-400 rounded-full"
                      style={{ width: `${client.pct * 12}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Panel 5: Top blocked domains */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-gray-800">
              <div>
                <h2 className="text-sm font-bold text-white">Top blocked domains</h2>
                <span className="text-[11px] text-gray-500">for the last 24 hours</span>
              </div>
              <button onClick={() => onNavigate('filtering')} className="p-1 text-gray-400 hover:text-cyan-400" title="Manage Blocklists">
                <Filter className="w-3.5 h-3.5" />
              </button>
            </div>

            <div className="space-y-3 text-xs font-mono pt-1">
              <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase">
                <span>Domain</span>
                <span>Requests count</span>
              </div>

              {topBlockedDomains.map((item, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 max-w-[70%]">
                      <span className="text-rose-400 font-semibold truncate cursor-pointer hover:underline" onClick={() => runQuickProbe(item.domain)}>
                        {item.domain}
                      </span>
                      <span className="text-[9px] px-1.5 py-0.2 rounded bg-rose-950 text-rose-300 border border-rose-800">
                        {item.category}
                      </span>
                    </div>
                    <div>
                      <span className="text-rose-400 font-bold">{item.count.toLocaleString()}</span>
                      <span className="text-gray-500 text-[11px] ml-2">{item.pct}%</span>
                    </div>
                  </div>
                  <div className="w-full h-1 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-rose-500 rounded-full"
                      style={{ width: `${item.pct * 2}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Panel 6: Average upstream response time */}
          <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-gray-800">
              <div>
                <h2 className="text-sm font-bold text-white">Average upstream response time</h2>
                <span className="text-[11px] text-gray-500">for the last 24 hours</span>
              </div>
              <button onClick={fetchHealth} className="p-1 text-gray-400 hover:text-cyan-400">
                <Clock className="w-3.5 h-3.5" />
              </button>
            </div>

            <div className="space-y-3 text-xs font-mono pt-1">
              <div className="flex justify-between text-[11px] text-gray-500 font-sans font-semibold uppercase">
                <span>Upstream</span>
                <span>Response time</span>
              </div>

              {topUpstreams.map((up, idx) => (
                <div key={idx} className="flex items-center justify-between py-1 border-b border-gray-800/40 last:border-0">
                  <span className="text-gray-300">{up.upstream}</span>
                  <span className="text-cyan-400 font-bold font-mono">{up.latency}</span>
                </div>
              ))}
            </div>
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
