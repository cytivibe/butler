package main

import (
	"strings"
	"testing"
)

func TestAddAndGetTags(t *testing.T) {
	store := testStore(t)

	tags, _ := GetTags(store, GetTagOpts{})
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}

	AddTag(store, "URGENT")
	AddTag(store, "Q3")

	tags, _ = GetTags(store, GetTagOpts{})
	if len(tags) != 2 || !strings.Contains(tags[0], "URGENT") || !strings.Contains(tags[1], "Q3") {
		t.Fatalf("got %v", tags)
	}
}

func TestTagValidation(t *testing.T) {
	store := testStore(t)

	if err := AddTag(store, "TOOLONGTAGS"); err == nil {
		t.Fatal("expected error for tag > 10 chars")
	}
	// "lower" now normalizes to "LOWER" and succeeds
	if err := AddTag(store, "lower"); err != nil {
		t.Fatalf("lowercase should normalize to LOWER: %v", err)
	}
	if err := AddTag(store, "HAS SPACE"); err == nil {
		t.Fatal("expected error for space")
	}
	if err := AddTag(store, ""); err == nil {
		t.Fatal("expected error for empty")
	}
	if err := AddTag(store, "NO-DASH"); err == nil {
		t.Fatal("expected error for dash")
	}
	AddTag(store, "DUP")
	if err := AddTag(store, "DUP"); err == nil {
		t.Fatal("expected error for duplicate")
	}
	if err := AddTag(store, "V2RELEASE"); err != nil {
		t.Fatalf("alphanumeric tag should work: %v", err)
	}
}

// --- Normalization tests (TDD: written before implementation) ---

func TestNormalizeTagName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
		desc     string
	}{
		// Cosmetic normalization -- should succeed
		{"BUTLER", "BUTLER", false, "already clean"},
		{"butler", "BUTLER", false, "lowercase to uppercase"},
		{"Butler", "BUTLER", false, "mixed case to uppercase"},
		{"#BUTLER", "BUTLER", false, "strip leading hash"},
		{"#butler", "BUTLER", false, "strip hash and uppercase"},
		{`"BUTLER"`, "BUTLER", false, "strip double quotes"},
		{`'BUTLER'`, "BUTLER", false, "strip single quotes"},
		{`"#BUTLER"`, "BUTLER", false, "strip quotes then hash"},
		{`'#BUTLER'`, "BUTLER", false, "strip single quotes then hash"},
		{`"#butler"`, "BUTLER", false, "strip quotes, hash, and uppercase"},
		{"  BUTLER  ", "BUTLER", false, "trim whitespace"},
		{" #BUTLER ", "BUTLER", false, "trim whitespace and strip hash"},
		{` "#BUTLER" `, "BUTLER", false, "trim whitespace, strip quotes and hash"},
		{"##BUTLER", "BUTLER", false, "strip multiple leading hashes"},
		{"V2", "V2", false, "alphanumeric"},
		{"v2", "V2", false, "alphanumeric lowercase"},
		{"#V2", "V2", false, "alphanumeric with hash"},

		// Invalid input -- should still fail after normalization
		{"", "", true, "empty string"},
		{"   ", "", true, "whitespace only"},
		{"#", "", true, "hash only"},
		{`""`, "", true, "empty quotes"},
		{`"#"`, "", true, "quoted hash only"},
		{"HAS SPACE", "", true, "internal space"},
		{`"MY TAG"`, "", true, "quoted but has internal space"},
		{"NO-DASH", "", true, "dash is invalid"},
		{"HELLO!", "", true, "exclamation is invalid"},
		{"TOOLONGTAGS", "", true, "exceeds 10 char limit"},
		{"NONE", "", true, "reserved name NONE"},
		{"none", "", true, "reserved name none (after uppercase)"},
		{"#NONE", "", true, "reserved name with hash"},
		{`"NONE"`, "", true, "reserved name in quotes"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := normalizeTagName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("normalizeTagName(%q) = %q, want error", tt.input, got)
				}
			} else {
				if err != nil {
					t.Errorf("normalizeTagName(%q) error: %v", tt.input, err)
				} else if got != tt.expected {
					t.Errorf("normalizeTagName(%q) = %q, want %q", tt.input, got, tt.expected)
				}
			}
		})
	}
}

