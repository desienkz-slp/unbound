package model

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// ListenerRole defines the operational purpose of a network listener.
type ListenerRole string

const (
	RoleInternet   ListenerRole = "INTERNET"   // Optional public listener; strictly ACL protected
	RoleClient     ListenerRole = "CLIENT"     // Customer/subscriber DNS listener
	RoleManagement ListenerRole = "MANAGEMENT" // Loopback / administrative / telemetry listener
	RoleInternal   ListenerRole = "INTERNAL"   // Trusted infrastructure network listener
)

// ResolutionMode defines how Unbound resolves queries.
type ResolutionMode string

const (
	ModeRecursive  ResolutionMode = "RECURSIVE"  // Full root-hints recursive resolution with DNSSEC
	ModeForwarding ResolutionMode = "FORWARDING" // Forwards to upstream resolvers (DoH/DoT/UDP/TCP)
	ModeHybrid     ResolutionMode = "HYBRID"     // Forward first, fallback to recursive on upstream failure
)

// UpstreamType defines the protocol used to communicate with an upstream resolver.
type UpstreamType string

const (
	UpstreamUDP UpstreamType = "UDP"
	UpstreamTCP UpstreamType = "TCP"
	UpstreamDoT UpstreamType = "DOT"
	UpstreamDoH UpstreamType = "DOH"
)

// PerformanceProfile defines standard resource allocation presets.
type PerformanceProfile string

const (
	ProfileSmall  PerformanceProfile = "SMALL"  // <= 1GB RAM, 1-2 cores
	ProfileMedium PerformanceProfile = "MEDIUM" // 2-4GB RAM, 2-4 cores
	ProfileLarge  PerformanceProfile = "LARGE"  // 4-8GB RAM, 4-8 cores
	ProfileISP    PerformanceProfile = "ISP"    // 16GB+ RAM, 8+ cores (Extreme throughput)
	ProfileCustom PerformanceProfile = "CUSTOM" // Explicit user-defined parameters
)

// ClientTier defines a subscriber density tier that drives Unbound tuning.
// Tiers are based on the number of unique subscriber IPs, not concurrent queries.
type ClientTier string

const (
	// TierSmall covers small deployments: 0–500 clients.
	// Optimizes for low memory footprint and minimal buffer overhead.
	TierSmall ClientTier = "TIER_SMALL"

	// TierMedium covers mid-size ISP/campus deployments: 501–2000 clients.
	// Balances throughput and resource efficiency.
	TierMedium ClientTier = "TIER_MEDIUM"

	// TierLarge covers large ISP deployments: > 2000 clients.
	// Prioritizes maximum throughput, large cache, and high queue depth.
	TierLarge ClientTier = "TIER_LARGE"
)

// ClientTierFromCount returns the appropriate ClientTier for a given subscriber count.
func ClientTierFromCount(clientCount int) ClientTier {
	switch {
	case clientCount <= 500:
		return TierSmall
	case clientCount <= 2000:
		return TierMedium
	default:
		return TierLarge
	}
}

// Listener represents a specific IP/Port binding on the node.
type Listener struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	IP       string       `json:"ip"`
	Port     int          `json:"port"`
	Role     ListenerRole `json:"role"`
	IsIPv6   bool         `json:"is_ipv6"`
	UDP      bool         `json:"udp"`
	TCP      bool         `json:"tcp"`
	Enabled  bool         `json:"enabled"`
	ACLRule  string       `json:"acl_rule"` // Optional override (e.g. "allow", "refuse")
}

// ClientNetwork defines an IP subnet permitted to query the resolver.
type ClientNetwork struct {
	ID          string `json:"id"`
	CIDR        string `json:"cidr"`
	Description string `json:"description"`
	Action      string `json:"action"` // "allow", "refuse", "deny", "allow_snoop"
	PolicyGroup string `json:"policy_group"`
	Enabled     bool   `json:"enabled"`
}

// UpstreamServer represents an upstream DNS server.
type UpstreamServer struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	IP          string       `json:"ip"`
	Port        int          `json:"port"`
	Type        UpstreamType `json:"type"`
	TLSName     string       `json:"tls_name,omitempty"`
	InsecureTLS bool         `json:"insecure_tls,omitempty"`
	Weight      int          `json:"weight,omitempty"`
	Enabled     bool         `json:"enabled"`
}

