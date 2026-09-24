# Data Plane

Unbound is the critical path.

Listeners:
- management: loopback/private management
- client DNS: private/customer-facing addresses
- internet-facing DNS: only when explicitly configured and ACL-protected

Resolution modes:
- recursive
- forwarding
- hybrid/fallback

Performance:
- multithreading
- reuseport where supported
- cache
- prefetch
- serve-expired
- negative caching
- efficient access-control
- minimal synchronous logging

Security:
- DNSSEC
- QNAME minimization where compatible
- anti-rebinding policy
- strict ACLs
- no accidental open resolver
