import React, { useState, useMemo } from 'react';
import {
  ArrowRight,
  Edit3,
  FileText,
  Plus,
  Search,
  Trash2,
  X,
} from 'lucide-react';

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export interface DnsRewriteRecord {
  domain: string;
  type: string;  // 'A' | 'AAAA' | 'CNAME' | 'PTR' | 'TXT'
  value: string;
  ttl: number;
}

interface DnsRewritesPanelProps {
  records: DnsRewriteRecord[];
  onChange: (records: DnsRewriteRecord[]) => void;
}

// ─────────────────────────────────────────────
// Record type badge colour map
// ─────────────────────────────────────────────

const TYPE_BADGE: Record<string, string> = {
  A:     'bg-cyan-950/80 text-cyan-300 border-cyan-800',
  AAAA:  'bg-indigo-950/80 text-indigo-300 border-indigo-800',
  CNAME: 'bg-amber-950/80 text-amber-300 border-amber-800',
  PTR:   'bg-purple-950/80 text-purple-300 border-purple-800',
  TXT:   'bg-emerald-950/80 text-emerald-300 border-emerald-800',
};

const RECORD_TYPES = ['A', 'AAAA', 'CNAME', 'PTR', 'TXT'];

const PLACEHOLDER: Record<string, string> = {
  A:     '192.168.1.10',
  AAAA:  '2001:db8::1',
  CNAME: 'alias.internal.lan',
  PTR:   '192.168.1.10',
  TXT:   'v=spf1 include:example.com ~all',
};

const TYPE_HINT: Record<string, string> = {
  A:     'IPv4 address',
  AAAA:  'IPv6 address',
  CNAME: 'Canonical hostname (no trailing dot needed)',
  PTR:   'IP address to map to the domain name',
  TXT:   'Free-form text value',
};

// ─────────────────────────────────────────────
// Modal — Add / Edit Rewrite
// ─────────────────────────────────────────────

interface ModalProps {
  initial?: DnsRewriteRecord;
  onSave: (r: DnsRewriteRecord) => void;
  onClose: () => void;
}

const EMPTY: DnsRewriteRecord = { domain: '', type: 'A', value: '', ttl: 3600 };

