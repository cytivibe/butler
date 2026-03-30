package main

import "testing"

// FuzzParseTaskRef ensures parseTaskRef never panics on arbitrary input
// and always returns a non-empty name or the original input.
func FuzzParseTaskRef(f *testing.F) {
	seeds := []string{
		"Email boss",
		"Email boss:1",
		"Email boss:1.a",
		"Email boss:1.a.2",
		"Step 1: Design",
		"Step 1: Design:1",
		"task",
		"task:",
		":1",
		":",
		"::",
		"",
		"a:b:c:d",
		"task:1.2.3.4.5",
		"task:aa.bb.cc",
		"task:999999",
		"task:0",
		"task:-1",
		"task:1.0.a",
		"名前:1.a",
		"task\x00name:1",
		"task\nnewline:1",
		"task with spaces:1.a",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		name, path := parseTaskRef(input)
		// Must not panic (implicit)
		// Name should not be empty if input is not empty
		if input != "" && name == "" && path == "" {
			t.Errorf("parseTaskRef(%q) returned empty name and empty path", input)
		}
		// If path is non-empty, isPositionPath must agree
		if path != "" && !isPositionPath(path) {
			t.Errorf("parseTaskRef(%q) returned path %q but isPositionPath is false", input, path)
		}
	})
}

// FuzzParseRecur ensures parseRecur never panics on arbitrary input.
func FuzzParseRecur(f *testing.F) {
	seeds := []string{
		"daily",
		"daily 09:00",
		"weekly mon",
		"weekly mon,thu",
		"weekly mon,thu 09:00",
		"monthly 1",
		"monthly 1,15",
		"monthly 1,15 09:00",
		"every 2d",
		"every 4h",
		"every 30min",
		"every 2w",
		"every 2mon",
		"every 0d",
		"every -1d",
		"every 999999999d",
		"every d",
		"every",
		"",
		"   ",
		"unknown",
		"daily extra args here",
		"weekly",
		"weekly ,,,",
		"monthly 0",
		"monthly 32",
		"monthly -1",
		"every 2",
		"every 2x",
		"hourly",
		"weekly mon,invalid",
		"\x00\x01\x02",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic — either returns a valid pattern or an error
		_, err := parseRecur(input)
		_ = err
	})
}

// FuzzParseDeadline ensures parseDeadline never panics on arbitrary input.
func FuzzParseDeadline(f *testing.F) {
	seeds := []string{
		"2026-04-15",
		"2026-04-15 14:00",
		"2026-01-01",
		"2026-12-31 23:59",
		"2026-02-29",
		"2026-13-01",
		"2026-00-01",
		"2026-04-32",
		"2026-04-15 25:00",
		"2026-04-15 14:60",
		"not-a-date",
		"",
		"   ",
		"2026",
		"2026-04",
		"04-15",
		"2026-04-15T14:00:00Z",
		"2026-04-15 14:00:00",
		"9999-12-31 23:59",
		"0000-01-01",
		"2026-04-15 14",
		"-2026-04-15",
		"\x00\x01\x02",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic — either returns a valid deadline or an error
		result, err := parseDeadline(input)
		if err == nil && result == "" {
			t.Errorf("parseDeadline(%q) returned empty result with no error", input)
		}
	})
}
