package main

import (
	"database/sql"
	"testing"
)

func TestCascadeDeleteTaskCleansJunctionTables(t *testing.T) {
	store := testStore(t)
	AddTag(store, "CTAG")
	AddTask(store, "CTask", "", false, false, "CTAG")

	// Verify tag association exists
	store.ReadTx(func(tx *sql.Tx) error {
		var count int
		tx.QueryRow("SELECT COUNT(*) FROM task_tags").Scan(&count)
		if count == 0 {
			t.Fatal("expected task_tag association")
		}
		return nil
	})

	// Delete the task directly via SQL (bypassing app logic) to test CASCADE
	store.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("DELETE FROM tasks WHERE name = 'CTask'")
		return err
	})

	// Junction table row should be gone due to CASCADE
	store.ReadTx(func(tx *sql.Tx) error {
		var count int
		tx.QueryRow("SELECT COUNT(*) FROM task_tags").Scan(&count)
		if count != 0 {
			t.Fatalf("expected 0 task_tag rows after CASCADE delete, got %d", count)
		}
		return nil
	})
}

func TestCascadeDeleteRuleCleansJunctionTables(t *testing.T) {
	store := testStore(t)
	AddTag(store, "RTAG")
	AddRule(store, "CRule", "RTAG")

	// Delete the rule directly via SQL to test CASCADE
	store.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("DELETE FROM rules WHERE name = 'CRule'")
		return err
	})

	store.ReadTx(func(tx *sql.Tx) error {
		var count int
		tx.QueryRow("SELECT COUNT(*) FROM rule_tags").Scan(&count)
		if count != 0 {
			t.Fatalf("expected 0 rule_tag rows after CASCADE delete, got %d", count)
		}
		return nil
	})
}

func TestCascadeDeleteTagCleansJunctionTables(t *testing.T) {
	store := testStore(t)
	AddTag(store, "GONE")
	AddTask(store, "T", "", false, false, "GONE")
	AddRule(store, "R", "GONE")

	// Delete the tag directly via SQL to test CASCADE
	store.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("DELETE FROM tags WHERE name = 'GONE'")
		return err
	})

	store.ReadTx(func(tx *sql.Tx) error {
		var taskTagCount, ruleTagCount int
		tx.QueryRow("SELECT COUNT(*) FROM task_tags").Scan(&taskTagCount)
		tx.QueryRow("SELECT COUNT(*) FROM rule_tags").Scan(&ruleTagCount)
		if taskTagCount != 0 || ruleTagCount != 0 {
			t.Fatalf("expected 0 junction rows after CASCADE, got task_tags=%d rule_tags=%d", taskTagCount, ruleTagCount)
		}
		return nil
	})
}
