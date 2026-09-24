# Netora DNS Product Requirements

## Primary goals
1. Fast DNS resolution
2. High availability
3. Low SERVFAIL/timeout caused by resolver-side failures
4. Safe filtering of ads/malware/phishing and optional categories
5. Simple setup
6. ISP-grade operations
7. Explainable DNS decisions
8. AI/NOC integration without AI dependency

## Listener roles

### Internet
Optional public listener. Must be ACL protected and enabled only intentionally.

### Client
Customer-facing DNS IPs. Supports IPv4/IPv6 and network assignment.

### Management
Private/loopback. Used by API/controller and operational tooling.

## Policy model

A policy profile can define:
- filtering categories
- allowlist
- blocklist sources
- local records
- SafeSearch
- rewrites
- schedules

## Availability

A node failure must not stop service when clients have multiple DNS endpoints or when the deployment provides routing/anycast failover.

## Explainability

For a domain query, operators should be able to see:
- allowed/blocked
- matching rule/list
- policy
- DNSSEC result
- cache state
- upstream
- latency
- error
