package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

// RuntimeParser provides on-demand parsing of Go source files.
type RuntimeParser struct{}

// NewRuntimeParser creates a new runtime parser.
func NewRuntimeParser() *RuntimeParser {
	return &RuntimeParser{}
}

// FindFunction searches for a function by name in the Go module.
// It searches from the given file's directory up to the module root (go.mod).
func (rp *RuntimeParser) FindFunction(name string, searchPath string) *analyzer.TemporalNode {
	var searchDir string
	
	// First try the specific file (fast path)
	if strings.HasSuffix(searchPath, ".go") {
		if node := rp.findFunctionInFile(name, searchPath); node != nil {
			return node
		}
		searchDir = filepath.Dir(searchPath)
	} else {
		searchDir = searchPath
	}

	// Search the same directory (same package) - fast path
	if node := rp.findFunctionInDir(name, searchDir); node != nil {
		return node
	}

	// Find module root and search entire module
	moduleRoot := rp.findModuleRoot(searchDir)
	if moduleRoot != "" && moduleRoot != searchDir {
		return rp.findFunctionInModule(name, moduleRoot, searchDir)
	}

	return nil
}

// findModuleRoot finds the nearest directory containing go.mod.
func (rp *RuntimeParser) findModuleRoot(startDir string) string {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return ""
		}
		dir = parent
	}
}

// findFunctionInModule searches for a function in all Go files under the module root.
func (rp *RuntimeParser) findFunctionInModule(name string, moduleRoot string, skipDir string) *analyzer.TemporalNode {
	var result *analyzer.TemporalNode

	_ = filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		
		// Skip vendor, .git, and other non-source directories
		if d.IsDir() {
			baseName := d.Name()
			if baseName == "vendor" || baseName == ".git" || baseName == "node_modules" || 
			   baseName == "testdata" || strings.HasPrefix(baseName, ".") {
				return filepath.SkipDir
			}
			// Already searched this directory
			if path == skipDir {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Only process Go files (skip tests for speed)
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		
		// Try to find function in this file
		if node := rp.findFunctionInFile(name, path); node != nil {
			result = node
			return filepath.SkipAll // Found it, stop walking
		}
		
		return nil
	})
	
	return result
}

// findFunctionInFile searches for a function in a specific file.
func (rp *RuntimeParser) findFunctionInFile(name string, filePath string) *analyzer.TemporalNode {
	fset := token.NewFileSet()
	
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil
	}

	// Look for the function
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Check function name
		if fn.Name.Name == name {
			return rp.buildNodeFromFunc(fn, file, filePath, fset)
		}

		// Check method name (receiver.Method)
		if fn.Recv != nil && fn.Name.Name == name {
			return rp.buildNodeFromFunc(fn, file, filePath, fset)
		}
	}

	return nil
}

// findFunctionInDir searches for a function in all Go files in a directory.
func (rp *RuntimeParser) findFunctionInDir(name string, dirPath string) *analyzer.TemporalNode {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue // Skip test files
		}

		filePath := filepath.Join(dirPath, entry.Name())
		if node := rp.findFunctionInFile(name, filePath); node != nil {
			return node
		}
	}

	return nil
}

// buildNodeFromFunc creates a TemporalNode from an ast.FuncDecl.
func (rp *RuntimeParser) buildNodeFromFunc(fn *ast.FuncDecl, file *ast.File, filePath string, fset *token.FileSet) *analyzer.TemporalNode {
	pos := fset.Position(fn.Pos())

	// Extract description from doc comments
	var description string
	if fn.Doc != nil {
		description = strings.TrimSpace(fn.Doc.Text())
		// Truncate long descriptions
		if len(description) > 200 {
			description = description[:197] + "..."
		}
	}

	// Determine type based on naming conventions
	nodeType := "function" // Internal functions get "function" type
	name := fn.Name.Name
	if strings.HasSuffix(name, "Workflow") {
		nodeType = "workflow"
	} else if strings.HasSuffix(name, "Activity") {
		nodeType = "activity"
	}

	// Extract parameters
	params := make(map[string]string)
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			paramType := rp.typeToString(field.Type)
			for _, name := range field.Names {
				params[name.Name] = paramType
			}
		}
	}

	// Extract return type
	var returnType string
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		var returns []string
		for _, result := range fn.Type.Results.List {
			returns = append(returns, rp.typeToString(result.Type))
		}
		returnType = strings.Join(returns, ", ")
	}

	// Extract internal calls from this function
	internalCalls := rp.extractInternalCalls(fn, filePath, fset)

	return &analyzer.TemporalNode{
		Name:          fn.Name.Name,
		Type:          nodeType,
		Package:       file.Name.Name,
		FilePath:      filePath,
		LineNumber:    pos.Line,
		Description:   description,
		Parameters:    params,
		ReturnType:    returnType,
		InternalCalls: internalCalls,
		CallSites:     []analyzer.CallSite{},
		Parents:       []string{},
	}
}

