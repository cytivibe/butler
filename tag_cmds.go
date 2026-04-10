package main

import (
	"fmt"
	"strings"
)

var addtagCmd = Command{
	Name: "addtag",
	Desc: "Add a tag. Must be uppercase alphanumeric, 10 chars max.",
	Flags: []Flag{
		TagNamePos,
	},
	Run: func(store *Store, p Parsed) (string, error) {
		name := p.Arg("name")
		if err := AddTag(store, name); err != nil {
			return "", err
		}
		return fmt.Sprintf("Tag added: %s", name), nil
	},
}

var gettagCmd = Command{
	Name:      "gettag",
	Desc:      "View tags. Use 'tag' for a specific tag, or 'all: true' for all tags. One of 'tag' or 'all' is required.",
	RawOutput: true,
	Flags: []Flag{
		TagNameOptPos,
		AllFlag,
	},
	Run: func(store *Store, p Parsed) (string, error) {
		tag := p.Arg("tag")
		all := p.Bool("--all")
		if tag == "" && !all {
			return "", fmt.Errorf("either 'tag' or 'all: true' is required")
		}
		opts := GetTagOpts{Tag: tag}
		tags, err := GetTags(store, opts)
		if err != nil {
			return "", err
		}
		if len(tags) == 0 {
			return "No tags found.", nil
		}
		return strings.Join(tags, "\n"), nil
	},
}

var settagCmd = Command{
	Name: "settag",
	Desc: "Rename a tag. All task and rule associations follow automatically.",
	Flags: []Flag{
		OldTagPos,
		NewTagPos,
	},
	Run: func(store *Store, p Parsed) (string, error) {
		oldName := p.Arg("old")
		newName := p.Arg("new")
		if err := SetTag(store, oldName, newName); err != nil {
			return "", err
		}
		return fmt.Sprintf("Tag renamed: %s \u2192 %s", oldName, newName), nil
	},
}

var deletetagCmd = Command{
	Name: "deletetag",
	Desc: "Permanently delete a tag. Removes it from all tasks and rules.",
	Flags: []Flag{
		TagNameReqPos,
		ForceFlag,
	},
	Confirm: func(store *Store, p Parsed) (string, error) {
		tagName := p.Arg("tag")
		taskCount, ruleCount, err := DeleteTagInfo(store, tagName)
		if err != nil {
			return "", err
		}
		msg := fmt.Sprintf("Delete tag \"%s\"?", tagName)
		if taskCount > 0 || ruleCount > 0 {
			msg += fmt.Sprintf(" %d tasks and %d rules will lose this tag.", taskCount, ruleCount)
		}
		msg += " Tasks with rules linked through this tag will no longer see those rules in --details. This is permanent. (y/N): "
		return msg, nil
	},
	Run: func(store *Store, p Parsed) (string, error) {
		tagName := p.Arg("tag")
		taskCount, ruleCount, err := DeleteTagAtomic(store, tagName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Tag \"%s\" deleted. %d tasks and %d rules untagged.", tagName, taskCount, ruleCount), nil
	},
}
