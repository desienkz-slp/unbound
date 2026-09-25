package telemetry

import (
	"context"
	"os"
	"testing"
)

func TestParseDumpInfra(t *testing.T) {
	sample := `8.8.4.4@853 . ttl 117 ping 35 var 11 rtt 79 rto 79 tA 0 tAAAA 0 tother 0 ednsknown 0 edns 0 delay 0 lame dnssec 0 rec 0 A 0 other 0
8.8.8.8@853 . ttl 116 ping 60 var 149 rtt 656 rto 656 tA 0 tAAAA 0 tother 0 ednsknown 0 edns 0 delay 0 lame dnssec 0 rec 0 A 0 other 0
1.0.0.1@853 . ttl 157 ping 37 var 31 rtt 161 rto 161 tA 0 tAAAA 0 tother 0 ednsknown 0 edns 0 delay 0 lame dnssec 0 rec 0 A 0 other 0
1.1.1.1@853 . ttl 198 ping 34 var 17 rtt 102 rto 102 tA 0 tAAAA 0 tother 0 ednsknown 0 edns 0 delay 0 lame dnssec 0 rec 0 A 0 other 0`

	upstreams := ParseDumpInfra(sample)
	if len(upstreams) != 4 {
		t.Fatalf("expected 4 upstreams, got %d", len(upstreams))
	}

	if upstreams[0].Upstream != "8.8.4.4:853 (DoT)" {
		t.Errorf("unexpected upstream label: %s", upstreams[0].Upstream)
	}
	if upstreams[0].Latency != "35.0 ms" {
		t.Errorf("expected 35.0 ms, got %s", upstreams[0].Latency)
	}
}

func TestQueryLogTailerParsing(t *testing.T) {
	// Create a temp log file
	tmpFile, err := os.CreateTemp("", "unbound-test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	sampleLogs := `[1790364732] unbound[30123:0] info: 172.18.20.254 rak8s.com. A IN NXDOMAIN 0.000000 1 27
[1790364732] unbound[30123:0] info: 172.18.20.254 shopee.co.id. TYPE65 IN NOERROR 0.000000 1 114
[1790364732] unbound[30123:1] info: 172.18.20.254 shopee.co.id. A IN NOERROR 0.028466 0 46
[1790364732] unbound[30123:0] info: 127.0.0.1 google.com. A IN NOERROR 0.000000 1 135
[1790364735] unbound[30123:0] info: 127.0.0.1 sbobet.com. A IN NXDOMAIN 0.000000 1 28
`
	if _, err := tmpFile.WriteString(sampleLogs); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	tailer := NewQueryLogTailer(tmpFile.Name())
	tailer.readRecentWindow(1024 * 1024)

	report := tailer.GetDetailedReport(context.Background())

	if report.TotalQueries < 5 {
		t.Errorf("expected at least 5 queries, got %d", report.TotalQueries)
	}
	if report.BlockedQueries < 2 {
		t.Errorf("expected at least 2 blocked queries (rak8s.com and sbobet.com), got %d", report.BlockedQueries)
	}

	if len(report.TopClients) < 2 {
		t.Errorf("expected at least 2 clients, got %d", len(report.TopClients))
	}

	// Verify categories
	if CategorizeDomain("sbobet.com") != "Judi Online" {
		t.Errorf("expected Judi Online for sbobet.com, got %s", CategorizeDomain("sbobet.com"))
	}
	if CategorizeDomain("xhamster.com") != "18+ Adult" {
		t.Errorf("expected 18+ Adult for xhamster.com, got %s", CategorizeDomain("xhamster.com"))
	}
}
