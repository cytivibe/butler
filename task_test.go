package main

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestAddAndGetTasks(t *testing.T) {
	store := testStore(t)

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}

	AddTask(store, "Task 1", "", false, false)
	AddTask(store, "Task 2", "", false, false)

	tasks, _ = GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if !strings.Contains(tasks[0], "Task 1") || !hasStatus(tasks[0], "not_started") ||
		!strings.Contains(tasks[1], "Task 2") || !hasStatus(tasks[1], "not_started") {
		t.Fatalf("got %v", tasks)
	}
}

func TestGetSingleTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "s1", "T1", false, false)
	AddTask(store, "s2", "T1", false, false)
	AddTask(store, "deep", "T1:1", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "other", "T2", false, false)

	// Get specific named task - should show T1 and its children, not T2
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T1", Depth: -1})
	if len(tasks) != 4 {
		t.Fatalf("expected 4 lines (T1 + 3 children), got %d: %v", len(tasks), tasks)
	}
	for _, line := range tasks {
		if strings.Contains(line, "other") || strings.Contains(line, "T2") {
			t.Fatalf("should not include T2 tasks: %v", tasks)
		}
	}

	// Get specific subtask - should show s1 and its child
	tasks, _ = GetTasks(store, GetTaskOpts{TaskRef: "T1:1", Depth: -1})
	if len(tasks) != 2 {
		t.Fatalf("expected 2 lines (s1 + deep), got %d: %v", len(tasks), tasks)
	}
}

func TestGetTaskDepthZero(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "s1", "T1", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "s2", "T2", false, false)

	// --all --depth 0 → only named tasks, no children
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: 0})
	if len(tasks) != 2 {
		t.Fatalf("expected 2 named tasks, got %d: %v", len(tasks), tasks)
	}
	for _, line := range tasks {
		if strings.Contains(line, "s1") || strings.Contains(line, "s2") {
			t.Fatalf("depth 0 should not show children: %v", tasks)
		}
	}
}

func TestGetTaskDepthOne(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "deep", "T:1", false, false)

	// --depth 1 → named task + direct children, not grandchildren
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: 1})
	if len(tasks) != 2 {
		t.Fatalf("expected 2 lines (T + s1), got %d: %v", len(tasks), tasks)
	}
	for _, line := range tasks {
		if strings.Contains(line, "deep") {
			t.Fatalf("depth 1 should not show grandchildren: %v", tasks)
		}
	}
}

func TestGetTaskDepthOnSingleTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "deep", "T:1", false, false)
	AddTask(store, "deeper", "T:1.1", false, false)

	// Single task with depth 0 - just the task
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T:1", Depth: 0})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 line (s1 only), got %d: %v", len(tasks), tasks)
	}

	// Single task with depth 1 - task + direct children
	tasks, _ = GetTasks(store, GetTaskOpts{TaskRef: "T:1", Depth: 1})
	if len(tasks) != 2 {
		t.Fatalf("expected 2 lines (s1 + deep), got %d: %v", len(tasks), tasks)
	}
}

func TestGetTaskFilterByStatus(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "s2", "T", false, false)
	AddTask(store, "s3", "T", false, false)

	setStatus(store, "T:1", "active")
	setStatus(store, "T:2", "active")
	setStatus(store, "T:2", "completed")

	// Filter by active - should show T (active via auto-activate), s1 (active)
	tasks, _ := GetTasks(store, GetTaskOpts{Status: "active", Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "s2") || strings.Contains(line, "s3") {
			t.Fatalf("should not show completed/not_started tasks: %v", tasks)
		}
	}

	tasks, _ = GetTasks(store, GetTaskOpts{Status: "completed", Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "s2") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected s2 in completed filter: %v", tasks)
	}
}

func TestGetTaskFilterByStatusOnSingleTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "s2", "T", false, false)

	setStatus(store, "T:1", "active")

	// Filter children of T by active
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T", Status: "active", Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "s2") {
			t.Fatalf("should not show not_started s2: %v", tasks)
		}
	}
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "s1") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected active s1: %v", tasks)
	}
}

func TestGetTaskStatusFilterWithDepth(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "deep", "T:1", false, false)

	setStatus(store, "T:1", "active")
	setStatus(store, "T:1.1", "active")

	// Filter active + depth 1 → T and s1 but not deep
	tasks, _ := GetTasks(store, GetTaskOpts{Status: "active", Depth: 1})
	for _, line := range tasks {
		if strings.Contains(line, "deep") {
			t.Fatalf("depth 1 should not show grandchild: %v", tasks)
		}
	}
}

func TestGetTaskDepthWithDetails(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "deep", "T:1", false, false)

	SetTask(store, "T", SetTaskOpts{Desc: strPtr("root desc")})
	SetTask(store, "T:1", SetTaskOpts{Desc: strPtr("child desc")})
	SetTask(store, "T:1.1", SetTaskOpts{Desc: strPtr("grand desc")})

	// Depth 1 + details - should show T desc and s1 desc, not deep desc
	tasks, _ := GetTasks(store, GetTaskOpts{Details: true, Depth: 1})
	hasRoot := false
	hasChild := false
	for _, line := range tasks {
		if strings.Contains(line, "root desc") {
			hasRoot = true
		}
		if strings.Contains(line, "child desc") {
			hasChild = true
		}
		if strings.Contains(line, "grand desc") {
			t.Fatalf("depth 1 should not show grandchild details: %v", tasks)
		}
	}
	if !hasRoot || !hasChild {
		t.Fatalf("expected root and child details: %v", tasks)
	}
}

func TestGetTaskFilterByTag(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "T3", "", false, false)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"URGENT"}})
	SetTask(store, "T2", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "BACKEND"}})
	SetTask(store, "T3", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})

	// Filter by URGENT - should show T1 and T2
	tasks, _ := GetTasks(store, GetTaskOpts{Tags: []string{"URGENT"}, Depth: -1})
	if len(tasks) != 2 {
		t.Fatalf("expected 2 urgent tasks, got %d: %v", len(tasks), tasks)
	}
	for _, line := range tasks {
		if strings.Contains(line, "T3") {
			t.Fatalf("T3 is not URGENT: %v", tasks)
		}
	}

	// Filter by BACKEND - should show T2 and T3
	tasks, _ = GetTasks(store, GetTaskOpts{Tags: []string{"BACKEND"}, Depth: -1})
	if len(tasks) != 2 {
		t.Fatalf("expected 2 backend tasks, got %d: %v", len(tasks), tasks)
	}
}

func TestGetTaskFilterByTagAndStatus(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddTag(store, "URGENT")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"URGENT"}})
	SetTask(store, "T2", SetTaskOpts{SetTags: true, Tags: []string{"URGENT"}})
	setStatus(store, "T1", "active")

	// Filter by URGENT + active - should show only T1
	tasks, _ := GetTasks(store, GetTaskOpts{Tags: []string{"URGENT"}, Status: "active", Depth: -1})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 active+urgent task, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "T1") {
		t.Fatalf("expected T1: %v", tasks)
	}
}

func TestGetTaskFilterByTagNone(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Tagged", "", false, false)
	AddTask(store, "Untagged", "", false, false)
	AddTag(store, "X")
	SetTask(store, "Tagged", SetTaskOpts{SetTags: true, Tags: []string{"X"}})

	// Filter by NONE - should show only untagged task
	tasks, _ := GetTasks(store, GetTaskOpts{Tags: []string{"NONE"}, Depth: -1})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 untagged task, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "Untagged") {
		t.Fatalf("expected untagged task: %v", tasks)
	}
}

func TestGetTaskFilterByTagNoneExcludesChildrenOfTagged(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Parent", "", false, false)
	AddTask(store, "child", "Parent", false, false)
	AddTag(store, "X")
	SetTask(store, "Parent", SetTaskOpts{SetTags: true, Tags: []string{"X"}})

	// child has no direct tags but parent does - should NOT appear in NONE
	tasks, _ := GetTasks(store, GetTaskOpts{Tags: []string{"NONE"}, Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "child") {
			t.Fatalf("children of tagged parents should not appear in NONE: %v", tasks)
		}
	}
}

func TestTasksAndRulesIndependent(t *testing.T) {
	store := testStore(t)

	AddTask(store, "My Task", "", false, false)
	AddRule(store, "My Rule")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	rules, _ := GetRules(store, GetRuleOpts{})

	if len(tasks) != 1 || !strings.HasPrefix(tasks[0], "My Task") {
		t.Fatalf("tasks: %v", tasks)
	}
	if len(rules) != 1 || rules[0] != "Rule 1: My Rule" {
		t.Fatalf("rules: %v", rules)
	}
}

