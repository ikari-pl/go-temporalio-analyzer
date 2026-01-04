package analyzer

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
)

func TestNewParser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger)
	if p == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestParseDirectory(t *testing.T) {
	// Create a temporary directory with test Go files
	tmpDir := t.TempDir()

	// Create a test workflow file
	// Note: To be classified as a workflow, the function must have workflow.Context
	// AND make workflow SDK calls (we don't use name-based detection)
	workflowContent := `package testpkg

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	// Must have workflow SDK call to be detected as a workflow
	workflow.Sleep(ctx, 0)
	return nil
}

func MyActivity(ctx context.Context) error {
	return nil
}
`
	workflowFile := filepath.Join(tmpDir, "workflow.go")
	if err := os.WriteFile(workflowFile, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a test file
	testContent := `package testpkg

func TestMyWorkflow(t *testing.T) {
}
`
	testFile := filepath.Join(tmpDir, "workflow_test.go")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir:      tmpDir,
		ExcludeDirs:  []string{},
		IncludeTests: false,
	}

	matches, err := p.ParseDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ParseDirectory failed: %v", err)
	}

	// Should find the workflow (by naming convention)
	found := false
	for _, match := range matches {
		if fn, ok := match.Node.(*ast.FuncDecl); ok {
			if fn.Name.Name == "MyWorkflow" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("Expected to find MyWorkflow")
	}
}

func TestParseDirectoryWithExcludes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a vendor directory with a file that should be excluded
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}

	vendorContent := `package vendor

func VendorWorkflow() {}
`
	vendorFile := filepath.Join(vendorDir, "vendor.go")
	if err := os.WriteFile(vendorFile, []byte(vendorContent), 0644); err != nil {
		t.Fatalf("Failed to create vendor file: %v", err)
	}

	// Create a file in the main directory
	mainContent := `package main

import "go.temporal.io/sdk/workflow"

func MainWorkflow(ctx workflow.Context) error {
	return nil
}
`
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to create main file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir:     tmpDir,
		ExcludeDirs: []string{"vendor"},
	}

	matches, err := p.ParseDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ParseDirectory failed: %v", err)
	}

	// Should not find VendorWorkflow
	for _, match := range matches {
		if fn, ok := match.Node.(*ast.FuncDecl); ok {
			if fn.Name.Name == "VendorWorkflow" {
				t.Error("Should not find VendorWorkflow (excluded)")
			}
		}
	}
}

func TestParseDirectoryWithTests(t *testing.T) {
	tmpDir := t.TempDir()

	testContent := `package testpkg

func TestWorkflow() {}
`
	testFile := filepath.Join(tmpDir, "workflow_test.go")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger)

	ctx := context.Background()

	// Without including tests
	optsNoTests := config.AnalysisOptions{
		RootDir:      tmpDir,
		IncludeTests: false,
	}
	matchesNoTests, err := p.ParseDirectory(ctx, tmpDir, optsNoTests)
	if err != nil {
		t.Fatalf("ParseDirectory failed: %v", err)
	}

	// With including tests
	optsWithTests := config.AnalysisOptions{
		RootDir:      tmpDir,
		IncludeTests: true,
	}
	matchesWithTests, err := p.ParseDirectory(ctx, tmpDir, optsWithTests)
	if err != nil {
		t.Fatalf("ParseDirectory failed: %v", err)
	}

	// Should have more matches when including tests (or equal if test file has no temporal functions)
	if len(matchesWithTests) < len(matchesNoTests) {
		t.Errorf("Expected more or equal matches with tests, got %d without and %d with",
			len(matchesNoTests), len(matchesWithTests))
	}
}

func TestParseDirectoryContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package testpkg

func MyWorkflow() {}
`
	file := filepath.Join(tmpDir, "workflow.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := config.AnalysisOptions{
		RootDir: tmpDir,
	}

	_, err := p.ParseDirectory(ctx, tmpDir, opts)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

func TestIsWorkflow(t *testing.T) {
	// Workflows are detected by workflow.Context parameter + workflow SDK calls.
	// Name-based detection is NOT used.
	tests := []struct {
		name     string
		code     string
		funcName string
		want     bool
	}{
		{
			name: "workflow with context and SDK calls",
			code: `package test
import "go.temporal.io/sdk/workflow"
func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(ctx, nil)
	return nil
}`,
			funcName: "MyWorkflow",
			want:     true,
		},
		{
			name: "function with Workflow suffix but no context is NOT a workflow",
			code: `package test
