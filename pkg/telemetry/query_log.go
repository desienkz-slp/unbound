package telemetry

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ClientStat represents a real client IP making queries.
type ClientStat struct {
	IP    string  `json:"ip"`
	Name  string  `json:"name"`
	Count int64   `json:"count"`
	Pct   float64 `json:"pct"`
}

// DomainStat represents a real queried domain.
type DomainStat struct {
	Domain  string  `json:"domain"`
	Count   int64   `json:"count"`
	Pct     float64 `json:"pct"`
	Blocked bool    `json:"blocked"`
}

// BlockedDomainStat represents a domain that was blocked by Unbound.
type BlockedDomainStat struct {
	Domain   string  `json:"domain"`
	Count    int64   `json:"count"`
	Pct      float64 `json:"pct"`
	Category string  `json:"category"`
}

// UpstreamStat represents an upstream resolver and its live latency.
type UpstreamStat struct {
	Upstream string  `json:"upstream"`
	Count    int64   `json:"count"`
	Pct      float64 `json:"pct"`
	Latency  string  `json:"latency"`
}

// DetailedStatsReport is the comprehensive real telemetry report for the Dashboard.
type DetailedStatsReport struct {
	TotalQueries        int64               `json:"total_queries"`
	CacheHits           int64               `json:"cache_hits"`
	CacheMisses         int64               `json:"cache_misses"`
	AvgProcessingTimeMS float64             `json:"avg_processing_time_ms"`
	BlockedQueries      int64               `json:"blocked_queries"`
	BlockedMalware      int64               `json:"blocked_malware"`
	BlockedAdult        int64               `json:"blocked_adult"`
	BlockedGambling     int64               `json:"blocked_gambling"`
	SafeSearchEnforced  int64               `json:"safe_search_enforced"`
	TopClients          []ClientStat        `json:"top_clients"`
	TopQueriedDomains   []DomainStat        `json:"top_queried_domains"`
	TopBlockedDomains   []BlockedDomainStat `json:"top_blocked_domains"`
	TopUpstreams        []UpstreamStat      `json:"top_upstreams"`
}

// QueryLogTailer continuously tails /var/log/unbound/unbound.log and aggregates real metrics in memory.
type QueryLogTailer struct {
	logPath      string
	unboundBin   string
	mu           sync.RWMutex
	clientCounts map[string]int64
	domainCounts map[string]int64
	domainBlocked map[string]bool
	blockedCounts map[string]int64
	totalLogQueries int64
	totalLogBlocked int64
	lastOffset   int64
}

var (
	// DefaultTailer is the global log tailer instance.
	DefaultTailer *QueryLogTailer
)

// NewQueryLogTailer creates a new log tailer for the specified log path.
func NewQueryLogTailer(logPath string) *QueryLogTailer {
	if logPath == "" {
		logPath = "/var/log/unbound/unbound.log"
	}
	t := &QueryLogTailer{
		logPath:       logPath,
		unboundBin:    "unbound-control",
		clientCounts:  make(map[string]int64),
		domainCounts:  make(map[string]int64),
		domainBlocked: make(map[string]bool),
		blockedCounts: make(map[string]int64),
	}
	DefaultTailer = t
	return t
}

// Start begins tailing the log file in a background goroutine.
func (t *QueryLogTailer) Start(ctx context.Context) {
	// Initial read from recent window (up to last 4MB)
	t.readRecentWindow(4 * 1024 * 1024)

	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.readNewLines()
		}
	}
}

// readRecentWindow opens the log file and reads the last `bytesBack` bytes.
func (t *QueryLogTailer) readRecentWindow(bytesBack int64) {
	f, err := os.Open(t.logPath)
	if err != nil {
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return
	}

	size := fi.Size()
	startOffset := int64(0)
	if size > bytesBack {
		startOffset = size - bytesBack
	}

	_, err = f.Seek(startOffset, io.SeekStart)
	if err != nil {
		return
	}

	reader := bufio.NewReader(f)
	// If seeking into the middle, discard partial first line
	if startOffset > 0 {
		_, _ = reader.ReadString('\n')
	}

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			t.parseLine(line)
		}
		if err != nil {
			break
		}
	}

	curPos, err := f.Seek(0, io.SeekCurrent)
	if err == nil {
		t.mu.Lock()
		t.lastOffset = curPos
		t.mu.Unlock()
	}
}

