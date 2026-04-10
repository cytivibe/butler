package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// timestampFormat is the canonical format for storing timestamps.
// Includes timezone offset so local time is never lost.
const timestampFormat = "2006-01-02 15:04:05-07:00"

// nowLocal returns the current local time formatted for storage.
func nowLocal() string {
	return time.Now().Format(timestampFormat)
}

// timestampLayouts lists all formats we accept when parsing stored timestamps,
// ordered from most specific (with timezone) to least specific (UTC assumed).
var timestampLayouts = []string{
	timestampFormat,                // new format with timezone
	"2006-01-02 15:04:05",         // legacy UTC format (from CURRENT_TIMESTAMP)
	time.RFC3339,                   // ISO 8601
	"2006-01-02T15:04:05Z",        // UTC variant
}

// formatTimestamp formats a stored datetime string as "Jan 2" in local time.
func formatTimestamp(ts string) string {
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.Local().Format("Jan 2")
		}
	}
	return ts
}

// statusPreposition returns "since" for ongoing states, "on" for terminal/snapshot states.
// not_started is displayed as "created on".
func statusPreposition(status string) string {
	switch status {
	case "active", "waiting", "deferred":
		return "since"
	default:
		return "on"
	}
}

// formatStatusWithTime returns e.g. "[active since Mar 15]" or "[created on Mar 15]".
func formatStatusWithTime(status, timestamp string) string {
	ts := formatTimestamp(timestamp)
	if status == "not_started" {
		return fmt.Sprintf("[created on %s]", ts)
	}
	statusStr := strings.ReplaceAll(status, "_", " ")
	return fmt.Sprintf("[%s %s %s]", statusStr, statusPreposition(status), ts)
}

// parseDeadline accepts "2026-04-15" or "2026-04-15 14:00" and returns a normalized datetime string
// with timezone offset. Date-only input is stored as end of day (23:59) in local time.
func parseDeadline(s string) (string, error) {
	s = strings.TrimSpace(s)
	loc := time.Now().Location()
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, loc); err == nil {
		return t.Format(timestampFormat), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return t.Add(23*time.Hour + 59*time.Minute).Format(timestampFormat), nil
	}
	return "", fmt.Errorf("invalid deadline format: '%s' (use YYYY-MM-DD or YYYY-MM-DD HH:MM)", s)
}

// formatDeadline formats a deadline for display in local time. Shows time only if not 23:59 (date-only input).
func formatDeadline(dl string) string {
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, dl); err == nil {
			lt := t.Local()
			if lt.Hour() == 23 && lt.Minute() == 59 {
				return lt.Format("Jan 2")
			}
			return lt.Format("Jan 2 15:04")
		}
	}
	return dl
}

// isOverdue checks if a deadline has passed relative to now.
func isOverdue(dl string, now time.Time) bool {
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, dl); err == nil {
			return now.After(t)
		}
	}
	return false
}

// overduableStatuses are statuses where overdue applies.
var overduableStatuses = map[string]bool{
	"not_started": true,
	"active":      true,
	"waiting":     true,
	"reopened":    true,
}

// getInheritedDeadline walks up the ancestor chain to find the nearest deadline.
func getInheritedDeadline(tx *sql.Tx, taskID int) string {
	currentID := taskID
	for {
		var dl sql.NullString
		if err := tx.QueryRow("SELECT deadline FROM tasks WHERE id = ?", currentID).Scan(&dl); err != nil {
			return ""
		}
		if dl.Valid && dl.String != "" {
			return dl.String
		}
		var parentID sql.NullInt64
		if err := tx.QueryRow("SELECT parent_id FROM tasks WHERE id = ?", currentID).Scan(&parentID); err != nil {
			return ""
		}
		if !parentID.Valid {
			return ""
		}
		currentID = int(parentID.Int64)
	}
}

// validateRecur checks that a recurrence pattern is valid.
func validateRecur(pattern string) error {
	_, err := parseRecur(pattern)
	return err
}

type recurPattern struct {
	kind      string // "daily", "weekly", "monthly", "interval"
	days      []string
	timeOfDay string
	interval  int
	unit      string // "min", "h", "d", "w", "mon"
}

