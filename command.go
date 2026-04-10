package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Flag defines a command-line flag or positional argument.
// Named flags have CLI set (e.g. "--status"). Positional args have Pos > 0.
type Flag struct {
	CLI      string              // primary form: "--status", empty for positional
	Short    []string            // accepted aliases: ["-status", "-s"]
	MCP      string              // MCP parameter name: "status"
	Bool     bool                // true = no value consumed (--force)
	Multi    bool                // true = consumes 1+ values until next flag (--tag A B)
	Required bool                // parser errors if missing
	Pos      int                 // 0 = named flag, 1+ = positional arg order
	Desc     string              // description for help text and MCP schema
	Hint     string              // shown on validation error
	MCPType  string              // override MCP schema type: "integer", "string", etc. Auto-derived if empty.
	Validate func([]string) error // custom validation, nil = skip
}

// Command defines a CLI command with its flags, positional args, and handler.
// A single Command definition drives CLI parsing, MCP validation, and schema generation.
type Command struct {
	Name      string                               // command name: "addtask"
	Short     string                               // shorthand alias: "at" (future use)
	Desc      string                               // description for help text and MCP tool description
	Flags     []Flag                               // all flags and positional args for this command
	Run       func(*Store, Parsed) (string, error) // single handler for both CLI and MCP
	Confirm   func(*Store, Parsed) (string, error) // CLI-only: returns prompt message, nil = no confirmation
	RawOutput bool                                 // true = print without colorSuccess (for get commands)
	Reject    map[string]string                    // rejected flag/param names -> custom error message
}

// flagResult holds the parsed value(s) for a single flag.
type flagResult struct {
	present bool
	value   string
	values  []string
}

// Parsed holds the result of parsing CLI args or MCP params against a Command.
// Produced by ParseCLI and ParseMCP. The Run handler uses this without knowing the input source.
type Parsed struct {
	args  map[string]flagResult // positional args keyed by Flag.MCP name
	flags map[string]flagResult // named flags keyed by Flag.CLI name
}

// Arg returns a positional arg value by its MCP name. Empty string if not provided.
func (p Parsed) Arg(name string) string {
	return p.args[name].value
}

// Bool returns whether a bool flag was present.
func (p Parsed) Bool(flag string) bool {
	return p.flags[flag].present
}

// String returns a single-value flag's value. Empty string if not provided.
func (p Parsed) String(flag string) string {
	return p.flags[flag].value
}

// Strings returns a multi-value flag's values. nil if not provided.
func (p Parsed) Strings(flag string) []string {
	return p.flags[flag].values
}

// Has returns whether a flag was provided at all.
func (p Parsed) Has(flag string) bool {
	return p.flags[flag].present
}

