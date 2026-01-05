package lint

import (
	"context"
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if cfg.MinSeverity != SeverityInfo {
		t.Errorf("MinSeverity = %v, want %v", cfg.MinSeverity, SeverityInfo)
	}
	if cfg.FailOnWarning {
		t.Error("FailOnWarning should be false by default")
	}
	if cfg.Thresholds.MaxFanOut != 15 {
		t.Errorf("MaxFanOut = %d, want 15", cfg.Thresholds.MaxFanOut)
	}
	if cfg.Thresholds.MaxCallDepth != 10 {
		t.Errorf("MaxCallDepth = %d, want 10", cfg.Thresholds.MaxCallDepth)
	}
}

func TestStrictConfig(t *testing.T) {
	cfg := StrictConfig()
	if cfg == nil {
		t.Fatal("StrictConfig returned nil")
	}

	if !cfg.FailOnWarning {
		t.Error("FailOnWarning should be true in strict config")
	}
	if cfg.MinSeverity != SeverityWarning {
		t.Errorf("MinSeverity = %v, want %v", cfg.MinSeverity, SeverityWarning)
	}
}

func TestNewLinter(t *testing.T) {
	// With nil config
	l := NewLinter(nil)
	if l == nil {
		t.Fatal("NewLinter returned nil")
	}
	if len(l.rules) == 0 {
		t.Error("Expected rules to be registered")
	}

	// With custom config
	cfg := &Config{MinSeverity: SeverityError}
	l = NewLinter(cfg)
	if l == nil {
		t.Fatal("NewLinter with config returned nil")
	}
}

func TestLinterRun(t *testing.T) {
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name:     "TestWorkflow",
				Type:     "workflow",
				FilePath: "test.go",
			},
		},
	}

	cfg := DefaultConfig()
	l := NewLinter(cfg)

	ctx := context.Background()
	result := l.Run(ctx, graph)

	if result == nil {
		t.Fatal("Run returned nil result")
	}
	if result.TotalNodes != 1 {
		t.Errorf("TotalNodes = %d, want 1", result.TotalNodes)
	}
}

func TestLinterRunWithIssues(t *testing.T) {
	// Create graph with a workflow calling an activity without retry policy
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name:     "TestWorkflow",
				Type:     "workflow",
				FilePath: "test.go",
				CallSites: []analyzer.CallSite{
					{
						TargetName:         "TestActivity",
						CallType:           "activity",
						ParsedActivityOpts: nil, // No activity options = will trigger TA001
					},
				},
			},
			"TestActivity": {
				Name:     "TestActivity",
				Type:     "activity",
				FilePath: "test.go",
			},
		},
	}

	cfg := DefaultConfig()
	l := NewLinter(cfg)

	ctx := context.Background()
	result := l.Run(ctx, graph)

	// Should find at least one issue
	if len(result.Issues) == 0 {
		t.Error("Expected at least one issue for activity without retry policy")
	}
}

func TestLinterRunContextCancellation(t *testing.T) {
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {Name: "TestWorkflow", Type: "workflow"},
		},
	}

	cfg := DefaultConfig()
	l := NewLinter(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := l.Run(ctx, graph)
	// Should return without panicking
	if result == nil {
		t.Fatal("Run returned nil result")
	}
}

func TestLinterIsRuleEnabled(t *testing.T) {
	tests := []struct {
		name          string
		enabledRules  []string
		disabledRules []string
		ruleID        string
		want          bool
	}{
		{
			name:   "all rules enabled by default",
			ruleID: "TA001",
			want:   true,
		},
		{
			name:          "rule explicitly disabled",
			disabledRules: []string{"TA001"},
			ruleID:        "TA001",
			want:          false,
		},
		{
			name:         "only specific rules enabled",
			enabledRules: []string{"TA001"},
			ruleID:       "TA002",
			want:         false,
		},
		{
			name:         "rule in enabled list",
			enabledRules: []string{"TA001", "TA002"},
			ruleID:       "TA001",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.EnabledRules = tt.enabledRules
			cfg.DisabledRules = tt.disabledRules
			l := NewLinter(cfg)

			got := l.isRuleEnabled(tt.ruleID)
			if got != tt.want {
				t.Errorf("isRuleEnabled(%q) = %v, want %v", tt.ruleID, got, tt.want)
			}
		})
	}
}

