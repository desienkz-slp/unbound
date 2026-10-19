import React, { useState } from 'react';
import {
  AlertTriangle,
  ArrowDown,
  ArrowRight,
  ArrowUp,
  Check,
  CheckCircle2,
  Copy,
  Edit3,
  ExternalLink,
  Globe,
  Layers,
  List,
  Lock,
  Network,
  Plus,
  RefreshCw,
  Save,
  Search,
  Server,
  Shield,
  ShieldCheck,
  Sparkles,
  Star,
  Trash2,
  Zap,
} from 'lucide-react';
import { ClientNetwork, DesiredState, Listener, UpstreamServer } from '../types';
import { api } from '../services/api';
import { DnsRewritesPanel } from '../components/DnsRewritesPanel';

export interface DoHProviderPreset {
  id: string;
  name: string;
  category: 'unfiltered' | 'security' | 'adblock' | 'family';
  categoryLabel: string;
  dohUrl: string;
  ip: string;
  port: number;
  type: 'DOT' | 'DOH' | 'UDP';
  tlsName: string;
  description: string;
  badge?: string;
}

export const DOH_PROVIDERS_CATALOG: DoHProviderPreset[] = [
  {
    id: 'cf-standard',
    name: 'Cloudflare Standard',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://cloudflare-dns.com/dns-query',
    ip: '1.1.1.1',
    port: 853,
    type: 'DOT',
    tlsName: 'cloudflare-dns.com',
    description: 'Resolusi ultra-cepat via Anycast global tanpa sensor. Efektif bypass pemblokiran port 53 ISP.',
    badge: 'Populer',
  },
  {
    id: 'cf-standard-secondary',
    name: 'Cloudflare Secondary',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://cloudflare-dns.com/dns-query',
    ip: '1.0.0.1',
    port: 853,
    type: 'DOT',
    tlsName: 'cloudflare-dns.com',
    description: 'Secondary IP Cloudflare Anycast berkecepatan tinggi dengan enkripsi DoT/DoH.',
  },
  {
    id: 'cf-security',
    name: 'Cloudflare Security',
    category: 'security',
    categoryLabel: 'Security & Anti-Malware',
    dohUrl: 'https://security.cloudflare-dns.com/dns-query',
    ip: '1.1.1.2',
    port: 853,
    type: 'DOT',
    tlsName: 'security.cloudflare-dns.com',
    description: 'Blokir otomatis domain berbahaya, malware, phishing, dan botnet tanpa menyentuh konten lain.',
  },
  {
    id: 'cf-family',
    name: 'Cloudflare Family',
    category: 'family',
    categoryLabel: 'Family & Konten Dewasa',
    dohUrl: 'https://family.cloudflare-dns.com/dns-query',
    ip: '1.1.1.3',
    port: 853,
    type: 'DOT',
    tlsName: 'family.cloudflare-dns.com',
    description: 'Proteksi keluarga dengan blokir otomatis konten dewasa/pornografi dan malware.',
  },
  {
    id: 'google-dns-primary',
    name: 'Google Public DNS Primary',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://dns.google/dns-query',
    ip: '8.8.8.8',
    port: 853,
    type: 'DOT',
    tlsName: 'dns.google',
    description: 'Infrastruktur anycast global Google. Cepat, stabil, tanpa sensor port 53.',
    badge: 'Google',
  },
  {
    id: 'google-dns-secondary',
    name: 'Google Public DNS Secondary',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://dns.google/dns-query',
    ip: '8.8.4.4',
    port: 853,
    type: 'DOT',
    tlsName: 'dns.google',
    description: 'Secondary Anycast Google Public DNS dengan proteksi DoT/DoH.',
  },
  {
    id: 'quad9-security',
    name: 'Quad9 Security (Malware Intel)',
    category: 'security',
    categoryLabel: 'Security & Anti-Malware',
    dohUrl: 'https://dns.quad9.net/dns-query',
    ip: '9.9.9.9',
    port: 853,
    type: 'DOT',
    tlsName: 'dns.quad9.net',
    description: 'DNS nirlaba berbasis Swiss dengan threat intelligence real-time untuk memblokir malware.',
    badge: 'Swiss',
  },
  {
    id: 'quad9-unsecured',
    name: 'Quad9 Unsecured (No Filter)',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://dns10.quad9.net/dns-query',
    ip: '9.9.9.10',
    port: 853,
    type: 'DOT',
    tlsName: 'dns10.quad9.net',
    description: 'Quad9 versi tanpa blocklist threat intel. Murni resolusi independen terenkripsi.',
  },
  {
    id: 'adguard-default',
    name: 'AdGuard DNS Default',
    category: 'adblock',
    categoryLabel: 'AdBlock & Tracker',
    dohUrl: 'https://dns.adguard-dns.com/dns-query',
    ip: '94.140.14.14',
    port: 853,
    type: 'DOT',
    tlsName: 'dns.adguard-dns.com',
    description: 'Blokir iklan web, tracker pelacak, dan domain phishing di level upstream resolver.',
    badge: 'AdGuard',
  },
  {
    id: 'adguard-unfiltered',
    name: 'AdGuard Non-filtering',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://unfiltered.adguard-dns.com/dns-query',
    ip: '94.140.14.140',
    port: 853,
    type: 'DOT',
    tlsName: 'unfiltered.adguard-dns.com',
    description: 'Resolver AdGuard tanpa filter iklan atau konten. Sangat privat, cepat, dan netral.',
  },
  {
    id: 'adguard-family',
    name: 'AdGuard Family Protection',
    category: 'family',
    categoryLabel: 'Family & Konten Dewasa',
    dohUrl: 'https://family.adguard-dns.com/dns-query',
    ip: '94.140.14.15',
    port: 853,
    type: 'DOT',
    tlsName: 'family.adguard-dns.com',
    description: 'Kombinasi blokir iklan + blokir domain dewasa (18+) + proteksi pencarian aman.',
  },
  {
    id: 'bebasdns-default',
    name: 'BebasDNS Indonesia Default',
    category: 'adblock',
    categoryLabel: 'AdBlock & Tracker',
    dohUrl: 'https://dns.bebasid.com/dns-query',
    ip: '103.87.68.194',
    port: 853,
    type: 'DOT',
    tlsName: 'dns.bebasid.com',
    description: 'Resolver publik netral Indonesia oleh Komunitas BebasID / KINI. Blokir iklan & malware.',
    badge: 'Lokal ID',
  },
  {
    id: 'bebasdns-unfiltered',
    name: 'BebasDNS Indonesia Unfiltered',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://dns.bebasid.com/unfiltered',
    ip: '103.87.68.194',
    port: 853,
    type: 'DOT',
    tlsName: 'unfiltered.dns.bebasid.com',
    description: 'BebasDNS murni tanpa filter apapun. Dirancang untuk internet netral dan bebas sensor ISP.',
    badge: 'Lokal ID',
  },
  {
    id: 'bebasdns-family',
    name: 'BebasDNS Internet Sehat',
    category: 'family',
    categoryLabel: 'Family & Konten Dewasa',
    dohUrl: 'https://internetsehat.bebasid.com/dns-query',
    ip: '103.87.68.196',
    port: 853,
    type: 'DOT',
    tlsName: 'internetsehat.bebasid.com',
    description: 'Varian BebasDNS ramah keluarga dengan filter konten pornografi dan judi online.',
    badge: 'Lokal ID',
  },
  {
    id: 'controld-uncensored',
    name: 'Control D Uncensored',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://freedns.controld.com/uncensored',
    ip: '76.76.2.0',
    port: 853,
    type: 'DOT',
    tlsName: 'freedns.controld.com',
    description: 'Resolver modern berkecepatan tinggi tanpa log dan tanpa sensor buatan Windscribe.',
  },
  {
    id: 'cleanbrowsing-security',
    name: 'CleanBrowsing Security',
    category: 'security',
    categoryLabel: 'Security & Anti-Malware',
    dohUrl: 'https://doh.cleanbrowsing.org/doh/security-filter/',
    ip: '185.228.168.9',
    port: 853,
    type: 'DOT',
    tlsName: 'security-filter-dns.cleanbrowsing.org',
    description: 'Memblokir phishing, malware, dan domain berbahaya tanpa menyentuh situs umum.',
  },
  {
    id: 'cleanbrowsing-adult',
    name: 'CleanBrowsing Adult Filter',
    category: 'family',
    categoryLabel: 'Family & Konten Dewasa',
    dohUrl: 'https://doh.cleanbrowsing.org/doh/adult-filter/',
    ip: '185.228.168.10',
    port: 853,
    type: 'DOT',
    tlsName: 'adult-filter-dns.cleanbrowsing.org',
    description: 'Filter ketat khusus memblokir domain dewasa/pornografi dan safe search paksa.',
  },
  {
    id: 'mullvad-public',
    name: 'Mullvad Public DNS',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://dns.mullvad.net/dns-query',
    ip: '194.242.2.2',
    port: 853,
    type: 'DOT',
    tlsName: 'dns.mullvad.net',
    description: 'DNS publik ramah privasi tinggi dari penyedia VPN Swedia Mullvad.',
  },
  {
    id: 'cisco-opendns',
    name: 'Cisco OpenDNS Standard',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://doh.opendns.com/dns-query',
    ip: '208.67.222.222',
    port: 853,
    type: 'DOT',
    tlsName: 'dns.opendns.com',
    description: 'Resolver tingkat enterprise oleh Cisco dengan uptime 100% dan latency rendah.',
  },
  {
    id: 'ali-dns',
    name: 'Ali DNS (Alibaba Cloud)',
    category: 'unfiltered',
    categoryLabel: 'Bypass Sensor & Unfiltered',
    dohUrl: 'https://dns.alidns.com/dns-query',
    ip: '223.5.5.5',
    port: 853,
    type: 'DOT',
    tlsName: 'dns.alidns.com',
    description: 'Infrastruktur Alibaba Cloud Anycast dengan rute peering optimal di kawasan Asia.',
  },
];

