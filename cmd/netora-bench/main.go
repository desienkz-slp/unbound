package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"netora-dns/pkg/benchmark"
)

func main() {
	serverAddr := flag.String("server", "127.0.0.1:53", "DNS server address:port")
	queries := flag.Int("queries", 1000, "Total queries to execute")
	concurrency := flag.Int("concurrency", 20, "Number of concurrent workers")
	domainsFlag := flag.String("domains", "google.com,cloudflare.com,netora.local,github.com", "Comma-separated domain list")
	warmup := flag.Bool("warmup", true, "Warm cache prior to benchmarking")
	flag.Parse()

	domains := strings.Split(*domainsFlag, ",")
	for i := range domains {
		domains[i] = strings.TrimSpace(domains[i])
	}

	fmt.Println("=================================================================")
	fmt.Println(" NETORA DNS — HIGH CONCURRENCY BENCHMARK RUNNER")
	fmt.Println("=================================================================")
	fmt.Printf(" Target Resolver:   %s\n", *serverAddr)
	fmt.Printf(" Total Queries:     %d\n", *queries)
	fmt.Printf(" Concurrency:       %d workers\n", *concurrency)
	fmt.Printf(" Cache Warmup:      %v\n", *warmup)
	fmt.Printf(" Domain Pool:       %v\n", domains)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println(" Benchmarking in progress...")

	opts := benchmark.BenchmarkOptions{
		ServerAddr:   *serverAddr,
		DomainList:   domains,
		Concurrency:  *concurrency,
		TotalQueries: *queries,
		Timeout:      1 * time.Second,
		WarmCache:    *warmup,
	}

	res, err := benchmark.RunBenchmark(context.Background(), opts)
	if err != nil {
		log.Fatalf("Benchmark error: %v", err)
	}

	fmt.Println("-----------------------------------------------------------------")
	fmt.Println(" BENCHMARK RESULTS")
	fmt.Println("-----------------------------------------------------------------")
	fmt.Printf(" Duration:          %v\n", res.Duration)
	fmt.Printf(" Measured QPS:      %.1f queries/sec\n", res.ActualQPS)
	fmt.Printf(" Success Rate:      %d / %d (%.2f%%)\n", res.SuccessCount, res.TotalQueries, float64(res.SuccessCount)/float64(res.TotalQueries)*100.0)
	fmt.Printf(" Timeouts:          %d\n", res.TimeoutCount)
	fmt.Printf(" Failures:          %d\n", res.FailureCount)
	fmt.Println(" Latency Breakdown:")
	fmt.Printf("   Min Latency:     %v\n", res.MinLatency)
	fmt.Printf("   Avg Latency:     %v\n", res.AvgLatency)
	fmt.Printf("   P50 (Median):    %v\n", res.P50Latency)
	fmt.Printf("   P90:             %v\n", res.P90Latency)
	fmt.Printf("   P95:             %v\n", res.P95Latency)
	fmt.Printf("   P99:             %v\n", res.P99Latency)
	fmt.Printf("   Max Latency:     %v\n", res.MaxLatency)
	fmt.Println("=================================================================")
}
