package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExportEmpty(t *testing.T) {
	store := testStore(t)
	data, err := Export(store)
	if err != nil {
		t.Fatal(err)
	}
	if data.Version != 1 {
		t.Fatalf("expected version 1, got %d", data.Version)
	}
	if len(data.Tags) != 0 || len(data.Rules) != 0 || len(data.Tasks) != 0 {
		t.Fatalf("expected empty export, got %d tags, %d rules, %d tasks", len(data.Tags), len(data.Rules), len(data.Tasks))
	}
}

func TestExportRoundTrip(t *testing.T) {
	store := testStore(t)

	// Create tags
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")

	// Create rules with tags
	AddRule(store, "Always test first", "URGENT")
	AddRule(store, "Use semantic versioning", "BACKEND")

	// Create tasks with hierarchy
	AddTask(store, "Email boss", "", false, false, "URGENT")
	AddTask(store, "Draft", "Email boss", false, false)
	AddTask(store, "Review", "Email boss", false, false)
	AddTask(store, "Option A", "Email boss", true, false)
	SetTask(store, "Email boss", SetTaskOpts{Status: "active"})
	SetTask(store, "Email boss:1", SetTaskOpts{Status: "active"})
	desc := "Send the quarterly report"
	SetTask(store, "Email boss", SetTaskOpts{Desc: &desc})
	dl := "2026-04-15"
	SetTask(store, "Email boss", SetTaskOpts{Deadline: &dl})
	rc := "weekly mon"
	SetTask(store, "Email boss", SetTaskOpts{Recur: &rc})

	// Create a second root task
	AddTask(store, "Fix bug", "", false, false, "BACKEND")

	// Export
	jsonBytes, err := ExportJSON(store)
	if err != nil {
		t.Fatal(err)
	}

	// Verify JSON structure
	var data ExportData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if data.Version != 1 {
		t.Fatalf("expected version 1, got %d", data.Version)
	}
	if len(data.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(data.Tags))
	}
	if len(data.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(data.Rules))
	}
	if len(data.Tasks) != 2 {
		t.Fatalf("expected 2 root tasks, got %d", len(data.Tasks))
	}

	// Check first task details
	email := data.Tasks[0]
	if email.Name != "Email boss" {
		t.Fatalf("expected 'Email boss', got '%s'", email.Name)
	}
	if email.Status != "active" {
		t.Fatalf("expected 'active', got '%s'", email.Status)
	}
	if email.Description != "Send the quarterly report" {
		t.Fatalf("expected description, got '%s'", email.Description)
	}
	if email.Deadline == "" {
		t.Fatal("expected deadline")
	}
	if email.Recur != "weekly mon" {
		t.Fatalf("expected 'weekly mon', got '%s'", email.Recur)
	}
	if len(email.Tags) != 1 || email.Tags[0] != "URGENT" {
		t.Fatalf("expected [URGENT], got %v", email.Tags)
	}
	if len(email.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(email.Children))
	}

	// Check parallel child
	found := false
	for _, child := range email.Children {
		if child.Name == "Option A" && child.Parallel {
			found = true
		}
	}
	if !found {
		t.Fatal("expected parallel child 'Option A'")
	}

	// Check rule tags
	if len(data.Rules[0].Tags) != 1 || data.Rules[0].Tags[0] != "URGENT" {
		t.Fatalf("expected rule 1 to have tag URGENT, got %v", data.Rules[0].Tags)
	}

	// Now import into a fresh store (replace mode)
	store2 := testStore(t)
	if err := ImportJSONWithBlockers(store2, jsonBytes, true); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	// Verify tags
	tags, _ := GetTags(store2, GetTagOpts{})
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags after import, got %d: %v", len(tags), tags)
	}

	// Verify rules
	rules, _ := GetRules(store2, GetRuleOpts{})
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules after import, got %d: %v", len(rules), rules)
	}

	// Verify tasks
	tasks, _ := GetTasks(store2, GetTaskOpts{Depth: -1})
	if len(tasks) < 2 {
		t.Fatalf("expected at least 2 task lines after import, got %d: %v", len(tasks), tasks)
	}

	// Verify hierarchy: Email boss should have children
	emailTasks, _ := GetTasks(store2, GetTaskOpts{TaskRef: "Email boss", Depth: -1})
	if len(emailTasks) < 4 {
		t.Fatalf("expected at least 4 lines (parent + 3 children), got %d: %v", len(emailTasks), emailTasks)
	}
}

func TestImportMerge(t *testing.T) {
	store := testStore(t)

	// Create existing data
	AddTag(store, "EXISTING")
	AddTask(store, "Existing task", "", false, false)
	AddRule(store, "Existing rule")

	// Import data that overlaps
	data := ExportData{
		Version: 1,
		Tags:    []ExportTag{{Name: "EXISTING"}, {Name: "NEW"}},
		Rules: []ExportRule{
			{Seq: 1, Name: "Existing rule", CreatedAt: nowLocal()},
			{Seq: 2, Name: "New rule", CreatedAt: nowLocal()},
		},
		Tasks: []ExportTask{
			{Name: "Existing task", Status: "active", CreatedAt: nowLocal(), StatusChangedAt: nowLocal()},
			{Name: "New task", Status: "not_started", CreatedAt: nowLocal(), StatusChangedAt: nowLocal()},
		},
	}
	jsonBytes, _ := json.Marshal(data)

	if err := ImportJSONWithBlockers(store, jsonBytes, false); err != nil {
		t.Fatalf("merge import failed: %v", err)
	}

	// EXISTING tag should not be duplicated
	tags, _ := GetTags(store, GetTagOpts{})
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %v", len(tags), tags)
	}

	// "Existing rule" should not be duplicated, "New rule" should be added
	rules, _ := GetRules(store, GetRuleOpts{})
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d: %v", len(rules), rules)
	}
	ruleCount := 0
	for _, line := range rules {
		if strings.Contains(line, "Existing rule") {
			ruleCount++
		}
	}
	if ruleCount != 1 {
		t.Fatalf("expected 1 'Existing rule', got %d in: %v", ruleCount, rules)
	}

	// "Existing task" should not be duplicated (merge skips)
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	taskCount := 0
	for _, line := range tasks {
		if strings.Contains(line, "Existing task") {
			taskCount++
		}
	}
	if taskCount != 1 {
		t.Fatalf("expected 1 'Existing task', got %d", taskCount)
	}

	// "New task" should be added
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "New task") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'New task' after merge, got: %v", tasks)
	}
}