func TestCreationOrder(t *testing.T) {
	store := testStore(t)

	names := []string{"C", "A", "B"}
	for _, n := range names {
		AddTask(store, n, "", false, false)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for i, n := range names {
		if !strings.HasPrefix(tasks[i], n+" ") {
			t.Fatalf("position %d: expected '%s...', got '%s'", i, n, tasks[i])
		}
	}
}

func TestWriteThenReadSeesLatestData(t *testing.T) {
	store := testStore(t)

	AddTask(store, "First", "", false, false)
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 1 || !strings.HasPrefix(tasks[0], "First") {
		t.Fatal("read did not see committed write")
	}

	AddTask(store, "Second", "", false, false)
	tasks, _ = GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 2 || !strings.HasPrefix(tasks[1], "Second") {
		t.Fatal("read did not see second committed write")
	}
}

func TestTaskNameLimit(t *testing.T) {
	store := testStore(t)

	name50 := strings.Repeat("a", 50)
	if err := AddTask(store, name50, "", false, false); err != nil {
		t.Fatalf("50 chars should work: %v", err)
	}
	name51 := strings.Repeat("a", 51)
	if err := AddTask(store, name51, "", false, false); err == nil {
		t.Fatal("expected error for > 50 chars")
	}
	if err := AddTask(store, name51, name50, false, false); err == nil {
		t.Fatal("expected error for subtask > 50 chars")
	}
}

func TestStatusTransitions(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	if err := setStatus(store, "T", "active"); err != nil {
		t.Fatal(err)
	}
	if err := setStatus(store, "T", "completed"); err != nil {
		t.Fatal(err)
	}
	if err := setStatus(store, "T", "reopened"); err != nil {
		t.Fatal(err)
	}
	if err := setStatus(store, "T", "active"); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidTransitions(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	if err := setStatus(store, "T", "completed"); err == nil {
		t.Fatal("expected error for not_started → completed")
	}
	if err := setStatus(store, "T", "waiting"); err == nil {
		t.Fatal("expected error for direct waiting")
	}
}

func TestCancelledIsTerminal(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	setStatus(store, "T", "active")
	setStatus(store, "T", "cancelled")

	if err := setStatus(store, "T", "active"); err == nil {
		t.Fatal("expected error: cancelled is terminal")
	}
}

func TestSetStatusWithPosition(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "sub1", "T", false, false)

	if err := setStatus(store, "T:1", "active"); err != nil {
		t.Fatal(err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "sub1") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("subtask status not updated: %v", tasks)
	}
}

func TestSetDescriptionAndVerification(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	SetTask(store, "T", SetTaskOpts{Desc: strPtr("do the thing"), Verify: strPtr("check it worked")})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "desc:") {
			t.Fatalf("description should be hidden without --details: %v", tasks)
		}
		// The [verify: pending] badge is expected; only the details line "verify: ..." should be hidden
		if strings.Contains(line, "verify:") && !strings.Contains(line, "[verify:") {
			t.Fatalf("verify details should be hidden without --details: %v", tasks)
		}
	}

	tasks, _ = GetTasks(store, GetTaskOpts{Details: true, Depth: -1})
	hasDesc := false
	hasVerify := false
	for _, line := range tasks {
		if strings.Contains(line, "desc: do the thing") {
			hasDesc = true
		}
		if strings.Contains(line, "verify: check it worked") {
			hasVerify = true
		}
	}
	if !hasDesc {
		t.Fatalf("expected description in details: %v", tasks)
	}
	if !hasVerify {
		t.Fatalf("expected verification in details: %v", tasks)
	}
}

func TestDetailsOnSubtasks(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)

	SetTask(store, "T:1", SetTaskOpts{Desc: strPtr("subtask desc"), Verify: strPtr("subtask verify")})

	tasks, _ := GetTasks(store, GetTaskOpts{Details: true, Depth: -1})
	hasDesc := false
	hasVerify := false
	for _, line := range tasks {
		if strings.Contains(line, "desc: subtask desc") {
			hasDesc = true
		}
		if strings.Contains(line, "verify: subtask verify") {
			hasVerify = true
		}
	}
	if !hasDesc || !hasVerify {
		t.Fatalf("expected subtask details: %v", tasks)
	}
}

func TestEmptyDetailsNotShown(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	tasks, _ := GetTasks(store, GetTaskOpts{Details: true, Depth: -1})
	if len(tasks) != 2 {
		t.Fatalf("expected 2 lines (task + created on), got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[1], "created:") {
		t.Fatalf("expected 'created:' detail line, got: %v", tasks)
	}
}

func TestTimestampDisplayFormat(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	// not_started shows as "created on"
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[created on") {
		t.Fatalf("expected [created on ...]: %v", tasks)
	}

	// active shows "since"
	setStatus(store, "T", "active")
	tasks, _ = GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[active since") {
		t.Fatalf("expected [active since ...]: %v", tasks)
	}

	// completed shows "on"
	setStatus(store, "T", "completed")
	tasks, _ = GetTasks(store, GetTaskOpts{Status: "completed", Depth: -1})
	if !strings.Contains(tasks[0], "[completed on") {
		t.Fatalf("expected [completed on ...]: %v", tasks)
	}
}

func TestInheritedStatusUsesParentTimestamp(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "child", "P", false, false)
	setStatus(store, "P", "active")
	setStatus(store, "P:1", "active")
	setStatus(store, "P", "deferred")

	// Both parent and child should show deferred - child inherits parent's display
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "child") {
			if !strings.Contains(line, "[deferred since") {
				t.Fatalf("child should inherit deferred display: %v", tasks)
			}
		}
	}
}

func TestDetailsShowsCreatedOn(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	setStatus(store, "T", "active")
	SetTask(store, "T", SetTaskOpts{Desc: strPtr("my desc")})

	tasks, _ := GetTasks(store, GetTaskOpts{Details: true, Depth: -1})
	foundCreated := false
	for _, line := range tasks {
		if strings.Contains(line, "created:") {
			foundCreated = true
		}
	}
	if !foundCreated {
		t.Fatalf("expected 'created:' in details: %v", tasks)
	}
}

func TestSortRecent(t *testing.T) {
	store := testStore(t)
	AddTask(store, "First", "", false, false)
	AddTask(store, "Second", "", false, false)

	// Without sort, order is by id (creation order)
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: 0})
	if !strings.Contains(tasks[0], "First") {
		t.Fatalf("expected 'First' first without sort: %v", tasks)
	}

	// Activate only "First" - it now has a more recent status_changed_at
	setStatus(store, "First", "active")

	// With sort=recent, "First" (active, changed later) should come before "Second" (not_started, older created_at)
	tasks, _ = GetTasks(store, GetTaskOpts{Sort: "recent", Depth: 0})
	if len(tasks) < 2 {
		t.Fatalf("expected 2 tasks: %v", tasks)
	}
	if !strings.Contains(tasks[0], "First") {
		t.Fatalf("expected 'First' first with --sort recent (it was changed more recently): %v", tasks)
	}
}

func TestSortDeadline(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Later", "", false, false)
	AddTask(store, "Sooner", "", false, false)
	AddTask(store, "NoDeadline", "", false, false)

	dl1 := "2026-06-01"
	SetTask(store, "Later", SetTaskOpts{Deadline: &dl1})
	dl2 := "2026-04-01"
	SetTask(store, "Sooner", SetTaskOpts{Deadline: &dl2})

	tasks, _ := GetTasks(store, GetTaskOpts{Sort: "deadline", Depth: 0})
	if len(tasks) < 3 {
		t.Fatalf("expected 3 tasks: %v", tasks)
	}
	if !strings.Contains(tasks[0], "Sooner") {
		t.Fatalf("expected 'Sooner' first with --sort deadline: %v", tasks)
	}
	if !strings.Contains(tasks[1], "Later") {
		t.Fatalf("expected 'Later' second: %v", tasks)
	}
	if !strings.Contains(tasks[2], "NoDeadline") {
		t.Fatalf("expected 'NoDeadline' last: %v", tasks)
	}
}

func TestSetDeadlineDate(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	dl := "2026-04-15"
	SetTask(store, "T", SetTaskOpts{Deadline: &dl})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[deadline: Apr 15]") {
		t.Fatalf("expected [deadline: Apr 15]: %v", tasks)
	}
}

func TestSetDeadlineDatetime(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	dl := "2026-04-15 14:00"
	SetTask(store, "T", SetTaskOpts{Deadline: &dl})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[deadline: Apr 15 14:00]") {
		t.Fatalf("expected [deadline: Apr 15 14:00]: %v", tasks)
	}
}

func TestClearDeadline(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	dl := "2026-04-15"
	SetTask(store, "T", SetTaskOpts{Deadline: &dl})

	none := "none"
	SetTask(store, "T", SetTaskOpts{Deadline: &none})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if strings.Contains(tasks[0], "deadline") || strings.Contains(tasks[0], "overdue") {
		t.Fatalf("expected no deadline after clearing: %v", tasks)
	}
}

func TestOverdueDisplay(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	dl := "2020-01-01"
	SetTask(store, "T", SetTaskOpts{Deadline: &dl})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[overdue: Jan 1]") {
		t.Fatalf("expected [overdue: Jan 1]: %v", tasks)
	}
}

