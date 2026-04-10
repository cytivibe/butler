package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var validTransitions = map[string][]string{
	"not_started": {"active", "cancelled", "archived"},
	"active":      {"waiting", "deferred", "completed", "cancelled", "archived"},
	"waiting":     {"active", "cancelled", "archived"},
	"deferred":    {"active", "cancelled", "archived"},
	"completed":   {"reopened", "archived"},
	"reopened":    {"active", "cancelled", "archived"},
	"cancelled":   {"archived"},
	"archived":    {"active"},
}

func isValidStatus(s string) bool {
	_, ok := validTransitions[s]
	return ok
}

func isValidTransition(from, to string) bool {
	for _, s := range validTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// checkDuplicateTask checks if a non-archived task with the same name exists at the same level.
// parentID 0 means root level (parent_id IS NULL); >0 means under that parent.
// If force is true, it archives the existing task. Otherwise returns an error.
func checkDuplicateTask(tx *sql.Tx, name string, parentID int, force bool, excludeID int) error {
	var existingID int
	var existingStatus string
	var query string
	var args []interface{}
	if parentID == 0 {
		query = "SELECT id, status FROM tasks WHERE name = ? COLLATE NOCASE AND parent_id IS NULL AND id != ? AND status != 'archived'"
		args = []interface{}{name, excludeID}
	} else {
		query = "SELECT id, status FROM tasks WHERE name = ? COLLATE NOCASE AND parent_id = ? AND id != ? AND status != 'archived'"
		args = []interface{}{name, parentID, excludeID}
	}
	err := tx.QueryRow(query, args...).Scan(&existingID, &existingStatus)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if !force {
		return fmt.Errorf("task '%s' already exists (status: %s) - use --force to archive it and create a new one", name, existingStatus)
	}
	if _, err := tx.Exec("UPDATE tasks SET status = 'archived', status_changed_at = ? WHERE id = ?", nowLocal(), existingID); err != nil {
		return err
	}
	if err := archiveChildren(tx, existingID); err != nil {
		return err
	}
	return cleanupBlockers(tx, existingID)
}

const maxTaskNameLen = 50

func AddTask(store *Store, name string, under string, parallel bool, force bool, tags ...string) error {
	if len(name) > maxTaskNameLen {
		return fmt.Errorf("task name must be %d characters or less, got %d", maxTaskNameLen, len(name))
	}
	if under == "" {
		return store.WriteTx(func(tx *sql.Tx) error {
			if err := checkDuplicateTask(tx, name, 0, force, 0); err != nil {
				return err
			}
			now := nowLocal()
			result, err := tx.Exec("INSERT INTO tasks (name, created_at, status_changed_at) VALUES (?, ?, ?)", name, now, now)
			if err != nil {
				return err
			}
			if len(tags) > 0 {
				taskID, _ := result.LastInsertId()
				return insertTags(tx, "task_tags", "task_id", int(taskID), tags)
			}
			return nil
		})
	}

	namedTask, posPath := parseTaskRef(under)
	return store.WriteTx(func(tx *sql.Tx) error {
		parentID, err := resolveTaskID(tx, namedTask, posPath)
		if err != nil {
			return err
		}

		if err := checkDuplicateTask(tx, name, parentID, force, 0); err != nil {
			return err
		}

		parInt := 0
		if parallel {
			parInt = 1
		}
		var maxPos sql.NullInt64
		if err := tx.QueryRow("SELECT MAX(position) FROM tasks WHERE parent_id = ? AND parallel = ?", parentID, parInt).Scan(&maxPos); err != nil && err != sql.ErrNoRows {
			return err
		}
		nextPos := 1
		if maxPos.Valid {
			nextPos = int(maxPos.Int64) + 1
		}

		now := nowLocal()
		result, err := tx.Exec("INSERT INTO tasks (name, parent_id, position, parallel, created_at, status_changed_at) VALUES (?, ?, ?, ?, ?, ?)", name, parentID, nextPos, parInt, now, now)
		if err != nil {
			return err
		}

		if len(tags) > 0 {
			taskID, _ := result.LastInsertId()
			if err := insertTags(tx, "task_tags", "task_id", int(taskID), tags); err != nil {
				return err
			}
		}

		// If parent was completed, adding a new child reopens it (and its ancestors)
		var parentStatus string
		if err := tx.QueryRow("SELECT status FROM tasks WHERE id = ?", parentID).Scan(&parentStatus); err != nil && err != sql.ErrNoRows {
			return err
		}
		if parentStatus == "completed" {
			if _, err := tx.Exec("UPDATE tasks SET status = 'reopened', status_changed_at = ? WHERE id = ?", nowLocal(), parentID); err != nil {
				return err
			}
			return reopenAncestors(tx, parentID)
		}
		return nil
	})
}

// GetTaskOpts holds options for GetTasks.
// Depth: -1 = unlimited (default), 0 = task itself only, 1 = direct children, etc.
type GetTaskOpts struct {
	TaskRef string
	Details bool
	Status  string   // filter by status (empty = no filter)
	Tags    []string // filter by tags (AND: must match all; empty = no filter)
	NotTags []string // exclude by tags (AND-NOT: must match none; empty = no filter)
	Depth   int      // -1 = unlimited, 0+ = max depth
	Sort    string   // "recent" = sort by status_changed_at desc
}

// tasksMatchFilter checks if a task matches the Tags/NotTags filter.
func tasksMatchFilter(tx *sql.Tx, taskID int, tags, notTags []string) bool {
	for _, tag := range tags {
		if !taskHasTag(tx, taskID, tag) {
			return false
		}
	}
	for _, tag := range notTags {
		if taskHasTag(tx, taskID, tag) {
			return false
		}
	}
	return true
}

// validateTagsExist checks that all tags in the list exist in the database.
// NONE is exempt from this check.
func validateTagsExist(store *Store, tags []string) error {
	for _, tag := range tags {
		if tag == "NONE" {
			continue
		}
		exists, err := tagExists(store, tag)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("tag %q does not exist", tag)
		}
	}
	return nil
}

