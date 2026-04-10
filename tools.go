package main

// allCommands is the registry of all commands.
// Used by registerTools for MCP and by main for CLI routing.
var allCommands = []*Command{
	&addtaskCmd, &settaskCmd, &gettaskCmd, &deletetaskCmd,
	&addruleCmd, &getruleCmd, &setruleCmd, &deleteruleCmd,
	&addtagCmd, &gettagCmd, &settagCmd, &deletetagCmd,
}

func registerTools(mcp *MCPServer, store *Store) {
	for _, cmd := range allCommands {
		cmd := cmd // capture for closure
		mcp.AddTool(Tool{
			Name:        cmd.Name,
			Description: cmd.Desc,
			InputSchema: cmd.MCPSchema(),
			Handler: func(params map[string]interface{}) (string, error) {
				p, err := cmd.ParseMCP(params)
				if err != nil {
					return "", err
				}
				return cmd.Run(store, p)
			},
		})
	}
}