// CacheConfig controls memory cache allocation in Unbound.
type CacheConfig struct {
	MsgCacheSize    string `json:"msg_cache_size"`    // e.g. "128m"
	RRSetCacheSize  string `json:"rrset_cache_size"`  // e.g. "256m"
	KeyCacheSize    string `json:"key_cache_size"`    // e.g. "64m"
	NegCacheSize    string `json:"neg_cache_size"`    // e.g. "64m"
	MinTTL          int    `json:"min_ttl"`           // seconds (e.g. 30)
	MaxTTL          int    `json:"max_ttl"`           // seconds (e.g. 86400)
	InfraHostTTL    int    `json:"infra_host_ttl"`    // seconds (e.g. 900)
}

// PerformanceConfig controls CPU multithreading and networking buffers.
type PerformanceConfig struct {
	Profile             PerformanceProfile `json:"profile"`
	NumThreads          int                `json:"num_threads"`
	MsgCacheSlabs       int                `json:"msg_cache_slabs"`
	RRSetCacheSlabs     int                `json:"rrset_cache_slabs"`
	KeyCacheSlabs       int                `json:"key_cache_slabs"`
	InfraCacheSlabs     int                `json:"infra_cache_slabs"`
	NumQueriesPerThread int                `json:"num_queries_per_thread"`
	OutgoingRange       int                `json:"outgoing_range"`
	IncomingNumTCP      int                `json:"incoming_num_tcp"`
	JostleTimeout       int                `json:"jostle_timeout"` // timeout in msec when overloaded to prevent drops (default 200)
	SoRcvBuf            string             `json:"so_rcvbuf"`       // e.g. "8m"
	SoSndBuf            string             `json:"so_sndbuf"`       // e.g. "8m"
	SoReuseport         bool               `json:"so_reuseport"`
}

// defaultCacheSlabs is used when a client omits a slabs value (zero in JSON).
const defaultCacheSlabs = 4

// defaultJostleTimeout prevents slow/stalled upstream queries from starving the requestlist buffer.
const defaultJostleTimeout = 200

// floorPow2 returns the largest power of two <= n, or def when n < 1.
// Unbound rejects slabs that are not a power of two.
func floorPow2(n, def int) int {
	if n < 1 {
		return def
	}
	p := 1
	for p*2 <= n {
		p *= 2
	}
	return p
}

// NormalizeSlabs fills missing slabs from the message-cache value and snaps all
// slabs to a power of two, so clients that omit fields cannot break checkconf.
func (p *PerformanceConfig) NormalizeSlabs() {
	msg := floorPow2(p.MsgCacheSlabs, defaultCacheSlabs)
	p.MsgCacheSlabs = msg
	p.RRSetCacheSlabs = floorPow2(p.RRSetCacheSlabs, msg)
	p.KeyCacheSlabs = floorPow2(p.KeyCacheSlabs, msg)
	p.InfraCacheSlabs = floorPow2(p.InfraCacheSlabs, msg)
}

// defaultIncomingNumTCP matches the non-ISP AutoTunePerformance value. Unbound
// with incoming-num-tcp 0 accepts no TCP queries and floods the log with
// "accepted too many tcp, connections full".
const defaultIncomingNumTCP = 1024

// Normalize repairs fields that clients may omit (zero in JSON) so the rendered
// unbound.conf is always valid and functional.
func (p *PerformanceConfig) Normalize() {
	p.NormalizeSlabs()
	if p.IncomingNumTCP < 1 {
		p.IncomingNumTCP = defaultIncomingNumTCP
	}
	if p.JostleTimeout < 1 {
		p.JostleTimeout = defaultJostleTimeout
	}
	if p.NumQueriesPerThread < 1024 {
		p.NumQueriesPerThread = 4096
	}
	if p.OutgoingRange < 4096 {
		p.OutgoingRange = 16384
	}
}

// DNSSECConfig defines DNSSEC validation settings and audited exceptions.
type DNSSECConfig struct {
	Enabled             bool     `json:"enabled"`
	AutoTrustAnchorFile string   `json:"auto_trust_anchor_file"` // e.g. "/var/lib/unbound/root.key"
	RootHintsFile       string   `json:"root_hints_file"`         // e.g. "/var/lib/unbound/root.hints"
	HardenGlue          bool     `json:"harden_glue"`
	HardenDNSSECStripped bool    `json:"harden_dnssec_stripped"`
	ValPermissiveMode   bool     `json:"val_permissive_mode"`
	InsecureDomains     []string `json:"insecure_domains"` // Explicit domain-insecure exceptions
}

