package main

import (
	"strings"
	"testing"
)

func TestAddAndGetRules(t *testing.T) {
	store := testStore(t)

	AddRule(store, "Rule A")
	AddRule(store, "Rule B")

	rules, _ := GetRules(store, GetRuleOpts{})
	if len(rules) != 2 || rules[0] != "Rule 1: Rule A" || rules[1] != "Rule 2: Rule B" {
		t.Fatalf("got %v", rules)
	}
}

func TestRuleSeqAutoAssigned(t *testing.T) {
	store := testStore(t)
	AddRule(store, "First")
	AddRule(store, "Second")
	AddRule(store, "Third")

	rules, _ := GetRules(store, GetRuleOpts{})
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0] != "Rule 1: First" || rules[1] != "Rule 2: Second" || rules[2] != "Rule 3: Third" {
		t.Fatalf("unexpected format: %v", rules)
	}
}

func TestAddRuleReturnsSeq(t *testing.T) {
	store := testStore(t)
	seq1, _ := AddRule(store, "First")
	seq2, _ := AddRule(store, "Second")
	if seq1 != 1 || seq2 != 2 {
		t.Fatalf("expected seq 1, 2, got %d, %d", seq1, seq2)
	}
}

func TestGetRuleBySeq(t *testing.T) {
	store := testStore(t)
	AddRule(store, "First")
	AddRule(store, "Second")
	AddRule(store, "Third")

	rules, _ := GetRules(store, GetRuleOpts{Seq: 2})
	if len(rules) != 1 || rules[0] != "Rule 2: Second" {
		t.Fatalf("expected Rule 2: Second, got %v", rules)
	}
}

func TestGetRuleBySeqNotFound(t *testing.T) {
	store := testStore(t)
	AddRule(store, "Only")

	_, err := GetRules(store, GetRuleOpts{Seq: 99})
	if err == nil {
		t.Fatal("expected error for nonexistent rule seq")
	}
}

func TestGetRuleBySeqShowsTags(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	AddTag(store, "BACKEND")
	setRuleTags(store, 1, "BACKEND")

	rules, _ := GetRules(store, GetRuleOpts{Seq: 1})
	if !strings.Contains(rules[0], "#BACKEND") {
		t.Fatalf("expected tags on single rule: %v", rules)
	}
}

func TestGetRuleAllShowsTags(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	AddTag(store, "BACKEND")
	setRuleTags(store, 1, "BACKEND")

	rules, _ := GetRules(store, GetRuleOpts{})
	if !strings.Contains(rules[0], "#BACKEND") {
		t.Fatalf("expected tags in --all mode: %v", rules)
	}
}

func TestGetRuleByTag(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	AddRule(store, "R2")
	AddRule(store, "R3")
	AddTag(store, "BACKEND")
	AddTag(store, "FRONTEND")

	setRuleTags(store, 1, "BACKEND")
	setRuleTags(store, 2, "BACKEND", "FRONTEND")

	// Filter by BACKEND - R1 and R2, no tags shown
	rules, _ := GetRules(store, GetRuleOpts{Tag: "BACKEND"})
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d: %v", len(rules), rules)
	}
	for _, r := range rules {
		if strings.Contains(r, "#") {
			t.Fatalf("tags should not be shown in --tag mode: %v", rules)
		}
	}
}

func TestGetRuleByTagNone(t *testing.T) {
	store := testStore(t)
	AddRule(store, "Tagged")
	AddRule(store, "Untagged")
	AddTag(store, "X")
	setRuleTags(store, 1, "X")

	rules, _ := GetRules(store, GetRuleOpts{Tag: "NONE"})
	if len(rules) != 1 {
		t.Fatalf("expected 1 untagged rule, got %d: %v", len(rules), rules)
	}
	if !strings.Contains(rules[0], "Untagged") {
		t.Fatalf("expected untagged rule: %v", rules)
	}
	if strings.Contains(rules[0], "#") {
		t.Fatalf("tags should not be shown in --tag mode: %v", rules)
	}
}

