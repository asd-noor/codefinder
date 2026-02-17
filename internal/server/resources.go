package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "codemap://usage-guidelines",
		Name:        "Usage Guidelines",
		Description: "System prompt and usage guidelines for the CodeMap MCP server",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "codemap://usage-guidelines",
					MIMEType: "text/markdown",
					Text:     s.systemPrompt,
				},
			},
		}, nil
	})

	registerToolSchemaResource[IndexArgs](s, "index")
	registerToolSchemaResource[IndexStatusArgs](s, "index_status")
	registerToolSchemaResource[GetSymbolsInFileArgs](s, "get_symbols_in_file")
	registerToolSchemaResource[FindImpactArgs](s, "find_impact")
	registerToolSchemaResource[GetSymbolArgs](s, "get_symbol")
}

func registerToolSchemaResource[T any](s *Server, name string) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return
	}
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")

	uri := fmt.Sprintf("codemap://schemas/%s", name)
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         uri,
		Name:        fmt.Sprintf("Tool Schema: %s", name),
		Description: fmt.Sprintf("JSON schema for the '%s' tool arguments", name),
		MIMEType:    "application/schema+json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      uri,
					MIMEType: "application/schema+json",
					Text:     string(schemaJSON),
				},
			},
		}, nil
	})
}
