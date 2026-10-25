import React from 'react';
import { ClientTier } from '../types';

interface TierBadgeProps {
  tier: ClientTier;
}

const TIER_META: Record<ClientTier, { label: string; gradVar: string }> = {
  TIER_SMALL:  { label: 'Small',  gradVar: 'var(--bg-grad-small)'  },
  TIER_MEDIUM: { label: 'Medium', gradVar: 'var(--bg-grad-medium)' },
  TIER_LARGE:  { label: 'Large',  gradVar: 'var(--bg-grad-large)'  },
};

export const TierBadge: React.FC<TierBadgeProps> = ({ tier }) => {
  const meta = TIER_META[tier] ?? { label: 'Unknown', gradVar: '#666' };

  return (
    <span
      className="tier-badge"
      style={{ background: meta.gradVar }}
    >
      {meta.label} Tier
    </span>
  );
};

