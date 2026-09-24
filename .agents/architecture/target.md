# Target Architecture

                    Internet / Transit
                           |
                     Edge / Routing
                           |
              +------------+------------+
              |                         |
        DNS Node A                 DNS Node B
        Unbound                   Unbound
              |                         |
              +------------+------------+
                           |
                  Client DNS Networks

Each node:
- multi-IP
- IPv4 + IPv6
- client ACLs
- DNSSEC
- cache/prefetch/serve-expired
- filtering
- local records
- private control socket
- metrics

Control plane:
UI -> API -> PostgreSQL desired state -> Config Engine -> node agents/API -> Unbound

Telemetry:
Unbound/exporter -> Prometheus
service logs -> Loki
AI/NOC -> Netora API or MCP