// FeatureConfig defines resilience, privacy, and lookup optimizations.
type FeatureConfig struct {
	Prefetch                 bool `json:"prefetch"`
	PrefetchKey              bool `json:"prefetch_key"`
	ServeExpired             bool `json:"serve_expired"`
	ServeExpiredTTL          int  `json:"serve_expired_ttl"`           // e.g. 86400
	ServeExpiredTTLReset     bool `json:"serve_expired_ttl_reset"`     // Reset expired TTL on continuous access
	ServeExpiredClientTimeout int  `json:"serve_expired_client_timeout"` // e.g. 1800 ms
	ServeExpiredReplyTTL     int  `json:"serve_expired_reply_ttl"`     // e.g. 30
	QnameMinimisation        bool `json:"qname_minimisation"`
	HideIdentity             bool `json:"hide_identity"`
	HideVersion              bool `json:"hide_version"`
	LogQueries               bool `json:"log_queries"` // Enable detailed per-query file logging (default false for high-throughput ISP)
	LogReplies               bool `json:"log_replies"` // Enable per-reply resolution time logging (default false for high-throughput ISP)
}

// RateLimitConfig defines Anti-DDoS and Response Rate Limiting (RRL) protection.
type RateLimitConfig struct {
	Enabled           bool   `json:"enabled"`
	IPRateLimit       int    `json:"ip_rate_limit"`        // Max QPS per subscriber IP (e.g. 500)
	IPRateLimitFactor int    `json:"ip_rate_limit_factor"` // 10 = reply 1/10 queries with TC bit (force TCP), drop others
	DomainRateLimit   int    `json:"domain_rate_limit"`    // Max QPS per target domain (e.g. 2000)
	RateLimitSize     string `json:"rate_limit_size"`      // Memory hash table size (e.g. "16m")
	RateLimitSlabs    int    `json:"rate_limit_slabs"`     // Multi-core slab locks (e.g. 4)
}

// SafeSearchConfig defines search engine safety enforcement.
type SafeSearchConfig struct {
	Enabled            bool   `json:"enabled"`
	GoogleRedirectIPv4 string `json:"google_redirect_ipv4"` // 216.239.38.120 (forcesafesearch.google.com)
	BingRedirectIPv4   string `json:"bing_redirect_ipv4"`   // 204.79.197.220 (strict.bing.com)
	DuckDuckGoVIP      string `json:"duckduckgo_vip"`       // safe.duckduckgo.com
	YouTubeRestrict    string `json:"youtube_restrict"`     // restrict.youtube.com
}

// ParentalControlConfig controls adult and 18+ content blocking at DNS level.
type ParentalControlConfig struct {
	Enabled       bool   `json:"enabled"`
	RedirectIP    string `json:"redirect_ip,omitempty"` // e.g. "172.18.20.234" or empty for NXDOMAIN sinkholing
	BlockAdult    bool   `json:"block_adult"`
	BlockGambling bool   `json:"block_gambling"`
}

// GamblingBlockConfig controls online gambling (judi online / slot / betting) blocking at DNS level.
type GamblingBlockConfig struct {
	Enabled    bool   `json:"enabled"`
	RedirectIP string `json:"redirect_ip,omitempty"` // e.g. "172.18.20.234" or empty for NXDOMAIN sinkholing
}

// FilterBlockConfig controls category-level DNS blocking and sinkholing.
type FilterBlockConfig struct {
	Enabled    bool   `json:"enabled"`
	RedirectIP string `json:"redirect_ip,omitempty"`
}

// LocalRecord defines custom local DNS authoritative answers.
type LocalRecord struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`  // "A", "AAAA", "CNAME", "PTR", "TXT"
	Value  string `json:"value"`
	TTL    int    `json:"ttl"`
}

// RemoteControlConfig defines the private loopback management control port.
type RemoteControlConfig struct {
	Enabled        bool   `json:"enabled"`
	Interface      string `json:"interface"` // Default "127.0.0.1" (NEVER 0.0.0.0)
	Port           int    `json:"port"`      // Default 8953
	ControlUseCert bool   `json:"control_use_cert"`
}

// NodeMetadata captures physical/VM machine information.
type NodeMetadata struct {
	NodeID       string    `json:"node_id"`
	Hostname     string    `json:"hostname"`
	OS           string    `json:"os"`
	UnboundVersion string  `json:"unbound_version"`
	CPUCores     int       `json:"cpu_cores"`
	TotalRAMMB   int       `json:"total_ram_mb"`
	DiskTotalGB  int       `json:"disk_total_gb"`
	DiskFreeGB   int       `json:"disk_free_gb"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// LogrotateConfig defines the automated and custom log rotation policy for the node host.
