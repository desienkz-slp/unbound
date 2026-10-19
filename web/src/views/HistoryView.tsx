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
  Database,
  Download,
  Trash2,
  Plus,
  HardDrive,
  FileArchive,
} from 'lucide-react';
import { ConfigVersion, DatabaseBackupInfo } from '../types';
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

  // ── Database Disaster Recovery Snapshots ─────────────────────────────────
  const [dbBackups, setDbBackups] = useState<DatabaseBackupInfo[]>([]);
  const [loadingDb, setLoadingDb] = useState(false);
  const [creatingDb, setCreatingDb] = useState(false);
  const [restoringDb, setRestoringDb] = useState<string | null>(null);
  const [deletingDb, setDeletingDb] = useState<string | null>(null);
  const [dbDesc, setDbDesc] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);

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

  const fetchDbBackups = async () => {
    setLoadingDb(true);
    try {
      const list = await api.getDatabaseBackups();
      setDbBackups(list);
    } catch (err) {
      console.error('Failed to load database backups:', err);
    } finally {
      setLoadingDb(false);
    }
  };

  useEffect(() => {
    fetchVersions();
    fetchDbBackups();
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

  const handleCreateDbBackup = async () => {
    setCreatingDb(true);
    setMsg(null);
    try {
      const res = await api.createDatabaseBackup(dbDesc || undefined);
      setMsg({ type: 'success', text: `Snapshot created: ${res.backup.filename}` });
      setDbDesc('');
      setShowCreateModal(false);
      await fetchDbBackups();
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to create database snapshot' });
    } finally {
      setCreatingDb(false);
    }
  };

  const handleRestoreDbBackup = async (filename: string) => {
    if (!window.confirm(`PERINGATAN DISASTER RECOVERY:\nApakah Anda yakin ingin me-restore database dari snapshot "${filename}"?\n\nTindakan ini akan menimpa seluruh tabel konfigurasi kontrol database dengan data dari snapshot.`)) {
      return;
    }
    setRestoringDb(filename);
    setMsg(null);
    try {
      const res = await api.restoreDatabaseBackup(filename);
      setMsg({ type: 'success', text: res.message || 'Database snapshot restored successfully!' });
      await fetchVersions();
      await fetchDbBackups();
      onRollbackComplete();
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to restore database snapshot' });
    } finally {
      setRestoringDb(null);
    }
  };

  const handleDeleteDbBackup = async (filename: string) => {
    if (!window.confirm(`Hapus file snapshot database "${filename}"?`)) {
      return;
    }
    setDeletingDb(filename);
    setMsg(null);
    try {
      await api.deleteDatabaseBackup(filename);
      setMsg({ type: 'success', text: `Snapshot "${filename}" telah dihapus.` });
      await fetchDbBackups();
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to delete snapshot' });
    } finally {
      setDeletingDb(null);
    }
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
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

      {/* ── Database Disaster Recovery Snapshots ────────────────────────────── */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 pb-3 border-b border-gray-800">
          <div>
            <div className="flex items-center gap-2">
              <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-violet-500/20 text-violet-400 border border-violet-500/30">
                POSTGRESQL RECOVERY
              </span>
              <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-cyan-500/20 text-cyan-400 border border-cyan-500/30">
                7-DAY RETENTION
              </span>
            </div>
            <h2 className="text-base font-bold text-white flex items-center gap-2 mt-1">
              <Database className="w-5 h-5 text-cyan-400" />
              Database Disaster Recovery &amp; Automated Snapshots
            </h2>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={fetchDbBackups}
              disabled={loadingDb}
              className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-medium rounded-lg border border-gray-700 flex items-center gap-1.5 transition-colors"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loadingDb ? 'animate-spin text-cyan-400' : ''}`} />
              <span>Refresh</span>
            </button>
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-3 py-1.5 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black text-xs font-bold rounded-lg flex items-center gap-1.5 transition-all shadow-md shadow-cyan-500/20"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>Buat Snapshot Baru</span>
            </button>
          </div>
        </div>

        {/* Snapshot Table */}
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs font-mono">
            <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
              <tr>
                <th className="py-2.5 px-3">Snapshot File</th>
                <th className="py-2.5 px-3">Size</th>
                <th className="py-2.5 px-3">Format</th>
                <th className="py-2.5 px-3">Deskripsi</th>
                <th className="py-2.5 px-3">Waktu Pembuatan</th>
                <th className="py-2.5 px-3 text-right">Aksi</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800/60">
              {dbBackups.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-8 text-center text-gray-500">
                    {loadingDb ? 'Memuat snapshot database...' : 'Belum ada snapshot database. Klik "Buat Snapshot Baru" untuk mencadangkan database.'}
                  </td>
                </tr>
              ) : (
                dbBackups.map((b) => (
                  <tr key={b.filename} className="hover:bg-gray-800/30">
                    <td className="py-3 px-3 font-bold text-white flex items-center gap-2">
                      <FileArchive className="w-4 h-4 text-cyan-400 shrink-0" />
                      <span className="text-cyan-300 font-mono text-[11px]">{b.filename}</span>
                    </td>
                    <td className="py-3 px-3 text-gray-300 font-mono">
                      {formatBytes(b.size)}
                    </td>
                    <td className="py-3 px-3">
                      <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-violet-950 text-violet-300 border border-violet-800">
                        {b.format || 'pg_dump -Fc'}
                      </span>
                    </td>
                    <td className="py-3 px-3 text-gray-300 max-w-xs truncate">
                      {b.description || 'Manual backup'}
                    </td>
                    <td className="py-3 px-3 text-gray-500 text-[11px]">
                      {new Date(b.created_at).toLocaleString()}
                    </td>
                    <td className="py-3 px-3 text-right">
                      <div className="flex items-center justify-end gap-1.5">
                        {/* Download */}
                        <a
                          href={api.getDatabaseBackupDownloadUrl(b.filename)}
                          download={b.filename}
                          target="_blank"
                          rel="noreferrer"
                          className="px-2 py-1 bg-gray-800 hover:bg-gray-700 text-cyan-400 text-[11px] rounded border border-gray-700 flex items-center gap-1 transition-colors"
                          title="Unduh snapshot database ke komputer lokal"
                        >
                          <Download className="w-3 h-3" />
                          <span>Unduh</span>
                        </a>

                        {/* Restore */}
                        <button
                          onClick={() => handleRestoreDbBackup(b.filename)}
                          disabled={restoringDb !== null}
                          className="px-2 py-1 bg-gray-800 hover:bg-amber-900/60 text-amber-300 text-[11px] rounded border border-gray-700 flex items-center gap-1 transition-colors disabled:opacity-30"
                          title="Restore database kontrol dari snapshot ini"
                        >
                          {restoringDb === b.filename ? (
                            <RefreshCw className="w-3 h-3 animate-spin" />
                          ) : (
                            <RotateCcw className="w-3 h-3" />
                          )}
                          <span>Restore</span>
                        </button>

                        {/* Delete */}
                        <button
                          onClick={() => handleDeleteDbBackup(b.filename)}
                          disabled={deletingDb !== null}
                          className="p-1 text-gray-500 hover:text-red-400 hover:bg-red-950/40 rounded transition-colors disabled:opacity-30"
                          title="Hapus file snapshot"
                        >
                          {deletingDb === b.filename ? (
                            <RefreshCw className="w-3 h-3 animate-spin" />
                          ) : (
                            <Trash2 className="w-3.5 h-3.5" />
                          )}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Informational Footer */}
        <div className="bg-gray-950/60 border border-gray-800 rounded-lg p-3 text-[11px] text-gray-400 flex items-start gap-2.5">
          <HardDrive className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
          <span>
            <strong>Informasi Penyimpanan:</strong> Snapshot database disimpan di direktori aman <code className="text-emerald-300">/var/backups/netora/db/</code> pada host server. Snapshot yang berusia lebih dari 7 hari akan dibersihkan secara otomatis untuk menghemat ruang disk host.
          </span>
        </div>
      </div>

      {/* Modal Buat Snapshot */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400">
                <Database className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-base font-bold text-white" title="Menjalankan pg_dump -Fc secara real-time.">Buat Snapshot PostgreSQL Baru</h3>
              </div>
            </div>

            <div className="space-y-2">
              <label className="block text-xs font-medium text-gray-300">
                Keterangan / Deskripsi Snapshot (Opsional)
              </label>
              <input
                type="text"
                value={dbDesc}
                onChange={(e) => setDbDesc(e.target.value)}
                placeholder="Contoh: Pra-upgrade maintenance v1.0.87"
                className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-cyan-500"
              />
            </div>

            <div className="flex items-center justify-end gap-2 pt-2 border-t border-gray-800">
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="px-3.5 py-1.5 text-xs text-gray-400 hover:text-white rounded-lg transition-colors"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleCreateDbBackup}
                disabled={creatingDb}
                className="px-4 py-1.5 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black text-xs font-bold rounded-lg flex items-center gap-1.5 transition-all shadow-md shadow-cyan-500/20 disabled:opacity-50"
              >
                {creatingDb ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Database className="w-3.5 h-3.5" />}
                <span>{creatingDb ? 'Mencadangkan...' : 'Proses Snapshot'}</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
