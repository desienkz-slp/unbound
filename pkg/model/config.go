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
	Enabled       bool   `json:"enabled"`
	Frequency     string `json:"frequency"`      // "daily", "weekly", "monthly"
	RotateCount   int    `json:"rotate_count"`   // 1 - 90
	MaxSize       string `json:"max_size"`       // e.g. "25M", "50M", "100M"
	Compress      bool   `json:"compress"`       // gzip
	DelayCompress bool   `json:"delay_compress"` // delaycompress
	RotateNetora  bool   `json:"rotate_netora"`  // rotate /var/log/netora-dns/*.log
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
}

// NewDefaultDesiredState returns a safe, production-grade baseline desired state.
func NewDefaultDesiredState(nodeID string, cores int, ramMB int) *DesiredState {
	perf := AutoTunePerformance(cores, ramMB)
	cache := AutoTuneCache(ramMB)

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
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			IPRateLimit:       500,
			IPRateLimitFactor: 10,
			DomainRateLimit:   2000,
			RateLimitSize:     "16m",
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

	// Find existing A records for this nodeName
	existingForNode := make(map[string]bool)
	for _, rec := range s.LocalRecords {
		if strings.EqualFold(rec.Domain, nodeName) && strings.ToUpper(rec.Type) == "A" {
			existingForNode[rec.Value] = true
		}
	}

	// If all target IPs are already present and count matches, no change needed
	if len(existingForNode) == len(targetIPs) {
		allMatch := true
		for _, ip := range targetIPs {
			if !existingForNode[ip] {
				allMatch = false
				break
			}
		}
		if allMatch {
			return false
		}
	}

	// Reconstruct LocalRecords: preserve all other domains, and replace nodeName A records with targetIPs
	var newRecords []LocalRecord
	for _, rec := range s.LocalRecords {
		if strings.EqualFold(rec.Domain, nodeName) && strings.ToUpper(rec.Type) == "A" {
			continue
		}
		newRecords = append(newRecords, rec)
	}

	// Append target IP rewrites for nodeName
	for _, ip := range targetIPs {
		newRecords = append(newRecords, LocalRecord{
			Domain: nodeName,
			Type:   "A",
			Value:  ip,
			TTL:    300,
		})
	}

	s.LocalRecords = newRecords
	return true
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

	numQueries := 4096
	outgoing := 16384
	incomingTCP := 2048
	rcvBuf := "8m"
	sndBuf := "8m"
	jostleTimeout := defaultJostleTimeout

	if profile == ProfileSmall {
		numQueries = 2048
		outgoing = 8192
		incomingTCP = 1024
		rcvBuf = "4m"
		sndBuf = "4m"
	} else if profile == ProfileISP || profile == ProfileLarge {
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
