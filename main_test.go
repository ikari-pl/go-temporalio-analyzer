package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/lint"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/tui"
)

// mockAnalyzer implements analyzer.Analyzer for testing
type mockAnalyzer struct {
	graph *analyzer.TemporalGraph
	err   error
}

func (m *mockAnalyzer) Analyze(ctx context.Context, opts config.AnalysisOptions) (*analyzer.TemporalGraph, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.graph, nil
}

// mockTUI implements tui.TUI for testing
type mockTUI struct {
	runCalled bool
	runErr    error
}

func (m *mockTUI) Run(ctx context.Context, graph *analyzer.TemporalGraph) error {
	m.runCalled = true
	return m.runErr
}

// =============================================================================
// NewLogger Tests
// =============================================================================

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantNil  bool
	}{
		{
			name: "default config",
			cfg: &config.Config{
				Debug:   false,
				Verbose: false,
			},
			wantNil: false,
		},
		{
			name: "debug mode",
			cfg: &config.Config{
				Debug:   true,
				Verbose: false,
			},
			wantNil: false,
		},
		{
			name: "verbose mode",
			cfg: &config.Config{
				Debug:   false,
				Verbose: true,
			},
			wantNil: false,
		},
		{
			name: "debug takes precedence over verbose",
			cfg: &config.Config{
				Debug:   true,
				Verbose: true,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.cfg)
			if (logger == nil) != tt.wantNil {
				t.Errorf("NewLogger() = nil: %v, want nil: %v", logger == nil, tt.wantNil)
			}
		})
	}
}

// =============================================================================
// severityFromString Tests
// =============================================================================

func TestSeverityFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected lint.Severity
	}{
		{
			name:     "error severity",
			input:    "error",
			expected: lint.SeverityError,
		},
		{
			name:     "warning severity",
			input:    "warning",
			expected: lint.SeverityWarning,
		},
		{
			name:     "info severity",
			input:    "info",
			expected: lint.SeverityInfo,
		},
		{
			name:     "unknown severity defaults to info",
			input:    "unknown",
			expected: lint.SeverityInfo,
		},
		{
			name:     "empty string defaults to info",
			input:    "",
			expected: lint.SeverityInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := severityFromString(tt.input)
			if result != tt.expected {
				t.Errorf("severityFromString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// categoryTitle Tests
// =============================================================================

func TestCategoryTitle(t *testing.T) {
	tests := []struct {
		name     string
		category lint.Category
		expected string
	}{
		{
			name:     "reliability category",
			category: lint.CategoryReliability,
			expected: "🔒 Reliability",
		},
		{
			name:     "best practice category",
			category: lint.CategoryBestPractice,
			expected: "✨ Best Practices",
		},
		{
			name:     "performance category",
			category: lint.CategoryPerformance,
			expected: "⚡ Performance",
		},
		{
			name:     "maintenance category",
			category: lint.CategoryMaintenance,
			expected: "🔧 Maintenance",
		},
		{
			name:     "security category",
			category: lint.CategorySecurity,
			expected: "🛡️  Security",
		},
		{
			name:     "unknown category",
			category: lint.Category("unknown"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categoryTitle(tt.category)
			if result != tt.expected {
				t.Errorf("categoryTitle(%v) = %q, want %q", tt.category, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// run() Tests
// =============================================================================

func TestRun(t *testing.T) {
	// Helper to create a minimal graph
	createGraph := func() *analyzer.TemporalGraph {
		return &analyzer.TemporalGraph{
			Nodes: map[string]*analyzer.TemporalNode{
				"TestWorkflow": {
					Name:     "TestWorkflow",
					Type:     "workflow",
					FilePath: "test.go",
					Parents:  []string{},
				},
			},
			Stats: analyzer.GraphStats{
				TotalWorkflows:  1,
				TotalActivities: 0,
			},
		}
	}

	tests := []struct {
		name          string
		cfg           *config.Config
		graph         *analyzer.TemporalGraph
		analyzerErr   error
		tuiErr        error
		expectError   bool
		errorContains string
	}{
		{
			name: "json output format",
			cfg: &config.Config{
				RootDir:      ".",
				OutputFormat: "json",
			},
			graph:       createGraph(),
			expectError: false,
		},
		{
			name: "dot output format",
			cfg: &config.Config{
				RootDir:      ".",
				OutputFormat: "dot",
			},
			graph:       createGraph(),
			expectError: false,
		},
		{
			name: "mermaid output format",
			cfg: &config.Config{
				RootDir:      ".",
				OutputFormat: "mermaid",
			},
			graph:       createGraph(),
			expectError: false,
		},
		{
			name: "markdown output format",
			cfg: &config.Config{
				RootDir:      ".",
				OutputFormat: "markdown",
			},
			graph:       createGraph(),
			expectError: false,
		},
		{
			name: "md output format (alias)",
			cfg: &config.Config{
				RootDir:      ".",
				OutputFormat: "md",
			},
			graph:       createGraph(),
			expectError: false,
		},
		{
			name: "tui format without TUI instance",
			cfg: &config.Config{
				RootDir:      ".",
				OutputFormat: "tui",
			},
			graph:         createGraph(),
			expectError:   true,
			errorContains: "TUI not initialized",
		},
		{
			name: "unsupported output format",
			cfg: &config.Config{
				RootDir:      ".",
				OutputFormat: "unsupported",
			},
			graph:         createGraph(),
			expectError:   true,
			errorContains: "unsupported output format",
		},
		{
			name: "analyzer error",
			cfg: &config.Config{
				RootDir:      ".",
				OutputFormat: "json",
			},
			analyzerErr:   io.EOF,
			expectError:   true,
			errorContains: "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock analyzer
			mockA := &mockAnalyzer{
				graph: tt.graph,
				err:   tt.analyzerErr,
			}

			// Create logger (suppress output)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			// Create mock TUI if needed (we leave it nil to test the nil case)
			var tuiApp tui.TUI

			// Capture stdout for format tests
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run the function
			err := run(tt.cfg, logger, mockA, tuiApp)

			// Restore stdout
			_ = w.Close()
			os.Stdout = oldStdout
			_, _ = io.Copy(io.Discard, r)

			// Check error
			if tt.expectError {
				if err == nil {
					t.Errorf("run() expected error containing %q, got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("run() error = %v, want error containing %q", err, tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("run() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRunWithTUI(t *testing.T) {
	cfg := &config.Config{
		RootDir:      ".",
		OutputFormat: "tui",
	}

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name:     "TestWorkflow",
				Type:     "workflow",
				FilePath: "test.go",
				Parents:  []string{},
			},
		},
		Stats: analyzer.GraphStats{
			TotalWorkflows: 1,
		},
	}

	mockA := &mockAnalyzer{graph: graph}
	mockT := &mockTUI{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := run(cfg, logger, mockA, mockT)
	if err != nil {
		t.Errorf("run() unexpected error: %v", err)
	}

	if !mockT.runCalled {
		t.Error("TUI.Run() was not called")
	}
}

func TestRunWithTUIError(t *testing.T) {
	cfg := &config.Config{
		RootDir:      ".",
		OutputFormat: "tui",
	}

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{},
		Stats: analyzer.GraphStats{},
	}

	mockA := &mockAnalyzer{graph: graph}
	mockT := &mockTUI{runErr: io.EOF}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := run(cfg, logger, mockA, mockT)
	if err == nil {
		t.Error("run() expected error, got nil")
	}
}

// =============================================================================
// renderDebugView Tests
// =============================================================================

func TestRenderDebugView(t *testing.T) {
	createGraph := func() *analyzer.TemporalGraph {
		return &analyzer.TemporalGraph{
			Nodes: map[string]*analyzer.TemporalNode{
				"TestWorkflow": {
					Name:     "TestWorkflow",
					Type:     "workflow",
					FilePath: "test.go",
					Parents:  []string{},
					CallSites: []analyzer.CallSite{
						{TargetName: "TestActivity"},
					},
				},
				"TestActivity": {
					Name:     "TestActivity",
					Type:     "activity",
					FilePath: "test.go",
					Parents:  []string{"TestWorkflow"},
				},
			},
			Stats: analyzer.GraphStats{
				TotalWorkflows:  1,
				TotalActivities: 1,
			},
		}
	}

	tests := []struct {
		name          string
		debugView     string
		graph         *analyzer.TemporalGraph
		expectError   bool
		errorContains string
	}{
		{
			name:        "list view",
			debugView:   "list",
			graph:       createGraph(),
			expectError: false,
		},
		{
			name:        "tree view",
			debugView:   "tree",
			graph:       createGraph(),
			expectError: false,
		},
		{
			name:        "details view",
			debugView:   "details",
			graph:       createGraph(),
			expectError: false,
		},
		{
			name:        "stats view",
			debugView:   "stats",
			graph:       createGraph(),
			expectError: false,
		},
		{
			name:        "help view",
			debugView:   "help",
			graph:       createGraph(),
			expectError: false,
		},
		{
			name:          "unknown view",
			debugView:     "unknown",
			graph:         createGraph(),
			expectError:   true,
			errorContains: "unknown debug view",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				DebugView: tt.debugView,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := renderDebugView(cfg, tt.graph)

			// Restore stdout
			_ = w.Close()
			os.Stdout = oldStdout
			_, _ = io.Copy(io.Discard, r)

			if tt.expectError {
				if err == nil {
					t.Errorf("renderDebugView() expected error containing %q, got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("renderDebugView() error = %v, want error containing %q", err, tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("renderDebugView() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRenderDebugViewDetailsNoWorkflows(t *testing.T) {
	// Graph with only activities (no workflows)
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestActivity": {
				Name:     "TestActivity",
				Type:     "activity",
				FilePath: "test.go",
				Parents:  []string{},
			},
		},
		Stats: analyzer.GraphStats{
			TotalWorkflows:  0,
			TotalActivities: 1,
		},
	}

	cfg := &config.Config{
		DebugView: "details",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := renderDebugView(cfg, graph)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(io.Discard, r)

	if err != nil {
		t.Errorf("renderDebugView() unexpected error: %v", err)
	}
}

func TestRenderDebugViewEmptyGraph(t *testing.T) {
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{},
		Stats: analyzer.GraphStats{},
	}

	cfg := &config.Config{
		DebugView: "details",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := renderDebugView(cfg, graph)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(io.Discard, r)

	if err != nil {
		t.Errorf("renderDebugView() unexpected error: %v", err)
	}
}

func TestRenderDebugViewInitialItemsFallback(t *testing.T) {
	// Graph where no workflow has no parents (all activities)
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"Activity1": {
				Name:     "Activity1",
				Type:     "activity",
				FilePath: "test.go",
				Parents:  []string{},
			},
			"Activity2": {
				Name:     "Activity2",
				Type:     "activity",
				FilePath: "test.go",
				Parents:  []string{},
			},
		},
		Stats: analyzer.GraphStats{
			TotalWorkflows:  0,
			TotalActivities: 2,
		},
	}

	cfg := &config.Config{
		DebugView: "list",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := renderDebugView(cfg, graph)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(io.Discard, r)

	if err != nil {
		t.Errorf("renderDebugView() unexpected error: %v", err)
	}
}

// =============================================================================
// runLint Tests
// =============================================================================

func TestRunLint(t *testing.T) {
	// Create a temp directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		cfg          *config.Config
		graph        *analyzer.TemporalGraph
		analyzerErr  error
		expectedCode int
	}{
		{
			name: "successful lint with no issues",
			cfg: &config.Config{
				RootDir:         tempDir,
				LintMode:        true,
				LintFormat:      "text",
				LintStrict:      false,
				LintMinSeverity: "info",
				LintMaxFanOut:   15,
				LintMaxCallDepth: 10,
			},
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{},
				Stats: analyzer.GraphStats{},
			},
			expectedCode: 0,
		},
		{
			name: "lint with warnings in strict mode",
			cfg: &config.Config{
				RootDir:         tempDir,
				LintMode:        true,
				LintFormat:      "text",
				LintStrict:      true,
				LintMinSeverity: "info",
				LintMaxFanOut:   15,
				LintMaxCallDepth: 10,
			},
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"TestWorkflow": {
						Name:     "TestWorkflow",
						Type:     "workflow",
						FilePath: "test.go",
						CallSites: []analyzer.CallSite{
							{
								TargetName:         "TestActivity",
								CallType:           "activity",
								ParsedActivityOpts: nil, // No retry policy triggers warning
							},
						},
					},
					"TestActivity": {
						Name:     "TestActivity",
						Type:     "activity",
						FilePath: "test.go",
						Parents:  []string{"TestWorkflow"},
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows:  1,
					TotalActivities: 1,
				},
			},
			expectedCode: 1, // Warning in strict mode = failure
		},
		{
			name: "lint with JSON format",
			cfg: &config.Config{
				RootDir:         tempDir,
				LintMode:        true,
				LintFormat:      "json",
				LintStrict:      false,
				LintMinSeverity: "info",
				LintMaxFanOut:   15,
				LintMaxCallDepth: 10,
			},
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{},
				Stats: analyzer.GraphStats{},
			},
			expectedCode: 0,
		},
		{
			name: "lint with GitHub format",
			cfg: &config.Config{
				RootDir:         tempDir,
				LintMode:        true,
				LintFormat:      "github",
				LintStrict:      false,
				LintMinSeverity: "info",
				LintMaxFanOut:   15,
				LintMaxCallDepth: 10,
			},
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{},
				Stats: analyzer.GraphStats{},
			},
			expectedCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockA := &mockAnalyzer{
				graph: tt.graph,
				err:   tt.analyzerErr,
			}

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			code := runLint(tt.cfg, logger, mockA)

			// Restore stdout
			_ = w.Close()
			os.Stdout = oldStdout
			_, _ = io.Copy(io.Discard, r)

			if code != tt.expectedCode {
				t.Errorf("runLint() = %d, want %d", code, tt.expectedCode)
			}
		})
	}
}

func TestRunLintAnalyzerError(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		RootDir:         tempDir,
		LintMode:        true,
		LintFormat:      "text",
		LintStrict:      false,
		LintMinSeverity: "info",
		LintMaxFanOut:   15,
		LintMaxCallDepth: 10,
	}

	mockA := &mockAnalyzer{
		err: io.EOF,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := runLint(cfg, logger, mockA)

	// Restore stderr
	_ = w.Close()
	os.Stderr = oldStderr
	_, _ = io.Copy(io.Discard, r)

	if code != 2 {
		t.Errorf("runLint() with analyzer error = %d, want 2", code)
	}
}

func TestRunLintNilGraph(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		RootDir:         tempDir,
		LintMode:        true,
		LintFormat:      "text",
		LintStrict:      false,
		LintMinSeverity: "info",
		LintMaxFanOut:   15,
		LintMaxCallDepth: 10,
	}

	mockA := &mockAnalyzer{
		graph: nil,
		err:   nil,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := runLint(cfg, logger, mockA)

	// Restore stderr
	_ = w.Close()
	os.Stderr = oldStderr
	_, _ = io.Copy(io.Discard, r)

	if code != 2 {
		t.Errorf("runLint() with nil graph = %d, want 2", code)
	}
}

func TestRunLintWithOutputFile(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := tempDir + "/lint-output.txt"

	cfg := &config.Config{
		RootDir:         tempDir,
		LintMode:        true,
		LintFormat:      "text",
		LintStrict:      false,
		LintMinSeverity: "info",
		OutputFile:      outputFile,
		LintMaxFanOut:   15,
		LintMaxCallDepth: 10,
	}

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{},
		Stats: analyzer.GraphStats{},
	}

	mockA := &mockAnalyzer{graph: graph}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	code := runLint(cfg, logger, mockA)

	if code != 0 {
		t.Errorf("runLint() with output file = %d, want 0", code)
	}

	// Verify file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("runLint() did not create output file")
	}
}

func TestRunLintWithInvalidOutputFile(t *testing.T) {
	tempDir := t.TempDir()
	// Invalid path that cannot be created
	outputFile := tempDir + "/nonexistent/subdir/lint-output.txt"

	cfg := &config.Config{
		RootDir:         tempDir,
		LintMode:        true,
		LintFormat:      "text",
		LintStrict:      false,
		LintMinSeverity: "info",
		OutputFile:      outputFile,
		LintMaxFanOut:   15,
		LintMaxCallDepth: 10,
	}

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{},
		Stats: analyzer.GraphStats{},
	}

	mockA := &mockAnalyzer{graph: graph}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := runLint(cfg, logger, mockA)

	// Restore stderr
	_ = w.Close()
	os.Stderr = oldStderr
	_, _ = io.Copy(io.Discard, r)

	if code != 2 {
		t.Errorf("runLint() with invalid output file = %d, want 2", code)
	}
}

func TestRunLintDisabledRules(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		RootDir:           tempDir,
		LintMode:          true,
		LintFormat:        "text",
		LintStrict:        false,
		LintMinSeverity:   "info",
		LintDisabledRules: "TA001,TA002",
		LintMaxFanOut:     15,
		LintMaxCallDepth:  10,
	}

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestActivity": {
				Name:     "TestActivity",
				Type:     "activity",
				FilePath: "test.go",
				Parents:  []string{},
			},
		},
		Stats: analyzer.GraphStats{
			TotalActivities: 1,
		},
	}

	mockA := &mockAnalyzer{graph: graph}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runLint(cfg, logger, mockA)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(io.Discard, r)

	// Should pass because rules are disabled
	if code != 0 {
		t.Errorf("runLint() with disabled rules = %d, want 0", code)
	}
}

// =============================================================================
// listLintRules Tests
// =============================================================================

func TestListLintRules(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listLintRules()

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Check that the output contains expected content
	expectedContents := []string{
		"Temporal Analyzer - Available Lint Rules",
		"TA001",
		"TA002",
		"Usage:",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(output, expected) {
			t.Errorf("listLintRules() output does not contain %q", expected)
		}
	}
}

// =============================================================================
// Integration-style Tests
// =============================================================================

func TestRunWithDebugView(t *testing.T) {
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name:     "TestWorkflow",
				Type:     "workflow",
				FilePath: "test.go",
				Parents:  []string{},
			},
		},
		Stats: analyzer.GraphStats{
			TotalWorkflows: 1,
		},
	}

	cfg := &config.Config{
		RootDir:      ".",
		OutputFormat: "tui",
		DebugView:    "list",
	}

	mockA := &mockAnalyzer{graph: graph}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When DebugView is set, it should render and exit
	err := run(cfg, logger, mockA, nil)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(io.Discard, r)

	if err != nil {
		t.Errorf("run() with debug view unexpected error: %v", err)
	}
}