func TestOverdueNotShownForCompleted(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	dl := "2020-01-01"
	SetTask(store, "T", SetTaskOpts{Deadline: &dl})
	setStatus(store, "T", "active")
	setStatus(store, "T", "completed")

	tasks, _ := GetTasks(store, GetTaskOpts{Status: "completed", Depth: -1})
	if strings.Contains(tasks[0], "overdue") {
		t.Fatalf("completed task should show deadline not overdue: %v", tasks)
	}
	if !strings.Contains(tasks[0], "[deadline: Jan 1]") {
		t.Fatalf("expected [deadline: Jan 1]: %v", tasks)
	}
}

func TestDeadlineInheritedByChildren(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "child", "P", false, false)

	dl := "2026-04-15"
	SetTask(store, "P", SetTaskOpts{Deadline: &dl})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "child") {
			if !strings.Contains(line, "[deadline: Apr 15]") {
				t.Fatalf("child should inherit parent deadline: %v", tasks)
			}
		}
	}
}

func TestChildOwnDeadlineOverridesParent(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "child", "P", false, false)

	pdl := "2026-04-15"
	SetTask(store, "P", SetTaskOpts{Deadline: &pdl})
	cdl := "2026-05-01"
	SetTask(store, "P:1", SetTaskOpts{Deadline: &cdl})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "child") {
			if !strings.Contains(line, "[deadline: May 1]") {
				t.Fatalf("child should use its own deadline: %v", tasks)
			}
		}
	}
}

func TestDeadlineInheritedOnSingleTaskView(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "child", "P", false, false)

	dl := "2026-04-15"
	SetTask(store, "P", SetTaskOpts{Deadline: &dl})

	// View child directly - should show inherited deadline
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "P:1", Depth: -1})
	if !strings.Contains(tasks[0], "[deadline: Apr 15]") {
		t.Fatalf("single task view should show inherited deadline: %v", tasks)
	}
}

func TestInvalidDeadlineFormat(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	dl := "next friday"
	err := SetTask(store, "T", SetTaskOpts{Deadline: &dl})
	if err == nil {
		t.Fatal("expected error for invalid deadline format")
	}
}

func TestSetRecurDaily(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "daily"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[recurs daily]") {
		t.Fatalf("expected [recurs daily]: %v", tasks)
	}
}

func TestSetRecurWeekly(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "weekly mon,thu 09:00"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[recurs weekly mon,thu 09:00]") {
		t.Fatalf("expected [recurs weekly mon,thu 09:00]: %v", tasks)
	}
}

func TestSetRecurInterval(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "every 2d"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[recurs every 2d]") {
		t.Fatalf("expected [recurs every 2d]: %v", tasks)
	}
}

func TestClearRecur(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "daily"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})

	none := "none"
	SetTask(store, "T", SetTaskOpts{Recur: &none})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if strings.Contains(tasks[0], "recurs") {
		t.Fatalf("expected no recurrence after clearing: %v", tasks)
	}
}

func TestInvalidRecurPattern(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	invalid := []string{"hourly", "every", "every 2x", "weekly xyz", "monthly 32", "every 0d"}
	for _, rc := range invalid {
		r := rc
		if err := SetTask(store, "T", SetTaskOpts{Recur: &r}); err == nil {
			t.Fatalf("expected error for invalid recur pattern '%s'", rc)
		}
	}
}

func TestRecurActivatesCompletedTask(t *testing.T) {
	// Test isRecurDue directly with controlled timestamps
	created := "2026-01-01 00:00:00"
	changed := "2026-01-01 00:00:00"
	now, _ := time.Parse("2006-01-02 15:04:05", "2026-01-01 00:05:00") // 5 min later

	if !isRecurDue("every 1min", created, changed, "completed", now) {
		t.Fatal("expected every 1min to be due 5 minutes after creation")
	}
	if !isRecurDue("every 2min", created, changed, "completed", now) {
		t.Fatal("expected every 2min to be due 5 minutes after creation")
	}
	if isRecurDue("every 1h", created, changed, "completed", now.Add(-55*time.Minute)) {
		t.Fatal("expected every 1h to NOT be due 5 minutes after creation")
	}

	// Also test via store integration
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "daily"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})
	setStatus(store, "T", "active")
	setStatus(store, "T", "completed")

	// Manually backdate status_changed_at to yesterday
	store.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE tasks SET status_changed_at = datetime('now', '-2 days') WHERE name = 'T'")
		return err
	})

	ActivateRecurring(store)

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[0], "active") {
		t.Fatalf("expected daily recurring task to be reactivated: %v", tasks)
	}
}

func TestRecurSkipsArchivedTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "every 1min"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})
	setStatus(store, "T", "archived")

	ActivateRecurring(store)

	tasks, _ := GetTasks(store, GetTaskOpts{Status: "archived", Depth: -1})
	if !hasStatus(tasks[0], "archived") {
		t.Fatalf("archived task should not be reactivated: %v", tasks)
	}
}

func TestRecurSkipsDeferredTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "every 1min"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})
	setStatus(store, "T", "active")
	setStatus(store, "T", "deferred")

	ActivateRecurring(store)

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[0], "deferred") {
		t.Fatalf("deferred task should not be reactivated: %v", tasks)
	}
}

func TestRecurDoesNotActivateAlreadyActiveTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "every 1min"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})
	setStatus(store, "T", "active")

	// Should not error or change anything
	ActivateRecurring(store)

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[0], "active") {
		t.Fatalf("active task should remain active: %v", tasks)
	}
}

func TestRecurValidUnits(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	valid := []string{"every 30min", "every 4h", "every 2d", "every 3w", "every 2mon"}
	for _, rc := range valid {
		r := rc
		if err := SetTask(store, "T", SetTaskOpts{Recur: &r}); err != nil {
			t.Fatalf("expected valid recur pattern '%s': %v", rc, err)
		}
	}
}

func TestRecurMonthlyDefault(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "monthly"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "[recurs monthly]") {
		t.Fatalf("expected [recurs monthly]: %v", tasks)
	}
}

func TestCombinedSetTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	SetTask(store, "T", SetTaskOpts{Status: "active", Desc: strPtr("my desc"), Verify: strPtr("my verify")})

	tasks, _ := GetTasks(store, GetTaskOpts{Details: true, Depth: -1})
	if !hasStatus(tasks[0], "active") {
		t.Fatalf("expected active status: %v", tasks)
	}
	hasDesc := false
	hasVerify := false
	for _, line := range tasks {
		if strings.Contains(line, "desc: my desc") {
			hasDesc = true
		}
		if strings.Contains(line, "verify: my verify") {
			hasVerify = true
		}
	}
	if !hasDesc || !hasVerify {
		t.Fatalf("expected combined desc+verify: %v", tasks)
	}
}

func TestSetTaskName(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Old Name", "", false, false)

	SetTask(store, "Old Name", SetTaskOpts{Name: strPtr("New Name")})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "New Name") {
		t.Fatalf("expected renamed task: %v", tasks)
	}
	if strings.Contains(tasks[0], "Old Name") {
		t.Fatalf("old name should not appear: %v", tasks)
	}
}

func TestSetTaskNameTooLong(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	long := strings.Repeat("x", 51)
	if err := SetTask(store, "T", SetTaskOpts{Name: &long}); err == nil {
		t.Fatal("expected error for name > 50 chars")
	}
}

func TestSetTaskNameAndStatus(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	SetTask(store, "T", SetTaskOpts{Name: strPtr("Renamed"), Status: "active"})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !strings.Contains(tasks[0], "Renamed") || !hasStatus(tasks[0], "active") {
		t.Fatalf("expected renamed + active: %v", tasks)
	}
}

func TestChildActiveAutoActivatesParent(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "s1", "P", false, false)
	AddTask(store, "deep", "P:1", false, false)

	setStatus(store, "P:1.1", "active")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[0], "active") {
		t.Fatalf("expected P to auto-activate: %v", tasks)
	}
	if !hasStatus(tasks[1], "active") {
		t.Fatalf("expected s1 to auto-activate: %v", tasks)
	}
}

func TestCannotCompleteWithIncompleteChildren(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "s1", "P", false, false)

	setStatus(store, "P:1", "active")

	if err := setStatus(store, "P", "completed"); err == nil {
		t.Fatal("expected error: child still incomplete")
	}
}

func TestCanCompleteWithCancelledChildren(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "s1", "P", false, false)
	AddTask(store, "s2", "P", false, false)

	setStatus(store, "P:1", "active")
	setStatus(store, "P:1", "completed")
	setStatus(store, "P:2", "active")
	setStatus(store, "P:2", "cancelled")

	if err := setStatus(store, "P", "completed"); err != nil {
		t.Fatalf("should allow completing when children are completed/cancelled: %v", err)
	}
}

