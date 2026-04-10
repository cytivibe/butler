package main

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test commands reused across tests
// ---------------------------------------------------------------------------

// fullCmd: 1 required positional, 1 bool flag, 1 value flag, 1 multi flag
var fullCmd = Command{
	Name: "fullcmd",
	Desc: "test command with all flag types",
	Flags: []Flag{
		{MCP: "name", Pos: 1, Required: true, Desc: "task name"},
		{CLI: "--force", Short: []string{"-force", "-f"}, MCP: "force", Bool: true, Desc: "force"},
		{CLI: "--status", Short: []string{"-status"}, MCP: "status", Desc: "status"},
		{CLI: "--tag", Short: []string{"-tag"}, MCP: "tags", Multi: true, Desc: "tags"},
	},
}

// twoPosCmd: 2 positionals (required + optional), 1 bool flag
var twoPosCmd = Command{
	Name: "twopos",
	Desc: "test command with two positional args",
	Flags: []Flag{
		{MCP: "task", Pos: 1, Required: true, Desc: "task ref"},
		{MCP: "newname", Pos: 2, Desc: "new name"},
		{CLI: "--force", Short: []string{"-force"}, MCP: "force", Bool: true, Desc: "force"},
	},
}

// noFlagsCmd: only a required positional, no named flags
var noFlagsCmd = Command{
	Name: "noflagscmd",
	Desc: "command with no named flags",
	Flags: []Flag{
		{MCP: "name", Pos: 1, Required: true, Desc: "tag name"},
	},
}

// onlyFlagsCmd: no positionals, only named flags
var onlyFlagsCmd = Command{
	Name: "onlyflagscmd",
	Desc: "command with no positional args",
	Flags: []Flag{
		{CLI: "--all", Short: []string{"-all"}, MCP: "all", Bool: true, Desc: "show all"},
		{CLI: "--status", Short: []string{"-status"}, MCP: "status", Desc: "status filter"},
	},
}

// emptyCmd: no flags, no positionals at all
var emptyCmd = Command{
	Name: "emptycmd",
	Desc: "command with nothing",
	Flags: []Flag{},
}

// ---------------------------------------------------------------------------
// Positional args
// ---------------------------------------------------------------------------

func TestParseCLI_SingleRequiredPositional(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "My Task" {
		t.Errorf("Arg(name) = %q, want %q", got, "My Task")
	}
}

func TestParseCLI_TwoPositionals_BothProvided(t *testing.T) {
	p, err := twoPosCmd.ParseCLI([]string{"Old Name", "New Name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("task"); got != "Old Name" {
		t.Errorf("Arg(task) = %q, want %q", got, "Old Name")
	}
	if got := p.Arg("newname"); got != "New Name" {
		t.Errorf("Arg(newname) = %q, want %q", got, "New Name")
	}
}

func TestParseCLI_TwoPositionals_OnlyFirstProvided(t *testing.T) {
	p, err := twoPosCmd.ParseCLI([]string{"My Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("task"); got != "My Task" {
		t.Errorf("Arg(task) = %q, want %q", got, "My Task")
	}
	if got := p.Arg("newname"); got != "" {
		t.Errorf("Arg(newname) = %q, want empty", got)
	}
}

func TestParseCLI_PositionalAfterFlags(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"--force", "My Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "My Task" {
		t.Errorf("Arg(name) = %q, want %q", got, "My Task")
	}
	if !p.Bool("--force") {
		t.Error("expected --force to be true")
	}
}

func TestParseCLI_PositionalBetweenFlags(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"--force", "My Task", "--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "My Task" {
		t.Errorf("Arg(name) = %q, want %q", got, "My Task")
	}
	if !p.Bool("--force") {
		t.Error("expected --force to be true")
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
}

func TestParseCLI_ExtraPositionalRejected(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"Name1", "Name2"})
	if err == nil {
		t.Fatal("expected error for extra positional, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Errorf("error = %q, want it to contain 'unexpected argument'", err)
	}
}

func TestParseCLI_RequiredPositionalMissing(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"--force"})
	if err == nil {
		t.Fatal("expected error for missing required positional, got nil")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error = %q, want it to mention 'name'", err)
	}
}

func TestParseCLI_RequiredPositionalMissing_NoArgs(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{})
	if err == nil {
		t.Fatal("expected error for empty args with required positional, got nil")
	}
}

// ---------------------------------------------------------------------------
// Bool flags
// ---------------------------------------------------------------------------

func TestParseCLI_BoolFlagPresent(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "--force"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Bool("--force") {
		t.Error("expected --force to be true")
	}
}

func TestParseCLI_BoolFlagAbsent(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Bool("--force") {
		t.Error("expected --force to be false")
	}
}

