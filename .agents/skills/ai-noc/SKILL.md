# Skill: AI-NOC Integration

Hermes/AI agents are consumers of diagnostics, not dependencies of DNS.

Provide safe, read-only diagnostic APIs first:
- node health
- Unbound stats
- upstream health
- domain resolution diagnosis
- client/network policy explanation
- recent config changes

Write actions must require explicit authorization and audit logging.

Example diagnostic:
"Why is example.com failing?"

Return:
- client policy
- allow/block result
- cache status
- DNSSEC result
- upstream result
- latency
- error class
- suggested next diagnostic