// extractInternalCalls extracts function calls from a function body.
func (rp *RuntimeParser) extractInternalCalls(fn *ast.FuncDecl, filePath string, fset *token.FileSet) []analyzer.InternalCall {
	if fn.Body == nil {
		return nil
	}

	var calls []analyzer.InternalCall
	seen := make(map[string]bool)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		lineNum := fset.Position(call.Pos()).Line

		switch fun := call.Fun.(type) {
		case *ast.Ident:
			// Direct function call
			name := fun.Name
			if !rp.isBuiltin(name) && !seen[name] {
				seen[name] = true
				calls = append(calls, analyzer.InternalCall{
					TargetName: name,
					CallType:   "function",
					LineNumber: lineNum,
					FilePath:   filepath.Base(filePath),
				})
			}

		case *ast.SelectorExpr:
			// Method call or package function
			methodName := fun.Sel.Name
			var receiverName string
			if ident, ok := fun.X.(*ast.Ident); ok {
				receiverName = ident.Name
			}

			// Skip boring calls
			if rp.isBoringCall(receiverName, methodName) {
				return true
			}

			fullName := methodName
			if receiverName != "" {
				fullName = receiverName + "." + methodName
			}

			if !seen[fullName] {
				seen[fullName] = true
				calls = append(calls, analyzer.InternalCall{
					TargetName: methodName,
					Receiver:   receiverName,
					CallType:   "method",
					LineNumber: lineNum,
					FilePath:   filepath.Base(filePath),
				})
			}
		}

		return true
	})

	return calls
}

// isBuiltin returns true for Go builtin functions.
func (rp *RuntimeParser) isBuiltin(name string) bool {
	builtins := map[string]bool{
		"append": true, "cap": true, "close": true, "complex": true,
		"copy": true, "delete": true, "imag": true, "len": true,
		"make": true, "new": true, "panic": true, "print": true,
		"println": true, "real": true, "recover": true,
	}
	return builtins[name]
}

// isBoringCall returns true for uninteresting calls.
func (rp *RuntimeParser) isBoringCall(receiver, method string) bool {
	// Error handling
	if method == "Error" || method == "Unwrap" || method == "Is" || method == "As" || method == "Wrap" || method == "Wrapf" {
		return true
	}
	// Context
	if receiver == "ctx" || receiver == "context" {
		return true
	}
	// Standard library packages
	boringReceivers := map[string]bool{
		"strings": true, "strconv": true, "fmt": true, "bytes": true,
		"time": true, "sync": true, "atomic": true, "math": true,
		"sort": true, "json": true, "xml": true, "io": true,
		"os": true, "path": true, "filepath": true, "regexp": true,
		"reflect": true, "runtime": true, "unsafe": true,
	}
	if boringReceivers[receiver] {
		return true
	}
	// Logging
	boringMethods := map[string]bool{
		"Info": true, "Debug": true, "Warn": true, "Error": true,
		"Infof": true, "Debugf": true, "Warnf": true, "Errorf": true,
		"InfoContext": true, "DebugContext": true, "WarnContext": true, "ErrorContext": true,
		"Printf": true, "Println": true, "Print": true, "Sprintf": true,
		"Log": true, "Logf": true,
		"String": true, "Int": true, "Bool": true, "Float64": true, // Common getters
		"Bytes": true, "Len": true, "Cap": true, "Close": true,
	}
	if boringMethods[method] {
		return true
	}
	// Logging receivers
	if receiver == "log" || receiver == "logger" || receiver == "l" || receiver == "slog" {
		return true
	}
	return false
}

// IsLocalFunction checks if a function name is likely defined locally (same package).
func (rp *RuntimeParser) IsLocalFunction(receiver string) bool {
	// If no receiver, it's a direct function call - likely local
	if receiver == "" {
		return true
	}
	// If receiver is a single letter (common for method receivers like p, s, m, etc.)
	// it's likely a method on a local type
	if len(receiver) == 1 {
		return true
	}
	// Common local receiver patterns
	localPatterns := []string{"self", "this", "srv", "svc", "service", "handler", "repo", "store"}
	for _, p := range localPatterns {
		if strings.EqualFold(receiver, p) {
			return true
		}
	}
	return false
}

// typeToString converts an AST type to a string.
func (rp *RuntimeParser) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.StarExpr:
		return "*" + rp.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + rp.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + rp.typeToString(t.Key) + "]" + rp.typeToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func"
	default:
		return "unknown"
	}
}