func MyWorkflow() {}`,
			funcName: "MyWorkflow",
			want:     false, // No workflow.Context = not a workflow
		},
		{
			name: "activity by name suffix",
			code: `package test
func MyActivity() {}`,
			funcName: "MyActivity",
			want:     false,
		},
		{
			name: "regular function",
			code: `package test
func helper() {}`,
			funcName: "helper",
			want:     false,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger).(*goParser)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == tt.funcName {
					got := p.IsWorkflow(fn)
					if got != tt.want {
						t.Errorf("IsWorkflow(%s) = %v, want %v", tt.funcName, got, tt.want)
					}
					return
				}
			}
			t.Fatalf("Function %s not found", tt.funcName)
		})
	}
}

func TestIsActivity(t *testing.T) {
	// Activities are NOT detected by name suffix alone anymore.
	// They must be registered via worker.RegisterActivity() or called via ExecuteActivity()
	// This test verifies that name-based detection is NOT used.
	tests := []struct {
		name     string
		code     string
		funcName string
		want     bool
	}{
		{
			name: "function with Activity suffix is NOT classified as activity",
			code: `package test
func MyActivity() {}`,
			funcName: "MyActivity",
			want:     false, // Name-based detection removed
		},
		{
			name: "workflow by name suffix",
			code: `package test
func MyWorkflow() {}`,
			funcName: "MyWorkflow",
			want:     false,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger).(*goParser)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == tt.funcName {
					got := p.IsActivity(fn)
					if got != tt.want {
						t.Errorf("IsActivity(%s) = %v, want %v", tt.funcName, got, tt.want)
					}
					return
				}
			}
			t.Fatalf("Function %s not found", tt.funcName)
		})
	}
}

func TestClassifyFunction(t *testing.T) {
	// Classification is based on reliable detection methods only:
	// - workflow.Context parameter + workflow SDK calls = workflow
	// - SetSignalHandler/SetQueryHandler/SetUpdateHandler calls in body = handlers
	// - Name-based detection is NOT used (too flaky)
	tests := []struct {
		name     string
		code     string
		funcName string
		want     string
	}{
		{
			name: "workflow with context and SDK calls",
			code: `package test
import "go.temporal.io/sdk/workflow"
func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(ctx, nil)
	return nil
}`,
			funcName: "MyWorkflow",
			want:     "workflow",
		},
		{
			name: "workflow context without SDK calls is not classified",
			code: `package test
import "go.temporal.io/sdk/workflow"
func MyWorkflow(ctx workflow.Context) error {
	return nil
}`,
			funcName: "MyWorkflow",
			want:     "", // No SDK calls = not classified as workflow
		},
		{
			name:     "function with Activity suffix but no registration is not classified",
			code:     `package test; func SendEmailActivity() {}`,
			funcName: "SendEmailActivity",
			want:     "", // Name-based detection removed
		},
		{
			name:     "function with Workflow suffix but no context is not classified",
			code:     `package test; func ProcessOrderWorkflow() {}`,
			funcName: "ProcessOrderWorkflow",
			want:     "", // No workflow.Context = not classified
		},
		{
			name:     "regular function",
			code:     `package test; func helper() {}`,
			funcName: "helper",
			want:     "",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger).(*goParser)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == tt.funcName {
					got := p.classifyFunction(fn)
					if got != tt.want {
						t.Errorf("classifyFunction(%s) = %q, want %q", tt.funcName, got, tt.want)
					}
					return
				}
			}
			t.Fatalf("Function %s not found", tt.funcName)
		})
	}
}

func TestClassifyFunctionNilInput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger).(*goParser)

	// Test nil function
	if got := p.classifyFunction(nil); got != "" {
		t.Errorf("classifyFunction(nil) = %q, want %q", got, "")
	}

	// Test function with nil name
	fn := &ast.FuncDecl{Name: nil}
	if got := p.classifyFunction(fn); got != "" {
		t.Errorf("classifyFunction(fn with nil name) = %q, want %q", got, "")
	}
}

func TestApplyFilters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger).(*goParser)

	fset := token.NewFileSet()
	code := `package testpkg