func TestReopenChildReopensCompletedParent(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "s1", "P", false, false)

	setStatus(store, "P:1", "active")
	setStatus(store, "P:1", "completed")
	setStatus(store, "P", "completed")

	setStatus(store, "P:1", "reopened")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[0], "reopened") {
		t.Fatalf("expected P to auto-reopen: %v", tasks)
	}
}

func TestNewChildReopensCompletedParent(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "s1", "P", false, false)

	setStatus(store, "P:1", "active")
	setStatus(store, "P:1", "completed")
	setStatus(store, "P", "completed")

	AddTask(store, "s2", "P", false, false)

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[0], "reopened") {
		t.Fatalf("expected P to auto-reopen when new child added: %v", tasks)
	}
}

func TestDeferredParentChildrenInheritDisplay(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "s1", "P", false, false)

	setStatus(store, "P:1", "active")
	setStatus(store, "P", "deferred")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[1], "deferred") {
		t.Fatalf("expected child to inherit deferred: %v", tasks)
	}

	setStatus(store, "P", "active")
	tasks, _ = GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[1], "active") {
		t.Fatalf("expected child to restore to active: %v", tasks)
	}
}

func TestCancelledParentChildrenInheritDisplay(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "s1", "P", false, false)

	setStatus(store, "P:1", "active")
	setStatus(store, "P", "cancelled")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[1], "cancelled") {
		t.Fatalf("expected child to inherit cancelled: %v", tasks)
	}
}

func TestWaitAndDisplay(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "s2", "T", false, false)

	setStatus(store, "T:1", "active")
	setStatus(store, "T:2", "active")
	setWait(store, "T:2", "T:1")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "s2") && strings.Contains(line, "[waiting: 1 ") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected s2 [waiting: 1], got: %v", tasks)
	}
}

func TestAutoTransition(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "s2", "T", false, false)

	setStatus(store, "T:1", "active")
	setStatus(store, "T:2", "active")
	setWait(store, "T:2", "T:1")

	setStatus(store, "T:1", "completed")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "s2") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected s2 to auto-transition to active: %v", tasks)
	}
}

func TestMultipleBlockers(t *testing.T) {
	store := testStore(t)
	AddTask(store, "A", "", false, false)
	AddTask(store, "a1", "A", false, false)
	AddTask(store, "a2", "A", false, false)
	AddTask(store, "a3", "A", false, false)

	setStatus(store, "A:1", "active")
	setStatus(store, "A:2", "active")
	setStatus(store, "A:3", "active")
	setWait(store, "A:3", "A:1", "A:2")

	// Complete only one blocker - should stay waiting
	setStatus(store, "A:1", "completed")
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "a3") && hasStatus(line, "active") {
			t.Fatal("should still be waiting - one blocker remains")
		}
	}

	// Complete second blocker - should auto-transition
	setStatus(store, "A:2", "completed")
	tasks, _ = GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "a3") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a3 to auto-transition: %v", tasks)
	}
}

func TestWaitReplacesBlockers(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "s2", "T", false, false)
	AddTask(store, "s3", "T", false, false)

	setStatus(store, "T:1", "active")
	setStatus(store, "T:2", "active")
	setStatus(store, "T:3", "active")

	// Set wait on s1
	setWait(store, "T:3", "T:1")

	// Replace with s2 - should only wait on s2 now
	setWait(store, "T:3", "T:2")

	// Complete s1 - should NOT auto-transition (s2 is the blocker now)
	setStatus(store, "T:1", "completed")
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "s3") && hasStatus(line, "active") {
			t.Fatal("should still be waiting - blocker was replaced to s2")
		}
	}

	// Complete s2 - NOW should auto-transition
	setStatus(store, "T:2", "completed")
	tasks, _ = GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "s3") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected s3 to auto-transition after blocker replaced: %v", tasks)
	}
}

func TestWaitRequiresBlockers(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	setStatus(store, "T", "active")

	if err := SetTask(store, "T", SetTaskOpts{Status: "wait"}); err == nil {
		t.Fatal("expected error: wait with no blockers")
	}
}

func TestCrossTaskBlocker(t *testing.T) {
	store := testStore(t)
	AddTask(store, "X", "", false, false)
	AddTask(store, "x1", "X", false, false)
	AddTask(store, "Y", "", false, false)
	AddTask(store, "y1", "Y", false, false)

	setStatus(store, "X:1", "active")
	setStatus(store, "Y:1", "active")
	setWait(store, "X:1", "Y:1")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "x1") && strings.Contains(line, "[waiting: Y(1)") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cross-task blocker display: %v", tasks)
	}
}

func TestCannotWaitOnAncestor(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "s1.1", "T:1", false, false)

	setStatus(store, "T:1", "active")
	setStatus(store, "T:1.1", "active")

	if err := setWait(store, "T:1.1", "T:1"); err == nil {
		t.Fatal("expected error: subtask waiting on parent")
	}

	if err := setWait(store, "T:1.1", "T"); err == nil {
		t.Fatal("expected error: subtask waiting on grandparent")
	}

	// Sibling should be fine
	AddTask(store, "s2", "T", false, false)
	setStatus(store, "T:2", "active")
	if err := setWait(store, "T:1.1", "T:2"); err != nil {
		t.Fatalf("sibling wait should work: %v", err)
	}
}

func TestCannotWaitOnSelf(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	setStatus(store, "T", "active")

	if err := setWait(store, "T", "T"); err == nil {
		t.Fatal("expected error when waiting on self")
	}
}

func TestArchiveTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	setStatus(store, "T", "active")

	setStatus(store, "T", "archived")

	// Should not appear in normal gettask
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 0 {
		t.Fatalf("archived task should be hidden: %v", tasks)
	}

	// Should appear with --status archived
	tasks, _ = GetTasks(store, GetTaskOpts{Status: "archived", Depth: -1})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 archived task: %v", tasks)
	}
	if !hasStatus(tasks[0], "archived") {
		t.Fatalf("expected archived status: %v", tasks)
	}
}

func TestArchiveCascadesToChildren(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "deep", "T:1", false, false)
	setStatus(store, "T:1", "active")
	setStatus(store, "T:1.1", "active")

	setStatus(store, "T", "archived")

	// All should be hidden
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 0 {
		t.Fatalf("archived tasks should be hidden: %v", tasks)
	}

	// All should show with --status archived
	tasks, _ = GetTasks(store, GetTaskOpts{Status: "archived", Depth: -1})
	if len(tasks) != 3 {
		t.Fatalf("expected 3 archived tasks, got %d: %v", len(tasks), tasks)
	}
}

func TestArchiveFromAnyStatus(t *testing.T) {
	store := testStore(t)

	// From not_started
	AddTask(store, "T1", "", false, false)
	if err := setStatus(store, "T1", "archived"); err != nil {
		t.Fatalf("archive from not_started: %v", err)
	}

	// From waiting
	AddTask(store, "T2", "", false, false)
	AddTask(store, "B", "", false, false)
	setStatus(store, "T2", "active")
	setStatus(store, "B", "active")
	setWait(store, "T2", "B")
	if err := setStatus(store, "T2", "archived"); err != nil {
		t.Fatalf("archive from waiting: %v", err)
	}

	// From completed
	AddTask(store, "T3", "", false, false)
	setStatus(store, "T3", "active")
	setStatus(store, "T3", "completed")
	if err := setStatus(store, "T3", "archived"); err != nil {
		t.Fatalf("archive from completed: %v", err)
	}

	// From cancelled
	AddTask(store, "T4", "", false, false)
	setStatus(store, "T4", "active")
	setStatus(store, "T4", "cancelled")
	if err := setStatus(store, "T4", "archived"); err != nil {
		t.Fatalf("archive from cancelled: %v", err)
	}

	// From deferred
	AddTask(store, "T5", "", false, false)
	setStatus(store, "T5", "active")
	setStatus(store, "T5", "deferred")
	if err := setStatus(store, "T5", "archived"); err != nil {
		t.Fatalf("archive from deferred: %v", err)
	}
}

func TestUnarchive(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	setStatus(store, "T", "active")
	setStatus(store, "T", "archived")

	// Unarchive back to active
	if err := setStatus(store, "T", "active"); err != nil {
		t.Fatalf("unarchive should work: %v", err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 1 || !hasStatus(tasks[0], "active") {
		t.Fatalf("expected active task: %v", tasks)
	}
}

func TestArchiveCleansUpBlockers(t *testing.T) {
	store := testStore(t)
	// Use two named tasks - one blocks the other
	AddTask(store, "Blocker", "", false, false)
	AddTask(store, "Waiter", "", false, false)

	setStatus(store, "Blocker", "active")
	setStatus(store, "Waiter", "active")
	setWait(store, "Waiter", "Blocker")

	// Archive the blocker - Waiter should auto-transition to active
	setStatus(store, "Blocker", "archived")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "Waiter") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Waiter to auto-transition to active after blocker archived: %v", tasks)
	}
}