const ToggleSwitch: React.FC<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: string;
  size?: 'sm' | 'md';
  activeColor?: string;
}> = ({ checked, onChange, label, size = 'md', activeColor = 'bg-cyan-500' }) => {
  const isSm = size === 'sm';
  return (
    <div
      onClick={(e) => {
        e.stopPropagation();
        onChange(!checked);
      }}
      className="inline-flex items-center cursor-pointer select-none"
    >
      <div
        className={`relative inline-flex items-center transition-colors rounded-full ${
          isSm ? 'w-8 h-4.5' : 'w-11 h-6'
        } ${checked ? `${activeColor} shadow-md shadow-cyan-500/30` : 'bg-gray-800 border border-gray-700'}`}
      >
        <span
          className={`inline-block transition-transform transform bg-white rounded-full ${
            isSm ? 'w-3.5 h-3.5' : 'w-4.5 h-4.5'
          } ${
            checked ? (isSm ? 'translate-x-3.5' : 'translate-x-5.5') : (isSm ? 'translate-x-0.5' : 'translate-x-0.5')
          }`}
        />
      </div>
      {label && <span className="ml-2 text-xs font-medium text-gray-300">{label}</span>}
    </div>
  );
};

interface DNSConfigViewProps {
  nodeId: string;
  config: DesiredState | null;
  onConfigUpdated: (newConfig: DesiredState) => void;
}

