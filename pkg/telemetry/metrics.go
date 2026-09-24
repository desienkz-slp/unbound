package telemetry

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds the thread-safe telemetry counters and gauges for Prometheus exposition.
type Metrics struct {
	mu sync.RWMutex

	// Counters
	QueriesTotalUDPv4   uint64
	QueriesTotalUDPv6   uint64
	QueriesTotalTCPv4   uint64
	QueriesTotalTCPv6   uint64
	CacheHitsTotal      uint64
	CacheMissesTotal    uint64
	ServfailTotal       uint64
	NxdomainTotal       uint64
	TimeoutsTotal       uint64
	DNSSECFailuresTotal uint64
	BlockedQueriesTotal uint64
	AllowedQueriesTotal uint64

	// Category block counters
	BlockedByCategory map[string]uint64

	// Gauges
	RequestListCurrent float64
	RequestListMax     float64
	UpstreamLatencyMS  map[string]float64
	CPUUtilization     float64
	MemoryUtilization  float64
	LastStatsUpdate    time.Time
}

var (
	// DefaultMetrics is the singleton metrics instance.
	DefaultMetrics = NewMetrics()
)

// NewMetrics initializes a clean Metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		BlockedByCategory: make(map[string]uint64),
		UpstreamLatencyMS: make(map[string]float64),
		LastStatsUpdate:   time.Now(),
	}
}

// IncQueries increments query counters by protocol and address family.
func (m *Metrics) IncQueries(protocol string, isIPv6 bool) {
	if strings.ToUpper(protocol) == "TCP" {
		if isIPv6 {
			atomic.AddUint64(&m.QueriesTotalTCPv6, 1)
		} else {
			atomic.AddUint64(&m.QueriesTotalTCPv4, 1)
		}
	} else {
		if isIPv6 {
			atomic.AddUint64(&m.QueriesTotalUDPv6, 1)
		} else {
			atomic.AddUint64(&m.QueriesTotalUDPv4, 1)
		}
	}
}

// RecordDecision updates allowed or blocked counters.
func (m *Metrics) RecordDecision(action string, category string) {
	if strings.ToUpper(action) == "BLOCKED" {
		atomic.AddUint64(&m.BlockedQueriesTotal, 1)
		if category != "" {
			m.mu.Lock()
			m.BlockedByCategory[category]++
			m.mu.Unlock()
		}
	} else {
		atomic.AddUint64(&m.AllowedQueriesTotal, 1)
	}
}

// SetUpstreamLatency records the RTT of an upstream resolver in milliseconds.
func (m *Metrics) SetUpstreamLatency(upstream string, latencyMS float64) {
	m.mu.Lock()
	m.UpstreamLatencyMS[upstream] = latencyMS
	m.mu.Unlock()
}

// UpdateUnboundStats parses stats from Unbound and maps them to internal counters.
func (m *Metrics) UpdateUnboundStats(stats map[string]float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if v, ok := stats["total.num.cachehits"]; ok {
		atomic.StoreUint64(&m.CacheHitsTotal, uint64(v))
	}
	if v, ok := stats["total.num.cachemiss"]; ok {
		atomic.StoreUint64(&m.CacheMissesTotal, uint64(v))
	}
	if v, ok := stats["num.answer.rcode.SERVFAIL"]; ok {
		atomic.StoreUint64(&m.ServfailTotal, uint64(v))
	}
	if v, ok := stats["num.answer.rcode.NXDOMAIN"]; ok {
		atomic.StoreUint64(&m.NxdomainTotal, uint64(v))
	}
	if v, ok := stats["total.requestlist.avg"]; ok {
		m.RequestListCurrent = v
	}
	if v, ok := stats["total.requestlist.max"]; ok {
		m.RequestListMax = v
	}
	m.LastStatsUpdate = time.Now()
}

