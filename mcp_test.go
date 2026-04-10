package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMCPAddAndGetTasks(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	resp := mcpCall(mcp, "addtask", map[string]interface{}{"name": "MCP Task"})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}

	resp = mcpCall(mcp, "gettask", map[string]interface{}{"all": true})
	if !strings.HasPrefix(mcpText(resp), "MCP Task") {
		t.Fatalf("got: %s", mcpText(resp))
	}
}

func TestMCPAddAndGetRules(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addrule", map[string]interface{}{"name": "MCP Rule"})
	resp := mcpCall(mcp, "getrule", map[string]interface{}{"all": true})
	if mcpText(resp) != "Rule 1: MCP Rule" {
		t.Fatalf("got: %s", mcpText(resp))
	}
}

func TestMCPEmptyLists(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	if mcpText(mcpCall(mcp, "gettask", map[string]interface{}{"all": true})) != "No tasks found." {
		t.Fatal("expected empty tasks")
	}
	if mcpText(mcpCall(mcp, "getrule", map[string]interface{}{"all": true})) != "No rules found." {
		t.Fatal("expected empty rules")
	}
}

func TestMCPUnknownTool(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	resp := mcpCall(mcp, "nonexistent", map[string]interface{}{})
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestMCPSetTask(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T"})
	resp := mcpCall(mcp, "settask", map[string]interface{}{"task": "T", "status": "active"})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}

	resp = mcpCall(mcp, "gettask", map[string]interface{}{"all": true})
	if !hasStatus(mcpText(resp), "active") {
		t.Fatalf("got: %s", mcpText(resp))
	}
}

func TestMCPSetTaskWait(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "s1", "under": "T"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "s2", "under": "T"})
	mcpCall(mcp, "settask", map[string]interface{}{"task": "T:1", "status": "active"})
	mcpCall(mcp, "settask", map[string]interface{}{"task": "T:2", "status": "active"})

	resp := mcpCall(mcp, "settask", map[string]interface{}{
		"task": "T:2", "status": "wait", "blockers": []interface{}{"T:1"},
	})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}

	resp = mcpCall(mcp, "gettask", map[string]interface{}{"all": true})
	if !strings.Contains(mcpText(resp), "[waiting: 1 ") {
		t.Fatalf("got: %s", mcpText(resp))
	}
}

func TestMCPDeleteTask(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "s1", "under": "T"})

	resp := mcpCall(mcp, "deletetask", map[string]interface{}{"task": "T"})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}
	text := mcpText(resp)
	if !strings.Contains(text, "Deleted") || !strings.Contains(text, "1 children") {
		t.Fatalf("unexpected response: %s", text)
	}

	resp = mcpCall(mcp, "gettask", map[string]interface{}{"all": true})
	if mcpText(resp) != "No tasks found." {
		t.Fatalf("expected no tasks after delete: %s", mcpText(resp))
	}
}

func TestMCPArchiveTask(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T"})
	mcpCall(mcp, "settask", map[string]interface{}{"task": "T", "status": "archived"})

	// Should be hidden
	resp := mcpCall(mcp, "gettask", map[string]interface{}{"all": true})
	if mcpText(resp) != "No tasks found." {
		t.Fatalf("archived should be hidden: %s", mcpText(resp))
	}

	// Should show with status filter
	resp = mcpCall(mcp, "gettask", map[string]interface{}{"all": true, "status": "archived"})
	if !hasStatus(mcpText(resp), "archived") {
		t.Fatalf("expected archived task: %s", mcpText(resp))
	}
}

func TestAllCommandsHaveMCPTools(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	toolNames := make(map[string]bool)
	for _, tool := range mcp.tools {
		toolNames[tool.Name] = true
	}

	for _, cmd := range CLICommands {
		if !toolNames[cmd] {
			t.Errorf("CLI command '%s' has no matching MCP tool", cmd)
		}
	}

	for _, tool := range mcp.tools {
		found := false
		for _, cmd := range CLICommands {
			if cmd == tool.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("MCP tool '%s' has no matching CLI command", tool.Name)
		}
	}
}