func TestImportReplace(t *testing.T) {
	store := testStore(t)

	// Create existing data
	AddTag(store, "OLD")
	AddTask(store, "Old task", "", false, false)

	// Import with replace
	data := ExportData{
		Version: 1,
		Tags:    []ExportTag{{Name: "NEW"}},
		Tasks: []ExportTask{
			{Name: "New task", Status: "not_started", CreatedAt: nowLocal(), StatusChangedAt: nowLocal()},
		},
	}
	jsonBytes, _ := json.Marshal(data)

	if err := ImportJSONWithBlockers(store, jsonBytes, true); err != nil {
		t.Fatalf("replace import failed: %v", err)
	}

	// Old data should be gone
	tags, _ := GetTags(store, GetTagOpts{})
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %v", len(tags), tags)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 1 || !strings.Contains(tasks[0], "New task") {
		t.Fatalf("expected only 'New task', got: %v", tasks)
	}
}

func TestImportInvalidVersion(t *testing.T) {
	store := testStore(t)
	data := `{"version": 99, "tags": [], "rules": [], "tasks": []}`
	err := ImportJSONWithBlockers(store, []byte(data), true)
	if err == nil || !strings.Contains(err.Error(), "unsupported export version") {
		t.Fatalf("expected version error, got: %v", err)
	}
}

func TestImportInvalidJSON(t *testing.T) {
	store := testStore(t)
	err := ImportJSONWithBlockers(store, []byte("not json"), true)
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected JSON error, got: %v", err)
	}
}

func TestExportWithBlockers(t *testing.T) {
	store := testStore(t)

	AddTask(store, "Task A", "", false, false)
	AddTask(store, "Step 1", "Task A", false, false)
	AddTask(store, "Step 2", "Task A", false, false)
	SetTask(store, "Task A:1", SetTaskOpts{Status: "active"})
	SetTask(store, "Task A:2", SetTaskOpts{Status: "active"})
	SetTask(store, "Task A:2", SetTaskOpts{Status: "wait", Blockers: []string{"Task A:1"}})

	data, err := Export(store)
	if err != nil {
		t.Fatal(err)
	}

	if len(data.Tasks) != 1 {
		t.Fatalf("expected 1 root task, got %d", len(data.Tasks))
	}
	taskA := data.Tasks[0]
	if len(taskA.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(taskA.Children))
	}

	// Step 2 should have a blocker reference
	step2 := taskA.Children[1]
	if step2.Name != "Step 2" {
		t.Fatalf("expected 'Step 2' at index 1, got '%s'", step2.Name)
	}
	if len(step2.Blockers) != 1 || step2.Blockers[0] != "Task A:1" {
		t.Fatalf("expected blocker 'Task A:1', got %v", step2.Blockers)
	}
}

func TestExportSummary(t *testing.T) {
	data := &ExportData{
		Version: 1,
		Tags:    []ExportTag{{Name: "A"}, {Name: "B"}},
		Rules:   []ExportRule{{Seq: 1, Name: "Rule 1"}},
		Tasks: []ExportTask{
			{Name: "Task 1", Children: []ExportTask{{Name: "Sub 1"}}},
			{Name: "Task 2"},
		},
	}
	summary := ExportSummary(data)
	if !strings.Contains(summary, "2 tags") {
		t.Fatalf("expected '2 tags' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "1 rules") {
		t.Fatalf("expected '1 rules' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "3 tasks") {
		t.Fatalf("expected '3 tasks' in summary, got: %s", summary)
	}
}

func TestImportRoundTripWithBlockers(t *testing.T) {
	store := testStore(t)

	// Setup: two tasks where B waits on A
	AddTask(store, "Alpha", "", false, false)
	AddTask(store, "Beta", "", false, false)
	SetTask(store, "Alpha", SetTaskOpts{Status: "active"})
	SetTask(store, "Beta", SetTaskOpts{Status: "active"})
	SetTask(store, "Beta", SetTaskOpts{Status: "wait", Blockers: []string{"Alpha"}})

	// Export
	jsonBytes, err := ExportJSON(store)
	if err != nil {
		t.Fatal(err)
	}

	// Import into fresh store
	store2 := testStore(t)
	if err := ImportJSONWithBlockers(store2, jsonBytes, true); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	// Beta should be waiting
	tasks, _ := GetTasks(store2, GetTaskOpts{Depth: -1})
	betaFound := false
	for _, line := range tasks {
		if strings.Contains(line, "Beta") && strings.Contains(line, "waiting") {
			betaFound = true
		}
	}
	if !betaFound {
		t.Fatalf("expected Beta to be waiting after import, got: %v", tasks)
	}
}