func GetTasks(store *Store, opts GetTaskOpts) ([]string, error) {
	if len(opts.Tags) > 0 {
		normalized, err := normalizeTagList(opts.Tags)
		if err != nil {
			return nil, err
		}
		opts.Tags = normalized
	}
	if len(opts.NotTags) > 0 {
		normalized, err := normalizeTagList(opts.NotTags)
		if err != nil {
			return nil, err
		}
		opts.NotTags = normalized
	}
	if len(opts.Tags) > 0 {
		if err := validateTagsExist(store, opts.Tags); err != nil {
			return nil, err
		}
	}
	if len(opts.NotTags) > 0 {
		if err := validateTagsExist(store, opts.NotTags); err != nil {
			return nil, err
		}
	}
	var lines []string
	err := store.ReadTx(func(tx *sql.Tx) error {
		now := time.Now()

		if opts.TaskRef != "" {
			namedTask, posPath := parseTaskRef(opts.TaskRef)
			taskID, err := resolveTaskID(tx, namedTask, posPath)
			if err != nil {
				return err
			}
			var name, status, description, verification, verifyStatus, createdAt, statusChangedAt string
			var recurNullable sql.NullString
			err = tx.QueryRow("SELECT name, status, description, verification, verify_status, created_at, status_changed_at, recur FROM tasks WHERE id = ?", taskID).Scan(&name, &status, &description, &verification, &verifyStatus, &createdAt, &statusChangedAt, &recurNullable)
			if err != nil {
				return err
			}
			recur := ""
			if recurNullable.Valid {
				recur = recurNullable.String
			}
			deadline := getInheritedDeadline(tx, taskID)
			rootName, posPrefix, _ := getTaskPath(tx, taskID)
			tagMatch := tasksMatchFilter(tx, taskID, opts.Tags, opts.NotTags)
			if (opts.Status == "" || status == opts.Status) && tagMatch {
				// Build line with inherited tags instead of direct tags
				line := formatTaskLine("", posPrefix, name, status, statusChangedAt, deadline, recur, verifyStatus, now, taskID, rootName, tx)
				inheritedTags := getInheritedTagNames(tx, taskID)
				directTags := getEntityTags(tx, "task_tags", "task_id", taskID)
				// Append parent tags not already shown
				for _, tag := range inheritedTags {
					if !strings.Contains(directTags, "#"+tag) {
						line += " #" + tag
					}
				}
				lines = append(lines, line)
				if opts.Details {
					detailPrefix := ""
					if cliColor && opts.Depth != 0 && hasChildren(tx, taskID) {
						detailPrefix = colorTree(treePipe)
					}
					appendDetails(&lines, detailPrefix, description, verification, verifyStatus, createdAt)
					rulesMap := getRulesForTags(tx, inheritedTags)
					appendRules(&lines, detailPrefix, inheritedTags, rulesMap)
				}
			}
			if opts.Depth != 0 {
				return collectSubTasksFiltered(subTaskContext{
					tx: tx, prefix: posPrefix, indent: "  ", namedTaskName: rootName,
					parentStatus: status, parentStatusTime: statusChangedAt, parentDeadline: deadline,
					now: now, opts: opts, currentDepth: 1, parentMatchedTag: tagMatch,
				}, taskID, &lines)
			}
			return nil
		}

		orderBy := "id"
		if opts.Sort == "recent" {
			orderBy = "CASE WHEN status = 'not_started' THEN created_at ELSE status_changed_at END DESC"
		} else if opts.Sort == "deadline" {
			orderBy = "CASE WHEN deadline IS NULL THEN 1 ELSE 0 END, deadline ASC"
		}
		rows, err := tx.Query(fmt.Sprintf("SELECT id, name, status, description, verification, verify_status, created_at, status_changed_at, deadline, recur FROM tasks WHERE parent_id IS NULL ORDER BY %s", orderBy))
		if err != nil {
			return err
		}
		type namedTaskRow struct {
			id              int
			name            string
			status          string
			description     string
			verification    string
			verifyStatus    string
			createdAt       string
			statusChangedAt string
			deadline        string
			recur           string
		}
		var named []namedTaskRow
		for rows.Next() {
			var t namedTaskRow
			var dl, rc sql.NullString
			if err := rows.Scan(&t.id, &t.name, &t.status, &t.description, &t.verification, &t.verifyStatus, &t.createdAt, &t.statusChangedAt, &dl, &rc); err != nil {
				rows.Close()
				return err
			}
			if dl.Valid {
				t.deadline = dl.String
			}
			if rc.Valid {
				t.recur = rc.String
			}
			named = append(named, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		firstShown := true
		for _, t := range named {
			// Hide archived tasks unless explicitly filtering for them
			if t.status == "archived" && opts.Status != "archived" {
				continue
			}
			statusMatch := opts.Status == "" || t.status == opts.Status
			tagMatch := tasksMatchFilter(tx, t.id, opts.Tags, opts.NotTags)
			if statusMatch && tagMatch {
				// Blank line between root tasks for readability
				if cliColor && !firstShown {
					lines = append(lines, "")
				}
				firstShown = false
				lines = append(lines, formatTaskLine("", "", t.name, t.status, t.statusChangedAt, t.deadline, t.recur, t.verifyStatus, now, t.id, t.name, tx))
				if opts.Details {
					// Use │ prefix for details when task has children (so tree connects)
					detailPrefix := ""
					if cliColor && opts.Depth != 0 && hasChildren(tx, t.id) {
						detailPrefix = colorTree(treePipe)
					}
					appendDetails(&lines, detailPrefix, t.description, t.verification, t.verifyStatus, t.createdAt)
					directTags := getInheritedTagNames(tx, t.id)
					rulesMap := getRulesForTags(tx, directTags)
					appendRules(&lines, detailPrefix, directTags, rulesMap)
				}
			}
			if opts.Depth != 0 {
				if err := collectSubTasksFiltered(subTaskContext{
					tx: tx, prefix: "", indent: "  ", namedTaskName: t.name,
					parentStatus: t.status, parentStatusTime: t.statusChangedAt, parentDeadline: t.deadline,
					now: now, opts: opts, currentDepth: 1, parentMatchedTag: tagMatch,
				}, t.id, &lines); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return lines, err
}

func appendDetails(lines *[]string, prefix string, description, verification, verifyStatus, createdAt string) {
	*lines = append(*lines, fmt.Sprintf("%s  %s %s", prefix, colorDetailLabel("created:"), colorDimText(formatTimestamp(createdAt))))
	if description != "" {
		*lines = append(*lines, fmt.Sprintf("%s  %s %s", prefix, colorDetailLabel("desc:"), colorDimText(description)))
	}
	if verification != "" {
		*lines = append(*lines, fmt.Sprintf("%s  %s %s", prefix, colorDetailLabel("verify:"), colorDimText(verification)))
	}
	if verifyStatus != "" {
		*lines = append(*lines, fmt.Sprintf("%s  %s %s", prefix, colorDetailLabel("verify status:"), colorDimText(verifyStatus)))
	}
}

// SetTaskOpts holds optional fields for SetTask.
// Nil pointers mean "don't change". Empty Status means "don't change".
type SetTaskOpts struct {
	Name         *string  // new task name (nil = don't change)
	Status       string   // "wait" requires Blockers; "waiting" is invalid (use "wait")
	Blockers     []string // blocker refs, replaces existing blockers when Status == "wait"
	Desc         *string
	Verify       *string
	VerifyStatus *string // "passed" or "pending" (nil = don't change)
	Deadline     *string // "none" clears, date or datetime string sets
	Recur        *string // "none" clears, recurrence pattern string sets
	SetTags      bool    // if true, replace all tags with Tags
	Tags         []string // tag names (only used when SetTags is true)
	Force        bool    // if true, archive conflicting task when renaming
}

// inheritedStatuses are parent statuses that override children at display time.
var inheritedStatuses = map[string]bool{
	"cancelled": true,
	"deferred":  true,
	"waiting":   true,
	"archived":  true,
}

// subTaskContext holds the traversal state for recursive subtask collection.
type subTaskContext struct {
	tx               *sql.Tx
	prefix           string
	indent           string     // plain indent for MCP
	treePrefix       string     // tree continuation prefix for CLI (e.g. "│   │   ")
	namedTaskName    string
	parentStatus     string
	parentStatusTime string
	parentDeadline   string
	now              time.Time
	opts             GetTaskOpts
	currentDepth     int
	parentMatchedTag bool
}

func collectSubTasksFiltered(ctx subTaskContext, parentID int, lines *[]string) error {
	tx := ctx.tx
	rows, err := tx.Query("SELECT id, name, position, parallel, status, description, verification, verify_status, created_at, status_changed_at, deadline, recur FROM tasks WHERE parent_id = ? ORDER BY parallel DESC, position", parentID)
	if err != nil {
		return err
	}
	type sub struct {
		id              int
		name            string
		position        int
		parallel        bool
		status          string
		description     string
		verification    string
		verifyStatus    string
		createdAt       string
		statusChangedAt string
		deadline        string
		recur           string
	}
	var subs []sub
	for rows.Next() {
		var s sub
		var dl, rc sql.NullString
		if err := rows.Scan(&s.id, &s.name, &s.position, &s.parallel, &s.status, &s.description, &s.verification, &s.verifyStatus, &s.createdAt, &s.statusChangedAt, &dl, &rc); err != nil {
			rows.Close()
			return err
		}
		if dl.Valid {
			s.deadline = dl.String
		}
		if rc.Valid {
			s.recur = rc.String
		}
		subs = append(subs, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Build visible list (after filtering archived) so we know which is last
	type visibleSub struct {
		sub
		path string
		displayStatus string
		displayTime   string
		displayDeadline string
	}
	var visible []visibleSub
	for _, s := range subs {
		var posStr string
		if s.parallel {
			posStr = numToAlpha(s.position)
		} else {
			posStr = strconv.Itoa(s.position)
		}
		path := posStr
		if ctx.prefix != "" {
			path = ctx.prefix + "." + posStr
		}
		ds := s.status
		dt := s.statusChangedAt
		if inheritedStatuses[ctx.parentStatus] {
			ds = ctx.parentStatus
			dt = ctx.parentStatusTime
		}
		dd := s.deadline
		if dd == "" {
			dd = ctx.parentDeadline
		}
		if ds == "archived" && ctx.opts.Status != "archived" {
			continue
		}
		visible = append(visible, visibleSub{sub: s, path: path, displayStatus: ds, displayTime: dt, displayDeadline: dd})
	}

	for i, v := range visible {
		isLast := i == len(visible)-1
		statusMatch := ctx.opts.Status == "" || v.displayStatus == ctx.opts.Status
		tagMatch := (len(ctx.opts.Tags) == 0 && len(ctx.opts.NotTags) == 0) || (ctx.parentMatchedTag && len(ctx.opts.NotTags) == 0) || tasksMatchFilter(tx, v.id, ctx.opts.Tags, ctx.opts.NotTags)

		// Tree connector for this item
		var linePrefix, detailPrefix string
		if cliColor {
			if isLast {
				linePrefix = ctx.treePrefix + colorTree(treeLastItem)
			} else {
				linePrefix = ctx.treePrefix + colorTree(treeBranch)
			}
			if isLast {
				detailPrefix = ctx.treePrefix + colorTree(treeBlank)
			} else {
				detailPrefix = ctx.treePrefix + colorTree(treePipe)
			}
		} else {
			linePrefix = ctx.indent
			detailPrefix = ctx.indent
		}

		if statusMatch && tagMatch {
			lines = appendLine(lines, formatTaskLine(linePrefix, v.path, v.name, v.displayStatus, v.displayTime, v.displayDeadline, v.recur, v.verifyStatus, ctx.now, v.id, ctx.namedTaskName, tx))
			if ctx.opts.Details {
				// If this subtask has children, add │ pipe so details connect to the tree below
				dp := detailPrefix
				if cliColor && (ctx.opts.Depth == -1 || ctx.currentDepth < ctx.opts.Depth) && hasChildren(tx, v.id) {
					dp = detailPrefix + colorTree(treePipe)
				}
				appendDetails(lines, dp, v.description, v.verification, v.verifyStatus, v.createdAt)
				directTags := getDirectTagNames(tx, v.id)
				if len(directTags) > 0 {
					rulesMap := getRulesForTags(tx, directTags)
					appendRules(lines, dp, directTags, rulesMap)
				}
			}
		}
		if ctx.opts.Depth == -1 || ctx.currentDepth < ctx.opts.Depth {
			childCtx := ctx
			childCtx.prefix = v.path
			childCtx.indent = ctx.indent + "  "
			if cliColor {
				if isLast {
					childCtx.treePrefix = ctx.treePrefix + colorTree(treeBlank)
				} else {
					childCtx.treePrefix = ctx.treePrefix + colorTree(treePipe)
				}
			}
			childCtx.parentStatus = v.displayStatus
			childCtx.parentStatusTime = v.displayTime
			childCtx.parentDeadline = v.displayDeadline
			childCtx.currentDepth = ctx.currentDepth + 1
			childCtx.parentMatchedTag = tagMatch
			if err := collectSubTasksFiltered(childCtx, v.id, lines); err != nil {
				return err
			}
		}
	}
	return nil
}

// hasChildren checks if a task has any child tasks.
func hasChildren(tx *sql.Tx, taskID int) bool {
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM tasks WHERE parent_id = ?", taskID).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func appendLine(lines *[]string, line string) *[]string {
	*lines = append(*lines, line)
	return lines
}

func formatTaskLine(prefix, path, name, status, statusTime, deadline, recur, verifyStatus string, now time.Time, taskID int, namedTaskName string, tx *sql.Tx) string {
	statusTag := colorStatus(status, statusTime)
	displayName := colorTaskName(name)
	var line string
	if path == "" {
		line = fmt.Sprintf("%s%s %s", colorRootMarker(), displayName, statusTag)
	} else {
		line = fmt.Sprintf("%s%s %s %s", prefix, colorPosition(path), displayName, statusTag)
	}
	if status == "waiting" {
		blockers := getBlockerDisplay(tx, taskID, namedTaskName)
		if blockers != "" {
			ts := formatTimestamp(statusTime)
			waitBadge := colorWaiting(blockers, ts)
			if path == "" {
				line = fmt.Sprintf("%s%s %s", colorRootMarker(), displayName, waitBadge)
			} else {
				line = fmt.Sprintf("%s%s %s %s", prefix, colorPosition(path), displayName, waitBadge)
			}
		}
	}
	if deadline != "" {
		dl := formatDeadline(deadline)
		if overduableStatuses[status] && isOverdue(deadline, now) {
			line += " " + colorOverdue(dl)
		} else {
			line += " " + colorDeadline(dl)
		}
	}
	if recur != "" {
		line += " " + colorRecur(recur)
	}
	if verifyStatus == "pending" {
		line += " " + colorVerifyPending()
	} else if verifyStatus == "passed" {
		line += " " + colorVerifyPassed()
	}
	tags := getEntityTags(tx, "task_tags", "task_id", taskID)
	if tags != "" {
		line += " " + colorTags(tags)
	}
	return line
}

func SetTask(store *Store, taskRef string, opts SetTaskOpts) error {
	namedTask, posPath := parseTaskRef(taskRef)

	return store.WriteTx(func(tx *sql.Tx) error {
		taskID, err := resolveTaskID(tx, namedTask, posPath)
		if err != nil {
			return err
		}

		if opts.Name != nil {
			if len(*opts.Name) > maxTaskNameLen {
				return fmt.Errorf("task name must be %d characters or less, got %d", maxTaskNameLen, len(*opts.Name))
			}
			var parentID sql.NullInt64
			if err := tx.QueryRow("SELECT parent_id FROM tasks WHERE id = ?", taskID).Scan(&parentID); err != nil {
				return err
			}
			dupParent := 0
			if parentID.Valid {
				dupParent = int(parentID.Int64)
			}
			if err := checkDuplicateTask(tx, *opts.Name, dupParent, opts.Force, taskID); err != nil {
				return err
			}
			if _, err := tx.Exec("UPDATE tasks SET name = ? WHERE id = ?", *opts.Name, taskID); err != nil {
				return err
			}
		}
		if opts.Desc != nil {
			if _, err := tx.Exec("UPDATE tasks SET description = ? WHERE id = ?", *opts.Desc, taskID); err != nil {
				return err
			}
		}
		if opts.Verify != nil && opts.VerifyStatus != nil {
			return fmt.Errorf("cannot set --verify and --verify-status in the same call")
		}
		if opts.Verify != nil {
			if strings.EqualFold(*opts.Verify, "none") {
				// Clear both criteria and status
				if _, err := tx.Exec("UPDATE tasks SET verification = '', verify_status = '' WHERE id = ?", taskID); err != nil {
					return err
				}
			} else {
				// Set criteria and auto-set status to pending
				if _, err := tx.Exec("UPDATE tasks SET verification = ?, verify_status = 'pending' WHERE id = ?", *opts.Verify, taskID); err != nil {
					return err
				}
			}
		}
		if opts.VerifyStatus != nil {
			var currentVerification string
			if err := tx.QueryRow("SELECT verification FROM tasks WHERE id = ?", taskID).Scan(&currentVerification); err != nil {
				return err
			}
			if currentVerification == "" {
				return fmt.Errorf("cannot set verify status: task has no verification criteria")
			}
			vs := strings.ToLower(*opts.VerifyStatus)
			if vs != "passed" && vs != "pending" {
				return fmt.Errorf("invalid verify status '%s': must be 'passed' or 'pending'", *opts.VerifyStatus)
			}
			if _, err := tx.Exec("UPDATE tasks SET verify_status = ? WHERE id = ?", vs, taskID); err != nil {
				return err
			}
		}
		if opts.Deadline != nil {
			if *opts.Deadline == "none" || *opts.Deadline == "" {
				if _, err := tx.Exec("UPDATE tasks SET deadline = NULL WHERE id = ?", taskID); err != nil {
					return err
				}
			} else {
				dl, err := parseDeadline(*opts.Deadline)
				if err != nil {
					return err
				}
				if _, err := tx.Exec("UPDATE tasks SET deadline = ? WHERE id = ?", dl, taskID); err != nil {
					return err
				}
			}
		}
		if opts.Recur != nil {
			if *opts.Recur == "none" || *opts.Recur == "" {
				if _, err := tx.Exec("UPDATE tasks SET recur = NULL WHERE id = ?", taskID); err != nil {
					return err
				}
			} else {
				if err := validateRecur(*opts.Recur); err != nil {
					return err
				}
				if _, err := tx.Exec("UPDATE tasks SET recur = ? WHERE id = ?", *opts.Recur, taskID); err != nil {
					return err
				}
			}
		}
		if opts.SetTags {
			if _, err := tx.Exec("DELETE FROM task_tags WHERE task_id = ?", taskID); err != nil {
				return err
			}
			if len(opts.Tags) > 0 {
				if err := insertTags(tx, "task_tags", "task_id", taskID, opts.Tags); err != nil {
					return err
				}
			}
		}

		if opts.Status == "" {
			return nil
		}

		if opts.Status == "waiting" {
			return fmt.Errorf("use 'wait' instead of 'waiting'")
		}

		if opts.Status == "wait" {
			if len(opts.Blockers) == 0 {
				return fmt.Errorf("--status wait requires at least one blocker")
			}

			var currentStatus string
			err = tx.QueryRow("SELECT status FROM tasks WHERE id = ?", taskID).Scan(&currentStatus)
			if err != nil {
				return err
			}
			if currentStatus != "active" && currentStatus != "waiting" {
				return fmt.Errorf("can only set wait from 'active' or 'waiting', current: '%s'", currentStatus)
			}

			// Clear existing blockers (replace semantics)
			if _, err := tx.Exec("DELETE FROM task_blockers WHERE task_id = ?", taskID); err != nil {
				return err
			}

			for _, blockerRef := range opts.Blockers {
				bNamed, bPos := parseTaskRef(blockerRef)
				blockerID, err := resolveTaskID(tx, bNamed, bPos)
				if err != nil {
					return fmt.Errorf("blocker task: %w", err)
				}
				if taskID == blockerID {
					return fmt.Errorf("a task cannot wait on itself")
				}
				if ancestor, _ := isAncestor(tx, taskID, blockerID); ancestor {
					return fmt.Errorf("a subtask cannot wait on its own ancestor - would deadlock")
				}
				if _, err := tx.Exec("INSERT INTO task_blockers (task_id, blocker_id) VALUES (?, ?)", taskID, blockerID); err != nil {
					return err
				}
			}

			_, err = tx.Exec("UPDATE tasks SET status = 'waiting', status_changed_at = ? WHERE id = ?", nowLocal(), taskID)
			return err
		}

		// Normal status transition
		newStatus := opts.Status
		if !isValidStatus(newStatus) {
			return fmt.Errorf("invalid status: '%s'", newStatus)
		}

		var currentStatus string
		err = tx.QueryRow("SELECT status FROM tasks WHERE id = ?", taskID).Scan(&currentStatus)
		if err != nil {
			return err
		}

		if !isValidTransition(currentStatus, newStatus) {
			return fmt.Errorf("cannot transition from '%s' to '%s'", currentStatus, newStatus)
		}

		if newStatus == "completed" {
			var incomplete int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND status NOT IN ('completed','cancelled')`, taskID).Scan(&incomplete); err != nil {
				return err
			}
			if incomplete > 0 {
				return fmt.Errorf("cannot complete: %d child task(s) still incomplete", incomplete)
			}
			var verifyStatus string
			if err := tx.QueryRow("SELECT verify_status FROM tasks WHERE id = ?", taskID).Scan(&verifyStatus); err != nil {
				return err
			}
			if verifyStatus != "" && verifyStatus != "passed" {
				return fmt.Errorf("cannot complete: verification not passed (use --verify-status passed first)")
			}
		}

		_, err = tx.Exec("UPDATE tasks SET status = ?, status_changed_at = ? WHERE id = ?", newStatus, nowLocal(), taskID)
		if err != nil {
			return err
		}

		if newStatus == "active" {
			if _, err := tx.Exec("DELETE FROM task_blockers WHERE task_id = ?", taskID); err != nil {
				return err
			}
			if err := activateAncestors(tx, taskID); err != nil {
				return err
			}
		}

		if newStatus == "reopened" {
			if err := reopenAncestors(tx, taskID); err != nil {
				return err
			}
			// Reset verification to pending if it was passed
			if _, err := tx.Exec("UPDATE tasks SET verify_status = 'pending' WHERE id = ? AND verify_status = 'passed'", taskID); err != nil {
				return err
			}
		}

		if newStatus == "completed" {
			return checkAutoTransitions(tx, taskID)
		}

		if newStatus == "archived" {
			// Only named (top-level) tasks can be archived directly
			var parentID sql.NullInt64
			if err := tx.QueryRow("SELECT parent_id FROM tasks WHERE id = ?", taskID).Scan(&parentID); err != nil {
				return err
			}
			if parentID.Valid {
				return fmt.Errorf("only named tasks can be archived - archive the parent task instead")
			}
			if err := archiveChildren(tx, taskID); err != nil {
				return err
			}
			return cleanupBlockers(tx, taskID)
		}
		return nil
	})
}

// activateAncestors walks up and sets any not_started parent to active.
func activateAncestors(tx *sql.Tx, taskID int) error {
	var parentID sql.NullInt64
	if err := tx.QueryRow("SELECT parent_id FROM tasks WHERE id = ?", taskID).Scan(&parentID); err != nil {
		return err
	}
	if !parentID.Valid {
		return nil
	}
	pid := int(parentID.Int64)
	var status string
	if err := tx.QueryRow("SELECT status FROM tasks WHERE id = ?", pid).Scan(&status); err != nil {
		return err
	}
	if status == "not_started" {
		if _, err := tx.Exec("UPDATE tasks SET status = 'active', status_changed_at = ? WHERE id = ?", nowLocal(), pid); err != nil {
			return err
		}
		return activateAncestors(tx, pid)
	}
	return nil
}

// reopenAncestors walks up and sets any completed parent to reopened.
func reopenAncestors(tx *sql.Tx, taskID int) error {
	var parentID sql.NullInt64
	if err := tx.QueryRow("SELECT parent_id FROM tasks WHERE id = ?", taskID).Scan(&parentID); err != nil {
		return err
	}
	if !parentID.Valid {
		return nil
	}
	pid := int(parentID.Int64)
	var status string
	if err := tx.QueryRow("SELECT status FROM tasks WHERE id = ?", pid).Scan(&status); err != nil {
		return err
	}
	if status == "completed" {
		if _, err := tx.Exec("UPDATE tasks SET status = 'reopened', status_changed_at = ? WHERE id = ?", nowLocal(), pid); err != nil {
			return err
		}
		return reopenAncestors(tx, pid)
	}
	return nil
}

// checkAutoTransitions transitions waiting tasks to active when all their blockers are completed.
func checkAutoTransitions(tx *sql.Tx, completedTaskID int) error {
	rows, err := tx.Query("SELECT DISTINCT task_id FROM task_blockers WHERE blocker_id = ?", completedTaskID)
	if err != nil {
		return err
	}
	var waitingTasks []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		waitingTasks = append(waitingTasks, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, taskID := range waitingTasks {
		var incompleteCount int
		err := tx.QueryRow(`
			SELECT COUNT(*) FROM task_blockers tb
			JOIN tasks t ON tb.blocker_id = t.id
			WHERE tb.task_id = ? AND t.status != 'completed'
		`, taskID).Scan(&incompleteCount)
		if err != nil {
			return err
		}

		if incompleteCount == 0 {
			_, err = tx.Exec("UPDATE tasks SET status = 'active', status_changed_at = ? WHERE id = ? AND status = 'waiting'", nowLocal(), taskID)
			if err != nil {
				return err
			}
			_, err = tx.Exec("DELETE FROM task_blockers WHERE task_id = ?", taskID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// archiveChildren recursively sets all children of a task to archived.
func archiveChildren(tx *sql.Tx, parentID int) error {
	rows, err := tx.Query("SELECT id FROM tasks WHERE parent_id = ?", parentID)
	if err != nil {
		return err
	}
	var childIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		childIDs = append(childIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, childID := range childIDs {
		if _, err := tx.Exec("UPDATE tasks SET status = 'archived', status_changed_at = ? WHERE id = ?", nowLocal(), childID); err != nil {
			return err
		}
		if err := cleanupBlockers(tx, childID); err != nil {
			return err
		}
		if err := archiveChildren(tx, childID); err != nil {
			return err
		}
	}
	return nil
}

// cleanupBlockers removes a task from all blocker lists.
// If a waiting task has no blockers left, it auto-transitions to active.
func cleanupBlockers(tx *sql.Tx, taskID int) error {
	rows, err := tx.Query("SELECT DISTINCT task_id FROM task_blockers WHERE blocker_id = ?", taskID)
	if err != nil {
		return err
	}
	var waitingTasks []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		waitingTasks = append(waitingTasks, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM task_blockers WHERE blocker_id = ?", taskID); err != nil {
		return err
	}

	for _, wt := range waitingTasks {
		var remaining int
		if err := tx.QueryRow("SELECT COUNT(*) FROM task_blockers WHERE task_id = ?", wt).Scan(&remaining); err != nil {
			return err
		}
		if remaining == 0 {
			// Auto-transition to active (only if still waiting)
			if _, err := tx.Exec("UPDATE tasks SET status = 'active', status_changed_at = ? WHERE id = ? AND status = 'waiting'", nowLocal(), wt); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteTask permanently deletes a task and all its children.
func DeleteTask(store *Store, taskRef string) (name string, childCount int, err error) {
	namedTask, posPath := parseTaskRef(taskRef)
	err = store.ReadTx(func(tx *sql.Tx) error {
		taskID, err := resolveTaskID(tx, namedTask, posPath)
		if err != nil {
			return err
		}
		if err := tx.QueryRow("SELECT name FROM tasks WHERE id = ?", taskID).Scan(&name); err != nil {
			return err
		}
		var err2 error
		childCount, err2 = countChildren(tx, taskID)
		return err2
	})
	return
}

// DeleteTaskConfirmed permanently deletes a task and all its children.
func DeleteTaskConfirmed(store *Store, taskRef string) error {
	namedTask, posPath := parseTaskRef(taskRef)
	return store.WriteTx(func(tx *sql.Tx) error {
		taskID, err := resolveTaskID(tx, namedTask, posPath)
		if err != nil {
			return err
		}
		if err := cleanupBlockersRecursive(tx, taskID); err != nil {
			return err
		}
		return deleteTaskTree(tx, taskID)
	})
}

// DeleteTaskAtomic combines info gathering and deletion in a single WriteTx.
// Returns name, child count, and any error. Used by MCP to avoid TOCTOU.
func DeleteTaskAtomic(store *Store, taskRef string) (name string, childCount int, err error) {
	namedTask, posPath := parseTaskRef(taskRef)
	err = store.WriteTx(func(tx *sql.Tx) error {
		taskID, err := resolveTaskID(tx, namedTask, posPath)
		if err != nil {
			return err
		}
		if err := tx.QueryRow("SELECT name FROM tasks WHERE id = ?", taskID).Scan(&name); err != nil {
			return err
		}
		var err2 error
		childCount, err2 = countChildren(tx, taskID)
		if err2 != nil {
			return err2
		}
		if err := cleanupBlockersRecursive(tx, taskID); err != nil {
			return err
		}
		return deleteTaskTree(tx, taskID)
	})
	return
}

// cleanupBlockersRecursive cleans up blockers for a task and all its children.
func cleanupBlockersRecursive(tx *sql.Tx, taskID int) error {
	if err := cleanupBlockers(tx, taskID); err != nil {
		return err
	}
	rows, err := tx.Query("SELECT id FROM tasks WHERE parent_id = ?", taskID)
	if err != nil {
		return err
	}
	var childIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		childIDs = append(childIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, childID := range childIDs {
		if err := cleanupBlockersRecursive(tx, childID); err != nil {
			return err
		}
	}
	return nil
}

// deleteTaskTree deletes a task and all its children recursively.
func deleteTaskTree(tx *sql.Tx, taskID int) error {
	// Delete children first (depth-first)
	rows, err := tx.Query("SELECT id FROM tasks WHERE parent_id = ?", taskID)
	if err != nil {
		return err
	}
	var childIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		childIDs = append(childIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, childID := range childIDs {
		if err := deleteTaskTree(tx, childID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM task_blockers WHERE task_id = ? OR blocker_id = ?", taskID, taskID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM task_tags WHERE task_id = ?", taskID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM tasks WHERE id = ?", taskID); err != nil {
		return err
	}
	return nil
}

// countChildren counts all descendants of a task.
func countChildren(tx *sql.Tx, taskID int) (int, error) {
	rows, err := tx.Query("SELECT id FROM tasks WHERE parent_id = ?", taskID)
	if err != nil {
		return 0, err
	}
	count := 0
	var childIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return count, err
		}
		childIDs = append(childIDs, id)
		count++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return count, err
	}
	for _, childID := range childIDs {
		n, err := countChildren(tx, childID)
		count += n
		if err != nil {
			return count, err
		}
	}
	return count, nil
}