func TestArchivePartialBlockerCleanup(t *testing.T) {
	store := testStore(t)
	// Use three named tasks - two block the third
	AddTask(store, "B1", "", false, false)
	AddTask(store, "B2", "", false, false)
	AddTask(store, "Waiter", "", false, false)

	setStatus(store, "B1", "active")
	setStatus(store, "B2", "active")
	setStatus(store, "Waiter", "active")
	setWait(store, "Waiter", "B1", "B2")

	// Archive only one blocker - Waiter should still be waiting
	setStatus(store, "B1", "archived")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "Waiter") && hasStatus(line, "active") {
			t.Fatal("Waiter should still be waiting - one blocker remains")
		}
	}
}

func TestArchiveParentCleansUpChildBlockers(t *testing.T) {
	store := testStore(t)
	// "task one" has subtask "task one:1" which blocks "another:2"
	AddTask(store, "task one", "", false, false)
	AddTask(store, "s1", "task one", false, false)

	AddTask(store, "another", "", false, false)
	AddTask(store, "a1", "another", false, false)
	AddTask(store, "a2", "another", false, false)

	setStatus(store, "task one:1", "active")
	setStatus(store, "another:1", "active")
	setStatus(store, "another:2", "active")
	setWait(store, "another:2", "task one:1")

	// Archive "task one" - cascades to "task one:1", should unblock "another:2"
	if err := setStatus(store, "task one", "archived"); err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "another", Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "a2") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected another:2 to auto-transition to active after parent archived its blocker: %v", tasks)
	}
}

func TestArchiveParentPartialChildBlockerCleanup(t *testing.T) {
	store := testStore(t)
	// "task one:1" and "other" both block "waiter"
	AddTask(store, "task one", "", false, false)
	AddTask(store, "s1", "task one", false, false)

	AddTask(store, "other", "", false, false)
	AddTask(store, "waiter", "", false, false)

	setStatus(store, "task one:1", "active")
	setStatus(store, "other", "active")
	setStatus(store, "waiter", "active")
	setWait(store, "waiter", "task one:1", "other")

	// Archive "task one" - cascades to "task one:1", but "other" still blocks "waiter"
	setStatus(store, "task one", "archived")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "waiter") && hasStatus(line, "active") {
			t.Fatal("waiter should still be waiting - 'other' still blocks it")
		}
	}
	// Verify waiter is still waiting
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "waiter") && strings.Contains(line, "[waiting") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected waiter to still be waiting: %v", tasks)
	}
}

func TestArchiveSubtaskRejected(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "child", "T", false, false)
	setStatus(store, "T:1", "active")

	err := setStatus(store, "T:1", "archived")
	if err == nil {
		t.Fatal("expected error when archiving a subtask directly")
	}
	if !strings.Contains(err.Error(), "only named tasks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)

	if err := DeleteTaskConfirmed(store, "T1"); err != nil {
		t.Fatal(err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after delete, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "T2") {
		t.Fatalf("expected T2 to remain: %v", tasks)
	}
}

func TestDeleteTaskWithChildren(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "deep", "T:1", false, false)

	if err := DeleteTaskConfirmed(store, "T"); err != nil {
		t.Fatal(err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks after delete, got %d: %v", len(tasks), tasks)
	}
}

func TestDeleteSubtask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "s2", "T", false, false)

	if err := DeleteTaskConfirmed(store, "T:1"); err != nil {
		t.Fatal(err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if len(tasks) != 2 { // T + s2
		t.Fatalf("expected 2 lines (T + s2), got %d: %v", len(tasks), tasks)
	}
	for _, line := range tasks {
		if strings.Contains(line, "s1") {
			t.Fatalf("s1 should be deleted: %v", tasks)
		}
	}
}

func TestDeleteCleansUpBlockers(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "s2", "T", false, false)

	setStatus(store, "T:1", "active")
	setStatus(store, "T:2", "active")
	setWait(store, "T:2", "T:1")

	// Delete the blocker - s2 should auto-transition to active
	DeleteTaskConfirmed(store, "T:1")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "s2") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected s2 to auto-transition to active after blocker deleted: %v", tasks)
	}
}

func TestDeletePartialBlockerCleanup(t *testing.T) {
	store := testStore(t)
	AddTask(store, "X", "", false, false)
	AddTask(store, "x1", "X", false, false)
	AddTask(store, "x2", "X", false, false)
	AddTask(store, "x3", "X", false, false)

	setStatus(store, "X:1", "active")
	setStatus(store, "X:2", "active")
	setStatus(store, "X:3", "active")
	setWait(store, "X:3", "X:1", "X:2")

	// Delete only one blocker - x3 should still be waiting
	DeleteTaskConfirmed(store, "X:1")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "x3") && hasStatus(line, "active") {
			t.Fatal("x3 should still be waiting - one blocker remains")
		}
	}
}

func TestDeleteNonexistent(t *testing.T) {
	store := testStore(t)

	if err := DeleteTaskConfirmed(store, "Nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestDeleteTaskInfo(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Parent", "", false, false)
	AddTask(store, "c1", "Parent", false, false)
	AddTask(store, "c2", "Parent", false, false)
	AddTask(store, "gc", "Parent:1", false, false)

	name, childCount, err := DeleteTask(store, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Parent" {
		t.Fatalf("expected name 'Parent', got '%s'", name)
	}
	if childCount != 3 {
		t.Fatalf("expected 3 children, got %d", childCount)
	}
}

func TestDeleteCrossTaskBlocker(t *testing.T) {
	store := testStore(t)
	AddTask(store, "A", "", false, false)
	AddTask(store, "a1", "A", false, false)
	AddTask(store, "B", "", false, false)
	AddTask(store, "b1", "B", false, false)

	setStatus(store, "A:1", "active")
	setStatus(store, "B:1", "active")
	setWait(store, "B:1", "A:1")

	// Delete A entirely - b1 should auto-transition to active
	DeleteTaskConfirmed(store, "A")

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "b1") && hasStatus(line, "active") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected b1 to auto-transition after cross-task blocker deleted: %v", tasks)
	}
}

func TestInheritedTagsOnQueriedTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTask(store, "deep", "T:1", false, false)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")
	AddTag(store, "DESIGN")

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "BACKEND"}})
	SetTask(store, "T:1", SetTaskOpts{SetTags: true, Tags: []string{"DESIGN"}})

	// Query the subtask - should show its own tag + parent tags
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T:1", Depth: -1})
	if !strings.Contains(tasks[0], "#DESIGN") {
		t.Fatalf("expected own tag #DESIGN: %v", tasks)
	}
	if !strings.Contains(tasks[0], "#URGENT") || !strings.Contains(tasks[0], "#BACKEND") {
		t.Fatalf("expected inherited tags #URGENT #BACKEND: %v", tasks)
	}

	// Children of queried task should NOT show inherited tags
	if len(tasks) > 1 && (strings.Contains(tasks[1], "#URGENT") || strings.Contains(tasks[1], "#BACKEND")) {
		t.Fatalf("children should not show inherited tags: %v", tasks)
	}

	// Deep child - inherits from grandparent too
	tasks, _ = GetTasks(store, GetTaskOpts{TaskRef: "T:1.1", Depth: -1})
	if !strings.Contains(tasks[0], "#URGENT") || !strings.Contains(tasks[0], "#BACKEND") || !strings.Contains(tasks[0], "#DESIGN") {
		t.Fatalf("expected all inherited tags on deep child: %v", tasks)
	}
}

func TestInheritedTagsNotShownInAllView(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTag(store, "URGENT")
	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"URGENT"}})

	// --all view - child should NOT show parent's tag
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "s1") && strings.Contains(line, "#URGENT") {
			t.Fatalf("child should not show inherited tag in --all view: %v", tasks)
		}
	}
}

func TestRulesViaTagsInDetails(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTag(store, "BACKEND")
	AddTag(store, "DESIGN")
	AddRule(store, "Code review before merge")
	AddRule(store, "Design approval required")

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})
	SetTask(store, "T:1", SetTaskOpts{SetTags: true, Tags: []string{"DESIGN"}})
	setRuleTags(store, 1, "BACKEND")
	setRuleTags(store, 2, "DESIGN")

	// Query subtask with details - should show rules via inherited + own tags
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T:1", Details: true, Depth: -1})
	hasBackendRule := false
	hasDesignRule := false
	for _, line := range tasks {
		if strings.Contains(line, "Code review before merge") {
			hasBackendRule = true
		}
		if strings.Contains(line, "Design approval required") {
			hasDesignRule = true
		}
	}
	if !hasBackendRule {
		t.Fatalf("expected inherited BACKEND rule: %v", tasks)
	}
	if !hasDesignRule {
		t.Fatalf("expected own DESIGN rule: %v", tasks)
	}
}

