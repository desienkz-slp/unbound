package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// JSON-RPC 2.0 Structures
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool Definition
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolCallResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type MCPServer struct {
	apiBase   string
	authToken string
	client    *http.Client
}

func NewMCPServer(apiBase, authToken string) *MCPServer {
	return &MCPServer{
		apiBase:   strings.TrimRight(apiBase, "/"),
		authToken: authToken,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *MCPServer) apiRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	url := fmt.Sprintf("%s%s", s.apiBase, path)
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.authToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

func (s *MCPServer) getTools() []ToolDefinition {
	return []ToolDefinition{
		// 1. get_dns_nodes
		{
			Name:        "get_dns_nodes",
			Description: "Lists all managed DNS nodes in the cluster with status, management IP, and version.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		// 2. get_node_health
		{
			Name:        "get_node_health",
			Description: "Inspects live health, Unbound process state, and port 53 responsiveness for a node.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"node_id": map[string]string{"type": "string", "description": "Target node ID (e.g. dns-dev)"},
				},
				"required": []string{"node_id"},
			},
		},
		// 3. get_unbound_stats
		{
			Name:        "get_unbound_stats",
			Description: "Retrieves runtime QPS, cache hit ratio, SERVFAIL, and latency metrics.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		// 4. get_upstream_health
		{
			Name:        "get_upstream_health",
			Description: "Returns configured upstream resolvers and latency RTTs.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"node_id": map[string]string{"type": "string", "description": "Target node ID"},
				},
				"required": []string{"node_id"},
			},
		},
		// 5. resolve_domain
		{
			Name:        "resolve_domain",
			Description: "Resolves a DNS domain directly against the local Unbound data plane.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"domain": map[string]string{"type": "string", "description": "Domain name to resolve"},
					"qtype":  map[string]string{"type": "string", "description": "Query record type (A, AAAA, TXT)"},
				},
				"required": []string{"domain"},
			},
		},
		// 6. diagnose_domain
		{
			Name:        "diagnose_domain",
			Description: "Explains 'Why Blocked?' with rule attribution, policy hierarchy, and live resolution latency.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"domain":    map[string]string{"type": "string", "description": "Domain to inspect"},
					"client_ip": map[string]string{"type": "string", "description": "Simulated client IP"},
					"policy":    map[string]string{"type": "string", "description": "Policy profile (DEFAULT, FAMILY)"},
				},
				"required": []string{"domain"},
			},
		},
		// 7. explain_policy
		{
			Name:        "explain_policy",
			Description: "Explains policy precedence and category filter rules.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"policy": map[string]string{"type": "string", "description": "Policy name"},
				},
			},
		},
		// 8. get_client_networks
		{
			Name:        "get_client_networks",
			Description: "Retrieves client CIDR access control lists.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"node_id": map[string]string{"type": "string", "description": "Target node ID"},
				},
				"required": []string{"node_id"},
			},
		},
		// 9. get_active_config
		{
			Name:        "get_active_config",
			Description: "Returns active desired state configuration for a node.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"node_id": map[string]string{"type": "string", "description": "Target node ID"},
				},
				"required": []string{"node_id"},
			},
		},
		// 10. get_config_history
		{
			Name:        "get_config_history",
			Description: "Lists configuration versions, checksums, and rollback targets.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"node_id": map[string]string{"type": "string", "description": "Target node ID"},
				},
				"required": []string{"node_id"},
			},
		},
		// 11. get_alerts
		{
			Name:        "get_alerts",
			Description: "Returns active operational alerts (Node down, Servfail spike, Cache collapse).",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		// Controlled Write: rollback_config
		{
			Name:        "rollback_config",
			Description: "Atomically restores Unbound to a previous known-good configuration version.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"node_id":    map[string]string{"type": "string", "description": "Target node ID"},
					"version_id": map[string]string{"type": "string", "description": "Target version ID to restore"},
					"reason":     map[string]string{"type": "string", "description": "Reason for rollback"},
				},
				"required": []string{"node_id", "version_id"},
			},
		},
		// Hermes AI-NOC Diagnostic
		{
			Name:        "hermes_investigate_issue",
			Description: "Executes deep 6-stage AI-NOC investigation of domain access problems with verifiable evidence.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"domain":       map[string]string{"type": "string", "description": "Target domain to investigate (e.g. whatsapp.com, ads.foo.com)"},
					"client_ip":    map[string]string{"type": "string", "description": "Client IP experiencing the issue"},
					"policy_group": map[string]string{"type": "string", "description": "Policy profile (DEFAULT, FAMILY, etc.)"},
				},
				"required": []string{"domain"},
			},
		},
	}
}