export const DNSConfigView: React.FC<DNSConfigViewProps> = ({
  nodeId,
  config,
  onConfigUpdated,
}) => {
  const [subTab, setSubTab] = useState<'listeners' | 'networks' | 'upstreams' | 'cache' | 'dnssec' | 'rewrites'>('listeners');
  const [localState, setLocalState] = useState<DesiredState | null>(config);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [upstreamMode, setUpstreamMode] = useState<'table' | 'raw'>('table');
  const [rawText, setRawText] = useState('');
  const [testStatus, setTestStatus] = useState<Record<string, { status: 'testing' | 'ok' | 'fail'; latency?: number }>>({});
  const [providerSearch, setProviderSearch] = useState('');
  const [providerCategory, setProviderCategory] = useState<'all' | 'unfiltered' | 'security' | 'adblock' | 'family'>('all');
  const [copiedUrl, setCopiedUrl] = useState<string | null>(null);

  React.useEffect(() => {
    setLocalState(config);
  }, [config]);

  if (!localState) {
    return <div className="text-gray-400 p-8">Loading configuration state...</div>;
  }

  const handleApply = async () => {
    setSaving(true);
    setMessage(null);

    const activeUps = (localState.upstreams || []).filter((u) => u.enabled);
    if (localState.resolution_mode !== 'RECURSIVE') {
      if (activeUps.length === 0) {
        setMessage({ type: 'error', text: 'Mode Forwarding atau Hybrid memerlukan minimal 1 upstream resolver aktif.' });
        setSaving(false);
        return;
      }
      const hasTLS = activeUps.some((u) => u.type === 'DOT' || u.type === 'DOH' || u.port === 853 || !!u.tls_name);
      const hasPlain = activeUps.some((u) => !(u.type === 'DOT' || u.type === 'DOH' || u.port === 853 || !!u.tls_name));
      if (hasTLS && hasPlain) {
        setMessage({
          type: 'error',
          text: 'Pencampuran protokol terdeteksi: Semua upstream aktif harus menggunakan protokol yang seragam (seluruhnya UDP standar port 53 atau seluruhnya DoT terenkripsi port 853). Unbound tidak mendukung pencampuran TLS dan non-TLS dalam satu zona forwarder.',
        });
        setSaving(false);
        return;
      }
    }

    try {
      const payload: DesiredState = {
        ...localState,
        node_id: nodeId || localState.node_id,
        version_id: `v-${Date.now()}`,
        resolution_mode: localState.resolution_mode || 'RECURSIVE',
        listeners: localState.listeners || [],
      };
      const res = await api.applyConfig(nodeId, payload, 'Updated via Netora Web UI');
      setMessage({ type: 'success', text: `Configuration applied atomically! Version ID: ${res.version_id}` });
      onConfigUpdated(payload);
    } catch (err: any) {
      setMessage({ type: 'error', text: err.message || 'Failed to apply configuration' });
    } finally {
      setSaving(false);
    }
  };

  // Helper updates
  const updateListener = (index: number, updated: Partial<Listener>) => {
    const list = [...(localState.listeners || [])];
    list[index] = { ...list[index], ...updated };
    setLocalState({ ...localState, listeners: list });
  };

  const addListener = () => {
    const newL: Listener = {
      id: `listener-${Date.now()}`,
      name: 'New Listener',
      ip: '0.0.0.0',
      port: 53,
      role: 'CLIENT',
      is_ipv6: false,
      udp: true,
      tcp: true,
      enabled: true,
    };
    setLocalState({ ...localState, listeners: [...(localState.listeners || []), newL] });
  };

  const removeListener = (index: number) => {
    const list = (localState.listeners || []).filter((_, i) => i !== index);
    setLocalState({ ...localState, listeners: list });
  };

  // Client Networks
  const updateNetwork = (index: number, updated: Partial<ClientNetwork>) => {
    const list = [...localState.client_networks];
    list[index] = { ...list[index], ...updated };
    setLocalState({ ...localState, client_networks: list });
  };

  const addNetwork = () => {
    const newN: ClientNetwork = {
      id: `net-${Date.now()}`,
      cidr: '192.168.1.0/24',
      description: 'Internal Client Subnet',
      action: 'allow',
      policy_group: 'DEFAULT',
      enabled: true,
    };
    setLocalState({ ...localState, client_networks: [...localState.client_networks, newN] });
  };

  const removeNetwork = (index: number) => {
    const list = localState.client_networks.filter((_, i) => i !== index);
    setLocalState({ ...localState, client_networks: list });
  };

  // Upstreams Management
  const updateUpstream = (index: number, updated: Partial<UpstreamServer>) => {
    const list = [...localState.upstreams];
    list[index] = { ...list[index], ...updated };
    setLocalState({ ...localState, upstreams: list });
  };

  const addUpstream = (preset?: Partial<UpstreamServer>) => {
    const newUp: UpstreamServer = {
      id: `up-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
      name: preset?.name || 'Custom Upstream',
      ip: preset?.ip || '1.1.1.1',
      port: preset?.port || 853,
      type: preset?.type || 'DOT',
      tls_name: preset?.tls_name || '',
      doh_url: preset?.doh_url || '',
      enabled: preset?.enabled ?? true,
    };
    setLocalState({
      ...localState,
      upstreams: [...localState.upstreams, newUp],
      resolution_mode: localState.resolution_mode === 'RECURSIVE' ? 'HYBRID' : localState.resolution_mode,
    });
  };

  const moveUpstream = (index: number, direction: 'up' | 'down') => {
    const list = [...localState.upstreams];
    const targetIdx = direction === 'up' ? index - 1 : index + 1;
    if (targetIdx < 0 || targetIdx >= list.length) return;
    const temp = list[index];
    list[index] = list[targetIdx];
    list[targetIdx] = temp;
    setLocalState({ ...localState, upstreams: list });
  };

  const applyISPRecommendedPresets = () => {
    const recommended: UpstreamServer[] = [
      {
        id: `up-goog-dot-1`,
        name: 'Google Public DNS Primary (ECS CDN)',
        ip: '8.8.8.8',
        port: 853,
        type: 'DOT',
        tls_name: 'dns.google',
        enabled: true,
      },
      {
        id: `up-cf-dot-1`,
        name: 'Cloudflare Standard (Jakarta Anycast)',
        ip: '1.1.1.1',
        port: 853,
        type: 'DOT',
        tls_name: 'cloudflare-dns.com',
        enabled: true,
      },
      {
        id: `up-goog-dot-2`,
        name: 'Google Public DNS Secondary',
        ip: '8.8.4.4',
        port: 853,
        type: 'DOT',
        tls_name: 'dns.google',
        enabled: true,
      },
      {
        id: `up-cf-dot-2`,
        name: 'Cloudflare Secondary',
        ip: '1.0.0.1',
        port: 853,
        type: 'DOT',
        tls_name: 'cloudflare-dns.com',
        enabled: true,
      },
    ];
    setLocalState({
      ...localState,
      upstreams: recommended,
      resolution_mode: localState.resolution_mode === 'RECURSIVE' ? 'HYBRID' : localState.resolution_mode,
    });
    setMessage({
      type: 'success',
      text: 'Rekomendasi ISP CDN Terdekat berhasil diterapkan! Google DoT (8.8.8.8) diatur sebagai PRIMARY (ECS), dan Cloudflare DoT (1.1.1.1) sebagai SECONDARY. Klik "Save & Atomic Apply" di kanan atas.',
    });
  };

  const isProviderActive = (preset: DoHProviderPreset) => {
    return localState.upstreams.some(
      (u) =>
        u.enabled &&
        (u.ip === preset.ip || (preset.tlsName && u.tls_name === preset.tlsName)) &&
        (u.port === preset.port || (!u.port && preset.port === 853))
    );
  };

  const toggleProvider = (preset: DoHProviderPreset) => {
    const existingIdx = localState.upstreams.findIndex(
      (u) =>
        (u.ip === preset.ip || (preset.tlsName && u.tls_name === preset.tlsName)) &&
        (u.port === preset.port || (!u.port && preset.port === 853))
    );

    let newUpstreams = [...localState.upstreams];
    if (existingIdx >= 0) {
      const current = newUpstreams[existingIdx];
      newUpstreams[existingIdx] = {
        ...current,
        enabled: !current.enabled,
        tls_name: preset.tlsName,
        doh_url: preset.dohUrl,
      };
    } else {
      newUpstreams.push({
        id: `up-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
        name: preset.name,
        ip: preset.ip,
        port: preset.port,
        type: preset.type,
        tls_name: preset.tlsName,
        doh_url: preset.dohUrl,
        enabled: true,
      });
    }

    setLocalState({
      ...localState,
      upstreams: newUpstreams,
      resolution_mode: localState.resolution_mode === 'RECURSIVE' ? 'HYBRID' : localState.resolution_mode,
    });
  };

  const handleCopyUrl = (url: string) => {
    navigator.clipboard.writeText(url);
    setCopiedUrl(url);
    setTimeout(() => setCopiedUrl(null), 2000);
  };

  const handleToggleAllCategory = (cat: string, enable: boolean) => {
    const targets = DOH_PROVIDERS_CATALOG.filter((p) => cat === 'all' || p.category === cat);
    let newUpstreams = [...localState.upstreams];

    targets.forEach((preset) => {
      const existingIdx = newUpstreams.findIndex(
        (u) =>
          (u.ip === preset.ip || (preset.tlsName && u.tls_name === preset.tlsName)) &&
          (u.port === preset.port || (!u.port && preset.port === 853))
      );
      if (existingIdx >= 0) {
        newUpstreams[existingIdx] = {
          ...newUpstreams[existingIdx],
          enabled: enable,
          tls_name: preset.tlsName,
          doh_url: preset.dohUrl,
        };
      } else if (enable) {
        newUpstreams.push({
          id: `up-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
          name: preset.name,
          ip: preset.ip,
          port: preset.port,
          type: preset.type,
          tls_name: preset.tlsName,
          doh_url: preset.dohUrl,
          enabled: true,
        });
      }
    });

    setLocalState({
      ...localState,
      upstreams: newUpstreams,
      resolution_mode: enable && localState.resolution_mode === 'RECURSIVE' ? 'HYBRID' : localState.resolution_mode,
    });
  };

  const removeUpstream = (index: number) => {
    const list = localState.upstreams.filter((_, i) => i !== index);
    setLocalState({ ...localState, upstreams: list });
  };

  const testUpstream = async (up: UpstreamServer) => {
    setTestStatus((prev) => ({ ...prev, [up.id]: { status: 'testing' } }));
    const startTime = performance.now();
    try {
      await api.getNodeHealth(nodeId);
      const latency = Math.round(performance.now() - startTime);
      setTestStatus((prev) => ({ ...prev, [up.id]: { status: 'ok', latency: Math.max(1, latency) } }));
    } catch {
      setTestStatus((prev) => ({ ...prev, [up.id]: { status: 'fail' } }));
    }
  };

  const handleParseRawText = () => {
    const lines = rawText.split('\n').map((l) => l.trim()).filter((l) => l.length > 0 && !l.startsWith('#'));
    if (lines.length === 0) return;
    const parsed: UpstreamServer[] = lines.map((line, idx) => {
      let ip = line;
      let port = 53;
      if (line.includes(':')) {
        const parts = line.split(':');
        ip = parts[0];
        port = parseInt(parts[1], 10) || 53;
      }
      return {
        id: `up-raw-${Date.now()}-${idx}`,
        name: `Upstream (${ip})`,
        ip,
        port,
        type: 'UDP',
        enabled: true,
      };
    });
    setLocalState({ ...localState, upstreams: [...localState.upstreams, ...parsed] });
    setUpstreamMode('table');
    setRawText('');
  };

  return (
    <div className="space-y-6">
      {/* Title & Actions Bar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Network className="w-5 h-5 text-cyan-400" />
            DNS Configuration & Runtime Sockets
          </h1>
                  </div>

        <button
          onClick={handleApply}
          disabled={saving}
          className="px-5 py-2.5 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2 disabled:opacity-50"
        >
          {saving ? (
            <RefreshCw className="w-4 h-4 animate-spin" />
          ) : (
            <Save className="w-4 h-4" />
          )}
          {saving ? 'Validating & Applying...' : 'Save & Atomic Apply'}
        </button>
      </div>

      {message && (
        <div
          className={`p-4 rounded-xl border text-xs flex items-center gap-2 ${
            message.type === 'success'
              ? 'bg-emerald-950/50 border-emerald-800 text-emerald-300'
              : 'bg-red-950/50 border-red-800 text-red-300'
          }`}
        >
          {message.type === 'success' ? (
            <CheckCircle2 className="w-4 h-4 shrink-0" />
          ) : (
            <AlertTriangle className="w-4 h-4 shrink-0" />
          )}
          <span>{message.text}</span>
        </div>
      )}

      {/* Tabs */}
      <div className="flex border-b border-gray-800 gap-2 text-xs font-medium">
        <button
          onClick={() => setSubTab('listeners')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'listeners'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <Server className="w-3.5 h-3.5" />
          Multi-IP Listeners ({localState.listeners.length})
        </button>

        <button
          onClick={() => setSubTab('networks')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'networks'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <Layers className="w-3.5 h-3.5" />
          Client ACL Networks ({localState.client_networks.length})
        </button>

        <button
          onClick={() => setSubTab('upstreams')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'upstreams'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <Globe className="w-3.5 h-3.5" />
          Upstream Resolvers ({localState.upstreams.length})
        </button>

        <button
          onClick={() => setSubTab('cache')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'cache'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <Zap className="w-3.5 h-3.5" />
          Cache & Prefetch
        </button>

        <button
          onClick={() => setSubTab('dnssec')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'dnssec'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <ShieldCheck className="w-3.5 h-3.5" />
          DNSSEC Validation
        </button>

        <button
          onClick={() => setSubTab('rewrites')}
          className={`px-4 py-2.5 border-b-2 transition-all flex items-center gap-2 ${
            subTab === 'rewrites'
              ? 'border-cyan-400 text-cyan-400 bg-cyan-950/20'
              : 'border-transparent text-gray-400 hover:text-gray-200'
          }`}
        >
          <ArrowRight className="w-3.5 h-3.5" />
          DNS Rewrites
          {(localState.local_records?.length ?? 0) > 0 && (
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded-full bg-cyan-900/60 text-cyan-300 border border-cyan-800">
              {localState.local_records.length}
            </span>
          )}
        </button>
      </div>

      {/* Subtab 1: Listeners */}
      {subTab === 'listeners' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-sm font-bold text-white" title="Define IP bindings and explicit roles (INTERNET, CLIENT, MANAGEMENT, INTERNAL) to prevent accidental public exposure.">Listener Sockets & Roles</h2>
            </div>
            <button
              onClick={addListener}
              className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-cyan-400 text-xs font-semibold rounded-lg border border-gray-700 flex items-center gap-1.5"
            >
              <Plus className="w-3.5 h-3.5" /> Add Listener
            </button>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs font-mono">
              <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
                <tr>
                  <th className="py-2.5 px-3">Role</th>
                  <th className="py-2.5 px-3">IP Address</th>
                  <th className="py-2.5 px-3">Port</th>
                  <th className="py-2.5 px-3">Protocol</th>
                  <th className="py-2.5 px-3">Family</th>
                  <th className="py-2.5 px-3">Enabled</th>
                  <th className="py-2.5 px-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800/60">
                {localState.listeners.map((l, idx) => (
                  <tr key={l.id} className="hover:bg-gray-800/30">
                    <td className="py-2.5 px-3">
                      <select
                        value={l.role}
                        onChange={(e) => updateListener(idx, { role: e.target.value as any })}
                        className={`px-2 py-1 rounded text-xs font-bold border ${
                          l.role === 'CLIENT'
                            ? 'bg-cyan-950 text-cyan-400 border-cyan-800'
                            : l.role === 'INTERNET'
                            ? 'bg-amber-950 text-amber-400 border-amber-800'
                            : l.role === 'INTERNAL'
                            ? 'bg-emerald-950 text-emerald-400 border-emerald-800'
                            : 'bg-purple-950 text-purple-400 border-purple-800'
                        }`}
                      >
                        <option value="CLIENT">CLIENT</option>
                        <option value="INTERNET">INTERNET</option>
                        <option value="INTERNAL">INTERNAL</option>
                        <option value="MANAGEMENT">MANAGEMENT</option>
                      </select>
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="text"
                        value={l.ip}
                        onChange={(e) => updateListener(idx, { ip: e.target.value })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-white text-xs w-36"
                      />
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="number"
                        value={l.port}
                        onChange={(e) => updateListener(idx, { port: parseInt(e.target.value) || 53 })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-white text-xs w-20"
                      />
                    </td>
                    <td className="py-2.5 px-3 text-gray-300">
                      <label className="mr-2 inline-flex items-center gap-1 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={l.udp}
                          onChange={(e) => updateListener(idx, { udp: e.target.checked })}
                          className="rounded bg-gray-950 border-gray-700 text-cyan-500"
                        />
                        UDP
                      </label>
                      <label className="inline-flex items-center gap-1 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={l.tcp}
                          onChange={(e) => updateListener(idx, { tcp: e.target.checked })}
                          className="rounded bg-gray-950 border-gray-700 text-cyan-500"
                        />
                        TCP
                      </label>
                    </td>
                    <td className="py-2.5 px-3">
                      <span className="text-gray-400">{l.is_ipv6 ? 'IPv6' : 'IPv4'}</span>
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="checkbox"
                        checked={l.enabled}
                        onChange={(e) => updateListener(idx, { enabled: e.target.checked })}
                        className="rounded bg-gray-950 border-gray-700 text-cyan-500 cursor-pointer"
                      />
                    </td>
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => removeListener(idx)}
                        disabled={localState.listeners.length <= 1}
                        className="p-1 text-gray-500 hover:text-red-400 disabled:opacity-30"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Subtab 2: Client ACL Networks */}
      {subTab === 'networks' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-sm font-bold text-white" title="Only explicitly declared client subnets can query Unbound. 0.0.0.0/0 allow is blocked by security invariant.">Client CIDR Networks & Access Control</h2>
            </div>
            <button
              onClick={addNetwork}
              className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-cyan-400 text-xs font-semibold rounded-lg border border-gray-700 flex items-center gap-1.5"
            >
              <Plus className="w-3.5 h-3.5" /> Add Network CIDR
            </button>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs font-mono">
              <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
                <tr>
                  <th className="py-2.5 px-3">CIDR Prefix</th>
                  <th className="py-2.5 px-3">Description</th>
                  <th className="py-2.5 px-3">Policy Group</th>
                  <th className="py-2.5 px-3">Action</th>
                  <th className="py-2.5 px-3">Enabled</th>
                  <th className="py-2.5 px-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800/60">
                {localState.client_networks.map((net, idx) => (
                  <tr key={net.id} className="hover:bg-gray-800/30">
                    <td className="py-2.5 px-3">
                      <input
                        type="text"
                        value={net.cidr}
                        onChange={(e) => updateNetwork(idx, { cidr: e.target.value })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-cyan-300 font-bold text-xs w-44"
                      />
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="text"
                        value={net.description}
                        onChange={(e) => updateNetwork(idx, { description: e.target.value })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-gray-300 text-xs w-52"
                      />
                    </td>
                    <td className="py-2.5 px-3">
                      <select
                        value={net.policy_group}
                        onChange={(e) => updateNetwork(idx, { policy_group: e.target.value })}
                        className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-gray-300 text-xs"
                      >
                        <option value="DEFAULT">DEFAULT</option>
                        <option value="FAMILY">FAMILY</option>
                        <option value="STRICT_ISP">STRICT_ISP</option>
                        <option value="UNFILTERED">UNFILTERED</option>
                      </select>
                    </td>
                    <td className="py-2.5 px-3">
                      <span className="px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 font-bold border border-emerald-800">
                        {net.action}
                      </span>
                    </td>
                    <td className="py-2.5 px-3">
                      <input
                        type="checkbox"
                        checked={net.enabled}
                        onChange={(e) => updateNetwork(idx, { enabled: e.target.checked })}
                        className="rounded bg-gray-950 border-gray-700 text-cyan-500 cursor-pointer"
                      />
                    </td>
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => removeNetwork(idx)}
                        disabled={localState.client_networks.length <= 1}
                        className="p-1 text-gray-500 hover:text-red-400 disabled:opacity-30"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Subtab 3: Upstream Resolvers */}
      {subTab === 'upstreams' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-5">
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div>
              <h2 className="text-sm font-bold text-white flex items-center gap-2">
                <Globe className="w-4 h-4 text-cyan-400" />
                Upstream Forwarders & Resolution Architecture
              </h2>
                          </div>

            <div className="flex items-center gap-2">
              <button
                onClick={() => setUpstreamMode(upstreamMode === 'table' ? 'raw' : 'table')}
                className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-medium rounded-lg border border-gray-700 flex items-center gap-1.5 transition-colors"
              >
                {upstreamMode === 'table' ? (
                  <>
                    <Edit3 className="w-3.5 h-3.5 text-cyan-400" /> Bulk Paste (AdGuard Style)
                  </>
                ) : (
                  <>
                    <List className="w-3.5 h-3.5 text-cyan-400" /> Table Editor
                  </>
                )}
              </button>

              <button
                onClick={() => addUpstream()}
                className="px-3 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-black text-xs font-bold rounded-lg flex items-center gap-1.5 shadow-lg shadow-cyan-600/20 transition-colors"
              >
                <Plus className="w-3.5 h-3.5" /> Add Upstream
              </button>
            </div>
          </div>

          {/* Mode Selector */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pb-2 border-b border-gray-800">
            {(['RECURSIVE', 'FORWARDING', 'HYBRID'] as const).map((m) => (
              <div
                key={m}
                onClick={() => setLocalState({ ...localState, resolution_mode: m })}
                className={`p-4 rounded-xl border cursor-pointer transition-all ${
                  localState.resolution_mode === m
                    ? 'bg-cyan-950/30 border-cyan-500 text-white shadow-lg shadow-cyan-950/40'
                    : 'bg-gray-950/60 border-gray-800 text-gray-400 hover:border-gray-700'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="font-bold font-mono text-sm text-cyan-400">{m}</div>
                  {localState.resolution_mode === m && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-300 border border-cyan-800">
                      ACTIVE
                    </span>
                  )}
                </div>
                <p className="text-[11px] mt-1.5 text-gray-400">
                  {m === 'RECURSIVE' && 'Direct iterative root-hints resolution; no upstream forwarders used.'}
                  {m === 'FORWARDING' && 'Forward all queries to specified high-speed upstream DNS servers.'}
                  {m === 'HYBRID' && 'Forward primary queries with automatic iterative fallback on failure.'}
                </p>
              </div>
            ))}
          </div>

          {/* DoH / Encrypted DNS Providers Directory & Toggle Card */}
          <div className="bg-gray-950/80 border border-gray-800 rounded-xl p-5 space-y-4">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 pb-3 border-b border-gray-800/80">
              <div className="flex items-center gap-2.5">
                <div className="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center shrink-0 shadow-inner">
                  <Shield className="w-5 h-5 text-cyan-400" />
                </div>
                <div>
                  <div className="text-sm font-bold text-white flex items-center gap-2">
                    Katalog Upstream DoH / DoT (AdGuard Providers Directory)
                    <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-cyan-900/60 text-cyan-300 border border-cyan-800">
                      Enkripsi TLS Port 853
                    </span>
                  </div>
                                  </div>
              </div>

              <div className="flex items-center gap-2">
                <a
                  href="https://adguard-dns.io/kb/general/dns-providers/?clid=y5dJ5x&utm_campaign=dns_kb_providers&utm_medium=ui&utm_source=home"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="px-3 py-1.5 bg-gray-900 hover:bg-gray-800 text-cyan-400 text-xs font-semibold rounded-lg border border-cyan-900/80 flex items-center gap-1.5 transition-colors"
                >
                  <ExternalLink className="w-3.5 h-3.5" />
                  <span>AdGuard KB</span>
                </a>
              </div>
            </div>

            {/* Filter and Search Bar */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 pt-1">
              <div className="flex flex-wrap items-center gap-1.5">
                {[
                  { id: 'all', label: `Semua (${DOH_PROVIDERS_CATALOG.length})` },
                  { id: 'unfiltered', label: 'Bypass Sensor & Unfiltered' },
                  { id: 'security', label: 'Security & Anti-Malware' },
                  { id: 'adblock', label: 'AdBlock & Tracker' },
                  { id: 'family', label: 'Family & Dewasa' },
                ].map((tab) => (
                  <button
                    key={tab.id}
                    onClick={() => setProviderCategory(tab.id as any)}
                    className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                      providerCategory === tab.id
                        ? 'bg-cyan-600 text-black font-bold shadow-md shadow-cyan-600/30'
                        : 'bg-gray-900 hover:bg-gray-800 text-gray-300 border border-gray-800'
                    }`}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>

              <div className="relative w-full sm:w-64">
                <Search className="w-3.5 h-3.5 absolute left-3 top-2.5 text-gray-500" />
                <input
                  type="text"
                  placeholder="Cari provider, IP, DoH URL..."
                  value={providerSearch}
                  onChange={(e) => setProviderSearch(e.target.value)}
                  className="w-full bg-gray-900 border border-gray-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-cyan-300 placeholder-gray-500 focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>

            {/* Quick Bulk Toggle Actions */}
            <div className="flex items-center justify-between text-xs text-gray-400 pt-1">
              <span>
                Menampilkan{' '}
                <strong className="text-white">
                  {
                    DOH_PROVIDERS_CATALOG.filter((p) => {
                      const matchCat = providerCategory === 'all' || p.category === providerCategory;
                      const matchQ =
                        !providerSearch ||
                        p.name.toLowerCase().includes(providerSearch.toLowerCase()) ||
                        p.ip.includes(providerSearch) ||
                        p.dohUrl.toLowerCase().includes(providerSearch.toLowerCase());
                      return matchCat && matchQ;
                    }).length
                  }
                </strong>{' '}
                resolver DoH / DoT
              </span>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => handleToggleAllCategory(providerCategory, true)}
                  className="text-[11px] text-cyan-400 hover:text-cyan-300 hover:underline"
                >
                  Aktifkan Semua Terfilter
                </button>
                <span className="text-gray-600">•</span>
                <button
                  onClick={() => handleToggleAllCategory(providerCategory, false)}
                  className="text-[11px] text-gray-400 hover:text-gray-300 hover:underline"
                >
                  Nonaktifkan Semua Terfilter
                </button>
              </div>
            </div>

            {/* Provider Grid Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
              {DOH_PROVIDERS_CATALOG.filter((p) => {
                const matchCat = providerCategory === 'all' || p.category === providerCategory;
                const matchQ =
                  !providerSearch ||
                  p.name.toLowerCase().includes(providerSearch.toLowerCase()) ||
                  p.ip.includes(providerSearch) ||
                  p.dohUrl.toLowerCase().includes(providerSearch.toLowerCase());
                return matchCat && matchQ;
              }).map((preset) => {
                const active = isProviderActive(preset);
                return (
                  <div
                    key={preset.id}
                    onClick={() => toggleProvider(preset)}
                    className={`p-3.5 rounded-xl border transition-all cursor-pointer select-none flex flex-col justify-between ${
                      active
                        ? 'bg-cyan-950/20 border-cyan-500/80 shadow-md shadow-cyan-950/30'
                        : 'bg-gray-900/60 border-gray-800/80 hover:border-gray-700'
                    }`}
                  >
                    <div>
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <div className="flex items-center gap-1.5">
                            <span className="text-xs font-bold text-white font-sans">{preset.name}</span>
                            {preset.badge && (
                              <span className="text-[9px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-800 font-mono">
                                {preset.badge}
                              </span>
                            )}
                          </div>
                          <span
                            className={`inline-block text-[10px] mt-1 px-2 py-0.5 rounded-full font-medium ${
                              preset.category === 'unfiltered'
                                ? 'bg-blue-950 text-blue-300 border border-blue-800'
                                : preset.category === 'security'
                                ? 'bg-amber-950 text-amber-300 border border-amber-800'
                                : preset.category === 'adblock'
                                ? 'bg-purple-950 text-purple-300 border border-purple-800'
                                : 'bg-rose-950 text-rose-300 border border-rose-800'
                            }`}
                          >
                            {preset.categoryLabel}
                          </span>
                        </div>

                        {/* Toggle Switch */}
                        <div className="shrink-0 pt-0.5">
                          <ToggleSwitch checked={active} onChange={() => toggleProvider(preset)} size="md" />
                        </div>
                      </div>

                      <p className="text-[11px] text-gray-400 mt-2.5 line-clamp-2 leading-relaxed">
                        {preset.description}
                      </p>
                    </div>

                    <div className="mt-3 pt-2.5 border-t border-gray-800/80 space-y-1.5 text-[11px] font-mono">
                      {/* DoH URL with copy button */}
                      <div className="flex items-center justify-between text-gray-400 bg-gray-950/80 px-2 py-1 rounded border border-gray-800">
                        <span className="truncate text-cyan-300" title={preset.dohUrl}>
                          {preset.dohUrl}
                        </span>
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleCopyUrl(preset.dohUrl);
                          }}
                          className="ml-1 text-gray-500 hover:text-cyan-400 p-0.5"
                          title="Salin DoH URL"
                        >
                          {copiedUrl === preset.dohUrl ? (
                            <Check className="w-3 h-3 text-emerald-400" />
                          ) : (
                            <Copy className="w-3 h-3" />
                          )}
                        </button>
                      </div>

                      {/* DoT IP & SNI */}
                      <div className="flex items-center justify-between text-[10px] text-gray-500">
                        <span>
                          IP: <strong className="text-gray-300">{preset.ip}</strong>:853
                        </span>
                        <span className="truncate text-gray-400" title={preset.tlsName}>
                          SNI: {preset.tlsName}
                        </span>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Mode Notification if Recursive */}
          {localState.resolution_mode === 'RECURSIVE' && (
            <div className="p-3 rounded-lg bg-amber-950/30 border border-amber-800/60 text-amber-300 text-xs flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 shrink-0 text-amber-400" />
                <span>
                  <strong>Mode RECURSIVE sedang aktif:</strong> Unbound mencari langsung via Root Hints IANA. Upstream di bawah ini akan aktif otomatis jika mode diubah ke <strong>FORWARDING</strong> atau <strong>HYBRID</strong>.
                </span>
              </div>
              <button
                onClick={() => setLocalState({ ...localState, resolution_mode: 'HYBRID' })}
                className="px-2.5 py-1 bg-amber-900/60 hover:bg-amber-800 text-amber-200 text-xs font-bold rounded shrink-0 border border-amber-700 transition-colors"
              >
                Aktifkan Mode HYBRID
              </button>
            </div>
          )}

          {/* Active Upstream Resolvers Table & Editor */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-bold text-gray-300 uppercase tracking-wider flex items-center gap-2">
                <Server className="w-3.5 h-3.5 text-cyan-400" />
                Daftar Upstream Aktif ({localState.upstreams.filter((u) => u.enabled).length} Aktif dari {localState.upstreams.length})
              </h3>
            </div>

            {upstreamMode === 'raw' ? (
              <div className="space-y-3 bg-gray-950 p-4 rounded-xl border border-gray-800">
                <div className="flex items-center justify-between">
                  <label className="text-xs font-semibold text-gray-300">
                    Paste Upstream DNS Servers (1 IP address / hostname per line)
                  </label>
                  <span className="text-[11px] text-gray-500 font-mono">Format: 1.1.1.1 or 8.8.8.8:53</span>
                </div>
                <textarea
                  rows={6}
                  value={rawText}
                  onChange={(e) => setRawText(e.target.value)}
                  placeholder={"1.1.1.1\n1.0.0.1\n8.8.8.8\n9.9.9.9\n94.140.14.14"}
                  className="w-full bg-gray-900 border border-gray-800 rounded-lg p-3 text-xs font-mono text-cyan-300 focus:border-cyan-500 focus:outline-none"
                />
                <div className="flex items-center justify-between">
                                    <button
                    onClick={handleParseRawText}
                    className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-black text-xs font-bold rounded-lg shadow-md transition-colors"
                  >
                    Import to Upstream List
                  </button>
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                {/* Priority Guidance Toolbar */}
                <div className="flex flex-wrap items-center justify-between gap-3 p-3 bg-gradient-to-r from-gray-950 via-slate-900 to-blue-950/40 rounded-xl border border-cyan-900/30 shadow-md">
                  <div className="flex items-center gap-2.5 text-xs">
                    <div className="p-1.5 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-400">
                      <Sparkles className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="font-bold text-white flex items-center gap-1.5">
                        Hierarki Prioritas & Optimasi CDN Terdekat
                      </div>
                      <p className="text-[11px] text-gray-400 mt-0.5 font-sans">
                        Baris paling atas otomatis menjadi <strong className="text-amber-300">PRIMARY (Main DNS)</strong>. Gunakan tombol panah <span className="text-cyan-400 font-mono">⬆ / ⬇</span> untuk mengatur urutan. Gunakan Google DoT untuk rute CDN terdekat (Shopee, Netflix, YouTube) via ECS.
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={applyISPRecommendedPresets}
                    className="px-3 py-1.5 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black text-xs font-bold rounded-lg shadow-md transition-all flex items-center gap-1.5 shrink-0"
                  >
                    <Zap className="w-3.5 h-3.5 fill-black" />
                    Terapkan Rekomendasi CDN Terdekat (1-Klik)
                  </button>
                </div>

                {/* Protocol Conflict Warning Banner */}
                {(() => {
                  const activeUps = (localState.upstreams || []).filter((u) => u.enabled);
                  const hasTLS = activeUps.some((u) => u.type === 'DOT' || u.type === 'DOH' || u.port === 853 || !!u.tls_name);
                  const hasPlain = activeUps.some((u) => !(u.type === 'DOT' || u.type === 'DOH' || u.port === 853 || !!u.tls_name));
                  if (hasTLS && hasPlain && localState.resolution_mode !== 'RECURSIVE') {
                    return (
                      <div className="p-3 bg-rose-950/40 border border-rose-800/80 rounded-xl text-rose-200 text-xs flex flex-wrap items-center justify-between gap-3 shadow-md">
                        <div className="flex items-center gap-2.5">
                          <AlertTriangle className="w-5 h-5 text-rose-400 shrink-0" />
                          <div>
                            <div className="font-bold text-rose-100">Perhatian: Pencampuran Protokol Terdeteksi</div>
                            <div className="text-[11px] text-rose-300">
                              Daftar upstream aktif mencampur UDP standar (Port 53) dan DoT (Port 853). Unbound mewajibkan seluruh upstream aktif dalam satu zona menggunakan protokol yang seragam.
                            </div>
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={() => {
                              const updated = (localState.upstreams || []).map((u) => ({
                                ...u,
                                type: 'DOT' as const,
                                port: 853,
                                tls_name:
                                  u.tls_name ||
                                  (u.ip === '8.8.8.8' || u.ip === '8.8.4.4'
                                    ? 'dns.google'
                                    : u.ip === '1.1.1.1' || u.ip === '1.0.0.1'
                                    ? 'cloudflare-dns.com'
                                    : ''),
                              }));
                              setLocalState({ ...localState, upstreams: updated });
                            }}
                            className="px-2.5 py-1 bg-rose-900/80 hover:bg-rose-800 text-white rounded font-semibold text-xs transition-colors border border-rose-700"
                          >
                            Set Semua ke DoT (853)
                          </button>
                          <button
                            type="button"
                            onClick={() => {
                              const updated = (localState.upstreams || []).map((u) => ({
                                ...u,
                                type: 'UDP' as const,
                                port: 53,
                                tls_name: '',
                              }));
                              setLocalState({ ...localState, upstreams: updated });
                            }}
                            className="px-2.5 py-1 bg-gray-800 hover:bg-gray-700 text-gray-200 rounded font-semibold text-xs transition-colors border border-gray-700"
                          >
                            Set Semua ke UDP (53)
                          </button>
                        </div>
                      </div>
                    );
                  }
                  return null;
                })()}

                <div className="overflow-x-auto rounded-xl border border-gray-800">
                  <table className="w-full text-left text-xs font-mono">
                    <thead className="bg-gray-950 text-gray-400 border-b border-gray-800">
                      <tr>
                        <th className="py-2.5 px-3">Prioritas / Role</th>
                        <th className="py-2.5 px-3">Status</th>
                        <th className="py-2.5 px-3">Provider / Name & CDN Info</th>
                        <th className="py-2.5 px-3">IP Address</th>
                        <th className="py-2.5 px-3">Port</th>
                        <th className="py-2.5 px-3">Protocol</th>
                        <th className="py-2.5 px-3">TLS SNI Name</th>
                        <th className="py-2.5 px-3">Latency Probe</th>
                        <th className="py-2.5 px-3 text-right">Urutan & Actions</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-800/60 bg-gray-900/40">
                      {localState.upstreams.map((up, idx) => {
                        const test = testStatus[up.id];
                        let enabledRank = 0;
                        if (up.enabled) {
                          for (let i = 0; i <= idx; i++) {
                            if (localState.upstreams[i].enabled) enabledRank++;
                          }
                        }

                        const isGoogleOrECS = up.ip.startsWith('8.8.') || up.ip === '9.9.9.11';
                        const isCloudflare = up.ip === '1.1.1.1' || up.ip === '1.0.0.1';
                        const isPlainUDP53 = up.port === 53 && up.type === 'UDP';

                        return (
                          <tr key={up.id} className="hover:bg-gray-800/30 transition-colors">
                            {/* Prioritas / Role */}
                            <td className="py-2.5 px-3">
                              {up.enabled ? (
                                enabledRank === 1 ? (
                                  <span className="px-2 py-0.5 rounded bg-amber-500/20 text-amber-300 border border-amber-500/50 text-[10px] font-sans font-bold flex items-center gap-1 w-max shadow-sm">
                                    ⭐ PRIMARY (MAIN)
                                  </span>
                                ) : enabledRank === 2 ? (
                                  <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-200 border border-slate-600 text-[10px] font-sans font-bold flex items-center gap-1 w-max">
                                    🥈 SECONDARY
                                  </span>
                                ) : (
                                  <span className="px-2 py-0.5 rounded bg-blue-950/60 text-blue-300 border border-blue-800/60 text-[10px] font-sans font-medium flex items-center gap-1 w-max">
                                    🛡️ BACKUP #{enabledRank - 2}
                                  </span>
                                )
                              ) : (
                                <span className="px-2 py-0.5 rounded bg-gray-900 text-gray-500 border border-gray-800 text-[10px] font-sans">
                                  OFF (Standby)
                                </span>
                              )}
                            </td>

                            {/* Status Toggle */}
                            <td className="py-2.5 px-3">
                              <div className="flex items-center gap-2">
                                <ToggleSwitch
                                  checked={up.enabled}
                                  onChange={(val) => updateUpstream(idx, { enabled: val })}
                                  size="sm"
                                />
                                <span
                                  className={`text-[10px] font-sans font-bold px-1.5 py-0.5 rounded ${
                                    up.enabled
                                      ? 'bg-emerald-950/80 text-emerald-400 border border-emerald-800/80'
                                      : 'bg-gray-800 text-gray-400'
                                  }`}
                                >
                                  {up.enabled ? 'AKTIF' : 'OFF'}
                                </span>
                              </div>
                            </td>

                            {/* Provider / Name & CDN Info */}
                            <td className="py-2.5 px-3">
                              <input
                                type="text"
                                value={up.name}
                                onChange={(e) => updateUpstream(idx, { name: e.target.value })}
                                className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-white text-xs w-52 font-sans focus:border-cyan-500"
                                placeholder="Provider Name"
                              />
                              <div className="mt-1">
                                {isGoogleOrECS ? (
                                  <span className="inline-flex items-center gap-1 text-[10px] text-emerald-400 font-sans">
                                    <Sparkles className="w-2.5 h-2.5 text-emerald-400" /> CDN Optimal (ECS Enabled)
                                  </span>
                                ) : isCloudflare ? (
                                  <span className="inline-flex items-center gap-1 text-[10px] text-cyan-400 font-sans">
                                    ⚡ Fast Anycast (Jakarta POP)
                                  </span>
                                ) : isPlainUDP53 ? (
                                  <span className="inline-flex items-center gap-1 text-[10px] text-rose-400 font-sans">
                                    ⚠️ Rawan Dibajak ISP (Non-TLS Port 53)
                                  </span>
                                ) : null}
                              </div>
                            </td>

                            {/* IP Address */}
                            <td className="py-2.5 px-3">
                              <input
                                type="text"
                                value={up.ip}
                                onChange={(e) => updateUpstream(idx, { ip: e.target.value })}
                                className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-cyan-300 text-xs w-36 font-mono focus:border-cyan-500"
                                placeholder="1.1.1.1"
                              />
                            </td>

                            {/* Port */}
                            <td className="py-2.5 px-3">
                              <input
                                type="number"
                                value={up.port}
                                onChange={(e) => updateUpstream(idx, { port: parseInt(e.target.value, 10) || 53 })}
                                className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-white text-xs w-16 font-mono focus:border-cyan-500"
                              />
                            </td>

                            {/* Protocol */}
                            <td className="py-2.5 px-3">
                              <select
                                value={up.type}
                                onChange={(e) => {
                                  const val = e.target.value;
                                  const newPort = val === 'DOT' ? 853 : up.port === 853 ? 53 : up.port;
                                  const newTls =
                                    val === 'DOT' && !up.tls_name
                                      ? up.ip === '8.8.8.8' || up.ip === '8.8.4.4'
                                        ? 'dns.google'
                                        : up.ip === '1.1.1.1' || up.ip === '1.0.0.1'
                                        ? 'cloudflare-dns.com'
                                        : ''
                                      : up.tls_name;
                                  updateUpstream(idx, { type: val, port: newPort, tls_name: newTls });
                                }}
                                className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-xs text-gray-300 font-mono focus:border-cyan-500"
                              >
                                <option value="DOT">DoT (TLS)</option>
                                <option value="DOH">DoH (HTTPS)</option>
                                <option value="UDP">UDP (Standard)</option>
                                <option value="TCP">TCP</option>
                              </select>
                            </td>

                            {/* TLS SNI Name */}
                            <td className="py-2.5 px-3">
                              <input
                                type="text"
                                value={up.tls_name || ''}
                                onChange={(e) => updateUpstream(idx, { tls_name: e.target.value })}
                                className="bg-gray-950 border border-gray-800 rounded px-2 py-1 text-gray-300 text-xs w-44 font-mono focus:border-cyan-500"
                                placeholder="e.g. cloudflare-dns.com"
                              />
                            </td>

                            {/* Latency Probe */}
                            <td className="py-2.5 px-3">
                              {test?.status === 'testing' ? (
                                <span className="text-[11px] text-cyan-400 flex items-center gap-1 font-sans">
                                  <RefreshCw className="w-3 h-3 animate-spin" /> Testing...
                                </span>
                              ) : test?.status === 'ok' ? (
                                <span className="px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-800 text-[10px] font-mono">
                                  ✔ {test.latency}ms
                                </span>
                              ) : test?.status === 'fail' ? (
                                <span className="px-2 py-0.5 rounded bg-red-950 text-red-400 border border-red-800 text-[10px] font-mono">
                                  ✖ Timeout
                                </span>
                              ) : (
                                <button
                                  type="button"
                                  onClick={() => testUpstream(up)}
                                  className="px-2 py-0.5 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded text-[10px] font-sans border border-gray-700 transition-colors"
                                >
                                  Test Ping
                                </button>
                              )}
                            </td>

                            {/* Actions & Reordering */}
                            <td className="py-2.5 px-3 text-right">
                              <div className="flex items-center justify-end gap-1">
                                <button
                                  type="button"
                                  onClick={() => moveUpstream(idx, 'up')}
                                  disabled={idx === 0}
                                  className="p-1 rounded bg-gray-950 border border-gray-800 text-gray-400 hover:text-cyan-400 hover:border-cyan-600 disabled:opacity-20 disabled:hover:text-gray-400 transition-colors"
                                  title="Geser Naik (Prioritas Lebih Tinggi)"
                                >
                                  <ArrowUp className="w-3.5 h-3.5" />
                                </button>
                                <button
                                  type="button"
                                  onClick={() => moveUpstream(idx, 'down')}
                                  disabled={idx === localState.upstreams.length - 1}
                                  className="p-1 rounded bg-gray-950 border border-gray-800 text-gray-400 hover:text-cyan-400 hover:border-cyan-600 disabled:opacity-20 disabled:hover:text-gray-400 transition-colors"
                                  title="Geser Turun (Prioritas Lebih Rendah)"
                                >
                                  <ArrowDown className="w-3.5 h-3.5" />
                                </button>
                                <button
                                  type="button"
                                  onClick={() => removeUpstream(idx)}
                                  disabled={localState.upstreams.length <= 1}
                                  className="p-1 rounded bg-gray-950 border border-gray-800 text-gray-500 hover:text-red-400 hover:border-red-600 disabled:opacity-30 transition-colors ml-1"
                                  title="Hapus Upstream"
                                >
                                  <Trash2 className="w-3.5 h-3.5" />
                                </button>
                              </div>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>
        </div>
      )}


      {/* Subtab 4: Cache & Prefetch */}
      {subTab === 'cache' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div>
            <h2 className="text-sm font-bold text-white" title="Configure Unbound memory cache partitions, prefetch expired records, and serve expired answers during upstream outages.">Cache & Latency Tuning</h2>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono">
            <div>
              <label className="block text-gray-400 mb-1">RRset Cache Size</label>
              <input
                type="text"
                value={localState.cache.rrset_cache_size}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    cache: { ...localState.cache, rrset_cache_size: e.target.value },
                  })
                }
                className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
              />
            </div>

            <div>
              <label className="block text-gray-400 mb-1">Message Cache Size</label>
              <input
                type="text"
                value={localState.cache.msg_cache_size}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    cache: { ...localState.cache, msg_cache_size: e.target.value },
                  })
                }
                className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
              />
            </div>

            <div>
              <label className="block text-gray-400 mb-1">Min TTL (Seconds)</label>
              <input
                type="number"
                value={localState.cache.min_ttl}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    cache: { ...localState.cache, min_ttl: parseInt(e.target.value) || 0 },
                  })
                }
                className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
              />
            </div>

            <div>
              <label className="block text-gray-400 mb-1">Max TTL (Seconds)</label>
              <input
                type="number"
                value={localState.cache.max_ttl}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    cache: { ...localState.cache, max_ttl: parseInt(e.target.value) || 86400 },
                  })
                }
                className="w-full bg-gray-950 border border-gray-800 rounded px-3 py-2 text-white"
              />
            </div>
          </div>

          <div className="pt-4 border-t border-gray-800 space-y-3">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={localState.features.prefetch}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    features: { ...localState.features, prefetch: e.target.checked },
                  })
                }
                className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
              />
              <div>
                <span className="text-xs font-semibold text-white">Enable Cache Prefetch</span>
                              </div>
            </label>

            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={localState.features.serve_expired}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    features: { ...localState.features, serve_expired: e.target.checked },
                  })
                }
                className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
              />
              <div>
                <span className="text-xs font-semibold text-white">Enable Serve-Expired Resilience</span>
                              </div>
            </label>
          </div>
        </div>
      )}

      {/* Subtab 5: DNSSEC */}
      {subTab === 'dnssec' && (
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4">
          <div>
            <h2 className="text-sm font-bold text-white" title="Validates digital signatures (RRSIG, DNSKEY, DS) using the IANA root trust anchor.">Cryptographic DNSSEC Validation</h2>
          </div>

          <div className="space-y-4">
            <label className="flex items-center gap-3 cursor-pointer p-4 bg-gray-950 border border-gray-800 rounded-lg">
              <input
                type="checkbox"
                checked={localState.dnssec.enabled}
                onChange={(e) =>
                  setLocalState({
                    ...localState,
                    dnssec: { ...localState.dnssec, enabled: e.target.checked },
                  })
                }
                className="rounded bg-gray-950 border-gray-700 text-cyan-500 w-4 h-4"
              />
              <div>
                <span className="text-xs font-bold text-white">Enforce DNSSEC Validation</span>
                              </div>
            </label>

            <div className="p-4 bg-gray-950 border border-gray-800 rounded-lg space-y-2 text-xs font-mono">
              <div className="flex justify-between">
                <span className="text-gray-400">Auto Trust Anchor File:</span>
                <span className="text-white">{localState.dnssec.auto_trust_anchor_file}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-400">Root Hints File:</span>
                <span className="text-white">{localState.dnssec.root_hints_file}</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Subtab 6: DNS Rewrites */}
      {subTab === 'rewrites' && (
        <DnsRewritesPanel
          records={localState.local_records ?? []}
          onChange={(updated) =>
            setLocalState({ ...localState, local_records: updated })
          }
        />
      )}
    </div>
  );
};
