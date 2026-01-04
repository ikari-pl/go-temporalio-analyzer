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

func TestNewGraphBuilder(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)
	if builder == nil {
		t.Fatal("NewGraphBuilder returned nil")
	}
}

func TestBuildGraph(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	return nil
}

func MyActivity() error {
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	var matches []NodeMatch
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			nodeType := ""
			switch fn.Name.Name {
			case "MyWorkflow":
				nodeType = "workflow"
			case "MyActivity":
				nodeType = "activity"
			}
			matches = append(matches, NodeMatch{
				Node:     fn,
				FileSet:  fset,
				FilePath: "test.go",
				Package:  "test",
				NodeType: nodeType,
			})
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)

	ctx := context.Background()
	graph, err := builder.BuildGraph(ctx, matches)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("BuildGraph returned nil graph")
	}

	if len(graph.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(graph.Nodes))
	}

	// Check for specific nodes
	foundWorkflow := false
	foundActivity := false
	for name, node := range graph.Nodes {
		switch name {
		case "MyWorkflow":
			foundWorkflow = true
			if node.Type != "workflow" {
				t.Errorf("MyWorkflow type = %s, want workflow", node.Type)
			}
		case "MyActivity":
			foundActivity = true
			if node.Type != "activity" {
				t.Errorf("MyActivity type = %s, want activity", node.Type)
			}
		}
	}
	if !foundWorkflow {
		t.Error("MyWorkflow not found in graph")
	}
	if !foundActivity {
		t.Error("MyActivity not found in graph")
	}
}

func TestBuildGraphWithRelationships(t *testing.T) {
	code := `package test

import "go.temporal.io/sdk/workflow"

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(ctx, MyActivity).Get(ctx, nil)
	return nil
}

func MyActivity() error {
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	var matches []NodeMatch
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			nodeType := ""
			switch fn.Name.Name {
			case "MyWorkflow":
				nodeType = "workflow"
			case "MyActivity":
				nodeType = "activity"
			}
			matches = append(matches, NodeMatch{
				Node:     fn,
				FileSet:  fset,
				FilePath: "test.go",
				Package:  "test",
				NodeType: nodeType,
			})
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)

	ctx := context.Background()
	graph, err := builder.BuildGraph(ctx, matches)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	// Find MyWorkflow and check it has call sites
	if node, ok := graph.Nodes["MyWorkflow"]; ok {
		if len(node.CallSites) == 0 {
			t.Error("MyWorkflow should have call sites")
		}
	} else {
		t.Error("MyWorkflow not found")
	}
}

func TestBuildGraphContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fset := token.NewFileSet()
	matches := []NodeMatch{{
		Node:     &ast.FuncDecl{Name: &ast.Ident{Name: "Test"}},
		FileSet:  fset,
		FilePath: "test.go",
		Package:  "test",
		NodeType: "workflow",
	}}

	_, err := builder.BuildGraph(ctx, matches)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

func TestBuildGraphEmptyInput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)

	ctx := context.Background()
	graph, err := builder.BuildGraph(ctx, nil)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	if graph == nil {
		t.Fatal("BuildGraph returned nil for empty input")
	}
	if len(graph.Nodes) != 0 {
		t.Errorf("Expected 0 nodes for empty input, got %d", len(graph.Nodes))
	}
}

func TestCalculateStats(t *testing.T) {
	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"Workflow1": {Name: "Workflow1", Type: "workflow", CallSites: []CallSite{{TargetName: "Activity1"}}},
			"Workflow2": {Name: "Workflow2", Type: "workflow", CallSites: []CallSite{{TargetName: "Activity1"}, {TargetName: "Activity2"}}},
			"Activity1": {Name: "Activity1", Type: "activity", Parents: []string{"Workflow1", "Workflow2"}},
			"Activity2": {Name: "Activity2", Type: "activity", Parents: []string{"Workflow2"}},
			"Signal1":   {Name: "Signal1", Type: "signal"},
			"Query1":    {Name: "Query1", Type: "query"},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)

	ctx := context.Background()
	err := builder.CalculateStats(ctx, graph)
	if err != nil {
		t.Fatalf("CalculateStats failed: %v", err)
	}

	if graph.Stats.TotalWorkflows != 2 {
		t.Errorf("TotalWorkflows = %d, want 2", graph.Stats.TotalWorkflows)
	}
	if graph.Stats.TotalActivities != 2 {
		t.Errorf("TotalActivities = %d, want 2", graph.Stats.TotalActivities)
	}
	if graph.Stats.TotalSignals != 1 {
		t.Errorf("TotalSignals = %d, want 1", graph.Stats.TotalSignals)
	}
	if graph.Stats.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", graph.Stats.TotalQueries)
	}
}

func TestCalculateStatsMaxFanOut(t *testing.T) {
	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"Workflow1": {Name: "Workflow1", Type: "workflow", CallSites: []CallSite{
				{TargetName: "A1"}, {TargetName: "A2"}, {TargetName: "A3"},
			}},
			"Workflow2": {Name: "Workflow2", Type: "workflow", CallSites: []CallSite{
				{TargetName: "A1"},
			}},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)

	ctx := context.Background()
	err := builder.CalculateStats(ctx, graph)
	if err != nil {
		t.Fatalf("CalculateStats failed: %v", err)
	}

	if graph.Stats.MaxFanOut != 3 {
		t.Errorf("MaxFanOut = %d, want 3", graph.Stats.MaxFanOut)
	}
}

func TestCalculateStatsContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"Test": {Name: "Test", Type: "workflow"},
		},
	}

	err := builder.CalculateStats(ctx, graph)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

func TestExtractDescription(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor).(*graphBuilder)

	code := `package test

// MyWorkflow processes orders.
// It calls activities to complete the order.
func MyWorkflow() {}

func NoCommentFunc() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			desc := builder.extractDescription(fn)
			if fn.Name.Name == "MyWorkflow" {
				if desc == "" {
					t.Error("Expected description for MyWorkflow")
				}
			} else if fn.Name.Name == "NoCommentFunc" {
				if desc != "" {
					t.Error("Expected empty description for NoCommentFunc")
				}
			}
		}
	}
}

