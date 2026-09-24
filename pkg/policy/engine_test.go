package policy

import (
	"strings"
	"testing"
	"time"
)

func TestPolicy_PrecedenceHierarchy(t *testing.T) {
	engine := NewPolicyEngine()

	// 1. Add block rule: doubleclick.net
	engine.AddBlockRule("doubleclick.net", "ADS", "Hagezi PopupAds", "||doubleclick.net^")

	// 2. Add allowlist rule: safe.doubleclick.net
	engine.AddAllowlistRule("safe.doubleclick.net", "PARTNER", "Custom Allowlist")

	// 3. Add rewrite rule: internal.corp -> 10.0.0.50
	engine.AddRewriteRule("internal.corp", "A", "10.0.0.50", 300)

	// Check: doubleclick.net -> BLOCKED
	dec1 := engine.Evaluate("doubleclick.net", "HOME")
	if dec1.Action != ActionBlock || dec1.Category != "ADS" || dec1.Source != "Hagezi PopupAds" {
		t.Errorf("expected doubleclick.net to be blocked by Hagezi PopupAds, got %+v", dec1)
	}

	// Check: ads.doubleclick.net (subdomain) -> BLOCKED
	decSub := engine.Evaluate("ads.doubleclick.net", "HOME")
	if decSub.Action != ActionBlock {
		t.Errorf("expected ads.doubleclick.net to be blocked by subdomain match, got %+v", decSub)
	}

	// Check: safe.doubleclick.net (allowlist override) -> ALLOW
	dec2 := engine.Evaluate("safe.doubleclick.net", "HOME")
	if dec2.Action != ActionAllow {
		t.Errorf("expected safe.doubleclick.net to be allowed by allowlist, got %+v", dec2)
	}

	// Check: internal.corp (rewrite) -> REWRITE 10.0.0.50
	dec3 := engine.Evaluate("internal.corp", "HOME")
	if dec3.Action != ActionRewrite || dec3.RewriteValue != "10.0.0.50" {
		t.Errorf("expected internal.corp to be rewritten to 10.0.0.50, got %+v", dec3)
	}

	// Check: Emergency Override
	engine.SetEmergencyOverride("doubleclick.net", ActionAllow)
	dec4 := engine.Evaluate("doubleclick.net", "HOME")
	if dec4.Action != ActionAllow {
		t.Errorf("expected emergency override to allow doubleclick.net, got %+v", dec4)
	}
}

func TestPolicy_FeedParser(t *testing.T) {
	feedData := `
# Advertising blocklist
! Comment line
||adservice.google.com^
||popups.tracking.co^
@@||whitelisted-domain.com^
0.0.0.0 malware-payload.net
127.0.0.1 c2-server.evil
plain-blocked-domain.org
`
	rules, err := ParseFeed(strings.NewReader(feedData), "MALWARE", "TestFeed")
	if err != nil {
		t.Fatalf("failed parsing feed: %v", err)
	}

	if len(rules) != 6 {
		t.Fatalf("expected 6 rules parsed, got %d", len(rules))
	}
}

func TestPolicy_HighSpeedBenchmark(t *testing.T) {
	engine := NewPolicyEngine()

	// Seed 20,000 synthetic rules
	for i := 0; i < 20000; i++ {
		engine.AddBlockRule(
			strings.ToLower(string(rune('a'+(i%26))))+".blocked-"+string(rune('0'+(i%10)))+".net",
			"ADS",
			"BenchFeed",
			"synthetic",
		)
	}

	start := time.Now()
	// Perform 100,000 lookups
	for i := 0; i < 100000; i++ {
		_ = engine.Evaluate("sub.a.blocked-5.net", "DEFAULT")
	}
	duration := time.Since(start)

	t.Logf("100,000 domain lookups against 20,000 rules took: %v (Avg: %v / lookup)", duration, duration/100000)
	if duration > 1*time.Second {
		t.Errorf("lookup performance too slow: %v for 100k queries", duration)
	}
}