type LogrotateConfig struct {
	Enabled            bool   `json:"enabled"`
	Frequency          string `json:"frequency"`             // "daily", "weekly", "monthly"
	RotateCount        int    `json:"rotate_count"`          // 1 - 90
	MaxSize            string `json:"max_size"`              // e.g. "25M", "50M", "100M"
	Compress           bool   `json:"compress"`              // gzip
	DelayCompress      bool   `json:"delay_compress"`        // delaycompress
	RotateNetora       bool   `json:"rotate_netora"`         // rotate /var/log/netora-dns/*.log
	AutoPurge          bool   `json:"auto_purge"`            // NVR-style: auto-delete oldest .gz when disk threshold reached
	AutoPurgeThreshold int    `json:"auto_purge_threshold"`  // Disk usage % trigger: 50, 75, or 90 (default 75)
}

// FeedConfig defines a blocklist feed source configurable per-node.
type FeedConfig struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	RepoFilename string   `json:"repo_filename"`
	FallbackURLs []string `json:"fallback_urls"`
	Enabled      bool     `json:"enabled"`
}

// DesiredState is the comprehensive desired configuration for a DNS node.
type DesiredState struct {
	VersionID        string                `json:"version_id"`
	NodeID           string                `json:"node_id"`
	ResolutionMode   ResolutionMode        `json:"resolution_mode"`
	Listeners        []Listener            `json:"listeners"`
	ClientNetworks   []ClientNetwork       `json:"client_networks"`
	Upstreams        []UpstreamServer      `json:"upstreams"`
	Cache            CacheConfig           `json:"cache"`
	Performance      PerformanceConfig     `json:"performance"`
	DNSSEC           DNSSECConfig          `json:"dnssec"`
	Features         FeatureConfig         `json:"features"`
	RateLimit        RateLimitConfig       `json:"rate_limit"`
	SafeSearch       SafeSearchConfig      `json:"safe_search"`
	ParentalControl  ParentalControlConfig `json:"parental_control"`
	GamblingBlock    GamblingBlockConfig   `json:"gambling_block"`
	AdBlock          FilterBlockConfig     `json:"ad_block"`
	MalwareBlock     FilterBlockConfig     `json:"malware_block"`
	CustomBlock      FilterBlockConfig     `json:"custom_block"`
	LocalRecords     []LocalRecord         `json:"local_records"`
	RemoteControl    RemoteControlConfig   `json:"remote_control"`
	Logrotate        LogrotateConfig       `json:"logrotate"`
	IncludeFiles     []string              `json:"include_files,omitempty"`
	BlocklistFeeds   []FeedConfig          `json:"blocklist_feeds,omitempty"`

	// ClientCount is the total number of unique subscriber IPs managed by this node.
	// When non-zero, AutoTuneByClientCount() overrides the hardware-only tuning
	// with parameters optimised for the actual subscriber density.
	// Valid tiers: 0–500 → TIER_SMALL, 501–2000 → TIER_MEDIUM, >2000 → TIER_LARGE.
	ClientCount int `json:"client_count,omitempty"`
}

