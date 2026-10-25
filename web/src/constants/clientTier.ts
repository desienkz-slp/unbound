// Client tier thresholds and default profiles.
// Thresholds match backend: pkg/model/config.go → ClientTierFromCount().
export const CLIENT_TIER_THRESHOLDS = {
  SMALL: 500,
  MEDIUM: 2000,
} as const;

export const MAX_CLIENT_COUNT = 5000;

/**
 * ProfileConfig mirrors the shape used by PerformanceView.applyProfile()
 * so that a tier card selection can be directly consumed by the same
 * spread-into-localConfig pattern used by the existing hardware profiles.
 */
export type ProfileConfig = {
  id: string;
  name: string;
  desc: string;
  // Hardware recommendation (informational)
  cores: number;
  ramMB: number;
  queueSize: number;
  description: string;
  // Values consumed by applyProfile() — must match existing profiles[] shape
  threads: number;
  slabs: number;
  rcvbuf: string;
  sndbuf: string;
  queriesPerThread: number;
  outgoingRange: number;
  msgCache: string;
  rrsetCache: string;
};

/**
 * Static tier profiles.
 * Values are aligned with pkg/model/config.go → GetClientTierTuning().
 */
export const TIER_PROFILES: Record<string, ProfileConfig> = {
  TIER_SMALL: {
    id: 'SMALL',
    name: 'Small Edge / Lab',
    desc: '0‑500 clients. 1‑2 vCPUs, ≤2 GB RAM. Minimal footprint.',
    cores: 2,
    ramMB: 2048,
    queueSize: 500,
    description: '0‑500 clients. 1‑2 vCPUs, ≤2 GB RAM. Minimal footprint.',
    threads: 2,
    slabs: 2,
    rcvbuf: '4m',
    sndbuf: '4m',
    queriesPerThread: 2048,
    outgoingRange: 8192,
    msgCache: '64m',
    rrsetCache: '128m',
  },
  TIER_MEDIUM: {
    id: 'MEDIUM',
    name: 'Medium Regional Node',
    desc: '501‑2000 clients. 4 vCPUs, 4‑8 GB RAM. Balanced throughput.',
    cores: 4,
    ramMB: 4096,
    queueSize: 1000,
    description: '501‑2000 clients. 4 vCPUs, 4‑8 GB RAM. Balanced throughput.',
    threads: 4,
    slabs: 4,
    rcvbuf: '8m',
    sndbuf: '8m',
    queriesPerThread: 4096,
    outgoingRange: 16384,
    msgCache: '128m',
    rrsetCache: '256m',
  },
  TIER_LARGE: {
    id: 'LARGE',
    name: 'Large Core Resolver',
    desc: '>2000 clients. 8+ vCPUs, 8‑16 GB RAM. High throughput.',
    cores: 8,
    ramMB: 8192,
    queueSize: 2000,
    description: '>2000 clients. 8+ vCPUs, 8‑16 GB RAM. High throughput.',
    threads: 8,
    slabs: 8,
    rcvbuf: '16m',
    sndbuf: '16m',
    queriesPerThread: 8192,
    outgoingRange: 32768,
    msgCache: '512m',
    rrsetCache: '1024m',
  },
};

/**
 * Mirrors backend ClientTierFromCount().
 * Returns the tier key for a given client count.
 */
export function getClientTier(clientCount: number): 'TIER_SMALL' | 'TIER_MEDIUM' | 'TIER_LARGE' {
  if (clientCount <= CLIENT_TIER_THRESHOLDS.SMALL) return 'TIER_SMALL';
  if (clientCount <= CLIENT_TIER_THRESHOLDS.MEDIUM) return 'TIER_MEDIUM';
  return 'TIER_LARGE';
}
