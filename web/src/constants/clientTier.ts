// Homepass-based tier thresholds and tuning profiles.
// 1 homepass = 1 rumah pelanggan ISP ≈ 5 device (rata-rata).
// Thresholds match backend: pkg/model/config.go → ClientTierFromCount().
export const DEVICES_PER_HOMEPASS = 5;

export const HOMEPASS_TIER_THRESHOLDS = {
  SMALL: 500,   // 0–500 homepass ≈ 2.500 device
  MEDIUM: 2000, // 501–2000 homepass ≈ 10.000 device
  LARGE: 5000,  // 2001–5000 homepass ≈ 25.000 device
} as const;

export const MAX_HOMEPASS_COUNT = 10000;

/**
 * ProfileConfig contains tuning values consumed by PerformanceView.applyProfile().
 * Values are scaled for homepass density (1 homepass ≈ 5 DNS-querying devices).
 */
export type ProfileConfig = {
  id: string;
  name: string;
  desc: string;
  // Informational
  cores: number;
  ramMB: number;
  queueSize: number;
  description: string;
  homepassRange: string;
  estimatedDevices: string;
  estimatedQPS: string;
  // Values consumed by applyProfile()
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
 * Static tier profiles scaled for homepass density.
 * 1 homepass ≈ 5 device, each device ≈ 2–5 QPS peak.
 */
export const TIER_PROFILES: Record<string, ProfileConfig> = {
  TIER_SMALL: {
    id: 'SMALL',
    name: 'Small ISP / WISP',
    desc: '≤500 homepass. Cocok untuk ISP kecil, WISP, atau RT/RW Net.',
    cores: 2,
    ramMB: 2048,
    queueSize: 500,
    description: '≤500 homepass (~2.500 device). Optimized untuk ISP kecil dengan footprint minimal.',
    homepassRange: '0 – 500',
    estimatedDevices: '~2.500',
    estimatedQPS: '~5.000 – 12.500',
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
    name: 'Medium ISP Regional',
    desc: '501–2.000 homepass. Balanced untuk ISP regional atau kampus besar.',
    cores: 4,
    ramMB: 4096,
    queueSize: 1000,
    description: '501–2.000 homepass (~10.000 device). Balanced throughput & cache untuk ISP regional.',
    homepassRange: '501 – 2.000',
    estimatedDevices: '~10.000',
    estimatedQPS: '~20.000 – 50.000',
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
    name: 'Large ISP / Carrier',
    desc: '>2.000 homepass. High-throughput untuk ISP besar atau carrier grade.',
    cores: 8,
    ramMB: 8192,
    queueSize: 2000,
    description: '>2.000 homepass (~10.000+ device). Maximum throughput, deep cache, high queue depth.',
    homepassRange: '> 2.000',
    estimatedDevices: '~10.000+',
    estimatedQPS: '~50.000 – 100.000+',
    threads: 8,
    slabs: 8,
    rcvbuf: '16m',
    sndbuf: '16m',
    queriesPerThread: 8192,
    outgoingRange: 32768,
    msgCache: '512m',
    rrsetCache: '1024m',
  },
  TIER_CARRIER: {
    id: 'ISP',
    name: 'Carrier Grade ISP',
    desc: '>5.000 homepass. Maximum capacity untuk ISP besar / carrier grade.',
    cores: 16,
    ramMB: 32768,
    queueSize: 5000,
    description: '>5.000 homepass (~25.000+ device). Carrier-grade: SO_REUSEPORT, 32k queues, aggressive buffering.',
    homepassRange: '> 5.000',
    estimatedDevices: '~25.000+',
    estimatedQPS: '~100.000+',
    threads: 16,
    slabs: 16,
    rcvbuf: '32m',
    sndbuf: '32m',
    queriesPerThread: 16384,
    outgoingRange: 65536,
    msgCache: '1024m',
    rrsetCache: '2048m',
  },
};

/**
 * Mirrors backend ClientTierFromCount().
 * Input: jumlah homepass (bukan device).
 */
export function getClientTier(homepassCount: number): 'TIER_SMALL' | 'TIER_MEDIUM' | 'TIER_LARGE' | 'TIER_CARRIER' {
  if (homepassCount <= HOMEPASS_TIER_THRESHOLDS.SMALL) return 'TIER_SMALL';
  if (homepassCount <= HOMEPASS_TIER_THRESHOLDS.MEDIUM) return 'TIER_MEDIUM';
  if (homepassCount <= HOMEPASS_TIER_THRESHOLDS.LARGE) return 'TIER_LARGE';
  return 'TIER_CARRIER';
}