func TestRulesNotShownWithoutDetails(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTag(store, "BACKEND")
	AddRule(store, "R1")
	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})
	setRuleTags(store, 1, "BACKEND")

	// Without --details - no rules shown
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T", Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "rules:") {
			t.Fatalf("rules should not show without --details: %v", tasks)
		}
	}
}

func TestNoRulesWhenNoTagsHaveRules(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTag(store, "MYTAG")
	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"MYTAG"}})

	// Tag has no rules - rules section should not appear
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T", Details: true, Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "rules:") {
			t.Fatalf("rules section should not appear when no tags have rules: %v", tasks)
		}
	}
}

func TestChildOwnTagShowsRules(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTag(store, "PARENT")
	AddTag(store, "CHILD")
	AddRule(store, "Parent Rule")
	AddRule(store, "Child Rule")

	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"PARENT"}})
	SetTask(store, "T:1", SetTaskOpts{SetTags: true, Tags: []string{"CHILD"}})
	setRuleTags(store, 1, "PARENT")
	setRuleTags(store, 2, "CHILD")

	// Query parent - should show parent rule, not child rule
	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T", Details: true, Depth: -1})
	hasParentRule := false
	hasChildRule := false
	for _, line := range tasks {
		if strings.Contains(line, "Parent Rule") {
			hasParentRule = true
		}
		if strings.Contains(line, "Child Rule") {
			hasChildRule = true
		}
	}
	if !hasParentRule {
		t.Fatalf("expected parent rule on T: %v", tasks)
	}
	if !hasChildRule {
		t.Fatalf("expected child's own rule on s1: %v", tasks)
	}
}

func TestChildDoesNotShowParentRules(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "s1", "T", false, false)
	AddTag(store, "PTAG")
	AddRule(store, "PR")
	SetTask(store, "T", SetTaskOpts{SetTags: true, Tags: []string{"PTAG"}})
	setRuleTags(store, 1, "PTAG")

	// --all --details: child s1 has no direct tags, so no rules should show on it
	tasks, _ := GetTasks(store, GetTaskOpts{Details: true, Depth: -1})
	// Find lines after s1 - should not have rules:
	foundS1 := false
	for _, line := range tasks {
		if strings.Contains(line, "s1") {
			foundS1 = true
			continue
		}
		if foundS1 && strings.Contains(line, "PR") {
			t.Fatalf("child should not show parent's rules: %v", tasks)
		}
	}
}

func TestDuplicateRootTaskBlocked(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Email boss", "", false, false)

	// Active task blocks creation of same name
	SetTask(store, "Email boss", SetTaskOpts{Status: "active"})
	err := AddTask(store, "Email boss", "", false, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}

	// Completed task also blocks
	SetTask(store, "Email boss", SetTaskOpts{Status: "completed"})
	err = AddTask(store, "Email boss", "", false, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error for completed, got: %v", err)
	}

	// Cancelled task also blocks
	SetTask(store, "Email boss", SetTaskOpts{Status: "reopened"})
	SetTask(store, "Email boss", SetTaskOpts{Status: "cancelled"})
	err = AddTask(store, "Email boss", "", false, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error for cancelled, got: %v", err)
	}
}

