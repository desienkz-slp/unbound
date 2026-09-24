package policy

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

var (
	// Matches ABP syntax: ||domain.com^ or ||domain.com
	abpRegex = regexp.MustCompile(`^\|\|([a-zA-Z0-9\.\-_]+)\^?`)
	// Matches ABP allowlist: @@||domain.com^
	abpAllowRegex = regexp.MustCompile(`^@@\|\|([a-zA-Z0-9\.\-_]+)\^?`)
	// Basic domain validator
	validDomainRegex = regexp.MustCompile(`^([a-zA-Z0-9_]([a-zA-Z0-9\-_]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
)

// ParseFeed reads a rule stream (ABP, hosts, or plain domains) and yields RuleEntries.
func ParseFeed(r io.Reader, category, source string) ([]*RuleEntry, error) {
	scanner := bufio.NewScanner(r)
	var rules []*RuleEntry

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		// 1. Check ABP Allowlist rule (e.g. @@||example.com^)
		if matches := abpAllowRegex.FindStringSubmatch(line); len(matches) > 1 {
			domain := normalizeDomain(matches[1])
			if isValidDomain(domain) {
				rules = append(rules, &RuleEntry{
					Domain:    domain,
					MatchType: MatchSubdomain,
					Category:  "ALLOWLIST",
					Source:    source,
					Original:  line,
				})
			}
			continue
		}

		// 2. Check ABP Block rule (e.g. ||doubleclick.net^)
		if matches := abpRegex.FindStringSubmatch(line); len(matches) > 1 {
			domain := normalizeDomain(matches[1])
			if isValidDomain(domain) {
				rules = append(rules, &RuleEntry{
					Domain:    domain,
					MatchType: MatchSubdomain,
					Category:  category,
					Source:    source,
					Original:  line,
				})
			}
			continue
		}

		// 3. Check HOSTS format (e.g. 0.0.0.0 badsite.com or 127.0.0.1 badsite.com)
		fields := strings.Fields(line)
		if len(fields) >= 2 && (fields[0] == "0.0.0.0" || fields[0] == "127.0.0.1") {
			domain := normalizeDomain(fields[1])
			if domain != "localhost" && domain != "broadcasthost" && isValidDomain(domain) {
				rules = append(rules, &RuleEntry{
					Domain:    domain,
					MatchType: MatchExact,
					Category:  category,
					Source:    source,
					Original:  line,
				})
			}
			continue
		}

		// 4. Check Plain Domain
		cleanDomain := normalizeDomain(fields[0])
		if isValidDomain(cleanDomain) {
			rules = append(rules, &RuleEntry{
				Domain:    cleanDomain,
				MatchType: MatchSubdomain,
				Category:  category,
				Source:    source,
				Original:  line,
			})
		}
	}

	return rules, scanner.Err()
}

func isValidDomain(d string) bool {
	if len(d) < 3 || len(d) > 253 {
		return false
	}
	return validDomainRegex.MatchString(d)
}
