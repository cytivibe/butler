package main

// ---------------------------------------------------------------------------
// Bool flags (no value)
// ---------------------------------------------------------------------------

var ForceFlag = Flag{
	CLI:   "--force",
	Short: []string{"-force"},
	MCP:   "force",
	Bool:  true,
	Desc:  "Skip confirmation",
}

var AllFlag = Flag{
	CLI:   "--all",
	Short: []string{"-all"},
	MCP:   "all",
	Bool:  true,
	Desc:  "Show all",
}

var DetailsFlag = Flag{
	CLI:   "--details",
	Short: []string{"-details"},
	MCP:   "details",
	Bool:  true,
	Desc:  "Include description and verification fields",
}

var ParallelFlag = Flag{
	CLI:   "--parallel",
	Short: []string{"-parallel"},
	MCP:   "parallel",
	Bool:  true,
	Desc:  "Add as parallel (lettered) subtask",
}

// ---------------------------------------------------------------------------
// Single value flags
// ---------------------------------------------------------------------------

var UnderFlag = Flag{
	CLI:   "--under",
	Short: []string{"-under"},
	MCP:   "under",
	Desc:  "Parent task reference for nesting",
}

var StatusFlag = Flag{
	CLI:   "--status",
	Short: []string{"-status"},
	MCP:   "status",
	Desc:  "Task status",
	Hint:  "valid: active, completed, deferred, reopened, cancelled, archived, wait",
}

var DescFlag = Flag{
	CLI:   "--desc",
	Short: []string{"-desc"},
	MCP:   "desc",
	Desc:  "Description text",
}

var VerifyFlag = Flag{
	CLI:   "--verify",
	Short: []string{"-verify"},
	MCP:   "verify",
	Desc:  "Verification criteria",
}

var VerifyStatusFlag = Flag{
	CLI:   "--verify-status",
	Short: []string{"-verify-status"},
	MCP:   "verify_status",
	Desc:  "Verification status",
}

var SortFlag = Flag{
	CLI:   "--sort",
	Short: []string{"-sort"},
	MCP:   "sort",
	Desc:  "Sort order",
	Hint:  "valid: recent, deadline",
}

var DepthFlag = Flag{
	CLI:     "--depth",
	Short:   []string{"-depth"},
	MCP:     "depth",
	MCPType: "integer",
	Desc:    "Max child depth to display",
}

var TagFilterFlag = Flag{
	CLI:   "--tag",
	Short: []string{"-tag"},
	MCP:   "tag",
	Desc:  "Filter by tag",
}

var TagAllFlag = Flag{
	CLI:   "--tag-all",
	Short: []string{"-tag-all"},
	MCP:   "tag_all",
	Bool:  true,
	Desc:  "Group all rules by tag",
}

// ---------------------------------------------------------------------------
// Multi value flags (1+ values)
// ---------------------------------------------------------------------------

// Three --tag flag variants exist because the same CLI flag maps to different MCP
// params and behaviors depending on the command:
//   TagsFlag       - Multi, MCP="tags"  - for setting tags (addtask, settask, addrule, setrule)
//   TagsFilterFlag - Multi, MCP="tag"   - for filtering by tags (gettask)
//   TagFilterFlag  - Single, MCP="tag"  - for filtering by one tag (getrule)
// Each command uses only one variant, so no collision occurs.
var TagsFlag = Flag{
	CLI:   "--tag",
	Short: []string{"-tag"},
	MCP:   "tags",
	Multi: true,
	Desc:  "Tags (space-separated)",
}

var TagsFilterFlag = Flag{
	CLI:   "--tag",
	Short: []string{"-tag"},
	MCP:   "tag",
	Multi: true,
	Desc:  "Filter by tags",
}

var NotTagFlag = Flag{
	CLI:   "--nottag",
	Short: []string{"-nottag"},
	MCP:   "nottag",
	Multi: true,
	Desc:  "Exclude tasks with these tags",
}

// DeadlineFlag and RecurFlag use Multi for CLI (to consume "2026-04-15 14:00" as
// two tokens) but MCPType="string" so the MCP schema advertises a single string.
// ParseMCP auto-coerces the MCP string into a single-element array, and the Run
// handler joins the values with space.
var DeadlineFlag = Flag{
	CLI:     "--deadline",
	Short:   []string{"-deadline"},
	MCP:     "deadline",
	Multi:   true,
	MCPType: "string",
	Desc:    "Deadline date (YYYY-MM-DD) or datetime (YYYY-MM-DD HH:MM). Use 'none' to clear.",
	Hint:    "e.g. 2026-04-15 or 2026-04-15 14:00",
}

var RecurFlag = Flag{
	CLI:     "--recur",
	Short:   []string{"-recur"},
	MCP:     "recur",
	Multi:   true,
	MCPType: "string",
	Desc:    "Recurrence pattern: 'daily', 'weekly mon,thu', 'monthly 1,15', 'every 2d'. Use 'none' to clear.",
	Hint:    "e.g. daily, weekly mon, monthly 1, every 2d",
}

var BlockersFlag = Flag{
	CLI:   "--blockers",
	Short: []string{"-blockers"},
	MCP:   "blockers",
	Multi: true,
	Desc:  "Blocker task references (used with --status wait)",
}

// ---------------------------------------------------------------------------
// Positional flags (Pos > 0)
// ---------------------------------------------------------------------------

var TaskNamePos = Flag{
	MCP:      "name",
	Pos:      1,
	Required: true,
	Desc:     "Task name",
}

var TaskRefPos = Flag{
	MCP:      "task",
	Pos:      1,
	Required: true,
	Desc:     "Task reference",
	Hint:     "e.g. 'My Task' or 'My Task:1.a'",
}

var TaskRefOptPos = Flag{
	MCP:  "task",
	Pos:  1,
	Desc: "Task reference",
	Hint: "e.g. 'My Task' or 'My Task:1.a'",
}

var NewNamePos = Flag{
	MCP:  "name",
	Pos:  2,
	Desc: "New name",
}

var RuleNamePos = Flag{
	MCP:      "name",
	Pos:      1,
	Required: true,
	Desc:     "Rule name or text",
}

var RuleSeqPos = Flag{
	MCP:      "seq",
	Pos:      1,
	Required: true,
	MCPType:  "integer",
	Desc:     "Rule sequence number",
}

var RuleSeqOptPos = Flag{
	MCP:     "seq",
	Pos:     1,
	MCPType: "integer",
	Desc:    "Rule sequence number",
}

var TagNamePos = Flag{
	MCP:      "name",
	Pos:      1,
	Required: true,
	Desc:     "Tag name",
	Hint:     "uppercase alphanumeric, max 10 chars",
}

var TagNameOptPos = Flag{
	MCP:  "tag",
	Pos:  1,
	Desc: "Tag name",
}

var TagNameReqPos = Flag{
	MCP:      "tag",
	Pos:      1,
	Required: true,
	Desc:     "Tag name",
}

var OldTagPos = Flag{
	MCP:      "old",
	Pos:      1,
	Required: true,
	Desc:     "Current tag name",
}

var NewTagPos = Flag{
	MCP:      "new",
	Pos:      2,
	Required: true,
	Desc:     "New tag name",
	Hint:     "uppercase alphanumeric, max 10 chars",
}