func TestGetRuleTagAll(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	AddRule(store, "R2")
	AddRule(store, "R3")
	AddTag(store, "BACKEND")
	AddTag(store, "FRONTEND")

	setRuleTags(store, 1, "BACKEND")
	setRuleTags(store, 2, "FRONTEND")
	// R3 is untagged

	rules, _ := GetRules(store, GetRuleOpts{TagAll: true})
	// #BACKEND, Rule 1: R1, #FRONTEND, Rule 2: R2, #NONE, Rule 3: R3
	if len(rules) != 6 {
		t.Fatalf("expected 6 lines, got %d: %v", len(rules), rules)
	}
	if rules[0] != "#BACKEND" || rules[2] != "#FRONTEND" || rules[4] != "#NONE" {
		t.Fatalf("expected tag headers (alpha then NONE last): %v", rules)
	}
	// No tags on rule lines in grouped view
	for _, r := range rules {
		if strings.HasPrefix(r, "  Rule") && strings.Contains(r, "#") {
			t.Fatalf("tags should not be shown in --tag --all mode: %v", rules)
		}
	}
}

func TestGetRuleTagAllWithRepeats(t *testing.T) {
	store := testStore(t)
	AddRule(store, "Multi")
	AddTag(store, "BACKEND")
	AddTag(store, "FRONTEND")
	setRuleTags(store, 1, "BACKEND", "FRONTEND")

	rules, _ := GetRules(store, GetRuleOpts{TagAll: true})
	// #BACKEND, Rule 1: Multi, #FRONTEND, Rule 1: Multi
	if len(rules) != 4 {
		t.Fatalf("expected 4 lines (rule appears under both tags), got %d: %v", len(rules), rules)
	}
}

func TestGetRuleRequiresFlag(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")

	// Empty opts (no --all, no seq, no --tag) still returns all rules
	// because service layer treats empty opts as "all"
	rules, _ := GetRules(store, GetRuleOpts{})
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule: %v", rules)
	}
}

func TestGetRuleTagAllNoRules(t *testing.T) {
	store := testStore(t)

	rules, _ := GetRules(store, GetRuleOpts{TagAll: true})
	if len(rules) != 0 {
		t.Fatalf("expected 0 lines for empty rules: %v", rules)
	}
}

func TestSetRuleName(t *testing.T) {
	store := testStore(t)
	AddRule(store, "Original")

	SetRule(store, 1, SetRuleOpts{Name: strPtr("Updated")})

	rules, _ := GetRules(store, GetRuleOpts{Seq: 1})
	if rules[0] != "Rule 1: Updated" {
		t.Fatalf("expected renamed rule: %v", rules)
	}
}

func TestSetRuleTagReplace(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	AddTag(store, "A")
	AddTag(store, "B")
	AddTag(store, "C")

	setRuleTags(store, 1, "A", "B")
	rules, _ := GetRules(store, GetRuleOpts{})
	if !strings.Contains(rules[0], "#A") || !strings.Contains(rules[0], "#B") {
		t.Fatalf("expected #A #B: %v", rules)
	}

	setRuleTags(store, 1, "C")
	rules, _ = GetRules(store, GetRuleOpts{})
	if !strings.Contains(rules[0], "#C") {
		t.Fatalf("expected #C: %v", rules)
	}
	if strings.Contains(rules[0], "#A") || strings.Contains(rules[0], "#B") {
		t.Fatalf("old tags should be replaced: %v", rules)
	}

	setRuleTags(store, 1)
	rules, _ = GetRules(store, GetRuleOpts{})
	if strings.Contains(rules[0], "#") {
		t.Fatalf("expected no tags: %v", rules)
	}
}

func TestSetRuleNotFound(t *testing.T) {
	store := testStore(t)

	if err := SetRule(store, 99, SetRuleOpts{Name: strPtr("x")}); err == nil {
		t.Fatal("expected error for nonexistent rule")
	}
}

