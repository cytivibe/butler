package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStoreAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func strPtr(s string) *string { return &s }

// hasStatus checks if a line contains a status display like "[active since" or "[created on".
func hasStatus(line, status string) bool {
	if status == "not_started" || status == "created" {
		return strings.Contains(line, "[created on")
	}
	return strings.Contains(line, "["+strings.ReplaceAll(status, "_", " "))
}

func setStatus(store *Store, ref, status string) error {
	return SetTask(store, ref, SetTaskOpts{Status: status})
}

func setWait(store *Store, ref string, blockers ...string) error {
	return SetTask(store, ref, SetTaskOpts{Status: "wait", Blockers: blockers})
}

func setRuleTags(store *Store, seq int, tags ...string) error {
	return SetRule(store, seq, SetRuleOpts{SetTags: true, Tags: tags})
}

func mcpCall(mcp *MCPServer, tool string, args map[string]interface{}) jsonRPCResponse {
	params, _ := json.Marshal(map[string]interface{}{
		"name":      tool,
		"arguments": args,
	})
	return mcp.handle(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  params,
	})
}

func mcpText(resp jsonRPCResponse) string {
	result := resp.Result.(map[string]interface{})
	content := result["content"].([]map[string]interface{})
	return content[0]["text"].(string)
}

// captureStdout runs fn and returns whatever it printed to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}