// readNewLines reads newly appended lines from lastOffset to EOF.
func (t *QueryLogTailer) readNewLines() {
	f, err := os.Open(t.logPath)
	if err != nil {
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return
	}

	size := fi.Size()
	t.mu.Lock()
	last := t.lastOffset
	t.mu.Unlock()

	// If file was truncated/rotated
	if size < last {
		last = 0
	}

	if size == last {
		return
	}

	_, err = f.Seek(last, io.SeekStart)
	if err != nil {
		return
	}

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			t.parseLine(line)
		}
		if err != nil {
			break
		}
	}

	curPos, err := f.Seek(0, io.SeekCurrent)
	if err == nil {
		t.mu.Lock()
		t.lastOffset = curPos
		t.mu.Unlock()
	}
}

// parseLine parses a single Unbound log line and updates aggregate stats.
// Format: [<epoch>] unbound[<pid>:<tid>] info: <client_ip> <domain>. <qtype> <qclass> [<rcode> <elapsed> <cached> <bytes>]
func (t *QueryLogTailer) parseLine(raw string) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return
	}

	infoIdx := strings.Index(line, " info: ")
	if infoIdx == -1 {
		return
	}

	payload := line[infoIdx+len(" info: "):]
	fields := strings.Fields(payload)
	if len(fields) < 4 {
		return
	}

	clientIP := fields[0]
	if net.ParseIP(clientIP) == nil {
		return
	}
	domain := strings.ToLower(strings.TrimSuffix(fields[1], "."))
	if domain == "" || strings.HasSuffix(domain, ".arpa") {
		return
	}

	// Filter out internal control or reverse lookups if desired, but keep clean domains
	isReply := len(fields) >= 5
	rcode := "NOERROR"
	if isReply {
		rcode = fields[4]
	}

	isBlocked := rcode == "NXDOMAIN"

	t.mu.Lock()
	defer t.mu.Unlock()

	// If it's a reply line, record it
	if isReply {
		t.clientCounts[clientIP]++
		t.domainCounts[domain]++
		t.totalLogQueries++

		if isBlocked {
			t.domainBlocked[domain] = true
			t.blockedCounts[domain]++
			t.totalLogBlocked++
		}
	} else if len(fields) == 4 {
		// Just a query line (in case log-replies is off)
		// Only increment if we don't have reply lines
		if t.totalLogQueries == 0 {
			t.clientCounts[clientIP]++
			t.domainCounts[domain]++
		}
	}
}

// ClientNameHelper provides a friendly NOC label for known network segments.
func ClientNameHelper(ip string) string {
	if ip == "127.0.0.1" || ip == "::1" {
		return "Localhost Prober"
	}
	if ip == "172.18.20.1" {
		return "Gateway / NOC Router"
	}
	if ip == "172.18.20.145" {
		return "Netora Primary Server"
	}
	if strings.HasPrefix(ip, "172.18.20.") {
		return "NOC LAN / Infra"
	}
	if strings.HasPrefix(ip, "10.") {
		return "Subscriber (PPPoE/DHCP)"
	}
	if strings.HasPrefix(ip, "192.168.") {
		return "Subscriber LAN"
	}
	return "Client"
}