func TestSetRuleNameAndTags(t *testing.T) {
	store := testStore(t)
	AddRule(store, "Old")
	AddTag(store, "X")

	SetRule(store, 1, SetRuleOpts{Name: strPtr("New"), SetTags: true, Tags: []string{"X"}})

	rules, _ := GetRules(store, GetRuleOpts{Seq: 1})
	if rules[0] != "Rule 1: New #X" {
		t.Fatalf("expected renamed + tagged: %v", rules)
	}
}

func TestSetRuleTagSyncsCounts(t *testing.T) {
	store := testStore(t)
	AddTag(store, "POLICY")
	AddRule(store, "R1")

	tags, _ := GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "POLICY") || !strings.Contains(tags[0], "unused") {
		t.Fatalf("expected no counts before tagging: %v", tags)
	}

	setRuleTags(store, 1, "POLICY")
	tags, _ = GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "POLICY") || !strings.Contains(tags[0], "1 rule") {
		t.Fatalf("expected '1 rule' after tagging: %v", tags)
	}

	// Clear tags - counts should revert
	setRuleTags(store, 1)
	tags, _ = GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "POLICY") || !strings.Contains(tags[0], "unused") {
		t.Fatalf("expected no counts after clearing: %v", tags)
	}
}

func TestDeleteRule(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	AddRule(store, "R2")
	AddRule(store, "R3")

	DeleteRule(store, 2)

	// R2 should be hidden
	rules, _ := GetRules(store, GetRuleOpts{})
	if len(rules) != 2 {
		t.Fatalf("expected 2 visible rules, got %d: %v", len(rules), rules)
	}
	if strings.Contains(rules[0], "R2") || strings.Contains(rules[1], "R2") {
		t.Fatalf("deleted rule should be hidden: %v", rules)
	}
	// Seq numbers preserved
	if rules[0] != "Rule 1: R1" || rules[1] != "Rule 3: R3" {
		t.Fatalf("unexpected rules: %v", rules)
	}
}

func TestDeleteRuleBySeqNotFound(t *testing.T) {
	store := testStore(t)
	if err := DeleteRule(store, 99); err == nil {
		t.Fatal("expected error for nonexistent rule")
	}
}

func TestDeleteRuleTwice(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	DeleteRule(store, 1)

	// Deleting again should fail (already empty)
	if err := DeleteRule(store, 1); err == nil {
		t.Fatal("expected error for already deleted rule")
	}
}

func TestDeleteRuleClearsTags(t *testing.T) {
	store := testStore(t)
	AddTag(store, "BACKEND")
	AddRule(store, "R1", "BACKEND")

	DeleteRule(store, 1)

	// Tag should no longer show rule count
	tags, _ := GetTags(store, GetTagOpts{})
	if !strings.Contains(tags[0], "BACKEND") || !strings.Contains(tags[0], "unused") {
		t.Fatalf("expected no counts after deleting only rule with that tag: %v", tags)
	}
}

func TestDeletedRuleHiddenInGetruleBySeq(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	DeleteRule(store, 1)

	_, err := GetRules(store, GetRuleOpts{Seq: 1})
	if err == nil {
		t.Fatal("expected error for deleted rule by seq")
	}
}

func TestDeletedRuleHiddenInGetruleByTag(t *testing.T) {
	store := testStore(t)
	AddTag(store, "X")
	AddRule(store, "R1", "X")
	AddRule(store, "R2", "X")
	DeleteRule(store, 1)

	rules, _ := GetRules(store, GetRuleOpts{Tag: "X"})
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d: %v", len(rules), rules)
	}
}

func TestDeletedRuleHiddenInTagAll(t *testing.T) {
	store := testStore(t)
	AddTag(store, "X")
	AddRule(store, "R1", "X")
	AddRule(store, "R2")
	DeleteRule(store, 2)

	rules, _ := GetRules(store, GetRuleOpts{TagAll: true})
	// Should only show #X group with R1, no NONE group (R2 is deleted)
	for _, r := range rules {
		if strings.Contains(r, "R2") {
			t.Fatalf("deleted rule should not appear: %v", rules)
		}
	}
}

