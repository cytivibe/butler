package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Color presets for CLI output. These are no-ops when color is disabled
// (piped output, MCP mode, NO_COLOR env).
var (
	colorGreen    = color.New(color.FgGreen)
	colorYellow   = color.New(color.FgYellow)
	colorRed      = color.New(color.FgRed)
	colorCyan     = color.New(color.FgCyan)
	colorMagenta  = color.New(color.FgMagenta)
	colorDim      = color.New(color.Faint)
	colorBold     = color.New(color.Bold)
	colorDimGreen = color.New(color.FgGreen, color.Faint)
	colorDimRed   = color.New(color.FgRed, color.Faint)
)

// cliColor controls whether colored output is used.
// Set to true in main() for CLI mode; false for MCP/tests.
var cliColor bool

// colorStatus returns the status badge with color for CLI, plain for MCP.
func colorStatus(status, timestamp string) string {
	tag := formatStatusWithTime(status, timestamp)
	if !cliColor {
		return tag
	}
	switch status {
	case "active":
		return colorGreen.Sprint(tag)
	case "completed":
		return colorDimGreen.Sprint(tag)
	case "waiting", "deferred":
		return colorYellow.Sprint(tag)
	case "reopened":
		return colorCyan.Sprint(tag)
	case "cancelled":
		return colorDimRed.Sprint(tag)
	case "not_started", "archived":
		return colorDim.Sprint(tag)
	default:
		return tag
	}
}

// colorOverdue returns a red-colored overdue badge for CLI.
func colorOverdue(dl string) string {
	s := fmt.Sprintf("[overdue: %s]", dl)
	if !cliColor {
		return s
	}
	return colorRed.Sprint(s)
}

// colorDeadline returns a styled deadline badge.
func colorDeadline(dl string) string {
	s := fmt.Sprintf("[deadline: %s]", dl)
	if !cliColor {
		return s
	}
	return colorYellow.Sprint(s)
}

// colorRecur returns a styled recurrence badge.
func colorRecur(pattern string) string {
	s := fmt.Sprintf("[recurs %s]", pattern)
	if !cliColor {
		return s
	}
	return colorCyan.Sprint(s)
}

// colorVerifyPending returns a styled verify pending badge.
func colorVerifyPending() string {
	s := "[verify: pending]"
	if !cliColor {
		return s
	}
	return colorYellow.Sprint(s)
}

// colorVerifyPassed returns a styled verify passed badge.
func colorVerifyPassed() string {
	s := "[verify: passed]"
	if !cliColor {
		return s
	}
	return colorGreen.Sprint(s)
}

// colorTag returns a colored tag like "#URGENT".
func colorTag(tag string) string {
	if !cliColor {
		return "#" + tag
	}
	return colorMagenta.Sprint("#" + tag)
}

// colorTags formats multiple tags with color.
func colorTags(tags string) string {
	if !cliColor || tags == "" {
		return tags
	}
	parts := strings.Fields(tags)
	for i, p := range parts {
		if strings.HasPrefix(p, "#") {
			parts[i] = colorMagenta.Sprint(p)
		}
	}
	return strings.Join(parts, " ")
}

// colorPosition returns a bold position prefix like "1." or "a.".
func colorPosition(pos string) string {
	if !cliColor {
		return pos + "."
	}
	return colorBold.Sprint(pos + ".")
}

// colorTaskName returns a bold task name.
func colorTaskName(name string) string {
	if !cliColor {
		return name
	}
	return colorBold.Sprint(name)
}

// colorRootMarker returns a colored bullet for root/named tasks.
func colorRootMarker() string {
	if !cliColor {
		return ""
	}
	return colorGreen.Sprint("● ")
}

// Tree drawing characters.
const (
	treeBranch   = "├── "
	treeLastItem = "└── "
	treePipe     = "│   "
	treeBlank    = "    "
)

// colorTree returns dimmed tree characters for CLI.
func colorTree(s string) string {
	if !cliColor {
		return s
	}
	return colorDim.Sprint(s)
}

// colorDetailLabel returns a dimmed label like "desc:" or "verify:".
func colorDetailLabel(label string) string {
	if !cliColor {
		return label
	}
	return colorDim.Sprint(label)
}

// colorDimText returns dim text for detail values - keeps task names prominent.
func colorDimText(s string) string {
	if !cliColor {
		return s
	}
	return colorDim.Sprint(s)
}

// colorWaiting returns a colored waiting-with-blockers badge.
func colorWaiting(blockers, ts string) string {
	s := fmt.Sprintf("[waiting: %s since %s]", blockers, ts)
	if !cliColor {
		return s
	}
	return colorYellow.Sprint(s)
}

// colorRuleHeader returns a colored "Rule N:" prefix.
func colorRuleHeader(seq int) string {
	s := fmt.Sprintf("Rule %d:", seq)
	if !cliColor {
		return s
	}
	return colorCyan.Sprint(s)
}

// colorTagHeader returns a colored "#TAG" section header.
func colorTagHeader(tag string) string {
	if !cliColor {
		return "#" + tag
	}
	return colorMagenta.Sprint("#" + tag)
}

// colorSuccess returns a green success message.
func colorSuccess(msg string) string {
	if !cliColor {
		return msg
	}
	return colorGreen.Sprint(msg)
}

// colorError returns a red error message.
func colorError(msg string) string {
	if !cliColor {
		return msg
	}
	return colorRed.Sprint(msg)
}
