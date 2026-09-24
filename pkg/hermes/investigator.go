package hermes

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"netora-dns/pkg/model"
	"netora-dns/pkg/policy"
	"netora-dns/pkg/runtime"
)

// EvidenceStep records a verified step in the investigation chain.
type EvidenceStep struct {
	Stage       string `json:"stage"`
	Passed      bool   `json:"passed"`
	Detail      string `json:"detail"`
	Evidence    string `json:"evidence"`
	Timestamp   string `json:"timestamp"`
}

// InvestigationReport contains the end-to-end NOC diagnosis.
type InvestigationReport struct {
	Domain               string         `json:"domain"`
	ClientIP             string         `json:"client_ip"`
	NodeID               string         `json:"node_id"`
	PrimaryFinding       string         `json:"primary_finding"`
	RootCause            string         `json:"root_cause"`
	Verdict              string         `json:"verdict"` // ALLOWED, BLOCKED, NETWORK_FAILURE, DNSSEC_BOGUS, INSUFFICIENT_EVIDENCE
	RecommendedAction    string         `json:"recommended_action"`
	EvidenceChain        []EvidenceStep `json:"evidence_chain"`
	InsufficientEvidence bool           `json:"insufficient_evidence"`
	CompletedAt          time.Time      `json:"completed_at"`
}

// Investigator coordinates multi-layer DNS and network diagnostics.
type Investigator struct {
	nodeID        string
	policyEngine  *policy.PolicyEngine
	healthChecker *runtime.DNSHealthChecker
}

// NewInvestigator creates an AI-NOC investigator.
func NewInvestigator(nodeID string, pe *policy.PolicyEngine, hc *runtime.DNSHealthChecker) *Investigator {
	if hc == nil {
		hc = runtime.NewDNSHealthChecker(3 * time.Second)
	}
	return &Investigator{
		nodeID:        nodeID,
		policyEngine:  pe,
		healthChecker: hc,
	}
}