func TestAddTagNormalization(t *testing.T) {
	store := testStore(t)

	// Lowercase normalizes to UPPER and succeeds
	if err := AddTag(store, "urgent"); err != nil {
		t.Fatalf("lowercase should normalize: %v", err)
	}
	tags, _ := GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "URGENT") {
		t.Fatalf("expected URGENT in tag list: %v", tags)
	}

	// Adding same tag with different case is duplicate
	if err := AddTag(store, "Urgent"); err == nil {
		t.Fatal("expected duplicate error for case variant")
	}

	// Hash prefix stripped
	if err := AddTag(store, "#BACKEND"); err != nil {
		t.Fatalf("hash prefix should be stripped: %v", err)
	}

	// Quoted with hash
	if err := AddTag(store, `"#DESIGN"`); err != nil {
		t.Fatalf("quoted hash should normalize: %v", err)
	}

	// Verify all 3 tags exist
	tags, _ = GetTags(store, GetTagOpts{})
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(tags), tags)
	}
}

func TestSetTagNormalization(t *testing.T) {
	store := testStore(t)
	AddTag(store, "OLD")

	// Rename using lowercase -- should normalize
	if err := SetTag(store, "old", "fresh"); err != nil {
		t.Fatalf("settag with lowercase should normalize both: %v", err)
	}
	tags, _ := GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "FRESH") {
		t.Fatalf("expected FRESH: %v", tags)
	}

	// Rename using hash prefix
	if err := SetTag(store, "#FRESH", "#COOL"); err != nil {
		t.Fatalf("settag with hash should normalize: %v", err)
	}
	tags, _ = GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "COOL") {
		t.Fatalf("expected COOL: %v", tags)
	}
}

func TestDeleteTagNormalization(t *testing.T) {
	store := testStore(t)
	AddTag(store, "DOOMED")

	// Delete using lowercase
	if err := DeleteTagConfirmed(store, "doomed"); err != nil {
		t.Fatalf("deletetag with lowercase should normalize: %v", err)
	}
	tags, _ := GetTags(store, GetTagOpts{})
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags: %v", tags)
	}
}

func TestGetTagNormalization(t *testing.T) {
	store := testStore(t)
	AddTag(store, "BACKEND")
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})

	// Lookup using lowercase
	tags, err := GetTags(store, GetTagOpts{Tag: "backend"})
	if err != nil {
		t.Fatalf("gettag with lowercase should normalize: %v", err)
	}
	if !strings.Contains(tags[0], "BACKEND") {
		t.Fatalf("expected BACKEND: %v", tags)
	}

	// Lookup using hash
	tags, err = GetTags(store, GetTagOpts{Tag: "#BACKEND"})
	if err != nil {
		t.Fatalf("gettag with hash should normalize: %v", err)
	}
	if !strings.Contains(tags[0], "BACKEND") {
		t.Fatalf("expected BACKEND: %v", tags)
	}
}

func TestSetTaskTagNormalization(t *testing.T) {
	store := testStore(t)
	AddTag(store, "URGENT")
	AddTask(store, "T", "", false, false)

	// Set tag using lowercase
	if err := SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"urgent"}}); err != nil {
		t.Fatalf("settask tag lowercase should normalize: %v", err)
	}
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "#URGENT") {
		t.Fatalf("expected #URGENT: %v", tasks)
	}
}

func TestAddTaskTagNormalization(t *testing.T) {
	store := testStore(t)
	AddTag(store, "URGENT")

	// Add task with lowercase tag
	if err := AddTask(store, "T", "", false, false, "urgent"); err != nil {
		t.Fatalf("addtask with lowercase tag should normalize: %v", err)
	}
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "#URGENT") {
		t.Fatalf("expected #URGENT: %v", tasks)
	}
}

