package main

import (
	"database/sql"
	"fmt"
	"strings"
)

func AddRule(store *Store, name string, tags ...string) (int, error) {
	if strings.TrimSpace(name) == "" {
		return 0, fmt.Errorf("rule name cannot be empty")
	}
	var seq int
	err := store.WriteTx(func(tx *sql.Tx) error {
		var emptyID int
		var emptySeq int
		err := tx.QueryRow("SELECT id, seq FROM rules WHERE name = '' ORDER BY seq LIMIT 1").Scan(&emptyID, &emptySeq)
		if err == nil {
			seq = emptySeq
			if _, err := tx.Exec("UPDATE rules SET name = ? WHERE id = ?", name, emptyID); err != nil {
				return err
			}
			if len(tags) > 0 {
				if err := insertTags(tx, "rule_tags", "rule_id", emptyID, tags); err != nil {
					return err
				}
				return syncRuleTagFlags(tx, tags)
			}
			return nil
		}

		var maxSeq sql.NullInt64
		if err := tx.QueryRow("SELECT MAX(seq) FROM rules").Scan(&maxSeq); err != nil && err != sql.ErrNoRows {
			return err
		}
		seq = 1
		if maxSeq.Valid {
			seq = int(maxSeq.Int64) + 1
		}
		result, err := tx.Exec("INSERT INTO rules (name, seq, created_at) VALUES (?, ?, ?)", name, seq, nowLocal())
		if err != nil {
			return err
		}
		if len(tags) > 0 {
			ruleID, _ := result.LastInsertId()
			if err := insertTags(tx, "rule_tags", "rule_id", int(ruleID), tags); err != nil {
				return err
			}
			return syncRuleTagFlags(tx, tags)
		}
		return nil
	})
	return seq, err
}

// GetRuleOpts holds options for GetRules.
type GetRuleOpts struct {
	Seq    int    // specific rule by seq number (0 = not specified)
	Tag    string // filter by tag name (empty = no filter, "NONE" = untagged)
	TagAll bool   // group all rules by tags
}