func TestDuplicateRootTaskAllowedWhenArchived(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Email boss", "", false, false)
	SetTask(store, "Email boss", SetTaskOpts{Status: "active"})
	SetTask(store, "Email boss", SetTaskOpts{Status: "archived"})

	// Archived task does NOT block
	err := AddTask(store, "Email boss", "", false, false)
	if err != nil {
		t.Fatalf("archived task should not block: %v", err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := 0
	for _, line := range tasks {
		if strings.Contains(line, "Email boss") {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("expected 1 visible task named 'Email boss', got %d: %v", found, tasks)
	}

	// Archived one should show with status filter
	archived, _ := GetTasks(store, GetTaskOpts{Depth: -1, Status: "archived"})
	archivedFound := false
	for _, line := range archived {
		if strings.Contains(line, "Email boss") {
			archivedFound = true
		}
	}
	if !archivedFound {
		t.Fatalf("expected archived 'Email boss' with status filter: %v", archived)
	}
}

func TestDuplicateRootTaskForceArchivesExisting(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Email boss", "", false, false)
	SetTask(store, "Email boss", SetTaskOpts{Status: "active"})
	AddTask(store, "child", "Email boss", false, false)

	// Force archives the existing active task and its children
	err := AddTask(store, "Email boss", "", false, true)
	if err != nil {
		t.Fatalf("force should succeed: %v", err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	newFound := false
	for _, line := range tasks {
		if strings.Contains(line, "Email boss") && hasStatus(line, "not_started") {
			newFound = true
		}
	}
	if !newFound {
		t.Fatalf("expected new task as not_started: %v", tasks)
	}

	// Old task should be archived
	archived, _ := GetTasks(store, GetTaskOpts{Depth: -1, Status: "archived"})
	archivedFound := false
	for _, line := range archived {
		if strings.Contains(line, "Email boss") {
			archivedFound = true
		}
	}
	if !archivedFound {
		t.Fatalf("expected old task to be archived: %v", archived)
	}
}

func TestSettaskRenameBlockedByDuplicate(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Task A", "", false, false)
	AddTask(store, "Task B", "", false, false)
	SetTask(store, "Task A", SetTaskOpts{Status: "active"})

	// Renaming B to A should fail (A is active)
	name := "Task A"
	err := SetTask(store, "Task B", SetTaskOpts{Name: &name})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error on rename, got: %v", err)
	}

	// Force rename archives the conflicting task
	err = SetTask(store, "Task B", SetTaskOpts{Name: &name, Force: true})
	if err != nil {
		t.Fatalf("force rename should succeed: %v", err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	renamedFound := false
	for _, line := range tasks {
		if strings.Contains(line, "Task A") && hasStatus(line, "not_started") {
			renamedFound = true
		}
	}
	if !renamedFound {
		t.Fatalf("expected renamed task to exist: %v", tasks)
	}

	// Old Task A should be archived
	archived, _ := GetTasks(store, GetTaskOpts{Depth: -1, Status: "archived"})
	archivedFound := false
	for _, line := range archived {
		if strings.Contains(line, "Task A") {
			archivedFound = true
		}
	}
	if !archivedFound {
		t.Fatalf("expected old 'Task A' to be archived: %v", archived)
	}
}

func TestSubtaskDuplicateBlocked(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "same", "T", false, false)

	err := AddTask(store, "same", "T", false, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error for subtask, got: %v", err)
	}
}

func TestSubtaskDuplicateCaseInsensitive(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "Setup", "T", false, false)

	err := AddTask(store, "setup", "T", false, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected case-insensitive duplicate error, got: %v", err)
	}
}

func TestSubtaskDuplicateAllowedUnderDifferentParents(t *testing.T) {
	store := testStore(t)
	AddTask(store, "A", "", false, false)
	AddTask(store, "B", "", false, false)
	AddTask(store, "same", "A", false, false)

	err := AddTask(store, "same", "B", false, false)
	if err != nil {
		t.Fatalf("same name under different parents should be allowed: %v", err)
	}
}

func TestSubtaskDuplicateAllowedWhenArchived(t *testing.T) {
	store := testStore(t)
	// Create T with a child, then force-recreate T (archives old T and its children)
	AddTask(store, "T", "", false, false)
	AddTask(store, "child", "T", false, false)
	AddTask(store, "T", "", false, true) // force archives old T + its children

	// Adding "child" under new T should succeed since old "child" is archived
	err := AddTask(store, "child", "T", false, false)
	if err != nil {
		t.Fatalf("archived subtask should not block: %v", err)
	}
}

func TestSubtaskDuplicateForceArchivesExisting(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "child", "T", false, false)

	err := AddTask(store, "child", "T", false, true)
	if err != nil {
		t.Fatalf("force should succeed: %v", err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "T", Depth: -1})
	combined := strings.Join(tasks, "\n")
	// Should have one visible child, not two
	count := strings.Count(combined, "child")
	if count != 1 {
		t.Fatalf("expected 1 visible 'child', got %d in:\n%s", count, combined)
	}
}

func TestSubtaskDuplicateCrossParallelSequential(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "Setup", "T", false, false) // sequential: 1. Setup

	// Adding parallel with same name should be blocked
	err := AddTask(store, "Setup", "T", true, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected cross-type duplicate error, got: %v", err)
	}
}

func TestSettaskRenameSubtaskBlockedByDuplicate(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	AddTask(store, "alpha", "T", false, false)
	AddTask(store, "beta", "T", false, false)

	name := "alpha"
	err := SetTask(store, "T:2", SetTaskOpts{Name: &name})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error on subtask rename, got: %v", err)
	}

	// Force rename archives the conflicting sibling
	err = SetTask(store, "T:2", SetTaskOpts{Name: &name, Force: true})
	if err != nil {
		t.Fatalf("force rename should succeed: %v", err)
	}
}

func TestRootDuplicateCaseInsensitive(t *testing.T) {
	store := testStore(t)
	AddTask(store, "MyTask", "", false, false)

	err := AddTask(store, "mytask", "", false, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected case-insensitive duplicate error for root, got: %v", err)
	}
}

func TestParallelSubtaskPositions(t *testing.T) {
	store := testStore(t)
	AddTask(store, "P", "", false, false)
	AddTask(store, "first", "P", true, false)
	AddTask(store, "second", "P", true, false)
	AddTask(store, "third", "P", true, false)

	tasks, _ := GetTasks(store, GetTaskOpts{TaskRef: "P", Depth: -1})
	combined := strings.Join(tasks, "\n")
	if !strings.Contains(combined, "a.") || !strings.Contains(combined, "b.") || !strings.Contains(combined, "c.") {
		t.Fatalf("expected lettered positions a, b, c in:\n%s", combined)
	}
}

func TestMaxTaskNameLen(t *testing.T) {
	store := testStore(t)
	longName := strings.Repeat("x", maxTaskNameLen+1)
	err := AddTask(store, longName, "", false, false)
	if err == nil {
		t.Fatal("expected error for name exceeding maxTaskNameLen")
	}
	if !strings.Contains(err.Error(), "50") {
		t.Fatalf("error should mention limit, got: %v", err)
	}

	exactName := strings.Repeat("x", maxTaskNameLen)
	err = AddTask(store, exactName, "", false, false)
	if err != nil {
		t.Fatalf("name at exactly maxTaskNameLen should be allowed: %v", err)
	}
}

func TestGetTaskFilterByMultipleTags(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "T3", "", false, false)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")
	AddTag(store, "INFRA")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "BACKEND"}})
	SetTask(store, "T2", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "INFRA"}})
	SetTask(store, "T3", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})

	tasks, err := GetTasks(store, GetTaskOpts{Tags: []string{"URGENT", "BACKEND"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task with URGENT+BACKEND, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "T1") {
		t.Fatalf("expected T1, got: %v", tasks)
	}
}

func TestGetTaskFilterByNotTag(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "T3", "", false, false)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"URGENT"}})
	SetTask(store, "T2", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "BACKEND"}})
	SetTask(store, "T3", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})

	tasks, err := GetTasks(store, GetTaskOpts{NotTags: []string{"URGENT"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 non-urgent task, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "T3") {
		t.Fatalf("expected T3, got: %v", tasks)
	}
}

func TestGetTaskFilterByMultipleNotTags(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "T3", "", false, false)
	AddTask(store, "T4", "", false, false)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"URGENT"}})
	SetTask(store, "T2", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})
	SetTask(store, "T3", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "BACKEND"}})

	tasks, err := GetTasks(store, GetTaskOpts{NotTags: []string{"URGENT", "BACKEND"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task with neither tag, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "T4") {
		t.Fatalf("expected T4, got: %v", tasks)
	}
}

func TestGetTaskFilterByTagAndNotTag(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "T3", "", false, false)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "BACKEND"}})
	SetTask(store, "T2", SetTaskOpts{SetTags: true, Tags: []string{"URGENT"}})
	SetTask(store, "T3", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})

	tasks, err := GetTasks(store, GetTaskOpts{Tags: []string{"URGENT"}, NotTags: []string{"BACKEND"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "T2") {
		t.Fatalf("expected T2, got: %v", tasks)
	}
}

func TestGetTaskFilterNotTagNone(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Tagged", "", false, false)
	AddTask(store, "Untagged", "", false, false)
	AddTag(store, "X")
	SetTask(store, "Tagged", SetTaskOpts{SetTags: true, Tags: []string{"X"}})

	tasks, err := GetTasks(store, GetTaskOpts{NotTags: []string{"NONE"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 tagged task, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "Tagged") {
		t.Fatalf("expected Tagged, got: %v", tasks)
	}
}

func TestGetTaskFilterNotTagInheritance(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Parent", "", false, false)
	AddTask(store, "child", "Parent", false, false)
	AddTask(store, "Other", "", false, false)
	AddTag(store, "X")
	SetTask(store, "Parent", SetTaskOpts{SetTags: true, Tags: []string{"X"}})

	// Parent has X, child inherits X -- both excluded
	tasks, err := GetTasks(store, GetTaskOpts{NotTags: []string{"X"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "Other") {
		t.Fatalf("expected Other, got: %v", tasks)
	}
}

func TestGetTaskFilterNotTagChildDirectlyTagged(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Parent", "", false, false)
	AddTask(store, "child", "Parent", false, false)
	AddTask(store, "Other", "", false, false)
	AddTag(store, "X")
	// Parent has NO tags, but child is directly tagged X
	SetTask(store, "Parent:1", SetTaskOpts{SetTags: true, Tags: []string{"X"}})

	// Exclude X -- child has X directly, parent does not
	// Parent and Other should show, child should NOT
	tasks, err := GetTasks(store, GetTaskOpts{NotTags: []string{"X"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range tasks {
		if strings.Contains(line, "child") {
			t.Fatalf("child with direct tag X should be excluded by nottag: %v", tasks)
		}
	}
	found := 0
	for _, line := range tasks {
		if strings.Contains(line, "Parent") || strings.Contains(line, "Other") {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected Parent and Other, got: %v", tasks)
	}
}

func TestGetTaskFilterNotTagWithStatusCombo(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "T3", "", false, false)
	AddTag(store, "DONE")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"DONE"}})
	setStatus(store, "T1", "active")
	setStatus(store, "T2", "active")

	tasks, err := GetTasks(store, GetTaskOpts{NotTags: []string{"DONE"}, Status: "active", Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "T2") {
		t.Fatalf("expected T2, got: %v", tasks)
	}
}

func TestGetTaskFilterByNonexistentTagErrors(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTag(store, "REAL")

	// Filtering by a tag that doesn't exist should error
	_, err := GetTasks(store, GetTaskOpts{Tags: []string{"NOPE"}, Depth: -1})
	if err == nil {
		t.Fatal("expected error for nonexistent tag")
	}
	if !strings.Contains(err.Error(), "NOPE") {
		t.Fatalf("error should mention the tag name: %v", err)
	}

	// Same for nottag
	_, err = GetTasks(store, GetTaskOpts{NotTags: []string{"NOPE"}, Depth: -1})
	if err == nil {
		t.Fatal("expected error for nonexistent nottag")
	}

	// NONE should not be checked for existence
	_, err = GetTasks(store, GetTaskOpts{Tags: []string{"NONE"}, Depth: -1})
	if err != nil {
		t.Fatalf("NONE should not require existence check: %v", err)
	}

	// Real tag should work fine
	_, err = GetTasks(store, GetTaskOpts{Tags: []string{"REAL"}, Depth: -1})
	if err != nil {
		t.Fatalf("existing tag should work: %v", err)
	}
}

func TestGetTaskFilterByCommaSeparatedTags(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T1", "", false, false)
	AddTask(store, "T2", "", false, false)
	AddTask(store, "T3", "", false, false)
	AddTag(store, "URGENT")
	AddTag(store, "BACKEND")

	SetTask(store, "T1", SetTaskOpts{SetTags: true, Tags: []string{"URGENT", "BACKEND"}})
	SetTask(store, "T2", SetTaskOpts{SetTags: true, Tags: []string{"URGENT"}})
	SetTask(store, "T3", SetTaskOpts{SetTags: true, Tags: []string{"BACKEND"}})

	// Simulate CLI passing "URGENT,BACKEND" as a single element (comma-separated)
	tasks, err := GetTasks(store, GetTaskOpts{Tags: []string{"URGENT,BACKEND"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task with both tags via comma input, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "T1") {
		t.Fatalf("expected T1, got: %v", tasks)
	}

	// Same for nottag: "URGENT,BACKEND" as single element excludes tasks with either
	tasks, err = GetTasks(store, GetTaskOpts{NotTags: []string{"URGENT,BACKEND"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	// T1 has both, T2 has URGENT, T3 has BACKEND -- all excluded
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d: %v", len(tasks), tasks)
	}
}

func TestGetTaskFilterNotTagDeepNesting(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Grandparent", "", false, false)
	AddTask(store, "Parent", "Grandparent", false, false)
	AddTask(store, "Child", "Grandparent:1", false, false)
	AddTask(store, "Other", "", false, false)
	AddTag(store, "X")
	AddTag(store, "Y")

	// Tag grandparent with X
	SetTask(store, "Grandparent", SetTaskOpts{SetTags: true, Tags: []string{"X"}})

	// --nottag X should exclude grandparent, parent (inherits X), and child (inherits X)
	tasks, err := GetTasks(store, GetTaskOpts{NotTags: []string{"X"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
	}
	if !strings.Contains(tasks[0], "Other") {
		t.Fatalf("expected Other, got: %v", tasks)
	}

	// Now also tag child directly with Y
	SetTask(store, "Grandparent:1.1", SetTaskOpts{SetTags: true, Tags: []string{"Y"}})

	// --tag X --nottag Y: grandparent has X (match), parent inherits X (match),
	// child inherits X (match) BUT also has Y (excluded)
	tasks, err = GetTasks(store, GetTaskOpts{Tags: []string{"X"}, NotTags: []string{"Y"}, Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	// Grandparent and Parent should show, Child should not
	for _, line := range tasks {
		if strings.Contains(line, "Child") {
			t.Fatalf("Child has tag Y and should be excluded: %v", tasks)
		}
	}
	found := 0
	for _, line := range tasks {
		if strings.Contains(line, "Grandparent") || strings.Contains(line, "Parent") {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("expected Grandparent and Parent, got: %v", tasks)
	}
}

func TestVerifyStatusDefaultEmpty(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	tasks, _ := GetTasks(store, GetTaskOpts{Details: true, Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "verify status:") {
			t.Fatalf("verify status should not appear when empty: %v", tasks)
		}
	}
}

func TestVerifyPendingBadge(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "[verify: pending]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected [verify: pending] badge: %v", tasks)
	}
}

func TestVerifyPassedBadge(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})
	passed := "passed"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "[verify: passed]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected [verify: passed] badge: %v", tasks)
	}
}

func TestVerifyAutoSetsPending(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "[verify: pending]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected [verify: pending] after setting criteria: %v", tasks)
	}
}

func TestVerifyBlocksCompletion(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	err := SetTask(store, "T", SetTaskOpts{Status: "completed"})
	if err == nil {
		t.Fatal("expected error when completing task with pending verification")
	}
	if !strings.Contains(err.Error(), "verification not passed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPassedAllowsCompletion(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	passed := "passed"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})

	err := SetTask(store, "T", SetTaskOpts{Status: "completed"})
	if err != nil {
		t.Fatalf("expected completion to succeed after passing verification: %v", err)
	}
}

func TestVerifyStatusResetOnNewCriteria(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check A")})

	passed := "passed"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check B")})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "[verify: pending]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected pending after setting new criteria: %v", tasks)
	}
}

func TestVerifyClearWithNone(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	none := "none"
	SetTask(store, "T", SetTaskOpts{Verify: &none})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "verify") {
			t.Fatalf("expected no verify after clearing: %v", tasks)
		}
	}

	err := SetTask(store, "T", SetTaskOpts{Status: "completed"})
	if err != nil {
		t.Fatalf("expected completion after clearing verify: %v", err)
	}
}

func TestVerifyClearCaseInsensitive(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	none := "NONE"
	SetTask(store, "T", SetTaskOpts{Verify: &none})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	for _, line := range tasks {
		if strings.Contains(line, "verify") {
			t.Fatalf("expected no verify after clearing with NONE: %v", tasks)
		}
	}
}

func TestVerifyStatusWithoutCriteriaErrors(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})

	passed := "passed"
	err := SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})
	if err == nil {
		t.Fatal("expected error when setting verify status without criteria")
	}
	if !strings.Contains(err.Error(), "no verification criteria") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyResetOnReopen(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	passed := "passed"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})
	SetTask(store, "T", SetTaskOpts{Status: "completed"})
	SetTask(store, "T", SetTaskOpts{Status: "reopened"})

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "[verify: pending]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected pending after reopen: %v", tasks)
	}
}

