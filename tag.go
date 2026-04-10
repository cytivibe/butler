package main

import (
	"database/sql"
	"fmt"
	"strings"
)

var allowedTagTables = map[string]bool{"task_tags": true, "rule_tags": true}
var allowedTagColumns = map[string]bool{"task_id": true, "rule_id": true}

func insertTags(tx *sql.Tx, table, column string, entityID int, tagNames []string) error {
	if !allowedTagTables[table] || !allowedTagColumns[column] {
		return fmt.Errorf("invalid table/column: %s/%s", table, column)
	}
	for i, name := range tagNames {
		n, err := normalizeTagName(name)
		if err != nil {
			return err
		}
		tagNames[i] = n
		var tagID int
		err = tx.QueryRow("SELECT id FROM tags WHERE name = ?", n).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("tag '%s' not found - add it first with addtag", name)
		}
		_, err = tx.Exec(fmt.Sprintf("INSERT OR IGNORE INTO %s (%s, tag_id) VALUES (?, ?)", table, column), entityID, tagID)
		if err != nil {
			return err
		}
	}
	return nil
}

// taskHasTag checks if a task or any ancestor has a specific tag.
// "NONE" matches tasks where neither the task nor any ancestor has tags.
func taskHasTag(tx *sql.Tx, taskID int, tag string) bool {
	if n, err := normalizeTagName(tag); err == nil {
		tag = n
	}
	// "NONE" is a reserved name that normalizeTagName rejects, but it's
	// valid here as a special filter meaning "no tags". Keep it as-is.
	currentID := taskID
	for {
		if tag == "NONE" {
			var count int
			if err := tx.QueryRow("SELECT COUNT(*) FROM task_tags WHERE task_id = ?", currentID).Scan(&count); err != nil {
				return false
			}
			if count > 0 {
				return false
			}
		} else {
			var count int
			if err := tx.QueryRow("SELECT COUNT(*) FROM task_tags tt JOIN tags t ON tt.tag_id = t.id WHERE tt.task_id = ? AND t.name = ?", currentID, tag).Scan(&count); err != nil {
				return false
			}
			if count > 0 {
				return true
			}
		}
		var parentID sql.NullInt64
		if err := tx.QueryRow("SELECT parent_id FROM tasks WHERE id = ?", currentID).Scan(&parentID); err != nil {
			return false
		}
		if !parentID.Valid {
			break
		}
		currentID = int(parentID.Int64)
	}
	return tag == "NONE"
}

// getDirectTagNames returns tag names directly assigned to a task (not inherited).
func getDirectTagNames(tx *sql.Tx, taskID int) []string {
	rows, err := tx.Query("SELECT t.name FROM tags t JOIN task_tags tt ON t.id = tt.tag_id WHERE tt.task_id = ? ORDER BY t.name", taskID)
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
	if rows.Err() != nil {
		return tags
	}
	return tags
}

// getInheritedTagNames returns all tag names for a task, including tags from all ancestors.
func getInheritedTagNames(tx *sql.Tx, taskID int) []string {
	seen := map[string]bool{}
	var tags []string
	currentID := taskID
	for {
		rows, err := tx.Query("SELECT t.name FROM tags t JOIN task_tags tt ON t.id = tt.tag_id WHERE tt.task_id = ? ORDER BY t.name", currentID)
		if err != nil {
			break
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return tags
			}
			if !seen[name] {
				seen[name] = true
				tags = append(tags, name)
			}
		}
		rows.Close()
		if rows.Err() != nil {
			return tags
		}

		var parentID sql.NullInt64
		if err := tx.QueryRow("SELECT parent_id FROM tasks WHERE id = ?", currentID).Scan(&parentID); err != nil {
			break
		}
		if !parentID.Valid {
			break
		}
		currentID = int(parentID.Int64)
	}
	return tags
}

// getRulesForTags returns rules grouped by tag name. Only tags with rules are included.
// Each rule is formatted as "Rule N: name #TAG1 #TAG2".
func getRulesForTags(tx *sql.Tx, tagNames []string) map[string][]string {
	result := map[string][]string{}
	for _, tagName := range tagNames {
		var tagID int
		err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", tagName).Scan(&tagID)
		if err != nil {
			continue
		}
		rows, err := tx.Query("SELECT r.id, r.seq, r.name FROM rules r JOIN rule_tags rt ON r.id = rt.rule_id WHERE rt.tag_id = ? ORDER BY r.seq", tagID)
		if err != nil {
			continue
		}
		for rows.Next() {
			var ruleID, seq int
			var ruleName string
			if err := rows.Scan(&ruleID, &seq, &ruleName); err != nil {
				rows.Close()
				continue
			}
			line := fmt.Sprintf("%s %s", colorRuleHeader(seq), colorDimText(ruleName))
			ruleTags := getEntityTags(tx, "rule_tags", "rule_id", ruleID)
			if ruleTags != "" {
				line += " " + colorTags(ruleTags)
			}
			result[tagName] = append(result[tagName], line)
		}
		rows.Close()
		// rows.Err() ignored: best-effort display function
	}
	return result
}

