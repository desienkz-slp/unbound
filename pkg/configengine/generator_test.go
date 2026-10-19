package configengine

import (
	"strings"
	"testing"

	"netora-dns/pkg/model"
)

func TestGenerator_RecursiveMode(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 4, 4096)
	state.ResolutionMode = model.ModeRecursive

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	if len(res.Files) == 0 {
		t.Fatalf("expected generated files, got 0")
	}

	mainConf := string(res.Files[0].Content)

	// Check core directives
	if !strings.Contains(mainConf, "module-config: \"validator iterator\"") {
		t.Errorf("expected DNSSEC validator iterator in recursive mode")
	}
	if !strings.Contains(mainConf, "auto-trust-anchor-file:") {
		t.Errorf("expected auto-trust-anchor-file")
	}
	if !strings.Contains(mainConf, "access-control: 0.0.0.0/0 refuse") {
		t.Errorf("expected default refuse ACL")
	}
	if strings.Contains(mainConf, "forward-zone:") {
		t.Errorf("did not expect forward-zone in pure recursive mode")
	}
}

func TestGenerator_ForwardingMode(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.ResolutionMode = model.ModeForwarding

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	mainConf := string(res.Files[0].Content)
	if !strings.Contains(mainConf, "forward-zone:") {
		t.Errorf("expected forward-zone in forwarding mode")
	}
	if !strings.Contains(mainConf, "forward-addr: 1.1.1.1") {
		t.Errorf("expected 1.1.1.1 upstream")
	}
	if !strings.Contains(mainConf, "forward-addr: 8.8.8.8") {
		t.Errorf("expected 8.8.8.8 upstream")
	}
}

func TestGenerator_HybridMode(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.ResolutionMode = model.ModeHybrid

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	mainConf := string(res.Files[0].Content)
	if !strings.Contains(mainConf, "forward-first: yes") {
		t.Errorf("expected forward-first: yes in hybrid mode")
	}
}

func TestGenerator_MultiListenerIPv4IPv6(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.Listeners = append(state.Listeners, model.Listener{
		ID:      "listen-ipv6",
		Name:    "Client IPv6",
		IP:      "2001:db8::1",
		Port:    53,
		Role:    model.RoleClient,
		IsIPv6:  true,
		UDP:     true,
		TCP:     true,
		Enabled: true,
	})

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	mainConf := string(res.Files[0].Content)
	if !strings.Contains(mainConf, "do-ip6: yes") {
		t.Errorf("expected do-ip6: yes when IPv6 listener is present")
	}
	if !strings.Contains(mainConf, "interface: 2001:db8::1") {
		t.Errorf("expected IPv6 interface binding")
	}
}

func TestGenerator_SecurityValidationOpenResolver(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	// Inject forbidden 0.0.0.0/0 allow
	state.ClientNetworks = append(state.ClientNetworks, model.ClientNetwork{
		ID:      "bad-open-resolver",
		CIDR:    "0.0.0.0/0",
		Action:  "allow",
		Enabled: true,
	})

	_, err = gen.Generate(state, "/etc/unbound")
	if err == nil {
		t.Fatalf("expected security error when configuring 0.0.0.0/0 allow, got nil")
	}
	if !strings.Contains(err.Error(), "open resolver") {
		t.Errorf("expected open resolver warning in error, got: %v", err)
	}
}

func TestGenerator_AllFilterConfigs(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.ParentalControl.Enabled = true
	state.GamblingBlock.Enabled = true
	state.AdBlock.Enabled = true
	state.MalwareBlock.Enabled = true
	state.CustomBlock.Enabled = true

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	expectedFiles := map[string]bool{
		"unbound.conf":                 false,
		"conf.d/domain-insecure.conf":  false,
		"conf.d/local-records.conf":    false,
		"conf.d/safesearch.conf":       false,
		"conf.d/parental-control.conf": false,
		"conf.d/gambling-filter.conf":  false,
		"conf.d/adblock-filter.conf":   false,
		"conf.d/malware-filter.conf":   false,
		"conf.d/custom-filter.conf":    false,
		"conf.d/whitelist.conf":        false,
	}

	for _, f := range res.Files {
		if _, ok := expectedFiles[f.RelativePath]; ok {
			expectedFiles[f.RelativePath] = true
		}
	}

	for path, found := range expectedFiles {
		if !found {
			t.Errorf("expected generated file %s was missing", path)
		}
	}
}

