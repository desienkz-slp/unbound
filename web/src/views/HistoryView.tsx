import React, { useState, useEffect } from 'react';
import {
  AlertTriangle,
  ArrowLeftRight,
  CheckCircle2,
  Clock,
  History,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
} from 'lucide-react';
import { ConfigVersion } from '../types';
import { api } from '../services/api';

interface HistoryViewProps {
  nodeId: string;
  onRollbackComplete: () => void;
}

export const HistoryView: React.FC<HistoryViewProps> = ({ nodeId, onRollbackComplete }) => {
  const [versions, setVersions] = useState<ConfigVersion[]>([]);
  const [loading, setLoading] = useState(false);
  const [rollingBack, setRollingBack] = useState<string | null>(null);
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const fetchVersions = async () => {
    setLoading(true);
    try {
      const list = await api.getConfigVersions(nodeId);
      setVersions(list);
    } catch (err) {
      console.error('Failed to load config versions:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchVersions();
  }, [nodeId]);

  const handleRollback = async (versionId: string) => {
    if (!window.confirm(`Are you sure you want to rollback to configuration version ${versionId}? This will atomically restore and reload Unbound.`)) {
      return;
    }
    setRollingBack(versionId);
    setMsg(null);
    try {
      const res = await api.rollbackConfig(nodeId, versionId);
      setMsg({ type: 'success', text: `Rollback successful! Restored to version: ${res.rolled_back_to}` });
      fetchVersions();
      onRollbackComplete();
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Rollback failed' });
    } finally {
      setRollingBack(null);
    }
  };

  return (
    <div className="space-y-6">
      {/* Title */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <History className="w-5 h-5 text-cyan-400" />
            Configuration Versions & Atomic Rollback
          </h1>
          <p className="text-xs text-gray-400 mt-1">
            Every configuration change is versioned with SHA256 checksums and can be atomically restored in &lt;150ms.
          </p>
        </div>

        <button
          onClick={fetchVersions}
          disabled={loading}
          className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-200 text-xs font-medium rounded-lg border border-gray-700 flex items-center gap-2"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-cyan-400' : ''}`} />
          Refresh History
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
          {msg.type === 'success' ? (
            <CheckCircle2 className="w-4 h-4 shrink-0" />
          ) : (
            <AlertTriangle className="w-4 h-4 shrink-0" />
          )}
          <span>{msg.text}</span>
        </div>
      )}

      {/* Version Table */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs font-mono">
            <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
              <tr>
                <th className="py-2.5 px-3">Version ID</th>
                <th className="py-2.5 px-3">Status</th>
                <th className="py-2.5 px-3">Checksum</th>
                <th className="py-2.5 px-3">Actor / Author</th>
                <th className="py-2.5 px-3">Reason / Description</th>
                <th className="py-2.5 px-3">Applied At</th>
                <th className="py-2.5 px-3 text-right">Rollback</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800/60">
              {versions.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-8 text-center text-gray-500">
                    {loading ? 'Loading version history...' : 'No historical versions found.'}
                  </td>
                </tr>
              ) : (
                versions.map((ver, idx) => (
                  <tr key={ver.version_id} className="hover:bg-gray-800/30">
                    <td className="py-3 px-3 font-bold text-white flex items-center gap-2">
                      <span className="text-cyan-400">{ver.version_id}</span>
                      {idx === 0 && (
                        <span className="text-[9px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-300 border border-cyan-800">
                          CURRENT
                        </span>
                      )}
                    </td>
                    <td className="py-3 px-3">
                      <span
                        className={`px-2 py-0.5 rounded font-bold text-[10px] ${
                          ver.status === 'APPLIED' || ver.status === 'ACTIVE'
                            ? 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                            : 'bg-gray-800 text-gray-400'
                        }`}
                      >
                        {ver.status}
                      </span>
                    </td>
                    <td className="py-3 px-3 text-gray-400">
                      {ver.checksum ? ver.checksum.slice(0, 10) + '...' : '-'}
                    </td>
                    <td className="py-3 px-3 text-gray-300">{ver.actor || 'admin'}</td>
                    <td className="py-3 px-3 text-gray-300 max-w-xs truncate">{ver.reason}</td>
                    <td className="py-3 px-3 text-gray-500">
                      {ver.applied_at ? new Date(ver.applied_at).toLocaleString() : new Date(ver.created_at).toLocaleString()}
                    </td>
                    <td className="py-3 px-3 text-right">
                      <button
                        onClick={() => handleRollback(ver.version_id)}
                        disabled={idx === 0 || rollingBack !== null}
                        className="px-2.5 py-1 bg-gray-800 hover:bg-amber-900/60 hover:text-amber-300 text-gray-300 text-[11px] font-semibold rounded border border-gray-700 transition-colors flex items-center gap-1.5 ml-auto disabled:opacity-30 disabled:cursor-not-allowed"
                        title={idx === 0 ? 'Current active version' : 'Rollback to this version'}
                      >
                        {rollingBack === ver.version_id ? (
                          <RefreshCw className="w-3 h-3 animate-spin" />
                        ) : (
                          <RotateCcw className="w-3 h-3 text-amber-400" />
                        )}
                        Rollback
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