// =============================================================================
// ListItem Tests (for coverage of tui.ListItem usage in renderDebugView)
// =============================================================================

func TestListItemCreation(t *testing.T) {
	node := &analyzer.TemporalNode{
		Name:     "TestNode",
		Type:     "workflow",
		FilePath: "test.go",
	}

	item := tui.ListItem{Node: node}

	// Test list.Item interface implementation
	var listItem list.Item = item
	_ = listItem

	// Verify the node was set
	if item.Node.Name != "TestNode" {
		t.Errorf("ListItem.Node.Name = %q, want %q", item.Node.Name, "TestNode")
	}
}

// =============================================================================
// transformLintSubcommand Tests
// =============================================================================

func TestTransformLintSubcommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "no args",
			args:     []string{"temporal-analyzer"},
			expected: []string{"temporal-analyzer"},
		},
		{
			name:     "lint subcommand basic",
			args:     []string{"temporal-analyzer", "lint"},
			expected: []string{"temporal-analyzer", "--lint"},
		},
		{
			name:     "lint subcommand with path",
			args:     []string{"temporal-analyzer", "lint", "./..."},
			expected: []string{"temporal-analyzer", "--lint", "./..."},
		},
		{
			name:     "lint subcommand with --format=github-actions",
			args:     []string{"temporal-analyzer", "lint", "--format=github-actions", "./..."},
			expected: []string{"temporal-analyzer", "--lint", "--lint-format=github", "./..."},
		},
		{
			name:     "lint subcommand with -format=github-actions",
			args:     []string{"temporal-analyzer", "lint", "-format=github-actions", "./..."},
			expected: []string{"temporal-analyzer", "--lint", "--lint-format=github", "./..."},
		},
		{
			name:     "lint subcommand with --format=text",
			args:     []string{"temporal-analyzer", "lint", "--format=text", "./..."},
			expected: []string{"temporal-analyzer", "--lint", "--lint-format=text", "./..."},
		},
		{
			name:     "lint subcommand with --format json (space separated)",
			args:     []string{"temporal-analyzer", "lint", "--format", "json", "./..."},
			expected: []string{"temporal-analyzer", "--lint", "--lint-format", "json", "./..."},
		},
		{
			name:     "not a lint subcommand - regular flag usage",
			args:     []string{"temporal-analyzer", "--lint", "--lint-format=github", "./..."},
			expected: []string{"temporal-analyzer", "--lint", "--lint-format=github", "./..."},
		},
		{
			name:     "not a lint subcommand - other first arg",
			args:     []string{"temporal-analyzer", "./..."},
			expected: []string{"temporal-analyzer", "./..."},
		},
		{
			name:     "lint subcommand with multiple flags",
			args:     []string{"temporal-analyzer", "lint", "--format=github-actions", "--lint-strict", "./..."},
			expected: []string{"temporal-analyzer", "--lint", "--lint-format=github", "--lint-strict", "./..."},
		},
		{
			name:     "lint subcommand preserves other flags",
			args:     []string{"temporal-analyzer", "lint", "--verbose", "--debug", "./..."},
			expected: []string{"temporal-analyzer", "--lint", "--verbose", "--debug", "./..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformLintSubcommand(tt.args)
			if len(result) != len(tt.expected) {
				t.Errorf("transformLintSubcommand(%v) = %v, want %v", tt.args, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("transformLintSubcommand(%v)[%d] = %q, want %q", tt.args, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

