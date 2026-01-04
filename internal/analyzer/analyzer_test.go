package analyzer

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
)

func TestNewAnalyzer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)
	if a == nil {
		t.Fatal("NewAnalyzer returned nil")
	}
}

func TestAnalyze(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	workflowContent := `package test

import "go.temporal.io/sdk/workflow"

// ProcessOrderWorkflow handles order processing
func ProcessOrderWorkflow(ctx workflow.Context, orderID string) error {
	workflow.ExecuteActivity(ctx, SendEmailActivity, orderID).Get(ctx, nil)
	return nil
}
`
	workflowFile := filepath.Join(tmpDir, "workflow.go")
	if err := os.WriteFile(workflowFile, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to create workflow file: %v", err)
	}

	activityContent := `package test

import "context"

// SendEmailActivity sends an email notification
func SendEmailActivity(ctx context.Context, orderID string) error {
	return nil
}
`
	activityFile := filepath.Join(tmpDir, "activity.go")
	if err := os.WriteFile(activityFile, []byte(activityContent), 0644); err != nil {
		t.Fatalf("Failed to create activity file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir:      tmpDir,
		IncludeTests: false,
	}

	graph, err := a.Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Analyze returned nil graph")
	}

	// Should have found both workflow and activity
	if len(graph.Nodes) < 2 {
		t.Errorf("Expected at least 2 nodes, got %d", len(graph.Nodes))
	}

	// Verify workflow was found
	foundWorkflow := false
	foundActivity := false
	for _, node := range graph.Nodes {
		if node.Name == "ProcessOrderWorkflow" {
			foundWorkflow = true
			if node.Type != "workflow" {
				t.Errorf("ProcessOrderWorkflow type = %s, want workflow", node.Type)
			}
		}
		if node.Name == "SendEmailActivity" {
			foundActivity = true
			if node.Type != "activity" {
				t.Errorf("SendEmailActivity type = %s, want activity", node.Type)
			}
		}
	}
	if !foundWorkflow {
		t.Error("ProcessOrderWorkflow not found")
	}
	if !foundActivity {
		t.Error("SendEmailActivity not found")
	}
}