// parseRecur parses patterns like "daily", "daily 09:00", "weekly mon,thu 09:00",
// "monthly 1,15", "every 2d", "every 4h", "every 2mon", "every 30min".
func parseRecur(pattern string) (recurPattern, error) {
	parts := strings.Fields(pattern)
	if len(parts) == 0 {
		return recurPattern{}, fmt.Errorf("empty recurrence pattern")
	}

	if parts[0] == "every" {
		if len(parts) != 2 {
			return recurPattern{}, fmt.Errorf("invalid interval pattern: use 'every 2d', 'every 4h', 'every 3w', 'every 2mon', 'every 30min'")
		}
		s := parts[1]
		var numStr, unit string
		for i, c := range s {
			if c < '0' || c > '9' {
				numStr = s[:i]
				unit = s[i:]
				break
			}
		}
		if numStr == "" || unit == "" {
			return recurPattern{}, fmt.Errorf("invalid interval: '%s' (use 2d, 4h, 3w, 2mon, 30min)", s)
		}
		n, err := strconv.Atoi(numStr)
		if err != nil || n <= 0 {
			return recurPattern{}, fmt.Errorf("invalid interval number: '%s'", numStr)
		}
		switch unit {
		case "min", "h", "d", "w", "mon":
		default:
			return recurPattern{}, fmt.Errorf("invalid interval unit: '%s' (use min, h, d, w, mon)", unit)
		}
		return recurPattern{kind: "interval", interval: n, unit: unit}, nil
	}

	var rp recurPattern
	switch parts[0] {
	case "daily":
		rp.kind = "daily"
		if len(parts) > 1 {
			if err := validateTime(parts[1]); err != nil {
				return recurPattern{}, err
			}
			rp.timeOfDay = parts[1]
		}
	case "weekly":
		rp.kind = "weekly"
		if len(parts) > 1 && !isTimeStr(parts[1]) {
			rp.days = strings.Split(parts[1], ",")
			for _, d := range rp.days {
				if !isValidWeekday(d) {
					return recurPattern{}, fmt.Errorf("invalid weekday: '%s' (use mon,tue,wed,thu,fri,sat,sun)", d)
				}
			}
			if len(parts) > 2 {
				if err := validateTime(parts[2]); err != nil {
					return recurPattern{}, err
				}
				rp.timeOfDay = parts[2]
			}
		} else if len(parts) > 1 {
			if err := validateTime(parts[1]); err != nil {
				return recurPattern{}, err
			}
			rp.timeOfDay = parts[1]
		}
		if len(rp.days) == 0 {
			rp.days = []string{"mon"}
		}
	case "monthly":
		rp.kind = "monthly"
		if len(parts) > 1 && !isTimeStr(parts[1]) {
			for _, ds := range strings.Split(parts[1], ",") {
				d, err := strconv.Atoi(ds)
				if err != nil || d < 1 || d > 31 {
					return recurPattern{}, fmt.Errorf("invalid month day: '%s' (use 1-31)", ds)
				}
			}
			rp.days = strings.Split(parts[1], ",")
			if len(parts) > 2 {
				if err := validateTime(parts[2]); err != nil {
					return recurPattern{}, err
				}
				rp.timeOfDay = parts[2]
			}
		} else if len(parts) > 1 {
			if err := validateTime(parts[1]); err != nil {
				return recurPattern{}, err
			}
			rp.timeOfDay = parts[1]
		}
		if len(rp.days) == 0 {
			rp.days = []string{"1"}
		}
	default:
		return recurPattern{}, fmt.Errorf("invalid recurrence: '%s' (use daily, weekly, monthly, or every)", parts[0])
	}
	return rp, nil
}

func isTimeStr(s string) bool {
	return len(s) == 5 && s[2] == ':'
}

func validateTime(s string) error {
	if !isTimeStr(s) {
		return fmt.Errorf("invalid time format: '%s' (use HH:MM)", s)
	}
	h, err := strconv.Atoi(s[:2])
	if err != nil || h < 0 || h > 23 {
		return fmt.Errorf("invalid hour: '%s'", s[:2])
	}
	m, err := strconv.Atoi(s[3:])
	if err != nil || m < 0 || m > 59 {
		return fmt.Errorf("invalid minute: '%s'", s[3:])
	}
	return nil
}

func isValidWeekday(s string) bool {
	switch s {
	case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
		return true
	}
	return false
}

var weekdayMap = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday,
	"sat": time.Saturday,
}

