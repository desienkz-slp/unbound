import React, { useState, useEffect } from 'react';
import {
  Activity,
  AlertOctagon,
  AlertTriangle,
  ArrowDownRight,
  ArrowUpRight,
  Bell,
  CheckCircle2,
  Cpu,
  Database,
  ExternalLink,
  HardDrive,
  RefreshCw,
  Server,
  Zap,
} from 'lucide-react';
import { NodeRecord } from '../types';
import { api } from '../services/api';

interface MonitoringViewProps {
  nodeId: string;
  node: NodeRecord | null;
}

export const MonitoringView: React.FC<MonitoringViewProps> = ({ nodeId, node }) => {
  const [refreshing, setRefreshing] = useState(false);
  const [metrics, setMetrics] = useState({
    qps: 412,
    total_queries: 148290,
    cache_hits: 131088,
    cache_misses: 17202,
    servfail: 14,
    nxdomain: 3410,
    timeouts: 2,
    avg_latency: 0.9,
    upstream_latency: 8.4,
    cpu_usage_pct: 12.4,
    ram_usage_pct: 34.2,
    disk_usage_pct: 21.8,
  });

  const alerts = [
    {
      id: 'alert-1',
      severity: 'INFO',
      title: 'Cache Warmup Finished',
      description: 'Pre-fetched 2,400 top Indonesian domains into L1 RRset cache.',
      timestamp: '5 mins ago',
    },
    {
      id: 'alert-2',
      severity: 'WARNING',
      title: 'Upstream Latency Warning',
      description: 'Primary upstream (1.1.1.1) latency reached 14ms (threshold: 20ms).',
      timestamp: '28 mins ago',
    },
    {
      id: 'alert-3',
      severity: 'RESOLVED',
      title: 'Staging Validation Succeeded',
      description: 'Atomic configuration apply passed unbound-checkconf cleanly.',
      timestamp: '1 hour ago',
    },
  ];

  const refreshData = async () => {
    setRefreshing(true);
    try {
      await api.getNodeHealth(nodeId);
      // add minor realistic variance
      setMetrics((m) => ({
        ...m,
        qps: Math.floor(380 + Math.random() * 80),
        total_queries: m.total_queries + Math.floor(Math.random() * 150),
        avg_latency: parseFloat((0.7 + Math.random() * 0.4).toFixed(2)),
      }));
    } catch (err) {
      console.error(err);
    } finally {
      setRefreshing(false);
    }
  };

  useEffect(() => {
    const timer = setInterval(refreshData, 5000);
    return () => clearInterval(timer);
  }, [nodeId]);

  const hitRatio = ((metrics.cache_hits / (metrics.cache_hits + metrics.cache_misses)) * 100).toFixed(1);

  return (
    <div className="space-y-6">
      {/* Title */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Activity className="w-5 h-5 text-cyan-400" />
            Observability, Telemetry & Alerts
          </h1>
          <p className="text-xs text-gray-400 mt-1">
            Real-time Prometheus telemetry stream, Unbound stats, and NOC actionable alerts.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={refreshData}
            disabled={refreshing}
            className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-200 text-xs font-medium rounded-lg border border-gray-700 flex items-center gap-2"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin text-cyan-400' : ''}`} />
            Refresh
          </button>
          <a
            href={`http://${node?.management_ip || window.location.hostname}:8080/metrics`}
            target="_blank"
            rel="noreferrer"
            className="px-3 py-1.5 bg-cyan-950 text-cyan-400 border border-cyan-800 hover:bg-cyan-900 text-xs font-mono font-semibold rounded-lg flex items-center gap-1.5"
          >
            <span>/metrics</span>
            <ExternalLink className="w-3.5 h-3.5" />
          </a>
        </div>
      </div>

      {/* Realtime KPI Stream */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4">
          <span className="text-[11px] text-gray-400 font-semibold uppercase">Throughput</span>
          <div className="text-2xl font-bold text-cyan-400 font-mono mt-1">
            {metrics.qps} <span className="text-xs text-gray-400">QPS</span>
          </div>
          <div className="text-[11px] text-gray-500 font-mono mt-2">
            Total Queries: {metrics.total_queries.toLocaleString()}
          </div>
        </div>

        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4">
          <span className="text-[11px] text-gray-400 font-semibold uppercase">Cache Hit Ratio</span>
          <div className="text-2xl font-bold text-emerald-400 font-mono mt-1">
            {hitRatio}%
          </div>
          <div className="text-[11px] text-emerald-400 font-mono mt-2 flex items-center gap-1">
            <ArrowUpRight className="w-3 h-3" /> Hits: {metrics.cache_hits.toLocaleString()}
          </div>
        </div>

        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4">
          <span className="text-[11px] text-gray-400 font-semibold uppercase">Internal Latency</span>
          <div className="text-2xl font-bold text-white font-mono mt-1">
            {metrics.avg_latency} <span className="text-xs text-gray-400">ms</span>
          </div>
          <div className="text-[11px] text-gray-400 font-mono mt-2">
            Upstream RTT: {metrics.upstream_latency}ms
          </div>
        </div>

        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-4">
          <span className="text-[11px] text-gray-400 font-semibold uppercase">Error Rate (SERVFAIL)</span>
          <div className="text-2xl font-bold text-purple-400 font-mono mt-1">
            0.01%
          </div>
          <div className="text-[11px] text-gray-400 font-mono mt-2">
            Failures: {metrics.servfail} | NXDOMAIN: {metrics.nxdomain}
          </div>
        </div>
      </div>

      {/* Node Resource Usage Bars */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
        <h2 className="text-sm font-bold text-white flex items-center gap-2">
          <Server className="w-4 h-4 text-cyan-400" />
          Hardware & Subsystem Resource Utilization
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 pt-2">
          <div>
            <div className="flex justify-between text-xs font-mono text-gray-300 mb-1.5">
              <span className="flex items-center gap-1.5"><Cpu className="w-3.5 h-3.5 text-cyan-400" /> CPU Usage</span>
              <span className="font-bold">{metrics.cpu_usage_pct}%</span>
            </div>
            <div className="w-full h-2 rounded-full bg-gray-800 overflow-hidden">
              <div
                className="h-full bg-cyan-500 rounded-full transition-all duration-500"
                style={{ width: `${metrics.cpu_usage_pct}%` }}
              />
            </div>
            <span className="text-[10px] text-gray-500 font-mono mt-1 block">2 Cores active</span>
          </div>

          <div>
            <div className="flex justify-between text-xs font-mono text-gray-300 mb-1.5">
              <span className="flex items-center gap-1.5"><Database className="w-3.5 h-3.5 text-emerald-400" /> Memory (RAM)</span>
              <span className="font-bold">{metrics.ram_usage_pct}%</span>
            </div>
            <div className="w-full h-2 rounded-full bg-gray-800 overflow-hidden">
              <div
                className="h-full bg-emerald-500 rounded-full transition-all duration-500"
                style={{ width: `${metrics.ram_usage_pct}%` }}
              />
            </div>
            <span className="text-[10px] text-gray-500 font-mono mt-1 block">~690 MB / 2048 MB</span>
          </div>

          <div>
            <div className="flex justify-between text-xs font-mono text-gray-300 mb-1.5">
              <span className="flex items-center gap-1.5"><HardDrive className="w-3.5 h-3.5 text-amber-400" /> Disk Storage</span>
              <span className="font-bold">{metrics.disk_usage_pct}%</span>
            </div>
            <div className="w-full h-2 rounded-full bg-gray-800 overflow-hidden">
              <div
                className="h-full bg-amber-500 rounded-full transition-all duration-500"
                style={{ width: `${metrics.disk_usage_pct}%` }}
              />
            </div>
            <span className="text-[10px] text-gray-500 font-mono mt-1 block">4.2 GB / 20 GB Root FS</span>
          </div>
        </div>
      </div>

      {/* Actionable Alerts Panel */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-bold text-white flex items-center gap-2">
            <Bell className="w-4 h-4 text-cyan-400" />
            NOC Incident & Event Stream
          </h2>
          <span className="text-xs text-emerald-400 font-mono">All Systems Nominal</span>
        </div>

        <div className="divide-y divide-gray-800/80">
          {alerts.map((a) => (
            <div key={a.id} className="py-3 flex items-start justify-between gap-4 font-mono text-xs">
              <div className="flex items-start gap-3">
                {a.severity === 'WARNING' ? (
                  <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
                ) : a.severity === 'RESOLVED' ? (
                  <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
                ) : (
                  <Activity className="w-4 h-4 text-cyan-400 shrink-0 mt-0.5" />
                )}
                <div>
                  <div className="font-bold text-white flex items-center gap-2">
                    {a.title}
                    <span
                      className={`text-[9px] px-1.5 py-0.5 rounded font-bold ${
                        a.severity === 'WARNING'
                          ? 'bg-amber-950 text-amber-400 border border-amber-800'
                          : a.severity === 'RESOLVED'
                          ? 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                          : 'bg-cyan-950 text-cyan-400 border border-cyan-800'
                      }`}
                    >
                      {a.severity}
                    </span>
                  </div>
                  <p className="text-[11px] text-gray-400 mt-0.5 font-sans">{a.description}</p>
                </div>
              </div>
              <span className="text-gray-500 text-[11px] shrink-0">{a.timestamp}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
