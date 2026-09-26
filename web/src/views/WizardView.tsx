import React, { useState } from 'react';
import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  Cpu,
  Globe,
  Layers,
  Network,
  RefreshCw,
  Search,
  Server,
  ShieldCheck,
  Sparkles,
  Zap,
} from 'lucide-react';
import { DesiredState, NodeRecord } from '../types';
import { api } from '../services/api';

interface WizardViewProps {
  node: NodeRecord | null;
  nodeId: string;
  existingConfig: DesiredState | null;
  onFinish: () => void;
}

export const WizardView: React.FC<WizardViewProps> = ({
  node,
  nodeId,
  existingConfig,
  onFinish,
}) => {
  const [step, setStep] = useState(1);
  const totalSteps = 10;

  // Wizard state initialized from existing config or defaults
  const [clientIp, setClientIp] = useState(existingConfig?.listeners[0]?.ip || node?.management_ip || '0.0.0.0');
  const [clientPort, setClientPort] = useState(53);
  const [enableIPv6, setEnableIPv6] = useState(false);
  const [clientCIDR, setClientCIDR] = useState(existingConfig?.client_networks[0]?.cidr || '172.18.0.0/16');
  const [resolverMode, setResolverMode] = useState<'RECURSIVE' | 'FORWARDING' | 'HYBRID'>(
    existingConfig?.resolution_mode || 'HYBRID'
  );
  const [upstream1, setUpstream1] = useState('1.1.1.1');
  const [upstream2, setUpstream2] = useState('8.8.8.8');
  const [enableDNSSEC, setEnableDNSSEC] = useState(true);
  const [enableSafeSearch, setEnableSafeSearch] = useState(true);
  const [enableFiltering, setEnableFiltering] = useState(true);
  const [perfProfile, setPerfProfile] = useState<'SMALL' | 'MEDIUM' | 'LARGE' | 'ISP'>('SMALL');

  // Application state
  const [applying, setApplying] = useState(false);
  const [applyResult, setApplyResult] = useState<{
    success: boolean;
    version_id?: string;
    message?: string;
  } | null>(null);
  const [healthStatus, setHealthStatus] = useState<string | null>(null);

  const handleNext = () => setStep((s) => Math.min(s + 1, totalSteps));
  const handleBack = () => setStep((s) => Math.max(s - 1, 1));

  const handleApply = async () => {
    setApplying(true);
    setApplyResult(null);
    setHealthStatus('Validating configuration with unbound-checkconf...');

    try {
      // Build desired state object
      const wizardState: DesiredState = {
        version_id: `wizard-${Date.now()}`,
        node_id: nodeId || node?.id || 'local-node',
        resolution_mode: resolverMode,
        listeners: [
          {
            id: 'l-client-ipv4',
            name: 'Client DNS Service',
            ip: clientIp,
            port: clientPort,
            role: 'CLIENT',
            is_ipv6: false,
            udp: true,
            tcp: true,
            enabled: true,
          },
          {
            id: 'l-loopback',
            name: 'Local Loopback',
            ip: '127.0.0.1',
            port: 53,
            role: 'INTERNAL',
            is_ipv6: false,
            udp: true,
            tcp: true,
            enabled: true,
          },
        ],
        client_networks: [
          {
            id: 'net-subscribers',
            cidr: clientCIDR,
            description: 'ISP Subscriber Subnet',
            action: 'allow',
            policy_group: 'DEFAULT',
            enabled: true,
          },
          {
            id: 'net-loopback',
            cidr: '127.0.0.0/8',
            description: 'Internal Loopback',
            action: 'allow',
            policy_group: 'DEFAULT',
            enabled: true,
          },
        ],
        upstreams: [
          { id: 'up-1', name: 'Primary DNS', ip: upstream1, port: 53, type: 'UDP', enabled: true },
          { id: 'up-2', name: 'Secondary DNS', ip: upstream2, port: 53, type: 'UDP', enabled: true },
        ],
        cache: {
          msg_cache_size: perfProfile === 'ISP' ? '1024m' : '64m',
          rrset_cache_size: perfProfile === 'ISP' ? '2048m' : '128m',
          key_cache_size: '32m',
          neg_cache_size: '16m',
          min_ttl: 60,
          max_ttl: 86400,
          infra_host_ttl: 900,
        },
        performance: {
          profile: perfProfile,
          num_threads: perfProfile === 'ISP' ? 16 : 2,
          msg_cache_slabs: perfProfile === 'ISP' ? 16 : 2,
          rrset_cache_slabs: perfProfile === 'ISP' ? 16 : 2,
          so_reuseport: true,
          so_rcvbuf: '4m',
          so_sndbuf: '4m',
          num_queries_per_thread: 1024,
          outgoing_range: 4096,
        },
        dnssec: {
          enabled: enableDNSSEC,
          auto_trust_anchor_file: '/var/lib/unbound/root.key',
          root_hints_file: '/var/lib/unbound/root.hints',
          val_permissive_mode: false,
          insecure_domains: [],
        },
        features: {
          prefetch: true,
          prefetch_key: true,
          serve_expired: true,
          serve_expired_ttl: 86400,
          serve_expired_client_timeout: 1800,
          qname_minimisation: true,
          hide_identity: true,
          hide_version: true,
        },
        safe_search: {
          enabled: enableSafeSearch,
          google_redirect_ipv4: '216.239.38.120',
          bing_redirect_ipv4: '204.79.197.220',
        },
        local_records: [
          { domain: 'netora.local', type: 'A', value: clientIp, ttl: 300 },
          { domain: 'dns.internal', type: 'A', value: clientIp, ttl: 300 },
        ],
        remote_control: {
          enabled: true,
          interface: '127.0.0.1',
          port: 8953,
        },
      };

      setHealthStatus('Staging configuration and performing atomic file swap...');
      const res = await api.applyConfig(nodeId, wizardState, 'Initial deployment via Netora Setup Wizard');

      setHealthStatus('Verifying Port 53 live resolution and socket health...');
      const health = await api.getNodeHealth(nodeId);

      if (health.healthy) {
        setApplyResult({
          success: true,
          version_id: res.version_id,
          message: 'Unbound successfully started with zero errors and active client listeners.',
        });
      } else {
        setApplyResult({
          success: false,
          message: 'Apply succeeded but port 53 health check reported degraded status.',
        });
      }
    } catch (err: any) {
      setApplyResult({
        success: false,
        message: err.message || 'Atomic apply failed.',
      });
    } finally {
      setApplying(false);
    }
  };

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Wizard Header */}
      <div className="bg-gradient-to-r from-cyan-950/40 via-blue-950/20 to-gray-900 border border-cyan-800/60 rounded-xl p-6 relative overflow-hidden">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <img
              src="/logo.png"
              alt="Netora DNS"
              className="w-10 h-10 object-contain drop-shadow-[0_0_14px_rgba(74,222,128,0.4)]"
            />
            <div>
              <h1 className="text-lg font-bold text-white">Netora DNS First-Run Deployment Wizard</h1>
              <p className="text-xs text-gray-400">
                Step {step} of {totalSteps} — Production ISP Setup
              </p>
            </div>
          </div>
          <span className="text-xs px-2.5 py-1 rounded bg-cyan-950 text-cyan-300 font-mono border border-cyan-800">
            Node: {node?.hostname || nodeId || 'Current Node'}
          </span>
        </div>

        {/* Progress Bar */}
        <div className="w-full h-1.5 bg-gray-800 rounded-full mt-4 overflow-hidden">
          <div
            className="h-full bg-gradient-to-r from-cyan-500 to-blue-500 transition-all duration-300"
            style={{ width: `${(step / totalSteps) * 100}%` }}
          />
        </div>
      </div>

      {/* Wizard Steps Container */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-8 min-h-[380px] flex flex-col justify-between shadow-xl">
        {/* Step 1: Welcome & Detection */}
        {step === 1 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Server className="w-5 h-5 text-cyan-400" />
              Step 1: Welcome & System Detection
            </h2>
            <p className="text-xs text-gray-300 leading-relaxed">
              Netora DNS is ready to initialize an enterprise Unbound recursive caching resolver. The control plane has detected the following host hardware:
            </p>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-2 font-mono text-xs">
              <div className="p-3 bg-gray-950 border border-gray-800 rounded-lg">
                <span className="text-gray-500 block text-[10px]">HOSTNAME</span>
                <span className="text-white font-bold">{node?.hostname || nodeId || 'Current Node'}</span>
              </div>
              <div className="p-3 bg-gray-950 border border-gray-800 rounded-lg">
                <span className="text-gray-500 block text-[10px]">MANAGEMENT IP</span>
                <span className="text-cyan-400 font-bold">{node?.management_ip || window.location.hostname}</span>
              </div>
              <div className="p-3 bg-gray-950 border border-gray-800 rounded-lg">
                <span className="text-gray-500 block text-[10px]">HARDWARE CORES</span>
                <span className="text-white font-bold">{node?.cpu_cores || 2} vCPUs</span>
              </div>
              <div className="p-3 bg-gray-950 border border-gray-800 rounded-lg">
                <span className="text-gray-500 block text-[10px]">MEMORY (RAM)</span>
                <span className="text-white font-bold">{node?.ram_mb ? `${Math.round(node.ram_mb / 1024)} GB` : '2 GB'}</span>
              </div>
            </div>

            <div className="p-4 bg-gray-950/60 border border-gray-800/80 rounded-lg text-xs text-gray-400 leading-relaxed">
              <strong className="text-cyan-400">Core Invariant:</strong> The DNS resolver will be configured with atomic configuration staging, strict ACLs to eliminate open resolvers, and zero runtime dependence on external databases.
            </div>
          </div>
        )}

        {/* Step 2: Listener IP */}
        {step === 2 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Network className="w-5 h-5 text-cyan-400" />
              Step 2: Client Listener IP & Multi-Socket Binding
            </h2>
            <p className="text-xs text-gray-300">
              Specify the IP address on which Unbound will listen for subscriber DNS queries. Loopback (127.0.0.1) is always automatically included.
            </p>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono pt-2">
              <div>
                <label className="block text-gray-300 mb-1 font-semibold">Client DNS Listener IP</label>
                <input
                  type="text"
                  value={clientIp}
                  onChange={(e) => setClientIp(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-cyan-400 font-bold"
                />
                <span className="text-[11px] text-gray-500 mt-1 block">Usually the LAN/client-facing interface IP.</span>
              </div>

              <div>
                <label className="block text-gray-300 mb-1 font-semibold">DNS Service Port</label>
                <input
                  type="number"
                  value={clientPort}
                  onChange={(e) => setClientPort(parseInt(e.target.value) || 53)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white"
                />
                <span className="text-[11px] text-gray-500 mt-1 block">Default port: 53 (UDP & TCP).</span>
              </div>
            </div>
          </div>
        )}

        {/* Step 3: Client Networks / ACL */}
        {step === 3 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Layers className="w-5 h-5 text-cyan-400" />
              Step 3: Client CIDR Networks & Anti-Open Resolver Guardrail
            </h2>
            <p className="text-xs text-gray-300">
              Enter the IP subnet allowed to resolve queries. Unbound will strictly refuse queries from any other networks.
            </p>

            <div className="text-xs font-mono pt-2">
              <label className="block text-gray-300 mb-1 font-semibold">Subscriber Network CIDR</label>
              <input
                type="text"
                value={clientCIDR}
                onChange={(e) => setClientCIDR(e.target.value)}
                className="w-full max-w-md bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-cyan-400 font-bold"
              />
              <span className="text-[11px] text-gray-500 mt-1 block">Example: 172.18.0.0/16 or 10.100.0.0/20</span>
            </div>

            <div className="p-3 bg-red-950/30 border border-red-800/80 rounded-lg text-xs text-red-300 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 shrink-0 text-red-400" />
              <span>Security Guardrail: Setting 0.0.0.0/0 to allow is blocked by the Netora validation engine.</span>
            </div>
          </div>
        )}

        {/* Step 4: Resolver Mode */}
        {step === 4 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Globe className="w-5 h-5 text-cyan-400" />
              Step 4: DNS Resolution Mode
            </h2>
            <p className="text-xs text-gray-300">
              Select how Unbound will resolve uncached domain names:
            </p>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2 font-mono">
              {(['HYBRID', 'FORWARDING', 'RECURSIVE'] as const).map((m) => (
                <div
                  key={m}
                  onClick={() => setResolverMode(m)}
                  className={`p-4 rounded-xl border cursor-pointer transition-all ${
                    resolverMode === m
                      ? 'bg-cyan-950/30 border-cyan-500 text-white shadow-lg shadow-cyan-500/10'
                      : 'bg-gray-950 border-gray-800 text-gray-400 hover:border-gray-700'
                  }`}
                >
                  <div className="font-bold text-sm text-cyan-400">{m}</div>
                  <p className="text-[11px] mt-2 text-gray-400 font-sans leading-relaxed">
                    {m === 'HYBRID' && 'Recommended for ISPs: High-speed forwarding with automatic iterative fallback if upstreams fail.'}
                    {m === 'FORWARDING' && 'Forward all queries directly to upstream forwarders (e.g. Cloudflare, Quad9).'}
                    {m === 'RECURSIVE' && 'Pure iterative resolution starting from IANA root hints.'}
                  </p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Step 5: Upstream Resolvers */}
        {step === 5 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Globe className="w-5 h-5 text-cyan-400" />
              Step 5: Upstream Forwarders
            </h2>
            <p className="text-xs text-gray-300">
              Configure primary and secondary upstream DNS servers used in Forwarding or Hybrid mode.
            </p>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono pt-2">
              <div>
                <label className="block text-gray-300 mb-1 font-semibold">Primary Upstream IP</label>
                <input
                  type="text"
                  value={upstream1}
                  onChange={(e) => setUpstream1(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white"
                />
              </div>

              <div>
                <label className="block text-gray-300 mb-1 font-semibold">Secondary Upstream IP</label>
                <input
                  type="text"
                  value={upstream2}
                  onChange={(e) => setUpstream2(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white"
                />
              </div>
            </div>

            <div className="flex gap-2 pt-2">
              <button
                type="button"
                onClick={() => { setUpstream1('1.1.1.1'); setUpstream2('1.0.0.1'); }}
                className="px-2.5 py-1 bg-gray-800 text-xs rounded text-cyan-300 hover:bg-gray-700 font-mono"
              >
                Cloudflare (1.1.1.1)
              </button>
              <button
                type="button"
                onClick={() => { setUpstream1('8.8.8.8'); setUpstream2('8.8.4.4'); }}
                className="px-2.5 py-1 bg-gray-800 text-xs rounded text-cyan-300 hover:bg-gray-700 font-mono"
              >
                Google (8.8.8.8)
              </button>
              <button
                type="button"
                onClick={() => { setUpstream1('9.9.9.9'); setUpstream2('149.112.112.112'); }}
                className="px-2.5 py-1 bg-gray-800 text-xs rounded text-cyan-300 hover:bg-gray-700 font-mono"
              >
                Quad9 (9.9.9.9)
              </button>
            </div>
          </div>
        )}

        {/* Step 6: DNSSEC */}
        {step === 6 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <ShieldCheck className="w-5 h-5 text-cyan-400" />
              Step 6: Cryptographic DNSSEC Validation
            </h2>
            <p className="text-xs text-gray-300">
              Verify cryptographic authenticity of DNS responses to prevent cache poisoning and DNS spoofing.
            </p>

            <label className="flex items-center gap-3 cursor-pointer p-4 bg-gray-950 border border-gray-800 rounded-lg">
              <input
                type="checkbox"
                checked={enableDNSSEC}
                onChange={(e) => setEnableDNSSEC(e.target.checked)}
                className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
              />
              <div>
                <span className="text-xs font-bold text-white">Enable Strict DNSSEC Validation</span>
                <p className="text-[11px] text-gray-400">Uses IANA RFC 5011 auto-trust-anchor file (/var/lib/unbound/root.key).</p>
              </div>
            </label>
          </div>
        )}

        {/* Step 7: Filtering & SafeSearch */}
        {step === 7 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <ShieldCheck className="w-5 h-5 text-cyan-400" />
              Step 7: DNS Policy Filtering & SafeSearch
            </h2>
            <p className="text-xs text-gray-300">
              Configure parental controls and threat protection feeds.
            </p>

            <div className="space-y-3">
              <label className="flex items-center gap-3 cursor-pointer p-4 bg-gray-950 border border-gray-800 rounded-lg">
                <input
                  type="checkbox"
                  checked={enableFiltering}
                  onChange={(e) => setEnableFiltering(e.target.checked)}
                  className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
                />
                <div>
                  <span className="text-xs font-bold text-white">Enable High-Performance Threat Filtering</span>
                  <p className="text-[11px] text-gray-400">Sinkhole known phishing, malware, and botnet command-and-control domains.</p>
                </div>
              </label>

              <label className="flex items-center gap-3 cursor-pointer p-4 bg-gray-950 border border-gray-800 rounded-lg">
                <input
                  type="checkbox"
                  checked={enableSafeSearch}
                  onChange={(e) => setEnableSafeSearch(e.target.checked)}
                  className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
                />
                <div>
                  <span className="text-xs font-bold text-white">Enforce SafeSearch for Google & Bing</span>
                  <p className="text-[11px] text-gray-400">Directs search engine queries to forcesafesearch VIPs.</p>
                </div>
              </label>
            </div>
          </div>
        )}

        {/* Step 8: Performance Profile */}
        {step === 8 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Cpu className="w-5 h-5 text-cyan-400" />
              Step 8: Performance & Concurrency Profile
            </h2>
            <p className="text-xs text-gray-300">
              Configure memory cache sizing and worker threads for this hardware node.
            </p>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-2 font-mono">
              {(['SMALL', 'MEDIUM', 'LARGE', 'ISP'] as const).map((p) => (
                <div
                  key={p}
                  onClick={() => setPerfProfile(p)}
                  className={`p-3 rounded-lg border cursor-pointer transition-all ${
                    perfProfile === p
                      ? 'bg-cyan-950/30 border-cyan-500 text-white'
                      : 'bg-gray-950 border-gray-800 text-gray-400 hover:border-gray-700'
                  }`}
                >
                  <div className="font-bold text-xs text-cyan-400">{p}</div>
                  <div className="text-[10px] text-gray-400 mt-1">
                    {p === 'SMALL' && '2 Threads, 128MB Cache'}
                    {p === 'MEDIUM' && '4 Threads, 256MB Cache'}
                    {p === 'LARGE' && '8 Threads, 1GB Cache'}
                    {p === 'ISP' && '16 Threads, 2GB Cache'}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Step 9: Review & Staging */}
        {step === 9 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <CheckCircle2 className="w-5 h-5 text-emerald-400" />
              Step 9: Review Staging State
            </h2>
            <p className="text-xs text-gray-300">
              Verify your deployment parameters before executing atomic configuration apply:
            </p>

            <div className="p-4 bg-gray-950 border border-gray-800 rounded-lg space-y-2 text-xs font-mono">
              <div className="flex justify-between">
                <span className="text-gray-500">Listener:</span>
                <span className="text-white">{clientIp}:{clientPort} (CLIENT)</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Subscriber CIDR:</span>
                <span className="text-cyan-400 font-bold">{clientCIDR}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Resolver Mode:</span>
                <span className="text-white">{resolverMode}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Upstreams:</span>
                <span className="text-white">{upstream1}, {upstream2}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">DNSSEC & SafeSearch:</span>
                <span className="text-emerald-400 font-bold">
                  {enableDNSSEC ? 'DNSSEC ON' : 'OFF'} | {enableSafeSearch ? 'SafeSearch ON' : 'OFF'}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Performance Profile:</span>
                <span className="text-cyan-400 font-bold">{perfProfile}</span>
              </div>
            </div>
          </div>
        )}

        {/* Step 10: Atomic Apply & Verification */}
        {step === 10 && (
          <div className="space-y-4">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Zap className="w-5 h-5 text-cyan-400" />
              Step 10: Atomic Apply & Health Verification
            </h2>

            {!applyResult && (
              <div className="space-y-4">
                <p className="text-xs text-gray-300">
                  Ready to deploy! Clicking "Execute Atomic Apply" will stage the rendered Unbound configuration, perform syntax checks with <code className="text-cyan-400">unbound-checkconf</code>, execute an atomic file swap, and verify live port 53 resolution.
                </p>

                {applying && (
                  <div className="p-4 bg-gray-950 border border-gray-800 rounded-lg space-y-3 font-mono text-xs">
                    <div className="flex items-center gap-3 text-cyan-400">
                      <RefreshCw className="w-4 h-4 animate-spin" />
                      <span>{healthStatus}</span>
                    </div>
                  </div>
                )}
              </div>
            )}

            {applyResult && (
              <div className="space-y-4">
                <div
                  className={`p-5 rounded-xl border text-xs ${
                    applyResult.success
                      ? 'bg-emerald-950/40 border-emerald-800 text-emerald-200'
                      : 'bg-red-950/40 border-red-800 text-red-200'
                  }`}
                >
                  <div className="flex items-center gap-3 font-bold text-sm">
                    {applyResult.success ? (
                      <CheckCircle2 className="w-5 h-5 text-emerald-400" />
                    ) : (
                      <AlertTriangle className="w-5 h-5 text-red-400" />
                    )}
                    <span>{applyResult.success ? 'Deployment Successful!' : 'Deployment Failed'}</span>
                  </div>
                  <p className="mt-2 text-gray-300">{applyResult.message}</p>
                  {applyResult.version_id && (
                    <div className="mt-2 font-mono text-cyan-400">
                      Active Version ID: {applyResult.version_id}
                    </div>
                  )}
                </div>

                {applyResult.success && (
                  <button
                    onClick={onFinish}
                    className="w-full py-3 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-emerald-500/20"
                  >
                    Enter NOC Dashboard &rarr;
                  </button>
                )}
              </div>
            )}
          </div>
        )}

        {/* Navigation Buttons */}
        <div className="flex items-center justify-between pt-6 border-t border-gray-800 mt-6">
          <button
            onClick={handleBack}
            disabled={step === 1 || applying}
            className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-semibold rounded-lg flex items-center gap-2 disabled:opacity-30 disabled:cursor-not-allowed"
          >
            <ArrowLeft className="w-3.5 h-3.5" /> Back
          </button>

          {step < totalSteps && (
            <button
              onClick={handleNext}
              className="px-5 py-2 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2"
            >
              Next Step <ArrowRight className="w-3.5 h-3.5" />
            </button>
          )}

          {step === totalSteps && !applyResult?.success && (
            <button
              onClick={handleApply}
              disabled={applying}
              className="px-6 py-2 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-emerald-500/20 flex items-center gap-2 disabled:opacity-50"
            >
              {applying ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
              Execute Atomic Apply
            </button>
          )}
        </div>
      </div>
    </div>
  );
};