func TestCLIAndMCPShareStore(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	AddTask(store, "CLI Task", "", false, false)
	resp := mcpCall(mcp, "gettask", map[string]interface{}{"all": true})
	if !strings.HasPrefix(mcpText(resp), "CLI Task") {
		t.Fatalf("MCP didn't see CLI write: %s", mcpText(resp))
	}

	mcpCall(mcp, "addtask", map[string]interface{}{"name": "MCP Task"})
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 2 || !strings.HasPrefix(tasks[1], "MCP Task") {
		t.Fatalf("CLI didn't see MCP write: %v", tasks)
	}
}

func TestMCPGetTagSingle(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "BACKEND"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T"})
	mcpCall(mcp, "settask", map[string]interface{}{"task": "T", "tags": []interface{}{"BACKEND"}})

	resp := mcpCall(mcp, "gettag", map[string]interface{}{"tag": "BACKEND"})
	text := mcpText(resp)
	if !strings.Contains(text, "BACKEND") || !strings.Contains(text, "Tasks") {
		t.Fatalf("expected tag details: %s", text)
	}
}

func TestMCPSetTag(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "OLD"})
	resp := mcpCall(mcp, "settag", map[string]interface{}{"old": "OLD", "new": "NEW"})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}

	resp = mcpCall(mcp, "gettag", map[string]interface{}{"all": true})
	if !strings.Contains(mcpText(resp), "NEW") {
		t.Fatalf("expected renamed tag: %s", mcpText(resp))
	}
}

func TestMCPDeleteTag(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "DOOMED"})
	resp := mcpCall(mcp, "deletetag", map[string]interface{}{"tag": "DOOMED"})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}

	resp = mcpCall(mcp, "gettag", map[string]interface{}{"all": true})
	if mcpText(resp) != "No tags found." {
		t.Fatalf("expected no tags: %s", mcpText(resp))
	}
}

func TestMCPDeleteRule(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addrule", map[string]interface{}{"name": "R1"})
	mcpCall(mcp, "addrule", map[string]interface{}{"name": "R2"})

	resp := mcpCall(mcp, "deleterule", map[string]interface{}{"seq": float64(1)})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}

	resp = mcpCall(mcp, "getrule", map[string]interface{}{"all": true})
	text := mcpText(resp)
	if strings.Contains(text, "R1") {
		t.Fatalf("deleted rule should be hidden: %s", text)
	}
	if !strings.Contains(text, "R2") {
		t.Fatalf("R2 should still be visible: %s", text)
	}
}

func TestMCPSetRule(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addrule", map[string]interface{}{"name": "Original rule"})

	// Update name via MCP (seq comes as float64 from JSON)
	resp := mcpCall(mcp, "setrule", map[string]interface{}{
		"seq":  float64(1),
		"name": "Updated rule",
	})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}

	resp = mcpCall(mcp, "getrule", map[string]interface{}{"all": true})
	text := mcpText(resp)
	if !strings.Contains(text, "Updated rule") {
		t.Fatalf("expected updated name: %s", text)
	}
	if strings.Contains(text, "Original rule") {
		t.Fatalf("old name should be gone: %s", text)
	}

	AddTag(store, "BACKEND")
	AddTag(store, "FRONTEND")
	resp = mcpCall(mcp, "setrule", map[string]interface{}{
		"seq":  float64(1),
		"tags": []interface{}{"BACKEND", "FRONTEND"},
	})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}

	resp = mcpCall(mcp, "getrule", map[string]interface{}{"all": true})
	text = mcpText(resp)
	if !strings.Contains(text, "#BACKEND") || !strings.Contains(text, "#FRONTEND") {
		t.Fatalf("expected tags on rule: %s", text)
	}
}

func TestMCPDeleteRuleAtomic(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addrule", map[string]interface{}{"name": "To delete"})
	resp := mcpCall(mcp, "deleterule", map[string]interface{}{"seq": float64(1)})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}
	text := mcpText(resp)
	if !strings.Contains(text, "deleted") {
		t.Fatalf("expected deleted message, got: %s", text)
	}

	// Verify it's gone
	resp = mcpCall(mcp, "getrule", map[string]interface{}{"all": true})
	if mcpText(resp) != "No rules found." {
		t.Fatalf("expected no rules, got: %s", mcpText(resp))
	}
}