const RewriteModal: React.FC<ModalProps> = ({ initial, onSave, onClose }) => {
  const [form, setForm] = useState<DnsRewriteRecord>(initial ?? EMPTY);
  const [errors, setErrors] = useState<Partial<Record<keyof DnsRewriteRecord, string>>>({});

  const isEdit = Boolean(initial);

  const validate = (): boolean => {
    const e: Partial<Record<keyof DnsRewriteRecord, string>> = {};
    if (!form.domain.trim())                               e.domain = 'Domain is required';
    if (!form.value.trim())                                e.value  = 'Answer / value is required';
    if (form.ttl < 0 || form.ttl > 2147483647)            e.ttl    = 'TTL must be 0–2 147 483 647';
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSave = () => {
    if (!validate()) return;
    onSave({
      ...form,
      domain: form.domain.trim().replace(/\.$/, ''),
      value:  form.value.trim(),
    });
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="relative w-full max-w-lg mx-4 bg-[#0d1117] border border-gray-700 rounded-2xl shadow-2xl shadow-black/60">

        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center">
              <ArrowRight className="w-4 h-4 text-cyan-400" />
            </div>
            <div>
              <h3 className="text-sm font-bold text-white">
                {isEdit ? 'Edit DNS Rewrite' : 'Add DNS Rewrite'}
              </h3>
              <p className="text-[11px] text-gray-500">
                Return a custom answer for a specific domain.
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-gray-500 hover:text-white hover:bg-gray-800 transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="px-6 py-5 space-y-4">

          {/* Domain */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-gray-300">
              Domain <span className="text-red-400">*</span>
            </label>
            <input
              id="rewrite-domain"
              autoFocus
              type="text"
              placeholder="speedtest.isp.net  or  *.internal.lan"
              value={form.domain}
              onChange={(e) => setForm({ ...form, domain: e.target.value })}
              className={`w-full bg-gray-950 border ${
                errors.domain ? 'border-red-600' : 'border-gray-700'
              } rounded-lg px-3 py-2 text-sm text-white font-mono placeholder:text-gray-600 focus:outline-none focus:border-cyan-500 transition-colors`}
            />
            {errors.domain && <p className="text-[11px] text-red-400">{errors.domain}</p>}
            <p className="text-[11px] text-gray-500">
              Use <code className="text-gray-400">*.example.com</code> to match all subdomains (wildcard redirect zone).
            </p>
          </div>

          {/* Type + TTL */}
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-gray-300">
                Record Type <span className="text-red-400">*</span>
              </label>
              <select
                id="rewrite-type"
                value={form.type}
                onChange={(e) => setForm({ ...form, type: e.target.value, value: '' })}
                className="w-full bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-cyan-500 transition-colors"
              >
                {RECORD_TYPES.map((t) => (
                  <option key={t} value={t}>{t}</option>
                ))}
              </select>
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-gray-300">TTL (seconds)</label>
              <input
                id="rewrite-ttl"
                type="number"
                min={0}
                max={2147483647}
                value={form.ttl}
                onChange={(e) => setForm({ ...form, ttl: parseInt(e.target.value, 10) || 0 })}
                className={`w-full bg-gray-950 border ${
                  errors.ttl ? 'border-red-600' : 'border-gray-700'
                } rounded-lg px-3 py-2 text-sm text-white font-mono focus:outline-none focus:border-cyan-500 transition-colors`}
              />
              {errors.ttl && <p className="text-[11px] text-red-400">{errors.ttl}</p>}
            </div>
          </div>

          {/* Answer / Value */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-gray-300">
              Answer / Value <span className="text-red-400">*</span>
            </label>
            <input
              id="rewrite-value"
              type="text"
              placeholder={PLACEHOLDER[form.type] ?? '...'}
              value={form.value}
              onChange={(e) => setForm({ ...form, value: e.target.value })}
              className={`w-full bg-gray-950 border ${
                errors.value ? 'border-red-600' : 'border-gray-700'
              } rounded-lg px-3 py-2 text-sm text-white font-mono placeholder:text-gray-600 focus:outline-none focus:border-cyan-500 transition-colors`}
            />
            {errors.value
              ? <p className="text-[11px] text-red-400">{errors.value}</p>
              : <p className="text-[11px] text-gray-500">{TYPE_HINT[form.type]}</p>
            }
          </div>

          {/* Live preview */}
          {form.domain && form.value && (
            <div className="flex flex-wrap items-center gap-1.5 px-3 py-2.5 bg-gray-900 border border-gray-800 rounded-lg font-mono text-[11px] text-gray-400">
              <span className="text-cyan-300">{form.domain}</span>
              <ArrowRight className="w-3 h-3 shrink-0 text-gray-600" />
              <span className={`px-1.5 py-0.5 rounded border text-[10px] font-bold ${TYPE_BADGE[form.type] ?? ''}`}>
                {form.type}
              </span>
              <span className="text-white break-all">{form.value}</span>
              <span className="ml-auto text-gray-600 whitespace-nowrap">TTL {form.ttl}s</span>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 px-6 py-4 border-t border-gray-800">
          <button
            onClick={onClose}
            className="px-4 py-2 text-xs font-semibold text-gray-400 hover:text-white bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors"
          >
            Cancel
          </button>
          <button
            id="rewrite-save-btn"
            onClick={handleSave}
            className="px-5 py-2 text-xs font-bold bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black rounded-lg transition-all shadow-lg shadow-cyan-500/20"
          >
            {isEdit ? 'Update Rewrite' : 'Add Rewrite'}
          </button>
        </div>
      </div>
    </div>
  );
};

// ─────────────────────────────────────────────
// Main Panel Component
// ─────────────────────────────────────────────

export const DnsRewritesPanel: React.FC<DnsRewritesPanelProps> = ({ records, onChange }) => {
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('ALL');
  const [showModal, setShowModal] = useState(false);
  const [editIndex, setEditIndex] = useState<number | null>(null);

  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    return records
      .map((r, i) => ({ ...r, _idx: i }))
      .filter((r) =>
        (typeFilter === 'ALL' || r.type === typeFilter) &&
        (r.domain.toLowerCase().includes(q) || r.value.toLowerCase().includes(q))
      );
  }, [records, search, typeFilter]);

  const counts = useMemo(() =>
    RECORD_TYPES.reduce<Record<string, number>>((acc, t) => {
      acc[t] = records.filter((r) => r.type === t).length;
      return acc;
    }, {}),
  [records]);

  const handleEdit = (origIdx: number) => { setEditIndex(origIdx); setShowModal(true); };
  const handleAdd  = () => { setEditIndex(null); setShowModal(true); };

  const handleSave = (rec: DnsRewriteRecord) => {
    const next = [...records];
    if (editIndex !== null) {
      next[editIndex] = rec;
    } else {
      next.push(rec);
    }
    onChange(next);
    setShowModal(false);
  };

  const handleDelete = (origIdx: number) => {
    onChange(records.filter((_, i) => i !== origIdx));
  };

  const editingRecord = editIndex !== null ? records[editIndex] : undefined;

  return (
    <>
      {showModal && (
        <RewriteModal
          initial={editingRecord}
          onSave={handleSave}
          onClose={() => setShowModal(false)}
        />
      )}

      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">

        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center shrink-0">
              <ArrowRight className="w-5 h-5 text-cyan-400" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-white flex items-center gap-2">
                DNS Rewrites
                <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-cyan-900/60 text-cyan-300 border border-cyan-800">
                  {records.length} rule{records.length !== 1 ? 's' : ''}
                </span>
              </h2>
              <p className="text-xs text-gray-400 mt-0.5">
                Kembalikan jawaban DNS custom untuk domain tertentu — portal captive, perangkat lokal, speedtest internal, wildcard subdomain.
              </p>
            </div>
          </div>

          <button
            id="add-dns-rewrite-btn"
            onClick={handleAdd}
            className="shrink-0 px-3 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-black text-xs font-bold rounded-lg flex items-center gap-1.5 shadow-lg shadow-cyan-600/20 transition-colors"
          >
            <Plus className="w-3.5 h-3.5" /> Add DNS Rewrite
          </button>
        </div>

        {/* Type-filter chips */}
        {records.length > 0 && (
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setTypeFilter('ALL')}
              className={`px-2.5 py-0.5 rounded-full text-[11px] font-bold border transition-colors ${
                typeFilter === 'ALL'
                  ? 'bg-gray-700 text-white border-gray-600'
                  : 'bg-gray-900 text-gray-500 border-gray-800 hover:border-gray-600 hover:text-gray-300'
              }`}
            >
              ALL ({records.length})
            </button>
            {RECORD_TYPES.filter((t) => counts[t] > 0).map((t) => (
              <button
                key={t}
                onClick={() => setTypeFilter(typeFilter === t ? 'ALL' : t)}
                className={`px-2.5 py-0.5 rounded-full text-[11px] font-bold border transition-colors ${
                  typeFilter === t
                    ? TYPE_BADGE[t]
                    : 'bg-gray-900 text-gray-500 border-gray-800 hover:border-gray-600 hover:text-gray-300'
                }`}
              >
                {t} ({counts[t]})
              </button>
            ))}
          </div>
        )}

        {/* Search */}
        {records.length > 0 && (
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500 pointer-events-none" />
            <input
              id="rewrite-search"
              type="text"
              placeholder="Search domain or IP address…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-8 pr-3 py-2 text-xs text-gray-300 placeholder:text-gray-600 focus:outline-none focus:border-cyan-700 transition-colors"
            />
          </div>
        )}

        {/* Table / Empty state */}
        {records.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 space-y-3 text-center">
            <div className="w-14 h-14 rounded-2xl bg-gray-800/60 border border-gray-700 flex items-center justify-center">
              <FileText className="w-7 h-7 text-gray-600" />
            </div>
            <p className="text-sm font-semibold text-gray-400">No DNS Rewrites configured</p>
            <p className="text-xs text-gray-600 max-w-xs">
              Add custom DNS responses for internal domains, local devices, captive portals, or speedtest servers.
            </p>
            <button
              onClick={handleAdd}
              className="mt-2 px-4 py-2 text-xs font-bold bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-400 border border-cyan-700 rounded-lg transition-colors"
            >
              <Plus className="w-3.5 h-3.5 inline mr-1" />
              Add First Rewrite
            </button>
          </div>
        ) : filtered.length === 0 ? (
          <div className="py-8 text-center text-xs text-gray-500">
            No records match your search.{' '}
            <button
              onClick={() => { setSearch(''); setTypeFilter('ALL'); }}
              className="text-cyan-400 hover:underline"
            >
              Clear filter
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-gray-800">
            <table className="w-full text-left text-xs font-mono">
              <thead className="bg-gray-950 text-gray-500 border-b border-gray-800">
                <tr>
                  <th className="py-2.5 px-4">Domain</th>
                  <th className="py-2.5 px-3 w-16">Type</th>
                  <th className="py-2.5 px-4">Answer / Value</th>
                  <th className="py-2.5 px-3 w-20">TTL (s)</th>
                  <th className="py-2.5 px-3 w-20 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800/50">
                {filtered.map((rec) => (
                  <tr
                    key={`${rec._idx}-${rec.domain}`}
                    className="group hover:bg-gray-800/30 transition-colors"
                  >
                    {/* Domain */}
                    <td className="py-2.5 px-4">
                      <span className="text-white">{rec.domain}</span>
                      {rec.domain.startsWith('*') && (
                        <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded bg-purple-950 text-purple-300 border border-purple-800">
                          wildcard
                        </span>
                      )}
                    </td>

                    {/* Type */}
                    <td className="py-2.5 px-3">
                      <span className={`px-2 py-0.5 rounded border text-[10px] font-bold ${TYPE_BADGE[rec.type] ?? 'bg-gray-800 text-gray-300 border-gray-700'}`}>
                        {rec.type}
                      </span>
                    </td>

                    {/* Value */}
                    <td className="py-2.5 px-4">
                      <div className="flex items-center gap-1.5 text-cyan-200">
                        <ArrowRight className="w-3 h-3 text-gray-600 shrink-0" />
                        <span className="truncate max-w-[200px]" title={rec.value}>
                          {rec.value}
                        </span>
                      </div>
                    </td>

                    {/* TTL */}
                    <td className="py-2.5 px-3 text-gray-500">
                      {rec.ttl.toLocaleString()}
                    </td>

                    {/* Actions */}
                    <td className="py-2.5 px-3 text-right">
                      <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <button
                          onClick={() => handleEdit(rec._idx)}
                          title="Edit"
                          className="p-1 rounded text-gray-500 hover:text-cyan-400 hover:bg-cyan-950/40 transition-colors"
                        >
                          <Edit3 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => handleDelete(rec._idx)}
                          title="Delete"
                          className="p-1 rounded text-gray-500 hover:text-red-400 hover:bg-red-950/40 transition-colors"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}


      </div>
    </>
  );
};