func TestAnalyzeWithFilters(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files in different packages
	pkg1Dir := filepath.Join(tmpDir, "pkg1")
	if err := os.MkdirAll(pkg1Dir, 0755); err != nil {
		t.Fatalf("Failed to create pkg1 dir: %v", err)
	}

	pkg1Content := `package pkg1

func Pkg1Workflow() {}
`
	pkg1File := filepath.Join(pkg1Dir, "workflow.go")
	if err := os.WriteFile(pkg1File, []byte(pkg1Content), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	pkg2Dir := filepath.Join(tmpDir, "pkg2")
	if err := os.MkdirAll(pkg2Dir, 0755); err != nil {
		t.Fatalf("Failed to create pkg2 dir: %v", err)
	}

	pkg2Content := `package pkg2

func Pkg2Workflow() {}
`
	pkg2File := filepath.Join(pkg2Dir, "workflow.go")
	if err := os.WriteFile(pkg2File, []byte(pkg2Content), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)

	ctx := context.Background()

	// Test with package filter
	opts := config.AnalysisOptions{
		RootDir:       tmpDir,
		FilterPackage: "pkg1",
	}

	graph, err := a.Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should only find pkg1 workflows
	for _, node := range graph.Nodes {
		if node.Package == "pkg2" {
			t.Error("Found pkg2 node when filtering for pkg1")
		}
	}
}

func TestAnalyzeContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package test
func TestWorkflow() {}
`
	file := filepath.Join(tmpDir, "workflow.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := config.AnalysisOptions{
		RootDir: tmpDir,
	}

	_, err := a.Analyze(ctx, opts)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

func TestAnalyzeNonExistentDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir: "/non/existent/path/that/definitely/does/not/exist/xyz123",
	}

	// The parser may return empty results instead of error for non-existent paths
	// This is acceptable behavior - we just verify it doesn't panic
	graph, err := a.Analyze(ctx, opts)
	// Either an error or an empty graph is acceptable
	if err == nil && graph != nil && len(graph.Nodes) > 0 {
		t.Error("Expected error or empty graph for non-existent directory")
	}
}

func TestAnalyzeWithExcludes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a vendor directory that should be excluded
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}

	vendorContent := `package vendor
func VendorWorkflow() {}
`
	vendorFile := filepath.Join(vendorDir, "workflow.go")
	if err := os.WriteFile(vendorFile, []byte(vendorContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create a main file
	mainContent := `package main
func MainWorkflow() {}
`
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir:     tmpDir,
		ExcludeDirs: []string{"vendor"},
	}

	graph, err := a.Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should not find VendorWorkflow
	for _, node := range graph.Nodes {
		if node.Name == "VendorWorkflow" {
			t.Error("Found VendorWorkflow when it should be excluded")
		}
	}
}

func TestAnalyzeComplexWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package test

import (
	"time"
	"go.temporal.io/sdk/workflow"
	"go.temporal.io/sdk/temporal"
)

func ComplexWorkflow(ctx workflow.Context) error {
	// Set activity options
	opts := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, opts)

	// Execute activity
	workflow.ExecuteActivity(ctx, MyActivity).Get(ctx, nil)

	// Set signal handler
	workflow.SetSignalHandler(ctx, "cancel", func() {})

	// Set query handler
	workflow.SetQueryHandler(ctx, "status", func() (string, error) { return "running", nil })

	// Sleep
	workflow.Sleep(ctx, time.Hour)

	// Version check
	v := workflow.GetVersion(ctx, "change1", workflow.DefaultVersion, 1)
	_ = v

	// Child workflow
	childOpts := workflow.ChildWorkflowOptions{WorkflowID: "child"}
	ctx = workflow.WithChildOptions(ctx, childOpts)
	workflow.ExecuteChildWorkflow(ctx, ChildWorkflow).Get(ctx, nil)

	return nil
}

func ChildWorkflow(ctx workflow.Context) error {
	return nil
}

func MyActivity() error {
	return nil
}
`
	file := filepath.Join(tmpDir, "workflow.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir: tmpDir,
	}

	graph, err := a.Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Find ComplexWorkflow
	var complexNode *TemporalNode
	for _, node := range graph.Nodes {
		if node.Name == "ComplexWorkflow" {
			complexNode = node
			break
		}
	}

	if complexNode == nil {
		t.Fatal("ComplexWorkflow not found")
	}

	// Verify it has various Temporal constructs
	if len(complexNode.CallSites) == 0 {
		t.Error("Expected call sites")
	}
	if len(complexNode.Signals) == 0 {
		t.Error("Expected signals")
	}
	if len(complexNode.Queries) == 0 {
		t.Error("Expected queries")
	}
}

func TestAnalyzeEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir: tmpDir,
	}

	graph, err := a.Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed on empty directory: %v", err)
	}

	if graph == nil {
		t.Fatal("Analyze returned nil graph")
	}

	if len(graph.Nodes) != 0 {
		t.Errorf("Expected 0 nodes for empty directory, got %d", len(graph.Nodes))
	}
}

func TestAnalyzeWithNameFilter(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package test

func ProcessOrderWorkflow() {}
func ProcessPaymentWorkflow() {}
func SendEmailActivity() {}
`
	file := filepath.Join(tmpDir, "workflow.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewAnalyzer(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir:    tmpDir,
		FilterName: "ProcessOrder.*",
	}

	graph, err := a.Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should only find ProcessOrderWorkflow
	for _, node := range graph.Nodes {
		if node.Name == "ProcessPaymentWorkflow" || node.Name == "SendEmailActivity" {
			t.Errorf("Found %s when filtering for ProcessOrder.*", node.Name)
		}
	}
}