// PrometheusHandler outputs metrics in the standard Prometheus text format.
func (m *Metrics) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var b strings.Builder

		// HELP & TYPE comments
		b.WriteString("# HELP netora_queries_total Total DNS queries received partitioned by protocol and IP version.\n")
		b.WriteString("# TYPE netora_queries_total counter\n")
		fmt.Fprintf(&b, "netora_queries_total{protocol=\"udp\",family=\"v4\"} %d\n", atomic.LoadUint64(&m.QueriesTotalUDPv4))
		fmt.Fprintf(&b, "netora_queries_total{protocol=\"udp\",family=\"v6\"} %d\n", atomic.LoadUint64(&m.QueriesTotalUDPv6))
		fmt.Fprintf(&b, "netora_queries_total{protocol=\"tcp\",family=\"v4\"} %d\n", atomic.LoadUint64(&m.QueriesTotalTCPv4))
		fmt.Fprintf(&b, "netora_queries_total{protocol=\"tcp\",family=\"v6\"} %d\n", atomic.LoadUint64(&m.QueriesTotalTCPv6))

		b.WriteString("\n# HELP netora_cache_hits_total Total DNS cache hits.\n")
		b.WriteString("# TYPE netora_cache_hits_total counter\n")
		fmt.Fprintf(&b, "netora_cache_hits_total %d\n", atomic.LoadUint64(&m.CacheHitsTotal))

		b.WriteString("\n# HELP netora_cache_misses_total Total DNS cache misses.\n")
		b.WriteString("# TYPE netora_cache_misses_total counter\n")
		fmt.Fprintf(&b, "netora_cache_misses_total %d\n", atomic.LoadUint64(&m.CacheMissesTotal))

		b.WriteString("\n# HELP netora_servfail_total Total SERVFAIL responses generated.\n")
		b.WriteString("# TYPE netora_servfail_total counter\n")
		fmt.Fprintf(&b, "netora_servfail_total %d\n", atomic.LoadUint64(&m.ServfailTotal))

		b.WriteString("\n# HELP netora_nxdomain_total Total NXDOMAIN responses generated.\n")
		b.WriteString("# TYPE netora_nxdomain_total counter\n")
		fmt.Fprintf(&b, "netora_nxdomain_total %d\n", atomic.LoadUint64(&m.NxdomainTotal))

		b.WriteString("\n# HELP netora_dnssec_failures_total Total cryptographic DNSSEC validation failures.\n")
		b.WriteString("# TYPE netora_dnssec_failures_total counter\n")
		fmt.Fprintf(&b, "netora_dnssec_failures_total %d\n", atomic.LoadUint64(&m.DNSSECFailuresTotal))

		b.WriteString("\n# HELP netora_blocked_queries_total Total DNS queries blocked by policy engine.\n")
		b.WriteString("# TYPE netora_blocked_queries_total counter\n")
		fmt.Fprintf(&b, "netora_blocked_queries_total %d\n", atomic.LoadUint64(&m.BlockedQueriesTotal))

		m.mu.RLock()
		for cat, count := range m.BlockedByCategory {
			fmt.Fprintf(&b, "netora_blocked_queries_total{category=\"%s\"} %d\n", cat, count)
		}

		b.WriteString("\n# HELP netora_allowed_queries_total Total DNS queries allowed by policy engine.\n")
		b.WriteString("# TYPE netora_allowed_queries_total counter\n")
		fmt.Fprintf(&b, "netora_allowed_queries_total %d\n", atomic.LoadUint64(&m.AllowedQueriesTotal))

		b.WriteString("\n# HELP netora_upstream_latency_seconds Latency to upstream recursive resolvers.\n")
		b.WriteString("# TYPE netora_upstream_latency_seconds gauge\n")
		for upstream, latency := range m.UpstreamLatencyMS {
			fmt.Fprintf(&b, "netora_upstream_latency_seconds{upstream=\"%s\"} %.4f\n", upstream, latency/1000.0)
		}

		b.WriteString("\n# HELP netora_requestlist_current Current depth of the internal request queue.\n")
		b.WriteString("# TYPE netora_requestlist_current gauge\n")
		fmt.Fprintf(&b, "netora_requestlist_current %.2f\n", m.RequestListCurrent)

		b.WriteString("\n# HELP netora_requestlist_max Maximum depth of the internal request queue.\n")
		b.WriteString("# TYPE netora_requestlist_max gauge\n")
		fmt.Fprintf(&b, "netora_requestlist_max %.2f\n", m.RequestListMax)
		m.mu.RUnlock()

		w.Write([]byte(b.String()))
	}
}