func TestGenerator_ResiliencyAndRateLimit(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-isp", 4, 8192)
	state.Features.Prefetch = true
	state.Features.PrefetchKey = true
	state.Features.ServeExpired = true
	state.Features.ServeExpiredTTL = 86400
	state.Features.ServeExpiredTTLReset = true
	state.Features.ServeExpiredClientTimeout = 1800
	state.Features.ServeExpiredReplyTTL = 30

	state.RateLimit.Enabled = true
	state.RateLimit.IPRateLimit = 600
	state.RateLimit.IPRateLimitFactor = 10
	state.RateLimit.DomainRateLimit = 2500
	state.RateLimit.RateLimitSize = "32m"
	state.RateLimit.RateLimitSlabs = 4

	// Deterministically enable the Unbound >= 1.9.0 advanced serve-expired keywords
	// so this test does not depend on whether unbound is installed on the host.
	serveExpiredAdvancedOnce.Do(func() {})
	serveExpiredAdvancedOK = true

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	mainConf := string(res.Files[0].Content)

	expectedDirectives := []string{
		"prefetch: yes",
		"prefetch-key: yes",
		"serve-expired: yes",
		"serve-expired-ttl: 86400",
		"serve-expired-ttl-reset: yes",
		"serve-expired-client-timeout: 1800",
		"serve-expired-reply-ttl: 30",
		"ip-ratelimit: 600",
		"ip-ratelimit-factor: 10",
		"ip-ratelimit-size: 32m",
		"ip-ratelimit-slabs: 4",
		"ratelimit: 2500",
	}

	for _, dir := range expectedDirectives {
		if !strings.Contains(mainConf, dir) {
			t.Errorf("expected directive %q in generated unbound.conf, but not found", dir)
		}
	}
}

func TestGenerator_NodeDnsRewrites(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Case 1: Node with 2 IPs -> 2 rewrites
	stateDual := model.NewDefaultDesiredState("node-dual", 2, 2048)
	stateDual.EnsureNodeDnsRewrites("node-dual", []string{"172.18.20.146", "10.10.10.1"})

	resDual, err := gen.Generate(stateDual, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	var recordsContent string
	for _, f := range resDual.Files {
		if f.RelativePath == "conf.d/local-records.conf" {
			recordsContent = string(f.Content)
			break
		}
	}

	if !strings.Contains(recordsContent, "node-dual 300 A 172.18.20.146") {
		t.Errorf("expected rewrite for 172.18.20.146 in local-records.conf, got:\n%s", recordsContent)
	}
	if !strings.Contains(recordsContent, "node-dual 300 A 10.10.10.1") {
		t.Errorf("expected rewrite for 10.10.10.1 in local-records.conf, got:\n%s", recordsContent)
	}

	// Verify no duplicate A records
	if strings.Count(recordsContent, "node-dual 300 A 172.18.20.146") != 1 {
		t.Errorf("expected exactly 1 occurrence of 172.18.20.146 rewrite, got %d", strings.Count(recordsContent, "node-dual 300 A 172.18.20.146"))
	}

	// Case 2: Node with 1 IP -> 1 rewrite
	stateSingle := model.NewDefaultDesiredState("node-single", 2, 2048)
	stateSingle.EnsureNodeDnsRewrites("node-single", []string{"172.18.20.145"})

	resSingle, err := gen.Generate(stateSingle, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	var recordsContentSingle string
	for _, f := range resSingle.Files {
		if f.RelativePath == "conf.d/local-records.conf" {
			recordsContentSingle = string(f.Content)
			break
		}
	}

	if !strings.Contains(recordsContentSingle, "node-single 300 A 172.18.20.145") {
		t.Errorf("expected rewrite for 172.18.20.145 in local-records.conf, got:\n%s", recordsContentSingle)
	}
	if strings.Count(recordsContentSingle, "node-single 300 A") != 1 {
		t.Errorf("expected exactly 1 rewrite A record, got %d", strings.Count(recordsContentSingle, "node-single 300 A"))
	}
}

func TestGenerator_LogQueriesAndReplies(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Case 1: Default (false) -> log-queries: no, log-replies: no
	stateDefault := model.NewDefaultDesiredState("node-01", 2, 2048)
	resDefault, err := gen.Generate(stateDefault, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate default config: %v", err)
	}
	mainConfDefault := string(resDefault.Files[0].Content)
	if !strings.Contains(mainConfDefault, "log-queries: no") {
		t.Errorf("expected log-queries: no by default")
	}
	if !strings.Contains(mainConfDefault, "log-replies: no") {
		t.Errorf("expected log-replies: no by default")
	}

	// Case 2: Explicit true -> log-queries: yes, log-replies: yes
	stateActive := model.NewDefaultDesiredState("node-02", 2, 2048)
	stateActive.Features.LogQueries = true
	stateActive.Features.LogReplies = true
	resActive, err := gen.Generate(stateActive, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate active logging config: %v", err)
	}
	mainConfActive := string(resActive.Files[0].Content)
	if !strings.Contains(mainConfActive, "log-queries: yes") {
		t.Errorf("expected log-queries: yes when enabled")
	}
	if !strings.Contains(mainConfActive, "log-replies: yes") {
		t.Errorf("expected log-replies: yes when enabled")
	}
}


