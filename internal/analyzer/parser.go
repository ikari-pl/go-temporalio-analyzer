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
	logger           *slog.Logger
	registrationInfo *RegistrationInfo // Populated during ParseDirectory
}

// NewParser creates a new Parser instance.
func NewParser(logger *slog.Logger) Parser {
	return &goParser{
		logger: logger,
	}
}

// ParseDirectory recursively parses all Go files in the given directory.
func (p *goParser) ParseDirectory(ctx context.Context, rootDir string, opts config.AnalysisOptions) ([]NodeMatch, error) {
	// First pass: scan for worker.Register* calls to identify registered activities/workflows
	scanner := NewRegistrationScanner(p.logger)
	regInfo, err := scanner.ScanDirectory(ctx, rootDir, opts)
	if err != nil {
		p.logger.Warn("Failed to scan for registrations", "error", err)
		// Continue without registration info
		regInfo = &RegistrationInfo{
			Activities:      make(map[string]*Registration),
			Workflows:       make(map[string]*Registration),
			RegisteredTypes: make(map[string]string),
		}
	}
	p.registrationInfo = regInfo

	var matches []NodeMatch

	// Create file set for tracking position information
	fset := token.NewFileSet()

	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
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

	// Classification is based on reliable detection methods only:
	// 1. workflow.Context parameter + workflow SDK calls = workflow
	// 2. Registration via worker.RegisterActivity/RegisterWorkflow
	// 3. Usage via ExecuteActivity, SetSignalHandler, etc. (tracked as call targets)
	//
	// We deliberately do NOT use name-based detection (e.g., *Activity, *Workflow suffixes)
	// because it's too flaky and produces false positives.

	funcName := fn.Name.Name
	receiverType := p.extractReceiverTypeName(fn)

	// Check if registered as a workflow
	if p.registrationInfo != nil && p.registrationInfo.IsRegisteredWorkflow(funcName) {
		return "workflow"
	}

	// Check if registered as an activity (direct registration or via struct type)
	if p.registrationInfo != nil && p.registrationInfo.IsRegisteredActivity(funcName, receiverType) {
		return "activity"
	}

	// Check based on first parameter type (workflow.Context indicates a workflow)
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

// extractReceiverTypeName extracts the receiver type name from a method declaration.
// Returns empty string for regular functions.
func (p *goParser) extractReceiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}

	recv := fn.Recv.List[0]
	switch t := recv.Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
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
	// Signal handlers are identified by how they're registered via SetSignalHandler,
	// not by heuristics on the function itself. Detection is handled by the extractor.
	return false
}

// isQueryHandler checks if this function is a query handler.
// Query handlers are detected by finding SetQueryHandler calls in workflows,
// not by heuristics on the handler function itself. The handler function
// doesn't have any distinguishing pattern - it's just a regular function
// passed to SetQueryHandler. Detection is handled by the extractor.
func (p *goParser) isQueryHandler(fn *ast.FuncDecl) bool {
	return false
}

// isUpdateHandler checks if this function is an update handler.
// Update handlers are detected by finding SetUpdateHandler calls in workflows,
// not by heuristics on the handler function itself. Detection is handled
// by the extractor.
func (p *goParser) isUpdateHandler(fn *ast.FuncDecl) bool {
	return false
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

// returnsError checks if the function returns error as its last return value.
// This is a common pattern for activities which should always return errors.
func (p *goParser) returnsError(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return false
	}

	// Check the last return value
	lastResult := fn.Type.Results.List[len(fn.Type.Results.List)-1]
	if ident, ok := lastResult.Type.(*ast.Ident); ok {
		return ident.Name == "error"
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
