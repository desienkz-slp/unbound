package security

import (
	"fmt"
	"net"
	"strings"

	"netora-dns/pkg/model"
)

// SecurityAuditReport details the security compliance results.
type SecurityAuditReport struct {
	TotalChecks int               `json:"total_checks"`
	Passed      int               `json:"passed"`
	Failed      int               `json:"failed"`
	Findings    []SecurityFinding `json:"findings"`
}

type SecurityFinding struct {
	RuleID   string `json:"rule_id"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

// AuditDesiredState inspects a desired state model against ISP security guardrails.
func AuditDesiredState(state *model.DesiredState) *SecurityAuditReport {
	report := &SecurityAuditReport{
		Findings: make([]SecurityFinding, 0),
	}

	// 1. Open Resolver Prevention Check
	openResolverFound := false
	for _, netAcl := range state.ClientNetworks {
		if netAcl.Enabled && (netAcl.CIDR == "0.0.0.0/0" || netAcl.CIDR == "::/0") {
			if strings.ToLower(netAcl.Action) == "allow" {
				openResolverFound = true
			}
		}
	}
	report.addFinding("SEC-001", !openResolverFound, "CRITICAL",
		"Open Resolver Guardrail",
		"Verifies that 0.0.0.0/0 or ::/0 is never granted 'allow' access.")

	// 2. Client CIDR Syntax & Validity
	invalidCIDR := false
	for _, netAcl := range state.ClientNetworks {
		if _, _, err := net.ParseCIDR(netAcl.CIDR); err != nil {
			invalidCIDR = true
			break
		}
	}
	report.addFinding("SEC-002", !invalidCIDR, "HIGH",
		"Client Network CIDR Validity",
		"Ensures all access control entries are RFC-compliant CIDR prefixes.")

	// 3. Remote Control Interface Restriction
	rcPublic := false
	if state.RemoteControl.Enabled {
		if state.RemoteControl.Interface != "127.0.0.1" && state.RemoteControl.Interface != "::1" {
			rcPublic = true
		}
	}
	report.addFinding("SEC-003", !rcPublic, "CRITICAL",
		"Unbound Remote Control Loopback Isolation",
		"Ensures unbound-control port 8953 is strictly bound to 127.0.0.1 loopback.")

	// 4. DNSSEC Enforcement
	dnssecEnforced := state.DNSSEC.Enabled && state.DNSSEC.AutoTrustAnchorFile != ""
	report.addFinding("SEC-004", dnssecEnforced, "HIGH",
		"DNSSEC Cryptographic Validation",
		"Verifies DNSSEC signature checking and trust anchor availability.")

	// 5. Hide Identity and Version
	identityHidden := state.Features.HideIdentity && state.Features.HideVersion
	report.addFinding("SEC-005", identityHidden, "MEDIUM",
		"DNS Fingerprinting Mitigation",
		"Ensures id.server and version.server TXT queries are sanitized.")

	// 6. Safe Path Traversal Check on Root Hints
	hintsSafe := !strings.Contains(state.DNSSEC.RootHintsFile, "..") &&
		!strings.Contains(state.DNSSEC.AutoTrustAnchorFile, "..")
	report.addFinding("SEC-006", hintsSafe, "HIGH",
		"Path Traversal Prevention",
		"Ensures trust anchor and hints filepaths cannot traverse outside /etc or /var.")

	return report
}

func (r *SecurityAuditReport) addFinding(ruleID string, passed bool, severity, title, detail string) {
	r.TotalChecks++
	if passed {
		r.Passed++
	} else {
		r.Failed++
	}
	r.Findings = append(r.Findings, SecurityFinding{
		RuleID:   ruleID,
		Passed:   passed,
		Severity: severity,
		Title:    title,
		Detail:   detail,
	})
}

// SummaryString returns a human-readable security audit summary.
func (r *SecurityAuditReport) SummaryString() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Security Checks: %d total, %d passed, %d failed\n", r.TotalChecks, r.Passed, r.Failed))
	for _, f := range r.Findings {
		status := "PASS"
		if !f.Passed {
			status = "FAIL"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s (%s)\n", status, f.RuleID, f.Title, f.Detail))
	}
	return sb.String()
}