func appendRules(lines *[]string, prefix string, tagNames []string, rulesMap map[string][]string) {
	if len(rulesMap) == 0 {
		return
	}
	*lines = append(*lines, fmt.Sprintf("%s  %s", prefix, colorDetailLabel("rules:")))
	for _, tag := range tagNames {
		rules, ok := rulesMap[tag]
		if !ok {
			continue
		}
		for _, rule := range rules {
			*lines = append(*lines, fmt.Sprintf("%s    %s", prefix, rule))
		}
	}
}

func getEntityTags(tx *sql.Tx, table, column string, entityID int) string {
	if !allowedTagTables[table] || !allowedTagColumns[column] {
		return ""
	}
	rows, err := tx.Query(fmt.Sprintf("SELECT t.name FROM tags t JOIN %s et ON t.id = et.tag_id WHERE et.%s = ? ORDER BY t.name", table, column), entityID)
	if err != nil {
		return ""
	}
	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return strings.Join(tags, " ")
		}
		tags = append(tags, "#"+name)
	}
	rows.Close()
	// rows.Err() ignored: best-effort display function
	return strings.Join(tags, " ")
}

// syncRuleTagFlags updates the ruletag flag for the given tag names
// based on whether they appear in rule_tags.
// INVARIANT: This must be called whenever rule-tag associations change
// (AddRule with tags, SetRule with tags, DeleteRule). Failure to call
// this will leave the ruletag flag stale, causing incorrect behavior
// in tag queries that rely on this denormalized flag.
func syncRuleTagFlags(tx *sql.Tx, tagNames []string) error {
	for _, name := range tagNames {
		var tagID int
		err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&tagID)
		if err != nil {
			continue
		}
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM rule_tags WHERE tag_id = ?", tagID).Scan(&count); err != nil {
			continue
		}
		ruletag := 0
		if count > 0 {
			ruletag = 1
		}
		if _, err := tx.Exec("UPDATE tags SET ruletag = ? WHERE id = ?", ruletag, tagID); err != nil {
			return err
		}
	}
	return nil
}

func AddTag(store *Store, name string) error {
	n, err := normalizeTagName(name)
	if err != nil {
		return err
	}
	return store.WriteTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO tags (name, created_at) VALUES (?, ?)", n, nowLocal())
		if err != nil && strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("tag '%s' already exists", name)
		}
		return err
	})
}

// GetTagOpts holds options for GetTags.
type GetTagOpts struct {
	Tag string // specific tag name (empty = all tags)
}

