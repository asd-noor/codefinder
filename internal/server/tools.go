package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"codemap/internal/graph"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Arguments structs

type IndexArgs struct {
	Force bool `json:"force" jsonschema:"description:Force a full re-index even if no changes are detected"`
}

type IndexStatusArgs struct{}

type GetSymbolsInFileArgs struct {
	FilePath string `json:"file_path" jsonschema:"required,description:The absolute path to the file to analyze"`
}

type FindImpactArgs struct {
	SymbolName string `json:"symbol_name" jsonschema:"required,description:The name of the symbol to analyze for impact"`
}

type GetSymbolArgs struct {
	SymbolName string `json:"symbol_name" jsonschema:"required,description:The name of the symbol to locate"`
	WithSource bool   `json:"with_source" jsonschema:"description:If true, includes the source code of the symbol in the response"`
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "index",
		Description: "Scans the workspace and updates the code graph",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args IndexArgs) (*mcp.CallToolResult, any, error) {
		cwd, _ := os.Getwd()

		// Check if already indexing
		s.indexMu.RLock()
		currentStatus := s.indexStatus
		s.indexMu.RUnlock()
		
		if currentStatus == IndexStatusInProgress {
			return errorResult("Indexing already in progress"), nil, nil
		}

		// Reset indexReady channel if this is a re-index
		if currentStatus == IndexStatusReady || currentStatus == IndexStatusFailed {
			s.indexMu.Lock()
			s.indexReady = make(chan struct{})
			s.indexMu.Unlock()
		}

		// Run indexing and track status
		s.setIndexStatus(IndexStatusInProgress, nil)
		startTime := time.Now()

		nodes, err := s.scanner.Scan(ctx, cwd)
		if err != nil {
			s.setIndexStatus(IndexStatusFailed, fmt.Errorf("scan failed: %w", err))
			return errorResult(fmt.Sprintf("Scan failed: %v", err)), nil, nil
		}

		// COLLECT VALID FILES
		validFiles := make(map[string]bool)
		var validFileList []string
		for _, n := range nodes {
			if !validFiles[n.FilePath] {
				validFiles[n.FilePath] = true
				validFileList = append(validFileList, n.FilePath)
			}
		}

		if err := s.store.BulkUpsertNodes(ctx, nodes); err != nil {
			s.setIndexStatus(IndexStatusFailed, fmt.Errorf("failed to store nodes: %w", err))
			return errorResult(fmt.Sprintf("Failed to store nodes: %v", err)), nil, nil
		}

		// PRUNE STALE DATA
		if err := s.store.PruneStaleFiles(ctx, validFileList); err != nil {
			// Log warning but don't fail
			fmt.Fprintf(os.Stderr, "Warning: Failed to prune stale files: %v\n", err)
		}

		edges, err := s.lsp.Enrich(ctx, nodes, s.store)
		if err != nil {
			s.setIndexStatus(IndexStatusFailed, fmt.Errorf("LSP enrichment failed: %w", err))
			return errorResult(fmt.Sprintf("Enrich failed: %v", err)), nil, nil
		}

		if err := s.store.BulkUpsertEdges(ctx, edges); err != nil {
			s.setIndexStatus(IndexStatusFailed, fmt.Errorf("failed to store edges: %w", err))
			return errorResult(fmt.Sprintf("Failed to store edges: %v", err)), nil, nil
		}

		s.setIndexStatus(IndexStatusReady, nil)
		duration := time.Since(startTime)
		msg := fmt.Sprintf("Indexed %d nodes and %d edges in %.2fs", len(nodes), len(edges), duration.Seconds())
		return textResult(msg), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "index_status",
		Description: "Returns the current indexing status of the workspace",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args IndexStatusArgs) (*mcp.CallToolResult, any, error) {
		status, err, duration := s.GetIndexStatus()

		result := map[string]any{
			"status": string(status),
		}

		if duration > 0 {
			result["duration_seconds"] = duration.Seconds()
		}

		if err != nil {
			result["error"] = err.Error()
		}

		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		return textResult(string(jsonBytes)), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_symbols_in_file",
		Description: "Returns the structure of a file",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetSymbolsInFileArgs) (*mcp.CallToolResult, any, error) {
		// Wait for initial indexing with timeout
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := s.WaitForIndex(waitCtx); err != nil {
			status, indexErr, _ := s.GetIndexStatus()
			if indexErr != nil {
				return errorResult(fmt.Sprintf("Indexing failed: %v", indexErr)), nil, nil
			}
			if status == IndexStatusInProgress {
				return errorResult("Indexing in progress, please try again"), nil, nil
			}
			return errorResult(fmt.Sprintf("Indexing wait failed: %v", err)), nil, nil
		}

		nodes, err := s.store.GetSymbolsInFile(ctx, args.FilePath)
		if err != nil {
			return errorResult(fmt.Sprintf("Query failed: %v", err)), nil, nil
		}

		type SimpleNode struct {
			Name  string `json:"name"`
			Kind  string `json:"kind"`
			Range string `json:"range"`
		}
		var simple []SimpleNode
		for _, n := range nodes {
			simple = append(simple, SimpleNode{
				Name:  n.Name,
				Kind:  n.Kind,
				Range: fmt.Sprintf("%d:%d-%d:%d", n.LineStart, n.ColStart, n.LineEnd, n.ColEnd),
			})
		}

		jsonBytes, _ := json.MarshalIndent(simple, "", "  ")
		return textResult(string(jsonBytes)), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "find_impact",
		Description: "Finds downstream dependents of a symbol",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args FindImpactArgs) (*mcp.CallToolResult, any, error) {
		// Wait for initial indexing with timeout
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := s.WaitForIndex(waitCtx); err != nil {
			status, indexErr, _ := s.GetIndexStatus()
			if indexErr != nil {
				return errorResult(fmt.Sprintf("Indexing failed: %v", indexErr)), nil, nil
			}
			if status == IndexStatusInProgress {
				return errorResult("Indexing in progress, please try again"), nil, nil
			}
			return errorResult(fmt.Sprintf("Indexing wait failed: %v", err)), nil, nil
		}

		nodes, err := s.store.FindImpact(ctx, args.SymbolName)
		if err != nil {
			return errorResult(fmt.Sprintf("Query failed: %v", err)), nil, nil
		}

		if len(nodes) == 0 {
			return textResult("No impacted symbols found."), nil, nil
		}

		type ImpactNode struct {
			Name     string `json:"name"`
			FilePath string `json:"file_path"`
			Kind     string `json:"kind"`
		}
		var impacted []ImpactNode
		for _, n := range nodes {
			impacted = append(impacted, ImpactNode{
				Name:     n.Name,
				FilePath: n.FilePath,
				Kind:     n.Kind,
			})
		}

		jsonBytes, _ := json.MarshalIndent(impacted, "", "  ")
		return textResult(string(jsonBytes)), nil, nil
	})

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_symbol",
		Description: "Finds the location and optionally the source code of a symbol",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetSymbolArgs) (*mcp.CallToolResult, any, error) {
		// Wait for initial indexing with timeout
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := s.WaitForIndex(waitCtx); err != nil {
			status, indexErr, _ := s.GetIndexStatus()
			if indexErr != nil {
				return errorResult(fmt.Sprintf("Indexing failed: %v", indexErr)), nil, nil
			}
			if status == IndexStatusInProgress {
				return errorResult("Indexing in progress, please try again"), nil, nil
			}
			return errorResult(fmt.Sprintf("Indexing wait failed: %v", err)), nil, nil
		}

		nodes, err := s.store.GetSymbolLocation(ctx, args.SymbolName)
		if err != nil {
			return errorResult(fmt.Sprintf("Query failed: %v", err)), nil, nil
		}

		if len(nodes) == 0 {
			return textResult("Symbol not found."), nil, nil
		}

		type SymbolInfo struct {
			graph.Node
			Source string `json:"source,omitempty"`
		}

		var info []SymbolInfo
		for _, n := range nodes {
			si := SymbolInfo{Node: *n}
			if args.WithSource {
				source, err := s.readSource(n.FilePath, n.LineStart, n.LineEnd)
				if err != nil {
					// Log warning but return what we have
					fmt.Fprintf(os.Stderr, "Warning: Failed to read source for %s in %s: %v\n", n.Name, n.FilePath, err)
				} else {
					si.Source = source
				}
			}
			info = append(info, si)
		}

		jsonBytes, _ := json.MarshalIndent(info, "", "  ")
		return textResult(string(jsonBytes)), nil, nil
	})
}

func (s *Server) readSource(filePath string, lineStart, lineEnd int) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var builder strings.Builder
	scanner := bufio.NewScanner(f)
	currentLine := 1
	first := true
	for scanner.Scan() {
		if currentLine >= lineStart && currentLine <= lineEnd {
			if !first {
				builder.WriteByte('\n')
			}
			builder.Write(scanner.Bytes())
			first = false
		}
		if currentLine > lineEnd {
			break
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return builder.String(), nil
}
