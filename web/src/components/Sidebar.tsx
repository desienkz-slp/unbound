import React from 'react';
import localVersion from '../version.json';
import {
  Activity,
  Cpu,
  FileText,
  Filter,
  History,
  LayoutDashboard,
  Network,
  Search,
  Sparkles,
  UserCheck,
} from 'lucide-react';

export type NavTab =
  | 'overview'
  | 'diagnostics'
  | 'dns-config'
  | 'filtering'
  | 'performance'
  | 'monitoring'
  | 'history'
  | 'audit'
  | 'wizard';

interface SidebarProps {
  currentTab: NavTab;
  onSelectTab: (tab: NavTab) => void;
  unboundStatus?: boolean;
  onOpenUserSettings?: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  currentTab,
  onSelectTab,
  unboundStatus = true,
  onOpenUserSettings,
}) => {
  const navItems: Array<{ id: NavTab; label: string; icon: React.ReactNode; badge?: string }> = [
    { id: 'overview', label: 'Overview', icon: <LayoutDashboard className="w-4 h-4" /> },
    { id: 'diagnostics', label: 'Domain Probe', icon: <Search className="w-4 h-4" />, badge: 'Explain' },
    { id: 'dns-config', label: 'DNS Configuration', icon: <Network className="w-4 h-4" /> },
    { id: 'filtering', label: 'Filtering & Safety', icon: <Filter className="w-4 h-4" /> },
    { id: 'performance', label: 'Tuning & Profiles', icon: <Cpu className="w-4 h-4" /> },
    { id: 'monitoring', label: 'Telemetry & Alerts', icon: <Activity className="w-4 h-4" /> },
    { id: 'history', label: 'Versions & Rollback', icon: <History className="w-4 h-4" />, badge: 'Atomic' },
    { id: 'audit', label: 'Audit Trail', icon: <FileText className="w-4 h-4" /> },
    { id: 'wizard', label: 'Setup Wizard', icon: <Sparkles className="w-4 h-4 text-cyan-400" />, badge: 'Setup' },
  ];

  return (
    <aside className="w-64 border-r border-gray-800 bg-[#0B0F19]/95 flex flex-col justify-between p-4 shrink-0 select-none h-full overflow-y-auto card-scrollbar">
      <div className="space-y-6">
        <div>
          <span className="text-[10px] font-semibold uppercase tracking-wider text-gray-500 px-3">
            DNS Operations
          </span>
          <nav className="mt-2 space-y-1">
            {navItems.slice(0, 5).map((item) => {
              const active = currentTab === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => onSelectTab(item.id)}
                  className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs font-medium transition-all ${
                    active
                      ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/30'
                      : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <span className={active ? 'text-cyan-400' : 'text-gray-400'}>{item.icon}</span>
                    <span>{item.label}</span>
                  </div>
                  {item.badge && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-400 font-mono">
                      {item.badge}
                    </span>
                  )}
                </button>
              );
            })}
          </nav>
        </div>

        <div>
          <span className="text-[10px] font-semibold uppercase tracking-wider text-gray-500 px-3">
            System & Resilience
          </span>
          <nav className="mt-2 space-y-1">
            {navItems.slice(5).map((item) => {
              const active = currentTab === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => onSelectTab(item.id)}
                  className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs font-medium transition-all ${
                    active
                      ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/30'
                      : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <span className={active ? 'text-cyan-400' : 'text-gray-400'}>{item.icon}</span>
                    <span>{item.label}</span>
                  </div>
                  {item.badge && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-300 font-mono">
                      {item.badge}
                    </span>
                  )}
                </button>
              );
            })}

            {onOpenUserSettings && (
              <button
                onClick={onOpenUserSettings}
                className="w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs font-medium text-gray-400 hover:text-cyan-400 hover:bg-gray-800/40 transition-all group"
              >
                <div className="flex items-center gap-3">
                  <span className="text-gray-400 group-hover:text-cyan-400"><UserCheck className="w-4 h-4" /></span>
                  <span>User Settings</span>
                </div>
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-cyan-400 font-mono">
                  Account
                </span>
              </button>
            )}
          </nav>
        </div>
      </div>

      {/* Footer Netora Build Version */}
      <div className="mt-6 pt-4 border-t border-gray-800/80">
        <div className="bg-gray-900/60 border border-gray-800/80 rounded-lg px-3 py-2 flex items-center justify-between text-xs">
          <span className="text-gray-400 font-sans">Netora Build</span>
          <span className="font-mono text-cyan-400 font-semibold">{localVersion.version || 'v1.0.44'}</span>
        </div>
      </div>
    </aside>
  );
};
