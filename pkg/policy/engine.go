package policy

import (
	"fmt"
	"strings"
	"sync"
)

// DecisionAction represents the resolution action decided by the policy engine.
type DecisionAction string

const (
	ActionAllow   DecisionAction = "ALLOW"
	ActionBlock   DecisionAction = "BLOCKED"
	ActionRewrite DecisionAction = "REWRITE"
)

// Decision contains the explainable outcome of a policy evaluation.
type Decision struct {
	Domain       string         `json:"domain"`
	Action       DecisionAction `json:"action"`
	Policy       string         `json:"policy"`
	Category     string         `json:"category,omitempty"`
	Source       string         `json:"source,omitempty"`
	Rule         string         `json:"rule,omitempty"`
	RewriteType  string         `json:"rewrite_type,omitempty"`
	RewriteValue string         `json:"rewrite_value,omitempty"`
	Explanation  string         `json:"explanation"`
}

// RewriteEntry represents a local override record.
type RewriteEntry struct {
	Domain string
	Type   string
	Value  string
	TTL    int
}

// PolicyEngine evaluates DNS query policies with full explainability.
type PolicyEngine struct {
	mu                 sync.RWMutex
	emergencyOverrides map[string]DecisionAction // domain -> ALLOW / BLOCKED
	allowlistMatcher   *DomainMatcher
	blocklistMatcher   *DomainMatcher
	rewrites           map[string]*RewriteEntry
	categoryMatchers   map[string]*DomainMatcher // category -> matcher
}

// NewPolicyEngine initializes the policy engine.
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		emergencyOverrides: make(map[string]DecisionAction),
		allowlistMatcher:   NewDomainMatcher(),
		blocklistMatcher:   NewDomainMatcher(),
		rewrites:           make(map[string]*RewriteEntry),
		categoryMatchers:   make(map[string]*DomainMatcher),
	}
}

// SetEmergencyOverride sets a highest-priority manual bypass or block.
func (e *PolicyEngine) SetEmergencyOverride(domain string, action DecisionAction) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.emergencyOverrides[normalizeDomain(domain)] = action
}

// AddAllowlistRule registers an allowlist entry that overrides all blocklists.
func (e *PolicyEngine) AddAllowlistRule(domain, category, source string) {
	e.allowlistMatcher.AddRule(&RuleEntry{
		Domain:    domain,
		MatchType: MatchSubdomain,
		Category:  category,
		Source:    source,
		Original:  domain,
	})
}

// AddRewriteRule registers a static DNS rewrite (local record).
func (e *PolicyEngine) AddRewriteRule(domain, recordType, value string, ttl int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	norm := normalizeDomain(domain)
	e.rewrites[norm] = &RewriteEntry{
		Domain: norm,
		Type:   recordType,
		Value:  value,
		TTL:    ttl,
	}
}

// AddBlockRule registers a blocklist rule tagged with category and feed source.
func (e *PolicyEngine) AddBlockRule(domain, category, source, original string) {
	entry := &RuleEntry{
		Domain:    domain,
		MatchType: MatchSubdomain,
		Category:  category,
		Source:    source,
		Original:  original,
	}
	e.blocklistMatcher.AddRule(entry)

	// Also index into category matcher
	e.mu.Lock()
	catMatcher, exists := e.categoryMatchers[category]
	if !exists {
		catMatcher = NewDomainMatcher()
		e.categoryMatchers[category] = catMatcher
	}
	e.mu.Unlock()
	catMatcher.AddRule(entry)
}

