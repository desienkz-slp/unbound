package model

import (
	"fmt"
	"net"
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
	SoRcvBuf            string             `json:"so_rcvbuf"` // e.g. "8m"
	SoSndBuf            string             `json:"so_sndbuf"` // e.g. "8m"
	SoReuseport         bool               `json:"so_reuseport"`
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
	ServeExpiredClientTimeout int  `json:"serve_expired_client_timeout"` // e.g. 1800 ms
	ServeExpiredReplyTTL     int  `json:"serve_expired_reply_ttl"`     // e.g. 30
	QnameMinimisation        bool `json:"qname_minimisation"`
	HideIdentity             bool `json:"hide_identity"`
	HideVersion              bool `json:"hide_version"`
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
	NodeID         string    `json:"node_id"`
	Hostname       string    `json:"hostname"`
	OS             string    `json:"os"`
	UnboundVersion string    `json:"unbound_version"`
	CPUCores       int       `json:"cpu_cores"`
	TotalRAMMB     int       `json:"total_ram_mb"`
	LastSeenAt     time.Time `json:"last_seen_at"`
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
	SafeSearch       SafeSearchConfig      `json:"safe_search"`
	ParentalControl  ParentalControlConfig `json:"parental_control"`
	GamblingBlock    GamblingBlockConfig   `json:"gambling_block"`
	LocalRecords     []LocalRecord         `json:"local_records"`
	RemoteControl    RemoteControlConfig   `json:"remote_control"`
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
				IP:      "172.18.20.145",
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
			ServeExpiredClientTimeout: 1800,
			ServeExpiredReplyTTL:      30,
			QnameMinimisation:         false, // Disabled by default for ISP resolvers to avoid NXDOMAIN on non-compliant CDN GSLBs (e.g. Baishan, Shopee)
			HideIdentity:              true,
			HideVersion:               true,
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
		LocalRecords: []LocalRecord{},
		RemoteControl: RemoteControlConfig{
			Enabled:        true,
			Interface:      "127.0.0.1",
			Port:           8953,
			ControlUseCert: false,
		},
	}
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
	incomingTCP := 1024
	rcvBuf := "4m"
	sndBuf := "4m"

	if profile == ProfileISP {
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
