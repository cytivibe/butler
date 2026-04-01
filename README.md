# Butler

A personal task manager that AI agents use via MCP to remember what you're working on, what to do next, and what rules to follow.

**100% local. 100% private.** Everything lives in a single file on your machine — `~/.butler/butler.db`. No accounts, no cloud, no telemetry.

## Install

### macOS / Linux

Install:
```
curl -fsSL https://raw.githubusercontent.com/cytivibe/butler/main/install.sh | sh
```

Add to Claude Code:
```
claude mcp add butler --scope user -- butler serve
```

Uninstall:
```
sudo butler uninstall
claude mcp remove butler
```

### Windows

Install (PowerShell):
```
irm https://raw.githubusercontent.com/cytivibe/butler/main/install.ps1 | iex
```

Add to Claude Code (PowerShell):
```
claude mcp add butler --scope user -- cmd /c "%LOCALAPPDATA%\butler\butler.exe" serve
```

Uninstall:
```
butler uninstall
claude mcp remove butler
```

### Manual download

Download the binary for your platform from [GitHub Releases](https://github.com/cytivibe/butler/releases), make it executable, and move it to a directory in your PATH.

### Start using Butler

Start a new Claude Code session and say **"summon butler"** to activate the workflow.

## Commands

### Tasks

- `addtask` — create a task (50 chars max)
  - Flags: `--under "task:pos"` (parent ref), `--parallel` (lettered instead of numbered), `--force` (archive existing task with same name), `--tag` (tag on creation)
  - Blocks if a non-archived root task with the same name exists; use `--force` to archive the existing one
  - `butler addtask "task name"` — top-level task
  - `butler addtask "subtask" --under "task"` — sequential subtask (numbered 1, 2, 3...)
  - `butler addtask "subtask" --under "task" --parallel` — parallel subtask (lettered a, b, c... z, aa, ab...)
  - `butler addtask "subtask" --under "task:1"` — nested under position 1 (becomes 1.1)
  - `butler addtask "subtask" --under "task:1.a"` — nested under parallel subtask a (becomes 1.a.1)
  - `butler addtask "task name" --tag URGENT BACKEND` — create and tag in one call
- `settask "task:pos"` — update name, status, description, verification, tags, or blockers
  - Flags: `--status`, `--desc`, `--verify`, `--deadline`, `--recur`, `--tag`, `--force` (can combine multiple in one call)
  - Renaming blocks if a non-archived root task with the new name exists; use `--force` to archive the conflicting one
  - `--status` values: `active`, `deferred`, `completed`, `reopened`, `cancelled`, `archived`, `wait`
  - `--status archived` only works on named (top-level) tasks, cascades to all children, removes task from blocker lists
  - `--status wait` requires one or more blocker refs after it (replaces existing blockers)
  - `--tag` replaces all existing tags with the given tags
  - `butler settask "task" --status active` — set status
  - `butler settask "task:1.a" --status completed` — set subtask status (colon notation)
  - `butler settask "task:2" --status wait "task:1"` — wait on a blocker
  - `butler settask "task:2" --status wait "task:1" "Other:3"` — wait on multiple blockers
  - `butler settask "task:1" --desc "description text"` — set description
  - `butler settask "task:1" --verify "criteria"` — set verification criteria
  - `butler settask "task" --tag URGENT BACKEND` — set tags (replaces existing)
  - `butler settask "task" --deadline "2026-04-15"` — set deadline (date)
  - `butler settask "task" --deadline "2026-04-15 14:00"` — set deadline (datetime, 24h)
  - `butler settask "task" --deadline none` — clear deadline
  - `butler settask "task" --recur daily` — recur daily (auto-activates)
  - `butler settask "task" --recur "daily 09:00"` — recur daily at 9am
  - `butler settask "task" --recur "weekly mon,thu"` — recur on specific weekdays
  - `butler settask "task" --recur "monthly 1,15"` — recur on specific days of month
  - `butler settask "task" --recur "every 2d"` — recur every 2 days (also: `min`, `h`, `w`, `mon`)
  - `butler settask "task" --recur none` — clear recurrence
  - `butler settask "task:1" "new task"` — update a task
  - `butler settask "task" --status archived` — archive task and all children
  - `butler settask "task" --status active --desc "text" --verify "criteria"` — combine in one call
- `deletetask "task:pos"` — permanently delete a task and all its children
  - Flags: `--force` (skip confirmation prompt)
  - Shows confirmation prompt by default: "Delete X and N children? This is permanent. (y/N)"
  - Removes deleted task from all blocker lists; waiting tasks with no blockers left auto-transition to active
  - `butler deletetask "task"` — delete with confirmation
  - `butler deletetask "task:1.a" --force` — delete without confirmation
- `gettask` — view tasks (requires `--all`, a task ref, `--tag`, or `--status`)
  - Flags: `--all`, `--status "STATUS"`, `--tag "TAG"`, `--depth N`, `--sort recent`, `--details`
  - `--depth` values: `0` = task itself only, `1` = direct children, `2` = grandchildren, etc. Default unlimited.
  - `--status` values: `not_started`, `active`, `waiting`, `deferred`, `completed`, `reopened`, `cancelled`, `archived`
  - Archived tasks are hidden by default, only shown with `--status archived`
  - When viewing a specific task, tags are inherited from all parent tasks
  - `butler gettask --all` — all tasks with hierarchy, statuses, and blockers
  - `butler gettask "task:pos"` — specific task and its children (with inherited tags)
  - `butler gettask --all --depth 0` — all named tasks only, no children
  - `butler gettask --status active` — all active tasks and their children
  - `butler gettask --status active --depth 1` — active tasks + direct children only
  - `butler gettask --tag URGENT` — all tasks tagged URGENT
  - `butler gettask --tag NONE` — all tasks with no tags
  - `butler gettask "task:1" --status waiting` — waiting children of task:1
  - `butler gettask --all --sort recent` — most recently changed first
  - `butler gettask --all --sort deadline` — soonest deadline first, no-deadline last
  - `butler gettask --all --details` — includes created date, desc, verify, and applicable rules
  - `butler gettask --all --status archived` — show archived tasks
  - `butler gettask "task:1" --details` — specific task with details, inherited tags, and applicable rules

### Rules

- `addrule` — create a rule (reuses lowest deleted slot, or auto-assigns next number)
  - Flags: `--tag` (tag on creation)
  - `butler addrule "rule name"` — saves a rule, prints `Rule N added: rule name`
  - `butler addrule "rule name" --tag BACKEND` — create and tag in one call
- `setrule N` — update a rule by index
  - Flags: `--tag` (replaces existing tags)
  - `butler setrule 5 "updated rule"` — update rule 5
  - `butler setrule 5 --tag TAG1 TAG2` — set tags (replaces existing)
  - `butler setrule 5 "updated rule" --tag TAG1` — update and set tags
- `deleterule N` — delete a rule by index (soft-delete, slot is reused by next `addrule`)
  - Flags: `--force` (skip confirmation prompt)
  - Shows confirmation prompt by default: "Delete Rule N: name? This is permanent. (y/N)"
  - `butler deleterule 5` — delete rule 5 with confirmation
  - `butler deleterule 5 --force` — delete without confirmation
- `getrule` — view rules (requires `--all`, index, or `--tag`)
  - Display format: `Rule N: rule name #TAG1 #TAG2`
  - Tags are shown with `--all` and by index, hidden with `--tag`
  - `butler getrule --all` — all rules in creation order
  - `butler getrule 5` — rule number 5
  - `butler getrule --tag BACKEND` — rules tagged BACKEND (no tags shown)
  - `butler getrule --tag NONE` — rules with no tags
  - `butler getrule --tag --all` — all rules grouped by tag (alphabetical, with repeats for multi-tagged rules, untagged under `#NONE`)

### Tags

- `addtag "TAG"` — saves a tag (uppercase alphanumeric, 10 chars max, `NONE` is reserved)
  - `butler addtag "URGENT"` — create a tag
- `settag "OLDTAG" "NEWTAG"` — rename a tag (all task and rule associations follow automatically)
  - `butler settag "OLD" "NEW"` — rename OLD to NEW
- `gettag` — view tags (requires `--all` or a tag name)
  - `butler gettag --all` — lists all tags with task/rule counts (e.g. `BACKEND [3 tasks, 1 rule]`)
  - `butler gettag BACKEND` — shows tag details with counts, lists all tasks and rules using that tag
- `deletetag TAG` — permanently delete a tag and remove it from all tasks and rules
  - Flags: `--force` (skip confirmation prompt)
  - `butler deletetag BACKEND` — delete with confirmation
  - `butler deletetag BACKEND --force` — delete without confirmation

### Data

- `export` — export all tasks, rules, and tags to JSON
  - `butler export` — print JSON to stdout
  - `butler export backup.json` — save to file
- `import` — import data from a JSON file
  - Flags: `--replace` (delete all existing data before importing; default is merge)
  - Merge (default): skips duplicates by name (tags, rules, root tasks)
  - Replace: wipes all data first, then imports
  - `butler import backup.json` — merge into existing data
  - `butler import backup.json --replace` — replace all data

### Other

- `butler --help` — list all commands; `butler --help <command>` for detailed usage, flags, and examples
- `butler serve` — starts the MCP server over stdio
- `butler uninstall` — remove butler binary and `~/.butler/` data directory
  - Flags: `--force` (skip confirmation prompt)
  - May require `sudo` if installed to `/usr/local/bin`

## Task Statuses

`not_started` → `active` ↔ `deferred`
`active` → `waiting` (via --status wait) | `completed` → `reopened` → `active`
Any state → `cancelled`
Any state → `archived` → `active`

- `waiting` always requires blocker(s), auto-transitions to `active` when all blockers complete
- `deferred` returns to `active`, not `not_started`
- `archived` can only be set on named (top-level) tasks — cascades to all children, removes from blocker lists
- A subtask cannot wait on its own ancestor (deadlock prevention)

## Recurring Tasks

- `--recur` makes a task auto-activate on a schedule
- Calendar-based: `daily`, `weekly`, `monthly` — activates at fixed points
- Interval-based: `every Nd`, `every Nh`, etc. — activates at intervals from creation time
- When a recurring task is `completed` or `not_started` and its next occurrence is due, it auto-transitions to `active`
- `archived`, `cancelled`, or `deferred` tasks skip recurrence
- Checked automatically on every butler command

## Tag Inheritance & Rules

- When viewing a specific task (`gettask "task:pos"`), the task displays its own tags plus all tags from its parents
- Children below it show only their own direct tags (no redundancy)
- In `--all` view, each task shows only its own direct tags
- With `--details`, rules that share a tag with the task are listed under `rules:`
  - On the queried task: rules from all inherited + own tags
  - On children: rules from only their own direct tags

## Parent-Child Status Rules

**Downward (inferred, not stored):** If a parent is `cancelled`, `deferred`, or `waiting`, all children display that same status. Their actual status is preserved in DB and restores when the parent resumes.

**Upward (auto-transitions):**
- Child becomes `active` → `not_started` ancestors auto-activate
- Child `reopened` or new child added → `completed` ancestors auto-reopen
- Parent cannot be `completed` unless all children are `completed` or `cancelled`

## MCP Server

All commands are available as MCP tools. See [MCP setup](#mcp-setup-claude-code) for configuration.

Tools: `addtask`, `settask`, `gettask`, `deletetask`, `addrule`, `setrule`, `getrule`, `deleterule`, `addtag`, `settag`, `gettag`, `deletetag`

## Architecture

```
Transport    ─  mcp.go (MCP protocol) + main.go (CLI)
Service      ─  service.go (business logic)
Data         ─  store.go (SQLite, transactions, locking)
```

- `store.go` — database layer. All reads/writes go through `WriteTx`/`ReadTx` with automatic locking, transactions, and rollback.
- `mcp.go` — MCP protocol layer. Handles JSON-RPC, tool registration, and dispatch. Add tools via `AddTool`.
- `main.go` — CLI routing and command logic. New commands go here.
- `service.go` — business logic, state machine, and helpers.

## Storage

- SQLite database at `~/.butler/butler.db`
- Tables: `tasks`, `task_blockers`, `task_tags`, `rules`, `rule_tags`, `tags`, `config`
- WAL mode, busy timeout, foreign keys enabled

