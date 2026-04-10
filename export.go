package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ExportData is the top-level JSON structure for butler export/import.
type ExportData struct {
	Version int          `json:"version"`
	Tags    []ExportTag  `json:"tags"`
	Rules   []ExportRule `json:"rules"`
	Tasks   []ExportTask `json:"tasks"`
}

type ExportTag struct {
	Name string `json:"name"`
}

type ExportRule struct {
	Seq       int      `json:"seq"`
	Name      string   `json:"name"`
	Tags      []string `json:"tags,omitempty"`
	CreatedAt string   `json:"created_at"`
}

type ExportTask struct {
	Name            string       `json:"name"`
	Status          string       `json:"status"`
	Description     string       `json:"description,omitempty"`
	Verification    string       `json:"verification,omitempty"`
	VerifyStatus    string       `json:"verify_status,omitempty"`
	Deadline        string       `json:"deadline,omitempty"`
	Recur           string       `json:"recur,omitempty"`
	Parallel        bool         `json:"parallel,omitempty"`
	Tags            []string     `json:"tags,omitempty"`
	Blockers        []string     `json:"blockers,omitempty"`
	CreatedAt       string       `json:"created_at"`
	StatusChangedAt string       `json:"status_changed_at"`
	Children        []ExportTask `json:"children,omitempty"`
}

// Export reads all data from the store and returns it as ExportData.
func Export(store *Store) (*ExportData, error) {
	data := &ExportData{Version: 1}
	err := store.ReadTx(func(tx *sql.Tx) error {
		// Tags
		rows, err := tx.Query("SELECT name FROM tags ORDER BY id")
		if err != nil {
			return err
		}
		for rows.Next() {
			var t ExportTag
			if err := rows.Scan(&t.Name); err != nil {
				rows.Close()
				return err
			}
			data.Tags = append(data.Tags, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		// Rules (skip soft-deleted)
		ruleRows, err := tx.Query("SELECT id, seq, name, created_at FROM rules WHERE name != '' ORDER BY seq")
		if err != nil {
			return err
		}
		type ruleRow struct {
			id        int
			seq       int
			name      string
			createdAt string
		}
		var ruleList []ruleRow
		for ruleRows.Next() {
			var r ruleRow
			if err := ruleRows.Scan(&r.id, &r.seq, &r.name, &r.createdAt); err != nil {
				ruleRows.Close()
				return err
			}
			ruleList = append(ruleList, r)
		}
		ruleRows.Close()
		if err := ruleRows.Err(); err != nil {
			return err
		}
		for _, r := range ruleList {
			er := ExportRule{Seq: r.seq, Name: r.name, CreatedAt: r.createdAt}
			tags := getRuleTagNames(tx, r.id)
			if len(tags) > 0 {
				er.Tags = tags
			}
			data.Rules = append(data.Rules, er)
		}

		// Tasks (root level, then recurse)
		taskRows, err := tx.Query("SELECT id FROM tasks WHERE parent_id IS NULL ORDER BY id")
		if err != nil {
			return err
		}
		var rootIDs []int
		for taskRows.Next() {
			var id int
			if err := taskRows.Scan(&id); err != nil {
				taskRows.Close()
				return err
			}
			rootIDs = append(rootIDs, id)
		}
		taskRows.Close()
		if err := taskRows.Err(); err != nil {
			return err
		}
		for _, id := range rootIDs {
			et, err := exportTask(tx, id)
			if err != nil {
				return err
			}
			data.Tasks = append(data.Tasks, et)
		}
		return nil
	})
	return data, err
}

func exportTask(tx *sql.Tx, taskID int) (ExportTask, error) {
	var et ExportTask
	var dl, rc sql.NullString
	var parallel int
	err := tx.QueryRow("SELECT name, status, description, verification, verify_status, deadline, recur, parallel, created_at, status_changed_at FROM tasks WHERE id = ?", taskID).
		Scan(&et.Name, &et.Status, &et.Description, &et.Verification, &et.VerifyStatus, &dl, &rc, &parallel, &et.CreatedAt, &et.StatusChangedAt)
	if err != nil {
		return et, err
	}
	if dl.Valid {
		et.Deadline = dl.String
	}
	if rc.Valid {
		et.Recur = rc.String
	}
	et.Parallel = parallel == 1

	// Tags
	et.Tags = getDirectTagNames(tx, taskID)

	// Blockers (as coordinate refs)
	blockerRows, err := tx.Query("SELECT blocker_id FROM task_blockers WHERE task_id = ?", taskID)
	if err != nil {
		return et, err
	}
	var blockerIDs []int
	for blockerRows.Next() {
		var bid int
		if err := blockerRows.Scan(&bid); err != nil {
			blockerRows.Close()
			return et, err
		}
		blockerIDs = append(blockerIDs, bid)
	}
	blockerRows.Close()
	if err := blockerRows.Err(); err != nil {
		return et, err
	}
	for _, bid := range blockerIDs {
		rootName, path, err := getTaskPath(tx, bid)
		if err != nil {
			continue
		}
		if path == "" {
			et.Blockers = append(et.Blockers, rootName)
		} else {
			et.Blockers = append(et.Blockers, rootName+":"+path)
		}
	}

	// Children (ordered by parallel DESC, position ASC - same as display)
	childRows, err := tx.Query("SELECT id FROM tasks WHERE parent_id = ? ORDER BY parallel DESC, position", taskID)
	if err != nil {
		return et, err
	}
	var childIDs []int
	for childRows.Next() {
		var cid int
		if err := childRows.Scan(&cid); err != nil {
			childRows.Close()
			return et, err
		}
		childIDs = append(childIDs, cid)
	}
	childRows.Close()
	if err := childRows.Err(); err != nil {
		return et, err
	}
	for _, cid := range childIDs {
		child, err := exportTask(tx, cid)
		if err != nil {
			return et, err
		}
		et.Children = append(et.Children, child)
	}
	return et, nil
}

func getRuleTagNames(tx *sql.Tx, ruleID int) []string {
	rows, err := tx.Query("SELECT t.name FROM tags t JOIN rule_tags rt ON t.id = rt.tag_id WHERE rt.rule_id = ? ORDER BY t.name", ruleID)
	if err != nil {
		return nil
	}
	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return tags
		}
		tags = append(tags, name)
	}
	rows.Close()
	return tags
}

