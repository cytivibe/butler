package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// CLICommands lists every command that must have both a CLI case
// and an MCP tool. Tests verify this.
var CLICommands = []string{"addtask", "gettask", "settask", "deletetask", "addrule", "setrule", "getrule", "deleterule", "addtag", "settag", "gettag", "deletetag"}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	// Handle uninstall before opening the DB - we can't delete a locked DB file.
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

	if RunCLI(allCommands, os.Args[1], os.Args[2:], store) {
		return
	}

	switch os.Args[1] {
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
