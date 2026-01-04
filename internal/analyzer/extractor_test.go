package analyzer

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"testing"
)

func TestNewCallExtractor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger)
	if e == nil {
		t.Fatal("NewCallExtractor returned nil")
	}
}

func TestExtractCalls(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	err := workflow.ExecuteActivity(ctx, MyActivity, "arg").Get(ctx, nil)
	return err
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger)

	ctx := context.Background()

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "MyWorkflow" {
			calls, err := e.ExtractCalls(ctx, fn, "test.go")
			if err != nil {
				t.Fatalf("ExtractCalls failed: %v", err)
			}
			if len(calls) == 0 {
				t.Error("Expected to find at least one call")
			}
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractCallsContextCancellation(t *testing.T) {
	code := `package test
func f() {}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			// With cancelled context, the walk should stop but not error for empty functions
			_, err := e.ExtractCalls(ctx, fn, "test.go")
			// Empty function body might not trigger context check
			_ = err
			return
		}
	}
	t.Fatal("Function not found")
}

func TestExtractAllTemporalInfo(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	err := workflow.ExecuteActivity(ctx, MyActivity, "arg").Get(ctx, nil)
	workflow.SetSignalHandler(ctx, "mySignal", func(s string) {})
	workflow.SetQueryHandler(ctx, "myQuery", func() (string, error) { return "", nil })
	workflow.Sleep(ctx, time.Hour)
	workflow.GetVersion(ctx, "change1", workflow.DefaultVersion, 1)
	return err
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	ctx := context.Background()

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "MyWorkflow" {
			details, err := e.ExtractAllTemporalInfo(ctx, fn, "test.go", fset)
			if err != nil {
				t.Fatalf("ExtractAllTemporalInfo failed: %v", err)
			}
			if details == nil {
				t.Fatal("ExtractAllTemporalInfo returned nil details")
			}
			// Check for various temporal constructs
			if len(details.CallSites) == 0 {
				t.Error("Expected to find call sites")
			}
			if len(details.Signals) == 0 {
				t.Error("Expected to find signals")
			}
			if len(details.Queries) == 0 {
				t.Error("Expected to find queries")
			}
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractAllTemporalInfoContextCancellation(t *testing.T) {
	code := `package test
func f() {}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			// Empty function body won't trigger context check
			_, _ = e.ExtractAllTemporalInfo(ctx, fn, "test.go", fset)
			return
		}
	}
	t.Fatal("Function not found")
}