func TestNoVerifyGateWithoutCriteria(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})

	err := SetTask(store, "T", SetTaskOpts{Status: "completed"})
	if err != nil {
		t.Fatalf("tasks without verify criteria should complete freely: %v", err)
	}
}

func TestVerifyStatusPendingRevert(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	passed := "passed"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})
	pending := "pending"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &pending})

	err := SetTask(store, "T", SetTaskOpts{Status: "completed"})
	if err == nil {
		t.Fatal("expected completion blocked after reverting to pending")
	}
}

func TestVerifyStatusExportImport(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})
	passed := "passed"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})

	// Export
	data, err := Export(store)
	if err != nil {
		t.Fatal(err)
	}
	if data.Tasks[0].VerifyStatus != "passed" {
		t.Fatalf("expected passed in export, got: %s", data.Tasks[0].VerifyStatus)
	}

	// Import into fresh store
	store2 := testStore(t)
	if err := Import(store2, data, false); err != nil {
		t.Fatal(err)
	}
	tasks, _ := GetTasks(store2, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "[verify: passed]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected [verify: passed] after import: %v", tasks)
	}
}

func TestVerifyStatusImportOldExport(t *testing.T) {
	store := testStore(t)
	data := &ExportData{
		Version: 1,
		Tasks: []ExportTask{
			{
				Name:            "T",
				Status:          "active",
				Verification:    "check output",
				VerifyStatus:    "",
				CreatedAt:       "2026-01-01T00:00:00Z",
				StatusChangedAt: "2026-01-01T00:00:00Z",
			},
		},
	}
	if err := Import(store, data, false); err != nil {
		t.Fatal(err)
	}

	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "[verify: pending]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected [verify: pending] for old export with criteria: %v", tasks)
	}
}

func TestVerifyStatusInDetails(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	tasks, _ := GetTasks(store, GetTaskOpts{Details: true, Depth: -1})
	hasVerify := false
	hasStatus := false
	for _, line := range tasks {
		if strings.Contains(line, "verify: check output") {
			hasVerify = true
		}
		if strings.Contains(line, "verify status: pending") {
			hasStatus = true
		}
	}
	if !hasVerify {
		t.Fatalf("expected verify criteria in details: %v", tasks)
	}
	if !hasStatus {
		t.Fatalf("expected verify status in details: %v", tasks)
	}
}

func TestClearVerifyOnCompletedTask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	passed := "passed"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})
	SetTask(store, "T", SetTaskOpts{Status: "completed"})

	// Clear verify on completed task - should be allowed
	none := "none"
	err := SetTask(store, "T", SetTaskOpts{Verify: &none})
	if err != nil {
		t.Fatalf("clearing verify on completed task should be allowed: %v", err)
	}

	// Task should still be completed
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[0], "completed") {
		t.Fatalf("task should still be completed: %v", tasks)
	}
	for _, line := range tasks {
		if strings.Contains(line, "verify") {
			t.Fatalf("verify should be cleared: %v", tasks)
		}
	}
}

func TestRecurResetsVerifyStatus(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)

	rc := "daily"
	SetTask(store, "T", SetTaskOpts{Recur: &rc})
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	passed := "passed"
	SetTask(store, "T", SetTaskOpts{VerifyStatus: &passed})
	SetTask(store, "T", SetTaskOpts{Status: "completed"})

	// Backdate so recurrence fires
	store.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE tasks SET status_changed_at = datetime('now', '-2 days') WHERE name = 'T'")
		return err
	})

	ActivateRecurring(store)

	// Task should be active again with verify reset to pending
	tasks, _ := GetTasks(store, GetTaskOpts{Depth: -1})
	if !hasStatus(tasks[0], "active") {
		t.Fatalf("expected reactivated: %v", tasks)
	}
	found := false
	for _, line := range tasks {
		if strings.Contains(line, "[verify: pending]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected verify reset to pending after recurrence: %v", tasks)
	}

	// Should block completion again
	err := SetTask(store, "T", SetTaskOpts{Status: "completed"})
	if err == nil {
		t.Fatal("expected completion blocked after recurrence reset")
	}
}

func TestVerifyCombinedFlagsRejected(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})

	// Setting both --verify and --verify-status in one call should be rejected
	passed := "passed"
	err := SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output"), VerifyStatus: &passed})
	if err == nil {
		t.Fatal("expected error when setting both --verify and --verify-status")
	}
	if !strings.Contains(err.Error(), "cannot set --verify and --verify-status in the same call") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyInvalidStatusRejected(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	garbage := "garbage"
	err := SetTask(store, "T", SetTaskOpts{VerifyStatus: &garbage})
	if err == nil {
		t.Fatal("expected error for invalid verify status")
	}
	if !strings.Contains(err.Error(), "invalid verify status") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelWithPendingVerification(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	// Cancel should work even with pending verification - gate only blocks completion
	err := SetTask(store, "T", SetTaskOpts{Status: "cancelled"})
	if err != nil {
		t.Fatalf("cancelling with pending verification should be allowed: %v", err)
	}
}

func TestCorruptedVerifyStatusBlocksCompletion(t *testing.T) {
	store := testStore(t)
	AddTask(store, "T", "", false, false)
	SetTask(store, "T", SetTaskOpts{Status: "active"})
	SetTask(store, "T", SetTaskOpts{Verify: strPtr("check output")})

	// Simulate corruption by writing garbage directly to DB
	store.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE tasks SET verify_status = 'pendng' WHERE name = 'T'")
		return err
	})

	err := SetTask(store, "T", SetTaskOpts{Status: "completed"})
	if err == nil {
		t.Fatal("corrupted verify_status should still block completion")
	}
}