func TestExtractReturnType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor).(*graphBuilder)

	code := `package test

func ReturnsError() error { return nil }
func ReturnsString() string { return "" }
func ReturnsMultiple() (int, error) { return 0, nil }
func ReturnsNothing() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	tests := map[string]string{
		"ReturnsError":    "error",
		"ReturnsString":   "string",
		"ReturnsMultiple": "int",
		"ReturnsNothing":  "",
	}

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			expected := tests[fn.Name.Name]
			got := builder.extractReturnType(fn)
			if got != expected {
				t.Errorf("extractReturnType(%s) = %q, want %q", fn.Name.Name, got, expected)
			}
		}
	}
}

func TestGraphBuilderTypeToString(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor).(*graphBuilder)

	code := `package test

var a int
var b string
var c *int
var d []string
var e map[string]int
var f interface{}
var g pkg.Type
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	tests := map[string]string{
		"a": "int",
		"b": "string",
		"c": "*int",
		"d": "[]string",
		"e": "map[string]int",
		"f": "interface{}",
		"g": "pkg.Type",
	}

	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Names) > 0 {
					name := vs.Names[0].Name
					expected := tests[name]
					got := builder.typeToString(vs.Type)
					if got != expected {
						t.Errorf("typeToString(%s) = %q, want %q", name, got, expected)
					}
				}
			}
		}
	}
}

func TestAddUniqueParent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor).(*graphBuilder)

	// Test adding new parent
	parents := []string{"Parent1"}
	parents = builder.addUniqueParent(parents, "Parent2")
	if len(parents) != 2 {
		t.Errorf("Expected 2 parents, got %d", len(parents))
	}

	// Test adding duplicate parent
	parents = builder.addUniqueParent(parents, "Parent1")
	if len(parents) != 2 {
		t.Errorf("Expected 2 parents after duplicate add, got %d", len(parents))
	}
}

func TestCalculateMaxDepth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor).(*graphBuilder)

	// Create a graph with depth 3: W1 -> A1 -> W2 -> A2
	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"W1": {Name: "W1", Type: "workflow", CallSites: []CallSite{{TargetName: "A1"}}},
			"A1": {Name: "A1", Type: "activity", Parents: []string{"W1"}, CallSites: []CallSite{{TargetName: "W2"}}},
			"W2": {Name: "W2", Type: "workflow", Parents: []string{"A1"}, CallSites: []CallSite{{TargetName: "A2"}}},
			"A2": {Name: "A2", Type: "activity", Parents: []string{"W2"}},
		},
	}

	ctx := context.Background()
	depth := builder.calculateMaxDepth(ctx, graph)
	if depth < 3 {
		t.Errorf("calculateMaxDepth = %d, want at least 3", depth)
	}
}

func TestCalculateNodeDepthCycleDetection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor).(*graphBuilder)

	// Create a cyclic graph: W1 -> A1 -> W1
	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"W1": {Name: "W1", Type: "workflow", CallSites: []CallSite{{TargetName: "A1"}}},
			"A1": {Name: "A1", Type: "activity", Parents: []string{"W1"}, CallSites: []CallSite{{TargetName: "W1"}}},
		},
	}

	ctx := context.Background()
	// Should not infinite loop due to cycle detection
	depth := builder.calculateMaxDepth(ctx, graph)
	if depth < 0 {
		t.Error("calculateMaxDepth returned negative for cyclic graph")
	}
}