// isRecurDue checks if a recurring task should be activated.
// createdAt is the task's creation time, now is current time.
func isRecurDue(pattern string, createdAt string, statusChangedAt string, status string, now time.Time) bool {
	// Only activate completed or not_started tasks
	if status != "completed" && status != "not_started" {
		return false
	}

	rp, err := parseRecur(pattern)
	if err != nil {
		return false
	}

	created, err := parseTimestamp(createdAt)
	if err != nil {
		return false
	}

	lastChanged, err := parseTimestamp(statusChangedAt)
	if err != nil {
		return false
	}

	switch rp.kind {
	case "interval":
		var d time.Duration
		switch rp.unit {
		case "min":
			d = time.Duration(rp.interval) * time.Minute
		case "h":
			d = time.Duration(rp.interval) * time.Hour
		case "d":
			d = time.Duration(rp.interval) * 24 * time.Hour
		case "w":
			d = time.Duration(rp.interval) * 7 * 24 * time.Hour
		case "mon":
			// For months, add N months from created_at and find the latest occurrence <= now
			next := created
			for next.Before(now) || next.Equal(now) {
				next = next.AddDate(0, rp.interval, 0)
			}
			// Go back one interval to find the most recent due time
			prev := next.AddDate(0, -rp.interval, 0)
			return now.After(prev) && lastChanged.Before(prev)
		}
		// Find latest interval occurrence based on created_at
		elapsed := now.Sub(created)
		if elapsed < 0 {
			return false
		}
		intervals := int(elapsed / d)
		lastDue := created.Add(time.Duration(intervals) * d)
		return lastChanged.Before(lastDue)

	case "daily":
		h, m := 0, 0
		if rp.timeOfDay != "" {
			fmt.Sscanf(rp.timeOfDay, "%d:%d", &h, &m)
		}
		todayDue := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		if now.Before(todayDue) {
			todayDue = todayDue.AddDate(0, 0, -1)
		}
		return lastChanged.Before(todayDue)

	case "weekly":
		h, m := 0, 0
		if rp.timeOfDay != "" {
			fmt.Sscanf(rp.timeOfDay, "%d:%d", &h, &m)
		}
		for _, dayStr := range rp.days {
			wd := weekdayMap[dayStr]
			// Find most recent occurrence of this weekday
			daysBack := (int(now.Weekday()) - int(wd) + 7) % 7
			candidate := time.Date(now.Year(), now.Month(), now.Day()-daysBack, h, m, 0, 0, now.Location())
			if now.Before(candidate) {
				candidate = candidate.AddDate(0, 0, -7)
			}
			if lastChanged.Before(candidate) {
				return true
			}
		}
		return false

	case "monthly":
		h, m := 0, 0
		if rp.timeOfDay != "" {
			fmt.Sscanf(rp.timeOfDay, "%d:%d", &h, &m)
		}
		for _, dayStr := range rp.days {
			day, _ := strconv.Atoi(dayStr)
			candidate := time.Date(now.Year(), now.Month(), day, h, m, 0, 0, now.Location())
			if now.Before(candidate) {
				candidate = candidate.AddDate(0, -1, 0)
			}
			if lastChanged.Before(candidate) {
				return true
			}
		}
		return false
	}
	return false
}

func parseTimestamp(ts string) (time.Time, error) {
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", ts)
}

