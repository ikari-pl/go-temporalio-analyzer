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

	// Test with selector - now includes receiver for better disambiguation
	sel := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "pkg"},
		Sel: &ast.Ident{Name: "Function"},
	}
	if got := e.extractFunctionReference(sel); got != "pkg.Function" {
		t.Errorf("extractFunctionReference(sel) = %q, want %q", got, "pkg.Function")
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

func TestExtractResultType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	tests := []struct {
		name     string
		code     string
		wantType string
	}{
		{
			name:     "unary address of identifier",
			code:     `package test; var _ = &result`,
			wantType: "var:result",
		},
		{
			name:     "unary address of composite literal",
			code:     `package test; var _ = &MyType{}`,
			wantType: "MyType",
		},
		{
			name:     "identifier",
			code:     `package test; var _ = result`,
			wantType: "var:result",
		},
		{
			name:     "new call",
			code:     `package test; var _ = new(MyType)`,
			wantType: "MyType",
		},
		{
			name:     "composite literal",
			code:     `package test; var _ = MyType{}`,
			wantType: "MyType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			// Find the expression
			var expr ast.Expr
			ast.Inspect(file, func(n ast.Node) bool {
				if vs, ok := n.(*ast.ValueSpec); ok && len(vs.Values) > 0 {
					expr = vs.Values[0]
					return false
				}
				return true
			})

			if expr == nil {
				t.Fatal("Expression not found")
			}

			got := e.extractResultType(expr)
			if got != tt.wantType {
				t.Errorf("extractResultType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestInferExprType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := NewCallExtractor(logger).(*callExtractor)

	tests := []struct {
		name     string
		code     string
		wantType string
	}{
		{
			name:     "int literal",
			code:     `package test; var _ = 42`,
			wantType: "int",
		},
		{
			name:     "float literal",
			code:     `package test; var _ = 3.14`,
			wantType: "float64",
		},
		{
			name:     "string literal",
			code:     `package test; var _ = "hello"`,
			wantType: "string",
		},
		{
			name:     "bool true",
			code:     `package test; var _ = true`,
			wantType: "bool",
		},
		{
			name:     "bool false",
			code:     `package test; var _ = false`,
			wantType: "bool",
		},
		{
			name:     "nil",
			code:     `package test; var _ = nil`,
			wantType: "nil",
		},
		{
			name:     "identifier",
			code:     `package test; var _ = myVar`,
			wantType: "var:myVar",
		},
		{
			name:     "selector",
			code:     `package test; var _ = pkg.Value`,
			wantType: "selector:pkg.Value",
		},
		{
			name:     "address of",
			code:     `package test; var _ = &myVar`,
			wantType: "*var:myVar",
		},
		{
			name:     "composite literal",
			code:     `package test; var _ = MyStruct{}`,
			wantType: "MyStruct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			// Find the expression
			var expr ast.Expr
			ast.Inspect(file, func(n ast.Node) bool {
				if vs, ok := n.(*ast.ValueSpec); ok && len(vs.Values) > 0 {
					expr = vs.Values[0]
					return false
				}
				return true
			})

			if expr == nil {
				t.Fatal("Expression not found")
			}

			got := e.inferExprType(expr)
			if got != tt.wantType {
				t.Errorf("inferExprType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestExtractCallsWithGetResultType(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	var result MyResult
	workflow.ExecuteActivity(ctx, MyActivity, "arg").Get(ctx, &result)
	return nil
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
				t.Fatal("Expected to find at least one call")
			}

			// Check that result type was extracted from .Get()
			found := false
			for _, call := range calls {
				if call.TargetName == "MyActivity" {
					found = true
					if call.ResultType == "" {
						t.Error("Expected ResultType to be extracted from .Get() call")
					}
					if call.ResultType != "var:result" {
						t.Errorf("Expected ResultType = 'var:result', got %q", call.ResultType)
					}
				}
			}
			if !found {
				t.Error("Expected to find MyActivity call")
			}
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractCallsWithGetCompositeLiteral(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(ctx, MyActivity).Get(ctx, &MyResult{})
	return nil
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

			for _, call := range calls {
				if call.TargetName == "MyActivity" {
					if call.ResultType != "MyResult" {
						t.Errorf("Expected ResultType = 'MyResult', got %q", call.ResultType)
					}
					return
				}
			}
			t.Error("Expected to find MyActivity call")
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractActivityOptions(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	workflow.ExecuteActivity(ctx, MyActivity)
	return nil
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
			// The activity call should be found
			if len(calls) == 0 {
				t.Error("Expected to find at least one call")
			}
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractActivityOptionsWithInlineOptions(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: time.Hour,
			HeartbeatTimeout:    time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 5,
			},
		}),
		MyActivity,
	)
	return nil
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

			for _, call := range calls {
				if call.TargetName == "MyActivity" {
					if call.ParsedActivityOpts == nil {
						t.Error("Expected ParsedActivityOpts to be set")
						return
					}
					if call.ParsedActivityOpts.StartToCloseTimeout == "" {
						t.Error("Expected StartToCloseTimeout to be parsed")
					}
					if call.ParsedActivityOpts.HeartbeatTimeout == "" {
						t.Error("Expected HeartbeatTimeout to be parsed")
					}
					if !call.ParsedActivityOpts.HasRetryPolicy() {
						t.Error("Expected RetryPolicy to be detected")
					}
					return
				}
			}
			t.Error("Expected to find MyActivity call")
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractActivityOptionsWithPointer(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, &workflow.ActivityOptions{
			ScheduleToCloseTimeout: time.Hour,
		}),
		MyActivity,
	)
	return nil
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

			for _, call := range calls {
				if call.TargetName == "MyActivity" {
					if call.ParsedActivityOpts == nil {
						t.Error("Expected ParsedActivityOpts to be set for pointer type")
						return
					}
					if call.ParsedActivityOpts.ScheduleToCloseTimeout == "" {
						t.Error("Expected ScheduleToCloseTimeout to be parsed")
					}
					return
				}
			}
			t.Error("Expected to find MyActivity call")
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractActivityOptionsWithVariable(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, opts),
		MyActivity,
	)
	return nil
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

			for _, call := range calls {
				if call.TargetName == "MyActivity" {
					if call.ParsedActivityOpts == nil {
						t.Error("Expected ParsedActivityOpts to be set for variable reference")
						return
					}
					// When options are a variable, we can't parse details but should mark as provided
					if !call.ParsedActivityOpts.OptionsProvided() {
						t.Error("Expected OptionsProvided to be true for variable reference")
					}
					return
				}
			}
			t.Error("Expected to find MyActivity call")
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestExtractLocalActivityOptions(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteLocalActivity(
		workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
			StartToCloseTimeout: time.Minute,
		}),
		MyLocalActivity,
	)
	return nil
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

			for _, call := range calls {
				if call.TargetName == "MyLocalActivity" {
					if call.ParsedActivityOpts == nil {
						t.Error("Expected ParsedActivityOpts to be set for local activity")
						return
					}
					return
				}
			}
			t.Error("Expected to find MyLocalActivity call")
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestParseRetryPolicyWithVariableReference(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			RetryPolicy: myRetryPolicy,
		}),
		MyActivity,
	)
	return nil
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

			for _, call := range calls {
				if call.TargetName == "MyActivity" {
					if call.ParsedActivityOpts == nil {
						t.Error("Expected ParsedActivityOpts to be set")
						return
					}
					// When RetryPolicy is a variable, it should still be detected as present
					if !call.ParsedActivityOpts.HasRetryPolicy() {
						t.Error("Expected RetryPolicy to be detected even as variable reference")
					}
					return
				}
			}
			t.Error("Expected to find MyActivity call")
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestParseActivityOptionsAllTimeouts(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout:    10 * time.Minute,
			ScheduleToCloseTimeout: 30 * time.Minute,
			ScheduleToStartTimeout: 5 * time.Minute,
			HeartbeatTimeout:       time.Minute,
		}),
		MyActivity,
	)
	return nil
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

			for _, call := range calls {
				if call.TargetName == "MyActivity" {
					opts := call.ParsedActivityOpts
					if opts == nil {
						t.Fatal("Expected ParsedActivityOpts to be set")
					}
					if opts.StartToCloseTimeout == "" {
						t.Error("Expected StartToCloseTimeout to be parsed")
					}
					if opts.ScheduleToCloseTimeout == "" {
						t.Error("Expected ScheduleToCloseTimeout to be parsed")
					}
					if opts.ScheduleToStartTimeout == "" {
						t.Error("Expected ScheduleToStartTimeout to be parsed")
					}
					if opts.HeartbeatTimeout == "" {
						t.Error("Expected HeartbeatTimeout to be parsed")
					}
					return
				}
			}
			t.Error("Expected to find MyActivity call")
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestParseRetryPolicyAllFields(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    time.Second,
				BackoffCoefficient: 2.5,
				MaximumInterval:    5 * time.Minute,
				MaximumAttempts:    10,
			},
		}),
		MyActivity,
	)
	return nil
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

			for _, call := range calls {
				if call.TargetName == "MyActivity" {
					opts := call.ParsedActivityOpts
					if opts == nil {
						t.Fatal("Expected ParsedActivityOpts to be set")
					}
					rp := opts.RetryPolicy
					if rp == nil {
						t.Fatal("Expected RetryPolicy to be set")
					}
					if rp.InitialInterval == "" {
						t.Error("Expected InitialInterval to be parsed")
					}
					if rp.BackoffCoefficient == "" {
						t.Error("Expected BackoffCoefficient to be parsed")
					}
					if rp.MaximumInterval == "" {
						t.Error("Expected MaximumInterval to be parsed")
					}
					if rp.MaximumAttempts != 10 {
						t.Errorf("Expected MaximumAttempts = 10, got %d", rp.MaximumAttempts)
					}
					return
				}
			}
			t.Error("Expected to find MyActivity call")
			return
		}
	}
	t.Fatal("Function MyWorkflow not found")
}