// NewDefaultDesiredState returns a safe, production-grade baseline desired state.
// Pass clientCount > 0 to seed client-tier tuning from the start; use 0 for
// a pure hardware-only default (tier can be applied later via ApplyClientTier).
func NewDefaultDesiredState(nodeID string, cores int, ramMB int) *DesiredState {
	perf, cache := AutoTuneByClientCount(0, cores, ramMB)
	// Use TierMedium rate-limit defaults so the baseline is reasonably conservative.
	defaultTier := clientTierTable[TierMedium]

	return &DesiredState{
		VersionID:      fmt.Sprintf("v-%d", time.Now().Unix()),
		NodeID:         nodeID,
		ResolutionMode: ModeRecursive,
		Logrotate: LogrotateConfig{
			Enabled:       true,
			Frequency:     "daily",
			RotateCount:   7,
			MaxSize:       "50M",
			Compress:      true,
			DelayCompress: true,
			RotateNetora:  true,
		},
		Listeners: []Listener{
			{
				ID:      "listen-mgmt",
				Name:    "Management Loopback",
				IP:      "127.0.0.1",
				Port:    53,
				Role:    RoleManagement,
				IsIPv6:  false,
				UDP:     true,
				TCP:     true,
				Enabled: true,
			},
			{
				ID:      "listen-client-v4",
				Name:    "Client DNS IPv4",
				IP:      "0.0.0.0",
				Port:    53,
				Role:    RoleClient,
				IsIPv6:  false,
				UDP:     true,
				TCP:     true,
				Enabled: true,
			},
		},
		ClientNetworks: []ClientNetwork{
			{
				ID:          "net-local",
				CIDR:        "127.0.0.0/8",
				Description: "Localhost Loopback",
				Action:      "allow",
				Enabled:     true,
			},
			{
				ID:          "net-mgmt",
				CIDR:        "172.18.20.0/24",
				Description: "Management & Internal Subnet",
				Action:      "allow",
				Enabled:     true,
			},
			{
				ID:          "net-rfc1918-1",
				CIDR:        "192.168.0.0/16",
				Description: "Client Subnet 192.168.0.0/16",
				Action:      "allow",
				Enabled:     true,
			},
			{
				ID:          "net-rfc1918-2",
				CIDR:        "10.0.0.0/8",
				Description: "Client Subnet 10.0.0.0/8",
				Action:      "allow",
				Enabled:     true,
			},
		},
		Upstreams: []UpstreamServer{
			{
				ID:      "up-cf-1",
				Name:    "Cloudflare Primary",
				IP:      "1.1.1.1",
				Port:    53,
				Type:    UpstreamUDP,
				Enabled: true,
			},
			{
				ID:      "up-goog-1",
				Name:    "Google Primary",
				IP:      "8.8.8.8",
				Port:    53,
				Type:    UpstreamUDP,
				Enabled: true,
			},
		},
		Cache:       cache,
		Performance: perf,
		DNSSEC: DNSSECConfig{
			Enabled:              true,
			AutoTrustAnchorFile:  "/var/lib/unbound/root.key",
			RootHintsFile:        "/var/lib/unbound/root.hints",
			HardenGlue:           true,
			HardenDNSSECStripped: true,
			ValPermissiveMode:    false,
			InsecureDomains:      []string{},
		},
		Features: FeatureConfig{
			Prefetch:                  true,
			PrefetchKey:               true,
			ServeExpired:              true,
			ServeExpiredTTL:           86400,
			ServeExpiredTTLReset:      true,
			ServeExpiredClientTimeout: 1800,
			ServeExpiredReplyTTL:      30,
			QnameMinimisation:         false, // Disabled by default for ISP resolvers to avoid NXDOMAIN on non-compliant CDN GSLBs (e.g. Baishan, Shopee)
			HideIdentity:              true,
			HideVersion:               true,
			LogQueries:                false, // Disabled by default for ISP performance (prevents disk bloat)
			LogReplies:                false, // Disabled by default for ISP performance (prevents disk bloat)
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			IPRateLimit:       defaultTier.IPRateLimit,
			IPRateLimitFactor: 10,
			DomainRateLimit:   defaultTier.DomainRateLimit,
			RateLimitSize:     defaultTier.RateLimitSize,
			RateLimitSlabs:    perf.MsgCacheSlabs,
		},
		SafeSearch: SafeSearchConfig{
			Enabled:            true,
			GoogleRedirectIPv4: "216.239.38.120",
			BingRedirectIPv4:   "204.79.197.220",
			DuckDuckGoVIP:      "safe.duckduckgo.com",
			YouTubeRestrict:    "restrict.youtube.com",
		},
		ParentalControl: ParentalControlConfig{
			Enabled:       false,
			BlockAdult:    true,
			BlockGambling: false,
		},
		GamblingBlock: GamblingBlockConfig{
			Enabled:    false,
			RedirectIP: "",
		},
		AdBlock: FilterBlockConfig{
			Enabled:    true,
			RedirectIP: "",
		},
		MalwareBlock: FilterBlockConfig{
			Enabled:    true,
			RedirectIP: "",
		},
		CustomBlock: FilterBlockConfig{
			Enabled:    true,
			RedirectIP: "",
		},
		LocalRecords: []LocalRecord{},
		RemoteControl: RemoteControlConfig{
			Enabled:        true,
			Interface:      "127.0.0.1",
			Port:           8953,
			ControlUseCert: false,
		},
	}
}