func TestMCPInitialize(t *testing.T) {
	mcp := NewMCPServer("butler", "1.0.0")
	resp := mcp.handle(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
	})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}
	result := resp.Result.(map[string]interface{})
	info := result["serverInfo"].(map[string]interface{})
	if info["name"] != "butler" || info["version"] != "1.0.0" {
		t.Fatalf("unexpected serverInfo: %v", info)
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Fatalf("unexpected protocol version: %v", result["protocolVersion"])
	}
	if result["instructions"] == nil || result["instructions"] == "" {
		t.Fatal("expected non-empty instructions")
	}
}

func TestMCPToolsList(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	resp := mcp.handle(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	})
	if resp.Error != nil {
		t.Fatal(resp.Error.Message)
	}
	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]map[string]interface{})
	if len(tools) != len(CLICommands) {
		t.Fatalf("expected %d tools, got %d", len(CLICommands), len(tools))
	}
	// Verify each tool has required fields
	for _, tool := range tools {
		if tool["name"] == nil || tool["name"] == "" {
			t.Fatal("tool missing name")
		}
		if tool["description"] == nil || tool["description"] == "" {
			t.Fatalf("tool %s missing description", tool["name"])
		}
		if tool["inputSchema"] == nil {
			t.Fatalf("tool %s missing inputSchema", tool["name"])
		}
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	mcp := NewMCPServer("butler", "1.0.0")
	resp := mcp.handle(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "nonexistent/method",
	})
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPInvalidToolCallParams(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	resp := mcp.handle(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  json.RawMessage(`invalid json`),
	})
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("expected code -32602, got %d", resp.Error.Code)
	}
}

func TestMCPGettaskRequiresAllOrTask(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	resp := mcpCall(mcp, "gettask", map[string]interface{}{})
	text := mcpText(resp)
	if !strings.Contains(text, "Error") || !strings.Contains(text, "'all: true'") {
		t.Fatalf("expected error about requiring all or task, got: %s", text)
	}
}

func TestMCPGetruleRequiresParam(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	resp := mcpCall(mcp, "getrule", map[string]interface{}{})
	text := mcpText(resp)
	if !strings.Contains(text, "Error") {
		t.Fatalf("expected error about requiring param, got: %s", text)
	}
}


func TestMCPServe(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	// Pipe two JSON-RPC requests into stdin
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	notif := `{"jsonrpc":"2.0","method":"notifications/initialized"}` // no id = notification
	input := initReq + "\n" + notif + "\n" + listReq + "\n"

	oldStdin := os.Stdin
	oldStdout := os.Stdout

	stdinR, stdinW, _ := os.Pipe()
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdin = stdinR
	os.Stdout = stdoutW

	go func() {
		stdinW.Write([]byte(input))
		stdinW.Close()
	}()

	err := mcp.Serve()
	stdoutW.Close()
	os.Stdin = oldStdin
	os.Stdout = oldStdout

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, stdoutR)
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have exactly 2 responses (notification has no response)
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %s", len(lines), output)
	}

	// First response should be initialize
	var resp1 jsonRPCResponse
	json.Unmarshal([]byte(lines[0]), &resp1)
	if resp1.Error != nil {
		t.Fatalf("initialize failed: %s", resp1.Error.Message)
	}

	// Second response should be tools/list
	var resp2 jsonRPCResponse
	json.Unmarshal([]byte(lines[1]), &resp2)
	if resp2.Error != nil {
		t.Fatalf("tools/list failed: %s", resp2.Error.Message)
	}
}

func TestMCPSetTaskRenameShowsNewName(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtask", map[string]interface{}{"name": "Old name"})
	resp := mcpCall(mcp, "settask", map[string]interface{}{"task": "Old name", "name": "New name"})
	text := mcpText(resp)
	if !strings.Contains(text, "New name") {
		t.Fatalf("expected new name in output, got: %s", text)
	}
	if strings.Contains(text, "Old name") {
		t.Fatalf("expected old name absent from output, got: %s", text)
	}
}

func TestMCPSetRuleRenameShowsNewName(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addrule", map[string]interface{}{"name": "Old rule"})
	resp := mcpCall(mcp, "setrule", map[string]interface{}{"seq": float64(1), "name": "New rule"})
	text := mcpText(resp)
	if !strings.Contains(text, "New rule") {
		t.Fatalf("expected new rule name in output, got: %s", text)
	}
}

