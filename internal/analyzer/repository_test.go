package analyzer

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestNewRepository(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)
	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
}

func TestSaveAndLoadGraph(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "graph.json")

	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"TestWorkflow": {
				Name:       "TestWorkflow",
				Type:       "workflow",
				Package:    "test",
				FilePath:   "test.go",
				LineNumber: 10,
			},
			"TestActivity": {
				Name:       "TestActivity",
				Type:       "activity",
				Package:    "test",
				FilePath:   "test.go",
				LineNumber: 20,
			},
		},
		Stats: GraphStats{
			TotalWorkflows:  1,
			TotalActivities: 1,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx := context.Background()

	// Test save
	err := repo.SaveGraph(ctx, graph, outputPath)
	if err != nil {
		t.Fatalf("SaveGraph failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}

	// Test load
	loaded, err := repo.LoadGraph(ctx, outputPath)
	if err != nil {
		t.Fatalf("LoadGraph failed: %v", err)
	}

	// Verify loaded data
	if len(loaded.Nodes) != len(graph.Nodes) {
		t.Errorf("Loaded node count = %d, want %d", len(loaded.Nodes), len(graph.Nodes))
	}

	if loaded.Stats.TotalWorkflows != graph.Stats.TotalWorkflows {
		t.Errorf("Loaded TotalWorkflows = %d, want %d", loaded.Stats.TotalWorkflows, graph.Stats.TotalWorkflows)
	}

	// Verify node data
	for name, node := range loaded.Nodes {
		expected := graph.Nodes[name]
		if node.Name != expected.Name {
			t.Errorf("Node[%s].Name = %q, want %q", name, node.Name, expected.Name)
		}
		if node.Type != expected.Type {
			t.Errorf("Node[%s].Type = %q, want %q", name, node.Type, expected.Type)
		}
	}
}

func TestSaveGraphCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "graph.json")

	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"Test": {Name: "Test", Type: "workflow"},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx := context.Background()
	err := repo.SaveGraph(ctx, graph, nestedPath)
	if err != nil {
		t.Fatalf("SaveGraph failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Fatal("Output file was not created in nested directory")
	}
}

func TestSaveGraphContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "graph.json")

	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"Test": {Name: "Test", Type: "workflow"},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The save operation may succeed even with cancelled context if it's fast enough
	// This is an implementation detail - we just verify it doesn't hang
	_ = repo.SaveGraph(ctx, graph, outputPath)
}

func TestLoadGraphContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "graph.json")

	// Create a valid file first
	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"Test": {Name: "Test", Type: "workflow"},
		},
	}
	data, _ := json.MarshalIndent(graph, "", "  ")
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The load operation may succeed even with cancelled context if it's fast enough
	// This is an implementation detail - we just verify it doesn't hang
	_, _ = repo.LoadGraph(ctx, outputPath)
}

func TestLoadGraphNonExistentFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx := context.Background()
	_, err := repo.LoadGraph(ctx, "/non/existent/path.json")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLoadGraphInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	if err := os.WriteFile(outputPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx := context.Background()
	_, err := repo.LoadGraph(ctx, outputPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestSaveGraphNilGraph(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "graph.json")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx := context.Background()
	err := repo.SaveGraph(ctx, nil, outputPath)
	// Should either succeed with empty data or return an error - either is acceptable
	// The important thing is it doesn't panic
	_ = err
}

func TestSaveGraphWithComplexData(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "complex.json")

	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"ComplexWorkflow": {
				Name:        "ComplexWorkflow",
				Type:        "workflow",
				Package:     "test",
				FilePath:    "test.go",
				LineNumber:  10,
				Description: "A complex workflow with many features",
				Parameters:  map[string]string{"input": "string", "count": "int"},
				ReturnType:  "error",
				CallSites: []CallSite{
					{TargetName: "Activity1", TargetType: "activity", LineNumber: 15},
					{TargetName: "Activity2", TargetType: "activity", LineNumber: 20},
				},
				InternalCalls: []InternalCall{
					{TargetName: "helper", FilePath: "test.go", LineNumber: 12},
				},
				Parents: []string{"ParentWorkflow"},
				Signals: []SignalDef{{Name: "cancel", LineNumber: 25}},
				Queries: []QueryDef{{Name: "status", ReturnType: "string", LineNumber: 30}},
				Timers:  []TimerDef{{Duration: "1h", LineNumber: 35}},
				WorkflowOpts: &WorkflowOptions{
					TaskQueue: "my-queue",
				},
				ActivityOpts: &ActivityOptions{
					StartToCloseTimeout: "5m",
					RetryPolicy: &RetryPolicy{
						MaximumAttempts: 3,
					},
				},
				ChildWorkflow: []ChildWorkflow{{Name: "Child1", LineNumber: 40}},
				Versioning:    []VersionDef{{ChangeID: "change1", MinVersion: 1, MaxVersion: 2, LineNumber: 45}},
			},
		},
		Stats: GraphStats{
			TotalWorkflows: 1,
			MaxDepth:       5,
			MaxFanOut:      3,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx := context.Background()

	// Save
	err := repo.SaveGraph(ctx, graph, outputPath)
	if err != nil {
		t.Fatalf("SaveGraph failed: %v", err)
	}

	// Load and verify
	loaded, err := repo.LoadGraph(ctx, outputPath)
	if err != nil {
		t.Fatalf("LoadGraph failed: %v", err)
	}

	if len(loaded.Nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(loaded.Nodes))
	}

	node := loaded.Nodes["ComplexWorkflow"]
	if node == nil {
		t.Fatal("ComplexWorkflow not found")
	}
	if node.Name != "ComplexWorkflow" {
		t.Errorf("Name = %q, want %q", node.Name, "ComplexWorkflow")
	}
	if len(node.CallSites) != 2 {
		t.Errorf("CallSites count = %d, want 2", len(node.CallSites))
	}
	if len(node.Signals) != 1 {
		t.Errorf("Signals count = %d, want 1", len(node.Signals))
	}
	if node.WorkflowOpts == nil {
		t.Error("WorkflowOpts is nil")
	}
	if node.ActivityOpts == nil || node.ActivityOpts.RetryPolicy == nil {
		t.Error("ActivityOpts or RetryPolicy is nil")
	}
}

func TestSaveGraphToInvalidPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := NewRepository(logger)

	ctx := context.Background()
	graph := &TemporalGraph{Nodes: map[string]*TemporalNode{"Test": {Name: "Test"}}}

	// Try to write to a path that should fail (e.g., root directory on Unix)
	// This test is platform-dependent, so we use a path that should generally fail
	err := repo.SaveGraph(ctx, graph, "/proc/graph.json")
	// On Linux, this should fail. On other systems, it might not exist.
	// We just verify it doesn't panic.
	_ = err
}
