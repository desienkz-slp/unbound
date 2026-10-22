package model

import (
	"testing"
)

func TestEnsureNodeDnsRewrites_SingleIP(t *testing.T) {
	state := NewDefaultDesiredState("dns-node-01", 2, 2048)
	ips := []string{"172.18.20.145"}

	changed := state.EnsureNodeDnsRewrites("dns-node-01", ips)
	if !changed {
		t.Fatalf("expected EnsureNodeDnsRewrites to return true on initial injection")
	}

	if len(state.LocalRecords) != 2 {
		t.Fatalf("expected exactly 2 DNS rewrites (A+PTR), got %d", len(state.LocalRecords))
	}

	// Verify A record
	rec := state.LocalRecords[0]
	if rec.Domain != "dns-node-01" {
		t.Errorf("expected domain 'dns-node-01', got '%s'", rec.Domain)
	}
	if rec.Type != "A" {
		t.Errorf("expected type 'A', got '%s'", rec.Type)
	}
	if rec.Value != "172.18.20.145" {
		t.Errorf("expected IP '172.18.20.145', got '%s'", rec.Value)
	}

	// Verify PTR record
	ptr := state.LocalRecords[1]
	if ptr.Type != "PTR" || ptr.Value != "172.18.20.145" {
		t.Errorf("expected PTR for 172.18.20.145, got %+v", ptr)
	}

	// Calling again should be idempotent and return false
	changedAgain := state.EnsureNodeDnsRewrites("dns-node-01", ips)
	if changedAgain {
		t.Errorf("expected EnsureNodeDnsRewrites to return false when nothing changed")
	}
	if len(state.LocalRecords) != 2 {
		t.Errorf("expected still exactly 2 DNS rewrites, got %d", len(state.LocalRecords))
	}
}

func TestEnsureNodeDnsRewrites_DualIP(t *testing.T) {
	state := NewDefaultDesiredState("netora-dns1", 4, 4096)
	ips := []string{"172.18.20.146", "10.10.10.1"}

	changed := state.EnsureNodeDnsRewrites("netora-dns1", ips)
	if !changed {
		t.Fatalf("expected EnsureNodeDnsRewrites to return true on initial injection")
	}

	if len(state.LocalRecords) != 4 {
		t.Fatalf("expected exactly 4 DNS rewrites (2 A + 2 PTR) for 2 IPs, got %d", len(state.LocalRecords))
	}

	foundIP1 := false
	foundIP2 := false
	for _, r := range state.LocalRecords {
		if r.Domain != "netora-dns1" {
			t.Errorf("unexpected record: %+v", r)
		}
		if r.Value == "172.18.20.146" {
			foundIP1 = true
		}
		if r.Value == "10.10.10.1" {
			foundIP2 = true
		}
	}

	if !foundIP1 || !foundIP2 {
		t.Fatalf("expected rewrites for both 172.18.20.146 and 10.10.10.1, got: %+v", state.LocalRecords)
	}
}

func TestEnsureNodeDnsRewrites_PreservesOtherDomains(t *testing.T) {
	state := NewDefaultDesiredState("node-alpha", 2, 2048)
	state.LocalRecords = []LocalRecord{
		{Domain: "router.lan", Type: "A", Value: "192.168.1.1", TTL: 300},
		{Domain: "nas.lan", Type: "CNAME", Value: "storage.lan", TTL: 300},
	}

	ips := []string{"192.168.1.100"}
	changed := state.EnsureNodeDnsRewrites("node-alpha", ips)
	if !changed {
		t.Fatalf("expected changed=true")
	}

	if len(state.LocalRecords) != 4 {
		t.Fatalf("expected 4 records (2 custom + 1 A + 1 PTR), got %d: %+v", len(state.LocalRecords), state.LocalRecords)
	}

	foundRouter := false
	foundNas := false
	foundNode := false
	for _, r := range state.LocalRecords {
		if r.Domain == "router.lan" && r.Value == "192.168.1.1" {
			foundRouter = true
		}
		if r.Domain == "nas.lan" && r.Value == "storage.lan" {
			foundNas = true
		}
		if r.Domain == "node-alpha" && r.Value == "192.168.1.100" {
			foundNode = true
		}
	}

	if !foundRouter || !foundNas || !foundNode {
		t.Fatalf("custom records were not preserved correctly: %+v", state.LocalRecords)
	}
}

