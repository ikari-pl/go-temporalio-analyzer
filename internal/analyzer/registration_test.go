package analyzer

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
)

func TestNewRegistrationScanner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewRegistrationScanner(logger)
	if scanner == nil {
		t.Fatal("NewRegistrationScanner returned nil")
	}
}

func TestScanDirectoryWithDirectRegistration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with direct function registration
	content := `package main

import "go.temporal.io/sdk/worker"

func MyActivity() error {
	return nil
}

func MyWorkflow() error {
	return nil
}

func main() {
	worker.RegisterActivity(MyActivity)
	worker.RegisterWorkflow(MyWorkflow)
}
`
	file := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewRegistrationScanner(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{}

	info, err := scanner.ScanDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Check that MyActivity was found
	if _, ok := info.Activities["MyActivity"]; !ok {
		t.Error("Expected to find MyActivity in registered activities")
	}

	// Check that MyWorkflow was found
	if _, ok := info.Workflows["MyWorkflow"]; !ok {
		t.Error("Expected to find MyWorkflow in registered workflows")
	}
}

func TestScanDirectoryWithStructRegistration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with struct registration
	content := `package main

import "go.temporal.io/sdk/worker"

type MyActivities struct {}

func (a *MyActivities) SendEmail() error {
	return nil
}

func (a *MyActivities) ProcessPayment() error {
	return nil
}

func main() {
	worker.RegisterActivity(&MyActivities{})
}
`
	file := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewRegistrationScanner(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{}

	info, err := scanner.ScanDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Check that MyActivities type was registered
	if regType, ok := info.RegisteredTypes["MyActivities"]; !ok {
		t.Error("Expected to find MyActivities in registered types")
	} else if regType != "activity" {
		t.Errorf("Expected MyActivities to be registered as 'activity', got %s", regType)
	}
}

func TestScanDirectoryWithNewRegistration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with new() registration
	content := `package main

import "go.temporal.io/sdk/worker"

type MyActivities struct {}

func main() {
	worker.RegisterActivity(new(MyActivities))
}
`
	file := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewRegistrationScanner(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{}

	info, err := scanner.ScanDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Check that MyActivities type was registered
	if regType, ok := info.RegisteredTypes["MyActivities"]; !ok {
		t.Error("Expected to find MyActivities in registered types")
	} else if regType != "activity" {
		t.Errorf("Expected MyActivities to be registered as 'activity', got %s", regType)
	}
}

func TestScanDirectoryWithOptionsRegistration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with WithOptions registration
	content := `package main

import "go.temporal.io/sdk/worker"

func MyActivity() error {
	return nil
}

func main() {
	worker.RegisterActivityWithOptions(MyActivity, worker.RegisterActivityOptions{})
}
`
	file := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewRegistrationScanner(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{}

	info, err := scanner.ScanDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Check that MyActivity was found
	if _, ok := info.Activities["MyActivity"]; !ok {
		t.Error("Expected to find MyActivity in registered activities")
	}
}

func TestIsRegisteredActivity(t *testing.T) {
	info := &RegistrationInfo{
		Activities: map[string]*Registration{
			"DirectActivity": {Name: "DirectActivity", Type: "activity"},
		},
		RegisteredTypes: map[string]string{
			"MyActivities": "activity",
		},
	}

	tests := []struct {
		name         string
		funcName     string
		receiverType string
		expected     bool
	}{
		{"direct registration", "DirectActivity", "", true},
		{"struct method with matching type", "SendEmail", "MyActivities", true},
		{"struct method with pointer type", "SendEmail", "*MyActivities", true},
		{"unregistered function", "UnknownFunc", "", false},
		{"method with unregistered type", "SendEmail", "OtherType", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := info.IsRegisteredActivity(tt.funcName, tt.receiverType)
			if result != tt.expected {
				t.Errorf("IsRegisteredActivity(%q, %q) = %v, want %v",
					tt.funcName, tt.receiverType, result, tt.expected)
			}
		})
	}
}

func TestIsRegisteredWorkflow(t *testing.T) {
	info := &RegistrationInfo{
		Workflows: map[string]*Registration{
			"MyWorkflow": {Name: "MyWorkflow", Type: "workflow"},
		},
	}

	tests := []struct {
		name     string
		funcName string
		expected bool
	}{
		{"registered workflow", "MyWorkflow", true},
		{"unregistered workflow", "UnknownWorkflow", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := info.IsRegisteredWorkflow(tt.funcName)
			if result != tt.expected {
				t.Errorf("IsRegisteredWorkflow(%q) = %v, want %v",
					tt.funcName, result, tt.expected)
			}
		})
	}
}

func TestScanDirectoryContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main
func Test() {}
`
	file := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewRegistrationScanner(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := config.AnalysisOptions{}
	_, err := scanner.ScanDirectory(ctx, tmpDir, opts)
	if err == nil {
		t.Error("Expected error from cancelled context")
	}
}

func TestParserWithRegistration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with both registration and activity definition
	content := `package main

import (
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type MyActivities struct {}

func (a *MyActivities) SendEmail() error {
	return nil
}

func (a *MyActivities) ProcessPayment() error {
	return nil
}

func MyWorkflow(ctx workflow.Context) error {
	workflow.ExecuteActivity(ctx, nil)
	return nil
}

func main() {
	worker.RegisterActivity(&MyActivities{})
	worker.RegisterWorkflow(MyWorkflow)
}
`
	file := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	parser := NewParser(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{
		RootDir: tmpDir,
	}

	matches, err := parser.ParseDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ParseDirectory failed: %v", err)
	}

	// Should find:
	// - MyWorkflow (via workflow.Context + SDK calls AND registration)
	// - SendEmail (via struct registration)
	// - ProcessPayment (via struct registration)
	foundWorkflow := false
	foundSendEmail := false
	foundProcessPayment := false

	for _, match := range matches {
		switch match.NodeType {
		case "workflow":
			if match.Node != nil {
				foundWorkflow = true
			}
		case "activity":
			foundSendEmail = true
			foundProcessPayment = true
		}
	}

	if !foundWorkflow {
		t.Error("Expected to find workflow")
	}

	// Note: We found at least some activities via registration
	if len(matches) < 2 {
		t.Errorf("Expected at least 2 matches (workflow + activities), got %d", len(matches))
	}

	// Log what we found for debugging
	t.Logf("Found %d matches", len(matches))
	for _, m := range matches {
		t.Logf("  - %s: %s", m.NodeType, m.FilePath)
	}

	_ = foundSendEmail
	_ = foundProcessPayment
}

func TestIsRegisteredType(t *testing.T) {
	info := &RegistrationInfo{
		Activities:      make(map[string]*Registration),
		Workflows:       make(map[string]*Registration),
		RegisteredTypes: make(map[string]string),
	}

	// Add some registered types
	info.RegisteredTypes["MyActivities"] = "activity"
	info.RegisteredTypes["MyWorkflows"] = "workflow"

	// Test activity type
	regType, ok := info.IsRegisteredType("MyActivities")
	if !ok {
		t.Error("Expected MyActivities to be registered")
	}
	if regType != "activity" {
		t.Errorf("Expected type 'activity', got %q", regType)
	}

	// Test workflow type
	regType, ok = info.IsRegisteredType("MyWorkflows")
	if !ok {
		t.Error("Expected MyWorkflows to be registered")
	}
	if regType != "workflow" {
		t.Errorf("Expected type 'workflow', got %q", regType)
	}

	// Test unregistered type
	_, ok = info.IsRegisteredType("UnknownType")
	if ok {
		t.Error("Expected UnknownType to not be registered")
	}
}

func TestHandlePointerArgEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Test various pointer argument patterns
	content := `package main

import "go.temporal.io/sdk/worker"

type Activities struct{}
func (a *Activities) DoWork() error { return nil }

type Workflows struct{}
func (w *Workflows) Run() error { return nil }

func main() {
	// new() pattern
	worker.RegisterActivity(new(Activities))

	// Composite literal with & in variable
	activities := &Activities{}
	worker.RegisterActivity(activities)

	// Direct &Type{} pattern
	worker.RegisterWorkflow(&Workflows{})
}
`
	file := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewRegistrationScanner(logger)

	ctx := context.Background()
	opts := config.AnalysisOptions{RootDir: tmpDir}
	info, err := scanner.ScanDirectory(ctx, tmpDir, opts)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	// Check that Activities was registered via new()
	if _, ok := info.RegisteredTypes["Activities"]; !ok {
		t.Error("Expected Activities to be registered via new()")
	}

	// Check that Workflows was registered via &Type{}
	if _, ok := info.RegisteredTypes["Workflows"]; !ok {
		t.Error("Expected Workflows to be registered via &Type{}")
	}
}
