package main

import (
	"strings"
	"testing"
)

func TestPrintCommandHelpKnown(t *testing.T) {
	// Verify all CLI commands have help entries
	for _, cmd := range CLICommands {
		if _, ok := commandHelp[cmd]; !ok {
			t.Errorf("CLI command '%s' has no help entry", cmd)
		}
	}
}

func TestCommandHelpCoversServeAndHelp(t *testing.T) {
	for _, cmd := range []string{"serve", "help"} {
		if _, ok := commandHelp[cmd]; !ok {
			t.Errorf("command '%s' has no help entry", cmd)
		}
	}
}

func TestPrintHelp(t *testing.T) {
	out := captureStdout(t, func() { printHelp() })
	if !strings.Contains(out, "Butler") {
		t.Fatal("expected 'Butler' in help output")
	}
	if !strings.Contains(out, "addtask") || !strings.Contains(out, "serve") {
		t.Fatal("expected command names in help output")
	}
}

func TestPrintCommandHelp(t *testing.T) {
	out := captureStdout(t, func() { printCommandHelp("addtask") })
	if !strings.Contains(out, "addtask") || !strings.Contains(out, "Usage") {
		t.Fatalf("expected addtask help, got: %s", out)
	}
}
