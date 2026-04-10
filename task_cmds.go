package main

import (
	"fmt"
	"strconv"
	"strings"
)

var addtaskCmd = Command{
	Name: "addtask",
	Desc: "Add a task. Use 'under' for subtasks with 'task:pos' notation (e.g. 'Email boss:1.a'). Use parallel=true for parallel (lettered) tasks.",
	Flags: []Flag{
		TaskNamePos,
		UnderFlag,
		ParallelFlag,
		ForceFlag,
		TagsFlag,
	},
	Reject: map[string]string{
		"--desc":          "--desc is a settask flag - create the task first, then use settask to set it",
		"--verify":        "--verify is a settask flag - create the task first, then use settask to set it",
		"--verify-status": "--verify-status is a settask flag - create the task first, then use settask to set it",
		"--deadline":      "--deadline is a settask flag - create the task first, then use settask to set it",
		"--recur":         "--recur is a settask flag - create the task first, then use settask to set it",
		"desc":            "desc is a settask field - create the task first, then use settask to set it",
		"verify":          "verify is a settask field - create the task first, then use settask to set it",
		"verify_status":   "verify_status is a settask field - create the task first, then use settask to set it",
		"deadline":        "deadline is a settask field - create the task first, then use settask to set it",
		"recur":           "recur is a settask field - create the task first, then use settask to set it",
	},
	Run: func(store *Store, p Parsed) (string, error) {
		name := p.Arg("name")
		under := p.String("--under")
		parallel := p.Bool("--parallel")
		force := p.Bool("--force")
		tags := p.Strings("--tag")
		if err := AddTask(store, name, under, parallel, force, tags...); err != nil {
			return "", err
		}
		msg := fmt.Sprintf("Task added: %s", name)
		if under != "" {
			msg += fmt.Sprintf(" (under %s)", under)
		}
		for _, t := range tags {
			msg += " #" + t
		}
		return msg, nil
	},
}

var gettaskCmd = Command{
	Name:      "gettask",
	Desc:      "View tasks. Use 'task' for a specific task, or 'all: true' for all tasks. One of 'task', 'all', 'tag', 'nottag', or 'status' is required.",
	RawOutput: true,
	Flags: []Flag{
		TaskRefOptPos,
		AllFlag,
		DetailsFlag,
		StatusFlag,
		DepthFlag,
		TagsFilterFlag,
		NotTagFlag,
		SortFlag,
	},
	Run: func(store *Store, p Parsed) (string, error) {
		opts := GetTaskOpts{Depth: -1}
		opts.TaskRef = p.Arg("task")
		all := p.Bool("--all")
		opts.Tags = p.Strings("--tag")
		opts.NotTags = p.Strings("--nottag")
		opts.Status = p.String("--status")
		if opts.TaskRef == "" && !all && len(opts.Tags) == 0 && len(opts.NotTags) == 0 && opts.Status == "" {
			return "", fmt.Errorf("either 'task' or 'all: true' is required")
		}
		opts.Details = p.Bool("--details")
		if p.Has("--depth") {
			d := p.String("--depth")
			n, err := strconv.Atoi(d)
			if err != nil || n < 0 {
				return "", fmt.Errorf("--depth must be a non-negative integer")
			}
			opts.Depth = n
		}
		opts.Sort = p.String("--sort")
		names, err := GetTasks(store, opts)
		if err != nil {
			return "", err
		}
		if len(names) == 0 {
			return "No tasks found.", nil
		}
		return strings.Join(names, "\n"), nil
	},
}

var settaskCmd = Command{
	Name: "settask",
	Desc: "Update a task. Set name, status, description, verification, tags, or blockers. Use 'wait' status with blockers. Combine multiple fields in one call.",
	Flags: []Flag{
		TaskRefPos,
		NewNamePos,
		ForceFlag,
		StatusFlag,
		BlockersFlag,
		DescFlag,
		VerifyFlag,
		VerifyStatusFlag,
		DeadlineFlag,
		RecurFlag,
		TagsFlag,
	},
	Run: func(store *Store, p Parsed) (string, error) {
		taskRef := p.Arg("task")
		var opts SetTaskOpts

		if p.Has("--force") {
			opts.Force = true
		}
		if n := p.Arg("name"); n != "" {
			opts.Name = &n
		}
		if p.Has("--status") {
			opts.Status = p.String("--status")
		}
		if p.Has("--blockers") {
			opts.Blockers = p.Strings("--blockers")
		}
		if p.Has("--desc") {
			d := p.String("--desc")
			opts.Desc = &d
		}
		if p.Has("--verify") {
			v := p.String("--verify")
			opts.Verify = &v
		}
		if p.Has("--verify-status") {
			vs := p.String("--verify-status")
			opts.VerifyStatus = &vs
		}
		if p.Has("--deadline") {
			dl := strings.Join(p.Strings("--deadline"), " ")
			opts.Deadline = &dl
		}
		if p.Has("--recur") {
			rc := strings.Join(p.Strings("--recur"), " ")
			opts.Recur = &rc
		}
		if p.Has("--tag") {
			opts.SetTags = true
			opts.Tags = p.Strings("--tag")
		}

		if opts.Name == nil && opts.Status == "" && opts.Desc == nil && opts.Verify == nil &&
			opts.VerifyStatus == nil && opts.Deadline == nil && opts.Recur == nil &&
			!opts.SetTags && opts.Blockers == nil {
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
}

var deletetaskCmd = Command{
	Name: "deletetask",
	Desc: "Permanently delete a task and all its children. Removes from blocker lists. This cannot be undone.",
	Flags: []Flag{
		TaskRefPos,
		ForceFlag,
	},
	Confirm: func(store *Store, p Parsed) (string, error) {
		taskRef := p.Arg("task")
		name, childCount, err := DeleteTask(store, taskRef)
		if err != nil {
			return "", err
		}
		if childCount > 0 {
			return fmt.Sprintf("Delete \"%s\" and %d children? This is permanent. (y/N): ", name, childCount), nil
		}
		return fmt.Sprintf("Delete \"%s\"? This is permanent. (y/N): ", name), nil
	},
	Run: func(store *Store, p Parsed) (string, error) {
		taskRef := p.Arg("task")
		name, childCount, err := DeleteTaskAtomic(store, taskRef)
		if err != nil {
			return "", err
		}
		if childCount > 0 {
			return fmt.Sprintf("Deleted \"%s\" and %d children.", name, childCount), nil
		}
		return fmt.Sprintf("Deleted \"%s\".", name), nil
	},
}
