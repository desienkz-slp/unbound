# Observability Architecture

Prometheus:
- node health
- Unbound statistics
- API latency/errors
- controller reconciliation
- filter update success/failure

Loki:
- API logs
- controller logs
- Unbound operational logs
- audit events where appropriate

Grafana dashboards:
1. Executive DNS health
2. NOC real-time
3. Resolver performance
4. Upstream health
5. Filtering
6. Node resources
7. Config changes

Alerts:
- DNS node down
- SERVFAIL spike
- timeout spike
- cache hit collapse
- upstream degradation
- disk exhaustion
- memory pressure
- config apply failure
- filter update failure
