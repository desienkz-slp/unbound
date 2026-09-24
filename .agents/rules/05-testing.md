# Testing Rules

Required test categories:

1. Unit
2. API
3. Configuration generation
4. Unbound syntax validation
5. Integration with real Unbound in containers/VM where possible
6. IPv4
7. IPv6
8. ACL isolation
9. DNSSEC
10. Filtering/allowlist precedence
11. rollback
12. upstream failure
13. stale/expired cache behavior
14. UI smoke tests
15. load tests

Failure tests are mandatory.

Minimum load-test progression:
- 1K QPS
- 10K QPS
- 50K QPS
- 100K QPS
- higher only after the previous stage is stable

Do not call the system "ISP-scale" based on configuration alone. Record benchmark hardware, query mix, cache hit ratio, latency, and error rate.
