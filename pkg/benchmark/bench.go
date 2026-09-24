package benchmark

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// BenchmarkOptions configures a benchmark run.
type BenchmarkOptions struct {
	ServerAddr   string        // Target DNS server (e.g. 127.0.0.1:53)
	DomainList   []string      // List of query domains
	Concurrency  int           // Number of parallel workers
	TotalQueries int           // Total queries to issue
	Timeout      time.Duration // Per-query timeout
	WarmCache    bool          // If true, pre-queries domains once before benchmark
}

// BenchmarkResult contains measurable performance KPIs.
type BenchmarkResult struct {
	TotalQueries      int           `json:"total_queries"`
	SuccessCount      int           `json:"success_count"`
	FailureCount      int           `json:"failure_count"`
	TimeoutCount      int           `json:"timeout_count"`
	Duration          time.Duration `json:"duration"`
	ActualQPS         float64       `json:"actual_qps"`
	P50Latency        time.Duration `json:"p50_latency"`
	P90Latency        time.Duration `json:"p90_latency"`
	P95Latency        time.Duration `json:"p95_latency"`
	P99Latency        time.Duration `json:"p99_latency"`
	MinLatency        time.Duration `json:"min_latency"`
	MaxLatency        time.Duration `json:"max_latency"`
	AvgLatency        time.Duration `json:"avg_latency"`
}

// RunBenchmark executes a progressive load test against a DNS server.
func RunBenchmark(ctx context.Context, opts BenchmarkOptions) (*BenchmarkResult, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 10
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 1 * time.Second
	}
	if len(opts.DomainList) == 0 {
		opts.DomainList = []string{"google.com", "cloudflare.com", "netora.local", "localhost"}
	}

	// Warmup if requested
	if opts.WarmCache {
		for _, d := range opts.DomainList {
			_ = resolveUDP(opts.ServerAddr, d, opts.Timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}

	queriesChan := make(chan string, opts.TotalQueries)
	for i := 0; i < opts.TotalQueries; i++ {
		domain := opts.DomainList[i%len(opts.DomainList)]
		queriesChan <- domain
	}
	close(queriesChan)

	var successCount uint64
	var failureCount uint64
	var timeoutCount uint64

	latencies := make([]time.Duration, 0, opts.TotalQueries)
	var latenciesMu sync.Mutex

	start := time.Now()
	var wg sync.WaitGroup

	for w := 0; w < opts.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localLatencies := make([]time.Duration, 0, opts.TotalQueries/opts.Concurrency+10)

			for domain := range queriesChan {
				t0 := time.Now()
				err := resolveUDP(opts.ServerAddr, domain, opts.Timeout)
				elapsed := time.Since(t0)

				localLatencies = append(localLatencies, elapsed)
				if err != nil {
					if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
						atomic.AddUint64(&timeoutCount, 1)
					} else {
						atomic.AddUint64(&failureCount, 1)
					}
				} else {
					atomic.AddUint64(&successCount, 1)
				}
			}

			latenciesMu.Lock()
			latencies = append(latencies, localLatencies...)
			latenciesMu.Unlock()
		}()
	}

	wg.Wait()
	totalDuration := time.Since(start)

	if len(latencies) == 0 {
		return nil, fmt.Errorf("no queries completed")
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}

	actualQPS := float64(opts.TotalQueries) / totalDuration.Seconds()
	p50 := latencies[int(float64(len(latencies))*0.50)]
	p90 := latencies[int(float64(len(latencies))*0.90)]
	p95 := latencies[int(float64(len(latencies))*0.95)]
	p99 := latencies[int(float64(len(latencies))*0.99)]

	return &BenchmarkResult{
		TotalQueries: opts.TotalQueries,
		SuccessCount: int(successCount),
		FailureCount: int(failureCount),
		TimeoutCount: int(timeoutCount),
		Duration:     totalDuration,
		ActualQPS:    actualQPS,
		P50Latency:   p50,
		P90Latency:   p90,
		P95Latency:   p95,
		P99Latency:   p99,
		MinLatency:   latencies[0],
		MaxLatency:   latencies[len(latencies)-1],
		AvgLatency:   totalLatency / time.Duration(len(latencies)),
	}, nil
}

// resolveUDP constructs a raw DNS wire format query and sends it over UDP.
func resolveUDP(serverAddr, domain string, timeout time.Duration) error {
	conn, err := net.DialTimeout("udp", serverAddr, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	packet := buildRawDNSQuery(domain)
	if _, err := conn.Write(packet); err != nil {
		return err
	}

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	if n < 12 {
		return fmt.Errorf("short response: %d bytes", n)
	}

	return nil
}

// buildRawDNSQuery creates a standard DNS query packet (A record, IN class, RD flag).
func buildRawDNSQuery(domain string) []byte {
	var packet []byte

	// Header (12 bytes)
	id := uint16(0x1337)
	flags := uint16(0x0100) // Standard query with Recursion Desired (RD)
	qdcount := uint16(1)
	ancount := uint16(0)
	nscount := uint16(0)
	arcount := uint16(0)

	packet = append(packet, byte(id>>8), byte(id))
	packet = append(packet, byte(flags>>8), byte(flags))
	packet = append(packet, byte(qdcount>>8), byte(qdcount))
	packet = append(packet, byte(ancount>>8), byte(ancount))
	packet = append(packet, byte(nscount>>8), byte(nscount))
	packet = append(packet, byte(arcount>>8), byte(arcount))

	// Question Section
	parts := strings.Split(domain, ".")
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		packet = append(packet, byte(len(part)))
		packet = append(packet, part...)
	}
	packet = append(packet, 0x00) // Null root label

	// QTYPE: A (0x0001)
	packet = append(packet, 0x00, 0x01)
	// QCLASS: IN (0x0001)
	packet = append(packet, 0x00, 0x01)

	return packet
}
