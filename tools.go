package main

import (
	"fmt"
	"strings"
)

func requireString(params map[string]interface{}, key string) (string, error) {
	v, ok := params[key].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("'%s' is required and must be a non-empty string", key)
	}
	return v, nil
}

func requireInt(params map[string]interface{}, key string) (int, error) {
	v, ok := params[key].(float64)
	if !ok {
		return 0, fmt.Errorf("'%s' is required and must be an integer", key)
	}
	return int(v), nil
}

func toStringSlice(v []interface{}) ([]string, error) {
	out := make([]string, len(v))
	for i, item := range v {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("array element %d must be a string", i)
		}
		out[i] = s
	}
	return out, nil
}

func registerTools(mcp *MCPServer, store *Store) {
	mcp.AddTool(Tool{
		Name:        "addtask",
		Description: "Add a task. Use 'under' for subtasks with 'task:pos' notation (e.g. 'Email boss:1.a'). Use parallel=true for parallel (lettered) tasks.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The task name (50 chars max)",
				},
				"under": map[string]interface{}{
					"type":        "string",
					"description": "Parent task reference, e.g. 'Email boss' or 'Email boss:1.a'. Omit for top-level task.",
				},
				"parallel": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, task is parallel (lettered a,b,c...). Default is sequential (numbered 1,2,3...).",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tags to apply on creation, e.g. ['URGENT','BACKEND']",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, archive any existing non-archived task with the same name",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			name, err := requireString(params, "name")
			if err != nil {
				return "", err
			}
			under := ""
			if u, ok := params["under"].(string); ok {
				under = u
			}
			parallel := false
			if p, ok := params["parallel"].(bool); ok {
				parallel = p
			}
			force := false
			if f, ok := params["force"].(bool); ok {
				force = f
			}
			var tags []string
			if t, ok := params["tags"].([]interface{}); ok {
				tags, err = toStringSlice(t)
				if err != nil {
					return "", err
				}
			}
			for _, field := range []string{"desc", "verify", "deadline", "recur"} {
				if _, ok := params[field]; ok {
					return "", fmt.Errorf("%s is a settask field — create the task first, then use settask to set it", field)
				}
			}
			if err := AddTask(store, name, under, parallel, force, tags...); err != nil {
				return "", err
			}
			msg := fmt.Sprintf("Task added: %s", name)
			for _, t := range tags {
				msg += " #" + t
			}
			return msg, nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "settask",
		Description: "Update a task. Set name, status, description, verification, tags, or blockers. Use 'wait' status with blockers array. Combine multiple fields in one call.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "Task reference, e.g. 'Email boss' or 'Email boss:1.a'",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "New name for the task (50 chars max)",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "New status: active, deferred, completed, reopened, cancelled, archived, or 'wait' (requires blockers). Archived cascades to children.",
				},
				"blockers": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Blocker task refs when status is 'wait', e.g. ['Email boss:1', 'Other task']. Replaces existing blockers.",
				},
				"desc": map[string]interface{}{
					"type":        "string",
					"description": "Description text",
				},
				"verify": map[string]interface{}{
					"type":        "string",
					"description": "Verification criteria",
				},
				"deadline": map[string]interface{}{
					"type":        "string",
					"description": "Deadline date (YYYY-MM-DD) or datetime (YYYY-MM-DD HH:MM). Use 'none' to clear.",
				},
				"recur": map[string]interface{}{
					"type":        "string",
					"description": "Recurrence pattern: 'daily', 'daily 09:00', 'weekly mon,thu', 'monthly 1,15', 'every 2d', 'every 4h', 'every 2mon', 'every 30min'. Use 'none' to clear.",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tag names to set (replaces existing tags), e.g. ['URGENT','BACKEND']",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, archive conflicting task when renaming to a name that already exists",
				},
			},
			"required": []string{"task"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			taskRef, err := requireString(params, "task")
			if err != nil {
				return "", err
			}
			var opts SetTaskOpts
			if n, ok := params["name"].(string); ok {
				opts.Name = &n
			}
			if f, ok := params["force"].(bool); ok {
				opts.Force = f
			}
			if s, ok := params["status"].(string); ok {
				opts.Status = s
			}
			if b, ok := params["blockers"].([]interface{}); ok {
				blockers, err := toStringSlice(b)
				if err != nil {
					return "", err
				}
				opts.Blockers = blockers
			}
			if d, ok := params["desc"].(string); ok {
				opts.Desc = &d
			}
			if v, ok := params["verify"].(string); ok {
				opts.Verify = &v
			}
			if dl, ok := params["deadline"].(string); ok {
				opts.Deadline = &dl
			}
			if rc, ok := params["recur"].(string); ok {
				opts.Recur = &rc
			}
			if t, ok := params["tags"].([]interface{}); ok {
				opts.SetTags = true
				tags, err := toStringSlice(t)
				if err != nil {
					return "", err
				}
				opts.Tags = tags
			}
			if opts.Name == nil && opts.Status == "" && opts.Desc == nil && opts.Verify == nil && opts.Deadline == nil && opts.Recur == nil && !opts.SetTags && opts.Blockers == nil {
				return "", fmt.Errorf("at least one field to update is required")
			}
			if err := SetTask(store, taskRef, opts); err != nil {
				return "", err
			}
			displayName := taskRef
			if opts.Name != nil {
				displayName = *opts.Name
			}
			return fmt.Sprintf("Task updated: %s", displayName), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "gettask",
		Description: "View tasks. Use 'task' for a specific task, or 'all: true' for all tasks. One of 'task', 'all', 'tag', or 'status' is required.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "Task reference, e.g. 'Email boss' or 'Email boss:1.a'.",
				},
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, return all tasks.",
				},
				"details": map[string]interface{}{
					"type":        "boolean",
					"description": "Include description and verification fields. Default false.",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by status: active, waiting, completed, deferred, not_started, reopened, cancelled, archived. Archived tasks are hidden unless this filter is used.",
				},
				"depth": map[string]interface{}{
					"type":        "integer",
					"description": "Max depth of children to show. 0 = task only, 1 = direct children, etc. Default unlimited.",
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Filter by tag name, e.g. 'URGENT'.",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort order: 'recent' sorts by most recently changed status. Only applies to all-tasks view.",
				},
			},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			opts := GetTaskOpts{Depth: -1}
			if t, ok := params["task"].(string); ok {
				opts.TaskRef = t
			}
			all, _ := params["all"].(bool)
			if t, ok := params["tag"].(string); ok {
				opts.Tag = t
			}
			if s, ok := params["status"].(string); ok {
				opts.Status = s
			}
			if opts.TaskRef == "" && !all && opts.Tag == "" && opts.Status == "" {
				return "", fmt.Errorf("either 'task' or 'all: true' is required")
			}
			if d, ok := params["details"].(bool); ok {
				opts.Details = d
			}
			if d, ok := params["depth"].(float64); ok {
				opts.Depth = int(d)
			}
			if so, ok := params["sort"].(string); ok {
				opts.Sort = so
			}
			names, err := GetTasks(store, opts)
			if err != nil {
				return "", err
			}
			if len(names) == 0 {
				return "No tasks found.", nil
			}
			return strings.Join(names, "\n"), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "deletetask",
		Description: "Permanently delete a task and all its children. Removes from blocker lists. This cannot be undone.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "Task reference, e.g. 'Email boss' or 'Email boss:1.a'",
				},
			},
			"required": []string{"task"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			taskRef, err := requireString(params, "task")
			if err != nil {
				return "", err
			}
			name, childCount, err := DeleteTaskAtomic(store, taskRef)
			if err != nil {
				return "", err
			}
			if childCount > 0 {
				return fmt.Sprintf("Deleted \"%s\" and %d children.", name, childCount), nil
			}
			return fmt.Sprintf("Deleted \"%s\".", name), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "addrule",
		Description: "Add a new rule. Optionally tag on creation.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The rule name",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tags to apply on creation, e.g. ['BACKEND','FRONTEND']",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			name, err := requireString(params, "name")
			if err != nil {
				return "", err
			}
			var tags []string
			if t, ok := params["tags"].([]interface{}); ok {
				tags, err = toStringSlice(t)
				if err != nil {
					return "", err
				}
			}
			seq, err := AddRule(store, name, tags...)
			if err != nil {
				return "", err
			}
			msg := fmt.Sprintf("Rule %d added: %s", seq, name)
			for _, t := range tags {
				msg += " #" + t
			}
			return msg, nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "getrule",
		Description: "Get rules. Use 'seq' for a specific rule, 'all: true' for all, 'tag' to filter by tag, or 'tag_all' to group by tags. One of seq/all/tag/tag_all is required.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"seq": map[string]interface{}{
					"type":        "integer",
					"description": "Rule number to retrieve",
				},
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, return all rules. Required when not specifying seq, tag, or tag_all.",
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Filter by tag name, e.g. 'BACKEND'. Use 'NONE' for untagged rules.",
				},
				"tag_all": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, show all rules grouped by tags",
				},
			},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			var opts GetRuleOpts
			if s, ok := params["seq"].(float64); ok {
				opts.Seq = int(s)
			}
			if t, ok := params["tag"].(string); ok {
				opts.Tag = t
			}
			if ta, ok := params["tag_all"].(bool); ok {
				opts.TagAll = ta
			}
			all, _ := params["all"].(bool)
			if opts.Seq == 0 && !all && opts.Tag == "" && !opts.TagAll {
				return "", fmt.Errorf("one of 'seq', 'all: true', 'tag', or 'tag_all: true' is required")
			}
			names, err := GetRules(store, opts)
			if err != nil {
				return "", err
			}
			if len(names) == 0 {
				return "No rules found.", nil
			}
			return strings.Join(names, "\n"), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "addtag",
		Description: "Add a tag. Must be uppercase alphanumeric, 10 chars max.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			name, err := requireString(params, "name")
			if err != nil {
				return "", err
			}
			if err := AddTag(store, name); err != nil {
				return "", err
			}
			return fmt.Sprintf("Tag added: %s", name), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "gettag",
		Description: "View tags. Use 'tag' for a specific tag, or 'all: true' for all tags. One of 'tag' or 'all' is required.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag name to view details for.",
				},
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, return all tags.",
				},
			},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			var opts GetTagOpts
			if t, ok := params["tag"].(string); ok {
				opts.Tag = t
			}
			all, _ := params["all"].(bool)
			if opts.Tag == "" && !all {
				return "", fmt.Errorf("either 'tag' or 'all: true' is required")
			}
			tags, err := GetTags(store, opts)
			if err != nil {
				return "", err
			}
			if len(tags) == 0 {
				return "No tags found.", nil
			}
			return strings.Join(tags, "\n"), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "settag",
		Description: "Rename a tag. All task and rule associations follow automatically.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"old": map[string]interface{}{
					"type":        "string",
					"description": "Current tag name",
				},
				"new": map[string]interface{}{
					"type":        "string",
					"description": "New tag name (uppercase alphanumeric, 10 chars max)",
				},
			},
			"required": []string{"old", "new"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			oldName, err := requireString(params, "old")
			if err != nil {
				return "", err
			}
			newName, err := requireString(params, "new")
			if err != nil {
				return "", err
			}
			if err := SetTag(store, oldName, newName); err != nil {
				return "", err
			}
			return fmt.Sprintf("Tag renamed: %s → %s", oldName, newName), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "deletetag",
		Description: "Permanently delete a tag. Removes it from all tasks and rules. Tasks with rules linked through this tag will no longer see those rules.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag name to delete",
				},
			},
			"required": []string{"tag"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			tagName, err := requireString(params, "tag")
			if err != nil {
				return "", err
			}
			taskCount, ruleCount, err := DeleteTagAtomic(store, tagName)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Tag \"%s\" deleted. %d tasks and %d rules untagged.", tagName, taskCount, ruleCount), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "setrule",
		Description: "Update a rule by number. Set name and/or tags (replaces existing tags).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"seq": map[string]interface{}{
					"type":        "integer",
					"description": "Rule number to update",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "New rule name",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tags to set (replaces existing), e.g. ['BACKEND','FRONTEND']",
				},
			},
			"required": []string{"seq"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			seq, err := requireInt(params, "seq")
			if err != nil {
				return "", err
			}
			var opts SetRuleOpts
			if n, ok := params["name"].(string); ok {
				opts.Name = &n
			}
			if t, ok := params["tags"].([]interface{}); ok {
				opts.SetTags = true
				tags, err := toStringSlice(t)
				if err != nil {
					return "", err
				}
				opts.Tags = tags
			}
			if opts.Name == nil && !opts.SetTags {
				return "", fmt.Errorf("at least one field to update is required")
			}
			if err := SetRule(store, seq, opts); err != nil {
				return "", err
			}
			if opts.Name != nil {
				return fmt.Sprintf("Rule %d updated: %s", seq, *opts.Name), nil
			}
			return fmt.Sprintf("Rule %d updated", seq), nil
		},
	})

	mcp.AddTool(Tool{
		Name:        "deleterule",
		Description: "Delete a rule by number. The slot is reused by the next addrule call.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"seq": map[string]interface{}{
					"type":        "integer",
					"description": "Rule number to delete",
				},
			},
			"required": []string{"seq"},
		},
		Handler: func(params map[string]interface{}) (string, error) {
			seq, err := requireInt(params, "seq")
			if err != nil {
				return "", err
			}
			if err := DeleteRule(store, seq); err != nil {
				return "", err
			}
			return fmt.Sprintf("Rule %d deleted", seq), nil
		},
	})

}
