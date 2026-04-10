Butler is a global task and rule manager. It tracks long-term tasks, subtasks, rules, and tags across projects and sessions. Interact with Butler exclusively through these MCP tools - never run butler as a CLI command.

# Principles

1. **MCP only.** Never run butler as a CLI command. Use MCP tools exclusively.
2. **Just-in-time context.** Only fetch task details when about to act on that task. Do not load details speculatively.
3. **Minimize calls.** Use flags (status, tag, nottag, depth, sort, details) to get exactly what is needed per call. Do not fetch the same information twice.
4. **Tag inheritance awareness.** Tags on a parent cascade to all children when viewing a specific task. Tag at the highest relevant level. Never redundantly tag children with tags their parent already carries.
5. **Subtask semantics.** Sequential subtasks (numbered) imply order dependency. Parallel subtasks (lettered) are independent. Respect this distinction.
6. **User agency.** Confirm with the user when selecting between tasks, before creating task structures, before starting work, and when verification is ambiguous. Butler is a tool for the user, not autonomous decision-making.
7. **Rules surface automatically.** Once rules and tasks share tags, applicable rules appear in the details output. No separate rule fetching is needed during execution.
8. **Verification is a hard gate.** Never mark a task completed without checking its verification criteria. No exceptions.

# Activation

## Explicit: "summon butler"

When the user says "summon butler" or a close variation, activate the full Butler workflow starting with Session Initialization below.

Engage Butler when the user:
- Says "summon butler" or "summon butler for [topic]"
- Directly references Butler tasks, rules, or tags by name

Do not engage the full workflow for direct, self-contained requests (fix this bug, explain this code) or quick questions unrelated to task tracking. Not all work needs to be tracked - do not force task creation on every interaction.

## Passive awareness (always active)

Even without explicit invocation, maintain passive awareness. When the user completes work during a session (bug fix, feature, refactor), consider whether it may correspond to a tracked Butler task. If likely, ask the user whether to update Butler - mark the task completed, advance to the next subtask, or note progress. The user decides; you surface the prompt. Do not update without asking.

# Workflow

## Session Initialization

Goal: Orient on existing work and determine what is relevant.

1. Call `gettask` with `all: true, depth: 0` to retrieve all named tasks with statuses and tags.
2. Match tasks against the current working directory, project context, or user's stated intent.
3. Determine the path:
   - Active tasks match current context → Task Resumption.
   - Multiple could match → present them and ask the user.
   - No tasks match or user wants new work → Task Creation.
   - No tasks exist at all → ask the user what they want to accomplish, then Task Creation.
   - User wants to modify existing tasks first → handle modifications, reassess.

Considerations:
- Recurring tasks may have auto-activated since last session. They appear as active - treat normally.
- For deadline-sensitive prioritization, call `gettask` with `all: true, sort: "deadline", depth: 0`.
- Archived tasks are hidden by default. Only use `status: "archived"` if the user asks.
- Filter by tags: `tag` accepts an array of tag names with AND semantics (tasks must match all specified tags). `nottag` accepts an array of tag names to exclude (tasks must match none). Both accept the special value NONE to match or exclude untagged tasks. All tag names are validated for existence (except NONE). Using `nottag` alone is sufficient to query - no need for `all: true`.
- This phase is orientation only. Do not fetch subtask details yet.

## Task Resumption

Goal: Identify the next actionable subtask within a selected task.

1. Call `gettask` with `task: "<name>", details: true` on the selected task. This returns the full hierarchy, statuses, descriptions, verification criteria, inherited tags, and applicable rules. Note: on the queried task, rules from all inherited + own tags apply. On its children, only rules from their own direct tags apply. To get full rule context for a child, query it directly.
2. Identify the next actionable subtask:
   - Sequential subtasks (numbered 1, 2, 3): work the lowest-numbered that is not completed, cancelled, or archived. Order matters - do not skip ahead.
   - Parallel subtasks (lettered a, b, c): work any that is not completed, cancelled, or archived. Dispatch parallel agents if the platform supports it.
   - Skip subtasks with waiting status - they have unresolved blockers. Suggest working on the blocker instead.
   - If a parent is deferred, cancelled, or waiting, children display that same status (inferred, not stored). Their actual status is preserved and restores when the parent resumes. Do not work on children of a suspended parent.
