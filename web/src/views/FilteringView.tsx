import React, { useState, useEffect } from 'react';
import {
  Check,
  CheckCircle2,
  Copy,
  Dices,
  DownloadCloud,
  ExternalLink,
  Filter,
  GitBranch,
  Lock,
  Plus,
  RefreshCw,
  Search,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Trash2,
  Power,
  ToggleLeft,
  ToggleRight,
  Zap,
  ChevronDown,
  ChevronUp,
  Eye,
  EyeOff,
} from 'lucide-react';
import { AllowlistRecord, DesiredState, FilterRecord, AutoUpdateStatus } from '../types';
import { api } from '../services/api';

interface FilteringViewProps {
  config: DesiredState | null;
  nodeId: string;
  onConfigUpdated?: (newCfg: DesiredState) => void;
}

const PRESET_FEEDS = [
  {
    name: 'Hagezi Multi PRO (Recommended)',
    category: 'MALWARE_ADS',
    url: 'https://raw.githubusercontent.com/hagezi/dns-blocklists/main/adblock/pro.txt',
  },
  {
    name: 'AdGuard DNS Filter',
    category: 'ADS_TRACKING',
    url: 'https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt',
  },
  {
    name: 'Parental Control — Adult Content (18+)',
    category: 'ADULT',
    url: 'https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/porn/hosts',
  },
  {
    name: 'Judi Online & Gambling Hosts',
    category: 'GAMBLING',
    url: 'https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/gambling/hosts',
  },
  {
    name: 'StevenBlack Unified Hosts',
    category: 'MALWARE_ADS',
    url: 'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts',
  },
  {
    name: 'OISD Blocklist Big',
    category: 'TELEMETRY_ADS',
    url: 'https://big.oisd.nl',
  },
];

// Reusable animated ToggleSwitch Component
const ToggleSwitch: React.FC<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  size?: 'sm' | 'md';
  disabled?: boolean;
}> = ({ checked, onChange, size = 'sm', disabled = false }) => {
  const isSm = size === 'sm';
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={(e) => {
        e.stopPropagation();
        if (!disabled) onChange(!checked);
      }}
      className={`relative inline-flex items-center rounded-full transition-colors focus:outline-none ${
        disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
      } ${
        isSm ? 'w-9 h-5' : 'w-11 h-6'
      } ${
        checked
          ? 'bg-gradient-to-r from-cyan-500 to-emerald-500 shadow-sm shadow-cyan-500/30'
          : 'bg-gray-800 border border-gray-700 hover:bg-gray-700'
      }`}
    >
      <span
        className={`inline-block transform rounded-full bg-white transition-transform shadow-md ${
          isSm ? 'w-3.5 h-3.5' : 'w-4 h-4'
        } ${
          checked
            ? isSm
              ? 'translate-x-4.5 bg-white'
              : 'translate-x-6 bg-white'
            : isSm
            ? 'translate-x-1 bg-gray-400'
            : 'translate-x-1 bg-gray-400'
        }`}
      />
    </button>
  );
};