func GetTags(store *Store, opts GetTagOpts) ([]string, error) {
	var lines []string
	err := store.ReadTx(func(tx *sql.Tx) error {
		if opts.Tag != "" {
			normalized, nerr := normalizeTagName(opts.Tag)
			if nerr != nil {
				return nerr
			}
			opts.Tag = normalized
			var tagID int
			err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", opts.Tag).Scan(&tagID)
			if err != nil {
				return fmt.Errorf("tag '%s' not found", opts.Tag)
			}
			header, err := tagCountHeader(tx, tagID, opts.Tag)
			if err != nil {
				return err
			}
			lines = append(lines, header)

			// Tasks with this tag (direct only)
			taskRows, err := tx.Query(`
				SELECT t.id, t.name, t.parent_id FROM tasks t
				JOIN task_tags tt ON t.id = tt.task_id
				WHERE tt.tag_id = ? ORDER BY t.id`, tagID)
			if err != nil {
				return err
			}
			type taggedTask struct {
				id        int
				name      string
				hasParent bool
			}
			var tasks []taggedTask
			for taskRows.Next() {
				var tt taggedTask
				var parentID sql.NullInt64
				if err := taskRows.Scan(&tt.id, &tt.name, &parentID); err != nil {
					taskRows.Close()
					return err
				}
				tt.hasParent = parentID.Valid
				tasks = append(tasks, tt)
			}
			taskRows.Close()
			if err := taskRows.Err(); err != nil {
				return err
			}

			if len(tasks) > 0 {
				lines = append(lines, "")
				lines = append(lines, colorDetailLabel("  Tasks"))
				for _, tt := range tasks {
					if !tt.hasParent {
						lines = append(lines, "  "+colorTaskName(tt.name))
					} else {
						rootName, path, _ := getTaskPath(tx, tt.id)
						lines = append(lines, fmt.Sprintf("  %s %s", colorTaskName(tt.name), colorDimText(fmt.Sprintf("(%s:%s)", rootName, path))))
					}
				}
			}

			// Rules with this tag
			ruleRows, err := tx.Query(`
				SELECT r.seq, r.name FROM rules r
				JOIN rule_tags rt ON r.id = rt.rule_id
				WHERE rt.tag_id = ? AND r.name != '' ORDER BY r.seq`, tagID)
			if err != nil {
				return err
			}
			var rules []string
			for ruleRows.Next() {
				var seq int
				var name string
				if err := ruleRows.Scan(&seq, &name); err != nil {
					ruleRows.Close()
					return err
				}
				rules = append(rules, fmt.Sprintf("  %s %s", colorRuleHeader(seq), colorDimText(name)))
			}
			ruleRows.Close()
			if err := ruleRows.Err(); err != nil {
				return err
			}

			if len(rules) > 0 {
				lines = append(lines, "")
				lines = append(lines, colorDetailLabel("  Rules"))
				lines = append(lines, rules...)
			}
			return nil
		}

		rows, err := tx.Query("SELECT id, name FROM tags ORDER BY id")
		if err != nil {
			return err
		}
		type tagRow struct {
			id   int
			name string
		}
		var tagRows []tagRow
		for rows.Next() {
			var tr tagRow
			if err := rows.Scan(&tr.id, &tr.name); err != nil {
				rows.Close()
				return err
			}
			tagRows = append(tagRows, tr)
		}
		rows.Close()
		for _, tr := range tagRows {
			header, err := tagCountHeader(tx, tr.id, tr.name)
			if err != nil {
				return err
			}
			lines = append(lines, header)
		}
		return rows.Err()
	})
	return lines, err
}

// tagCountHeader builds a tag header like "BACKEND [3 tasks, 1 rule]".
func tagCountHeader(tx *sql.Tx, tagID int, name string) (string, error) {
	var taskCount, ruleCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM task_tags WHERE tag_id = ?", tagID).Scan(&taskCount); err != nil {
		return "", err
	}
	if err := tx.QueryRow("SELECT COUNT(*) FROM rule_tags rt JOIN rules r ON rt.rule_id = r.id WHERE rt.tag_id = ? AND r.name != ''", tagID).Scan(&ruleCount); err != nil {
		return "", err
	}
	coloredName := colorTagHeader(name)
	if taskCount == 0 && ruleCount == 0 {
		return fmt.Sprintf("%s  %s", coloredName, colorDimText("unused")), nil
	}
	var parts []string
	if taskCount > 0 {
		if taskCount == 1 {
			parts = append(parts, "1 task")
		} else {
			parts = append(parts, fmt.Sprintf("%d tasks", taskCount))
		}
	}
	if ruleCount > 0 {
		if ruleCount == 1 {
			parts = append(parts, "1 rule")
		} else {
			parts = append(parts, fmt.Sprintf("%d rules", ruleCount))
		}
	}
	return fmt.Sprintf("%s  %s", coloredName, colorDimText(strings.Join(parts, ", "))), nil
}

// normalizeTagName strips cosmetic noise (whitespace, quotes, #, case)
// and then validates. Returns the cleaned name or an error.
func normalizeTagName(name string) (string, error) {
	name = strings.TrimSpace(name)
	// Strip matching outer quotes
	if len(name) >= 2 {
		if (name[0] == '"' && name[len(name)-1] == '"') || (name[0] == '\'' && name[len(name)-1] == '\'') {
			name = name[1 : len(name)-1]
		}
	}
	name = strings.TrimSpace(name)
	// Strip leading # characters
	name = strings.TrimLeft(name, "#")
	name = strings.TrimSpace(name)
	name = strings.ToUpper(name)
	if err := validateTagName(name); err != nil {
		return "", err
	}
	return name, nil
}