// Evaluate performs hierarchical, explainable policy resolution for a domain.
func (e *PolicyEngine) Evaluate(domain, policyGroup string) *Decision {
	normDomain := normalizeDomain(domain)
	if policyGroup == "" {
		policyGroup = "DEFAULT"
	}

	// 1. Emergency Overrides
	e.mu.RLock()
	if action, exists := e.emergencyOverrides[normDomain]; exists {
		e.mu.RUnlock()
		return &Decision{
			Domain:      normDomain,
			Action:      action,
			Policy:      policyGroup,
			Explanation: fmt.Sprintf("Emergency override: %s", action),
		}
	}
	e.mu.RUnlock()

	// 2. Explicit Allowlist Precedence
	if allowRule := e.allowlistMatcher.Match(normDomain); allowRule != nil {
		return &Decision{
			Domain:      normDomain,
			Action:      ActionAllow,
			Policy:      policyGroup,
			Category:    allowRule.Category,
			Source:      allowRule.Source,
			Rule:        allowRule.Original,
			Explanation: fmt.Sprintf("Allowed by allowlist rule from '%s' (%s)", allowRule.Source, allowRule.Original),
		}
	}

	// 3. Custom Local Rewrites
	e.mu.RLock()
	if rewrite, exists := e.rewrites[normDomain]; exists {
		e.mu.RUnlock()
		return &Decision{
			Domain:       normDomain,
			Action:       ActionRewrite,
			Policy:       policyGroup,
			RewriteType:  rewrite.Type,
			RewriteValue: rewrite.Value,
			Explanation:  fmt.Sprintf("Rewritten locally to %s %s", rewrite.Type, rewrite.Value),
		}
	}
	e.mu.RUnlock()

	// 4. Blocklist Feed Match
	if blockRule := e.blocklistMatcher.Match(normDomain); blockRule != nil {
		return &Decision{
			Domain:      normDomain,
			Action:      ActionBlock,
			Policy:      policyGroup,
			Category:    blockRule.Category,
			Source:      blockRule.Source,
			Rule:        blockRule.Original,
			Explanation: fmt.Sprintf("Blocked by category '%s' from feed '%s' (Rule: %s)", blockRule.Category, blockRule.Source, blockRule.Original),
		}
	}

	// 5. Normal Resolution Fall-through
	return &Decision{
		Domain:      normDomain,
		Action:      ActionAllow,
		Policy:      policyGroup,
		Explanation: "Domain allowed (no blocking rules matched)",
	}
}

// GenerateUnboundBlockConf renders the memory block rules into an Unbound configuration file.
func (e *PolicyEngine) GenerateUnboundBlockConf(redirectIP string) string {
	var sb strings.Builder
	sb.WriteString("# ==============================================================================\n")
	sb.WriteString("# Netora DNS — High Performance Blocklist Configuration\n")
	sb.WriteString("# ==============================================================================\n\n")
	sb.WriteString("server:\n")

	e.blocklistMatcher.mu.RLock()
	defer e.blocklistMatcher.mu.RUnlock()

	// Output exact rules
	for domain, rule := range e.blocklistMatcher.exactRules {
		// Verify not in allowlist
		if e.allowlistMatcher.Match(domain) != nil {
			continue
		}
		if rule.Category == "ADULT" && redirectIP != "" {
			sb.WriteString(fmt.Sprintf("  local-data: \"%s A %s\"\n", domain, redirectIP))
			sb.WriteString(fmt.Sprintf("  local-data: \"www.%s A %s\"\n", domain, redirectIP))
		} else {
			sb.WriteString(fmt.Sprintf("  local-zone: \"%s\" static\n", domain))
		}
	}

	// Output subdomain trie rules
	var traverse func(node *TrieNode, labelStack []string)
	traverse = func(node *TrieNode, labelStack []string) {
		if node.isEnd && node.rule != nil {
			domain := node.rule.Domain
			if e.allowlistMatcher.Match(domain) == nil {
				if node.rule.Category == "ADULT" && redirectIP != "" {
					sb.WriteString(fmt.Sprintf("  local-data: \"%s A %s\"\n", domain, redirectIP))
					sb.WriteString(fmt.Sprintf("  local-data: \"www.%s A %s\"\n", domain, redirectIP))
				} else {
					sb.WriteString(fmt.Sprintf("  local-zone: \"%s\" static\n", domain))
				}
			}
		}
		for label, child := range node.children {
			traverse(child, append(labelStack, label))
		}
	}
	traverse(e.blocklistMatcher.root, nil)

	return sb.String()
}