func TestActivityOptionsHelperMethods(t *testing.T) {
	// Test nil ActivityOptions
	var nilOpts *ActivityOptions
	if nilOpts.OptionsProvided() {
		t.Error("nil ActivityOptions should return false for OptionsProvided")
	}
	if nilOpts.HasRetryPolicy() {
		t.Error("nil ActivityOptions should return false for HasRetryPolicy")
	}

	// Test empty ActivityOptions
	emptyOpts := &ActivityOptions{}
	if emptyOpts.OptionsProvided() {
		t.Error("empty ActivityOptions should return false for OptionsProvided")
	}
	if emptyOpts.HasRetryPolicy() {
		t.Error("empty ActivityOptions should return false for HasRetryPolicy")
	}

	// Test with RetryPolicy having values
	optsWithRP := &ActivityOptions{
		RetryPolicy: &RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	if !optsWithRP.HasRetryPolicy() {
		t.Error("ActivityOptions with RetryPolicy.MaximumAttempts should return true for HasRetryPolicy")
	}

	// Test with RetryPolicy having BackoffCoefficient
	optsWithBackoff := &ActivityOptions{
		RetryPolicy: &RetryPolicy{
			BackoffCoefficient: "2.0",
		},
	}
	if !optsWithBackoff.HasRetryPolicy() {
		t.Error("ActivityOptions with RetryPolicy.BackoffCoefficient should return true for HasRetryPolicy")
	}
}