// normalizeTagList splits each element on commas, normalizes each piece,
// deduplicates, and returns the cleaned list. NONE is allowed but cannot
// be mixed with other tags.
func normalizeTagList(raw []string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)
	for _, r := range raw {
		parts := strings.Split(r, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			upper := strings.ToUpper(strings.TrimSpace(strings.TrimLeft(p, "#")))
			if upper == "NONE" {
				if !seen["NONE"] {
					result = append(result, "NONE")
					seen["NONE"] = true
				}
				continue
			}
			n, err := normalizeTagName(p)
			if err != nil {
				return nil, fmt.Errorf("invalid tag %q: %w", p, err)
			}
			if !seen[n] {
				result = append(result, n)
				seen[n] = true
			}
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid tags provided")
	}
	if seen["NONE"] && len(result) > 1 {
		return nil, fmt.Errorf("NONE cannot be combined with other tags")
	}
	return result, nil
}

// tagExists checks whether a tag with the given name exists in the database.
func tagExists(store *Store, name string) (bool, error) {
	var count int
	err := store.ReadTx(func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT COUNT(*) FROM tags WHERE name = ?", name).Scan(&count)
	})
	return count > 0, err
}

func validateTagName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("tag cannot be empty")
	}
	if len(name) > 10 {
		return fmt.Errorf("tag must be 10 characters or less, got %d", len(name))
	}
	for _, c := range name {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return fmt.Errorf("tag must be strictly alphanumeric and uppercase, invalid character: '%c'", c)
		}
	}
	if name == "NONE" {
		return fmt.Errorf("'NONE' is a reserved tag name")
	}
	return nil
}

// SetTag renames a tag.
func SetTag(store *Store, oldName, newName string) error {
	var err error
	oldName, err = normalizeTagName(oldName)
	if err != nil {
		return err
	}
	newName, err = normalizeTagName(newName)
	if err != nil {
		return err
	}
	return store.WriteTx(func(tx *sql.Tx) error {
		var tagID int
		err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", oldName).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("tag '%s' not found", oldName)
		}
		_, err = tx.Exec("UPDATE tags SET name = ? WHERE id = ?", newName, tagID)
		if err != nil && strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("tag '%s' already exists", newName)
		}
		return err
	})
}

// DeleteTagInfo returns info about a tag for confirmation prompts.
func DeleteTagInfo(store *Store, name string) (taskCount, ruleCount int, err error) {
	name, err = normalizeTagName(name)
	if err != nil {
		return 0, 0, err
	}
	err = store.ReadTx(func(tx *sql.Tx) error {
		var tagID int
		err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("tag '%s' not found", name)
		}
		if err := tx.QueryRow("SELECT COUNT(*) FROM task_tags WHERE tag_id = ?", tagID).Scan(&taskCount); err != nil {
			return err
		}
		if err := tx.QueryRow("SELECT COUNT(*) FROM rule_tags WHERE tag_id = ?", tagID).Scan(&ruleCount); err != nil {
			return err
		}
		return nil
	})
	return
}

// DeleteTagConfirmed permanently deletes a tag and removes it from all tasks and rules.
func DeleteTagConfirmed(store *Store, name string) error {
	name, nerr := normalizeTagName(name)
	if nerr != nil {
		return nerr
	}
	return store.WriteTx(func(tx *sql.Tx) error {
		var tagID int
		err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("tag '%s' not found", name)
		}
		if _, err := tx.Exec("DELETE FROM task_tags WHERE tag_id = ?", tagID); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM rule_tags WHERE tag_id = ?", tagID); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM tags WHERE id = ?", tagID); err != nil {
			return err
		}
		return nil
	})
}

// DeleteTagAtomic combines info gathering and deletion in a single WriteTx.
// Returns task/rule counts and any error. Used by MCP to avoid TOCTOU.
func DeleteTagAtomic(store *Store, name string) (taskCount, ruleCount int, err error) {
	name, err = normalizeTagName(name)
	if err != nil {
		return 0, 0, err
	}
	err = store.WriteTx(func(tx *sql.Tx) error {
		var tagID int
		err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("tag '%s' not found", name)
		}
		if err := tx.QueryRow("SELECT COUNT(*) FROM task_tags WHERE tag_id = ?", tagID).Scan(&taskCount); err != nil {
			return err
		}
		if err := tx.QueryRow("SELECT COUNT(*) FROM rule_tags WHERE tag_id = ?", tagID).Scan(&ruleCount); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM task_tags WHERE tag_id = ?", tagID); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM rule_tags WHERE tag_id = ?", tagID); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM tags WHERE id = ?", tagID); err != nil {
			return err
		}
		return nil
	})
	return
}