// ApplyClientTier re-tunes an existing DesiredState for a given subscriber count
// without touching any protected operational fields (credentials, ACLs, blocklists,
// upstream resolvers, local records, or SafeSearch/parental-control settings).
// It is safe to call on a live state snapshot loaded from the database.
func (s *DesiredState) ApplyClientTier(clientCount, cores, ramMB int) {
	if s == nil || clientCount < 0 {
		return
	}
	s.ClientCount = clientCount

	perf, cache := AutoTuneByClientCount(clientCount, cores, ramMB)
	// Preserve thread count and slabs from hardware baseline (caller may have overridden them).
	perf.NumThreads = s.Performance.NumThreads
	perf.MsgCacheSlabs = s.Performance.MsgCacheSlabs
	perf.RRSetCacheSlabs = s.Performance.RRSetCacheSlabs
	perf.KeyCacheSlabs = s.Performance.KeyCacheSlabs
	perf.InfraCacheSlabs = s.Performance.InfraCacheSlabs

	s.Performance = perf
	s.Cache = cache

	// Update rate-limit parameters from the new tier.
	t := GetClientTierTuning(clientCount)
	s.RateLimit.IPRateLimit = t.IPRateLimit
	s.RateLimit.DomainRateLimit = t.DomainRateLimit
	s.RateLimit.RateLimitSize = t.RateLimitSize
	s.RateLimit.RateLimitSlabs = perf.MsgCacheSlabs
}

// EnsureNodeDnsRewrites verifies and ensures that the DesiredState contains default DNS rewrite (A records)
// matching the node's IP addresses.
// If the node has 2 IPs, it will have 2 DNS rewrites; if 1 IP, it will have 1 rewrite.
// Existing custom rewrites for other domains are strictly preserved.
func (s *DesiredState) EnsureNodeDnsRewrites(nodeName string, ips []string) bool {
	if s == nil {
		return false
	}
	if nodeName == "" {
		nodeName = s.NodeID
	}
	if nodeName == "" {
		return false
	}

	// If ips slice is not provided, extract concrete IPv4 addresses from s.Listeners
	if len(ips) == 0 {
		seenIP := make(map[string]bool)
		for _, l := range s.Listeners {
			ip := strings.TrimSpace(l.IP)
			if ip != "" && ip != "0.0.0.0" && ip != "127.0.0.1" && !l.IsIPv6 && !strings.Contains(ip, ":") && !seenIP[ip] {
				seenIP[ip] = true
				ips = append(ips, ip)
			}
		}
	}

	// Clean and deduplicate target IPs
	var targetIPs []string
	seen := make(map[string]bool)
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip != "" && ip != "0.0.0.0" && ip != "127.0.0.1" && !strings.Contains(ip, ":") && !seen[ip] {
			seen[ip] = true
			targetIPs = append(targetIPs, ip)
		}
	}

	if len(targetIPs) == 0 {
		return false
	}

	// Find existing A and PTR records for this nodeName
	existingA := make(map[string]bool)
	existingPTR := make(map[string]bool)
	for _, rec := range s.LocalRecords {
		if strings.EqualFold(rec.Domain, nodeName) {
			switch strings.ToUpper(rec.Type) {
			case "A":
				existingA[rec.Value] = true
			case "PTR":
				existingPTR[rec.Value] = true
			}
		}
	}

	// If all target IPs already have both A and PTR records, no change needed
	if len(existingA) == len(targetIPs) && len(existingPTR) == len(targetIPs) {
		allMatch := true
		for _, ip := range targetIPs {
			if !existingA[ip] || !existingPTR[ip] {
				allMatch = false
				break
			}
		}
		if allMatch {
			return false
		}
	}

	// Reconstruct LocalRecords: preserve all other domains, and replace nodeName A+PTR records with targetIPs
	var newRecords []LocalRecord
	for _, rec := range s.LocalRecords {
		if strings.EqualFold(rec.Domain, nodeName) && (strings.ToUpper(rec.Type) == "A" || strings.ToUpper(rec.Type) == "PTR") {
			continue
		}
		newRecords = append(newRecords, rec)
	}

	// Append target IP rewrites for nodeName (A + PTR for nslookup reverse DNS)
	for _, ip := range targetIPs {
		newRecords = append(newRecords, LocalRecord{
			Domain: nodeName,
			Type:   "A",
			Value:  ip,
			TTL:    300,
		})
		newRecords = append(newRecords, LocalRecord{
			Domain: nodeName,
			Type:   "PTR",
			Value:  ip,
			TTL:    3600,
		})
	}

	s.LocalRecords = newRecords
	return true
}

