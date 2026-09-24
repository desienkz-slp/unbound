# MCP Tool Contracts

## get_node_health
Input:
{ "node_id": "string" }

Output:
node status, process status, listeners, version, uptime, CPU, memory.

## get_unbound_stats
Input:
{ "node_id": "string" }

Output:
QPS, cache hit/miss, latency, SERVFAIL, NXDOMAIN, requestlist, DNSSEC failures.

## resolve_domain
Input:
{ "node_id": "string", "domain": "string", "qtype": "A|AAAA|CNAME|MX|TXT|NS" }

Output:
rcode, answer, DNSSEC state, latency, cache status.

## diagnose_domain
Input:
{ "node_id": "string", "domain": "string", "client_ip": "optional string" }

Output:
policy decision, block source, DNSSEC, upstream, cache, latency, error class, evidence.

## apply_config
Input:
{ "node_id": "string", "config_version": "string", "reason": "string" }

Output:
validation, apply result, health result, rollback availability.

## rollback_config
Input:
{ "node_id": "string", "config_version": "string", "reason": "string" }

Output:
rollback result and post-rollback health.
