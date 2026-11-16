package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// FromToolkit flattens an Agno toolkit into one MCP tool per method, the way
// an agent takes it apart (Python: MCPConfig(tools=[SomeToolkit()])). Narrow
// the published set with the toolkit's IncludeTools/ExcludeTools.
//
// Methods that require confirmation are refused: an MCP call runs the tool
// directly, so the approval step would be silently skipped.
func FromToolkit(tk toolkit.Tool) ([]CustomTool, error) {
	methods := tk.GetMethods()
	names := make([]string, 0, len(methods))
	for n := range methods {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]CustomTool, 0, len(names))
	for _, name := range names {
		m := methods[name]
		if m.RequiresConfirmation {
			return nil, fmt.Errorf("mcp: toolkit method %q requires confirmation, which an MCP call would skip; serve it over REST instead", name)
		}
		schema := m.Schema
		if schema == nil {
			schema = map[string]any{"type": "object"}
		}
		if _, ok := schema["type"]; !ok {
			schema = map[string]any{"type": "object", "properties": schema}
		}
		method := name
		out = append(out, CustomTool{
			Tool: &mcp.Tool{Name: name, Description: m.Description, InputSchema: schema},
			Handler: func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var args json.RawMessage = []byte("{}")
				if req.Params != nil && len(req.Params.Arguments) > 0 {
					args = req.Params.Arguments
				}
				result, err := tk.Execute(method, args)
				if err != nil {
					return errorResult(err), nil
				}
				text, ok := result.(string)
				if !ok {
					b, err := json.Marshal(result)
					if err != nil {
						text = fmt.Sprint(result)
					} else {
						text = string(b)
					}
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
			},
		})
	}
	return out, nil
}