// ClientTierTuning holds the Unbound performance and cache overrides for a specific client tier.
// Values are applied on top of the hardware-derived baseline from AutoTunePerformance.
type ClientTierTuning struct {
	Tier                ClientTier
	NumQueriesPerThread int
	OutgoingRange       int
	IncomingNumTCP      int
	SoRcvBuf            string
	SoSndBuf            string
	CacheMinTTL         int    // seconds — higher = better hit rate for small fleets
	IPRateLimit         int    // QPS limit per subscriber IP
	DomainRateLimit     int    // QPS limit per target domain
	RateLimitSize       string // hash table memory
	InfraHostTTL        int    // seconds
}

// clientTierTable is the single source-of-truth for all client-tier tuning.
// Tier boundaries: TierSmall 0–500 | TierMedium 501–2000 | TierLarge >2000.
var clientTierTable = map[ClientTier]ClientTierTuning{
	TierSmall: {
		Tier:                TierSmall,
		NumQueriesPerThread: 1024,  // Low queue depth — avoids wasted RAM for small fleet
		OutgoingRange:       4096,  // Max concurrent outgoing sockets
		IncomingNumTCP:      256,   // TCP DNS rare at this scale
		SoRcvBuf:            "2m",  // OS socket receive buffer
		SoSndBuf:            "2m",  // OS socket send buffer
		CacheMinTTL:         60,    // Shorter TTL floor → faster propagation
		IPRateLimit:         200,   // 200 QPS/subscriber (small fleet, be generous)
		DomainRateLimit:     1000,  // 1000 QPS/domain
		RateLimitSize:       "4m",  // Small hash table
		InfraHostTTL:        900,
	},
	TierMedium: {
		Tier:                TierMedium,
		NumQueriesPerThread: 4096,  // Medium queue — handles burst from 500–2K clients
		OutgoingRange:       8192,  // Concurrent outgoing sockets
		IncomingNumTCP:      512,   // Moderate TCP allowance
		SoRcvBuf:            "4m",
		SoSndBuf:            "4m",
		CacheMinTTL:         45,    // Balance between freshness and hit rate
		IPRateLimit:         500,   // 500 QPS/subscriber
		DomainRateLimit:     2000,
		RateLimitSize:       "8m",
		InfraHostTTL:        900,
	},
	TierLarge: {
		Tier:                TierLarge,
		NumQueriesPerThread: 8192,  // Full depth — handles >2K concurrent subscribers
		OutgoingRange:       32768, // Max concurrent outgoing sockets
		IncomingNumTCP:      2048,  // Higher TCP for large deployments
		SoRcvBuf:            "8m",
		SoSndBuf:            "8m",
		CacheMinTTL:         30,    // Lower floor — CDN TTLs matter at scale
		IPRateLimit:         1000,  // 1000 QPS/subscriber (shared resolver)
		DomainRateLimit:     5000,
		RateLimitSize:       "16m",
		InfraHostTTL:        1800,  // Longer infra TTL reduces upstream churn
	},
}

// GetClientTierTuning returns the canonical tuning parameters for a given client count.
// Falls back to TierMedium if the tier is unrecognised.
func GetClientTierTuning(clientCount int) ClientTierTuning {
	tier := ClientTierFromCount(clientCount)
	if t, ok := clientTierTable[tier]; ok {
		return t
	}
	return clientTierTable[TierMedium]
}

// AutoTuneByClientCount generates PerformanceConfig and CacheConfig optimised for the
// given subscriber count and hardware. The client tier sets queue/buffer parameters
// while the hardware baseline (cores, RAM) sets thread count and cache sizes.
// If clientCount is 0, it falls back to pure hardware-based tuning.
func AutoTuneByClientCount(clientCount, cores, ramMB int) (PerformanceConfig, CacheConfig) {
	perf := AutoTunePerformance(cores, ramMB)
	cache := AutoTuneCache(ramMB)

	if clientCount <= 0 {
		return perf, cache
	}

	t := GetClientTierTuning(clientCount)

	// Override hardware-derived buffer parameters with client-tier values
	perf.NumQueriesPerThread = t.NumQueriesPerThread
	perf.OutgoingRange = t.OutgoingRange
	perf.IncomingNumTCP = t.IncomingNumTCP
	perf.SoRcvBuf = t.SoRcvBuf
	perf.SoSndBuf = t.SoSndBuf

	// Apply cache TTL floor and infra TTL from tier
	cache.MinTTL = t.CacheMinTTL
	cache.InfraHostTTL = t.InfraHostTTL

	return perf, cache
}

