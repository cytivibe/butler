package main

import (
	"fmt"
	"os"
)

func printHelp() {
	fmt.Println(`Butler — CLI + MCP task and rule manager

Tasks
  addtask      Create a task or subtask
  settask      Update name, status, description, tags, deadline, recurrence
  gettask      View tasks with filters and sorting
  deletetask   Permanently delete a task and its children

Rules
  addrule      Create a rule
  setrule      Update a rule's name or tags
  getrule      View rules with filters
  deleterule   Delete a rule (slot reused by next addrule)

Tags
  addtag       Create a tag (uppercase alphanumeric, 10 chars max)
  settag       Rename a tag
  gettag       View tags and their usage
  deletetag    Delete a tag and remove from all tasks/rules

Data
  export       Export all data to JSON
  import       Import data from JSON file

Other
  serve        Start MCP server over stdio
  help         Show this help (use 'butler help <command>' for details)

Run 'butler help <command>' for detailed usage of any command.`)
}

func printCommandHelp(cmd string) {
	help, ok := commandHelp[cmd]
	if !ok {
		fmt.Printf("Unknown command: %s\nRun 'butler help' for a list of commands.\n", cmd)
		os.Exit(1)
	}
	fmt.Println(help)
}

var commandHelp = map[string]string{
	"addtask": `addtask — Create a task or subtask

Usage:
  butler addtask "task name" [--under "task:pos"] [--parallel] [--force] [--tag TAG1 TAG2]

Flags:
  --under "task:pos"   Create as a subtask under the given position
  --parallel           Use lettered positions (a, b, c) instead of numbered (1, 2, 3)
  --force              Archive any existing non-archived task with the same name
  --tag TAG1 TAG2      Assign tags on creation

Examples:
  butler addtask "Email boss"                          Top-level task
  butler addtask "Draft" --under "Email boss"          Sequential subtask (1, 2, 3...)
  butler addtask "Review" --under "Email boss:1"       Nested under position 1 (becomes 1.1)
  butler addtask "Option" --under "Email boss" --parallel   Parallel subtask (a, b, c...)
  butler addtask "Bug fix" --tag URGENT BACKEND        Create with tags
  butler addtask "Email boss" --force                  Archive old "Email boss", create new

Why use addtask:
  Tasks are the core unit of work. Top-level tasks are identified by name, subtasks
  by position path (e.g. "Email boss:1.a"). Sequential subtasks imply order (do 1 then
  2), parallel subtasks can be done in any order. Use --under to build hierarchies.
  Blocks if a non-archived root task with the same name exists — use --force to
  archive the old one and start fresh.`,

	"settask": `settask — Update a task's name, status, description, tags, deadline, or recurrence

Usage:
  butler settask "task:pos" ["new name"] [--status STATUS] [--desc "text"]
    [--verify "criteria"] [--deadline DATE] [--recur PATTERN] [--tag TAG1 TAG2] [--force]

Flags:
  --status STATUS      Set status (see values below)
  --desc "text"        Set or update description
  --verify "criteria"  Set verification criteria
  --deadline DATE      Set deadline (YYYY-MM-DD or "YYYY-MM-DD HH:MM"), "none" to clear
  --recur PATTERN      Set recurrence (see patterns below), "none" to clear
  --tag TAG1 TAG2      Replace all tags with the given tags
  --force              When renaming, archive any conflicting non-archived task

Status values:
  active       Start working on it
  deferred     Postpone it (children inherit deferred display)
  completed    Mark done (requires all children complete/cancelled)
  reopened     Reopen a completed task
  cancelled    Cancel it (children inherit cancelled display)
  archived     Archive task and all children (top-level only)
  wait         Set to waiting with blockers: --status wait "blocker:pos" "other"

Recurrence patterns:
  daily              Every day
  daily 09:00        Every day at 9am
  weekly mon,thu     Specific weekdays
  monthly 1,15       Specific days of month
  every 2d           Every 2 days (also: min, h, w, mon)

Examples:
  butler settask "Email boss" --status active
  butler settask "Email boss:1" --status completed
  butler settask "Email boss:2" --status wait "Email boss:1"
  butler settask "Email boss" --desc "Send the quarterly report"
  butler settask "Email boss" --deadline "2026-04-15 14:00"
  butler settask "Email boss" --recur "weekly mon"
  butler settask "Email boss" --tag URGENT
  butler settask "Email boss" "Email manager" --force
  butler settask "Email boss" --status active --desc "text" --tag URGENT

Why use settask:
  This is the Swiss Army knife for task updates. Combine multiple flags in one call.
  Status transitions are validated (e.g. can't complete a task with incomplete children).
  Setting status to "wait" requires blocker refs — the task auto-activates when all
  blockers complete. Archived status cascades to all children and cleans up blockers.`,

	"gettask": `gettask — View tasks with hierarchy, statuses, and filters

Usage:
  butler gettask --all [--status STATUS] [--tag TAG] [--depth N] [--sort FIELD] [--details]
  butler gettask "task:pos" [--depth N] [--details]

Flags:
  --all              Show all tasks (required when not specifying a task ref)
  --status STATUS    Filter by status (active, completed, archived, etc.)
  --tag TAG          Filter by tag name, "NONE" for untagged tasks
  --depth N          Limit subtask depth (0 = task only, 1 = direct children, etc.)
  --sort recent      Sort by most recently changed
  --sort deadline    Sort by deadline (earliest first, no-deadline last)
  --details          Show description, verification, rules, and inherited tags

Examples:
  butler gettask --all                                 All tasks with hierarchy
  butler gettask "Email boss"                          Single task and its subtasks
  butler gettask "Email boss:1"                        Specific subtask
  butler gettask --all --status active                 Only active tasks
  butler gettask --all --tag URGENT                    Only tasks tagged URGENT
  butler gettask --all --sort deadline                 Sorted by deadline
  butler gettask "Email boss" --details                Full details with rules

Why use gettask:
  The main way to see what needs doing. Shows task hierarchy with status indicators,
  deadlines (with overdue warnings), blocker references, and tags. Archived tasks are
  hidden unless you filter with --status archived. Use --details for the full picture
  including descriptions, verification criteria, and rules linked through tags.`,

	"deletetask": `deletetask — Permanently delete a task and all its children

Usage:
  butler deletetask "task:pos" [--force]

Flags:
  --force    Skip the confirmation prompt

Examples:
  butler deletetask "Email boss"            Delete with confirmation
  butler deletetask "Email boss:1" --force  Delete subtask without confirmation

Why use deletetask:
  Permanent deletion — removes the task, all its children, all blocker references,
  and all tag assignments. Waiting tasks that lose their last blocker auto-activate.
  Prefer archiving (settask --status archived) if you might want to see the task
  again. Use deletetask when you want it truly gone.`,

	"addrule": `addrule — Create a rule

Usage:
  butler addrule "rule text" [--tag TAG1 TAG2]

Flags:
  --tag TAG1 TAG2    Assign tags on creation

Examples:
  butler addrule "Always write tests first"
  butler addrule "Use semantic versioning" --tag BACKEND

Why use addrule:
  Rules are guidelines or constraints attached to your workflow. Tag rules to associate
  them with tasks — when viewing a task with --details, rules linked through shared
  tags are shown. Rules are numbered sequentially. Deleted rule slots are reused by
  the next addrule call.`,

	"setrule": `setrule — Update a rule's name or tags

Usage:
  butler setrule SEQ ["new name"] [--tag TAG1 TAG2]

Flags:
  --tag TAG1 TAG2    Replace all tags (removes existing tags first)

Examples:
  butler setrule 1 "Always write tests"
  butler setrule 1 --tag BACKEND FRONTEND
  butler setrule 1 "Always write tests" --tag BACKEND

Why use setrule:
  Update an existing rule's text or change which tags it's associated with. Tag
  changes automatically update the ruletag flag on the affected tags.`,

	"getrule": `getrule — View rules with optional filters

Usage:
  butler getrule [SEQ] [--tag TAG] [--tag-all]

Flags:
  SEQ          Show a specific rule by number
  --tag TAG    Filter rules by tag, "NONE" for untagged rules
  --tag-all    Group all rules by their tags

Examples:
  butler getrule                  All rules in order
  butler getrule 1                Specific rule by number
  butler getrule --tag BACKEND    Rules tagged BACKEND
  butler getrule --tag NONE       Untagged rules
  butler getrule --tag-all        All rules grouped by tag

Why use getrule:
  See your rules at a glance. Use --tag-all for a grouped overview of which rules
  apply to which areas. Rules tagged with the same tags as a task will appear in
  that task's --details view.`,

	"deleterule": `deleterule — Delete a rule by number

Usage:
  butler deleterule SEQ [--force]

Flags:
  --force    Skip the confirmation prompt

Examples:
  butler deleterule 3
  butler deleterule 3 --force

Why use deleterule:
  Soft-deletes the rule by clearing its name and tags. The slot number is reused by
  the next addrule call, keeping rule numbers compact.`,

	"addtag": `addtag — Create a tag

Usage:
  butler addtag TAG

Constraints:
  - Uppercase letters and digits only (A-Z, 0-9)
  - 10 characters maximum
  - "NONE" is reserved

Examples:
  butler addtag URGENT
  butler addtag BACKEND
  butler addtag Q2

Why use addtag:
  Tags connect tasks and rules. A task tagged BACKEND will show BACKEND-tagged rules
  in its --details view. Tags must be created before they can be assigned. Subtasks
  inherit their parent's tags for display and filtering purposes.`,

	"settag": `settag — Rename a tag

Usage:
  butler settag OLD NEW

Examples:
  butler settag BACKEND API

Why use settag:
  Renames a tag everywhere — all task and rule associations follow automatically.
  The new name must follow the same rules (uppercase alphanumeric, 10 chars max).`,

	"gettag": `gettag — View tags and their usage

Usage:
  butler gettag [TAG]

Examples:
  butler gettag              All tags with task/rule counts
  butler gettag BACKEND      Tasks and rules using BACKEND

Why use gettag:
  Without an argument, shows all tags with counts of how many tasks and rules use
  each. With a tag name, shows the specific tasks and rules using that tag.`,

	"deletetag": `deletetag — Permanently delete a tag

Usage:
  butler deletetag TAG [--force]

Flags:
  --force    Skip the confirmation prompt

Examples:
  butler deletetag OBSOLETE
  butler deletetag OBSOLETE --force

Why use deletetag:
  Removes the tag from all tasks and rules, then deletes it. Tasks that had rules
  linked through this tag will no longer see those rules in --details. This is
  permanent.`,

	"export": `export — Export all tasks, rules, and tags to JSON

Usage:
  butler export [file.json]

If a file path is given, writes to that file. Otherwise prints to stdout.

Examples:
  butler export                     Print JSON to stdout
  butler export backup.json         Save to backup.json
  butler export ~/butler-data.json  Save to home directory

Why use export:
  Back up your butler data, transfer it to another machine, or inspect
  the raw data structure. The JSON format preserves the full task hierarchy,
  all statuses, tags, rules, blockers, deadlines, and recurrence patterns.
  Use with import to restore or migrate data.`,

	"import": `import — Import tasks, rules, and tags from a JSON file

Usage:
  butler import <file.json> [--replace]

Flags:
  --replace    Delete all existing data before importing (default: merge)

Merge behavior (default):
  - Tags: skips tags that already exist
  - Rules: skips rules with the same seq number
  - Tasks: skips root tasks with the same name (non-archived)
  - Blockers are resolved after all tasks are imported

Replace behavior:
  - Deletes ALL existing tasks, rules, and tags first
  - Then imports everything from the file

Examples:
  butler import backup.json             Merge: add new data, skip duplicates
  butler import backup.json --replace   Replace: wipe and restore from file

Why use import:
  Restore from a backup, migrate data from another machine, or merge data
  from multiple butler instances. Use --replace for a clean restore, or
  omit it to add missing items without touching existing ones.`,

	"serve": `serve — Start MCP server over stdio

Usage:
  butler serve

Why use serve:
  Runs butler as an MCP (Model Context Protocol) server, exposing all commands as
  tools over stdin/stdout. Used for integration with AI assistants and other MCP
  clients. All CLI commands have equivalent MCP tools.`,

	"help": `help — Show help for butler commands

Usage:
  butler help              List all commands
  butler help <command>    Detailed help for a specific command

Examples:
  butler help addtask
  butler help settask`,
}
