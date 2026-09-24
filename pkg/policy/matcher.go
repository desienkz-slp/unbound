package policy

import (
	"strings"
	"sync"
)

// MatchType defines whether a rule matches exact domains or all subdomains.
type MatchType int

const (
	MatchExact MatchType = iota
	MatchSubdomain
)

// RuleEntry represents a single filtering or allowlist rule with metadata.
type RuleEntry struct {
	Domain    string
	MatchType MatchType
	Category  string
	Source    string
	Original  string
}

// TrieNode implements a reversed-label domain trie for O(L) subdomain lookups.
type TrieNode struct {
	children map[string]*TrieNode
	rule     *RuleEntry
	isEnd    bool
}

func newTrieNode() *TrieNode {
	return &TrieNode{
		children: make(map[string]*TrieNode),
	}
}

// DomainMatcher performs high-speed domain and subdomain lookups.
type DomainMatcher struct {
	mu         sync.RWMutex
	exactRules map[string]*RuleEntry
	root       *TrieNode
	totalRules int
}

// NewDomainMatcher initializes an empty matcher.
func NewDomainMatcher() *DomainMatcher {
	return &DomainMatcher{
		exactRules: make(map[string]*RuleEntry),
		root:       newTrieNode(),
	}
}

// AddRule adds an exact or subdomain rule into the matcher.
func (m *DomainMatcher) AddRule(entry *RuleEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	domain := normalizeDomain(entry.Domain)
	if domain == "" {
		return
	}

	entry.Domain = domain

	if entry.MatchType == MatchExact {
		m.exactRules[domain] = entry
	} else {
		// Insert into reversed-label trie for subdomain matching
		labels := splitReversedLabels(domain)
		curr := m.root
		for _, label := range labels {
			if _, exists := curr.children[label]; !exists {
				curr.children[label] = newTrieNode()
			}
			curr = curr.children[label]
		}
		curr.isEnd = true
		curr.rule = entry
	}
	m.totalRules++
}

// Match checks whether a domain matches any exact rule or subdomain tree.
func (m *DomainMatcher) Match(domain string) *RuleEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	norm := normalizeDomain(domain)
	if norm == "" {
		return nil
	}

	// 1. Check Exact Match: O(1)
	if rule, exists := m.exactRules[norm]; exists {
		return rule
	}

	// 2. Check Subdomain Match via Reversed Trie: O(Labels)
	labels := splitReversedLabels(norm)
	curr := m.root
	for _, label := range labels {
		next, exists := curr.children[label]
		if !exists {
			break
		}
		curr = next
		if curr.isEnd && curr.rule != nil {
			return curr.rule
		}
	}

	return nil
}

// TotalRules returns the number of indexed rules.
func (m *DomainMatcher) TotalRules() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalRules
}

func normalizeDomain(d string) string {
	d = strings.TrimSpace(strings.ToLower(d))
	d = strings.TrimSuffix(d, ".")
	d = strings.TrimPrefix(d, ".")
	return d
}

// splitReversedLabels converts "ads.example.com" -> ["com", "example", "ads"]
func splitReversedLabels(domain string) []string {
	parts := strings.Split(domain, ".")
	n := len(parts)
	reversed := make([]string, n)
	for i, p := range parts {
		reversed[n-1-i] = p
	}
	return reversed
}
