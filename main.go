package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CLICommands lists every command that must have both a CLI case
// and an MCP tool. Tests verify this.
var CLICommands = []string{"addtask", "gettask", "settask", "deletetask", "addrule", "setrule", "getrule", "deleterule", "addtag", "settag", "gettag", "deletetag"}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	// Handle uninstall before opening the DB — we can't delete a locked DB file.
	if os.Args[1] == "uninstall" {
		runUninstall(os.Args[2:])
		return
	}

	// Enable colored output for CLI (not MCP serve mode).
	if os.Args[1] != "serve" {
		cliColor = true
	}

	store, err := OpenStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
		os.Exit(1)
	}
	defer store.Close()

	ActivateRecurring(store)

	switch os.Args[1] {
	case "addtask":
		if len(os.Args) < 3 {
			fmt.Println("Usage: butler addtask \"task name\" [--under \"task:pos\"] [--parallel] [--force] [--tag TAG1 TAG2]")
			os.Exit(1)
		}
		args := os.Args[2:]
		parallel := false
		force := false
		under := ""
		var name string
		var tags []string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--parallel":
				parallel = true
			case "--force":
				force = true
			case "--desc", "--verify", "--deadline", "--recur":
				fmt.Fprintf(os.Stderr, "%s\n", colorError(fmt.Sprintf("Error: %s is a settask flag — create the task first, then use settask to set it", args[i])))
				os.Exit(1)
			case "--under":
				i++
				if i >= len(args) {
					fmt.Println("Error: --under requires a value")
					os.Exit(1)
				}
				under = args[i]
			case "--tag":
				i++
				for i < len(args) && !strings.HasPrefix(args[i], "--") {
					tags = append(tags, args[i])
					i++
				}
				i--
			default:
				name = args[i]
			}
		}
		if name == "" {
			fmt.Println("Usage: butler addtask \"task name\" [--under \"task:pos\"] [--parallel] [--force] [--tag TAG1 TAG2]")
			os.Exit(1)
		}
		if err := AddTask(store, name, under, parallel, force, tags...); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		msg := fmt.Sprintf("Task added: %s", name)
		for _, t := range tags {
			msg += " #" + t
		}
		fmt.Println(colorSuccess(msg))
	case "settask":
		if len(os.Args) < 4 {
			fmt.Println("Usage: butler settask \"task:pos\" [\"new name\"] [--status ...] [--desc ...] [--verify ...] [--deadline ...] [--tag ...] [--force]")
			os.Exit(1)
		}
		taskRef := os.Args[2]
		args := os.Args[3:]
		var opts SetTaskOpts
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--force":
				opts.Force = true
			case "--status":
				i++
				if i >= len(args) {
					fmt.Println("Error: --status requires a value")
					os.Exit(1)
				}
				opts.Status = args[i]
				if opts.Status == "wait" {
					i++
					for i < len(args) && !strings.HasPrefix(args[i], "--") {
						opts.Blockers = append(opts.Blockers, args[i])
						i++
					}
					i-- // loop will i++
				}
			case "--desc":
				i++
				if i >= len(args) {
					fmt.Println("Error: --desc requires a value")
					os.Exit(1)
				}
				desc := args[i]
				opts.Desc = &desc
			case "--verify":
				i++
				if i >= len(args) {
					fmt.Println("Error: --verify requires a value")
					os.Exit(1)
				}
				verify := args[i]
				opts.Verify = &verify
			case "--deadline":
				i++
				if i >= len(args) {
					fmt.Println("Error: --deadline requires a value")
					os.Exit(1)
				}
				// Support "YYYY-MM-DD HH:MM" as two args
				dl := args[i]
				if dl != "none" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					if len(args[i+1]) == 5 && args[i+1][2] == ':' {
						i++
						dl = dl + " " + args[i]
					}
				}
				opts.Deadline = &dl
			case "--recur":
				i++
				if i >= len(args) {
					fmt.Println("Error: --recur requires a value")
					os.Exit(1)
				}
				rc := args[i]
				for i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					i++
					rc = rc + " " + args[i]
				}
				opts.Recur = &rc
			case "--tag":
				opts.SetTags = true
				i++
				for i < len(args) && !strings.HasPrefix(args[i], "--") {
					opts.Tags = append(opts.Tags, args[i])
					i++
				}
				i-- // loop will i++
			default:
				name := args[i]
				opts.Name = &name
			}
		}
		if err := SetTask(store, taskRef, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		displayName := taskRef
		if opts.Name != nil {
			displayName = *opts.Name
		}
		fmt.Println(colorSuccess(fmt.Sprintf("Task updated: %s", displayName)))
	case "gettask":
		args := os.Args[2:]
		all := false
		opts := GetTaskOpts{Depth: -1}
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--details":
				opts.Details = true
			case "--all":
				all = true
			case "--status":
				i++
				if i >= len(args) {
					fmt.Println("Error: --status requires a value")
					os.Exit(1)
				}
				opts.Status = args[i]
			case "--depth":
				i++
				if i >= len(args) {
					fmt.Println("Error: --depth requires a value")
					os.Exit(1)
				}
				d, err := strconv.Atoi(args[i])
				if err != nil || d < 0 {
					fmt.Println("Error: --depth must be a non-negative integer")
					os.Exit(1)
				}
				opts.Depth = d
			case "--tag":
				i++
				if i >= len(args) {
					fmt.Println("Error: --tag requires a value")
					os.Exit(1)
				}
				opts.Tag = args[i]
			case "--sort":
				i++
				if i >= len(args) {
					fmt.Println("Error: --sort requires a value")
					os.Exit(1)
				}
				opts.Sort = args[i]
			default:
				opts.TaskRef = args[i]
			}
		}
		if !all && opts.TaskRef == "" && opts.Tag == "" && opts.Status == "" {
			fmt.Println("Usage: butler gettask --all [--status STATUS] [--tag TAG] [--depth N] [--sort recent] [--details]")
			fmt.Println("       butler gettask \"task:pos\" [--status STATUS] [--tag TAG] [--depth N] [--details]")
			os.Exit(1)
		}
		names, err := GetTasks(store, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if len(names) == 0 {
			fmt.Println("No tasks found.")
		} else {
			for _, n := range names {
				fmt.Println(n)
			}
		}
	case "deletetask":
		if len(os.Args) < 3 {
			fmt.Println("Usage: butler deletetask \"task:pos\" [--force]")
			os.Exit(1)
		}
		args := os.Args[2:]
		var taskRef string
		force := false
		for _, arg := range args {
			if arg == "--force" {
				force = true
			} else {
				taskRef = arg
			}
		}
		if taskRef == "" {
			fmt.Println("Usage: butler deletetask \"task:pos\" [--force]")
			os.Exit(1)
		}
		if !force {
			name, childCount, err := DeleteTask(store, taskRef)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
				os.Exit(1)
			}
			if childCount > 0 {
				fmt.Printf("Delete \"%s\" and %d children? This is permanent. (y/N): ", name, childCount)
			} else {
				fmt.Printf("Delete \"%s\"? This is permanent. (y/N): ", name)
			}
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				os.Exit(0)
			}
		}
		name, childCount, err := DeleteTaskAtomic(store, taskRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if childCount > 0 {
			fmt.Println(colorSuccess(fmt.Sprintf("Deleted \"%s\" and %d children.", name, childCount)))
		} else {
			fmt.Println(colorSuccess(fmt.Sprintf("Deleted \"%s\".", name)))
		}
	case "addrule":
		if len(os.Args) < 3 {
			fmt.Println("Usage: butler addrule \"rule name\" [--tag TAG1 TAG2]")
			os.Exit(1)
		}
		args := os.Args[2:]
		var name string
		var tags []string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--tag":
				i++
				for i < len(args) && !strings.HasPrefix(args[i], "--") {
					tags = append(tags, args[i])
					i++
				}
				i--
			default:
				name = args[i]
			}
		}
		if name == "" {
			fmt.Println("Usage: butler addrule \"rule name\" [--tag TAG1 TAG2]")
			os.Exit(1)
		}
		seq, err := AddRule(store, name, tags...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		msg := fmt.Sprintf("Rule %d added: %s", seq, name)
		for _, t := range tags {
			msg += " #" + t
		}
		fmt.Println(colorSuccess(msg))
	case "getrule":
		args := os.Args[2:]
		var opts GetRuleOpts
		all := false
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--all":
				all = true
			case "--tag":
				i++
				if i >= len(args) {
					fmt.Println("Error: --tag requires a value or use --tag --all")
					os.Exit(1)
				}
				if args[i] == "--all" {
					opts.TagAll = true
				} else {
					opts.Tag = args[i]
				}
			default:
				n, err := strconv.Atoi(args[i])
				if err != nil || n <= 0 {
					fmt.Printf("Error: invalid argument '%s'\n", args[i])
					os.Exit(1)
				}
				opts.Seq = n
			}
		}
		if opts.Seq == 0 && !all && opts.Tag == "" && !opts.TagAll {
			fmt.Println("Usage: butler getrule --all | butler getrule N | butler getrule --tag TAG | butler getrule --tag --all")
			os.Exit(1)
		}
		names, err := GetRules(store, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if len(names) == 0 {
			fmt.Println("No rules found.")
		} else {
			for _, n := range names {
				fmt.Println(n)
			}
		}
	case "deleterule":
		if len(os.Args) < 3 {
			fmt.Println("Usage: butler deleterule N [--force]")
			os.Exit(1)
		}
		args := os.Args[2:]
		force := false
		var seqStr string
		for _, arg := range args {
			if arg == "--force" {
				force = true
			} else {
				seqStr = arg
			}
		}
		seq, err := strconv.Atoi(seqStr)
		if err != nil || seq <= 0 {
			fmt.Printf("Error: invalid rule number '%s'\n", seqStr)
			os.Exit(1)
		}
		if !force {
			rules, err := GetRules(store, GetRuleOpts{Seq: seq})
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
				os.Exit(1)
			}
			fmt.Printf("Delete \"%s\"? This is permanent. (y/N): ", rules[0])
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				os.Exit(0)
			}
		}
		name, err := DeleteRuleAtomic(store, seq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		_ = name
		fmt.Println(colorSuccess(fmt.Sprintf("Rule %d deleted", seq)))
	case "addtag":
		if len(os.Args) < 3 {
			fmt.Println("Usage: butler addtag \"TAG\"")
			os.Exit(1)
		}
		if err := AddTag(store, os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		fmt.Println(colorSuccess(fmt.Sprintf("Tag added: %s", os.Args[2])))
	case "gettag":
		args := os.Args[2:]
		all := false
		var tagName string
		for _, arg := range args {
			if arg == "--all" {
				all = true
			} else {
				tagName = arg
			}
		}
		if !all && tagName == "" {
			fmt.Println("Usage: butler gettag --all")
			fmt.Println("       butler gettag TAG")
			os.Exit(1)
		}
		opts := GetTagOpts{Tag: tagName}
		tags, err := GetTags(store, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if len(tags) == 0 {
			fmt.Println("No tags found.")
		} else {
			for _, t := range tags {
				fmt.Println(t)
			}
		}
	case "settag":
		if len(os.Args) < 4 {
			fmt.Println("Usage: butler settag \"OLDTAG\" \"NEWTAG\"")
			os.Exit(1)
		}
		if err := SetTag(store, os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		fmt.Println(colorSuccess(fmt.Sprintf("Tag renamed: %s → %s", os.Args[2], os.Args[3])))
	case "deletetag":
		if len(os.Args) < 3 {
			fmt.Println("Usage: butler deletetag TAG [--force]")
			os.Exit(1)
		}
		args := os.Args[2:]
		force := false
		var tagName string
		for _, arg := range args {
			if arg == "--force" {
				force = true
			} else {
				tagName = arg
			}
		}
		if tagName == "" {
			fmt.Println("Usage: butler deletetag TAG [--force]")
			os.Exit(1)
		}
		if !force {
			taskCount, ruleCount, err := DeleteTagInfo(store, tagName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
				os.Exit(1)
			}
			fmt.Printf("Delete tag \"%s\"?", tagName)
			if taskCount > 0 || ruleCount > 0 {
				fmt.Printf(" %d tasks and %d rules will lose this tag.", taskCount, ruleCount)
			}
			fmt.Print(" Tasks with rules linked through this tag will no longer see those rules in --details. This is permanent. (y/N): ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				os.Exit(0)
			}
		}
		taskCount, ruleCount, err := DeleteTagAtomic(store, tagName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		fmt.Println(colorSuccess(fmt.Sprintf("Tag \"%s\" deleted. %d tasks and %d rules untagged.", tagName, taskCount, ruleCount)))
	case "setrule":
		if len(os.Args) < 4 {
			fmt.Println("Usage: butler setrule N [\"new name\"] [--tag TAG1 TAG2]")
			os.Exit(1)
		}
		seq, err := strconv.Atoi(os.Args[2])
		if err != nil || seq <= 0 {
			fmt.Printf("Error: invalid rule number '%s'\n", os.Args[2])
			os.Exit(1)
		}
		args := os.Args[3:]
		var opts SetRuleOpts
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--tag":
				opts.SetTags = true
				i++
				for i < len(args) && !strings.HasPrefix(args[i], "--") {
					opts.Tags = append(opts.Tags, args[i])
					i++
				}
				i--
			default:
				name := args[i]
				opts.Name = &name
			}
		}
		if err := SetRule(store, seq, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if opts.Name != nil {
			fmt.Println(colorSuccess(fmt.Sprintf("Rule %d updated: %s", seq, *opts.Name)))
		} else {
			fmt.Println(colorSuccess(fmt.Sprintf("Rule %d updated", seq)))
		}
	case "export":
		args := os.Args[2:]
		var filePath string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--file":
				i++
				if i >= len(args) {
					fmt.Println("Error: --file requires a path")
					os.Exit(1)
				}
				filePath = args[i]
			default:
				filePath = args[i]
			}
		}
		jsonBytes, err := ExportJSON(store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if filePath == "" {
			fmt.Println(string(jsonBytes))
		} else {
			if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
				os.Exit(1)
			}
			data, _ := Export(store)
			fmt.Println(colorSuccess(fmt.Sprintf("Exported %s to %s", ExportSummary(data), filePath)))
		}
	case "import":
		if len(os.Args) < 3 {
			fmt.Println("Usage: butler import <file.json> [--replace]")
			os.Exit(1)
		}
		args := os.Args[2:]
		var filePath string
		replace := false
		for _, arg := range args {
			switch arg {
			case "--replace":
				replace = true
			default:
				filePath = arg
			}
		}
		if filePath == "" {
			fmt.Println("Usage: butler import <file.json> [--replace]")
			os.Exit(1)
		}
		jsonBytes, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if err := ImportJSONWithBlockers(store, jsonBytes, replace); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		mode := "merged"
		if replace {
			mode = "replaced all data with"
		}
		// Count what was imported
		var data ExportData
		json.Unmarshal(jsonBytes, &data)
		fmt.Println(colorSuccess(fmt.Sprintf("Imported %s from %s (%s)", ExportSummary(&data), filePath, mode)))
	case "serve":
		mcp := NewMCPServer("butler", "1.0.0")
		registerTools(mcp, store)
		if err := mcp.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
	case "--help", "-h", "help":
		if len(os.Args) >= 3 {
			printCommandHelp(os.Args[2])
		} else {
			printHelp()
		}
	default:
		fmt.Printf("Unknown command: %s\nRun 'butler --help' for a list of commands.\n", os.Args[1])
		os.Exit(1)
	}
}