// ExportJSON returns the export data as indented JSON bytes.
func ExportJSON(store *Store) ([]byte, error) {
	data, err := Export(store)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(data, "", "  ")
}

// Import loads data from ExportData into the store.
// If replace is true, all existing data is deleted first.
// If replace is false (merge), tags/rules/tasks are added, skipping duplicates.
func Import(store *Store, data *ExportData, replace bool) error {
	if data.Version != 1 {
		return fmt.Errorf("unsupported export version: %d (expected 1)", data.Version)
	}
	return store.WriteTx(func(tx *sql.Tx) error {
		if replace {
			// Delete all existing data (order matters for foreign keys)
			for _, table := range []string{"task_blockers", "task_tags", "rule_tags", "tasks", "rules", "tags"} {
				if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
					return fmt.Errorf("clearing %s: %w", table, err)
				}
			}
		}

		// Import tags
		for _, t := range data.Tags {
			if replace {
				if _, err := tx.Exec("INSERT INTO tags (name, created_at) VALUES (?, ?)", t.Name, nowLocal()); err != nil {
					return fmt.Errorf("importing tag '%s': %w", t.Name, err)
				}
			} else {
				// Merge: skip if exists
				var id int
				err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", t.Name).Scan(&id)
				if err == sql.ErrNoRows {
					if _, err := tx.Exec("INSERT INTO tags (name, created_at) VALUES (?, ?)", t.Name, nowLocal()); err != nil {
						return fmt.Errorf("importing tag '%s': %w", t.Name, err)
					}
				}
			}
		}

		// Import rules
		for _, r := range data.Rules {
			if replace {
				result, err := tx.Exec("INSERT INTO rules (name, seq, created_at) VALUES (?, ?, ?)", r.Name, r.Seq, r.CreatedAt)
				if err != nil {
					return fmt.Errorf("importing rule '%s': %w", r.Name, err)
				}
				ruleID, _ := result.LastInsertId()
				if len(r.Tags) > 0 {
					if err := insertTags(tx, "rule_tags", "rule_id", int(ruleID), r.Tags); err != nil {
						return fmt.Errorf("importing rule tags for '%s': %w", r.Name, err)
					}
				}
			} else {
				// Merge: skip if a rule with the same name already exists
				var existingID int
				err := tx.QueryRow("SELECT id FROM rules WHERE name = ?", r.Name).Scan(&existingID)
				if err == sql.ErrNoRows {
					// Assign next available seq slot (reuse deleted slots first, then next number)
					seq := r.Seq
					var emptySeq int
					if err := tx.QueryRow("SELECT seq FROM rules WHERE name = '' ORDER BY seq LIMIT 1").Scan(&emptySeq); err == nil {
						seq = emptySeq
					} else {
						var maxSeq sql.NullInt64
						if err := tx.QueryRow("SELECT MAX(seq) FROM rules").Scan(&maxSeq); err == nil && maxSeq.Valid {
							seq = int(maxSeq.Int64) + 1
						}
					}
					result, err := tx.Exec("INSERT INTO rules (name, seq, created_at) VALUES (?, ?, ?)", r.Name, seq, r.CreatedAt)
					if err != nil {
						return fmt.Errorf("importing rule '%s': %w", r.Name, err)
					}
					ruleID, _ := result.LastInsertId()
					if len(r.Tags) > 0 {
						if err := insertTags(tx, "rule_tags", "rule_id", int(ruleID), r.Tags); err != nil {
							return fmt.Errorf("importing rule tags for '%s': %w", r.Name, err)
						}
					}
				}
			}
		}

		// Import tasks (recursive, depth-first)
		for _, t := range data.Tasks {
			if !replace {
				// Merge: skip if a non-archived root task with same name exists
				var existingID int
				err := tx.QueryRow("SELECT id FROM tasks WHERE name = ? AND parent_id IS NULL AND status != 'archived'", t.Name).Scan(&existingID)
				if err == nil {
					continue
				}
			}
			if _, err := importTask(tx, t, nil); err != nil {
				return err
			}
		}

		// Sync ruletag flags
		var allTagNames []string
		for _, t := range data.Tags {
			allTagNames = append(allTagNames, t.Name)
		}
		if len(allTagNames) > 0 {
			return syncRuleTagFlags(tx, allTagNames)
		}
		return nil
	})
}

