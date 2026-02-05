package graph

// Node represents a symbol in the codebase.
type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	FilePath  string `json:"file_path"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	ColStart  int    `json:"col_start"`
	ColEnd    int    `json:"col_end"`
	SymbolURI string `json:"symbol_uri"`
}

// Edge represents a relationship between two nodes.
type Edge struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Relation string `json:"relation"` // calls, implements, references, imports
}

const (
	RelationCalls      = "calls"
	RelationImplements = "implements"
	RelationReferences = "references"
	RelationImports    = "imports"
)
