package main

import (
	"fmt"
	"strconv"
	"strings"
)

var addruleCmd = Command{
	Name: "addrule",
	Desc: "Add a new rule. Optionally tag on creation.",
	Flags: []Flag{
		RuleNamePos,
		TagsFlag,
	},
	Run: func(store *Store, p Parsed) (string, error) {
		name := p.Arg("name")
		tags := p.Strings("--tag")
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
}

var getruleCmd = Command{
	Name:      "getrule",
	Desc:      "Get rules. Use 'seq' for a specific rule, 'all: true' for all, 'tag' to filter by tag, or 'tag_all' to group by tags.",
	RawOutput: true,
	Flags: []Flag{
		RuleSeqOptPos,
		AllFlag,
		TagFilterFlag,
		TagAllFlag,
	},
	Run: func(store *Store, p Parsed) (string, error) {
		var opts GetRuleOpts
		if seqStr := p.Arg("seq"); seqStr != "" {
			n, err := strconv.Atoi(seqStr)
			if err != nil || n <= 0 {
				return "", fmt.Errorf("invalid rule number %q", seqStr)
			}
			opts.Seq = n
		}
		all := p.Bool("--all")
		opts.Tag = p.String("--tag")
		opts.TagAll = p.Bool("--tag-all")

		if opts.Seq == 0 && !all && opts.Tag == "" && !opts.TagAll {
			return "", fmt.Errorf("one of seq, --all, --tag, or --tag-all is required")
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
}

var setruleCmd = Command{
	Name: "setrule",
	Desc: "Update a rule by number. Set name and/or tags (replaces existing tags).",
	Flags: []Flag{
		RuleSeqPos,
		NewNamePos,
		TagsFlag,
	},
	Run: func(store *Store, p Parsed) (string, error) {
		seqStr := p.Arg("seq")
		seq, err := strconv.Atoi(seqStr)
		if err != nil || seq <= 0 {
			return "", fmt.Errorf("invalid rule number %q", seqStr)
		}
		var opts SetRuleOpts
		if n := p.Arg("name"); n != "" {
			opts.Name = &n
		}
		if p.Has("--tag") {
			opts.SetTags = true
			opts.Tags = p.Strings("--tag")
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
}

var deleteruleCmd = Command{
	Name: "deleterule",
	Desc: "Delete a rule by number. The slot is reused by the next addrule call.",
	Flags: []Flag{
		RuleSeqPos,
		ForceFlag,
	},
	Confirm: func(store *Store, p Parsed) (string, error) {
		seqStr := p.Arg("seq")
		seq, err := strconv.Atoi(seqStr)
		if err != nil || seq <= 0 {
			return "", fmt.Errorf("invalid rule number %q", seqStr)
		}
		rules, err := GetRules(store, GetRuleOpts{Seq: seq})
		if err != nil {
			return "", err
		}
		if len(rules) == 0 {
			return "", fmt.Errorf("rule %d not found", seq)
		}
		return fmt.Sprintf("Delete \"%s\"? This is permanent. (y/N): ", rules[0]), nil
	},
	Run: func(store *Store, p Parsed) (string, error) {
		seqStr := p.Arg("seq")
		seq, err := strconv.Atoi(seqStr)
		if err != nil || seq <= 0 {
			return "", fmt.Errorf("invalid rule number %q", seqStr)
		}
		if err := DeleteRule(store, seq); err != nil {
			return "", err
		}
		return fmt.Sprintf("Rule %d deleted", seq), nil
	},
}
