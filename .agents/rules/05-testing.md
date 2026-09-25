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
16. Real-world CDN and streaming validation via Browser Subagent (e-commerce, video, heavy media)

Failure tests are mandatory.

Browser Subagent Testing Rules:
- When validating DNS resolver health or quality changes, execute end-to-end visual tests with a browser subagent against real multi-CDN websites (e.g. Shopee, e-commerce, video streaming).
- Verify that dynamic banners, category icons, lazy-loaded product thumbnails, and live video players render without broken placeholders or network timeouts.
- Set strict timeout and step limits (max 8-10 steps) with measurable stop conditions to prevent hanging.

Minimum load-test progression:
- 1K QPS
- 10K QPS
- 50K QPS
- 100K QPS
- higher only after the previous stage is stable

Do not call the system "ISP-scale" based on configuration alone. Record benchmark hardware, query mix, cache hit ratio, latency, and error rate.