// ParseCLI parses CLI arguments against the command's flag definitions.
// Returns a Parsed struct or an error if validation fails.
func (c Command) ParseCLI(args []string) (Parsed, error) {
	p := Parsed{
		args:  make(map[string]flagResult),
		flags: make(map[string]flagResult),
	}

	// Build lookup: all accepted flag forms -> index into c.Flags
	lookup := make(map[string]int)
	for i, f := range c.Flags {
		if f.Pos > 0 {
			continue // positional, no CLI/Short to register
		}
		if f.CLI != "" {
			lookup[f.CLI] = i
		}
		for _, s := range f.Short {
			lookup[s] = i
		}
	}

	// Collect positional flag defs sorted by Pos
	var positionals []int // indices into c.Flags
	for i, f := range c.Flags {
		if f.Pos > 0 {
			positionals = append(positionals, i)
		}
	}
	sortPositionals(positionals, c.Flags)
	posNext := 0 // next positional to fill

	// Track which flags have been set (by index) to detect duplicates
	seen := make(map[int]bool)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if strings.HasPrefix(arg, "-") {
			// It's a flag-like argument
			idx, ok := lookup[arg]
			if !ok {
				// Check Reject map for domain-specific errors
				if msg, rejected := c.Reject[arg]; rejected {
					return p, fmt.Errorf("%s", msg)
				}
				return p, flagError(arg, lookup, c.Flags)
			}
			f := c.Flags[idx]

			// Duplicate check
			if seen[idx] {
				return p, fmt.Errorf("%s provided more than once", f.CLI)
			}
			seen[idx] = true

			if f.Bool {
				p.flags[f.CLI] = flagResult{present: true}
			} else if f.Multi {
				// Consume 1+ values until next flag
				i++
				if i >= len(args) || strings.HasPrefix(args[i], "-") {
					hint := ""
					if f.Hint != "" {
						hint = " (" + f.Hint + ")"
					}
					return p, fmt.Errorf("%s requires at least one value%s", f.CLI, hint)
				}
				var vals []string
				for i < len(args) && !strings.HasPrefix(args[i], "-") {
					vals = append(vals, args[i])
					i++
				}
				i-- // outer loop will i++

				if f.Validate != nil {
					if err := f.Validate(vals); err != nil {
						return p, err
					}
				}
				p.flags[f.CLI] = flagResult{present: true, values: vals}
			} else {
				// Single value
				i++
				if i >= len(args) {
					hint := ""
					if f.Hint != "" {
						hint = " (" + f.Hint + ")"
					}
					return p, fmt.Errorf("%s requires a value%s", f.CLI, hint)
				}
				val := args[i]
				if strings.HasPrefix(val, "-") {
					return p, fmt.Errorf("%s value %q looks like a flag; missing value?", f.CLI, val)
				}

				if f.Validate != nil {
					if err := f.Validate([]string{val}); err != nil {
						return p, err
					}
				}
				p.flags[f.CLI] = flagResult{present: true, value: val}
			}
		} else {
			// Positional argument
			if posNext >= len(positionals) {
				return p, fmt.Errorf("unexpected argument %q", arg)
			}
			f := c.Flags[positionals[posNext]]

			if f.Validate != nil {
				if err := f.Validate([]string{arg}); err != nil {
					return p, err
				}
			}
			p.args[f.MCP] = flagResult{present: true, value: arg}
			posNext++
		}
	}

	// Check required fields
	for _, f := range c.Flags {
		if !f.Required {
			continue
		}
		if f.Pos > 0 {
			if _, ok := p.args[f.MCP]; !ok {
				hint := ""
				if f.Hint != "" {
					hint = " (" + f.Hint + ")"
				}
				return p, fmt.Errorf("missing required argument: %s%s", f.MCP, hint)
			}
		} else {
			if _, ok := p.flags[f.CLI]; !ok {
				hint := ""
				if f.Hint != "" {
					hint = " (" + f.Hint + ")"
				}
				return p, fmt.Errorf("missing required flag: %s%s", f.CLI, hint)
			}
		}
	}

	return p, nil
}

// ParseMCP parses MCP parameters against the command's flag definitions.
// Returns a Parsed struct or an error if validation fails.
func (c Command) ParseMCP(params map[string]interface{}) (Parsed, error) {
	p := Parsed{
		args:  make(map[string]flagResult),
		flags: make(map[string]flagResult),
	}

	// Build lookup: MCP param name -> index into c.Flags
	mcpLookup := make(map[string]int)
	for i, f := range c.Flags {
		if f.MCP != "" {
			mcpLookup[f.MCP] = i
		}
	}

	// Reject unknown params
	for key := range params {
		if _, ok := mcpLookup[key]; !ok {
			// Check Reject map for domain-specific errors
			if msg, rejected := c.Reject[key]; rejected {
				return p, fmt.Errorf("%s", msg)
			}
			return p, fmt.Errorf("unknown parameter: %s", key)
		}
	}

	// Extract params
	for key, val := range params {
		f := c.Flags[mcpLookup[key]]

		if f.Bool {
			b, ok := val.(bool)
			if !ok {
				return p, fmt.Errorf("parameter %q must be a boolean", key)
			}
			if b {
				p.flags[f.CLI] = flagResult{present: true}
			}
		} else if f.Multi {
			// Auto-coerce single string to array for backward compat
			if s, ok := val.(string); ok {
				val = []interface{}{s}
			}
			arr, ok := val.([]interface{})
			if !ok {
				return p, fmt.Errorf("parameter %q must be an array", key)
			}
			vals := make([]string, len(arr))
			for i, elem := range arr {
				s, ok := elem.(string)
				if !ok {
					return p, fmt.Errorf("parameter %q: element %d must be a string", key, i)
				}
				vals[i] = s
			}
			if f.Validate != nil {
				if err := f.Validate(vals); err != nil {
					return p, err
				}
			}
			p.flags[f.CLI] = flagResult{present: true, values: vals}
		} else {
			// Auto-coerce float64 (JSON number) to string, reject non-integer floats
			if n, ok := val.(float64); ok {
				if n != float64(int(n)) {
					return p, fmt.Errorf("parameter %q must be an integer, got %g", key, n)
				}
				val = fmt.Sprintf("%d", int(n))
			}
			s, ok := val.(string)
			if !ok {
				return p, fmt.Errorf("parameter %q must be a string", key)
			}
			if f.Validate != nil {
				if err := f.Validate([]string{s}); err != nil {
					return p, err
				}
			}
			if f.Pos > 0 {
				p.args[f.MCP] = flagResult{present: true, value: s}
			} else {
				p.flags[f.CLI] = flagResult{present: true, value: s}
			}
		}
	}

	// Check required fields
	for _, f := range c.Flags {
		if !f.Required {
			continue
		}
		if f.Pos > 0 {
			if _, ok := p.args[f.MCP]; !ok {
				return p, fmt.Errorf("missing required parameter: %s", f.MCP)
			}
		} else {
			if _, ok := p.flags[f.CLI]; !ok {
				return p, fmt.Errorf("missing required parameter: %s", f.MCP)
			}
		}
	}

	return p, nil
}

