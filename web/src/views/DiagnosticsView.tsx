import React, { useState } from 'react';
import {
  AlertCircle,
  ArrowRight,
  CheckCircle2,
  Clock,
  Filter,
  Globe,
  HelpCircle,
  PlusCircle,
  Search,
  ShieldAlert,
  ShieldCheck,
  Zap,
} from 'lucide-react';
import { DiagnosisResult } from '../types';
import { api } from '../services/api';

interface DiagnosticsViewProps {
  initialDomain?: string;
}

export const DiagnosticsView: React.FC<DiagnosticsViewProps> = ({ initialDomain = '' }) => {
  const [domain, setDomain] = useState(initialDomain || 'doubleclick.net');
  const [clientIp, setClientIp] = useState('172.18.20.100');
  const [policy, setPolicy] = useState('DEFAULT');
  const [result, setResult] = useState<DiagnosisResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Allowlist modal state
  const [showAllowModal, setShowAllowModal] = useState(false);
  const [allowCategory, setAllowCategory] = useState('OPERATIONAL_EXCEPTION');
  const [allowReason, setAllowReason] = useState('ISP customer request');
  const [allowSuccess, setAllowSuccess] = useState<string | null>(null);

  const handleDiagnose = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!domain.trim()) return;

    setLoading(true);
    setError(null);
    setAllowSuccess(null);

    try {
      const res = await api.diagnoseDomain(domain.trim(), clientIp.trim(), policy);
      setResult(res);
    } catch (err: any) {
      setError(err.message || 'Diagnostics failed to execute');
    } finally {
      setLoading(false);
    }
  };

  const handleAddToAllowlist = async () => {
    if (!result?.domain) return;
    try {
      await api.addAllowlist(result.domain, allowCategory, allowReason);
      setAllowSuccess(`Domain ${result.domain} successfully added to Allowlist!`);
      setShowAllowModal(false);
      // Re-run diagnosis to show it's now allowed!
      handleDiagnose();
    } catch (err: any) {
      setError(`Failed to add to allowlist: ${err.message}`);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header section */}
      <div>
        <h1 className="text-xl font-bold text-white flex items-center gap-2">
          <Search className="w-5 h-5 text-cyan-400" />
          Domain Diagnostics & "Why Blocked?" Inspector
        </h1>
        <p className="text-xs text-gray-400 mt-1">
          Perform explainable DNS policy inspection, simulate client IP context, and inspect exact rule sources and resolution latency.
        </p>
      </div>

      {/* Query Form */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 shadow-lg">
        <form onSubmit={handleDiagnose} className="grid grid-cols-1 md:grid-cols-4 gap-4 items-end">
          <div className="md:col-span-2">
            <label className="block text-xs font-semibold text-gray-300 mb-1.5">
              Domain Name to Probe
            </label>
            <input
              type="text"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="e.g. doubleclick.net, safe.google.com, github.com"
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white placeholder-gray-500 font-mono focus:outline-none focus:border-cyan-500"
              required
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-300 mb-1.5">
              Simulated Client IP
            </label>
            <input
              type="text"
              value={clientIp}
              onChange={(e) => setClientIp(e.target.value)}
              placeholder="172.18.20.100"
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white placeholder-gray-500 font-mono focus:outline-none focus:border-cyan-500"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-300 mb-1.5">
              Applied Policy Profile
            </label>
            <select
              value={policy}
              onChange={(e) => setPolicy(e.target.value)}
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-cyan-300 font-mono focus:outline-none focus:border-cyan-500 cursor-pointer"
            >
              <option value="DEFAULT">DEFAULT (Security + Malware)</option>
              <option value="FAMILY">FAMILY (SafeSearch + Adult Block)</option>
              <option value="STRICT_ISP">STRICT_ISP (Ads + Trackers + Gambling)</option>
              <option value="UNFILTERED">UNFILTERED (Direct Resolution)</option>
            </select>
          </div>

          <div className="md:col-span-4 flex items-center justify-between pt-2 border-t border-gray-800">
            <div className="flex items-center gap-2 text-xs text-gray-400">
              <HelpCircle className="w-3.5 h-3.5 text-cyan-400" />
              <span>Evaluates: Allowlist &rarr; Custom Rewrites &rarr; Blocklists &rarr; SafeSearch &rarr; Data Plane</span>
            </div>
            <button
              type="submit"
              disabled={loading}
              className="px-6 py-2 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-semibold text-xs rounded-lg transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2 disabled:opacity-50"
            >
              {loading ? (
                <>Evaluating Policy Pipeline...</>
              ) : (
                <>
                  <Search className="w-3.5 h-3.5" /> Run Diagnostic Probe
                </>
              )}
            </button>
          </div>
        </form>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-red-950/50 border border-red-800 text-red-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {allowSuccess && (
        <div className="p-4 rounded-xl bg-emerald-950/50 border border-emerald-800 text-emerald-300 text-xs flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4 shrink-0" />
          <span>{allowSuccess}</span>
        </div>
      )}

      {/* Diagnosis Results Display */}
      {result && (
        <div className="space-y-6">
          {/* Main Decision Banner */}
          <div
            className={`p-6 rounded-xl border flex flex-col md:flex-row items-start md:items-center justify-between gap-4 ${
              result.action === 'BLOCKED'
                ? 'bg-red-950/30 border-red-800/80 text-red-200'
                : result.action === 'REWRITE'
                ? 'bg-amber-950/30 border-amber-800/80 text-amber-200'
                : 'bg-emerald-950/30 border-emerald-800/80 text-emerald-200'
            }`}
          >
            <div className="flex items-center gap-4">
              <div
                className={`w-12 h-12 rounded-xl flex items-center justify-center font-bold text-lg ${
                  result.action === 'BLOCKED'
                    ? 'bg-red-500/20 text-red-400 border border-red-500/40'
                    : result.action === 'REWRITE'
                    ? 'bg-amber-500/20 text-amber-400 border border-amber-500/40'
                    : 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/40'
                }`}
              >
                {result.action === 'BLOCKED' ? (
                  <ShieldAlert className="w-6 h-6" />
                ) : (
                  <ShieldCheck className="w-6 h-6" />
                )}
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-xl font-bold tracking-wider font-mono">{result.action}</span>
                  <span className="text-xs px-2 py-0.5 rounded bg-gray-900 border border-gray-700 text-gray-300 font-mono">
                    Policy: {result.policy}
                  </span>
                </div>
                <p className="text-xs mt-1 text-gray-300">
                  Target Domain: <strong className="font-mono text-white">{result.domain}</strong>
                </p>
              </div>
            </div>

            {result.action === 'BLOCKED' && (
              <button
                onClick={() => setShowAllowModal(true)}
                className="px-4 py-2 bg-emerald-500 hover:bg-emerald-400 text-black font-semibold text-xs rounded-lg transition-all flex items-center gap-2 shadow-lg shadow-emerald-500/20 shrink-0"
              >
                <PlusCircle className="w-4 h-4" />
                Add to Allowlist Exception
              </button>
            )}
          </div>

          {/* Details Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Policy & Source Explanation */}
            <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
              <h2 className="text-sm font-bold text-white flex items-center gap-2">
                <Filter className="w-4 h-4 text-cyan-400" />
                Policy & Rule Attribution
              </h2>

              <div className="space-y-3 text-xs font-mono">
                <div className="p-3 bg-gray-950 rounded-lg flex items-center justify-between border border-gray-800/80">
                  <span className="text-gray-400">Decision Reason:</span>
                  <span className="text-white font-semibold">{result.explanation}</span>
                </div>

                {result.category && (
                  <div className="p-3 bg-gray-950 rounded-lg flex items-center justify-between border border-gray-800/80">
                    <span className="text-gray-400">Category:</span>
                    <span className="text-amber-400 font-bold">{result.category}</span>
                  </div>
                )}

                {result.source && (
                  <div className="p-3 bg-gray-950 rounded-lg flex items-center justify-between border border-gray-800/80">
                    <span className="text-gray-400">Source Feed:</span>
                    <span className="text-cyan-400 font-semibold">{result.source}</span>
                  </div>
                )}

                {result.rule && (
                  <div className="p-3 bg-gray-950 rounded-lg flex items-center justify-between border border-gray-800/80">
                    <span className="text-gray-400">Matched Rule Pattern:</span>
                    <span className="text-rose-400 font-bold">{result.rule}</span>
                  </div>
                )}

                {result.rewrite_value && (
                  <div className="p-3 bg-gray-950 rounded-lg flex items-center justify-between border border-gray-800/80">
                    <span className="text-gray-400">SafeSearch CNAME Target:</span>
                    <span className="text-emerald-400 font-bold">{result.rewrite_value}</span>
                  </div>
                )}
              </div>
            </div>

            {/* Live Resolution Pipeline */}
            <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
              <h2 className="text-sm font-bold text-white flex items-center gap-2">
                <Globe className="w-4 h-4 text-cyan-400" />
                Live Data Plane Query
              </h2>

              <div className="space-y-3 text-xs font-mono">
                <div className="p-3 bg-gray-950 rounded-lg flex items-center justify-between border border-gray-800/80">
                  <span className="text-gray-400">Resolver Health:</span>
                  <span className="text-emerald-400 flex items-center gap-1 font-semibold">
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    {result.live_resolution?.healthy ? 'Operational' : 'Failed'}
                  </span>
                </div>

                <div className="p-3 bg-gray-950 rounded-lg flex items-center justify-between border border-gray-800/80">
                  <span className="text-gray-400">Query Resolution Time:</span>
                  <span className="text-cyan-400 font-bold flex items-center gap-1">
                    <Clock className="w-3.5 h-3.5" />
                    {result.live_resolution?.latency || '0.9ms'}
                  </span>
                </div>

                <div className="p-3 bg-gray-950 rounded-lg space-y-2 border border-gray-800/80">
                  <span className="text-gray-400 block">Resolved Answer Records:</span>
                  {result.live_resolution?.resolved_ips && result.live_resolution.resolved_ips.length > 0 ? (
                    <div className="flex flex-wrap gap-2 pt-1">
                      {result.live_resolution.resolved_ips.map((ip, idx) => (
                        <span key={idx} className="px-2 py-1 rounded bg-gray-900 border border-gray-700 text-emerald-400 font-bold">
                          {ip}
                        </span>
                      ))}
                    </div>
                  ) : (
                    <span className="text-gray-500 italic block">
                      {result.action === 'BLOCKED' ? '0.0.0.0 (Sinkhole)' : 'No A records returned'}
                    </span>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Add to Allowlist Modal */}
      {showAllowModal && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 max-w-md w-full space-y-4 shadow-2xl">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <ShieldCheck className="w-5 h-5 text-emerald-400" />
              Add Domain to Allowlist Exception
            </h2>

            <p className="text-xs text-gray-400 leading-relaxed">
              Whitelisting will override any blocklist or category policy across all client networks.
            </p>

            <div className="space-y-3">
              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Target Domain</label>
                <input
                  type="text"
                  disabled
                  value={result?.domain || ''}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-gray-300 font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Category Exception</label>
                <select
                  value={allowCategory}
                  onChange={(e) => setAllowCategory(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-cyan-300 font-mono"
                >
                  <option value="OPERATIONAL_EXCEPTION">Operational Exception</option>
                  <option value="BUSINESS_OVERRIDE">Business Override</option>
                  <option value="CUSTOMER_REQUEST">Customer Request</option>
                  <option value="GOVERNMENT_COMPLIANCE">Government Compliance</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Reason / NOC Ticket Reference</label>
                <input
                  type="text"
                  value={allowReason}
                  onChange={(e) => setAllowReason(e.target.value)}
                  placeholder="e.g. NOC-8924: Unblock banking subsystem"
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white placeholder-gray-500 font-mono focus:outline-none focus:border-cyan-500"
                />
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-3 border-t border-gray-800">
              <button
                onClick={() => setShowAllowModal(false)}
                className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-medium rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={handleAddToAllowlist}
                className="px-4 py-2 bg-emerald-500 hover:bg-emerald-400 text-black font-semibold text-xs rounded-lg transition-colors"
              >
                Confirm Whitelist
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