func (s *MCPServer) executeTool(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
	nodeID := "dns-dev"
	if nid, ok := args["node_id"].(string); ok && nid != "" {
		nodeID = nid
	}

	switch name {
	case "get_dns_nodes":
		data, err := s.apiRequest("GET", "/api/v1/nodes", nil)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "get_node_health":
		data, err := s.apiRequest("GET", fmt.Sprintf("/api/v1/nodes/%s/health", nodeID), nil)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "get_unbound_stats":
		data, err := s.apiRequest("GET", "/metrics", nil)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "get_upstream_health":
		data, err := s.apiRequest("GET", fmt.Sprintf("/api/v1/nodes/%s/config", nodeID), nil)
		if err != nil {
			return nil, err
		}
		var cfg map[string]interface{}
		_ = json.Unmarshal(data, &cfg)
		upstreams, _ := json.Marshal(cfg["upstreams"])
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(upstreams)}}}, nil

	case "resolve_domain":
		domain, _ := args["domain"].(string)
		data, err := s.apiRequest("POST", "/api/v1/diagnose", map[string]string{"domain": domain})
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "diagnose_domain":
		domain, _ := args["domain"].(string)
		clientIP, _ := args["client_ip"].(string)
		policy, _ := args["policy"].(string)
		body := map[string]string{"domain": domain, "client_ip": clientIP, "policy": policy}
		data, err := s.apiRequest("POST", "/api/v1/diagnose", body)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "explain_policy":
		explanation := map[string]interface{}{
			"precedence": []string{
				"1. Emergency Override",
				"2. Explicit Allowlist",
				"3. Custom Rewrites",
				"4. Category Policy",
				"5. External Blocklists",
				"6. Normal DNS Resolution",
			},
			"categories_supported": []string{"ADS", "TRACKERS", "MALWARE", "PHISHING", "BOTNET", "ADULT", "GAMBLING"},
		}
		b, _ := json.Marshal(explanation)
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(b)}}}, nil

	case "get_client_networks":
		data, err := s.apiRequest("GET", fmt.Sprintf("/api/v1/nodes/%s/config", nodeID), nil)
		if err != nil {
			return nil, err
		}
		var cfg map[string]interface{}
		_ = json.Unmarshal(data, &cfg)
		networks, _ := json.Marshal(cfg["client_networks"])
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(networks)}}}, nil

	case "get_active_config":
		data, err := s.apiRequest("GET", fmt.Sprintf("/api/v1/nodes/%s/config", nodeID), nil)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "get_config_history":
		data, err := s.apiRequest("GET", fmt.Sprintf("/api/v1/nodes/%s/config/versions", nodeID), nil)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "get_alerts":
		data, err := s.apiRequest("GET", "/api/v1/alerts", nil)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "rollback_config":
		versionID, _ := args["version_id"].(string)
		data, err := s.apiRequest("POST", fmt.Sprintf("/api/v1/nodes/%s/config/rollback", nodeID), map[string]string{
			"version_id": versionID,
		})
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	case "hermes_investigate_issue":
		domain, _ := args["domain"].(string)
		clientIP, _ := args["client_ip"].(string)
		policyGroup, _ := args["policy_group"].(string)
		body := map[string]string{
			"domain":       domain,
			"client_ip":    clientIP,
			"policy_group": policyGroup,
		}
		data, err := s.apiRequest("POST", "/api/v1/hermes/investigate", body)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []TextContent{{Type: "text", Text: string(data)}}}, nil

	default:
		return nil, fmt.Errorf("unknown MCP tool: %s", name)
	}
}

func main() {
	apiBase := flag.String("api-base", "http://127.0.0.1:8080", "Netora API base URL")
	authToken := flag.String("token", "", "Bearer token for Netora API authentication")
	flag.Parse()

	server := NewMCPServer(*apiBase, *authToken)
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32700, Message: "Parse error"},
			}
			out, _ := json.Marshal(resp)
			fmt.Println(string(out))
			continue
		}

		switch req.Method {
		case "initialize":
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"serverInfo": map[string]string{
						"name":    "netora-mcp",
						"version": "1.0.0",
					},
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
				},
			}
			out, _ := json.Marshal(resp)
			fmt.Println(string(out))

		case "notifications/initialized":
			// Acknowledge notification; no response required

		case "tools/list":
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"tools": server.getTools(),
				},
			}
			out, _ := json.Marshal(resp)
			fmt.Println(string(out))

		case "tools/call":
			var params ToolCallParams
			if err := json.Unmarshal(req.Params, &params); err != nil {
				resp := JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &JSONRPCError{Code: -32602, Message: "Invalid tool params"},
				}
				out, _ := json.Marshal(resp)
				fmt.Println(string(out))
				continue
			}

			result, err := server.executeTool(context.Background(), params.Name, params.Arguments)
			if err != nil {
				resp := JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: &ToolCallResult{
						Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
						IsError: true,
					},
				}
				out, _ := json.Marshal(resp)
				fmt.Println(string(out))
				continue
			}

			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}
			out, _ := json.Marshal(resp)
			fmt.Println(string(out))

		default:
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
			}
			out, _ := json.Marshal(resp)
			fmt.Println(string(out))
		}
	}
}