func TestMCPAddTaskRejectsSetTaskFields(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	for _, field := range []string{"desc", "verify", "deadline", "recur"} {
		resp := mcpCall(mcp, "addtask", map[string]interface{}{
			"name": "My task",
			field:  "some value",
		})
		text := mcpText(resp)
		if !strings.Contains(text, "settask") {
			t.Fatalf("expected error mentioning settask for %s, got: %s", field, text)
		}
	}
}

func TestMCPGetTaskTagFilterIncludesInheritedChildren(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "PROJ"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "Parent", "tags": []interface{}{"PROJ"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "child", "under": "Parent"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "grandchild", "under": "Parent:1"})

	resp := mcpCall(mcp, "gettask", map[string]interface{}{"tag": "PROJ"})
	text := mcpText(resp)
	if !strings.Contains(text, "child") {
		t.Fatalf("expected inherited child in tag filter, got: %s", text)
	}
	if !strings.Contains(text, "grandchild") {
		t.Fatalf("expected inherited grandchild in tag filter, got: %s", text)
	}
}

func TestMCPGetTaskTagFilterSubtaskTaggedParentNot(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "DEEP"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "Untagged"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "tagged child", "under": "Untagged", "tags": []interface{}{"DEEP"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "grandchild", "under": "Untagged:1"})

	resp := mcpCall(mcp, "gettask", map[string]interface{}{"tag": "DEEP"})
	text := mcpText(resp)
	if strings.Contains(text, "Untagged") {
		t.Fatalf("untagged parent should not appear in tag filter, got: %s", text)
	}
	if !strings.Contains(text, "tagged child") {
		t.Fatalf("expected directly tagged child, got: %s", text)
	}
	if !strings.Contains(text, "grandchild") {
		t.Fatalf("expected grandchild inheriting tag from child, got: %s", text)
	}
}

func TestMCPGetTaskSingleViewInheritedTag(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "PROJ"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "Parent", "tags": []interface{}{"PROJ"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "child", "under": "Parent"})

	resp := mcpCall(mcp, "gettask", map[string]interface{}{"task": "Parent:1", "tag": "PROJ"})
	text := mcpText(resp)
	if !strings.Contains(text, "child") {
		t.Fatalf("expected child with inherited tag in single view, got: %s", text)
	}
}

func TestMCPAddTaskShowsTagsInResponse(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "URGENT"})
	mcpCall(mcp, "addtag", map[string]interface{}{"name": "BACKEND"})

	resp := mcpCall(mcp, "addtask", map[string]interface{}{
		"name": "My task",
		"tags": []interface{}{"URGENT", "BACKEND"},
	})
	text := mcpText(resp)
	if !strings.Contains(text, "#URGENT") || !strings.Contains(text, "#BACKEND") {
		t.Fatalf("expected both tags in response, got: %s", text)
	}

	resp = mcpCall(mcp, "addtask", map[string]interface{}{
		"name": "No tags task",
	})
	text = mcpText(resp)
	if strings.Contains(text, "#") {
		t.Fatalf("expected no tags in response, got: %s", text)
	}

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "SOLO"})
	resp = mcpCall(mcp, "addtask", map[string]interface{}{
		"name": "One tag task",
		"tags": []interface{}{"SOLO"},
	})
	text = mcpText(resp)
	if !strings.Contains(text, "#SOLO") {
		t.Fatalf("expected single tag in response, got: %s", text)
	}
}

func TestMCPAddTaskShowsParentInResponse(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	// Root task should NOT show parent
	resp := mcpCall(mcp, "addtask", map[string]interface{}{"name": "Project"})
	text := mcpText(resp)
	if strings.Contains(text, "under") {
		t.Fatalf("root task should not show parent, got: %s", text)
	}

	// Subtask should show parent
	resp = mcpCall(mcp, "addtask", map[string]interface{}{
		"name":  "Backend",
		"under": "Project",
	})
	text = mcpText(resp)
	if !strings.Contains(text, "(under Project)") {
		t.Fatalf("expected '(under Project)' in response, got: %s", text)
	}

	// Deep subtask should show full parent ref
	resp = mcpCall(mcp, "addtask", map[string]interface{}{
		"name":  "API",
		"under": "Project:1",
	})
	text = mcpText(resp)
	if !strings.Contains(text, "(under Project:1)") {
		t.Fatalf("expected '(under Project:1)' in response, got: %s", text)
	}

	// Subtask with tags should show both parent and tags
	mcpCall(mcp, "addtag", map[string]interface{}{"name": "URGENT"})
	resp = mcpCall(mcp, "addtask", map[string]interface{}{
		"name":  "Database",
		"under": "Project",
		"tags":  []interface{}{"URGENT"},
	})
	text = mcpText(resp)
	if !strings.Contains(text, "(under Project)") || !strings.Contains(text, "#URGENT") {
		t.Fatalf("expected parent and tag in response, got: %s", text)
	}
}

