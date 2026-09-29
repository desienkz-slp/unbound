package model

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9_]([a-zA-Z0-9\-_]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// ValidationError contains specific field-level validation errors.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("validation error in '%s': %s", v.Field, v.Message)
}

// ValidateDesiredState performs deep semantic and security verification on the configuration.
func ValidateDesiredState(state *DesiredState) []ValidationError {
	var errs []ValidationError

	if state == nil {
		return append(errs, ValidationError{Field: "state", Message: "desired state cannot be nil"})
	}

	if state.VersionID == "" {
		errs = append(errs, ValidationError{Field: "version_id", Message: "version ID cannot be empty"})
	}

	if state.NodeID == "" {
		errs = append(errs, ValidationError{Field: "node_id", Message: "node ID cannot be empty"})
	}

	// 1. Validate Listeners
	if len(state.Listeners) == 0 {
		errs = append(errs, ValidationError{Field: "listeners", Message: "at least one listener must be configured"})
	}

	activeListeners := 0
	for idx, l := range state.Listeners {
		prefix := fmt.Sprintf("listeners[%d]", idx)
		if !l.Enabled {
			continue
		}
		activeListeners++

		ip := net.ParseIP(l.IP)
		if ip == nil {
			errs = append(errs, ValidationError{Field: prefix + ".ip", Message: fmt.Sprintf("invalid IP address: %s", l.IP)})
		}

		if l.Port <= 0 || l.Port > 65535 {
			errs = append(errs, ValidationError{Field: prefix + ".port", Message: fmt.Sprintf("invalid port: %d (must be 1-65535)", l.Port)})
		}

		// Security Check: Never bind 0.0.0.0 directly with RoleInternet without strict controls
		if l.IP == "0.0.0.0" && l.Role == RoleInternet {
			errs = append(errs, ValidationError{
				Field:   prefix + ".ip",
				Message: "binding 0.0.0.0 as INTERNET role is dangerous; bind to a specific WAN IP address",
			})
		}
	}

	if activeListeners == 0 {
		errs = append(errs, ValidationError{Field: "listeners", Message: "at least one listener must be enabled"})
	}

	// 2. Validate Client Networks (ACL)
	for idx, netw := range state.ClientNetworks {
		prefix := fmt.Sprintf("client_networks[%d]", idx)
		if !netw.Enabled {
			continue
		}

		_, _, err := net.ParseCIDR(netw.CIDR)
		if err != nil {
			errs = append(errs, ValidationError{Field: prefix + ".cidr", Message: fmt.Sprintf("invalid CIDR '%s': %v", netw.CIDR, err)})
		}

		// Open Resolver Guard: 0.0.0.0/0 allow is strictly forbidden
		if (netw.CIDR == "0.0.0.0/0" || netw.CIDR == "::/0") && netw.Action == "allow" {
			errs = append(errs, ValidationError{
				Field:   prefix + ".cidr",
				Message: "CRITICAL SECURITY VIOLATION: '0.0.0.0/0' or '::/0' cannot be configured with 'allow' action (creates an open resolver)",
			})
		}

		validActions := map[string]bool{"allow": true, "deny": true, "refuse": true, "allow_snoop": true}
		if !validActions[netw.Action] {
			errs = append(errs, ValidationError{
				Field:   prefix + ".action",
				Message: fmt.Sprintf("invalid ACL action '%s' (expected allow, deny, refuse, or allow_snoop)", netw.Action),
			})
		}
	}

	// 3. Validate Resolution Mode & Upstreams
	switch state.ResolutionMode {
	case ModeRecursive:
		// Upstreams optional, but DNSSEC should be configured
	case ModeForwarding, ModeHybrid:
		activeUpstreams := 0
		for idx, up := range state.Upstreams {
			prefix := fmt.Sprintf("upstreams[%d]", idx)
			if !up.Enabled {
				continue
			}
			activeUpstreams++

			ip := net.ParseIP(up.IP)
			if ip == nil {
				errs = append(errs, ValidationError{Field: prefix + ".ip", Message: fmt.Sprintf("invalid upstream IP: %s", up.IP)})
			}

			if up.Port <= 0 || up.Port > 65535 {
				errs = append(errs, ValidationError{Field: prefix + ".port", Message: fmt.Sprintf("invalid upstream port: %d", up.Port)})
			}
		}

		if activeUpstreams == 0 {
			errs = append(errs, ValidationError{
				Field:   "upstreams",
				Message: fmt.Sprintf("resolution mode '%s' requires at least one active upstream server", state.ResolutionMode),
			})
		}
	default:
		errs = append(errs, ValidationError{
			Field:   "resolution_mode",
			Message: fmt.Sprintf("unknown resolution mode '%s'", state.ResolutionMode),
		})
	}

	// 4. Validate Remote Control
	if state.RemoteControl.Enabled {
		if state.RemoteControl.Interface == "0.0.0.0" {
			errs = append(errs, ValidationError{
				Field:   "remote_control.interface",
				Message: "CRITICAL SECURITY VIOLATION: remote-control interface must NOT be 0.0.0.0 (must be 127.0.0.1 or protected loopback)",
			})
		}
	}

	// 5. Validate DNSSEC Insecure Domains
	for idx, domain := range state.DNSSEC.InsecureDomains {
		cleanDomain := strings.TrimSpace(domain)
		if cleanDomain == "" {
			continue
		}
		// ensure domain is well-formed
		if !domainRegex.MatchString(cleanDomain) && !strings.Contains(cleanDomain, ".") {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("dnssec.insecure_domains[%d]", idx),
				Message: fmt.Sprintf("invalid domain syntax: %s", cleanDomain),
			})
		}
	}

	// 6. Validate Local Records (DNS Rewrites)
	for idx, rec := range state.LocalRecords {
		prefix := fmt.Sprintf("local_records[%d]", idx)
		if rec.Domain == "" {
			errs = append(errs, ValidationError{Field: prefix + ".domain", Message: "local record domain cannot be empty"})
		}
		// Allow wildcard prefix "*.domain"; validate the bare domain portion.
		bareDomain := strings.TrimPrefix(rec.Domain, "*.")
		if rec.Domain != "" && bareDomain == "" {
			errs = append(errs, ValidationError{Field: prefix + ".domain", Message: "wildcard domain must have a base name after '*.' (e.g. *.example.com)"})
		}
		validTypes := map[string]bool{"A": true, "AAAA": true, "CNAME": true, "PTR": true, "TXT": true}
		if !validTypes[rec.Type] {
			errs = append(errs, ValidationError{Field: prefix + ".type", Message: fmt.Sprintf("unsupported record type: %s", rec.Type)})
		}
		// PTR + wildcard is semantically invalid in Unbound.
		if rec.Type == "PTR" && strings.HasPrefix(rec.Domain, "*.") {
			errs = append(errs, ValidationError{Field: prefix + ".type", Message: "PTR records cannot be combined with a wildcard (*.) domain"})
		}
		if rec.Value == "" {
			errs = append(errs, ValidationError{Field: prefix + ".value", Message: "local record value cannot be empty"})
		}
	}

	// 7. Validate Anti-DDoS Rate Limiting
	if state.RateLimit.Enabled {
		if state.RateLimit.IPRateLimit < 0 {
			errs = append(errs, ValidationError{Field: "rate_limit.ip_rate_limit", Message: "IP rate limit cannot be negative"})
		}
		if state.RateLimit.DomainRateLimit < 0 {
			errs = append(errs, ValidationError{Field: "rate_limit.domain_rate_limit", Message: "domain rate limit cannot be negative"})
		}
		if state.RateLimit.IPRateLimitFactor < 0 {
			errs = append(errs, ValidationError{Field: "rate_limit.ip_rate_limit_factor", Message: "rate limit factor cannot be negative"})
		}
	}

	return errs
}