func TestParseCLI_MultipleBoolFlags(t *testing.T) {
	cmd := Command{
		Name: "multibool",
		Flags: []Flag{
			{CLI: "--force", MCP: "force", Bool: true},
			{CLI: "--all", MCP: "all", Bool: true},
			{CLI: "--details", MCP: "details", Bool: true},
		},
	}
	p, err := cmd.ParseCLI([]string{"--force", "--details"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Bool("--force") {
		t.Error("expected --force true")
	}
	if p.Bool("--all") {
		t.Error("expected --all false")
	}
	if !p.Bool("--details") {
		t.Error("expected --details true")
	}
}

// ---------------------------------------------------------------------------
// Single value flags
// ---------------------------------------------------------------------------

func TestParseCLI_ValueFlag(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
}

func TestParseCLI_ValueFlagMissingValue(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--status"})
	if err == nil {
		t.Fatal("expected error for missing value, got nil")
	}
	if !strings.Contains(err.Error(), "--status") && !strings.Contains(err.Error(), "requires a value") {
		t.Errorf("error = %q, want it to mention --status and value requirement", err)
	}
}

func TestParseCLI_ValueFlagAbsent(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.String("--status"); got != "" {
		t.Errorf("String(--status) = %q, want empty", got)
	}
	if p.Has("--status") {
		t.Error("expected Has(--status) false")
	}
}

// ---------------------------------------------------------------------------
// Multi value flags
// ---------------------------------------------------------------------------

func TestParseCLI_MultiFlag_MultipleValues(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "--tag", "WORK", "URGENT", "BACKEND"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Strings("--tag")
	want := []string{"WORK", "URGENT", "BACKEND"}
	if len(got) != len(want) {
		t.Fatalf("Strings(--tag) len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Strings(--tag)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseCLI_MultiFlag_SingleValue(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "--tag", "WORK"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Strings("--tag")
	if len(got) != 1 || got[0] != "WORK" {
		t.Errorf("Strings(--tag) = %v, want [WORK]", got)
	}
}

func TestParseCLI_MultiFlag_StopsAtNextFlag(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "--tag", "WORK", "URGENT", "--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Strings("--tag")
	if len(got) != 2 || got[0] != "WORK" || got[1] != "URGENT" {
		t.Errorf("Strings(--tag) = %v, want [WORK URGENT]", got)
	}
	if s := p.String("--status"); s != "active" {
		t.Errorf("String(--status) = %q, want %q", s, "active")
	}
}

func TestParseCLI_MultiFlag_StopsAtShortFlag(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "--tag", "WORK", "-force"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Strings("--tag")
	if len(got) != 1 || got[0] != "WORK" {
		t.Errorf("Strings(--tag) = %v, want [WORK]", got)
	}
	if !p.Bool("--force") {
		t.Error("expected --force true")
	}
}

func TestParseCLI_MultiFlag_ZeroValues_NextIsFlag(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--tag", "--status", "active"})
	if err == nil {
		t.Fatal("expected error for multi flag with zero values, got nil")
	}
	if !strings.Contains(err.Error(), "--tag") {
		t.Errorf("error = %q, want it to mention --tag", err)
	}
}

func TestParseCLI_MultiFlag_ZeroValues_EndOfArgs(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--tag"})
	if err == nil {
		t.Fatal("expected error for multi flag with no values at end, got nil")
	}
	if !strings.Contains(err.Error(), "--tag") {
		t.Errorf("error = %q, want it to mention --tag", err)
	}
}

func TestParseCLI_MultiFlag_Absent(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Strings("--tag"); got != nil {
		t.Errorf("Strings(--tag) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// Short aliases
// ---------------------------------------------------------------------------

func TestParseCLI_ShortAlias_DoubleDash(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "-force"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Bool("--force") {
		t.Error("expected --force true via -force alias")
	}
}

func TestParseCLI_ShortAlias_SingleChar(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "-f"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Bool("--force") {
		t.Error("expected --force true via -f alias")
	}
}

func TestParseCLI_ShortAlias_ValueFlag(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "-status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q via -status alias", got, "active")
	}
}

func TestParseCLI_ShortAlias_MultiFlag(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "-tag", "WORK", "URGENT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Strings("--tag")
	if len(got) != 2 || got[0] != "WORK" || got[1] != "URGENT" {
		t.Errorf("Strings(--tag) = %v, want [WORK URGENT]", got)
	}
}

// ---------------------------------------------------------------------------
// Unknown flag rejection
// ---------------------------------------------------------------------------

func TestParseCLI_UnknownDoubleDashFlag(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--unknown"})
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if !strings.Contains(err.Error(), "--unknown") {
		t.Errorf("error = %q, want it to mention --unknown", err)
	}
}

func TestParseCLI_UnknownSingleDashFlag(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "-unknown"})
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if !strings.Contains(err.Error(), "-unknown") {
		t.Errorf("error = %q, want it to mention -unknown", err)
	}
}

func TestParseCLI_UnknownFlag_NoPositionalOverwrite(t *testing.T) {
	// Ensure unknown flags don't silently become positional args
	_, err := fullCmd.ParseCLI([]string{"My Task", "--tagg", "WORK"})
	if err == nil {
		t.Fatal("expected error for --tagg (unknown flag), got nil")
	}
}

// ---------------------------------------------------------------------------
// Case-insensitive suggestions
// ---------------------------------------------------------------------------

func TestParseCLI_CaseSuggestion_UpperCase(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--FORCE"})
	if err == nil {
		t.Fatal("expected error for wrong case flag, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %q, want suggestion for --force", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "did you mean") {
		t.Errorf("error = %q, want 'did you mean' suggestion", err)
	}
}

func TestParseCLI_CaseSuggestion_MixedCase(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--Status"})
	if err == nil {
		t.Fatal("expected error for wrong case flag, got nil")
	}
	if !strings.Contains(err.Error(), "--status") {
		t.Errorf("error = %q, want suggestion for --status", err)
	}
}

func TestParseCLI_CaseSuggestion_ShortAlias(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "-Force"})
	if err == nil {
		t.Fatal("expected error for wrong case short alias, got nil")
	}
	if !strings.Contains(err.Error(), "-force") {
		t.Errorf("error = %q, want suggestion for -force", err)
	}
}

func TestParseCLI_CaseSuggestion_NoMatch(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--zzzzz"})
	if err == nil {
		t.Fatal("expected error for completely unknown flag, got nil")
	}
	// Should NOT contain "did you mean" since there's no close match
	if strings.Contains(strings.ToLower(err.Error()), "did you mean") {
		t.Errorf("error = %q, should not suggest for completely unknown flag", err)
	}
}

// ---------------------------------------------------------------------------
// Required field validation
// ---------------------------------------------------------------------------

func TestParseCLI_RequiredFlagMissing(t *testing.T) {
	cmd := Command{
		Name: "reqflag",
		Flags: []Flag{
			{CLI: "--status", MCP: "status", Required: true, Desc: "status"},
		},
	}
	_, err := cmd.ParseCLI([]string{})
	if err == nil {
		t.Fatal("expected error for missing required flag, got nil")
	}
	if !strings.Contains(err.Error(), "--status") {
		t.Errorf("error = %q, want it to mention --status", err)
	}
}

func TestParseCLI_RequiredFlagProvided(t *testing.T) {
	cmd := Command{
		Name: "reqflag",
		Flags: []Flag{
			{CLI: "--status", MCP: "status", Required: true, Desc: "status"},
		},
	}
	p, err := cmd.ParseCLI([]string{"--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
}

func TestParseCLI_RequiredPositionalProvided_OptionalAbsent(t *testing.T) {
	p, err := twoPosCmd.ParseCLI([]string{"Task Ref"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("task"); got != "Task Ref" {
		t.Errorf("Arg(task) = %q, want %q", got, "Task Ref")
	}
	if got := p.Arg("newname"); got != "" {
		t.Errorf("Arg(newname) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Value-looks-like-flag protection
// ---------------------------------------------------------------------------

func TestParseCLI_ValueLooksLikeFlag_DoubleDash(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--status", "--tag"})
	if err == nil {
		t.Fatal("expected error when value looks like a flag, got nil")
	}
	if !strings.Contains(err.Error(), "looks like a flag") || !strings.Contains(err.Error(), "--status") {
		t.Errorf("error = %q, want mention of --status and 'looks like a flag'", err)
	}
}

func TestParseCLI_ValueLooksLikeFlag_SingleDash(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--status", "-tag"})
	if err == nil {
		t.Fatal("expected error when value looks like a flag, got nil")
	}
	if !strings.Contains(err.Error(), "looks like a flag") {
		t.Errorf("error = %q, want 'looks like a flag'", err)
	}
}

func TestParseCLI_ValueLooksLikeFlag_UnknownFlag(t *testing.T) {
	// --status --xyz where --xyz isn't even a known flag - still an error
	_, err := fullCmd.ParseCLI([]string{"My Task", "--status", "--xyz"})
	if err == nil {
		t.Fatal("expected error when value looks like a flag, got nil")
	}
}

// ---------------------------------------------------------------------------
// Validate function
// ---------------------------------------------------------------------------

func TestParseCLI_Validate_Passes(t *testing.T) {
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
			{CLI: "--status", MCP: "status", Desc: "status", Validate: func(v []string) error {
				valid := map[string]bool{"active": true, "completed": true, "deferred": true}
				if !valid[v[0]] {
					return fmt.Errorf("invalid status %q", v[0])
				}
				return nil
			}},
		},
	}
	p, err := cmd.ParseCLI([]string{"Task", "--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
}

func TestParseCLI_Validate_Fails(t *testing.T) {
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
			{CLI: "--status", MCP: "status", Desc: "status", Validate: func(v []string) error {
				valid := map[string]bool{"active": true, "completed": true}
				if !valid[v[0]] {
					return fmt.Errorf("invalid status %q", v[0])
				}
				return nil
			}},
		},
	}
	_, err := cmd.ParseCLI([]string{"Task", "--status", "badvalue"})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "badvalue") {
		t.Errorf("error = %q, want it to mention 'badvalue'", err)
	}
}

func TestParseCLI_Validate_MultiFlag_ReceivesAllValues(t *testing.T) {
	var received []string
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
			{CLI: "--tag", MCP: "tags", Multi: true, Desc: "tags", Validate: func(v []string) error {
				received = v
				return nil
			}},
		},
	}
	_, err := cmd.ParseCLI([]string{"Task", "--tag", "A", "B", "C"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) != 3 || received[0] != "A" || received[1] != "B" || received[2] != "C" {
		t.Errorf("Validate received %v, want [A B C]", received)
	}
}

func TestParseCLI_Validate_SingleFlag_ReceivesSliceOfOne(t *testing.T) {
	var received []string
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
			{CLI: "--status", MCP: "status", Desc: "status", Validate: func(v []string) error {
				received = v
				return nil
			}},
		},
	}
	_, err := cmd.ParseCLI([]string{"Task", "--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) != 1 || received[0] != "active" {
		t.Errorf("Validate received %v, want [active]", received)
	}
}

func TestParseCLI_Validate_NotCalledWhenFlagAbsent(t *testing.T) {
	called := false
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
			{CLI: "--status", MCP: "status", Desc: "status", Validate: func(v []string) error {
				called = true
				return nil
			}},
		},
	}
	_, err := cmd.ParseCLI([]string{"Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("Validate should not be called when flag is absent")
	}
}

func TestParseCLI_Validate_PositionalArg(t *testing.T) {
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name", Validate: func(v []string) error {
				if len(v[0]) > 10 {
					return fmt.Errorf("name too long")
				}
				return nil
			}},
		},
	}
	_, err := cmd.ParseCLI([]string{"this is a really long name"})
	if err == nil {
		t.Fatal("expected validation error for long name, got nil")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Errorf("error = %q, want 'too long'", err)
	}
}