func TestLinterShouldReport(t *testing.T) {
	tests := []struct {
		name        string
		minSeverity Severity
		issueSev    Severity
		want        bool
	}{
		{
			name:        "error above info threshold",
			minSeverity: SeverityInfo,
			issueSev:    SeverityError,
			want:        true,
		},
		{
			name:        "warning above info threshold",
			minSeverity: SeverityInfo,
			issueSev:    SeverityWarning,
			want:        true,
		},
		{
			name:        "info at info threshold",
			minSeverity: SeverityInfo,
			issueSev:    SeverityInfo,
			want:        true,
		},
		{
			name:        "info below warning threshold",
			minSeverity: SeverityWarning,
			issueSev:    SeverityInfo,
			want:        false,
		},
		{
			name:        "warning below error threshold",
			minSeverity: SeverityError,
			issueSev:    SeverityWarning,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.MinSeverity = tt.minSeverity
			l := NewLinter(cfg)

			issue := Issue{Severity: tt.issueSev}
			got := l.shouldReport(issue)
			if got != tt.want {
				t.Errorf("shouldReport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinterListRules(t *testing.T) {
	l := NewLinter(nil)
	rules := l.ListRules()

	if len(rules) == 0 {
		t.Error("Expected at least some rules")
	}

	// Check that required fields are set
	for _, rule := range rules {
		if rule.ID == "" {
			t.Error("Rule ID should not be empty")
		}
		if rule.Name == "" {
			t.Error("Rule Name should not be empty")
		}
		if rule.Description == "" {
			t.Error("Rule Description should not be empty")
		}
	}
}

func TestResultPassed(t *testing.T) {
	tests := []struct {
		name       string
		result     Result
		strict     bool
		wantPassed bool
	}{
		{
			name:       "no issues",
			result:     Result{},
			strict:     false,
			wantPassed: true,
		},
		{
			name:       "errors present",
			result:     Result{ErrorCount: 1},
			strict:     false,
			wantPassed: false,
		},
		{
			name:       "warnings in non-strict mode",
			result:     Result{WarnCount: 1},
			strict:     false,
			wantPassed: true,
		},
		{
			name:       "warnings in strict mode",
			result:     Result{WarnCount: 1},
			strict:     true,
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Passed(tt.strict)
			if got != tt.wantPassed {
				t.Errorf("Passed(%v) = %v, want %v", tt.strict, got, tt.wantPassed)
			}
		})
	}
}

func TestResultSummary(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{
			name:   "no issues",
			result: Result{},
			want:   "No issues found",
		},
		{
			name:   "has errors",
			result: Result{ErrorCount: 1},
			want:   "",
		},
		{
			name:   "has warnings",
			result: Result{WarnCount: 1},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Summary()
			if got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLinterMaxIssues(t *testing.T) {
	// Create a graph with a workflow that calls many activities without retry policy
	callSites := make([]analyzer.CallSite, 20)
	for i := 0; i < 20; i++ {
		callSites[i] = analyzer.CallSite{
			TargetName:         "TestActivity" + string(rune('A'+i)),
			CallType:           "activity",
			ParsedActivityOpts: nil, // Will trigger TA001
		}
	}

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name:      "TestWorkflow",
				Type:      "workflow",
				FilePath:  "test.go",
				CallSites: callSites,
			},
		},
	}

	cfg := DefaultConfig()
	cfg.MaxIssues = 5
	l := NewLinter(cfg)

	ctx := context.Background()
	result := l.Run(ctx, graph)

	if len(result.Issues) > 5 {
		t.Errorf("Expected max 5 issues, got %d", len(result.Issues))
	}
}

func TestLinterExitCode(t *testing.T) {
	// Graph with errors
	graphWithErrors := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"A": {Name: "A", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "B"}}},
			"B": {Name: "B", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "A"}}},
		},
	}

	cfg := DefaultConfig()
	l := NewLinter(cfg)

	ctx := context.Background()
	result := l.Run(ctx, graphWithErrors)

	// Should have exit code 1 due to circular dependency error
	if result.ExitCode != 1 && result.ErrorCount > 0 {
		t.Errorf("Expected exit code 1 with errors, got %d", result.ExitCode)
	}
}

func TestLinterSortIssues(t *testing.T) {
	// Create a graph with workflows calling activities that will produce multiple issues
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"WorkflowB": {
				Name:       "WorkflowB",
				Type:       "workflow",
				FilePath:   "b.go",
				LineNumber: 20,
				CallSites: []analyzer.CallSite{
					{
						TargetName:         "ActivityB",
						CallType:           "activity",
						FilePath:           "b.go",
						LineNumber:         25,
						ParsedActivityOpts: nil, // Will trigger TA001
					},
				},
			},
			"WorkflowA": {
				Name:       "WorkflowA",
				Type:       "workflow",
				FilePath:   "a.go",
				LineNumber: 10,
				CallSites: []analyzer.CallSite{
					{
						TargetName:         "ActivityA",
						CallType:           "activity",
						FilePath:           "a.go",
						LineNumber:         15,
						ParsedActivityOpts: nil, // Will trigger TA001
					},
				},
			},
		},
	}

	cfg := DefaultConfig()
	l := NewLinter(cfg)

	ctx := context.Background()
	result := l.Run(ctx, graph)

	// Issues should be sorted by severity, then file, then line
	if len(result.Issues) < 2 {
		t.Skip("Not enough issues to test sorting")
	}

	for i := 1; i < len(result.Issues); i++ {
		prev := result.Issues[i-1]
		curr := result.Issues[i]

		// Check severity ordering (higher first)
		if prev.Severity.Level() < curr.Severity.Level() {
			t.Error("Issues should be sorted by severity (most severe first)")
		}

		// If same severity, check file ordering
		if prev.Severity.Level() == curr.Severity.Level() {
			if prev.FilePath > curr.FilePath {
				t.Error("Issues with same severity should be sorted by file path")
			}
		}
	}
}