func TestGetTaskTagFilterNormalization(t *testing.T) {
	store := testStore(t)
	AddTag(store, "URGENT")
	AddTask(store, "T", "", false, false, "URGENT")

	// Filter using lowercase
	tasks, err := GetTasks(store, GetTaskOpts{Tags: []string{"urgent"}, Depth: -1})
	if err != nil {
		t.Fatalf("gettask --tag lowercase should normalize: %v", err)
	}
	if len(tasks) == 0 || !strings.Contains(tasks[0], "T") {
		t.Fatalf("expected task T with tag filter: %v", tasks)
	}

	// Filter using hash
	tasks, err = GetTasks(store, GetTaskOpts{Tags: []string{"#URGENT"}, Depth: -1})
	if err != nil {
		t.Fatalf("gettask --tag #URGENT should normalize: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatalf("expected results with hash filter: %v", tasks)
	}
}

func TestAddRuleTagNormalization(t *testing.T) {
	store := testStore(t)
	AddTag(store, "POLICY")

	// Add rule with lowercase tag
	if _, err := AddRule(store, "My Rule", "policy"); err != nil {
		t.Fatalf("addrule with lowercase tag should normalize: %v", err)
	}
	rules, _ := GetRules(store, GetRuleOpts{})
	if !strings.Contains(rules[0], "#POLICY") {
		t.Fatalf("expected #POLICY on rule: %v", rules)
	}
}

func TestMCPAddTagNormalization(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	// Add via MCP with lowercase
	resp := mcpCall(mcp, "addtag", map[string]interface{}{"name": "urgent"})
	if resp.Error != nil {
		t.Fatalf("MCP addtag lowercase should normalize: %v", resp.Error.Message)
	}

	// Add via MCP with hash
	resp = mcpCall(mcp, "addtag", map[string]interface{}{"name": "#BACKEND"})
	if resp.Error != nil {
		t.Fatalf("MCP addtag hash should normalize: %v", resp.Error.Message)
	}

	// Add via MCP with quoted hash
	resp = mcpCall(mcp, "addtag", map[string]interface{}{"name": `"#DESIGN"`})
	if resp.Error != nil {
		t.Fatalf("MCP addtag quoted hash should normalize: %v", resp.Error.Message)
	}

	// Verify tags created correctly
	resp = mcpCall(mcp, "gettag", map[string]interface{}{"all": true})
	text := mcpText(resp)
	if !strings.Contains(text, "URGENT") || !strings.Contains(text, "BACKEND") || !strings.Contains(text, "DESIGN") {
		t.Fatalf("expected all 3 tags: %s", text)
	}
}

func TestMCPSetTaskTagNormalization(t *testing.T) {
	store := testStore(t)
	mcp := NewMCPServer("butler", "1.0.0")
	registerTools(mcp, store)

	mcpCall(mcp, "addtag", map[string]interface{}{"name": "URGENT"})
	mcpCall(mcp, "addtask", map[string]interface{}{"name": "T"})

	// Set tag via MCP with lowercase
	resp := mcpCall(mcp, "settask", map[string]interface{}{
		"task": "T",
		"tags": []interface{}{"urgent"},
	})
	if resp.Error != nil {
		t.Fatalf("MCP settask lowercase tag should normalize: %v", resp.Error.Message)
	}
}

func TestTagTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "BACKEND"}})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "#URGENT") || !strings.Contains(tasks[0], "#BACKEND") {
		t.Fatalf("expected tags in display: %v", tasks)
	}
}

func TestTagSubtask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTag(store, "DESIGN")

	SetTask(store, "T:1", SetTaskOpts{SetTags: true, Tags: []string{"DESIGN"}})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "s1") && strings.Contains(line, "#DESIGN") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tag on subtask: %v", tasks)
	}
}

func TestSetRuleTags(t *testing.T) {
	store := testStore(t)
	AddRule(store, "My Rule")
	AddTag(store, "POLICY")

	setRuleTags(store, 1, "POLICY")

	rules, _ := GetRules(store, GetRuleOpts{})
	if !strings.Contains(rules[0], "Rule 1: My Rule") || !strings.Contains(rules[0], "#POLICY") {
		t.Fatalf("expected tag on rule: %v", rules)
	}
}

func TestTagCountsInGetTagAll(t *testing.T) {
	store := testStore(t)
	AddTag(store, "POLICY")
	AddRule(store, "R1")

	// No associations yet
	tags, _ := GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "POLICY") || !strings.Contains(tags[0], "unused") {
		t.Fatalf("expected no counts before tagging: %v", tags)
	}

	setRuleTags(store, 1, "POLICY")
	tags, _ = GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "POLICY") || !strings.Contains(tags[0], "1 rule") {
		t.Fatalf("expected '1 rule' count: %v", tags)
	}

	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"POLICY"}})
	tags, _ = GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "POLICY") || !strings.Contains(tags[0], "1 task") || !strings.Contains(tags[0], "1 rule") {
		t.Fatalf("expected '1 task, 1 rule': %v", tags)
	}
}

func TestTagCountsOnlyTasksNoRules(t *testing.T) {
	store := testStore(t)
	AddTag(store, "MYTAG")
	AddTask(store, "T", "", false, false)

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"MYTAG"}})
	tags, _ := GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "MYTAG") || !strings.Contains(tags[0], "1 task") {
		t.Fatalf("expected '1 task' with no rule count: %v", tags)
	}
}