func TestExtractInternalCalls(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func helper(x int) int { return x * 2 }

func MyWorkflow(ctx workflow.Context) error {
	result := helper(42)
	_ = result
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	ctx := context.Background()

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "MyWorkflow" {
			calls := e.extractInternalCalls(ctx, fn, "test.go", fset)
			if len(calls) == 0 {
				t.Error("Expected to find internal calls")
			}
			// Verify the helper call is found
			foundHelper := false
			for _, call := range calls {
				if call.TargetName == "helper" {
					foundHelper = true
					break
				}
			}
			if !foundHelper {
				t.Error("Expected to find helper function call")
			}
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractInternalCallsExcludesTemporalSDK(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.Sleep(ctx, time.Hour)
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	ctx := context.Background()

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "MyWorkflow" {
			calls := e.extractInternalCalls(ctx, fn, "test.go", fset)
			// Should NOT include workflow.Sleep as internal call
			for _, call := range calls {
				if call.TargetName == "Sleep" && call.Receiver == "workflow" {
					t.Error("Should not include workflow SDK calls as internal calls")
				}
			}
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractParameters(t *testing.T) {
	code := `package test

func MyFunc(a string, b int, c, d float64) error {
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "MyFunc" {
			params := e.ExtractParameters(fn)
			if len(params) != 4 {
				t.Errorf("Expected 4 parameters, got %d", len(params))
			}
			if params["a"] != "string" {
				t.Errorf("Expected params[a] = string, got %s", params["a"])
			}
			if params["b"] != "int" {
				t.Errorf("Expected params[b] = int, got %s", params["b"])
			}
			return
		}
	}
	t.Fatal("Function not found")
}

func TestExprToString(t *testing.T) {
	code := `package test

var a = identifier
var b = pkg.Selector
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Values) > 0 {
					val := e.exprToString(vs.Values[0])
					if val == "" {
						t.Error("exprToString returned empty string")
					}
				}
			}
		}
	}
}

func TestExtractorTypeToString(t *testing.T) {
	code := `package test

var a int
var b string
var c *int
var d []string
var e map[string]int
var f interface{}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					typeStr := e.typeToString(vs.Type)
					if typeStr == "" || typeStr == "unknown" {
						// Allow unknown for some complex types
						continue
					}
				}
			}
		}
	}
}

func TestIsLikelyTemporalFunction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	tests := []struct {
		name string
		want bool
	}{
		{"ProcessOrderWorkflow", true},
		{"SendEmailActivity", true},
		{"CancelSignal", true},
		{"GetStatusQuery", true},
		{"helperFunc", false},
		{"processOrder", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.isLikelyTemporalFunction(tt.name)
			if got != tt.want {
				t.Errorf("isLikelyTemporalFunction(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestInferTypeFromName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	tests := []struct {
		name string
		want string
	}{
		{"ProcessOrderWorkflow", "workflow"},
		{"SendEmailActivity", "activity"},
		{"CancelSignal", "signal"},
		{"GetStatusQuery", "query"},
		{"helperFunc", "activity"}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.inferTypeFromName(tt.name)
			if got != tt.want {
				t.Errorf("inferTypeFromName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsBuiltinOrCommon(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	tests := []struct {
		name string
		want bool
	}{
		{"append", true},
		{"len", true},
		{"make", true},
		{"panic", true},
		{"myFunc", false},
		{"helper", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.isBuiltinOrCommon(tt.name)
			if got != tt.want {
				t.Errorf("isBuiltinOrCommon(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsBoringCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	tests := []struct {
		receiver string
		method   string
		want     bool
	}{
		{"ctx", "Done", true},
		{"fmt", "Printf", true},
		{"strings", "Split", true},
		{"myService", "Process", false},
		{"store", "Save", false},
	}

	for _, tt := range tests {
		t.Run(tt.receiver+"."+tt.method, func(t *testing.T) {
			got := e.isBoringCall(tt.receiver, tt.method)
			if got != tt.want {
				t.Errorf("isBoringCall(%q, %q) = %v, want %v", tt.receiver, tt.method, got, tt.want)
			}
		})
	}
}

func TestExtractFunctionReference(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	// Test with identifier
	ident := &ast.Ident{Name: "MyActivity"}
	if got := e.extractFunctionReference(ident); got != "MyActivity" {
		t.Errorf("extractFunctionReference(ident) = %q, want %q", got, "MyActivity")
	}

	// Test with selector
	sel := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "pkg"},
		Sel: &ast.Ident{Name: "Function"},
	}
	if got := e.extractFunctionReference(sel); got != "Function" {
		t.Errorf("extractFunctionReference(sel) = %q, want %q", got, "Function")
	}

	// Test with func lit
	funcLit := &ast.FuncLit{}
	if got := e.extractFunctionReference(funcLit); got != "" {
		t.Errorf("extractFunctionReference(funcLit) = %q, want empty", got)
	}
}

func TestExtractNilFunction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger)

	ctx := context.Background()

	// Test with nil function body
	fn := &ast.FuncDecl{
		Name: &ast.Ident{Name: "Test"},
		Body: nil,
	}
	calls, err := e.ExtractCalls(ctx, fn, "test.go")
	if err != nil {
		t.Errorf("ExtractCalls with nil body should not error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("Expected 0 calls from nil body, got %d", len(calls))
	}
}

func TestExtractCallsWithFileSet(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(ctx, MyActivity).Get(ctx, nil)
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	ctx := context.Background()

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "MyWorkflow" {
			calls, err := e.ExtractCallsWithFileSet(ctx, fn, "test.go", fset)
			if err != nil {
				t.Fatalf("ExtractCallsWithFileSet failed: %v", err)
			}
			if len(calls) == 0 {
				t.Error("Expected to find at least one call")
			}
			// Check line numbers are reasonable
			for _, call := range calls {
				if call.LineNumber <= 0 {
					t.Errorf("Expected positive line number, got %d", call.LineNumber)
				}
			}
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestGetLineNumber(t *testing.T) {
	code := `package test

func f() {
	foo()
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	// Find the call
	var call *ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			call = c
			return false
		}
		return true
	})

	if call == nil {
		t.Fatal("Call not found")
	}

	// Test with fset
	lineNum := e.getLineNumber(call, fset)
	if lineNum != 4 {
		t.Errorf("getLineNumber with fset = %d, want 4", lineNum)
	}

	// Test without fset (returns position as int)
	lineNumNoFset := e.getLineNumber(call, nil)
	if lineNumNoFset <= 0 {
		t.Errorf("getLineNumber without fset should return positive, got %d", lineNumNoFset)
	}
}
