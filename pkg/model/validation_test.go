package model

import (
	"strings"
	"testing"
)

func TestValidateDesiredState_UpstreamProtocolConsistency(t *testing.T) {
	state := NewDefaultDesiredState("test-node", 4, 4096)
	state.ResolutionMode = ModeHybrid

	// Case 1: Pure UDP Upstreams -> Valid
	state.Upstreams = []UpstreamServer{
		{ID: "up-1", Name: "Google 1", IP: "8.8.8.8", Port: 53, Type: UpstreamUDP, Enabled: true},
		{ID: "up-2", Name: "Cloudflare 1", IP: "1.1.1.1", Port: 53, Type: UpstreamUDP, Enabled: true},
	}
	errs := ValidateDesiredState(state)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors for pure UDP upstreams, got: %v", errs)
	}

	// Case 2: Pure DoT Upstreams -> Valid
	state.Upstreams = []UpstreamServer{
		{ID: "up-1", Name: "Google DoT", IP: "8.8.8.8", Port: 853, Type: UpstreamDoT, TLSName: "dns.google", Enabled: true},
		{ID: "up-2", Name: "Cloudflare DoT", IP: "1.1.1.1", Port: 853, Type: UpstreamDoT, TLSName: "cloudflare-dns.com", Enabled: true},
	}
	errs = ValidateDesiredState(state)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors for pure DoT upstreams, got: %v", errs)
	}

	// Case 3: Mixed UDP and DoT -> Expected Validation Error
	state.Upstreams = []UpstreamServer{
		{ID: "up-1", Name: "Google UDP", IP: "8.8.8.8", Port: 53, Type: UpstreamUDP, Enabled: true},
		{ID: "up-2", Name: "Google DoT", IP: "8.8.8.8", Port: 853, Type: UpstreamDoT, TLSName: "dns.google", Enabled: true},
	}
	errs = ValidateDesiredState(state)
	foundMixedErr := false
	for _, e := range errs {
		if strings.Contains(e.Message, "Pencampuran protokol terdeteksi") {
			foundMixedErr = true
			break
		}
	}
	if !foundMixedErr {
		t.Fatalf("expected protocol mix error, got: %v", errs)
	}

	// Case 4: Zero active upstreams in hybrid mode -> Expected Validation Error
	state.Upstreams = []UpstreamServer{
		{ID: "up-1", Name: "Google UDP", IP: "8.8.8.8", Port: 53, Type: UpstreamUDP, Enabled: false},
	}
	errs = ValidateDesiredState(state)
	foundZeroErr := false
	for _, e := range errs {
		if strings.Contains(e.Message, "requires at least one active upstream server") {
			foundZeroErr = true
			break
		}
	}
	if !foundZeroErr {
		t.Fatalf("expected zero active upstreams error, got: %v", errs)
	}
}