// ---------------------------------------------------------------------------
// Duplicate flag
// ---------------------------------------------------------------------------

func TestParseCLI_DuplicateFlag(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--status", "active", "--status", "completed"})
	if err == nil {
		t.Fatal("expected error for duplicate flag, got nil")
	}
	if !strings.Contains(err.Error(), "--status") {
		t.Errorf("error = %q, want it to mention --status", err)
	}
}

func TestParseCLI_DuplicateBoolFlag(t *testing.T) {
	_, err := fullCmd.ParseCLI([]string{"My Task", "--force", "--force"})
	if err == nil {
		t.Fatal("expected error for duplicate bool flag, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %q, want it to mention --force", err)
	}
}

func TestParseCLI_DuplicateViaShortAlias(t *testing.T) {
	// --force and -f both resolve to same flag
	_, err := fullCmd.ParseCLI([]string{"My Task", "--force", "-f"})
	if err == nil {
		t.Fatal("expected error for duplicate flag via alias, got nil")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestParseCLI_EmptyArgs_NoRequiredFields(t *testing.T) {
	p, err := emptyCmd.ParseCLI([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = p // just verify it returns successfully
}

func TestParseCLI_EmptyArgs_WithOptionalFlags(t *testing.T) {
	p, err := onlyFlagsCmd.ParseCLI([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Bool("--all") {
		t.Error("expected --all false")
	}
}

func TestParseCLI_NoFlagsCommand_UnknownFlagRejected(t *testing.T) {
	_, err := noFlagsCmd.ParseCLI([]string{"MyTag", "--unknown"})
	if err == nil {
		t.Fatal("expected error for unknown flag on no-flags command, got nil")
	}
}

func TestParseCLI_NoFlagsCommand_BareArgNotRejected(t *testing.T) {
	p, err := noFlagsCmd.ParseCLI([]string{"MyTag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "MyTag" {
		t.Errorf("Arg(name) = %q, want %q", got, "MyTag")
	}
}

func TestParseCLI_BareDashDash(t *testing.T) {
	// Just "--" by itself should be treated as unknown flag
	_, err := fullCmd.ParseCLI([]string{"My Task", "--"})
	if err == nil {
		t.Fatal("expected error for bare --, got nil")
	}
}

func TestParseCLI_PositionalWithDashInMiddle(t *testing.T) {
	// "my-task" contains dash but doesn't start with -, should be fine as positional
	p, err := fullCmd.ParseCLI([]string{"my-task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "my-task" {
		t.Errorf("Arg(name) = %q, want %q", got, "my-task")
	}
}

func TestParseCLI_AllFlagTypesTogether(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "--force", "--status", "active", "--tag", "WORK", "URGENT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "My Task" {
		t.Errorf("Arg(name) = %q, want %q", got, "My Task")
	}
	if !p.Bool("--force") {
		t.Error("expected --force true")
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
	tags := p.Strings("--tag")
	if len(tags) != 2 || tags[0] != "WORK" || tags[1] != "URGENT" {
		t.Errorf("Strings(--tag) = %v, want [WORK URGENT]", tags)
	}
}

func TestParseCLI_FlagsInAnyOrder(t *testing.T) {
	// A named flag (--force) between multi-value and positional stops the multi consumption
	p, err := fullCmd.ParseCLI([]string{"--tag", "WORK", "--force", "My Task", "--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "My Task" {
		t.Errorf("Arg(name) = %q, want %q", got, "My Task")
	}
	if !p.Bool("--force") {
		t.Error("expected --force true")
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
	tags := p.Strings("--tag")
	if len(tags) != 1 || tags[0] != "WORK" {
		t.Errorf("Strings(--tag) = %v, want [WORK]", tags)
	}
}

func TestParseCLI_MultiFlag_ConsumesBarePosAfterIt(t *testing.T) {
	// Without a flag in between, multi-value consumes the positional as a value
	_, err := fullCmd.ParseCLI([]string{"--tag", "WORK", "My Task"})
	if err == nil {
		t.Fatal("expected error: multi flag consumes bare arg, leaving no required positional")
	}
}

func TestParseCLI_TwoPositionals_WithFlags(t *testing.T) {
	p, err := twoPosCmd.ParseCLI([]string{"Old Name", "--force", "New Name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("task"); got != "Old Name" {
		t.Errorf("Arg(task) = %q, want %q", got, "Old Name")
	}
	if got := p.Arg("newname"); got != "New Name" {
		t.Errorf("Arg(newname) = %q, want %q", got, "New Name")
	}
	if !p.Bool("--force") {
		t.Error("expected --force true")
	}
}

func TestParseCLI_OnlyFlags_NoPositionalExpected(t *testing.T) {
	p, err := onlyFlagsCmd.ParseCLI([]string{"--all", "--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Bool("--all") {
		t.Error("expected --all true")
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
}

func TestParseCLI_OnlyFlags_UnexpectedPositional(t *testing.T) {
	_, err := onlyFlagsCmd.ParseCLI([]string{"stray-arg"})
	if err == nil {
		t.Fatal("expected error for unexpected positional on flags-only command, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Errorf("error = %q, want 'unexpected argument'", err)
	}
}

// ---------------------------------------------------------------------------
// Parsed accessors on empty/missing data
// ---------------------------------------------------------------------------

func TestParsed_Accessors_EmptyParsed(t *testing.T) {
	p := Parsed{
		args:  map[string]flagResult{},
		flags: map[string]flagResult{},
	}
	if got := p.Arg("missing"); got != "" {
		t.Errorf("Arg(missing) = %q, want empty", got)
	}
	if got := p.Bool("--missing"); got != false {
		t.Errorf("Bool(--missing) = %v, want false", got)
	}
	if got := p.String("--missing"); got != "" {
		t.Errorf("String(--missing) = %q, want empty", got)
	}
	if got := p.Strings("--missing"); got != nil {
		t.Errorf("Strings(--missing) = %v, want nil", got)
	}
	if got := p.Has("--missing"); got != false {
		t.Errorf("Has(--missing) = %v, want false", got)
	}
}

func TestParsed_Has_DistinguishesPresentFromAbsent(t *testing.T) {
	p, err := fullCmd.ParseCLI([]string{"My Task", "--status", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Has("--status") {
		t.Error("expected Has(--status) true when flag provided")
	}
	if p.Has("--force") {
		t.Error("expected Has(--force) false when flag not provided")
	}
}

// ===========================================================================
// ParseMCP tests
// ===========================================================================

// ---------------------------------------------------------------------------
// Basic extraction
// ---------------------------------------------------------------------------

func TestParseMCP_PositionalAsNamedParam(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{"name": "My Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "My Task" {
		t.Errorf("Arg(name) = %q, want %q", got, "My Task")
	}
}

func TestParseMCP_BoolParam(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "force": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Bool("--force") {
		t.Error("expected Bool(--force) true")
	}
}

func TestParseMCP_BoolParamFalse(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "force": false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Bool("--force") {
		t.Error("expected Bool(--force) false when explicitly set to false")
	}
}

func TestParseMCP_StringParam(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "status": "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
}

func TestParseMCP_ArrayParam(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{
		"name": "Task",
		"tags": []interface{}{"WORK", "URGENT"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Strings("--tag")
	if len(got) != 2 || got[0] != "WORK" || got[1] != "URGENT" {
		t.Errorf("Strings(--tag) = %v, want [WORK URGENT]", got)
	}
}

func TestParseMCP_AllParamTypes(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{
		"name":   "Task",
		"force":  true,
		"status": "active",
		"tags":   []interface{}{"WORK"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("name"); got != "Task" {
		t.Errorf("Arg(name) = %q, want %q", got, "Task")
	}
	if !p.Bool("--force") {
		t.Error("expected --force true")
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
	tags := p.Strings("--tag")
	if len(tags) != 1 || tags[0] != "WORK" {
		t.Errorf("Strings(--tag) = %v, want [WORK]", tags)
	}
}

func TestParseMCP_TwoPositionals(t *testing.T) {
	p, err := twoPosCmd.ParseMCP(map[string]interface{}{"task": "Old", "newname": "New"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("task"); got != "Old" {
		t.Errorf("Arg(task) = %q, want %q", got, "Old")
	}
	if got := p.Arg("newname"); got != "New" {
		t.Errorf("Arg(newname) = %q, want %q", got, "New")
	}
}

// ---------------------------------------------------------------------------
// Unknown param rejection
// ---------------------------------------------------------------------------

func TestParseMCP_UnknownParam(t *testing.T) {
	_, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "priority": "high"})
	if err == nil {
		t.Fatal("expected error for unknown param, got nil")
	}
	if !strings.Contains(err.Error(), "priority") {
		t.Errorf("error = %q, want it to mention 'priority'", err)
	}
}

func TestParseMCP_UnknownParamOnly(t *testing.T) {
	_, err := fullCmd.ParseMCP(map[string]interface{}{"xyz": "abc"})
	if err == nil {
		t.Fatal("expected error for unknown param, got nil")
	}
}

// ---------------------------------------------------------------------------
// Required param validation
// ---------------------------------------------------------------------------

func TestParseMCP_RequiredParamMissing(t *testing.T) {
	_, err := fullCmd.ParseMCP(map[string]interface{}{"force": true})
	if err == nil {
		t.Fatal("expected error for missing required param 'name', got nil")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error = %q, want it to mention 'name'", err)
	}
}

func TestParseMCP_RequiredFlagMissing(t *testing.T) {
	cmd := Command{
		Name: "reqcmd",
		Flags: []Flag{
			{CLI: "--status", MCP: "status", Required: true, Desc: "status"},
		},
	}
	_, err := cmd.ParseMCP(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing required flag, got nil")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Errorf("error = %q, want it to mention 'status'", err)
	}
}

func TestParseMCP_EmptyParams_NoRequired(t *testing.T) {
	p, err := emptyCmd.ParseMCP(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = p
}

func TestParseMCP_EmptyParams_OptionalFlags(t *testing.T) {
	p, err := onlyFlagsCmd.ParseMCP(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Bool("--all") {
		t.Error("expected --all false")
	}
}

// ---------------------------------------------------------------------------
// Type checking
// ---------------------------------------------------------------------------

func TestParseMCP_StringParam_NonString(t *testing.T) {
	_, err := fullCmd.ParseMCP(map[string]interface{}{"name": 123})
	if err == nil {
		t.Fatal("expected error for non-string param, got nil")
	}
}

func TestParseMCP_BoolParam_NonBool(t *testing.T) {
	_, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "force": "yes"})
	if err == nil {
		t.Fatal("expected error for non-bool param, got nil")
	}
}

func TestParseMCP_ArrayParam_StringCoerced(t *testing.T) {
	// Single string auto-coerced to array for backward compat
	p, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "tags": "WORK"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Strings("--tag")
	if len(got) != 1 || got[0] != "WORK" {
		t.Errorf("Strings(--tag) = %v, want [WORK]", got)
	}
}

func TestParseMCP_ArrayParam_NonArrayNonString(t *testing.T) {
	_, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "tags": 123})
	if err == nil {
		t.Fatal("expected error for non-array non-string param, got nil")
	}
}

func TestParseMCP_ArrayParam_NonStringElement(t *testing.T) {
	_, err := fullCmd.ParseMCP(map[string]interface{}{
		"name": "Task",
		"tags": []interface{}{"WORK", 123},
	})
	if err == nil {
		t.Fatal("expected error for non-string array element, got nil")
	}
}

// ---------------------------------------------------------------------------
// Validate function
// ---------------------------------------------------------------------------

func TestParseMCP_Validate_Passes(t *testing.T) {
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{CLI: "--status", MCP: "status", Desc: "status", Validate: func(v []string) error {
				if v[0] != "active" && v[0] != "completed" {
					return fmt.Errorf("invalid status %q", v[0])
				}
				return nil
			}},
		},
	}
	p, err := cmd.ParseMCP(map[string]interface{}{"status": "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.String("--status"); got != "active" {
		t.Errorf("String(--status) = %q, want %q", got, "active")
	}
}

func TestParseMCP_Validate_Fails(t *testing.T) {
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{CLI: "--status", MCP: "status", Desc: "status", Validate: func(v []string) error {
				return fmt.Errorf("invalid status %q", v[0])
			}},
		},
	}
	_, err := cmd.ParseMCP(map[string]interface{}{"status": "badvalue"})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "badvalue") {
		t.Errorf("error = %q, want 'badvalue'", err)
	}
}

func TestParseMCP_Validate_MultiReceivesAllValues(t *testing.T) {
	var received []string
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{CLI: "--tag", MCP: "tags", Multi: true, Desc: "tags", Validate: func(v []string) error {
				received = v
				return nil
			}},
		},
	}
	_, err := cmd.ParseMCP(map[string]interface{}{"tags": []interface{}{"A", "B"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) != 2 || received[0] != "A" || received[1] != "B" {
		t.Errorf("Validate received %v, want [A B]", received)
	}
}

func TestParseMCP_Validate_NotCalledWhenAbsent(t *testing.T) {
	called := false
	cmd := Command{
		Name: "valcmd",
		Flags: []Flag{
			{CLI: "--status", MCP: "status", Desc: "status", Validate: func(v []string) error {
				called = true
				return nil
			}},
		},
	}
	_, err := cmd.ParseMCP(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("Validate should not be called when param absent")
	}
}

// ---------------------------------------------------------------------------
// Has/absent accessors via MCP
// ---------------------------------------------------------------------------

func TestParseMCP_Has(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "status": "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Has("--status") {
		t.Error("expected Has(--status) true")
	}
	if p.Has("--force") {
		t.Error("expected Has(--force) false")
	}
}

func TestParseMCP_AbsentFlagsReturnDefaults(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Bool("--force") {
		t.Error("expected false for absent bool")
	}
	if p.String("--status") != "" {
		t.Error("expected empty for absent string")
	}
	if p.Strings("--tag") != nil {
		t.Error("expected nil for absent multi")
	}
}

// ===========================================================================
// MCPSchema tests
// ===========================================================================

func TestMCPSchema_ContainsAllParams(t *testing.T) {
	schema := fullCmd.MCPSchema()
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema missing 'properties'")
	}
	// Should have: name, force, status, tags
	for _, mcp := range []string{"name", "force", "status", "tags"} {
		if _, ok := props[mcp]; !ok {
			t.Errorf("schema missing property %q", mcp)
		}
	}
}

func TestMCPSchema_RequiredList(t *testing.T) {
	schema := fullCmd.MCPSchema()
	req, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("schema missing 'required'")
	}
	found := false
	for _, r := range req {
		if r == "name" {
			found = true
		}
		// force, status, tags are not required in fullCmd
		if r == "force" || r == "status" || r == "tags" {
			t.Errorf("unexpected required field %q", r)
		}
	}
	if !found {
		t.Error("'name' should be in required list")
	}
}

func TestMCPSchema_BoolType(t *testing.T) {
	schema := fullCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	force := props["force"].(map[string]interface{})
	if force["type"] != "boolean" {
		t.Errorf("force type = %q, want 'boolean'", force["type"])
	}
}

func TestMCPSchema_StringType(t *testing.T) {
	schema := fullCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	status := props["status"].(map[string]interface{})
	if status["type"] != "string" {
		t.Errorf("status type = %q, want 'string'", status["type"])
	}
}

func TestMCPSchema_ArrayType(t *testing.T) {
	schema := fullCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	tags := props["tags"].(map[string]interface{})
	if tags["type"] != "array" {
		t.Errorf("tags type = %q, want 'array'", tags["type"])
	}
	items, ok := tags["items"].(map[string]interface{})
	if !ok {
		t.Fatal("tags missing 'items'")
	}
	if items["type"] != "string" {
		t.Errorf("tags items type = %q, want 'string'", items["type"])
	}
}

func TestMCPSchema_Description(t *testing.T) {
	schema := fullCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	name := props["name"].(map[string]interface{})
	if name["description"] != "task name" {
		t.Errorf("name description = %q, want 'task name'", name["description"])
	}
}

func TestMCPSchema_PositionalType(t *testing.T) {
	schema := fullCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	name := props["name"].(map[string]interface{})
	if name["type"] != "string" {
		t.Errorf("positional name type = %q, want 'string'", name["type"])
	}
}

func TestMCPSchema_EmptyCommand(t *testing.T) {
	schema := emptyCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	if len(props) != 0 {
		t.Errorf("expected 0 properties for empty command, got %d", len(props))
	}
}

func TestMCPSchema_NoExtraFields(t *testing.T) {
	schema := fullCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	if len(props) != 4 {
		t.Errorf("expected 4 properties (name, force, status, tags), got %d", len(props))
	}
}

// ===========================================================================
// ParseMCP auto-coercion tests
// ===========================================================================

func TestParseMCP_FloatToStringCoercion(t *testing.T) {
	cmd := Command{
		Name: "coerce",
		Flags: []Flag{
			{MCP: "depth", Pos: 1, Desc: "depth"},
		},
	}
	p, err := cmd.ParseMCP(map[string]interface{}{"depth": float64(5)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("depth"); got != "5" {
		t.Errorf("Arg(depth) = %q, want %q", got, "5")
	}
}

func TestParseMCP_FloatToStringCoercion_RejectsFractional(t *testing.T) {
	cmd := Command{
		Name: "coerce",
		Flags: []Flag{
			{MCP: "depth", Pos: 1, Desc: "depth"},
		},
	}
	_, err := cmd.ParseMCP(map[string]interface{}{"depth": float64(5.7)})
	if err == nil {
		t.Fatal("expected error for fractional float, got nil")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error = %q, want it to mention 'integer'", err)
	}
}

func TestParseMCP_FloatToStringCoercion_AcceptsWholeNumber(t *testing.T) {
	cmd := Command{
		Name: "coerce",
		Flags: []Flag{
			{MCP: "depth", Pos: 1, Desc: "depth"},
		},
	}
	p, err := cmd.ParseMCP(map[string]interface{}{"depth": float64(3.0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Arg("depth"); got != "3" {
		t.Errorf("Arg(depth) = %q, want %q", got, "3")
	}
}

func TestParseMCP_StringToArrayCoercion(t *testing.T) {
	p, err := fullCmd.ParseMCP(map[string]interface{}{"name": "Task", "tags": "WORK"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Strings("--tag")
	if len(got) != 1 || got[0] != "WORK" {
		t.Errorf("Strings(--tag) = %v, want [WORK]", got)
	}
}

// ===========================================================================
// Confirm and RawOutput tests
// ===========================================================================

func TestCommand_ConfirmField(t *testing.T) {
	confirmCalled := false
	runCalled := false
	cmd := Command{
		Name: "testdelete",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
			{CLI: "--force", MCP: "force", Bool: true, Desc: "force"},
		},
		Confirm: func(store *Store, p Parsed) (string, error) {
			confirmCalled = true
			return fmt.Sprintf("Delete %q? (y/N): ", p.Arg("name")), nil
		},
		Run: func(store *Store, p Parsed) (string, error) {
			runCalled = true
			return "deleted", nil
		},
	}
	if cmd.Confirm == nil {
		t.Fatal("expected Confirm to be set")
	}
	// Simulate what RunCLI does: parse, call Confirm, call Run
	p, err := cmd.ParseCLI([]string{"MyItem"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg, err := cmd.Confirm(nil, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "MyItem") {
		t.Errorf("confirm message = %q, want it to contain 'MyItem'", msg)
	}
	if !confirmCalled {
		t.Error("expected Confirm to be called")
	}
	result, err := cmd.Run(nil, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "deleted" {
		t.Errorf("Run result = %q, want 'deleted'", result)
	}
	if !runCalled {
		t.Error("expected Run to be called")
	}
}

func TestCommand_ConfirmSkippedWithForce(t *testing.T) {
	cmd := Command{
		Name: "testdelete",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
			{CLI: "--force", MCP: "force", Bool: true, Desc: "force"},
		},
		Confirm: func(store *Store, p Parsed) (string, error) {
			t.Fatal("Confirm should not be called when --force is set")
			return "", nil
		},
		Run: func(store *Store, p Parsed) (string, error) {
			return "deleted", nil
		},
	}
	p, err := cmd.ParseCLI([]string{"MyItem", "--force"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// RunCLI skips Confirm when --force is present
	if cmd.Confirm != nil && !p.Bool("--force") {
		t.Fatal("should have skipped confirm")
	}
}

func TestCommand_ConfirmNilMeansNoConfirmation(t *testing.T) {
	cmd := Command{
		Name: "testadd",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
		},
		Run: func(store *Store, p Parsed) (string, error) {
			return "added", nil
		},
	}
	if cmd.Confirm != nil {
		t.Error("expected Confirm to be nil for non-delete command")
	}
}

func TestCommand_RawOutputTrue(t *testing.T) {
	cmd := Command{
		Name:      "testget",
		RawOutput: true,
	}
	if !cmd.RawOutput {
		t.Error("expected RawOutput true for get command")
	}
}

func TestCommand_RawOutputFalseByDefault(t *testing.T) {
	cmd := Command{
		Name: "testadd",
	}
	if cmd.RawOutput {
		t.Error("expected RawOutput false by default")
	}
}

func TestCommand_ConfirmError(t *testing.T) {
	cmd := Command{
		Name: "testdelete",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
		},
		Confirm: func(store *Store, p Parsed) (string, error) {
			return "", fmt.Errorf("item not found")
		},
		Run: func(store *Store, p Parsed) (string, error) {
			return "deleted", nil
		},
	}
	p, _ := cmd.ParseCLI([]string{"Missing"})
	_, err := cmd.Confirm(nil, p)
	if err == nil {
		t.Fatal("expected error from Confirm")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

// Verify real delete commands have Confirm set
func TestDeleteCommands_HaveConfirm(t *testing.T) {
	for _, cmd := range []*Command{&deletetaskCmd, &deleteruleCmd, &deletetagCmd} {
		if cmd.Confirm == nil {
			t.Errorf("%s should have Confirm set", cmd.Name)
		}
	}
}

// Verify non-delete commands don't have Confirm
func TestNonDeleteCommands_NoConfirm(t *testing.T) {
	for _, cmd := range []*Command{
		&addtaskCmd, &settaskCmd, &gettaskCmd,
		&addruleCmd, &setruleCmd, &getruleCmd,
		&addtagCmd, &settagCmd, &gettagCmd,
	} {
		if cmd.Confirm != nil {
			t.Errorf("%s should not have Confirm set", cmd.Name)
		}
	}
}

// Verify get commands have RawOutput
func TestGetCommands_HaveRawOutput(t *testing.T) {
	for _, cmd := range []*Command{&gettaskCmd, &getruleCmd, &gettagCmd} {
		if !cmd.RawOutput {
			t.Errorf("%s should have RawOutput=true", cmd.Name)
		}
	}
}

// Verify non-get commands don't have RawOutput
func TestNonGetCommands_NoRawOutput(t *testing.T) {
	for _, cmd := range []*Command{
		&addtaskCmd, &settaskCmd, &deletetaskCmd,
		&addruleCmd, &setruleCmd, &deleteruleCmd,
		&addtagCmd, &settagCmd, &deletetagCmd,
	} {
		if cmd.RawOutput {
			t.Errorf("%s should not have RawOutput=true", cmd.Name)
		}
	}
}

// Verify allCommands has all 12 commands
func TestAllCommands_Count(t *testing.T) {
	if len(allCommands) != 12 {
		t.Errorf("allCommands has %d commands, want 12", len(allCommands))
	}
}

func TestAllCommands_NamesMatchCLICommands(t *testing.T) {
	names := make(map[string]bool)
	for _, cmd := range allCommands {
		names[cmd.Name] = true
	}
	for _, name := range CLICommands {
		if !names[name] {
			t.Errorf("CLICommands has %q but allCommands does not", name)
		}
	}
}

// ===========================================================================
// MCPType override tests
// ===========================================================================

func TestMCPSchema_MCPTypeOverride_Integer(t *testing.T) {
	cmd := Command{
		Name: "test",
		Flags: []Flag{
			{MCP: "depth", MCPType: "integer", Desc: "depth"},
		},
	}
	schema := cmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	depth := props["depth"].(map[string]interface{})
	if depth["type"] != "integer" {
		t.Errorf("depth type = %q, want 'integer'", depth["type"])
	}
}

func TestMCPSchema_MCPTypeOverride_StringOnMulti(t *testing.T) {
	cmd := Command{
		Name: "test",
		Flags: []Flag{
			{CLI: "--deadline", MCP: "deadline", Multi: true, MCPType: "string", Desc: "deadline"},
		},
	}
	schema := cmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	dl := props["deadline"].(map[string]interface{})
	if dl["type"] != "string" {
		t.Errorf("deadline type = %q, want 'string' (MCPType override)", dl["type"])
	}
	// Should not have items since MCPType overrides the array behavior
	if _, ok := dl["items"]; ok {
		t.Error("deadline should not have 'items' when MCPType overrides to string")
	}
}

func TestMCPSchema_RealDepthFlag_IsInteger(t *testing.T) {
	schema := gettaskCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	depth := props["depth"].(map[string]interface{})
	if depth["type"] != "integer" {
		t.Errorf("gettask depth type = %q, want 'integer'", depth["type"])
	}
}

func TestMCPSchema_RealSeqFlags_AreInteger(t *testing.T) {
	for _, cmd := range []*Command{&getruleCmd, &setruleCmd, &deleteruleCmd} {
		schema := cmd.MCPSchema()
		props := schema["properties"].(map[string]interface{})
		seq := props["seq"].(map[string]interface{})
		if seq["type"] != "integer" {
			t.Errorf("%s seq type = %q, want 'integer'", cmd.Name, seq["type"])
		}
	}
}

func TestMCPSchema_RealDeadlineRecur_AreString(t *testing.T) {
	schema := settaskCmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})
	for _, key := range []string{"deadline", "recur"} {
		p := props[key].(map[string]interface{})
		if p["type"] != "string" {
			t.Errorf("settask %s type = %q, want 'string'", key, p["type"])
		}
	}
}

// ===========================================================================
// Reject map tests
// ===========================================================================

func TestParseCLI_Reject_CLIFlag(t *testing.T) {
	cmd := Command{
		Name: "test",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
		},
		Reject: map[string]string{
			"--desc": "--desc is a settask flag",
		},
	}
	_, err := cmd.ParseCLI([]string{"Task", "--desc", "text"})
	if err == nil {
		t.Fatal("expected error for rejected flag, got nil")
	}
	if !strings.Contains(err.Error(), "settask") {
		t.Errorf("error = %q, want it to mention 'settask'", err)
	}
}

func TestParseMCP_Reject_MCPParam(t *testing.T) {
	cmd := Command{
		Name: "test",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
		},
		Reject: map[string]string{
			"desc": "desc is a settask field",
		},
	}
	_, err := cmd.ParseMCP(map[string]interface{}{"name": "Task", "desc": "text"})
	if err == nil {
		t.Fatal("expected error for rejected param, got nil")
	}
	if !strings.Contains(err.Error(), "settask") {
		t.Errorf("error = %q, want it to mention 'settask'", err)
	}
}

func TestParseCLI_Reject_NotTriggeredForValidFlags(t *testing.T) {
	cmd := Command{
		Name: "test",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
			{CLI: "--force", MCP: "force", Bool: true, Desc: "force"},
		},
		Reject: map[string]string{
			"--desc": "--desc is a settask flag",
		},
	}
	p, err := cmd.ParseCLI([]string{"Task", "--force"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Bool("--force") {
		t.Error("expected --force true")
	}
}

func TestParseMCP_Reject_NotTriggeredForValidParams(t *testing.T) {
	cmd := Command{
		Name: "test",
		Flags: []Flag{
			{MCP: "name", Pos: 1, Required: true, Desc: "name"},
		},
		Reject: map[string]string{
			"desc": "desc is a settask field",
		},
	}
	p, err := cmd.ParseMCP(map[string]interface{}{"name": "Task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Arg("name") != "Task" {
		t.Errorf("Arg(name) = %q, want 'Task'", p.Arg("name"))
	}
}

func TestAddtaskCmd_RejectsSettaskFields_CLI(t *testing.T) {
	for _, flag := range []string{"--desc", "--verify", "--verify-status", "--deadline", "--recur"} {
		_, err := addtaskCmd.ParseCLI([]string{"Task", flag, "value"})
		if err == nil {
			t.Fatalf("expected error for %s, got nil", flag)
		}
		if !strings.Contains(err.Error(), "settask") {
			t.Errorf("error for %s = %q, want it to mention 'settask'", flag, err)
		}
	}
}

func TestAddtaskCmd_RejectsSettaskFields_MCP(t *testing.T) {
	for _, field := range []string{"desc", "verify", "verify_status", "deadline", "recur"} {
		_, err := addtaskCmd.ParseMCP(map[string]interface{}{"name": "Task", field: "value"})
		if err == nil {
			t.Fatalf("expected error for %s, got nil", field)
		}
		if !strings.Contains(err.Error(), "settask") {
			t.Errorf("error for %s = %q, want it to mention 'settask'", field, err)
		}
	}
}

// ===========================================================================
// CLI integration tests: ParseCLI -> Run with real store
// ===========================================================================

func TestCLIIntegration_Addtask(t *testing.T) {
	store := testStore(t)
	AddTag(store, "WORK")
	p, err := addtaskCmd.ParseCLI([]string{"My Task", "--tag", "WORK"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := addtaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "My Task") || !strings.Contains(result, "#WORK") {
		t.Errorf("result = %q, want task name and tag", result)
	}
}

func TestCLIIntegration_Addtask_Under(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Parent", "", false, false)
	p, err := addtaskCmd.ParseCLI([]string{"Child", "--under", "Parent"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := addtaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "Child") || !strings.Contains(result, "under Parent") {
		t.Errorf("result = %q, want child name and parent", result)
	}
}

func TestCLIIntegration_Settask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "My Task", "", false, false)
	p, err := settaskCmd.ParseCLI([]string{"My Task", "--status", "active", "--desc", "hello"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := settaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "Task updated") {
		t.Errorf("result = %q, want 'Task updated'", result)
	}
}

func TestCLIIntegration_Settask_Rename(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Old Name", "", false, false)
	p, err := settaskCmd.ParseCLI([]string{"Old Name", "New Name", "--force"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := settaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "New Name") {
		t.Errorf("result = %q, want 'New Name'", result)
	}
}

func TestCLIIntegration_Settask_DeadlineMultiToken(t *testing.T) {
	store := testStore(t)
	AddTask(store, "My Task", "", false, false)
	p, err := settaskCmd.ParseCLI([]string{"My Task", "--deadline", "2026-04-15", "14:00"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := settaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "Task updated") {
		t.Errorf("result = %q, want 'Task updated'", result)
	}
}

func TestCLIIntegration_Settask_RecurMultiToken(t *testing.T) {
	store := testStore(t)
	AddTask(store, "My Task", "", false, false)
	p, err := settaskCmd.ParseCLI([]string{"My Task", "--recur", "weekly", "mon"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := settaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "Task updated") {
		t.Errorf("result = %q, want 'Task updated'", result)
	}
}

func TestCLIIntegration_Settask_Blockers(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Blocker", "", false, false)
	SetTask(store, "Blocker", SetTaskOpts{Status: "active"})
	AddTask(store, "Blocked", "", false, false)
	SetTask(store, "Blocked", SetTaskOpts{Status: "active"})
	p, err := settaskCmd.ParseCLI([]string{"Blocked", "--status", "wait", "--blockers", "Blocker"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := settaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "Task updated") {
		t.Errorf("result = %q, want 'Task updated'", result)
	}
}

func TestCLIIntegration_Gettask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Task A", "", false, false)
	AddTask(store, "Task B", "", false, false)
	p, err := gettaskCmd.ParseCLI([]string{"--all"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := gettaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "Task A") || !strings.Contains(result, "Task B") {
		t.Errorf("result = %q, want both tasks", result)
	}
}

func TestCLIIntegration_Gettask_ByRef(t *testing.T) {
	store := testStore(t)
	AddTask(store, "My Task", "", false, false)
	p, err := gettaskCmd.ParseCLI([]string{"My Task"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := gettaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "My Task") {
		t.Errorf("result = %q, want 'My Task'", result)
	}
}

func TestCLIIntegration_Deletetask(t *testing.T) {
	store := testStore(t)
	AddTask(store, "Doomed", "", false, false)
	p, err := deletetaskCmd.ParseCLI([]string{"Doomed"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := deletetaskCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "Doomed") {
		t.Errorf("result = %q, want 'Doomed'", result)
	}
}

func TestCLIIntegration_Addrule(t *testing.T) {
	store := testStore(t)
	AddTag(store, "BACKEND")
	p, err := addruleCmd.ParseCLI([]string{"No force push", "--tag", "BACKEND"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := addruleCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "No force push") || !strings.Contains(result, "#BACKEND") {
		t.Errorf("result = %q, want rule name and tag", result)
	}
}

func TestCLIIntegration_Getrule(t *testing.T) {
	store := testStore(t)
	AddRule(store, "Rule one")
	p, err := getruleCmd.ParseCLI([]string{"--all"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := getruleCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "Rule one") {
		t.Errorf("result = %q, want 'Rule one'", result)
	}
}

func TestCLIIntegration_Setrule(t *testing.T) {
	store := testStore(t)
	AddRule(store, "Old rule")
	p, err := setruleCmd.ParseCLI([]string{"1", "New rule"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := setruleCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "New rule") {
		t.Errorf("result = %q, want 'New rule'", result)
	}
}

func TestCLIIntegration_Deleterule(t *testing.T) {
	store := testStore(t)
	AddRule(store, "Doomed rule")
	p, err := deleteruleCmd.ParseCLI([]string{"1"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := deleteruleCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "deleted") {
		t.Errorf("result = %q, want 'deleted'", result)
	}
}

func TestCLIIntegration_Addtag(t *testing.T) {
	store := testStore(t)
	p, err := addtagCmd.ParseCLI([]string{"WORK"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := addtagCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "WORK") {
		t.Errorf("result = %q, want 'WORK'", result)
	}
}

func TestCLIIntegration_Gettag(t *testing.T) {
	store := testStore(t)
	AddTag(store, "URGENT")
	p, err := gettagCmd.ParseCLI([]string{"--all"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := gettagCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "URGENT") {
		t.Errorf("result = %q, want 'URGENT'", result)
	}
}

func TestCLIIntegration_Settag(t *testing.T) {
	store := testStore(t)
	AddTag(store, "OLD")
	p, err := settagCmd.ParseCLI([]string{"OLD", "NEW"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := settagCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "NEW") {
		t.Errorf("result = %q, want 'NEW'", result)
	}
}

func TestCLIIntegration_Deletetag(t *testing.T) {
	store := testStore(t)
	AddTag(store, "DOOMED")
	p, err := deletetagCmd.ParseCLI([]string{"DOOMED"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := deletetagCmd.Run(store, p)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(result, "DOOMED") {
		t.Errorf("result = %q, want 'DOOMED'", result)
	}
}

// ===========================================================================
// Schema contract tests: verify real command schemas have correct properties
// ===========================================================================

func TestSchemaContract_Addtask(t *testing.T) {
	assertSchemaProps(t, addtaskCmd, map[string]string{
		"name":     "string",
		"under":    "string",
		"parallel": "boolean",
		"force":    "boolean",
		"tags":     "array",
	}, []string{"name"})
}

func TestSchemaContract_Settask(t *testing.T) {
	assertSchemaProps(t, settaskCmd, map[string]string{
		"task":          "string",
		"name":          "string",
		"force":         "boolean",
		"status":        "string",
		"blockers":      "array",
		"desc":          "string",
		"verify":        "string",
		"verify_status": "string",
		"deadline":      "string",
		"recur":         "string",
		"tags":          "array",
	}, []string{"task"})
}

func TestSchemaContract_Gettask(t *testing.T) {
	assertSchemaProps(t, gettaskCmd, map[string]string{
		"task":    "string",
		"all":     "boolean",
		"details": "boolean",
		"status":  "string",
		"depth":   "integer",
		"tag":     "array",
		"nottag":  "array",
		"sort":    "string",
	}, nil)
}

func TestSchemaContract_Deletetask(t *testing.T) {
	assertSchemaProps(t, deletetaskCmd, map[string]string{
		"task":  "string",
		"force": "boolean",
	}, []string{"task"})
}

func TestSchemaContract_Addrule(t *testing.T) {
	assertSchemaProps(t, addruleCmd, map[string]string{
		"name": "string",
		"tags": "array",
	}, []string{"name"})
}

func TestSchemaContract_Getrule(t *testing.T) {
	assertSchemaProps(t, getruleCmd, map[string]string{
		"seq":     "integer",
		"all":     "boolean",
		"tag":     "string",
		"tag_all": "boolean",
	}, nil)
}

func TestSchemaContract_Setrule(t *testing.T) {
	assertSchemaProps(t, setruleCmd, map[string]string{
		"seq":  "integer",
		"name": "string",
		"tags": "array",
	}, []string{"seq"})
}

func TestSchemaContract_Deleterule(t *testing.T) {
	assertSchemaProps(t, deleteruleCmd, map[string]string{
		"seq":   "integer",
		"force": "boolean",
	}, []string{"seq"})
}

func TestSchemaContract_Addtag(t *testing.T) {
	assertSchemaProps(t, addtagCmd, map[string]string{
		"name": "string",
	}, []string{"name"})
}

func TestSchemaContract_Gettag(t *testing.T) {
	assertSchemaProps(t, gettagCmd, map[string]string{
		"tag": "string",
		"all": "boolean",
	}, nil)
}

func TestSchemaContract_Settag(t *testing.T) {
	assertSchemaProps(t, settagCmd, map[string]string{
		"old": "string",
		"new": "string",
	}, []string{"old", "new"})
}

func TestSchemaContract_Deletetag(t *testing.T) {
	assertSchemaProps(t, deletetagCmd, map[string]string{
		"tag":   "string",
		"force": "boolean",
	}, []string{"tag"})
}

// assertSchemaProps verifies a command's MCPSchema has exactly the expected properties
// with the expected types, and the expected required list.
func assertSchemaProps(t *testing.T, cmd Command, wantProps map[string]string, wantRequired []string) {
	t.Helper()
	schema := cmd.MCPSchema()
	props := schema["properties"].(map[string]interface{})

	if len(props) != len(wantProps) {
		t.Errorf("%s: schema has %d properties, want %d", cmd.Name, len(props), len(wantProps))
	}
	for name, wantType := range wantProps {
		prop, ok := props[name]
		if !ok {
			t.Errorf("%s: schema missing property %q", cmd.Name, name)
			continue
		}
		gotType := prop.(map[string]interface{})["type"]
		if gotType != wantType {
			t.Errorf("%s: property %q type = %q, want %q", cmd.Name, name, gotType, wantType)
		}
	}
	for name := range props {
		if _, ok := wantProps[name]; !ok {
			t.Errorf("%s: schema has unexpected property %q", cmd.Name, name)
		}
	}

	if wantRequired == nil {
		if _, ok := schema["required"]; ok {
			req := schema["required"].([]string)
			if len(req) > 0 {
				t.Errorf("%s: expected no required fields, got %v", cmd.Name, req)
			}
		}
		return
	}
	req, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("%s: schema missing 'required' list", cmd.Name)
	}
	reqSet := make(map[string]bool)
	for _, r := range req {
		reqSet[r] = true
	}
	for _, want := range wantRequired {
		if !reqSet[want] {
			t.Errorf("%s: %q should be required", cmd.Name, want)
		}
	}
	if len(req) != len(wantRequired) {
		t.Errorf("%s: required list has %d items, want %d: %v", cmd.Name, len(req), len(wantRequired), req)
	}
}
