# Benchmark Plan

Do not benchmark with only one synthetic query.

Use a mixed query set:
- cache hits
- cache misses
- A
- AAAA
- CNAME
- DNSSEC domains
- NXDOMAIN
- blocked domains
- allowed domains

Stages:
1K -> 10K -> 50K -> 100K -> 250K -> 500K -> 1M QPS when hardware permits.

For every stage capture:
- achieved QPS
- P50
- P95
- P99
- SERVFAIL
- timeout
- CPU
- RSS
- cache hit ratio
- packet loss

Run both:
- warm cache
- cold cache

Record exact hardware and Unbound version.