func TestAddRuleReusesEmptySlot(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	AddRule(store, "R2")
	AddRule(store, "R3")
	DeleteRule(store, 2)

	// Next addrule should reuse slot 2
	seq, _ := AddRule(store, "New Rule")
	if seq != 2 {
		t.Fatalf("expected reused seq 2, got %d", seq)
	}

	rules, _ := GetRules(store, GetRuleOpts{})
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d: %v", len(rules), rules)
	}
	if rules[1] != "Rule 2: New Rule" {
		t.Fatalf("expected slot 2 reused: %v", rules)
	}
}

func TestAddRuleReusesLowestEmptySlot(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")
	AddRule(store, "R2")
	AddRule(store, "R3")
	DeleteRule(store, 1)
	DeleteRule(store, 3)

	// Should reuse slot 1 (lowest)
	seq, _ := AddRule(store, "New")
	if seq != 1 {
		t.Fatalf("expected reused seq 1, got %d", seq)
	}
}

func TestAddRuleEmptyNameRejected(t *testing.T) {
	store := testStore(t)
	_, err := AddRule(store, "")
	if err == nil {
		t.Fatal("expected error for empty rule name")
	}
	_, err = AddRule(store, "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only rule name")
	}
}

func TestSetRuleEmptyNameRejected(t *testing.T) {
	store := testStore(t)
	AddRule(store, "R1")

	empty := ""
	if err := SetRule(store, 1, SetRuleOpts{Name: &empty}); err == nil {
		t.Fatal("expected error for empty name via setrule")
	}

	spaces := "   "
	if err := SetRule(store, 1, SetRuleOpts{Name: &spaces}); err == nil {
		t.Fatal("expected error for whitespace name via setrule")
	}
}

func TestAddRuleReusesSlotWithTags(t *testing.T) {
	store := testStore(t)
	AddTag(store, "BACKEND")
	AddRule(store, "R1")
	AddRule(store, "R2")
	DeleteRule(store, 1)

	// Reuse slot 1 with tags
	seq, _ := AddRule(store, "New", "BACKEND")
	if seq != 1 {
		t.Fatalf("expected reused seq 1, got %d", seq)
	}

	rules, _ := GetRules(store, GetRuleOpts{Seq: 1})
	if !strings.Contains(rules[0], "#BACKEND") {
		t.Fatalf("expected tag on reused rule: %v", rules)
	}
}

func TestAddRuleWithTags(t *testing.T) {
	store := testStore(t)
	AddTag(store, "BACKEND")
	AddTag(store, "FRONTEND")

	AddRule(store, "R1", "BACKEND", "FRONTEND")

	rules, _ := GetRules(store, GetRuleOpts{})
	if !strings.Contains(rules[0], "#BACKEND") || !strings.Contains(rules[0], "#FRONTEND") {
		t.Fatalf("expected tags on rule: %v", rules)
	}
}

func TestDeleteRuleAtomic(t *testing.T) {
	store := testStore(t)
	AddTag(store, "DTAG")
	seq, err := AddRule(store, "Atomic rule", "DTAG")
	if err != nil {
		t.Fatal(err)
	}
	name, err := DeleteRuleAtomic(store, seq)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Atomic rule" {
		t.Fatalf("expected 'Atomic rule', got '%s'", name)
	}
	// Verify rule is deleted
	rules, _ := GetRules(store, GetRuleOpts{Seq: seq})
	if len(rules) != 0 {
		t.Fatalf("rule should be deleted, got: %v", rules)
	}
}

func TestDeleteRuleAtomicNotFound(t *testing.T) {
	store := testStore(t)
	_, err := DeleteRuleAtomic(store, 999)
	if err == nil {
		t.Fatal("expected error for nonexistent rule")
	}
}