func TestTagNotFound(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	if err := SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"NONEXISTENT"}}); err == nil {
		t.Fatal("expected error for nonexistent tag")
	}
}

func TestTagReplaceSemantics(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTag(store, "A")
	AddTag(store, "B")
	AddTag(store, "C")

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"A", "B"}})
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "#A") || !strings.Contains(tasks[0], "#B") {
		t.Fatalf("expected #A #B: %v", tasks)
	}

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"C"}})
	tasks, _ = GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "#C") {
		t.Fatalf("expected #C: %v", tasks)
	}
	if strings.Contains(tasks[0], "#A") || strings.Contains(tasks[0], "#B") {
		t.Fatalf("old tags should be replaced: %v", tasks)
	}

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{}})
	tasks, _ = GetTasks(store, GetTaskOpts{Depth: -1})
	if strings.Contains(tasks[0], "#") {
		t.Fatalf("expected no tags: %v", tasks)
	}
}

func TestGetTagRequiresArgument(t *testing.T) {
	store := testStore(t)
	AddTag(store, "X")

	// GetTags with empty opts returns all tags (--all)
	tags, _ := GetTags(store, GetTagOpts{})
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %v", len(tags), tags)
	}
}

func TestGetTagSingleView(t *testing.T) {
	store := testStore(t)
	AddTag(store, "BACKEND")
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddRule(store, "Code review")

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})
	SetTask(store, "T:1", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})
	setRuleTags(store, 1, "BACKEND")

	tags, _ := GetTags(store, GetTagOpts{Tag: "BACKEND"})
	if !strings.Contains(tags[0], "BACKEND") || !strings.Contains(tags[0], "2 tasks") || !strings.Contains(tags[0], "1 rule") {
		t.Fatalf("expected header with counts: %v", tags)
	}
	// Find task and rule entries
	foundT := false
	foundS1 := false
	foundRule := false
	for _, line := range tags {
		if strings.Contains(line, "T") && !strings.Contains(line, "Tasks") && !strings.Contains(line, ":") {
			foundT = true
		}
		if strings.Contains(line, "s1") && strings.Contains(line, "(T:1)") {
			foundS1 = true
		}
		if strings.Contains(line, "Rule 1") {
			foundRule = true
		}
	}
	if !foundT || !foundS1 || !foundRule {
		t.Fatalf("expected task T, subtask s1 (T:1), and Rule 1 in output: %v", tags)
	}
}

func TestGetTagSingleViewNoTasks(t *testing.T) {
	store := testStore(t)
	AddTag(store, "EMPTY")

	tags, _ := GetTags(store, GetTagOpts{Tag: "EMPTY"})
	if !strings.Contains(tags[0], "EMPTY") {
		t.Fatalf("expected tag name in header: %v", tags)
	}
}

func TestGetTagSingleViewNotFound(t *testing.T) {
	store := testStore(t)

	_, err := GetTags(store, GetTagOpts{Tag: "NOPE"})
	if err == nil {
		t.Fatal("expected error for nonexistent tag")
	}
}

func TestGetTagSingleViewHidesDeletedRules(t *testing.T) {
	store := testStore(t)
	AddTag(store, "X")
	AddRule(store, "R1", "X")
	AddRule(store, "R2", "X")
	DeleteRule(store, 1)

	tags, _ := GetTags(store, GetTagOpts{Tag: "X"})
	for _, line := range tags {
		if strings.Contains(line, "R1") {
			t.Fatalf("deleted rule should be hidden: %v", tags)
		}
	}
}

func TestSetTag(t *testing.T) {
	store := testStore(t)
	AddTag(store, "OLD")
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"OLD"}})

	if err := SetTag(store, "OLD", "NEW"); err != nil {
		t.Fatal(err)
	}

	// Tag list should show NEW with count
	tags, _ := GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "NEW") || !strings.Contains(tags[0], "1 task") {
		t.Fatalf("expected renamed tag with count: %v", tags)
	}

	// Task should show new tag
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "#NEW") {
		t.Fatalf("expected #NEW on task: %v", tasks)
	}
	if strings.Contains(tasks[0], "#OLD") {
		t.Fatalf("old tag should not appear: %v", tasks)
	}
}

