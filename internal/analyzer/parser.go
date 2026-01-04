package analyzer

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
)

// goParser implements the Parser interface.
type goParser struct {
	logger *slog.Logger
}

// NewParser creates a new Parser instance.
func NewParser(logger *slog.Logger) Parser {
	return &goParser{
		logger: logger,
	}
}

// ParseDirectory recursively parses all Go files in the given directory.
func (p *goParser) ParseDirectory(ctx context.Context, rootDir string, opts config.AnalysisOptions) ([]NodeMatch, error) {
	var matches []NodeMatch

	// Create file set for tracking position information
	fset := token.NewFileSet()

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			p.logger.Warn("Error accessing path", "path", path, "error", err)
			return nil // Continue walking
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			// Skip excluded directories
			for _, excludeDir := range opts.ExcludeDirs {
				if info.Name() == excludeDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip if not a Go file
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files if not included
		if !opts.IncludeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse the file
		fileMatches, err := p.parseFile(ctx, path, fset)
		if err != nil {
			p.logger.Warn("Error parsing file", "path", path, "error", err)
			return nil // Continue with other files
		}

		// Apply filters
		filteredMatches := p.applyFilters(fileMatches, opts)
		matches = append(matches, filteredMatches...)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", rootDir, err)
	}

	p.logger.Info("Parsed directory", "root", rootDir, "matches", len(matches))
	return matches, nil
}

// parseFile parses a single Go file and extracts temporal nodes.
func (p *goParser) parseFile(ctx context.Context, filePath string, fset *token.FileSet) ([]NodeMatch, error) {
	// Parse the file
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	var matches []NodeMatch

	// Extract package name
	packageName := node.Name.Name

	// Visit all function declarations
	ast.Inspect(node, func(n ast.Node) bool {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return false
		default:
		}

		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if it's a workflow, activity, or handler
		nodeType := p.classifyFunction(fn)
		if nodeType == "" {
			return true // Not a temporal function
		}

		matches = append(matches, NodeMatch{
			Node:     fn,
			FileSet:  fset,
			FilePath: filePath,
			Package:  packageName,
			NodeType: nodeType,
		})

		return true
	})

	return matches, nil
}

// classifyFunction determines what type of Temporal function this is.
func (p *goParser) classifyFunction(fn *ast.FuncDecl) string {
	if fn == nil || fn.Name == nil {
		return ""
	}

	name := fn.Name.Name

	// Check explicit naming patterns
	switch {
	case strings.HasSuffix(name, "Workflow"):
		return "workflow"
	case strings.HasSuffix(name, "Activity"):
		return "activity"
	case strings.HasSuffix(name, "SignalHandler") || strings.HasSuffix(name, "Signal"):
		return "signal_handler"
	case strings.HasSuffix(name, "QueryHandler") || strings.HasSuffix(name, "Query"):
		return "query_handler"
	case strings.HasSuffix(name, "UpdateHandler") || strings.HasSuffix(name, "Update"):
		return "update_handler"
	}

	// Check based on first parameter type
	if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
		firstParam := fn.Type.Params.List[0]
		if p.isWorkflowContext(firstParam.Type) {
			// Check function body for workflow-specific calls
			if fn.Body != nil {
				if p.hasWorkflowCalls(fn.Body) {
					return "workflow"
				}
			}
		}
		if p.isActivityContext(firstParam.Type) {
			return "activity"
		}
	}

	// Check function body for workflow-specific patterns
	if fn.Body != nil {
		if p.isSignalHandler(fn) {
			return "signal_handler"
		}
		if p.isQueryHandler(fn) {
			return "query_handler"
		}
		if p.isUpdateHandler(fn) {
			return "update_handler"
		}
	}

	return ""
}

// IsWorkflow determines if the given function declaration is a Temporal workflow.
func (p *goParser) IsWorkflow(fn *ast.FuncDecl) bool {
	return p.classifyFunction(fn) == "workflow"
}

// IsActivity determines if the given function declaration is a Temporal activity.
func (p *goParser) IsActivity(fn *ast.FuncDecl) bool {
	return p.classifyFunction(fn) == "activity"
}

