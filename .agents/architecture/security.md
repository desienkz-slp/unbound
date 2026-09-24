# Security Architecture

Threats:
- accidental open resolver
- malicious admin
- compromised UI
- config injection
- malicious filter feed
- DNS cache poisoning
- DNSSEC failures
- IPv6 ACL bypass
- leaked control socket
- unsafe AI automation

Controls:
- strict ACL defaults
- input validation
- config validation
- signed/versioned configuration where practical
- RBAC
- audit logs
- secret management
- isolated management plane
- no arbitrary shell via API/MCP
- allowlisted write actions
- rollback
- security tests