// MCPSchema generates an MCP InputSchema from the command's flag definitions.
func (c Command) MCPSchema() map[string]interface{} {
	props := make(map[string]interface{})
	var required []string

	for _, f := range c.Flags {
		if f.MCP == "" {
			continue
		}
		prop := map[string]interface{}{
			"description": f.Desc,
		}
		if f.MCPType != "" {
			prop["type"] = f.MCPType
		} else if f.Bool {
			prop["type"] = "boolean"
		} else if f.Multi {
			prop["type"] = "array"
			prop["items"] = map[string]interface{}{"type": "string"}
		} else {
			prop["type"] = "string"
		}
		props[f.MCP] = prop

		if f.Required {
			required = append(required, f.MCP)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// flagError returns an error for an unknown flag, with a case-insensitive suggestion if possible.
func flagError(arg string, lookup map[string]int, flags []Flag) error {
	lower := strings.ToLower(arg)

	// Check CLI and Short fields of all flags for case-insensitive match
	for _, f := range flags {
		if f.Pos > 0 {
			continue
		}
		if strings.ToLower(f.CLI) == lower {
			return fmt.Errorf("unknown flag %s, did you mean %s?", arg, f.CLI)
		}
		for _, s := range f.Short {
			if strings.ToLower(s) == lower {
				return fmt.Errorf("unknown flag %s, did you mean %s?", arg, s)
			}
		}
	}

	return fmt.Errorf("unknown flag: %s", arg)
}

// RunCLI executes a command from CLI args with confirmation and output formatting.
// It handles the full CLI lifecycle: parse, confirm (if needed and --force absent), run, print.
// Returns true if the command was found and handled.
func RunCLI(commands []*Command, name string, args []string, store *Store) bool {
	for _, cmd := range commands {
		if cmd.Name != name {
			continue
		}
		p, err := cmd.ParseCLI(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if cmd.Confirm != nil && !p.Bool("--force") {
			msg, err := cmd.Confirm(store, p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
				os.Exit(1)
			}
			fmt.Print(msg)
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				os.Exit(0)
			}
		}
		result, err := cmd.Run(store, p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", colorError("Error: "+err.Error()))
			os.Exit(1)
		}
		if cmd.RawOutput {
			fmt.Println(result)
		} else {
			fmt.Println(colorSuccess(result))
		}
		return true
	}
	return false
}

// sortPositionals sorts positional indices by their Pos value.
func sortPositionals(indices []int, flags []Flag) {
	for i := 1; i < len(indices); i++ {
		for j := i; j > 0 && flags[indices[j]].Pos < flags[indices[j-1]].Pos; j-- {
			indices[j], indices[j-1] = indices[j-1], indices[j]
		}
	}
}