func GetRules(store *Store, opts GetRuleOpts) ([]string, error) {
	if opts.Tag != "" {
		if n, err := normalizeTagName(opts.Tag); err == nil {
			opts.Tag = n
		} else if strings.ToUpper(strings.TrimSpace(opts.Tag)) != "NONE" {
			return nil, err
		} else {
			opts.Tag = "NONE"
		}
	}
	var lines []string
	err := store.ReadTx(func(tx *sql.Tx) error {
		if opts.Seq > 0 {
			var id int
			var name string
			err := tx.QueryRow("SELECT id, name FROM rules WHERE seq = ?", opts.Seq).Scan(&id, &name)
			if err != nil {
				return fmt.Errorf("rule %d not found", opts.Seq)
			}
			if name == "" {
				return fmt.Errorf("rule %d not found", opts.Seq)
			}
			line := fmt.Sprintf("%s %s", colorRuleHeader(opts.Seq), name)
			tags := getEntityTags(tx, "rule_tags", "rule_id", id)
			if tags != "" {
				line += " " + colorTags(tags)
			}
			lines = append(lines, line)
			return nil
		}

		if opts.TagAll {
			tagRows, err := tx.Query("SELECT DISTINCT t.name FROM tags t JOIN rule_tags rt ON t.id = rt.tag_id ORDER BY t.name")
			if err != nil {
				return err
			}
			var tagNames []string
			for tagRows.Next() {
				var name string
				if err := tagRows.Scan(&name); err != nil {
					tagRows.Close()
					return err
				}
				tagNames = append(tagNames, name)
			}
			tagRows.Close()
			if err := tagRows.Err(); err != nil {
				return err
			}

			var untaggedCount int
			if err := tx.QueryRow("SELECT COUNT(*) FROM rules WHERE name != '' AND id NOT IN (SELECT DISTINCT rule_id FROM rule_tags)").Scan(&untaggedCount); err != nil {
				return err
			}

			allGroups := tagNames
			if untaggedCount > 0 {
				allGroups = append(allGroups, "NONE")
			}

			for _, group := range allGroups {
				lines = append(lines, colorTagHeader(group))
				var rows *sql.Rows
				if group == "NONE" {
					rows, err = tx.Query("SELECT seq, name FROM rules WHERE name != '' AND id NOT IN (SELECT DISTINCT rule_id FROM rule_tags) ORDER BY seq")
				} else {
					rows, err = tx.Query("SELECT r.seq, r.name FROM rules r JOIN rule_tags rt ON r.id = rt.rule_id JOIN tags t ON rt.tag_id = t.id WHERE t.name = ? ORDER BY r.seq", group)
				}
				if err != nil {
					return err
				}
				for rows.Next() {
					var seq int
					var name string
					if err := rows.Scan(&seq, &name); err != nil {
						rows.Close()
						return err
					}
					lines = append(lines, fmt.Sprintf("  %s %s", colorRuleHeader(seq), name))
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return err
				}
			}
			return nil
		}

		if opts.Tag != "" {
			if opts.Tag == "NONE" {
				rows, err := tx.Query("SELECT seq, name FROM rules WHERE name != '' AND id NOT IN (SELECT DISTINCT rule_id FROM rule_tags) ORDER BY seq")
				if err != nil {
					return err
				}
				for rows.Next() {
					var seq int
					var name string
					if err := rows.Scan(&seq, &name); err != nil {
						rows.Close()
						return err
					}
					lines = append(lines, fmt.Sprintf("%s %s", colorRuleHeader(seq), name))
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return err
				}
			} else {
				rows, err := tx.Query("SELECT r.seq, r.name FROM rules r JOIN rule_tags rt ON r.id = rt.rule_id JOIN tags t ON rt.tag_id = t.id WHERE t.name = ? ORDER BY r.seq", opts.Tag)
				if err != nil {
					return err
				}
				for rows.Next() {
					var seq int
					var name string
					if err := rows.Scan(&seq, &name); err != nil {
						rows.Close()
						return err
					}
					lines = append(lines, fmt.Sprintf("%s %s", colorRuleHeader(seq), name))
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return err
				}
			}
			return nil
		}

		// All rules in seq order (hide deleted/empty rules)
		rows, err := tx.Query("SELECT id, seq, name FROM rules WHERE name != '' ORDER BY seq")
		if err != nil {
			return err
		}
		type rule struct {
			id   int
			seq  int
			name string
		}
		var rules []rule
		for rows.Next() {
			var r rule
			if err := rows.Scan(&r.id, &r.seq, &r.name); err != nil {
				rows.Close()
				return err
			}
			rules = append(rules, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, r := range rules {
			line := fmt.Sprintf("%s %s", colorRuleHeader(r.seq), r.name)
			tags := getEntityTags(tx, "rule_tags", "rule_id", r.id)
			if tags != "" {
				line += " " + colorTags(tags)
			}
			lines = append(lines, line)
		}
		return nil
	})
	return lines, err
}

// SetRuleOpts holds optional fields for SetRule.
type SetRuleOpts struct {
	Name    *string  // new rule name (nil = don't change)
	SetTags bool     // if true, replace all tags with Tags
	Tags    []string // tag names (only used when SetTags is true)
}

func SetRule(store *Store, seq int, opts SetRuleOpts) error {
	return store.WriteTx(func(tx *sql.Tx) error {
		var ruleID int
		err := tx.QueryRow("SELECT id FROM rules WHERE seq = ?", seq).Scan(&ruleID)
		if err != nil {
			return fmt.Errorf("rule %d not found", seq)
		}
		if opts.Name != nil {
			if strings.TrimSpace(*opts.Name) == "" {
				return fmt.Errorf("rule name cannot be empty - use deleterule to remove a rule")
			}
			if _, err := tx.Exec("UPDATE rules SET name = ? WHERE id = ?", *opts.Name, ruleID); err != nil {
				return err
			}
		}
		if opts.SetTags {
			// Get old tags for ruletag flag sync
			var oldTags []string
			rows, err := tx.Query("SELECT t.name FROM tags t JOIN rule_tags rt ON t.id = rt.tag_id WHERE rt.rule_id = ?", ruleID)
			if err == nil {
				for rows.Next() {
					var name string
					if err := rows.Scan(&name); err != nil {
						rows.Close()
						return err
					}
					oldTags = append(oldTags, name)
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return err
				}
			}
			if _, err := tx.Exec("DELETE FROM rule_tags WHERE rule_id = ?", ruleID); err != nil {
				return err
			}
			if len(opts.Tags) > 0 {
				if err := insertTags(tx, "rule_tags", "rule_id", ruleID, opts.Tags); err != nil {
					return err
				}
			}
			allTags := make([]string, 0, len(oldTags)+len(opts.Tags))
			allTags = append(allTags, oldTags...)
			allTags = append(allTags, opts.Tags...)
			if len(allTags) > 0 {
				return syncRuleTagFlags(tx, allTags)
			}
		}
		return nil
	})
}

// DeleteRuleAtomic combines info gathering and deletion in a single WriteTx.
// Returns the rule name and any error. Used to avoid TOCTOU.
func DeleteRuleAtomic(store *Store, seq int) (string, error) {
	var name string
	err := store.WriteTx(func(tx *sql.Tx) error {
		var ruleID int
		err := tx.QueryRow("SELECT id, name FROM rules WHERE seq = ?", seq).Scan(&ruleID, &name)
		if err != nil {
			return fmt.Errorf("rule %d not found", seq)
		}
		if name == "" {
			return fmt.Errorf("rule %d not found", seq)
		}
		var oldTags []string
		rows, err := tx.Query("SELECT t.name FROM tags t JOIN rule_tags rt ON t.id = rt.tag_id WHERE rt.rule_id = ?", ruleID)
		if err == nil {
			for rows.Next() {
				var tname string
				if err := rows.Scan(&tname); err != nil {
					rows.Close()
					return err
				}
				oldTags = append(oldTags, tname)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("DELETE FROM rule_tags WHERE rule_id = ?", ruleID); err != nil {
			return err
		}
		if len(oldTags) > 0 {
			if err := syncRuleTagFlags(tx, oldTags); err != nil {
				return err
			}
		}
		_, err = tx.Exec("UPDATE rules SET name = '' WHERE id = ?", ruleID)
		return err
	})
	return name, err
}

// DeleteRule soft-deletes a rule by emptying its name and clearing its tags.
func DeleteRule(store *Store, seq int) error {
	return store.WriteTx(func(tx *sql.Tx) error {
		var ruleID int
		var name string
		err := tx.QueryRow("SELECT id, name FROM rules WHERE seq = ?", seq).Scan(&ruleID, &name)
		if err != nil {
			return fmt.Errorf("rule %d not found", seq)
		}
		if name == "" {
			return fmt.Errorf("rule %d not found", seq)
		}
		// Get old tags for ruletag flag sync
		var oldTags []string
		rows, err := tx.Query("SELECT t.name FROM tags t JOIN rule_tags rt ON t.id = rt.tag_id WHERE rt.rule_id = ?", ruleID)
		if err == nil {
			for rows.Next() {
				var tname string
				if err := rows.Scan(&tname); err != nil {
					rows.Close()
					return err
				}
				oldTags = append(oldTags, tname)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("DELETE FROM rule_tags WHERE rule_id = ?", ruleID); err != nil {
			return err
		}
		if len(oldTags) > 0 {
			if err := syncRuleTagFlags(tx, oldTags); err != nil {
				return err
			}
		}
		_, err = tx.Exec("UPDATE rules SET name = '' WHERE id = ?", ruleID)
		return err
	})
}