// hasWorkflowCalls checks if the function body contains workflow-specific calls.
func (p *goParser) hasWorkflowCalls(body *ast.BlockStmt) bool {
	hasWorkflowCalls := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if p.isWorkflowCall(call) {
				hasWorkflowCalls = true
				return false
			}
		}
		return true
	})
	return hasWorkflowCalls
}

// isSignalHandler checks if this function is a signal handler by looking for
// signal-specific patterns in the function body.
func (p *goParser) isSignalHandler(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	isHandler := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for workflow.GetSignalChannel calls
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if ident.Name == "workflow" && sel.Sel.Name == "GetSignalChannel" {
					isHandler = true
					return false
				}
			}
		}
		return true
	})

	return isHandler
}

// isQueryHandler checks if this function is a query handler by looking for
// query-specific patterns. Query handlers typically have a specific signature
// and don't perform side effects.
func (p *goParser) isQueryHandler(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	// Query handlers must return something (they answer queries)
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return false
	}

	// Check for workflow.SetQueryHandler being called with this function's name
	// or workflow-specific read patterns without execution calls
	hasWorkflowContext := false
	if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
		for _, param := range fn.Type.Params.List {
			if p.isWorkflowContext(param.Type) {
				hasWorkflowContext = true
				break
			}
		}
	}

	// If it has workflow context and returns values but doesn't execute activities,
	// it's likely a query handler
	if hasWorkflowContext {
		hasActivityCall := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "workflow" && 
						(sel.Sel.Name == "ExecuteActivity" || 
						 sel.Sel.Name == "ExecuteChildWorkflow" ||
						 sel.Sel.Name == "ExecuteLocalActivity") {
						hasActivityCall = true
						return false
					}
				}
			}
			return true
		})
		// Query handlers don't execute activities
		if !hasActivityCall {
			return true
		}
	}

	return false
}

// isUpdateHandler checks if this function is an update handler by looking for
// update-specific patterns like workflow.SetUpdateHandler or update validation.
func (p *goParser) isUpdateHandler(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	isHandler := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for workflow update-related calls
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if ident.Name == "workflow" && 
					(sel.Sel.Name == "SetUpdateHandler" ||
					 sel.Sel.Name == "SetUpdateHandlerWithOptions") {
					isHandler = true
					return false
				}
			}
		}
		return true
	})

	return isHandler
}

// applyFilters applies the configured filters to the matches.
func (p *goParser) applyFilters(matches []NodeMatch, opts config.AnalysisOptions) []NodeMatch {
	var filtered []NodeMatch

	for _, match := range matches {
		// Apply package filter
		if opts.FilterPackage != "" {
			matched, err := regexp.MatchString(opts.FilterPackage, match.Package)
			if err != nil {
				p.logger.Warn("Invalid package filter regex", "pattern", opts.FilterPackage, "error", err)
				continue
			}
			if !matched {
				continue
			}
		}

		// Apply name filter
		if opts.FilterName != "" {
			fn := match.Node.(*ast.FuncDecl)
			matched, err := regexp.MatchString(opts.FilterName, fn.Name.Name)
			if err != nil {
				p.logger.Warn("Invalid name filter regex", "pattern", opts.FilterName, "error", err)
				continue
			}
			if !matched {
				continue
			}
		}

		filtered = append(filtered, match)
	}

	return filtered
}

// isWorkflowContext checks if the type expression represents workflow.Context.
func (p *goParser) isWorkflowContext(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == "workflow" && t.Sel.Name == "Context"
		}
	}
	return false
}

// isActivityContext checks if the type expression represents context.Context.
func (p *goParser) isActivityContext(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == "context" && t.Sel.Name == "Context"
		}
	}
	return false
}

// isWorkflowCall checks if the call expression is a workflow-related call.
func (p *goParser) isWorkflowCall(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if ident, ok := fun.X.(*ast.Ident); ok {
			if ident.Name == "workflow" {
				switch fun.Sel.Name {
				case "ExecuteActivity", "ExecuteChildWorkflow", "ExecuteLocalActivity",
					"SetSignalHandler", "SetQueryHandler", "SetUpdateHandler",
					"GetSignalChannel", "Sleep", "NewTimer", "GetVersion",
					"SideEffect", "MutableSideEffect", "UpsertSearchAttributes",
					"NewContinueAsNewError", "Go", "GoNamed", "Await", "AwaitWithTimeout":
					return true
				}
			}
		}
	}
	return false
}