func TestMCPGetTaskMultiTag(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "URGENT"})
	mcpCall(mcp, "addtag", map[string]interface{}{"name": "BACKEND"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T1", "tags": []interface{}{"URGENT", "BACKEND"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T2", "tags": []interface{}{"URGENT"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T3", "tags": []interface{}{"BACKEND"}})

	resp := mcpCall(mcp, "gettask", map[string]interface{}{
		"tag": []interface{}{"URGENT", "BACKEND"},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}
	text := mcpText(resp)
	if !strings.Contains(text, "T1") {
		t.Fatalf("expected T1 in output: %s", text)
	}
	if strings.Contains(text, "T2") || strings.Contains(text, "T3") {
		t.Fatalf("expected only T1: %s", text)
	}
}

func TestMCPGetTaskNotTag(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "URGENT"})
	mcpCall(mcp, "addtag", map[string]interface{}{"name": "BACKEND"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T1", "tags": []interface{}{"URGENT"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T2", "tags": []interface{}{"BACKEND"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T3"})

	resp := mcpCall(mcp, "gettask", map[string]interface{}{
		"nottag": []interface{}{"URGENT"},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}
	text := mcpText(resp)
	if strings.Contains(text, "T1") {
		t.Fatalf("T1 should be excluded: %s", text)
	}
	if !strings.Contains(text, "T2") || !strings.Contains(text, "T3") {
		t.Fatalf("expected T2 and T3: %s", text)
	}
}

func TestMCPGetTaskTagAndNotTagCombo(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "URGENT"})
	mcpCall(mcp, "addtag", map[string]interface{}{"name": "BACKEND"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T1", "tags": []interface{}{"URGENT", "BACKEND"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T2", "tags": []interface{}{"URGENT"}})

	resp := mcpCall(mcp, "gettask", map[string]interface{}{
		"tag":    []interface{}{"URGENT"},
		"nottag": []interface{}{"BACKEND"},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}
	text := mcpText(resp)
	if !strings.Contains(text, "T2") {
		t.Fatalf("expected T2: %s", text)
	}
	if strings.Contains(text, "T1") {
		t.Fatalf("T1 should be excluded: %s", text)
	}
}

func TestMCPGetTaskNonexistentTagErrors(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	resp := mcpCall(mcp, "gettask", map[string]interface{}{
		"tag": []interface{}{"NOPE"},
	})
	result := resp.Result.(map[string]interface{})
	if isErr, ok := result["isError"].(bool); !ok || !isErr {
		t.Fatal("expected error for nonexistent tag")
	}
}

func TestMCPGetTaskTagSingleStringBackcompat(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "URGENT"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T1", "tags": []interface{}{"URGENT"}})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T2"})

	// Some MCP clients might still send a string -- handle gracefully
	resp := mcpCall(mcp, "gettask", map[string]interface{}{
		"tag": "URGENT",
	})
	if resp.Error != nil {
		t.Fatalf("single string should still work: %v", resp.Error.Message)
	}
	text := mcpText(resp)
	if !strings.Contains(text, "T1") {
		t.Fatalf("expected T1: %s", text)
	}
}

func TestMCPAddRuleShowsTagsInResponse(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "BACKEND"})
	mcpCall(mcp, "addtag", map[string]interface{}{"name": "FRONTEND"})

	resp := mcpCall(mcp, "addrule", map[string]interface{}{
		"name": "No force push",
		"tags": []interface{}{"BACKEND", "FRONTEND"},
	})
	text := mcpText(resp)
	if !strings.Contains(text, "#BACKEND") || !strings.Contains(text, "#FRONTEND") {
		t.Fatalf("expected both tags in response, got: %s", text)
	}

	resp = mcpCall(mcp, "addrule", map[string]interface{}{
		"name": "Be kind",
	})
	text = mcpText(resp)
	if strings.Contains(text, "#") {
		t.Fatalf("expected no tags in response, got: %s", text)
	}
}