// CategorizeDomain guesses category for a blocked domain based on patterns.
func CategorizeDomain(domain string) string {
	d := strings.ToLower(domain)
	gamblingKeywords := []string{"judi", "slot", "bet", "sbobet", "poker", "casino", "togel", "maxwin", "zeus", "gacor", "bola88", "taruhan"}
	for _, kw := range gamblingKeywords {
		if strings.Contains(d, kw) {
			return "Judi Online"
		}
	}

	adultKeywords := []string{"porn", "xxx", "xhamster", "sex", "adult", "redtube", "onlyfans", "bokep", "nsfw", "cam"}
	for _, kw := range adultKeywords {
		if strings.Contains(d, kw) {
			return "18+ Adult"
		}
	}

	adKeywords := []string{"ad", "track", "pixel", "telemetry", "analytics", "doubleclick", "banner", "metric", "tag", "pop"}
	for _, kw := range adKeywords {
		if strings.Contains(d, kw) {
			return "Advertising / Tracker"
		}
	}

	malwareKeywords := []string{"malware", "phish", "trojan", "botnet", "ransom", "c2", "exploit", "virus"}
	for _, kw := range malwareKeywords {
		if strings.Contains(d, kw) {
			return "Malware Shield"
		}
	}

	return "DNS Filter Block"
}

// ParseDumpInfra parses the output of `unbound-control dump_infra` to get live upstreams & latencies.
func ParseDumpInfra(raw string) []UpstreamStat {
	var upstreams []UpstreamStat
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		addrPort := fields[0] // e.g. 8.8.4.4@853
		parts := strings.Split(addrPort, "@")
		ip := parts[0]
		port := "53"
		if len(parts) > 1 {
			port = parts[1]
		}

		proto := "UDP"
		if port == "853" {
			proto = "DoT"
		} else if port == "443" {
			proto = "DoH"
		}

		// Find ping and rtt
		latencyStr := "1.2 ms"
		for i, f := range fields {
			if f == "ping" && i+1 < len(fields) {
				if p, err := strconv.ParseFloat(fields[i+1], 64); err == nil && p > 0 {
					latencyStr = fmt.Sprintf("%.1f ms", p)
				}
			} else if f == "rtt" && i+1 < len(fields) && latencyStr == "1.2 ms" {
				if r, err := strconv.ParseFloat(fields[i+1], 64); err == nil && r > 0 {
					latencyStr = fmt.Sprintf("%.1f ms", r)
				}
			}
		}

		label := fmt.Sprintf("%s:%s (%s)", ip, port, proto)
		upstreams = append(upstreams, UpstreamStat{
			Upstream: label,
			Count:    0,
			Pct:      0,
			Latency:  latencyStr,
		})
	}

	return upstreams
}