func TestSetTagValidation(t *testing.T) {
	store := testStore(t)
	AddTag(store, "VALID")

	// "lower" now normalizes to "LOWER" and succeeds
	if err := SetTag(store, "VALID", "lower"); err != nil {
		t.Fatalf("lowercase should normalize to LOWER: %v", err)
	}
	// Re-create VALID since it was renamed to LOWER
	AddTag(store, "VALID")
	if err := SetTag(store, "VALID", "TOOLONGTAGS"); err == nil {
		t.Fatal("expected error for > 10 chars")
	}
	if err := SetTag(store, "VALID", ""); err == nil {
		t.Fatal("expected error for empty")
	}
	if err := SetTag(store, "VALID", "NONE"); err == nil {
		t.Fatal("expected error for NONE")
	}
}

func TestSetTagDuplicate(t *testing.T) {
	store := testStore(t)
	AddTag(store, "A")
	AddTag(store, "B")

	if err := SetTag(store, "A", "B"); err == nil {
		t.Fatal("expected error for duplicate tag name")
	}
}

func TestSetTagNotFound(t *testing.T) {
	store := testStore(t)

	if err := SetTag(store, "NOPE", "NEW"); err == nil {
		t.Fatal("expected error for nonexistent tag")
	}
}

func TestSetTagRuleAssociation(t *testing.T) {
	store := testStore(t)
	AddTag(store, "OLD")
	AddRule(store, "R1", "OLD")

	SetTag(store, "OLD", "NEW")

	// Rule should show new tag
	rules, _ := GetRules(store, GetRuleOpts{})
	if !strings.Contains(rules[0], "#NEW") {
		t.Fatalf("expected #NEW on rule: %v", rules)
	}
}

func TestDeleteTag(t *testing.T) {
	store := testStore(t)
	AddTag(store, "DOOMED")

	if err := DeleteTagConfirmed(store, "DOOMED"); err != nil {
		t.Fatal(err)
	}

	tags, _ := GetTags(store, GetTagOpts{})
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags: %v", tags)
	}
}

func TestDeleteTagRemovesFromTasks(t *testing.T) {
	store := testStore(t)
	AddTag(store, "DOOMED")
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"DOOMED"}})

	DeleteTagConfirmed(store, "DOOMED")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if strings.Contains(tasks[0], "#DOOMED") {
		t.Fatalf("tag should be removed from task: %v", tasks)
	}
}

func TestDeleteTagRemovesFromRules(t *testing.T) {
	store := testStore(t)
	AddTag(store, "DOOMED")
	AddRule(store, "R1", "DOOMED")

	DeleteTagConfirmed(store, "DOOMED")

	rules, _ := GetRules(store, GetRuleOpts{})
	if strings.Contains(rules[0], "#DOOMED") {
		t.Fatalf("tag should be removed from rule: %v", rules)
	}
}

func TestDeleteTagNotFound(t *testing.T) {
	store := testStore(t)

	if err := DeleteTagConfirmed(store, "NOPE"); err == nil {
		t.Fatal("expected error for nonexistent tag")
	}
}

func TestDeleteTagInfo(t *testing.T) {
	store := testStore(t)
	AddTag(store, "X")
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddRule(store, "R1")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"X"}})
	SetTask(store, "T2", SetTaskOpts{SetTags: true, Tags: []string{"X"}})
	setRuleTags(store, 1, "X")

	taskCount, ruleCount, err := DeleteTagInfo(store, "X")
	if err != nil {
		t.Fatal(err)
	}
	if taskCount != 2 {
		t.Fatalf("expected 2 tasks, got %d", taskCount)
	}
	if ruleCount != 1 {
		t.Fatalf("expected 1 rule, got %d", ruleCount)
	}
}

func TestDeleteTagBreaksRuleLink(t *testing.T) {
	store := testStore(t)
	AddTag(store, "LINK")
	AddTask(store, "T", "", false, false)
	AddRule(store, "R1")

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"LINK"}})
	setRuleTags(store, 1, "LINK")

	// Before delete, rule shows in details
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T", Details: true, Depth: -1})
	hasRule := false
	for _, line := range tasks {
		if strings.Contains(line, "R1") {
			hasRule = true
		}
	}
	if !hasRule {
		t.Fatalf("expected rule in details before delete: %v", tasks)
	}

	// Delete tag - rule link broken
	DeleteTagConfirmed(store, "LINK")

	tasks, _ = GetTasks(store, GetTaskOpts{TaskRef: "T", Details: true, Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "R1") {
			t.Fatalf("rule should not show after tag deleted: %v", tasks)
		}
	}
}