// AutoTuneCache computes safe cache allocations based on available RAM in MB.
func AutoTuneCache(ramMB int) CacheConfig {
	if ramMB <= 512 {
		return CacheConfig{
			MsgCacheSize:   "32m",
			RRSetCacheSize: "64m",
			KeyCacheSize:   "16m",
			NegCacheSize:   "16m",
			MinTTL:         30,
			MaxTTL:         86400,
			InfraHostTTL:   900,
		}
	} else if ramMB <= 1024 {
		return CacheConfig{
			MsgCacheSize:   "64m",
			RRSetCacheSize: "128m",
			KeyCacheSize:   "32m",
			NegCacheSize:   "32m",
			MinTTL:         30,
			MaxTTL:         86400,
			InfraHostTTL:   900,
		}
	} else if ramMB <= 2048 {
		return CacheConfig{
			MsgCacheSize:   "128m",
			RRSetCacheSize: "256m",
			KeyCacheSize:   "64m",
			NegCacheSize:   "64m",
			MinTTL:         30,
			MaxTTL:         86400,
			InfraHostTTL:   900,
		}
	} else if ramMB <= 4096 {
		return CacheConfig{
			MsgCacheSize:   "256m",
			RRSetCacheSize: "512m",
			KeyCacheSize:   "128m",
			NegCacheSize:   "128m",
			MinTTL:         30,
			MaxTTL:         86400,
			InfraHostTTL:   900,
		}
	} else if ramMB <= 8192 {
		return CacheConfig{
			MsgCacheSize:   "512m",
			RRSetCacheSize: "1024m",
			KeyCacheSize:   "256m",
			NegCacheSize:   "256m",
			MinTTL:         30,
			MaxTTL:         86400,
			InfraHostTTL:   900,
		}
	}
	return CacheConfig{
		MsgCacheSize:   "1024m",
		RRSetCacheSize: "2048m",
		KeyCacheSize:   "512m",
		NegCacheSize:   "512m",
		MinTTL:         30,
		MaxTTL:         86400,
		InfraHostTTL:   900,
	}
}

// AutoTunePerformance calculates threading, slabs, and buffer sizes for CPU and RAM.
func AutoTunePerformance(cores int, ramMB int) PerformanceConfig {
	// Unbound requires slabs to be power-of-two close to the thread count.
	threads := cores
	if threads < 1 {
		threads = 1
	}
	slabs := 1
	for slabs < threads {
		slabs *= 2
	}

	profile := ProfileMedium
	if ramMB <= 1024 {
		profile = ProfileSmall
	} else if ramMB >= 8192 && cores >= 4 {
		profile = ProfileISP
	} else if ramMB >= 4096 {
		profile = ProfileLarge
	}

	// Hardware baseline for queue depth and buffer sizes (may be overridden by client tier).
	numQueries := 4096
	outgoing := 16384
	incomingTCP := 2048
	rcvBuf := "8m"
	sndBuf := "8m"
	jostleTimeout := defaultJostleTimeout

	switch profile {
	case ProfileSmall:
		numQueries = 2048
		outgoing = 8192
		incomingTCP = 1024
		rcvBuf = "4m"
		sndBuf = "4m"
	case ProfileISP, ProfileLarge:
		numQueries = 8192
		outgoing = 32768
		incomingTCP = 4096
		rcvBuf = "8m"
		sndBuf = "8m"
	}

	return PerformanceConfig{
		Profile:             profile,
		NumThreads:          threads,
		MsgCacheSlabs:       slabs,
		RRSetCacheSlabs:     slabs,
		KeyCacheSlabs:       slabs,
		InfraCacheSlabs:     slabs,
		NumQueriesPerThread: numQueries,
		OutgoingRange:       outgoing,
		IncomingNumTCP:      incomingTCP,
		JostleTimeout:       jostleTimeout,
		SoRcvBuf:            rcvBuf,
		SoSndBuf:            sndBuf,
		SoReuseport:         true,
	}
}

// IsIPv6Address returns true if an IP string is valid IPv6.
func IsIPv6Address(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	return ip != nil && ip.To4() == nil
}