export const FilteringView: React.FC<FilteringViewProps> = ({ config, nodeId, onConfigUpdated }) => {
  const [showRawFeeds, setShowRawFeeds] = useState(false);
  const [filters, setFilters] = useState<FilterRecord[]>([]);
  const [allowlist, setAllowlist] = useState<AllowlistRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchFilter, setSearchFilter] = useState('');
  const [togglingSafeSearch, setTogglingSafeSearch] = useState(false);
  const [togglingParental, setTogglingParental] = useState(false);
  const [togglingGambling, setTogglingGambling] = useState(false);
  const [togglingAdBlock, setTogglingAdBlock] = useState(false);
  const [togglingMalware, setTogglingMalware] = useState(false);
  const [syncingGitHub, setSyncingGitHub] = useState(false);
  const [autoUpdateStatus, setAutoUpdateStatus] = useState<AutoUpdateStatus | null>(null);

  // Add Allowlist Modal state
  const [showAddAllowModal, setShowAddAllowModal] = useState(false);
  const [newDomain, setNewDomain] = useState('');
  const [newCategory, setNewCategory] = useState('OPERATIONAL_EXCEPTION');
  const [newReason, setNewReason] = useState('');

  // Add Blocklist Modal state
  const [showAddBlockModal, setShowAddBlockModal] = useState(false);
  const [newFeedName, setNewFeedName] = useState('');
  const [newFeedCategory, setNewFeedCategory] = useState('MALWARE_ADS');
  const [newFeedUrl, setNewFeedUrl] = useState('');
  const [newFeedLocalPath, setNewFeedLocalPath] = useState('');
  const [newFeedEnabled, setNewFeedEnabled] = useState(true);

  const [actionMsg, setActionMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const fetchFilters = async () => {
    try {
      const data = await api.getFilters();
      if (data && data.length > 0) {
        setFilters(data);
      } else {
        // Fallback default seeds if database empty
        setFilters([
          {
            id: 'feed-hagezi',
            name: 'Hagezi Multi-PRO Threat Feed',
            category: 'MALWARE_ADS',
            source_url: 'https://raw.githubusercontent.com/hagezi/dns-blocklists/main/adblock/pro.txt',
            local_path: '/var/lib/netora/filters/hagezi-pro.txt',
            rule_count: 142300,
            enabled: true,
            created_at: new Date().toISOString(),
          },
          {
            id: 'feed-adguard',
            name: 'AdGuard Base DNS Filter',
            category: 'ADS_TRACKING',
            source_url: 'https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt',
            local_path: '/var/lib/netora/filters/adguard.txt',
            rule_count: 48900,
            enabled: true,
            created_at: new Date().toISOString(),
          },
          {
            id: 'feed-stevenblack',
            name: 'StevenBlack Unified Hosts File',
            category: 'HOSTS_BLOCK',
            source_url: 'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts',
            local_path: '/etc/unbound/blocklists/stevenblack.hosts',
            rule_count: 73200,
            enabled: true,
            created_at: new Date().toISOString(),
          },
        ]);
      }
    } catch (err) {
      console.error('Failed to load filters:', err);
    }
  };

  const fetchAllowlist = async () => {
    try {
      const records = await api.getAllowlist();
      setAllowlist(records);
    } catch (err) {
      console.error('Failed to load allowlist:', err);
    }
  };

  const fetchAutoUpdateStatus = async () => {
    try {
      const status = await api.getAutoUpdateStatus();
      if (status) {
        setAutoUpdateStatus(status);
      }
    } catch (err) {
      console.error('Failed to load auto-update status:', err);
    }
  };

  const loadAll = async () => {
    setLoading(true);
    await Promise.all([fetchFilters(), fetchAllowlist(), fetchAutoUpdateStatus()]);
    setLoading(false);
  };

  useEffect(() => {
    loadAll();
  }, []);

  // Handlers for Blocklists Toggle ON/OFF
  const handleToggleFeed = async (id: string, currentEnabled: boolean) => {
    const nextVal = !currentEnabled;
    // Optimistic UI update
    setFilters((prev) => prev.map((f) => (f.id === id ? { ...f, enabled: nextVal } : f)));
    try {
      await api.toggleFilter(id, nextVal);
      const feed = filters.find((f) => f.id === id);
      setActionMsg({
        type: 'success',
        text: `Feed "${feed?.name || id}" berhasil di-${nextVal ? 'aktifkan' : 'nonaktifkan'}.`,
      });
    } catch (err: any) {
      // Revert on failure
      setFilters((prev) => prev.map((f) => (f.id === id ? { ...f, enabled: currentEnabled } : f)));
      setActionMsg({
        type: 'error',
        text: `Gagal mengubah status feed: ${err.message}`,
      });
    }
  };

  // Bulk Toggle All Feeds
  const handleToggleAllFeeds = async (enableAll: boolean) => {
    // Optimistic UI update
    setFilters((prev) => prev.map((f) => ({ ...f, enabled: enableAll })));
    try {
      await Promise.all(filters.map((f) => api.toggleFilter(f.id, enableAll)));
      setActionMsg({
        type: 'success',
        text: `Semua (${filters.length}) filter feed berhasil di-${enableAll ? 'aktifkan' : 'nonaktifkan'}.`,
      });
    } catch (err: any) {
      fetchFilters(); // Re-sync
      setActionMsg({
        type: 'error',
        text: `Gagal mengubah semua feed: ${err.message}`,
      });
    }
  };

  // SafeSearch Toggle ON/OFF Handler
  const handleToggleSafeSearch = async (newVal: boolean) => {
    if (!config) return;
    setTogglingSafeSearch(true);
    try {
      const updatedConfig: DesiredState = {
        ...config,
        safe_search: {
          ...config.safe_search,
          enabled: newVal,
        },
      };
      await api.applyConfig(nodeId, updatedConfig, `SafeSearch set to ${newVal ? 'ENABLED' : 'DISABLED'} from Filtering View`);
      if (onConfigUpdated) {
        onConfigUpdated(updatedConfig);
      }
      setActionMsg({
        type: 'success',
        text: `SafeSearch berhasil di-${newVal ? 'aktifkan' : 'nonaktifkan'}! Unbound berhasil di-reload secara atomik.`,
      });
    } catch (err: any) {
      setActionMsg({
        type: 'error',
        text: `Gagal memperbarui SafeSearch: ${err.message}`,
      });
    } finally {
      setTogglingSafeSearch(false);
    }
  };

  // Parental Control (18+ Adult Content) Toggle ON/OFF Handler
  const handleToggleParentalControl = async (newVal: boolean) => {
    if (!config) return;
    setTogglingParental(true);
    try {
      const updatedConfig: DesiredState = {
        ...config,
        parental_control: {
          enabled: newVal,
          block_adult: newVal,
          block_gambling: config.gambling_block?.enabled ?? config.parental_control?.block_gambling ?? false,
          redirect_ip: config.parental_control?.redirect_ip || '',
        },
      };
      await api.applyConfig(nodeId, updatedConfig, `Parental Control set to ${newVal ? 'ENABLED' : 'DISABLED'} from Filtering View`);
      if (onConfigUpdated) {
        onConfigUpdated(updatedConfig);
      }
      setActionMsg({
        type: 'success',
        text: `Parental Control berhasil di-${newVal ? 'aktifkan (Situs 18+ & Dewasa Diblokir)' : 'nonaktifkan (Situs 18+ Terbuka Bebas)'}! Unbound berhasil di-reload secara atomik.`,
      });
    } catch (err: any) {
      setActionMsg({
        type: 'error',
        text: `Gagal memperbarui Parental Control: ${err.message}`,
      });
    } finally {
      setTogglingParental(false);
    }
  };

  // Judi Online (Gambling Block) Toggle ON/OFF Handler
  const handleToggleGambling = async (newVal: boolean) => {
    if (!config) return;
    setTogglingGambling(true);
    try {
      const updatedConfig: DesiredState = {
        ...config,
        gambling_block: {
          enabled: newVal,
          redirect_ip: config.gambling_block?.redirect_ip || '',
        },
        parental_control: {
          ...config.parental_control,
          enabled: config.parental_control?.enabled ?? false,
          block_adult: config.parental_control?.block_adult ?? false,
          block_gambling: newVal,
          redirect_ip: config.parental_control?.redirect_ip || '',
        },
      };
      await api.applyConfig(nodeId, updatedConfig, `Judi Online Filter set to ${newVal ? 'ENABLED (BLOCKED)' : 'DISABLED (ALLOWED)'} from Filtering View`);
      if (onConfigUpdated) {
        onConfigUpdated(updatedConfig);
      }
      setActionMsg({
        type: 'success',
        text: `Filter Judi Online berhasil di-${newVal ? 'aktifkan (Situs Judi & Slot Diblokir)' : 'nonaktifkan (Situs Judi Terbuka Bebas)'}! Unbound berhasil di-reload secara atomik.`,
      });
    } catch (err: any) {
      setActionMsg({
        type: 'error',
        text: `Gagal memperbarui Filter Judi Online: ${err.message}`,
      });
    } finally {
      setTogglingGambling(false);
    }
  };

  // Iklan & Pelacak (AdBlock) Toggle ON/OFF Handler
  const handleToggleAdBlock = async (newVal: boolean) => {
    if (!config) return;
    setTogglingAdBlock(true);
    try {
      const updatedConfig: DesiredState = {
        ...config,
        ad_block: {
          enabled: newVal,
          redirect_ip: config.ad_block?.redirect_ip || '',
        },
      };
      await api.applyConfig(nodeId, updatedConfig, `AdBlock set to ${newVal ? 'ENABLED' : 'DISABLED'} from Filtering View`);
      if (onConfigUpdated) onConfigUpdated(updatedConfig);
      setActionMsg({
        type: 'success',
        text: `Filter Iklan & Pelacak berhasil di-${newVal ? 'aktifkan (Iklan Diblokir)' : 'nonaktifkan (Iklan Terbuka)'}! Unbound di-reload atomik.`,
      });
    } catch (err: any) {
      setActionMsg({ type: 'error', text: `Gagal memperbarui Filter Iklan: ${err.message}` });
    } finally {
      setTogglingAdBlock(false);
    }
  };

  // Malware & Phishing Shield Toggle ON/OFF Handler
  const handleToggleMalwareBlock = async (newVal: boolean) => {
    if (!config) return;
    setTogglingMalware(true);
    try {
      const updatedConfig: DesiredState = {
        ...config,
        malware_block: {
          enabled: newVal,
          redirect_ip: config.malware_block?.redirect_ip || '',
        },
      };
      await api.applyConfig(nodeId, updatedConfig, `MalwareBlock set to ${newVal ? 'ENABLED' : 'DISABLED'} from Filtering View`);
      if (onConfigUpdated) onConfigUpdated(updatedConfig);
      setActionMsg({
        type: 'success',
        text: `Malware & Threat Shield berhasil di-${newVal ? 'aktifkan (Malware Diblokir)' : 'nonaktifkan (Proteksi Dimatikan)'}! Unbound di-reload atomik.`,
      });
    } catch (err: any) {
      setActionMsg({ type: 'error', text: `Gagal memperbarui Filter Malware: ${err.message}` });
    } finally {
      setTogglingMalware(false);
    }
  };

  // Sync All Blocklists Handler
  const handleSyncBlocklists = async () => {
    setSyncingGitHub(true);
    try {
      const res = await api.syncBlocklists();
      setActionMsg({
        type: 'success',
        text: res.message || `Berhasil sinkronisasi seluruh filter! Total ${res.total_domains?.toLocaleString() || (res.adult_domains_count + res.gambling_domains_count).toLocaleString()} domain terblokir. Unbound berhasil di-reload secara atomik.`,
      });
      await Promise.all([fetchFilters(), fetchAutoUpdateStatus()]);
    } catch (err: any) {
      setActionMsg({
        type: 'error',
        text: `Gagal sinkronisasi seluruh filter: ${err.message}`,
      });
    } finally {
      setSyncingGitHub(false);
    }
  };

  // Add Blocklist Handler
  const handleAddBlocklist = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newFeedName.trim() || (!newFeedUrl.trim() && !newFeedLocalPath.trim())) return;
    try {
      await api.addFilter(newFeedName.trim(), newFeedCategory, newFeedUrl.trim(), newFeedLocalPath.trim());
      setActionMsg({ type: 'success', text: `Filter feed "${newFeedName}" berhasil ditambahkan!` });
      setShowAddBlockModal(false);
      setNewFeedName('');
      setNewFeedUrl('');
      setNewFeedLocalPath('');
      setNewFeedEnabled(true);
      fetchFilters();
    } catch (err: any) {
      setActionMsg({ type: 'error', text: `Error: ${err.message}` });
    }
  };

  const handleDeleteFilter = async (id: string, name: string) => {
    if (!confirm(`Are you sure you want to remove filter feed "${name}"?`)) return;
    try {
      await api.deleteFilter(id);
      setActionMsg({ type: 'success', text: `Filter feed "${name}" removed.` });
      setFilters(filters.filter((f) => f.id !== id));
    } catch (err: any) {
      setActionMsg({ type: 'error', text: `Error deleting feed: ${err.message}` });
    }
  };

  const handleSelectPreset = (preset: typeof PRESET_FEEDS[0]) => {
    setNewFeedName(preset.name);
    setNewFeedCategory(preset.category);
    setNewFeedUrl(preset.url);
  };

  // Handlers for Allowlist
  const handleAddAllowlist = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain.trim()) return;
    try {
      await api.addAllowlist(newDomain.trim(), newCategory, newReason || 'NOC operator exception');
      setActionMsg({ type: 'success', text: `Added ${newDomain} to allowlist!` });
      setShowAddAllowModal(false);
      setNewDomain('');
      setNewReason('');
      fetchAllowlist();
    } catch (err: any) {
      setActionMsg({ type: 'error', text: `Error: ${err.message}` });
    }
  };

  const handleDeleteAllowlist = async (id: string, domain: string) => {
    try {
      await api.deleteAllowlist(domain);
      setActionMsg({ type: 'success', text: `Allowlist exception "${domain}" removed.` });
      fetchAllowlist();
    } catch (err: any) {
      setActionMsg({ type: 'error', text: `Error removing entry: ${err.message}` });
    }
  };

  const filteredFilters = filters.filter(
    (f) =>
      f.name.toLowerCase().includes(searchFilter.toLowerCase()) ||
      f.category.toLowerCase().includes(searchFilter.toLowerCase()) ||
      f.source_url?.toLowerCase().includes(searchFilter.toLowerCase())
  );

  const filteredAllowlist = allowlist.filter(
    (a) =>
      a.domain.toLowerCase().includes(searchFilter.toLowerCase()) ||
      a.reason?.toLowerCase().includes(searchFilter.toLowerCase())
  );

  const activeFilters = filters.filter((f) => f.enabled);
  const activeRules = activeFilters.reduce((acc, f) => acc + (f.rule_count || 0), 0);
  const totalRules = filters.reduce((acc, f) => acc + (f.rule_count || 0), 0);

  const isSafeSearchActive = Boolean(config?.safe_search?.enabled ?? true);
  const isParentalActive = Boolean(config?.parental_control?.enabled);
  const isGamblingActive = Boolean(config?.gambling_block?.enabled || config?.parental_control?.block_gambling);
  const isAdBlockActive = Boolean(config?.ad_block?.enabled ?? true);
  const isMalwareBlockActive = Boolean(config?.malware_block?.enabled ?? true);

  return (
    <div className="space-y-6">
      {/* Title & Top Action */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Filter className="w-5 h-5 text-cyan-400" />
            DNS Policy Filtering & Protection Engine
          </h1>
          <p className="text-xs text-gray-400 mt-1">
            Kelola kebijakan penyaringan DNS: SafeSearch VIP, Konten 18+ Dewasa, Judi Online, Iklan, dan Pengecualian Allowlist.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={handleSyncBlocklists}
            disabled={syncingGitHub || autoUpdateStatus?.is_syncing}
            className="px-3.5 py-2 bg-gray-800 hover:bg-gray-750 text-gray-200 hover:text-white border border-gray-700 hover:border-cyan-500/50 font-medium text-xs rounded-lg transition-all flex items-center gap-2 shadow-sm disabled:opacity-50"
            title="Sinkronisasi seluruh filter dari repositori dan feed upstream"
          >
            <RefreshCw className={`w-4 h-4 text-cyan-400 ${syncingGitHub || autoUpdateStatus?.is_syncing ? 'animate-spin' : ''}`} />
            <span>{syncingGitHub || autoUpdateStatus?.is_syncing ? 'Syncing...' : 'Sync Seluruh Filter'}</span>
          </button>

          <button
            onClick={() => setShowAddAllowModal(true)}
            className="px-4 py-2 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-emerald-500/20 flex items-center gap-2"
          >
            <Plus className="w-4 h-4" />
            Add Allowlist Exception
          </button>
        </div>
      </div>

      {actionMsg && (
        <div
          className={`p-3 rounded-xl border text-xs flex items-center justify-between font-mono animate-in fade-in duration-200 ${
            actionMsg.type === 'success'
              ? 'bg-emerald-950/40 border-emerald-800 text-emerald-300'
              : 'bg-rose-950/40 border-rose-800 text-rose-300'
          }`}
        >
          <div className="flex items-center gap-2">
            {actionMsg.type === 'success' ? (
              <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
            ) : (
              <ShieldAlert className="w-4 h-4 text-rose-400 shrink-0" />
            )}
            <span>{actionMsg.text}</span>
          </div>
          <button onClick={() => setActionMsg(null)} className="text-gray-400 hover:text-white ml-4 font-bold">
            &times;
          </button>
        </div>
      )}

      {/* Auto-Update Autonomous Engine Card */}
      <div className="bg-gradient-to-r from-gray-900 via-gray-900/90 to-gray-850 border border-cyan-900/30 rounded-2xl p-5 shadow-lg relative overflow-hidden">
        <div className="absolute top-0 right-0 w-96 h-96 bg-cyan-500/5 rounded-full blur-3xl pointer-events-none" />
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 relative z-10">
          <div className="flex items-start gap-3.5">
            <div className="p-2.5 rounded-xl bg-cyan-500/10 border border-cyan-500/20 text-cyan-400 shrink-0 mt-0.5">
              <RefreshCw className={`w-5 h-5 ${syncingGitHub || autoUpdateStatus?.is_syncing ? 'animate-spin text-cyan-300' : ''}`} />
            </div>
            <div>
              <div className="flex items-center gap-2.5 flex-wrap">
                <h3 className="text-sm font-bold text-white tracking-wide">
                  Sistem Pembaruan Otomatis Filter (Auto-Update)
                </h3>
                <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-emerald-950/60 border border-emerald-500/40 text-emerald-300">
                  <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
                  AKTIF (Tiap 12 Jam)
                </span>
                <span className="text-[11px] text-gray-400 font-mono bg-gray-800/80 px-2 py-0.5 rounded border border-gray-700">
                  Dual-Trigger: In-Process + OS Cron Backup
                </span>
              </div>
              <p className="text-xs text-gray-400 mt-1 max-w-2xl">
                Seluruh feed filter (Dewasa 18+, Judi Online, Iklan, Malware, dan Custom ISP) disinkronisasi, disanitasi, divalidasi dengan <code className="text-cyan-300 font-mono text-[11px]">unbound-checkconf</code>, dan di-reload secara atomik tanpa downtime.
              </p>
              <div className="flex items-center gap-4 mt-2.5 text-[11px] text-gray-400 flex-wrap">
                <span className="flex items-center gap-1">
                  <span className="text-gray-500">Terakhir Diperbarui:</span>{' '}
                  <strong className="text-gray-200 font-mono">
                    {autoUpdateStatus?.last_sync_time
                      ? new Date(autoUpdateStatus.last_sync_time).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
                      : 'Baru saja disinkronkan'}
                  </strong>
                </span>
                <span className="text-gray-600">•</span>
                <span className="flex items-center gap-1">
                  <span className="text-gray-500">Pembaruan Berikutnya:</span>{' '}
                  <strong className="text-gray-200 font-mono">
                    {autoUpdateStatus?.next_sync_time
                      ? new Date(autoUpdateStatus.next_sync_time).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
                      : 'Dalam 12 Jam'}
                  </strong>
                </span>
                <span className="text-gray-600">•</span>
                <span className="flex items-center gap-1">
                  <span className="text-gray-500">Total Terblokir:</span>{' '}
                  <strong className="text-cyan-300 font-mono">
                    {autoUpdateStatus?.total_domains
                      ? autoUpdateStatus.total_domains.toLocaleString()
                      : (totalRules > 0 ? totalRules.toLocaleString() : '250,000+')} domain
                  </strong>
                </span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <button
              onClick={handleSyncBlocklists}
              disabled={syncingGitHub || autoUpdateStatus?.is_syncing}
              className="px-4 py-2.5 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-semibold text-xs rounded-xl transition-all shadow-md shadow-cyan-500/20 flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <RefreshCw className={`w-4 h-4 ${syncingGitHub || autoUpdateStatus?.is_syncing ? 'animate-spin' : ''}`} />
              <span>{syncingGitHub || autoUpdateStatus?.is_syncing ? 'Menyinkronkan...' : 'Sync Seluruh Filter Sekarang'}</span>
            </button>
          </div>
        </div>
      </div>

      {/* Master Policy Protection Cards (5 Column Grid) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-5 gap-4">
        {/* Card 1: SafeSearch */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 relative overflow-hidden group hover:border-cyan-800/60 transition-all">
          <div className="flex items-center justify-between text-gray-400">
            <span className="text-xs font-semibold uppercase tracking-wider text-cyan-300">SafeSearch</span>
            <div className="flex items-center gap-2">
              <ToggleSwitch
                checked={isSafeSearchActive}
                onChange={handleToggleSafeSearch}
                size="md"
                disabled={togglingSafeSearch || !config}
              />
              <ShieldCheck className={`w-4 h-4 ${isSafeSearchActive ? 'text-cyan-400' : 'text-gray-600'}`} />
            </div>
          </div>
          <div className="text-2xl font-bold font-mono mt-2 flex items-center gap-2">
            {isSafeSearchActive ? (
              <span className="text-emerald-400 flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse"></span>
                ACTIVE
              </span>
            ) : (
              <span className="text-gray-500">DISABLED</span>
            )}
            {togglingSafeSearch && <RefreshCw className="w-4 h-4 text-cyan-400 animate-spin" />}
          </div>
          <p className="text-[11px] text-gray-400 mt-1 font-mono truncate">
            {isSafeSearchActive ? 'Forces Google, Bing, YT safe mode' : 'Direct search queries allowed'}
          </p>
          <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-cyan-400 font-mono flex items-center justify-between">
            <span className="truncate">{isSafeSearchActive ? '216.239.38.120 (VIP)' : 'SafeSearch Off'}</span>
            <span className="text-[10px] text-gray-500 font-sans">Toggle ON/OFF</span>
          </div>
        </div>

        {/* Card 2: 18+ Dewasa */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 relative overflow-hidden group hover:border-purple-800/60 transition-all">
          <div className="flex items-center justify-between text-gray-400">
            <span className="text-xs font-semibold uppercase tracking-wider text-purple-300">18+ Dewasa</span>
            <div className="flex items-center gap-2">
              <ToggleSwitch
                checked={isParentalActive}
                onChange={handleToggleParentalControl}
                size="md"
                disabled={togglingParental || !config}
              />
              <Lock className={`w-4 h-4 ${isParentalActive ? 'text-purple-400' : 'text-gray-600'}`} />
            </div>
          </div>
          <div className="text-2xl font-bold font-mono mt-2 flex items-center gap-2">
            {isParentalActive ? (
              <span className="text-purple-400 flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full bg-purple-400 animate-pulse"></span>
                BLOCKED
              </span>
            ) : (
              <span className="text-gray-400">ALLOWED</span>
            )}
            {togglingParental && <RefreshCw className="w-4 h-4 text-purple-400 animate-spin" />}
          </div>
          <p className="text-[11px] text-gray-400 mt-1 font-mono truncate">
            {isParentalActive ? 'Situs 18+ & pornografi diblokir' : 'Konten 18+ bebas akses (terbuka)'}
          </p>
          <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-purple-300 font-mono flex items-center justify-between">
            <span className="truncate">{isParentalActive ? 'Sinkhole NXDOMAIN' : 'Akses Terbuka'}</span>
            <span className="text-[10px] text-gray-500 font-sans">Toggle ON/OFF</span>
          </div>
        </div>

        {/* Card 3: Judi Online */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 relative overflow-hidden group hover:border-amber-800/60 transition-all">
          <div className="flex items-center justify-between text-gray-400">
            <span className="text-xs font-semibold uppercase tracking-wider text-amber-300">Judi Online</span>
            <div className="flex items-center gap-2">
              <ToggleSwitch
                checked={isGamblingActive}
                onChange={handleToggleGambling}
                size="md"
                disabled={togglingGambling || !config}
              />
              <Dices className={`w-4 h-4 ${isGamblingActive ? 'text-amber-400' : 'text-gray-600'}`} />
            </div>
          </div>
          <div className="text-2xl font-bold font-mono mt-2 flex items-center gap-2">
            {isGamblingActive ? (
              <span className="text-amber-400 flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full bg-amber-400 animate-pulse"></span>
                BLOCKED
              </span>
            ) : (
              <span className="text-gray-400">ALLOWED</span>
            )}
            {togglingGambling && <RefreshCw className="w-4 h-4 text-amber-400 animate-spin" />}
          </div>
          <p className="text-[11px] text-gray-400 mt-1 font-mono truncate">
            {isGamblingActive ? 'Situs judi & slot diblokir' : 'Situs judi tidak diblokir'}
          </p>
          <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-amber-300 font-mono flex items-center justify-between">
            <span className="truncate">{isGamblingActive ? 'Sinkhole NXDOMAIN' : 'Akses Terbuka'}</span>
            <span className="text-[10px] text-gray-500 font-sans">Toggle ON/OFF</span>
          </div>
        </div>

        {/* Card 4: Iklan & Pelacak */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 relative overflow-hidden group hover:border-rose-800/60 transition-all">
          <div className="flex items-center justify-between text-gray-400">
            <span className="text-xs font-semibold uppercase tracking-wider text-rose-300">Iklan & Pelacak</span>
            <div className="flex items-center gap-2">
              <ToggleSwitch
                checked={isAdBlockActive}
                onChange={handleToggleAdBlock}
                size="md"
                disabled={togglingAdBlock || !config}
              />
              <ShieldAlert className={`w-4 h-4 ${isAdBlockActive ? 'text-rose-400' : 'text-gray-600'}`} />
            </div>
          </div>
          <div className="text-2xl font-bold font-mono mt-2 flex items-center gap-2">
            {isAdBlockActive ? (
              <span className="text-rose-400 flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full bg-rose-400 animate-pulse"></span>
                BLOCKED
              </span>
            ) : (
              <span className="text-gray-400">ALLOWED</span>
            )}
            {togglingAdBlock && <RefreshCw className="w-4 h-4 text-rose-400 animate-spin" />}
          </div>
          <p className="text-[11px] text-gray-400 mt-1 font-mono truncate">
            {isAdBlockActive ? 'Banner iklan & tracker diblokir' : 'Iklan bebas tampil'}
          </p>
          <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-rose-300 font-mono flex items-center justify-between">
            <span className="truncate">{isAdBlockActive ? 'Sinkhole NXDOMAIN' : 'Akses Terbuka'}</span>
            <span className="text-[10px] text-gray-500 font-sans">Toggle ON/OFF</span>
          </div>
        </div>

        {/* Card 5: Malware Shield */}
        <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 relative overflow-hidden group hover:border-emerald-800/60 transition-all">
          <div className="flex items-center justify-between text-gray-400">
            <span className="text-xs font-semibold uppercase tracking-wider text-emerald-300">Malware Shield</span>
            <div className="flex items-center gap-2">
              <ToggleSwitch
                checked={isMalwareBlockActive}
                onChange={handleToggleMalwareBlock}
                size="md"
                disabled={togglingMalware || !config}
              />
              <ShieldCheck className={`w-4 h-4 ${isMalwareBlockActive ? 'text-emerald-400' : 'text-gray-600'}`} />
            </div>
          </div>
          <div className="text-2xl font-bold font-mono mt-2 flex items-center gap-2">
            {isMalwareBlockActive ? (
              <span className="text-emerald-400 flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse"></span>
                PROTECTED
              </span>
            ) : (
              <span className="text-gray-400">DISABLED</span>
            )}
            {togglingMalware && <RefreshCw className="w-4 h-4 text-emerald-400 animate-spin" />}
          </div>
          <p className="text-[11px] text-gray-400 mt-1 font-mono truncate">
            {isMalwareBlockActive ? 'Phishing, C2 botnet & malware' : 'Proteksi malware dinonaktifkan'}
          </p>
          <div className="mt-3 pt-3 border-t border-gray-800 text-xs text-emerald-300 font-mono flex items-center justify-between">
            <span className="truncate">{isMalwareBlockActive ? 'Sinkhole NXDOMAIN' : 'Proteksi Mati'}</span>
            <span className="text-[10px] text-gray-500 font-sans">Toggle ON/OFF</span>
          </div>
        </div>
      </div>

      {/* Embedded Feeds Architecture Note */}
      <div className="bg-gradient-to-r from-gray-900/90 via-gray-900 to-gray-850 border border-gray-800 rounded-xl p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3 shadow-md">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 shrink-0">
            <ShieldCheck className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h4 className="text-xs font-bold text-white tracking-wide">
                Seluruh Filter (18+ Dewasa, Judi Online, Iklan & Malware) Tertanam Otonom
              </h4>
              <span className="text-[10px] px-2 py-0.5 rounded font-bold bg-emerald-950 text-emerald-400 border border-emerald-800 font-mono">
                GITHUB SYNCED
              </span>
            </div>
            <p className="text-[11px] text-gray-400 mt-0.5">
              Daftar domain disinkronkan otomatis dari repository GitHub via Auto-Update Engine. Cukup edit file di repository Anda untuk memperbarui domain, atau gunakan saklar master di atas untuk mengaktifkan/menonaktifkan proteksi tanpa downtime.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={() => setShowAddAllowModal(true)}
            className="px-3.5 py-2 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-black font-bold text-xs rounded-lg transition-all shadow-md shadow-emerald-500/20 flex items-center gap-2"
          >
            <Plus className="w-3.5 h-3.5" />
            Add Allowlist Exception
          </button>
        </div>
      </div>

      {/* Whitelisted Domain Rules (Exceptions) Table */}
      <div className="bg-gray-900/80 border border-gray-800 rounded-xl p-5 space-y-4 shadow-md">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div>
            <h2 className="text-sm font-bold text-white flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4 text-emerald-400" />
              Whitelisted Domain Rules (Pengecualian Pelanggan)
            </h2>
            <p className="text-xs text-gray-400 mt-1">
              Domain di dalam daftar ini memiliki prioritas tertinggi dan tidak akan pernah diblokir oleh filter 18+, Judi Online, Iklan, maupun Malware.
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
                <th className="py-2.5 px-3">Added By</th>
                <th className="py-2.5 px-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800/60">
              {filteredAllowlist.length === 0 ? (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-gray-500">
                    Belum ada domain pengecualian khusus. Klik "Add Allowlist Exception" jika ada domain pelanggan yang perlu dikecualikan dari pemblokiran.
                  </td>
                </tr>
              ) : (
                filteredAllowlist.map((a) => (
                  <tr key={a.id || a.domain} className="hover:bg-gray-800/30">
                    <td className="py-2.5 px-3 font-bold text-emerald-400 flex items-center gap-2">
                      <CheckCircle2 className="w-3.5 h-3.5 shrink-0" />
                      {a.domain}
                    </td>
                    <td className="py-2.5 px-3">
                      <span className="px-2 py-0.5 rounded bg-gray-950 border border-gray-800 text-cyan-300 font-semibold">
                        {a.category}
                      </span>
                    </td>
                    <td className="py-2.5 px-3 text-gray-300">{a.reason}</td>
                    <td className="py-2.5 px-3 text-gray-500 font-sans">{a.created_by}</td>
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => handleDeleteAllowlist(a.id, a.domain)}
                        className="p-1 text-gray-500 hover:text-red-400 transition-colors"
                        title="Delete exception"
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

      {/* Optional Discreet Collapsible for Advanced Feeds */}
      <div className="pt-2">
        <button
          type="button"
          onClick={() => setShowRawFeeds(!showRawFeeds)}
          className="text-xs text-gray-500 hover:text-gray-300 transition-colors flex items-center gap-1.5 font-mono py-1 px-2 rounded hover:bg-gray-900"
        >
          {showRawFeeds ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
          <span>{showRawFeeds ? 'Sembunyikan Detail File Feed Teknis' : 'Tampilkan Detail File Feed Teknis (Lanjutan)'}</span>
        </button>

        {showRawFeeds && (
          <div className="mt-3 bg-gray-900/60 border border-gray-800/80 rounded-xl p-4 space-y-3 animate-in fade-in duration-200">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-xs font-bold text-gray-300">File Feed & Sumber Domain (Mode Lanjutan)</h3>
                <p className="text-[11px] text-gray-500">File-file ini tertanam di sistem dan diperbarui otonom via auto-update.</p>
              </div>
              <span className="text-[11px] text-gray-500 font-mono">{filters.length} feeds terdaftar</span>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs font-mono">
                <thead className="bg-gray-950 text-gray-500 border-b border-gray-800">
                  <tr>
                    <th className="py-2 px-3">Feed Name</th>
                    <th className="py-2 px-3">Category</th>
                    <th className="py-2 px-3">Path / Source</th>
                    <th className="py-2 px-3">Rules</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800/40 text-gray-400">
                  {filters.map((f) => (
                    <tr key={f.id} className="hover:bg-gray-800/20">
                      <td className="py-2 px-3 text-gray-300 font-semibold">{f.name}</td>
                      <td className="py-2 px-3">
                        <span className="px-1.5 py-0.5 rounded bg-gray-950 text-[10px] text-cyan-400 border border-gray-800">{f.category}</span>
                      </td>
                      <td className="py-2 px-3 text-[11px] text-gray-500 max-w-xs truncate">{f.source_url || f.local_path}</td>
                      <td className="py-2 px-3 text-gray-300">{f.rule_count?.toLocaleString() || '-'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>


      {/* Add Blocklist Modal */}
      {showAddBlockModal && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 max-w-lg w-full space-y-4 shadow-2xl">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Plus className="w-5 h-5 text-cyan-400" />
              Add Blocklist / External Hosts Feed
            </h2>

            {/* Presets Selection */}
            <div>
              <label className="block text-xs font-semibold text-gray-300 mb-2">Recommended Community Feeds:</label>
              <div className="grid grid-cols-2 gap-2">
                {PRESET_FEEDS.map((preset) => (
                  <button
                    key={preset.name}
                    type="button"
                    onClick={() => handleSelectPreset(preset)}
                    className="p-2 bg-gray-950 hover:bg-gray-800 border border-gray-800 rounded text-left transition-colors"
                  >
                    <div className="text-[11px] font-bold text-cyan-300 truncate">{preset.name}</div>
                    <div className="text-[10px] text-gray-500">{preset.category}</div>
                  </button>
                ))}
              </div>
            </div>

            <form onSubmit={handleAddBlocklist} className="space-y-4 pt-2 border-t border-gray-800">
              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Feed Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Hagezi Multi PRO, OISD"
                  value={newFeedName}
                  onChange={(e) => setNewFeedName(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-cyan-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Category</label>
                <select
                  value={newFeedCategory}
                  onChange={(e) => setNewFeedCategory(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-cyan-300 font-mono"
                >
                  <option value="MALWARE_ADS">Malware, Phishing & Aggressive Ads</option>
                  <option value="ADS_TRACKING">Ads & Tracking Only</option>
                  <option value="HOSTS_BLOCK">Hosts File Format</option>
                  <option value="ADULT_GAMBLING">Adult & Gambling Filter</option>
                  <option value="CUSTOM_ISP">Custom ISP Policy Feed</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Source URL (Adblock or Hosts format)</label>
                <input
                  type="url"
                  placeholder="https://raw.githubusercontent.com/.../hosts"
                  value={newFeedUrl}
                  onChange={(e) => setNewFeedUrl(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-cyan-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">Or Local File Path on Server</label>
                <input
                  type="text"
                  placeholder="e.g. /etc/unbound/blocklists/custom.hosts"
                  value={newFeedLocalPath}
                  onChange={(e) => setNewFeedLocalPath(e.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-cyan-500"
                />
              </div>

              <div className="flex items-center justify-between p-3 bg-gray-950 rounded-lg border border-gray-800">
                <span className="text-xs text-gray-300 font-medium">Aktifkan feed ini segera setelah disimpan:</span>
                <ToggleSwitch
                  checked={newFeedEnabled}
                  onChange={setNewFeedEnabled}
                  size="sm"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-gray-800">
                <button
                  type="button"
                  onClick={() => setShowAddBlockModal(false)}
                  className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-medium rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-cyan-500 hover:bg-cyan-400 text-black font-semibold text-xs rounded-lg transition-colors"
                >
                  Save & Compile Feed
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add Allowlist Modal */}
      {showAddAllowModal && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 max-w-md w-full space-y-4 shadow-2xl">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Plus className="w-5 h-5 text-emerald-400" />
              Add Allowlist Domain Exception
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
                  onClick={() => setShowAddAllowModal(false)}
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