// importTask recursively imports a task and its children. Returns the new task ID.
func importTask(tx *sql.Tx, et ExportTask, parentID *int) (int, error) {
	parallel := 0
	if et.Parallel {
		parallel = 1
	}
	var result sql.Result
	var err error
	if parentID == nil {
		result, err = tx.Exec("INSERT INTO tasks (name, status, description, verification, verify_status, deadline, recur, parallel, created_at, status_changed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			et.Name, et.Status, et.Description, et.Verification, et.VerifyStatus, nullStr(et.Deadline), nullStr(et.Recur), parallel, et.CreatedAt, et.StatusChangedAt)
	} else {
		// Determine next position among siblings of the same type (parallel/sequential)
		var maxPos sql.NullInt64
		if err := tx.QueryRow("SELECT MAX(position) FROM tasks WHERE parent_id = ? AND parallel = ?", *parentID, parallel).Scan(&maxPos); err != nil && err != sql.ErrNoRows {
			return 0, err
		}
		nextPos := 1
		if maxPos.Valid {
			nextPos = int(maxPos.Int64) + 1
		}
		result, err = tx.Exec("INSERT INTO tasks (name, parent_id, position, parallel, status, description, verification, verify_status, deadline, recur, created_at, status_changed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			et.Name, *parentID, nextPos, parallel, et.Status, et.Description, et.Verification, et.VerifyStatus, nullStr(et.Deadline), nullStr(et.Recur), et.CreatedAt, et.StatusChangedAt)
	}
	if err != nil {
		return 0, fmt.Errorf("importing task '%s': %w", et.Name, err)
	}
	taskID64, _ := result.LastInsertId()
	taskID := int(taskID64)

	// Backfill verify_status for imports from old exports that lack the field.
	if et.VerifyStatus == "" && et.Verification != "" {
		if _, err := tx.Exec("UPDATE tasks SET verify_status = 'pending' WHERE id = ?", taskID); err != nil {
			return 0, fmt.Errorf("backfilling verify_status for '%s': %w", et.Name, err)
		}
	}

	// Tags
	if len(et.Tags) > 0 {
		if err := insertTags(tx, "task_tags", "task_id", taskID, et.Tags); err != nil {
			return 0, fmt.Errorf("importing task tags for '%s': %w", et.Name, err)
		}
	}

	// Children
	for _, child := range et.Children {
		if _, err := importTask(tx, child, &taskID); err != nil {
			return 0, err
		}
	}

	return taskID, nil
}

// nullStr returns a *string that is nil for empty strings (for nullable DB columns).
func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ImportJSON parses JSON bytes and imports the data.
func ImportJSON(store *Store, jsonBytes []byte, replace bool) error {
	var data ExportData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return Import(store, &data, replace)
}

// resolveBlockers resolves blocker references after all tasks have been imported.
// This is called as a second pass since blockers may reference tasks imported later.
func resolveBlockers(store *Store, tasks []ExportTask) error {
	return store.WriteTx(func(tx *sql.Tx) error {
		return resolveBlockersRecursive(tx, tasks)
	})
}

func resolveBlockersRecursive(tx *sql.Tx, tasks []ExportTask) error {
	for _, t := range tasks {
		if len(t.Blockers) > 0 {
			// Find this task's ID
			namedTask, posPath := parseTaskRef(t.Name)
			taskID, err := resolveTaskID(tx, namedTask, posPath)
			if err != nil {
				continue
			}
			for _, blockerRef := range t.Blockers {
				bNamed, bPos := parseTaskRef(blockerRef)
				blockerID, err := resolveTaskID(tx, bNamed, bPos)
				if err != nil {
					continue
				}
				tx.Exec("INSERT OR IGNORE INTO task_blockers (task_id, blocker_id) VALUES (?, ?)", taskID, blockerID)
			}
		}
		if len(t.Children) > 0 {
			if err := resolveBlockersRecursive(tx, t.Children); err != nil {
				return err
			}
		}
	}
	return nil
}

// ImportJSONWithBlockers does a full import including blocker resolution.
func ImportJSONWithBlockers(store *Store, jsonBytes []byte, replace bool) error {
	var data ExportData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := Import(store, &data, replace); err != nil {
		return err
	}
	// Second pass: resolve blockers now that all tasks exist
	return resolveBlockers(store, data.Tasks)
}

// ExportSummary returns a human-readable summary of what would be exported.
func ExportSummary(data *ExportData) string {
	parts := []string{}
	if len(data.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("%d tags", len(data.Tags)))
	}
	if len(data.Rules) > 0 {
		parts = append(parts, fmt.Sprintf("%d rules", len(data.Rules)))
	}
	taskCount := countExportTasks(data.Tasks)
	if taskCount > 0 {
		parts = append(parts, fmt.Sprintf("%d tasks", taskCount))
	}
	if len(parts) == 0 {
		return "nothing to export"
	}
	return strings.Join(parts, ", ")
}

func countExportTasks(tasks []ExportTask) int {
	count := len(tasks)
	for _, t := range tasks {
		count += countExportTasks(t.Children)
	}
	return count
}
