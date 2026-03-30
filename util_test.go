package main

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestIsAlphaSegment(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"a", true},
		{"z", true},
		{"ab", true},
		{"abc", true},
		{"aa", true},
		{"1", false},
		{"123", false},
		{"1a", false},
		{"a1", false},
		{"1a2", false},
		{"A", false},
		{"Ab", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isAlphaSegment(tt.input); got != tt.want {
			t.Errorf("isAlphaSegment(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNumToAlpha(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "a"},
		{2, "b"},
		{3, "c"},
		{26, "z"},
		{27, "aa"},
		{28, "ab"},
		{52, "az"},
		{53, "ba"},
	}
	for _, tt := range tests {
		got := numToAlpha(tt.n)
		if got != tt.want {
			t.Errorf("numToAlpha(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestAlphaToNum(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"a", 1},
		{"b", 2},
		{"z", 26},
		{"aa", 27},
		{"ab", 28},
		{"az", 52},
		{"ba", 53},
	}
	for _, tt := range tests {
		got := alphaToNum(tt.s)
		if got != tt.want {
			t.Errorf("alphaToNum(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestNumToAlphaRoundTrip(t *testing.T) {
	for i := 1; i <= 100; i++ {
		s := numToAlpha(i)
		got := alphaToNum(s)
		if got != i {
			t.Errorf("roundtrip failed: %d -> %q -> %d", i, s, got)
		}
	}
}

func TestIsRecurDueDaily(t *testing.T) {
	now := time.Date(2026, 3, 30, 15, 0, 0, 0, time.Local)
	created := now.Add(-48 * time.Hour).Format(timestampFormat)
	lastChanged := now.Add(-25 * time.Hour).Format(timestampFormat)
	// Last changed 25 hours ago, daily recurrence -> should be due
	if !isRecurDue("daily", created, lastChanged, "completed", now) {
		t.Error("expected daily task to be due")
	}
}

func TestIsRecurDueDailyWithTime(t *testing.T) {
	now := time.Date(2026, 3, 30, 10, 0, 0, 0, time.Local)
	created := now.Add(-48 * time.Hour).Format(timestampFormat)
	lastChanged := now.Add(-25 * time.Hour).Format(timestampFormat)

	if !isRecurDue("daily 09:00", created, lastChanged, "completed", now) {
		t.Error("expected daily 09:00 to be due at 10:00")
	}

	// Before the time -> not yet due today, but was due yesterday
	nowBefore := time.Date(2026, 3, 30, 8, 0, 0, 0, time.Local)
	lastJustChanged := nowBefore.Add(-1 * time.Hour).Format(timestampFormat)
	if isRecurDue("daily 09:00", created, lastJustChanged, "completed", nowBefore) {
		t.Error("should not be due before 09:00 if changed 1h ago")
	}
}

func TestIsRecurDueWeekly(t *testing.T) {
	// Monday at noon
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.Local) // Monday
	created := now.Add(-14 * 24 * time.Hour).Format(timestampFormat)
	lastChanged := now.Add(-8 * 24 * time.Hour).Format(timestampFormat)

	if !isRecurDue("weekly mon", created, lastChanged, "completed", now) {
		t.Error("expected weekly mon task to be due on Monday")
	}
}

func TestIsRecurDueMonthly(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.Local)
	created := now.Add(-60 * 24 * time.Hour).Format(timestampFormat)
	lastChanged := now.Add(-20 * 24 * time.Hour).Format(timestampFormat)

	if !isRecurDue("monthly 15", created, lastChanged, "completed", now) {
		t.Error("expected monthly 15 task to be due on the 15th")
	}
}

func TestIsRecurDueInterval(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.Local)
	created := now.Add(-10 * 24 * time.Hour).Format(timestampFormat)
	lastChanged := now.Add(-3 * 24 * time.Hour).Format(timestampFormat)

	if !isRecurDue("every 2d", created, lastChanged, "completed", now) {
		t.Error("expected every 2d task to be due")
	}
}

func TestIsRecurDueOnlyCompletedOrNotStarted(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.Local)
	created := now.Add(-48 * time.Hour).Format(timestampFormat)
	lastChanged := now.Add(-25 * time.Hour).Format(timestampFormat)

	// Active tasks should not trigger recurrence
	if isRecurDue("daily", created, lastChanged, "active", now) {
		t.Error("active task should not be due for recurrence")
	}
	if isRecurDue("daily", created, lastChanged, "archived", now) {
		t.Error("archived task should not be due for recurrence")
	}
}

func TestIsRecurDueIntervalMonths(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.Local)
	created := time.Date(2026, 1, 15, 12, 0, 0, 0, time.Local).Format(timestampFormat)
	lastChanged := time.Date(2026, 4, 16, 12, 0, 0, 0, time.Local).Format(timestampFormat)

	// every 2mon: due dates at Jan 15, Mar 15, May 15. Last changed Apr 16 (after Mar 15).
	// Now is Jun 15, which is past May 15 due date, so should be due.
	if !isRecurDue("every 2mon", created, lastChanged, "completed", now) {
		t.Error("expected every 2mon task to be due")
	}
}

func TestNowLocalIncludesTimezone(t *testing.T) {
	ts := nowLocal()
	// Should contain timezone offset like +05:30 or -07:00 or Z
	if !strings.Contains(ts, "+") && !strings.Contains(ts, "-") {
		t.Fatalf("expected timezone offset in timestamp, got: %s", ts)
	}
	// Should be parseable
	_, err := time.Parse(timestampFormat, ts)
	if err != nil {
		t.Fatalf("nowLocal() not parseable: %v", err)
	}
}

func TestFormatTimestampLocalTime(t *testing.T) {
	// A UTC timestamp should be displayed in local time
	utcTs := "2026-03-30 00:30:00"
	result := formatTimestamp(utcTs)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	// A timezone-aware timestamp should also work
	localTs := "2026-03-30 10:30:00+05:30"
	result2 := formatTimestamp(localTs)
	if result2 == "" {
		t.Fatal("expected non-empty result for timezone-aware timestamp")
	}
}

func TestFormatDeadlineLocalTime(t *testing.T) {
	// Date-only deadline (23:59)
	dl := "2026-04-15 23:59:00+05:30"
	result := formatDeadline(dl)
	if !strings.Contains(result, "Apr") || !strings.Contains(result, "15") {
		t.Fatalf("expected 'Apr 15' in result, got: %s", result)
	}
	// Datetime deadline
	dl2 := "2026-04-15 14:00:00+05:30"
	result2 := formatDeadline(dl2)
	if !strings.Contains(result2, "Apr") {
		t.Fatalf("expected 'Apr' in result, got: %s", result2)
	}
}

func TestParseDeadlineIncludesTimezone(t *testing.T) {
	dl, err := parseDeadline("2026-04-15")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dl, "+") && !strings.Contains(dl, "-") {
		t.Fatalf("expected timezone in parsed deadline, got: %s", dl)
	}

	dl2, err := parseDeadline("2026-04-15 14:00")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dl2, "+") && !strings.Contains(dl2, "-") {
		t.Fatalf("expected timezone in parsed deadline, got: %s", dl2)
	}
}

func TestSQLWhitelistRejectsInvalidTable(t *testing.T) {
	store := testStore(t)

	err := store.WriteTx(func(tx *sql.Tx) error {
		return insertTags(tx, "evil_table", "task_id", 1, []string{"TAG"})
	})
	if err == nil || !strings.Contains(err.Error(), "invalid table/column") {
		t.Fatalf("expected whitelist rejection, got: %v", err)
	}

	err = store.WriteTx(func(tx *sql.Tx) error {
		return insertTags(tx, "task_tags", "evil_col", 1, []string{"TAG"})
	})
	if err == nil || !strings.Contains(err.Error(), "invalid table/column") {
		t.Fatalf("expected whitelist rejection, got: %v", err)
	}

	var result string
	store.ReadTx(func(tx *sql.Tx) error {
		result = getEntityTags(tx, "evil_table", "task_id", 1)
		return nil
	})
	if result != "" {
		t.Fatalf("expected empty string for invalid table, got: %q", result)
	}
}