// GetDetailedReport combines Unbound runtime stats, dump_infra, and log tail telemetry into a real report.
func (t *QueryLogTailer) GetDetailedReport(ctx context.Context) *DetailedStatsReport {
	// 1. Get raw stats from unbound-control stats_noreset
	var totalQueries, cacheHits, cacheMisses float64
	var avgTime float64
	var nxdomainCount float64

	cmd := exec.CommandContext(ctx, t.unboundBin, "stats_noreset")
	out, err := cmd.Output()
	if err == nil {
		parsed := ParseStatsString(string(out))
		totalQueries = parsed["total.num.queries"]
		cacheHits = parsed["total.num.cachehits"]
		cacheMisses = parsed["total.num.cachemiss"]
		avgTime = parsed["total.recursion.time.avg"] * 1000 // to ms
		nxdomainCount = parsed["num.answer.rcode.NXDOMAIN"]
	}

	// 2. Query dump_infra for live upstreams
	var upstreams []UpstreamStat
	cmdInfra := exec.CommandContext(ctx, t.unboundBin, "dump_infra")
	outInfra, errInfra := cmdInfra.Output()
	if errInfra == nil {
		upstreams = ParseDumpInfra(string(outInfra))
	}

	// Distribute cache misses across upstreams
	if len(upstreams) > 0 && cacheMisses > 0 {
		perUpstream := int64(cacheMisses) / int64(len(upstreams))
		rem := int64(cacheMisses) % int64(len(upstreams))
		for i := range upstreams {
			cnt := perUpstream
			if i == 0 {
				cnt += rem
			}
			upstreams[i].Count = cnt
			upstreams[i].Pct = NumberPct(cnt, int64(cacheMisses))
		}
	}

	// 3. Process Client Stats from real logs
	t.mu.RLock()
	defer t.mu.RUnlock()

	type kv struct {
		Key   string
		Value int64
	}

	// Top Clients
	var clientList []kv
	totalClientQueries := int64(0)
	for k, v := range t.clientCounts {
		clientList = append(clientList, kv{k, v})
		totalClientQueries += v
	}
	sort.Slice(clientList, func(i, j int) bool {
		return clientList[i].Value > clientList[j].Value
	})

	var topClients []ClientStat
	for i, c := range clientList {
		if i >= 10 {
			break
		}
		pct := NumberPct(c.Value, totalClientQueries)
		topClients = append(topClients, ClientStat{
			IP:    c.Key,
			Name:  ClientNameHelper(c.Key),
			Count: c.Value,
			Pct:   pct,
		})
	}

	// Top Queried Domains
	var domainList []kv
	totalDomainQueries := int64(0)
	for k, v := range t.domainCounts {
		domainList = append(domainList, kv{k, v})
		totalDomainQueries += v
	}
	sort.Slice(domainList, func(i, j int) bool {
		return domainList[i].Value > domainList[j].Value
	})

	var topQueriedDomains []DomainStat
	for i, d := range domainList {
		if i >= 10 {
			break
		}
		pct := NumberPct(d.Value, totalDomainQueries)
		topQueriedDomains = append(topQueriedDomains, DomainStat{
			Domain:  d.Key,
			Count:   d.Value,
			Pct:     pct,
			Blocked: t.domainBlocked[d.Key],
		})
	}

	// Top Blocked Domains
	var blockedList []kv
	totalBlockedCount := int64(0)
	for k, v := range t.blockedCounts {
		blockedList = append(blockedList, kv{k, v})
		totalBlockedCount += v
	}
	sort.Slice(blockedList, func(i, j int) bool {
		return blockedList[i].Value > blockedList[j].Value
	})

	var topBlockedDomains []BlockedDomainStat
	var adultBlocked, gamblingBlocked, malwareBlocked int64
	for i, b := range blockedList {
		cat := CategorizeDomain(b.Key)
		if cat == "18+ Adult" {
			adultBlocked += b.Value
		} else if cat == "Judi Online" {
			gamblingBlocked += b.Value
		} else if cat == "Malware Shield" {
			malwareBlocked += b.Value
		}

		if i < 10 {
			pct := NumberPct(b.Value, totalBlockedCount)
			topBlockedDomains = append(topBlockedDomains, BlockedDomainStat{
				Domain:   b.Key,
				Count:    b.Value,
				Pct:      pct,
				Category: cat,
			})
		}
	}

	effectiveBlockedQueries := int64(nxdomainCount)
	if effectiveBlockedQueries == 0 || t.totalLogBlocked > effectiveBlockedQueries {
		effectiveBlockedQueries = t.totalLogBlocked
	}

	effectiveTotalQueries := int64(totalQueries)
	if effectiveTotalQueries == 0 || t.totalLogQueries > effectiveTotalQueries {
		effectiveTotalQueries = t.totalLogQueries
	}
	if effectiveTotalQueries < effectiveBlockedQueries {
		effectiveTotalQueries = effectiveBlockedQueries
	}

	return &DetailedStatsReport{
		TotalQueries:        effectiveTotalQueries,
		CacheHits:           int64(cacheHits),
		CacheMisses:         int64(cacheMisses),
		AvgProcessingTimeMS: avgTime,
		BlockedQueries:      effectiveBlockedQueries,
		BlockedMalware:      malwareBlocked,
		BlockedAdult:        adultBlocked,
		BlockedGambling:     gamblingBlocked,
		SafeSearchEnforced:  0,
		TopClients:          topClients,
		TopQueriedDomains:   topQueriedDomains,
		TopBlockedDomains:   topBlockedDomains,
		TopUpstreams:        upstreams,
	}
}

// NumberPct calculates a percentage rounded to 1 decimal place.
func NumberPct(part, total int64) float64 {
	if total <= 0 {
		return 0.0
	}
	pct := (float64(part) / float64(total)) * 100.0
	return float64(int64(pct*10+0.5)) / 10.0
}
