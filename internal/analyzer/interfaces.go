package analyzer

import (
	"context"
	"go/ast"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
)

// Analyzer provides methods for analyzing Temporal.io codebases.
type Analyzer interface {
	// Analyze performs a complete analysis of the given directory and returns a temporal graph.
	Analyze(ctx context.Context, opts config.AnalysisOptions) (*TemporalGraph, error)
}

// Parser handles parsing of Go source files and AST analysis.
type Parser interface {
	// ParseDirectory recursively parses all Go files in the given directory.
	ParseDirectory(ctx context.Context, rootDir string, opts config.AnalysisOptions) ([]NodeMatch, error)

	// IsWorkflow determines if the given function declaration is a Temporal workflow.
	IsWorkflow(fn *ast.FuncDecl) bool

	// IsActivity determines if the given function declaration is a Temporal activity.
	IsActivity(fn *ast.FuncDecl) bool
}

// CallExtractor extracts call relationships from AST nodes.
type CallExtractor interface {
	// ExtractCalls finds all temporal workflow and activity calls within a function.
	ExtractCalls(ctx context.Context, fn *ast.FuncDecl, filePath string) ([]CallSite, error)

	// ExtractParameters extracts parameter information from a function declaration.
	ExtractParameters(fn *ast.FuncDecl) map[string]string
}

// GraphBuilder constructs temporal graphs from parsed nodes.
type GraphBuilder interface {
	// BuildGraph creates a temporal graph from the given parsed nodes.
	BuildGraph(ctx context.Context, nodes []NodeMatch) (*TemporalGraph, error)

	// CalculateStats computes statistics for the given graph.
	CalculateStats(ctx context.Context, graph *TemporalGraph) error
}

// Repository provides persistence operations for temporal graphs.
type Repository interface {
	// SaveGraph persists a temporal graph to storage.
	SaveGraph(ctx context.Context, graph *TemporalGraph, path string) error

	// LoadGraph loads a temporal graph from storage.
	LoadGraph(ctx context.Context, path string) (*TemporalGraph, error)
}

// Service provides high-level business operations for temporal analysis.
type Service interface {
	// AnalyzeWorkflows performs a complete workflow analysis.
	AnalyzeWorkflows(ctx context.Context, opts config.AnalysisOptions) (*TemporalGraph, error)

	// ValidateGraph checks the graph for common issues or anti-patterns.
	ValidateGraph(ctx context.Context, graph *TemporalGraph) ([]ValidationIssue, error)
}

// ValidationIssue represents a potential problem found in the temporal graph.
type ValidationIssue struct {
	Type       string `json:"type"` // "warning", "error", "info"
	Message    string `json:"message"`
	NodeName   string `json:"node_name,omitempty"`
	Severity   int    `json:"severity"` // 1-10, 10 being most severe
	Suggestion string `json:"suggestion,omitempty"`
}
