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
 * Displays a single client-tier profile card.
 * Does NOT call the API directly — delegates selection to the parent
 * (PerformanceView), which spreads localConfig and saves via applyConfig(nodeId, state, reason).
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
      <h3 className="text-xl font-semibold mb-2">{profile.name}</h3>
      <p className="text-sm text-gray-300 mb-4">{profile.description}</p>
      <ul className="mb-4 space-y-1 text-sm">
        <li>🧮 Cores: {profile.cores}</li>
        <li>💾 RAM: {profile.ramMB / 1024} GB</li>
        <li>📊 Queue size: {profile.queueSize}</li>
      </ul>
      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); onSelect(profile); }}
        className={`px-4 py-2 rounded text-white transition-colors ${
          selected
            ? 'bg-green-600 hover:bg-green-500'
            : 'bg-indigo-600 hover:bg-indigo-500'
        }`}
      >
        {selected ? '✓ Selected' : 'Select'}
      </button>
    </div>
  );
};
