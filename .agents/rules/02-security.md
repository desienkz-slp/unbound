# Security Rules

- Default deny for client networks.
- Never create an open resolver.
- Separate management listener from DNS listener.
- Restrict `unbound-control` to loopback or protected control network.
- Use TLS for remote control where applicable.
- Use least privilege for services.
- Never store passwords/API keys in Git.
- Validate CIDRs, IPs, ports, domains, and rule syntax server-side.
- Protect configuration endpoints with RBAC.
- Audit security-sensitive actions.
- Rate-limit administrative APIs.
- Do not use blocklists as a substitute for firewall security.
- Do not disable DNSSEC globally to solve one broken domain.
- Make IPv6 security equivalent to IPv4 security.