func TestEnsureNodeDnsRewrites_ExtractFromListeners(t *testing.T) {
	state := NewDefaultDesiredState("node-beta", 2, 2048)
	state.Listeners = []Listener{
		{ID: "l1", IP: "127.0.0.1", Port: 53, Role: RoleManagement, Enabled: true},
		{ID: "l2", IP: "10.0.0.5", Port: 53, Role: RoleClient, Enabled: true},
		{ID: "l3", IP: "10.0.0.6", Port: 53, Role: RoleClient, Enabled: true},
	}

	// Pass empty slice to test extracting from listeners
	changed := state.EnsureNodeDnsRewrites("node-beta", nil)
	if !changed {
		t.Fatalf("expected changed=true")
	}

	if len(state.LocalRecords) != 4 {
		t.Fatalf("expected 4 records (2 A + 2 PTR) extracted from concrete listeners, got %d: %+v", len(state.LocalRecords), state.LocalRecords)
	}
}

func TestNormalizeSlabs(t *testing.T) {
	cases := []struct {
		name string
		in   PerformanceConfig
		want [4]int // msg, rrset, key, infra
	}{
		{"UI omits key/infra", PerformanceConfig{MsgCacheSlabs: 4, RRSetCacheSlabs: 4}, [4]int{4, 4, 4, 4}},
		{"all zero", PerformanceConfig{}, [4]int{4, 4, 4, 4}},
		{"non power of 2", PerformanceConfig{MsgCacheSlabs: 6, RRSetCacheSlabs: 12, KeyCacheSlabs: 3, InfraCacheSlabs: 5}, [4]int{4, 8, 2, 4}},
		{"valid kept", PerformanceConfig{MsgCacheSlabs: 8, RRSetCacheSlabs: 8, KeyCacheSlabs: 2, InfraCacheSlabs: 1}, [4]int{8, 8, 2, 1}},
	}
	for _, c := range cases {
		p := c.in
		p.NormalizeSlabs()
		got := [4]int{p.MsgCacheSlabs, p.RRSetCacheSlabs, p.KeyCacheSlabs, p.InfraCacheSlabs}
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestNormalizeIncomingNumTCP(t *testing.T) {
	p := PerformanceConfig{}
	p.Normalize()
	if p.IncomingNumTCP != 1024 {
		t.Errorf("omitted incoming_num_tcp: got %d want 1024", p.IncomingNumTCP)
	}
	p = PerformanceConfig{IncomingNumTCP: 4096}
	p.Normalize()
	if p.IncomingNumTCP != 4096 {
		t.Errorf("explicit incoming_num_tcp overwritten: got %d", p.IncomingNumTCP)
	}
}

func TestNormalizeAntiDropBuffers(t *testing.T) {
	p := PerformanceConfig{}
	p.Normalize()
	if p.JostleTimeout != 200 {
		t.Errorf("omitted jostle_timeout: got %d want 200", p.JostleTimeout)
	}
	if p.NumQueriesPerThread < 4096 {
		t.Errorf("expected NumQueriesPerThread >= 4096, got %d", p.NumQueriesPerThread)
	}
	if p.OutgoingRange < 16384 {
		t.Errorf("expected OutgoingRange >= 16384, got %d", p.OutgoingRange)
	}

	// Custom higher values should be preserved
	p2 := PerformanceConfig{
		NumQueriesPerThread: 8192,
		OutgoingRange:       32768,
		JostleTimeout:       300,
	}
	p2.Normalize()
	if p2.JostleTimeout != 300 {
		t.Errorf("explicit jostle_timeout overwritten: got %d want 300", p2.JostleTimeout)
	}
	if p2.NumQueriesPerThread != 8192 {
		t.Errorf("explicit NumQueriesPerThread overwritten: got %d want 8192", p2.NumQueriesPerThread)
	}
	if p2.OutgoingRange != 32768 {
		t.Errorf("explicit OutgoingRange overwritten: got %d want 32768", p2.OutgoingRange)
	}
}