func MyWorkflow() {}
func OtherWorkflow() {}
`
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	var matches []NodeMatch
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			matches = append(matches, NodeMatch{
				Node:     fn,
				FileSet:  fset,
				FilePath: "test.go",
				Package:  "testpkg",
				NodeType: "workflow",
			})
		}
	}

	// Test package filter
	optsPackageFilter := config.AnalysisOptions{
		FilterPackage: "testpkg",
	}
	filtered := p.applyFilters(matches, optsPackageFilter)
	if len(filtered) != 2 {
		t.Errorf("Package filter: got %d matches, want 2", len(filtered))
	}

	// Test package filter that doesn't match
	optsNoMatch := config.AnalysisOptions{
		FilterPackage: "otherpkg",
	}
	filtered = p.applyFilters(matches, optsNoMatch)
	if len(filtered) != 0 {
		t.Errorf("Package filter (no match): got %d matches, want 0", len(filtered))
	}

	// Test name filter
	optsNameFilter := config.AnalysisOptions{
		FilterName: "My.*",
	}
	filtered = p.applyFilters(matches, optsNameFilter)
	if len(filtered) != 1 {
		t.Errorf("Name filter: got %d matches, want 1", len(filtered))
	}

	// Test invalid regex for package filter
	optsInvalidPkgRegex := config.AnalysisOptions{
		FilterPackage: "[invalid",
	}
	filtered = p.applyFilters(matches, optsInvalidPkgRegex)
	if len(filtered) != 0 {
		t.Errorf("Invalid package regex: got %d matches, want 0 (skipped)", len(filtered))
	}

	// Test invalid regex for name filter
	optsInvalidNameRegex := config.AnalysisOptions{
		FilterName: "[invalid",
	}
	filtered = p.applyFilters(matches, optsInvalidNameRegex)
	if len(filtered) != 0 {
		t.Errorf("Invalid name regex: got %d matches, want 0 (skipped)", len(filtered))
	}
}

func TestIsWorkflowContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger).(*goParser)

	// Test workflow.Context
	fset := token.NewFileSet()
	code := `package test
import "go.temporal.io/sdk/workflow"
func f(ctx workflow.Context) {}`
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
				if p.isWorkflowContext(fn.Type.Params.List[0].Type) {
					return // Success
				}
			}
		}
	}
	t.Error("Failed to identify workflow.Context")
}

func TestIsActivityContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger).(*goParser)

	fset := token.NewFileSet()
	code := `package test
import "context"
func f(ctx context.Context) {}`
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
				if p.isActivityContext(fn.Type.Params.List[0].Type) {
					return // Success
				}
			}
		}
	}
	t.Error("Failed to identify context.Context")
}

func TestIsWorkflowCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger).(*goParser)

	workflowCalls := []string{
		"ExecuteActivity", "ExecuteChildWorkflow", "ExecuteLocalActivity",
		"SetSignalHandler", "SetQueryHandler", "SetUpdateHandler",
		"GetSignalChannel", "Sleep", "NewTimer", "GetVersion",
		"SideEffect", "MutableSideEffect", "UpsertSearchAttributes",
		"NewContinueAsNewError", "Go", "GoNamed", "Await", "AwaitWithTimeout",
	}

	for _, callName := range workflowCalls {
		t.Run(callName, func(t *testing.T) {
			code := `package test
import "go.temporal.io/sdk/workflow"
func f() { workflow.` + callName + `() }`
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", code, 0)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			found := false
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if p.isWorkflowCall(call) {
						found = true
						return false
					}
				}
				return true
			})

			if !found {
				t.Errorf("Failed to identify workflow.%s as a workflow call", callName)
			}
		})
	}
}

func TestParseDirectoryNonExistent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir: "/non/existent/path/xyz123abc",
	}

	// The parser may return empty results for non-existent paths
	// This is acceptable behavior - we just verify it doesn't panic
	matches, err := p.ParseDirectory(ctx, "/non/existent/path/xyz123abc", opts)
	// Either an error or empty results is acceptable
	if err == nil && len(matches) > 0 {
		t.Error("Expected error or empty results for non-existent directory")
	}
}

func TestParseDirectoryInvalidGoFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid Go file
	invalidContent := `package test
func broken( {}`
	invalidFile := filepath.Join(tmpDir, "invalid.go")
	if err := os.WriteFile(invalidFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	p := NewParser(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir: tmpDir,
	}

	// Should not error, just log a warning and continue
	matches, err := p.ParseDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ParseDirectory should not fail on invalid file: %v", err)
	}
	// Should return empty matches since the file couldn't be parsed
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches from invalid file, got %d", len(matches))
	}
}

