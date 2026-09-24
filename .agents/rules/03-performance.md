# Performance Rules

Target high throughput, low tail latency, and predictable memory usage.

Measure:
- QPS
- cache hit ratio
- cache miss ratio
- P50/P95/P99 latency
- SERVFAIL
- timeout
- NXDOMAIN
- recursive replies
- requestlist depth
- UDP/TCP usage
- DNSSEC failures
- memory
- CPU
- file descriptors

Guidelines:
- Tune `num-threads` based on CPU topology and workload.
- Use `so-reuseport` where supported and tested.
- Size caches from measured workload, not arbitrary RAM ratios alone.
- Avoid synchronous database writes on the DNS request path.
- Prefer in-memory runtime policy structures.
- Treat blocklist scale as a benchmarked workload.
- Benchmark before and after tuning.
