package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
)

//go:embed instructions.md
var serverInstructions string

// MCPServer handles the MCP protocol over stdio.
// Register tools with AddTool, then call Serve.
// Same idea as Store — set it up once, never think about
// the protocol again when adding new commands.
type MCPServer struct {
	name    string
	version string
	tools   []Tool
}

// Tool is a single MCP tool. Handler receives the arguments
// and returns a text result or an error.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Handler     func(params map[string]interface{}) (string, error)
}

// JSON-RPC 2.0 types — the wire format MCP uses.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewMCPServer(name, version string) *MCPServer {
	return &MCPServer{name: name, version: version}
}

func (s *MCPServer) AddTool(t Tool) {
	s.tools = append(s.tools, t)
}

// Serve reads JSON-RPC messages from stdin, dispatches them,
// and writes responses to stdout. One JSON object per line.
func (s *MCPServer) Serve() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var req jsonRPCRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		// Notifications (no id) don't get a response
		if req.ID == nil {
			continue
		}

		resp := s.handle(req)
		out, _ := json.Marshal(resp)
		fmt.Fprintf(os.Stdout, "%s\n", out)
	}
	return scanner.Err()
}

func (s *MCPServer) handle(req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    s.name,
					"version": s.version,
				},
				"instructions": serverInstructions,
			},
		}

	case "tools/list":
		tools := make([]map[string]interface{}, len(s.tools))
		for i, t := range s.tools {
			tools[i] = map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			}
		}
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"tools": tools},
		}

	case "tools/call":
		return s.handleToolCall(req)

	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *MCPServer) handleToolCall(req jsonRPCRequest) jsonRPCResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "invalid params"},
		}
	}

	for _, t := range s.tools {
		if t.Name == params.Name {
			result, err := t.Handler(params.Arguments)
			if err != nil {
				return jsonRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
						},
						"isError": true,
					},
				}
			}
			return jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": result},
					},
				},
			}
		}
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error:   &rpcError{Code: -32602, Message: fmt.Sprintf("unknown tool: %s", params.Name)},
	}
}