3. Call `gettask` with `task: "<name>:<pos>", details: true` on the specific subtask. This surfaces applicable rules, inherited tags, description, and verification criteria for that subtask.
4. If the subtask has children, recurse deeper to find the actionable leaf.
5. Present the subtask context (description, rules, verification) to the user. Ask whether to proceed.

Considerations:
- A waiting task has explicit blockers. Check whether blockers are in the same task tree (work on them first) or a different task (inform the user).
- If all subtasks are completed, check the parent's verification criteria before completing the parent.
- Task state persists across sessions. When resuming, Butler reflects exactly where the previous session left off. No recovery steps are needed.

## Task Execution

Goal: Complete a specific subtask following its rules and meeting its verification criteria.

1. Read the task description for requirements and context.
2. Follow the applicable rules from the details output - they are binding constraints.
3. Do the work.
4. Before marking complete, check verification criteria:
   - Concrete criteria (tests pass, file exists, endpoint responds): verify directly.
   - Subjective criteria: present completed work and criteria to the user for confirmation.
   - If verification fails, fix the issues and re-verify. Do not mark complete until verification passes.
5. Call `settask` with `task: "<name>:<pos>", status: "completed"`.
6. Proceed to Post-Completion.

Mid-execution:
- Task more complex than expected: create sub-subtasks with `addtask` using `under`, then set `desc` and `verify` on each with `settask`.
- Task blocked by another: call `settask` with `status: "wait", blockers: ["<ref>"]`. Setting wait replaces all existing blockers (does not append). Move to next actionable task.
- Task needs renaming: call `settask` with `task` and `name`. Renaming blocks if a non-archived root task with the new name exists - use `force: true` to archive the conflicting one.
- Task should be deferred: call `settask` with `status: "deferred"`. Move to next task.
- New reusable constraint emerges: create a rule with `addrule` using `tags` so it auto-associates with relevant tasks.
- Conflicting rules: present both to the user, ask for resolution. Update, retire, or re-tag based on their decision. Never silently pick one.

## Task Creation

Goal: Set up new work with proper structure, tagging, rules, and verification.

Before creating, load organizational context:
1. Call `gettag` (no params) to see all existing tags and their usage counts.
2. Call `getrule` with `all: true` to see all existing rules and their tags.

Then:
1. Create the named task with `addtask`.
2. Create subtasks with `addtask` using `under`. Default is sequential (numbered). Use `parallel: true` for independent work. Keep hierarchy 2-3 levels deep. Do not create fine-grained subtasks upfront - add detail when about to work on a subtask. Name subtasks to describe outcomes, not activities.
3. Set description and verification on every task and subtask: call `settask` with `desc` and `verify`. Verification must be concrete enough for another agent in a future session to evaluate without ambiguity.
4. Tag with user confirmation: before adding a task, review existing tags from step 1 and recommend relevant ones based on the current context and work. Present the recommendations to the user and let them confirm, adjust, or decline. Never apply tags without user approval. Only create new tags with `addtag` when the user explicitly requests it -- never create tags on your own initiative. Tag at the highest relevant level -- children inherit parent tags. Do not redundantly tag children with tags their parent already carries. Tags: uppercase alphanumeric, 10 chars max, `NONE` is reserved. Apply with `settask` using `tags` (replaces existing tags on that task). Tags should form a coherent taxonomy, not an ad-hoc collection.
5. Create rules when appropriate: rules are high-level, long-lasting policies - not one-time instructions. They can be updated, retired, or deleted as practices evolve, but should be broadly applicable by default. Do not over-create rules - if a constraint only applies to one task, put it in the task description. Before creating, check existing rules for contradictions. If a new rule conflicts with an existing one, ask the user: update old, replace with new, or keep both with different tags for different contexts. If a rule is too broad or narrow, adjust its tag associations. Create with `addrule` using `tags` so rules auto-associate via details.
6. Set deadlines if time-sensitive: `settask` with `deadline` (YYYY-MM-DD or YYYY-MM-DD HH:MM, "none" to clear).
7. Set recurrence if ongoing: `settask` with `recur`.
8. Set blockers for cross-dependencies: `settask` with `status: "wait", blockers: [...]`.