// Investigate executes the 6-stage evidence collection pipeline.
func (inv *Investigator) Investigate(ctx context.Context, domain, clientIP, policyGroup string, state *model.DesiredState) *InvestigationReport {
	now := time.Now().UTC()
	report := &InvestigationReport{
		Domain:        domain,
		ClientIP:      clientIP,
		NodeID:        inv.nodeID,
		EvidenceChain: make([]EvidenceStep, 0),
		CompletedAt:   now,
	}

	if domain == "" {
		report.Verdict = "INSUFFICIENT_EVIDENCE"
		report.PrimaryFinding = "No domain specified for investigation."
		report.InsufficientEvidence = true
		return report
	}

	// Stage 1: Client Network ACL Verification
	aclPassed := true
	aclDetail := "Client IP is permitted by ACL."
	if clientIP != "" && state != nil && len(state.ClientNetworks) > 0 {
		clientNet := net.ParseIP(clientIP)
		if clientNet != nil {
			matched := false
			for _, cn := range state.ClientNetworks {
				if !cn.Enabled {
					continue
				}
				_, cidrNet, err := net.ParseCIDR(cn.CIDR)
				if err == nil && cidrNet.Contains(clientNet) {
					matched = true
					aclDetail = fmt.Sprintf("Client IP %s matched authorized network %s (%s).", clientIP, cn.CIDR, cn.Description)
					break
				}
			}
			if !matched {
				aclPassed = false
				aclDetail = fmt.Sprintf("Client IP %s is NOT in any authorized subscriber network (REFUSED).", clientIP)
			}
		}
	}

	report.EvidenceChain = append(report.EvidenceChain, EvidenceStep{
		Stage:     "1. Client ACL Verification",
		Passed:    aclPassed,
		Detail:    aclDetail,
		Evidence:  fmt.Sprintf("ClientIP=%s", clientIP),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})

	if !aclPassed {
		report.Verdict = "BLOCKED"
		report.RootCause = "CLIENT_ACL_DENIED"
		report.PrimaryFinding = fmt.Sprintf("Queries from %s are dropped by Unbound ACL to prevent open resolver abuse.", clientIP)
		report.RecommendedAction = "Add the client subnet to authorized Client Networks in DNS Configuration."
		return report
	}

	// Stage 2: Policy & Threat Feed Evaluation
	decision := inv.policyEngine.Evaluate(domain, policyGroup)
	policyPassed := decision.Action != policy.ActionBlock

	report.EvidenceChain = append(report.EvidenceChain, EvidenceStep{
		Stage:     "2. DNS Policy & Threat Filtering",
		Passed:    policyPassed,
		Detail:    decision.Explanation,
		Evidence:  fmt.Sprintf("Action=%s, Source=%s, Category=%s, Rule=%s", decision.Action, decision.Source, decision.Category, decision.Rule),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})

	if !policyPassed {
		report.Verdict = "BLOCKED"
		report.RootCause = fmt.Sprintf("FILTERED_%s", decision.Category)
		report.PrimaryFinding = fmt.Sprintf("Domain '%s' is blocked by %s (Rule: %s, Category: %s).", domain, decision.Source, decision.Rule, decision.Category)
		report.RecommendedAction = fmt.Sprintf("If this is a legitimate service, click 'Add to Allowlist' for '%s' under Category '%s'.", domain, decision.Category)
		return report
	}

	// Stage 3: Rewrites & SafeSearch Verification
	if decision.Action == policy.ActionRewrite {
		report.EvidenceChain = append(report.EvidenceChain, EvidenceStep{
			Stage:     "3. Rewrite & SafeSearch Enforcement",
			Passed:    true,
			Detail:    fmt.Sprintf("SafeSearch active: redirecting %s to %s.", domain, decision.RewriteValue),
			Evidence:  fmt.Sprintf("CNAME Target=%s", decision.RewriteValue),
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		})
		report.Verdict = "ALLOWED"
		report.RootCause = "SAFESEARCH_REDIRECT"
		report.PrimaryFinding = fmt.Sprintf("Domain '%s' is actively redirected to %s via SafeSearch policy.", domain, decision.RewriteValue)
		report.RecommendedAction = "None required. SafeSearch is functioning as expected."
		return report
	}

	// Stage 4: Live Data Plane DNS Resolution
	probe := inv.healthChecker.CheckResolution(ctx, "127.0.0.1", 53, "udp", domain)
	dnsPassed := probe.Healthy

	report.EvidenceChain = append(report.EvidenceChain, EvidenceStep{
		Stage:     "4. Live Data Plane Resolution",
		Passed:    dnsPassed,
		Detail:    fmt.Sprintf("Response time: %v, Answers: %v", probe.Latency, strings.Join(probe.ResolvedIPs, ", ")),
		Evidence:  fmt.Sprintf("Healthy=%v, Error=%s, Latency=%v", probe.Healthy, probe.ErrorMessage, probe.Latency),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})

	if !dnsPassed {
		report.Verdict = "NETWORK_FAILURE"
		report.RootCause = "UPSTREAM_OR_DNSSEC_FAILURE"
		report.PrimaryFinding = fmt.Sprintf("Local resolver failed to resolve '%s': %s", domain, probe.ErrorMessage)
		report.RecommendedAction = "Check upstream DNS connectivity or inspect if domain has DNSSEC validation errors."
		return report
	}

	// Stage 5: Synthesis
	report.Verdict = "ALLOWED"
	report.RootCause = "NONE"
	report.PrimaryFinding = fmt.Sprintf("Domain '%s' is fully operational and allowed across all policy filters (Resolution time: %v).", domain, probe.Latency)
	report.RecommendedAction = "Domain resolves cleanly. If client continues to report issues, verify local device DHCP, TCP port 443 connectivity, or HTTP CDN reachability."

	return report
}
