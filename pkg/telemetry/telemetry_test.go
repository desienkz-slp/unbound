package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsPrometheusExposition(t *testing.T) {
	m := NewMetrics()
	m.IncQueries("UDP", false)
	m.IncQueries("UDP", true)
	m.IncQueries("TCP", false)
	m.RecordDecision("BLOCKED", "Advertising")
	m.RecordDecision("ALLOW", "")
	m.SetUpstreamLatency("1.1.1.1", 12.5)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	handler := m.PrometheusHandler()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /metrics, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "netora_queries_total{protocol=\"udp\",family=\"v4\"} 1") {
		t.Errorf("missing UDP v4 query count in /metrics output: %s", body)
	}
	if !strings.Contains(body, "netora_blocked_queries_total{category=\"Advertising\"} 1") {
		t.Errorf("missing category block count in /metrics output: %s", body)
	}
	if !strings.Contains(body, "netora_upstream_latency_seconds{upstream=\"1.1.1.1\"} 0.0125") {
		t.Errorf("missing upstream latency in /metrics output: %s", body)
	}
}

func TestUnboundStatsParser(t *testing.T) {
	sampleStats := `
total.num.queries=42890
total.num.cachehits=39102
total.num.cachemiss=3788
num.answer.rcode.SERVFAIL=12
num.answer.rcode.NXDOMAIN=450
total.requestlist.avg=1.45
total.requestlist.max=8
`
	parsed := ParseStatsString(sampleStats)
	if parsed["total.num.queries"] != 42890 {
		t.Errorf("expected 42890 queries, got %v", parsed["total.num.queries"])
	}
	if parsed["total.num.cachehits"] != 39102 {
		t.Errorf("expected 39102 cachehits, got %v", parsed["total.num.cachehits"])
	}

	m := NewMetrics()
	m.UpdateUnboundStats(parsed)
	if m.CacheHitsTotal != 39102 {
		t.Errorf("expected m.CacheHitsTotal = 39102, got %d", m.CacheHitsTotal)
	}
}

func TestAlertEngineTriggerAndResolve(t *testing.T) {
	m := NewMetrics()
	ae := NewAlertEngine("test-node", nil, m)

	// Case 1: Unbound node down
	events := ae.Evaluate(context.Background(), false)
	if len(events) == 0 {
		t.Fatalf("expected DNS_NODE_DOWN alert to trigger when unhealthy")
	}
	if events[0].RuleName != "DNS_NODE_DOWN" {
		t.Errorf("expected rule DNS_NODE_DOWN, got %s", events[0].RuleName)
	}

	active := ae.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(active))
	}

	// Case 2: Node becomes healthy -> resolves
	ae.Evaluate(context.Background(), true)
	activeAfter := ae.GetActiveAlerts()
	if len(activeAfter) != 0 {
		t.Fatalf("expected 0 active alerts after recovery, got %d", len(activeAfter))
	}
}