// ActivateRecurring checks all recurring tasks and activates those that are due.
func ActivateRecurring(store *Store) error {
	now := time.Now()
	return store.WriteTx(func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT id, status, recur, created_at, status_changed_at FROM tasks WHERE recur IS NOT NULL AND recur != '' AND status NOT IN ('archived', 'cancelled', 'deferred')")
		if err != nil {
			return err
		}
		type recurTask struct {
			id              int
			status          string
			recur           string
			createdAt       string
			statusChangedAt string
		}
		var tasks []recurTask
		for rows.Next() {
			var t recurTask
			if err := rows.Scan(&t.id, &t.status, &t.recur, &t.createdAt, &t.statusChangedAt); err != nil {
				rows.Close()
				return err
			}
			tasks = append(tasks, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, t := range tasks {
			if isRecurDue(t.recur, t.createdAt, t.statusChangedAt, t.status, now) {
				if _, err := tx.Exec("UPDATE tasks SET status = 'active', status_changed_at = ? WHERE id = ?", nowLocal(), t.id); err != nil {
					return err
				}
				// Reset verify_status so each recurrence cycle requires fresh verification.
				if _, err := tx.Exec("UPDATE tasks SET verify_status = 'pending' WHERE id = ? AND verification != '' AND verify_status = 'passed'", t.id); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// parseTaskRef splits "Email boss:1.a" into ("Email boss", "1.a").
// Plain "Email boss" returns ("Email boss", "").
func parseTaskRef(ref string) (string, string) {
	idx := strings.LastIndex(ref, ":")
	if idx == -1 {
		return ref, ""
	}
	candidate := ref[idx+1:]
	if isPositionPath(candidate) {
		return ref[:idx], candidate
	}
	return ref, ""
}

func isPositionPath(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c != '.' && !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}

// isAncestor checks if potentialAncestorID is a parent/grandparent/etc of taskID.
func isAncestor(tx *sql.Tx, taskID, potentialAncestorID int) (bool, error) {
	currentID := taskID
	for {
		var parentID sql.NullInt64
		err := tx.QueryRow("SELECT parent_id FROM tasks WHERE id = ?", currentID).Scan(&parentID)
		if err != nil {
			return false, err
		}
		if !parentID.Valid {
			return false, nil
		}
		if int(parentID.Int64) == potentialAncestorID {
			return true, nil
		}
		currentID = int(parentID.Int64)
	}
}

func resolveTaskID(tx *sql.Tx, namedTask string, posPath string) (int, error) {
	var taskID int
	err := tx.QueryRow("SELECT id FROM tasks WHERE name = ? AND parent_id IS NULL ORDER BY CASE WHEN status = 'archived' THEN 1 ELSE 0 END, id DESC LIMIT 1", namedTask).Scan(&taskID)
	if err != nil {
		return 0, fmt.Errorf("named task '%s' not found", namedTask)
	}

	if posPath == "" {
		return taskID, nil
	}

	parts := strings.Split(posPath, ".")
	for _, part := range parts {
		var pos, par int
		if isAlphaSegment(part) {
			pos = alphaToNum(part)
			par = 1
		} else {
			pos, err = strconv.Atoi(part)
			if err != nil {
				return 0, fmt.Errorf("invalid position: %s", posPath)
			}
			par = 0
		}
		var childID int
		err = tx.QueryRow("SELECT id FROM tasks WHERE parent_id = ? AND position = ? AND parallel = ?", taskID, pos, par).Scan(&childID)
		if err != nil {
			return 0, fmt.Errorf("subtask at position %s not found", posPath)
		}
		taskID = childID
	}
	return taskID, nil
}

// getTaskPath walks up the parent chain to build the display path and find the root name.
func getTaskPath(tx *sql.Tx, taskID int) (namedTaskName string, path string, err error) {
	var segments []string
	currentID := taskID
	for {
		var name string
		var parentID sql.NullInt64
		var position sql.NullInt64
		var parallel bool
		err := tx.QueryRow("SELECT name, parent_id, position, parallel FROM tasks WHERE id = ?", currentID).Scan(&name, &parentID, &position, &parallel)
		if err != nil {
			return "", "", err
		}
		if !parentID.Valid {
			return name, strings.Join(segments, "."), nil
		}
		var posStr string
		if parallel {
			posStr = numToAlpha(int(position.Int64))
		} else {
			posStr = strconv.Itoa(int(position.Int64))
		}
		segments = append([]string{posStr}, segments...)
		currentID = int(parentID.Int64)
	}
}

// getBlockerDisplay builds a display string for all blockers of a task.
func getBlockerDisplay(tx *sql.Tx, taskID int, currentNamedTask string) string {
	rows, err := tx.Query("SELECT blocker_id FROM task_blockers WHERE task_id = ?", taskID)
	if err != nil {
		return ""
	}
	var blockerIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return ""
		}
		blockerIDs = append(blockerIDs, id)
	}
	rows.Close()
	if rows.Err() != nil {
		return ""
	}

	if len(blockerIDs) == 0 {
		return ""
	}

	// Same-task blockers first, then cross-task blockers
	var sameParts, crossParts []string
	for _, bid := range blockerIDs {
		namedTask, path, err := getTaskPath(tx, bid)
		if err != nil {
			continue
		}
		if namedTask == currentNamedTask {
			if path == "" {
				sameParts = append(sameParts, namedTask)
			} else {
				sameParts = append(sameParts, path)
			}
		} else {
			if path == "" {
				crossParts = append(crossParts, namedTask)
			} else {
				crossParts = append(crossParts, namedTask+"("+path+")")
			}
		}
	}
	return strings.Join(append(sameParts, crossParts...), ", ")
}

func numToAlpha(n int) string {
	var result string
	for n > 0 {
		n--
		result = string(rune('a'+n%26)) + result
		n /= 26
	}
	return result
}

func alphaToNum(s string) int {
	n := 0
	for _, c := range s {
		n = n*26 + int(c-'a') + 1
	}
	return n
}

func isAlphaSegment(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}