func TestAddTagNoneReserved(t *testing.T) {
	store := testStore(t)
	if err := AddTag(store, "NONE"); err == nil {
		t.Fatal("expected error: NONE is reserved")
	}
}

func TestNormalizeTagList(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
		wantErr  bool
		desc     string
	}{
		// Single tag
		{[]string{"URGENT"}, []string{"URGENT"}, false, "single tag passthrough"},
		{[]string{"urgent"}, []string{"URGENT"}, false, "single tag normalized"},
		{[]string{"#urgent"}, []string{"URGENT"}, false, "single tag with hash"},

		// Multiple tags via separate elements
		{[]string{"URGENT", "BACKEND"}, []string{"URGENT", "BACKEND"}, false, "two separate tags"},
		{[]string{"urgent", "backend"}, []string{"URGENT", "BACKEND"}, false, "two separate lowercase"},
		{[]string{"#URGENT", "#BACKEND"}, []string{"URGENT", "BACKEND"}, false, "two with hashes"},

		// Comma-separated within single element
		{[]string{"URGENT,BACKEND"}, []string{"URGENT", "BACKEND"}, false, "comma separated"},
		{[]string{"urgent,backend"}, []string{"URGENT", "BACKEND"}, false, "comma separated lowercase"},
		{[]string{"URGENT, BACKEND"}, []string{"URGENT", "BACKEND"}, false, "comma space separated"},
		{[]string{" URGENT , BACKEND "}, []string{"URGENT", "BACKEND"}, false, "comma with extra whitespace"},

		// Mixed: some comma-separated, some separate elements
		{[]string{"URGENT,BACKEND", "INFRA"}, []string{"URGENT", "BACKEND", "INFRA"}, false, "mixed comma and separate"},

		// Deduplication
		{[]string{"URGENT", "URGENT"}, []string{"URGENT"}, false, "dedup same element"},
		{[]string{"URGENT,URGENT"}, []string{"URGENT"}, false, "dedup within comma"},
		{[]string{"urgent", "URGENT"}, []string{"URGENT"}, false, "dedup after normalization"},

		// NONE special case
		{[]string{"NONE"}, []string{"NONE"}, false, "NONE passthrough"},
		{[]string{"none"}, []string{"NONE"}, false, "none lowercase"},
		{[]string{"#NONE"}, []string{"NONE"}, false, "NONE with hash"},
		{[]string{"#none"}, []string{"NONE"}, false, "none with hash lowercase"},

		// NONE mixed with real tags
		{[]string{"NONE", "URGENT"}, nil, true, "NONE cannot mix with real tags"},
		{[]string{"URGENT,NONE"}, nil, true, "NONE cannot mix comma"},

		// Empty after split
		{[]string{""}, nil, true, "empty string"},
		{[]string{","}, nil, true, "just comma"},
		{[]string{"URGENT,"}, []string{"URGENT"}, false, "trailing comma ignored"},
		{[]string{",URGENT"}, []string{"URGENT"}, false, "leading comma ignored"},

		// Invalid tag in list
		{[]string{"URGENT", "BAD TAG"}, nil, true, "invalid tag in list"},
		{[]string{"URGENT,TOOLONGTAGS"}, nil, true, "invalid tag in comma list"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := normalizeTagList(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("normalizeTagList(%v) = %v, want error", tt.input, got)
				}
			} else {
				if err != nil {
					t.Errorf("normalizeTagList(%v) error: %v", tt.input, err)
				} else if !slicesEqual(got, tt.expected) {
					t.Errorf("normalizeTagList(%v) = %v, want %v", tt.input, got, tt.expected)
				}
			}
		})
	}
}

// slicesEqual compares two string slices for equality.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTagExists(t *testing.T) {
	store := testStore(t)
	AddTag(store, "URGENT")

	exists, err := tagExists(store, "URGENT")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected URGENT to exist")
	}

	exists, err = tagExists(store, "NOPE")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected NOPE to not exist")
	}
}

func TestAddTaskWithTags(t *testing.T) {
	store := testStore(t)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")

	AddTask(store, "T1", "", false, false, "URGENT", "BACKEND")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "#URGENT") || !strings.Contains(tasks[0], "#BACKEND") {
		t.Fatalf("expected tags on task: %v", tasks)
	}
}

func TestAddSubtaskWithTags(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTag(store, "DESIGN")

	AddTask(store, "sub", "T", false, false, "DESIGN")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "sub") && strings.Contains(line, "#DESIGN") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tag on subtask: %v", tasks)
	}
}
