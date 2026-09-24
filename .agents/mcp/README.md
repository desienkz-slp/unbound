# MCP Integration

MCP is optional.

Recommended MCP tools for an AI/NOC agent:

Read-only:
- get_dns_nodes
- get_node_health
- get_unbound_stats
- get_upstream_health
- resolve_domain
- diagnose_domain
- explain_policy
- get_client_networks
- get_active_config
- get_config_history
- get_alerts

Controlled write:
- validate_config
- stage_config
- apply_config
- rollback_config
- reload_unbound
- update_filter_list

Security:
- write tools must be explicitly permissioned
- all writes must be audited
- MCP must not expose shell execution by default
- do not expose arbitrary `unbound-control` commands; wrap safe operations

Suggested transport:
- local stdio for development
- authenticated HTTP/SSE or the current MCP transport supported by the agent environment for remote deployment

The MCP server should call the Netora API rather than manipulating Unbound files directly.
