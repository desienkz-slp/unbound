import React, { useState } from 'react';
import {
  Activity,
  LogOut,
  Search,
  Server,
  User,
} from 'lucide-react';
import { NodeRecord } from '../types';

interface HeaderProps {
  nodes: NodeRecord[];
  selectedNode: string;
  onSelectNode: (nodeId: string) => void;
  onQuickDiagnose: (domain: string) => void;
  onLogout: () => void;
  currentUser: string;
}

export const Header: React.FC<HeaderProps> = ({
  nodes,
  selectedNode,
  onSelectNode,
  onQuickDiagnose,
  onLogout,
  currentUser,
}) => {
  const [quickDomain, setQuickDomain] = useState('');

  const handleQuickSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (quickDomain.trim()) {
      onQuickDiagnose(quickDomain.trim());
      setQuickDomain('');
    }
  };

  return (
    <header className="h-16 border-b border-gray-800 bg-[#0B0F19]/90 backdrop-blur px-6 flex items-center justify-between sticky top-0 z-30">
      {/* Brand & Node Selector */}
      <div className="flex items-center gap-6">
        <div className="flex items-center gap-3">
          <img
            src="/logo.png"
            alt="Netora DNS"
            className="w-9 h-9 object-contain drop-shadow-[0_0_14px_rgba(74,222,128,0.4)] hover:scale-105 transition-transform duration-200"
          />
          <div>
            <div className="flex items-center gap-2">
              <span className="font-bold text-base tracking-wider text-white">NETORA</span>
              <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-950/80 text-emerald-400 border border-emerald-800/80 font-mono">
                DNS NOC
              </span>
            </div>
          </div>
        </div>

        {/* Node dropdown */}
        <div className="hidden md:flex items-center gap-2 bg-gray-900/80 border border-gray-800 rounded-lg px-3 py-1.5 text-xs text-gray-300">
          <Server className="w-3.5 h-3.5 text-cyan-400" />
          <span className="text-gray-400">Node:</span>
          <select
            value={selectedNode}
            onChange={(e) => onSelectNode(e.target.value)}
            className="bg-transparent text-cyan-300 font-medium focus:outline-none cursor-pointer"
          >
            {nodes.map((n) => (
              <option key={n.id} value={n.id} className="bg-gray-900 text-gray-200">
                {n.hostname} ({n.management_ip})
              </option>
            ))}
          </select>
          <div className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse ml-1" title="Node Online" />
        </div>
      </div>

      {/* Center Search / Quick Diagnose */}
      <form onSubmit={handleQuickSubmit} className="hidden sm:flex items-center relative max-w-md w-full mx-4">
        <Search className="w-4 h-4 text-gray-400 absolute left-3 pointer-events-none" />
        <input
          type="text"
          placeholder="Quick domain diagnosis (e.g. doubleclick.net, netora.internal)..."
          value={quickDomain}
          onChange={(e) => setQuickDomain(e.target.value)}
          className="w-full bg-gray-900/90 border border-gray-800 rounded-lg pl-9 pr-4 py-1.5 text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-cyan-500 focus:ring-1 focus:ring-cyan-500 transition-all font-mono"
        />
      </form>

      {/* Right controls */}
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2 text-xs text-gray-300 bg-gray-900/60 border border-gray-800 px-3 py-1.5 rounded-lg">
          <User className="w-3.5 h-3.5 text-cyan-400" />
          <span className="font-mono text-cyan-300">{currentUser || 'admin'}</span>
          <span className="text-gray-500">|</span>
          <span className="text-emerald-400 flex items-center gap-1 font-mono">
            <Activity className="w-3 h-3" /> NOC Active
          </span>
        </div>

        <button
          onClick={onLogout}
          title="Sign out"
          className="p-2 text-gray-400 hover:text-red-400 hover:bg-gray-800/60 rounded-lg transition-colors"
        >
          <LogOut className="w-4 h-4" />
        </button>
      </div>
    </header>
  );
};
