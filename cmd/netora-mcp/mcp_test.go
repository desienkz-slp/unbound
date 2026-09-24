package main

import (
	"context"
	"testing"
)

func TestMCPToolsList(t *testing.T) {
	server := NewMCPServer("http://127.0.0.1:8080", "mock-token")
	tools := server.getTools()

	if len(tools) < 10 {
		t.Fatalf("expected at least 10 MCP tools, got %d", len(tools))
	}

	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	requiredTools := []string{
		"get_dns_nodes",
		"get_node_health",
		"get_unbound_stats",
		"get_upstream_health",
		"resolve_domain",
		"diagnose_domain",
		"explain_policy",
		"get_client_networks",
		"get_active_config",
		"get_config_history",
		"get_alerts",
		"rollback_config",
	}

	for _, req := range requiredTools {
		if !toolMap[req] {
			t.Errorf("missing required MCP tool: %s", req)
		}
	}
}

func TestMCPExecuteExplainPolicy(t *testing.T) {
	server := NewMCPServer("http://127.0.0.1:8080", "mock-token")
	result, err := server.executeTool(context.Background(), "explain_policy", map[string]interface{}{})
	if err != nil {
		t.Fatalf("explain_policy returned error: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatalf("expected content in result, got empty")
	}

	text := result.Content[0].Text
	if len(text) == 0 {
		t.Errorf("expected non-empty explanation text")
	}
}

func TestMCPUnknownTool(t *testing.T) {
	server := NewMCPServer("http://127.0.0.1:8080", "mock-token")
	_, err := server.executeTool(context.Background(), "execute_shell", map[string]interface{}{"command": "rm -rf /"})
	if err == nil {
		t.Fatalf("expected error for unauthorized/unknown tool execute_shell")
	}
}
