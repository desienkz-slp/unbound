import React, { useState, useEffect } from 'react';
import { 
  GitCommit, 
  GitBranch, 
  ShieldCheck, 
  Server, 
  RefreshCw, 
  CheckCircle2, 
  ExternalLink,
  Sparkles,
  Info
} from 'lucide-react';
import { NodeRecord, SystemVersion } from '../types';
import { api } from '../services/api';
import localVersion from '../version.json';

interface FooterProps {
  node: NodeRecord;
  unboundHealthy: boolean;
}

export const Footer: React.FC<FooterProps> = ({ node, unboundHealthy }) => {
  const [versionInfo, setVersionInfo] = useState<SystemVersion>({
    version: localVersion.version || 'v1.0.44',
    build: localVersion.commit_count || 44,
    commit_count: localVersion.commit_count || 44,
    git_commit: localVersion.git_commit || 'de28649',
    git_branch: localVersion.git_branch || 'main',
    build_time: localVersion.build_time || new Date().toISOString(),
  });
  const [showDrawer, setShowDrawer] = useState(false);

  useEffect(() => {
    let active = true;
    const loadVersion = async () => {
      try {
        const v = await api.getSystemVersion();
        if (active && v && v.version) {
          setVersionInfo(v);
        }
      } catch {
        // Fallback to static localVersion
      }
    };
    loadVersion();
    const interval = setInterval(loadVersion, 30000);
    return () => {
      active = false;
      clearInterval(interval);
    };
  }, []);

  return (
    <>
      <footer className="h-9 bg-[#070B12] border-t border-gray-800/80 px-4 flex items-center justify-between text-xs select-none shrink-0 z-30 font-sans shadow-lg">
        {/* Left Section: Operational Status & Target Node */}
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <span className="relative flex h-2 w-2">
              <span className={`animate-ping absolute inline-flex h-full w-full rounded-full ${unboundHealthy ? 'bg-emerald-400 opacity-75' : 'bg-red-400 opacity-75'}`}></span>
              <span className={`relative inline-flex rounded-full h-2 w-2 ${unboundHealthy ? 'bg-emerald-500' : 'bg-red-500'}`}></span>
            </span>
            <span className={`text-[11px] font-medium hidden sm:inline ${unboundHealthy ? 'text-emerald-400' : 'text-red-400'}`}>
              {unboundHealthy ? 'System Operational' : 'Data Plane Degraded'}
            </span>
          </div>

          <span className="text-gray-700 hidden sm:inline">|</span>

          {/* Node Badge */}
          <div className="flex items-center gap-1.5 text-gray-400 text-[11px] font-mono">
            <Server className="w-3.5 h-3.5 text-cyan-400" />
            <span className="text-gray-300 font-medium">{node.hostname || node.id}</span>
            <span className="text-gray-500 hidden md:inline">({node.management_ip})</span>
          </div>

          <span className="text-gray-700 hidden md:inline">|</span>

          {/* Data Plane Resolver */}
          <div className="hidden md:flex items-center gap-1.5 text-gray-400 text-[11px]">
            <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
            <span>Unbound {node.unbound_version || '1.13.1'}</span>
          </div>
        </div>

        {/* Right Section: Continuous Versioning & GitHub Sync */}
        <div className="flex items-center gap-3">
          {/* Auto-Update Indicator */}
          <div className="hidden lg:flex items-center gap-1.5 text-gray-400 text-[11px]">
            <RefreshCw className="w-3 h-3 text-cyan-400" />
            <span className="text-gray-400">Fleet Sync:</span>
            <span className="text-emerald-400 font-medium">Continuous</span>
          </div>

          <span className="text-gray-700 hidden lg:inline">|</span>

          {/* Version Badge (Clickable for Detail Modal) */}
          <button
            onClick={() => setShowDrawer(true)}
            className="group flex items-center gap-2 px-2.5 py-1 rounded bg-gray-900/90 border border-gray-800 hover:border-cyan-500/50 hover:bg-gray-800/80 transition-all text-[11px] font-mono shadow-sm cursor-pointer"
            title="Klik untuk detail rilis dan riwayat update GitHub"
          >
            <div className="flex items-center gap-1.5 text-cyan-400 group-hover:text-cyan-300">
              <GitCommit className="w-3.5 h-3.5" />
              <span className="font-semibold text-gray-200 group-hover:text-white">
                {versionInfo.version}
              </span>
            </div>
            <span className="px-1.5 py-0.2 rounded bg-gray-800 group-hover:bg-cyan-950 text-gray-300 group-hover:text-cyan-300 border border-gray-700 group-hover:border-cyan-800 text-[10px]">
              git:{versionInfo.git_commit ? versionInfo.git_commit.substring(0, 7) : 'de28649'}
            </span>
          </button>
        </div>
      </footer>

      {/* Release Details Popover Modal */}
      {showDrawer && (
        <div 
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in duration-150"
          onClick={() => setShowDrawer(false)}
        >
          <div 
            className="w-full max-w-md bg-[#0D131F] border border-gray-800 rounded-xl p-5 shadow-2xl space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between pb-3 border-b border-gray-800">
              <div className="flex items-center gap-2.5">
                <div className="p-2 rounded-lg bg-cyan-500/10 border border-cyan-500/20 text-cyan-400">
                  <Sparkles className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="text-sm font-semibold text-white">Netora DNS Platform</h3>
                  <p className="text-xs text-gray-400">Autonomous Carrier-Grade ISP Release</p>
                </div>
              </div>
              <button 
                onClick={() => setShowDrawer(false)}
                className="text-gray-400 hover:text-white text-xs px-2.5 py-1 rounded bg-gray-800/60 hover:bg-gray-800 transition"
              >
                ✕
              </button>
            </div>

            <div className="space-y-2 text-xs font-mono">
              <div className="flex justify-between items-center py-2 px-3 rounded-lg bg-gray-900/80 border border-gray-800">
                <span className="text-gray-400 font-sans">Current Version</span>
                <span className="text-cyan-400 font-bold text-sm">{versionInfo.version}</span>
              </div>
              <div className="flex justify-between items-center py-2 px-3 rounded-lg bg-gray-900/80 border border-gray-800">
                <span className="text-gray-400 font-sans">Build / Total Commits</span>
                <span className="text-gray-200 font-semibold">#{versionInfo.commit_count || versionInfo.build || 44}</span>
              </div>
              <div className="flex justify-between items-center py-2 px-3 rounded-lg bg-gray-900/80 border border-gray-800">
                <span className="text-gray-400 font-sans">Git Commit SHA</span>
                <span className="text-emerald-400 font-semibold">{versionInfo.git_commit}</span>
              </div>
              <div className="flex justify-between items-center py-2 px-3 rounded-lg bg-gray-900/80 border border-gray-800">
                <span className="text-gray-400 font-sans">Upstream Branch</span>
                <span className="text-purple-400 flex items-center gap-1">
                  <GitBranch className="w-3.5 h-3.5" /> {versionInfo.git_branch || 'main'}
                </span>
              </div>
              <div className="flex justify-between items-center py-2 px-3 rounded-lg bg-gray-900/80 border border-gray-800">
                <span className="text-gray-400 font-sans">Auto-Increment Engine</span>
                <span className="text-emerald-400 flex items-center gap-1 font-sans">
                  <CheckCircle2 className="w-3.5 h-3.5" /> Active via GitHub Pull
                </span>
              </div>
              {versionInfo.build_time && (
                <div className="flex justify-between items-center py-2 px-3 rounded-lg bg-gray-900/80 border border-gray-800">
                  <span className="text-gray-400 font-sans">Release Timestamp</span>
                  <span className="text-gray-400 text-[11px]">{new Date(versionInfo.build_time).toLocaleString()}</span>
                </div>
              )}
            </div>

            <div className="p-3 rounded-lg bg-cyan-950/20 border border-cyan-800/40 text-[11px] text-cyan-300 flex items-start gap-2">
              <Info className="w-4 h-4 shrink-0 text-cyan-400 mt-0.5" />
              <span>Versi ini terus bertambah secara otomatis setiap kali ada pembaruan atau commit baru di GitHub repositori.</span>
            </div>

            <div className="pt-2 border-t border-gray-800 flex items-center justify-between text-xs">
              <span className="text-gray-500 text-[11px]">Canonical Repository</span>
              <a
                href="https://github.com/desienkz-slp/unbound"
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-1 text-cyan-400 hover:text-cyan-300 transition underline underline-offset-2"
              >
                desienkz-slp/unbound <ExternalLink className="w-3 h-3" />
              </a>
            </div>
          </div>
        </div>
      )}
    </>
  );
};
