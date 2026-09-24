package hermes

import (
	"context"
	"testing"

	"netora-dns/pkg/model"
	"netora-dns/pkg/policy"
)

func TestHermesInvestigate_BlockedDomain(t *testing.T) {
	pe := policy.NewPolicyEngine()
	pe.AddBlockRule("malicious.phishing.com", "PHISHING", "OpenPhish", "||malicious.phishing.com^")

	inv := NewInvestigator("dns-dev", pe, nil)

	report := inv.Investigate(context.Background(), "malicious.phishing.com", "172.18.20.100", "DEFAULT", nil)

	if report.Verdict != "BLOCKED" {
		t.Fatalf("expected verdict BLOCKED, got %s", report.Verdict)
	}
	if report.RootCause != "FILTERED_PHISHING" {
		t.Errorf("expected root cause FILTERED_PHISHING, got %s", report.RootCause)
	}
	if len(report.EvidenceChain) < 2 {
		t.Errorf("expected at least 2 evidence steps, got %d", len(report.EvidenceChain))
	}
}

func TestHermesInvestigate_ClientACLDenied(t *testing.T) {
	pe := policy.NewPolicyEngine()
	inv := NewInvestigator("dns-dev", pe, nil)

	state := &model.DesiredState{
		ClientNetworks: []model.ClientNetwork{
			{CIDR: "10.0.0.0/24", Enabled: true, Description: "Internal LAN"},
		},
	}

	// Client IP from outside authorized subnet
	report := inv.Investigate(context.Background(), "example.com", "192.168.1.50", "DEFAULT", state)

	if report.Verdict != "BLOCKED" {
		t.Fatalf("expected verdict BLOCKED due to ACL, got %s", report.Verdict)
	}
	if report.RootCause != "CLIENT_ACL_DENIED" {
		t.Errorf("expected root cause CLIENT_ACL_DENIED, got %s", report.RootCause)
	}
}

func TestHermesInvestigate_InsufficientEvidence(t *testing.T) {
	pe := policy.NewPolicyEngine()
	inv := NewInvestigator("dns-dev", pe, nil)

	report := inv.Investigate(context.Background(), "", "", "DEFAULT", nil)

	if !report.InsufficientEvidence {
		t.Errorf("expected InsufficientEvidence to be true for empty input")
	}
	if report.Verdict != "INSUFFICIENT_EVIDENCE" {
		t.Errorf("expected verdict INSUFFICIENT_EVIDENCE, got %s", report.Verdict)
	}
}
