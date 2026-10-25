import React from 'react';
import { ProfileConfig } from '../constants/clientTier';

interface TierProfileCardProps {
  tier: string;
  profile: ProfileConfig;
  isRecommended: boolean;
  onSelect: (profile: ProfileConfig) => void;
  selected: boolean;
}

/**
 * Displays a single homepass-tier profile card.
 * Delegates selection to the parent (PerformanceView).
 */
export const TierProfileCard: React.FC<TierProfileCardProps> = ({
  tier: _tier,
  profile,
  isRecommended,
  onSelect,
  selected,
}) => {
  return (
    <div
      className={`glass-card cursor-pointer ${selected ? 'ring-2 ring-indigo-400' : ''}`}
      onClick={() => onSelect(profile)}
    >
      {isRecommended && (
        <span className="text-xs font-semibold px-2 py-0.5 rounded bg-green-600 text-white mb-2 inline-block">
          ⚡ Recommended
        </span>
      )}
      <h3 className="text-lg font-semibold mb-1">{profile.name}</h3>
      <p className="text-sm text-gray-300 mb-3">{profile.desc}</p>

      <div className="space-y-1.5 text-xs font-mono mb-4">
        <div className="flex justify-between text-gray-400">
          <span>Homepass</span>
          <span className="text-cyan-400 font-bold">{profile.homepassRange}</span>
        </div>
        <div className="flex justify-between text-gray-400">
          <span>Est. Device</span>
          <span className="text-white">{profile.estimatedDevices}</span>
        </div>
        <div className="flex justify-between text-gray-400">
          <span>Est. QPS</span>
          <span className="text-white">{profile.estimatedQPS}</span>
        </div>
        <div className="flex justify-between text-gray-400">
          <span>Threads</span>
          <span className="text-cyan-400">{profile.threads}</span>
        </div>
        <div className="flex justify-between text-gray-400">
          <span>Cache</span>
          <span className="text-white">{profile.rrsetCache}</span>
        </div>
        <div className="flex justify-between text-gray-400">
          <span>Socket Buffer</span>
          <span className="text-white">{profile.rcvbuf}</span>
        </div>
      </div>

      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); onSelect(profile); }}
        className={`w-full px-4 py-2 rounded text-white font-semibold text-sm transition-colors ${
          selected
            ? 'bg-green-600 hover:bg-green-500'
            : 'bg-indigo-600 hover:bg-indigo-500'
        }`}
      >
        {selected ? '✓ Selected' : 'Pilih Profil'}
      </button>
    </div>
  );
};
