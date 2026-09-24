package security

import (
	"testing"

	"netora-dns/pkg/model"
)

func TestSecurityAudit_CompliantState(t *testing.T) {
	state := &model.DesiredState{
		ClientNetworks: []model.ClientNetwork{
			{CIDR: "172.18.20.0/24", Action: "allow", Enabled: true},
			{CIDR: "127.0.0.0/8", Action: "allow", Enabled: true},
		},
		RemoteControl: model.RemoteControlConfig{
			Enabled:   true,
			Interface: "127.0.0.1",
			Port:      8953,
		},
		DNSSEC: model.DNSSECConfig{
			Enabled:             true,
			AutoTrustAnchorFile: "/var/lib/unbound/root.key",
			RootHintsFile:       "/var/lib/unbound/root.hints",
		},
		Features: model.FeatureConfig{
			HideIdentity: true,
			HideVersion:  true,
		},
	}

	report := AuditDesiredState(state)
	if report.Failed > 0 {
		t.Fatalf("expected 0 failures in compliant state, got %d failures:\n%s", report.Failed, report.SummaryString())
	}
	if report.Passed != 6 {
		t.Errorf("expected 6 passing security checks, got %d", report.Passed)
	}
}

func TestSecurityAudit_OpenResolverDetection(t *testing.T) {
	state := &model.DesiredState{
		ClientNetworks: []model.ClientNetwork{
			{CIDR: "0.0.0.0/0", Action: "allow", Enabled: true}, // Dangerous!
		},
		RemoteControl: model.RemoteControlConfig{Interface: "127.0.0.1"},
		DNSSEC: model.DNSSECConfig{
			Enabled:             true,
			AutoTrustAnchorFile: "/var/lib/unbound/root.key",
		},
		Features: model.FeatureConfig{HideIdentity: true, HideVersion: true},
	}

	report := AuditDesiredState(state)
	if report.Passed == report.TotalChecks {
		t.Fatalf("expected open resolver 0.0.0.0/0 allow to fail audit")
	}

	found := false
	for _, f := range report.Findings {
		if f.RuleID == "SEC-001" && !f.Passed {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected SEC-001 failure for open resolver")
	}
}

func TestSecurityAudit_RemoteControlPublicExposure(t *testing.T) {
	state := &model.DesiredState{
		ClientNetworks: []model.ClientNetwork{
			{CIDR: "172.18.20.0/24", Action: "allow", Enabled: true},
		},
		RemoteControl: model.RemoteControlConfig{
			Enabled:   true,
			Interface: "0.0.0.0", // Dangerous!
			Port:      8953,
		},
		DNSSEC: model.DNSSECConfig{
			Enabled:             true,
			AutoTrustAnchorFile: "/var/lib/unbound/root.key",
		},
		Features: model.FeatureConfig{HideIdentity: true, HideVersion: true},
	}

	report := AuditDesiredState(state)
	found := false
	for _, f := range report.Findings {
		if f.RuleID == "SEC-003" && !f.Passed {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected SEC-003 failure for public remote control exposure")
	}
}