Constraints: task names 50 chars max. Duplicate root names blocked - use `force: true` to archive existing. A subtask cannot wait on its own ancestor.

## Post-Completion

Goal: Handle cascading effects and advance to next work.

1. Completing a task may unblock waiting tasks. Butler auto-transitions them to active.
2. Check parent: call `gettask` with `task: "<parent>", details: true`. If all siblings are completed or cancelled, verify the parent's own criteria before completing it. Parent verification often includes holistic checks beyond individual subtask criteria.
3. Get fresh details on the next actionable subtask. Do not reuse stale context. If the parent is fully complete, check upward. If all work in the task tree is done, return to Session Initialization.
4. Ask the user before starting the next task.
5. When a full task tree is complete, ask the user whether to archive it with `settask` using `status: "archived"`.

## Task Maintenance

- Archive completed task trees to keep views clean. Confirm with user first - cascades to children. Archiving removes the task from blocker lists of other tasks.
- Prefer archiving over deleting. Delete only for mistakes. Deletion is permanent and also removes from all blocker lists. Waiting tasks with no remaining blockers auto-transition to active.
- Mark unnecessary subtasks as cancelled (preserves history) rather than deleting.
- Restructuring a hierarchy may affect tags, rules, and blockers - discuss with user first.
- Create rules when a constraint has been stated more than once across sessions.
- Update existing rules with `setrule` using `seq` and `name` (new text) and/or `tags` (replaces existing tags). Retire rules that no longer reflect current practices with `deleterule`. Rules are policies, not laws - they evolve.
- Check for unused tags periodically with `gettag` (no params). Remove empty ones with `deletetag` - this removes the tag from all tasks and rules.
- Rename tags with `settag` using `old` and `new` - all task and rule associations follow automatically.

# Status Reference

Statuses: not_started, active, waiting, deferred, completed, reopened, cancelled, archived.

Key transitions: not_started to active. active to waiting, deferred, or completed. waiting to active (auto when blockers clear). deferred to active. completed to reopened. reopened to active. Any status to cancelled. Any status to archived. archived to active.

waiting requires blocker refs. archived only on named (top-level) tasks - cascades to children.

Parent-child rules: parent cancelled/deferred/waiting means children display that same status. Child becomes active means not_started ancestors auto-activate. Child reopened or new child added means completed ancestors auto-reopen. Parent cannot complete unless all children are completed or cancelled. Complete children before parents - completing a child of a completed parent auto-reopens the parent.

# Edge Cases

- All subtasks done but parent verification fails: do not complete the parent. Identify what is missing - it may require additional subtasks, fixes, or updated criteria. Discuss with the user.
- Circular blockers: Butler prevents a subtask from waiting on its own ancestor. If a blocker does not make sense, set the task to a non-waiting status and restructure dependencies.
- Recurring task reappears after completion: this is expected. Recurrence only fires when a task is in `completed` or `not_started` status. Tasks that are `archived`, `cancelled`, or `deferred` skip recurrence entirely. Recurrence is checked on every Butler command. If recurrence is no longer needed, clear with `settask` using `recur: "none"`. Recurrence patterns: `daily`, `daily HH:MM`, `weekly DAY,DAY`, `monthly N,N`, `every Nd`/`Nh`/`Nw`/`Nmon`/`Nmin`.
- Multiple agents or sessions: concurrent access is safe (SQLite WAL mode with locking), but two agents on the same task may conflict. Use task status as coordination - an active task is being worked on.
