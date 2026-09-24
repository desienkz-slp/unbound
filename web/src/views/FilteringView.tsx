import React, { useState, useEffect } from 'react';
import {
  CheckCircle2,
  Filter,
  Globe,
  Plus,
  RefreshCw,
  Search,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Trash2,
} from 'lucide-react';
import { AllowlistRecord, DesiredState } from '../types';
import { api } from '../services/api';

interface FilteringViewProps {
  config: DesiredState | null;
  nodeId: string;
}

export const FilteringView: React.FC<FilteringViewProps> = ({ config, nodeId }) => {
  const [allowlist, setAllowlist] = useState<AllowlistRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchFilter, setSearchFilter] = useState('');

  // Add modal state
  const [showAddModal, setShowAddModal] = useState(false);
  const [newDomain, setNewDomain] = useState('');
  const [newCategory, setNewCategory] = useState('OPERATIONAL_EXCEPTION');
  const [newReason, setNewReason] = useState('');
  const [actionMsg, setActionMsg] = useState<string | null>(null);

  const fetchAllowlist = async () => {
    setLoading(true);
    try {
      const records = await api.getAllowlist();
      setAllowlist(records);
    } catch (err) {
      console.error('Failed to load allowlist:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAllowlist();
  }, []);

  const handleAddAllowlist = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain.trim()) return;
    try {
      await api.addAllowlist(newDomain.trim(), newCategory, newReason || 'NOC operator exception');
      setActionMsg(`Added ${newDomain} to allowlist`);
      setShowAddModal(false);
      setNewDomain('');
      setNewReason('');
      fetchAllowlist();
    } catch (err: any) {
      setActionMsg(`Error: ${err.message}`);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteAllowlist(id);
      setActionMsg('Allowlist entry removed');
      fetchAllowlist();
    } catch (err: any) {
      setActionMsg(`Error removing entry: ${err.message}`);
    }
  };

  const filteredAllowlist = allowlist.filter(
    (a) =>
      a.domain.toLowerCase().includes(searchFilter.toLowerCase()) ||
      a.reason.toLowerCase().includes(searchFilter.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Title */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Filter className="w-5 h-5 text-cyan-400" />
            DNS Policy Filtering & Safety
          </h1>
          <p className="text-xs text-gray-400 mt-1">
            Manage high-performance Trie blocklists, SafeSearch redirects, and explicit allowlist exceptions.
          </p>
        </div>

        <button
          onClick={() => setShowAddModal(true)}
          className="px-4 py-2 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-emerald-500/20 flex items-center gap-2"
        >
          <Plus className="w-4 h-4" />
          Add Allowlist Exception
        </button>
      </div>

      {actionMsg && (
        <div className="p-3 rounded-lg bg-gray-900 border border-gray-800 text-xs text-cyan-300 font-mono flex items-center justify-between">
          <span>{actionMsg}</span>
          <button onClick={() => setActionMsg(null)} className="text-gray-500 hover:text-white">&times;</button>
        </div>
      )}

      {/* Filter Feeds Summary */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5">
          <div className="flex items-center justify-between text-gray-400">
            <span className="text-xs font-semibold uppercase tracking-wider">Active Threat Feeds</span>
            <ShieldAlert className="w-4 h-4 text-rose-400" />
          </div>
          <div className="text-2xl font-bold text-white font-mono mt-2">
            4 <span className="text-sm font-normal text-gray-400">Sources</span>
          </div>
          <p className="text-[11px] text-gray-400 mt-1 font-mono">
            Hagezi Multi-PRO, AdGuard DNS, Phishing, Malware
          </p>
          <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-cyan-400 font-mono">
            ~185,400 Trie-Indexed Rules
          </div>
        </div>

        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5">
          <div className="flex items-center justify-between text-gray-400">
            <span className="text-xs font-semibold uppercase tracking-wider">SafeSearch Status</span>
            <ShieldCheck className="w-4 h-4 text-cyan-400" />
          </div>
          <div className="text-2xl font-bold text-white font-mono mt-2">
            {config?.safe_search.enabled ? 'ACTIVE' : 'DISABLED'}
          </div>
          <p className="text-[11px] text-gray-400 mt-1 font-mono">
            Forces SafeSearch for Google, Bing, YouTube
          </p>
          <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-emerald-400 font-mono">
            216.239.38.120 (Google VIP)
          </div>
        </div>

        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5">
          <div className="flex items-center justify-between text-gray-400">
            <span className="text-xs font-semibold uppercase tracking-wider">Explicit Allowlist</span>
            <CheckCircle2 className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="text-2xl font-bold text-white font-mono mt-2">
            {allowlist.length} <span className="text-sm font-normal text-gray-400">Exceptions</span>
          </div>
          <p className="text-[11px] text-emerald-400 mt-1 font-mono">
            Priority 2: Overrides all external feeds
          </p>
          <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-gray-400 font-mono">
            Evaluated in &lt;50ns
          </div>
        </div>
      </div>

      {/* Allowlist Management Table */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div>
            <h2 className="text-sm font-bold text-white">Whitelisted Domain Rules</h2>
            <p className="text-xs text-gray-400">
              Domains in this list will never be sinkholed or blocked, ensuring zero false-positive disruption for ISP subscribers.
            </p>
          </div>

          <div className="relative w-full sm:w-64">
            <Search className="w-4 h-4 text-gray-500 absolute left-3 top-2.5" />
            <input
              type="text"
              placeholder="Search whitelisted domains..."
              value={searchFilter}
              onChange={(e) => setSearchFilter(e.target.value)}
              className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-9 pr-3 py-1.5 text-xs text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500 font-mono"
            />
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs font-mono">
            <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
              <tr>
                <th className="py-2.5 px-3">Domain</th>
                <th className="py-2.5 px-3">Category</th>
                <th className="py-2.5 px-3">Reason / Ticket</th>
                <th className="py-2.5 px-3">Created By</th>
                <th className="py-2.5 px-3">Created At</th>
                <th className="py-2.5 px-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800/60">
              {filteredAllowlist.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-8 text-center text-gray-500">
                    {loading ? 'Loading allowlist...' : 'No allowlist rules found.'}
                  </td>
                </tr>
              ) : (
                filteredAllowlist.map((rec) => (
                  <tr key={rec.id} className="hover:bg-gray-800/30">
                    <td className="py-2.5 px-3 font-bold text-emerald-400">{rec.domain}</td>
                    <td className="py-2.5 px-3">
                      <span className="px-2 py-0.5 rounded bg-gray-950 border border-gray-800 text-gray-300">
                        {rec.category}
                      </span>
                    </td>
                    <td className="py-2.5 px-3 text-gray-300">{rec.reason}</td>
                    <td className="py-2.5 px-3 text-gray-400">{rec.created_by}</td>
                    <td className="py-2.5 px-3 text-gray-500">
                      {new Date(rec.created_at).toLocaleString()}
                    </td>
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => handleDelete(rec.id)}
                        className="p-1 text-gray-500 hover:text-red-400 transition-colors"
                        title="Delete allowlist rule"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Add Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 max-w-md w-full space-y-4 shadow-2xl">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Plus className="w-5 h-5 text-emerald-400" />
              Add Allowlist Domain
            </h2>

            <form onSubmit={handleAddAllowlist} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Domain Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. portal.isp.net, bank-api.co.id"
                  value={newDomain}
                  onChange={(e) => setNewDomain(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-cyan-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Category</label>
                <select
                  value={newCategory}
                  onChange={(e) => setNewCategory(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-cyan-300 font-mono"
                >
                  <option value="OPERATIONAL_EXCEPTION">Operational Exception</option>
                  <option value="CRITICAL_INFRASTRUCTURE">Critical Infrastructure</option>
                  <option value="CUSTOMER_REQUEST">Customer Request</option>
                  <option value="BUSINESS_OVERRIDE">Business Override</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Reason / Ticket</label>
                <input
                  type="text"
                  placeholder="e.g. Whitelisted for NOC billing callback"
                  value={newReason}
                  onChange={(e) => setNewReason(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-cyan-500"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-gray-800">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-medium rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-500 hover:bg-emerald-400 text-black font-semibold text-xs rounded-lg transition-colors"
                >
                  Add Domain
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
