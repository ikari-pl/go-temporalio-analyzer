package lint

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		format string
		want   string // expected formatter type
	}{
		{"json", "*lint.JSONFormatter"},
		{"github", "*lint.GitHubFormatter"},
		{"sarif", "*lint.SARIFFormatter"},
		{"checkstyle", "*lint.CheckstyleFormatter"},
		{"text", "*lint.TextFormatter"},
		{"text-no-color", "*lint.TextFormatter"},
		{"", "*lint.TextFormatter"},
		{"unknown", "*lint.TextFormatter"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			f := NewFormatter(tt.format)
			if f == nil {
				t.Fatal("NewFormatter returned nil")
			}
		})
	}
}

func TestTextFormatter(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:     "TA001",
				RuleName:   "test-rule",
				Severity:   SeverityError,
				Message:    "Test error message",
				FilePath:   "test.go",
				LineNumber: 10,
				Suggestion: "Fix the error",
			},
			{
				RuleID:     "TA002",
				RuleName:   "warning-rule",
				Severity:   SeverityWarning,
				Message:    "Test warning message",
				FilePath:   "test.go",
				LineNumber: 20,
			},
		},
		ErrorCount: 1,
		WarnCount:  1,
	}

	f := &TextFormatter{Color: false}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Test error message") {
		t.Error("Output should contain error message")
	}
	if !strings.Contains(output, "Test warning message") {
		t.Error("Output should contain warning message")
	}
	if !strings.Contains(output, "test.go") {
		t.Error("Output should contain file path")
	}
}

func TestTextFormatterNoIssues(t *testing.T) {
	result := &Result{
		Issues: []Issue{},
	}

	f := &TextFormatter{Color: false}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No issues found") {
		t.Error("Output should indicate no issues")
	}
}

func TestTextFormatterWithColor(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:   "TA001",
				Severity: SeverityError,
				Message:  "Test message",
				FilePath: "test.go",
			},
		},
		ErrorCount: 1,
	}

	f := &TextFormatter{Color: true}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := buf.String()
	// Check for ANSI escape codes
	if !strings.Contains(output, "\033[") {
		t.Error("Color output should contain ANSI escape codes")
	}
}

func TestTextFormatterGeneralIssues(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:   "TA010",
				Severity: SeverityError,
				Message:  "Circular dependency",
				// No FilePath - general issue
			},
		},
		ErrorCount: 1,
	}

	f := &TextFormatter{Color: false}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "General Issues") {
		t.Error("Output should contain 'General Issues' section")
	}
}

func TestJSONFormatter(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:   "TA001",
				RuleName: "test-rule",
				Severity: SeverityError,
				Message:  "Test message",
				FilePath: "test.go",
			},
		},
		ErrorCount: 1,
		TotalNodes: 5,
		ExitCode:   1,
	}

	f := &JSONFormatter{}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify valid JSON
	var output JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if output.Version != "1.0" {
		t.Errorf("Version = %q, want %q", output.Version, "1.0")
	}
	if output.TotalNodes != 5 {
		t.Errorf("TotalNodes = %d, want 5", output.TotalNodes)
	}
	if len(output.Issues) != 1 {
		t.Errorf("Issues count = %d, want 1", len(output.Issues))
	}
	if output.Summary.Errors != 1 {
		t.Errorf("Summary.Errors = %d, want 1", output.Summary.Errors)
	}
}

func TestGitHubFormatter(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:      "TA001",
				RuleName:    "test-rule",
				Severity:    SeverityError,
				Message:     "Test error",
				Description: "Description of the issue",
				Suggestion:  "How to fix it",
				FilePath:    "test.go",
				LineNumber:  10,
			},
			{
				RuleID:     "TA002",
				RuleName:   "warning-rule",
				Severity:   SeverityWarning,
				Message:    "Test warning",
				FilePath:   "test.go",
				LineNumber: 20,
			},
			{
				RuleID:   "TA003",
				RuleName: "info-rule",
				Severity: SeverityInfo,
				Message:  "Test info",
			},
		},
		ErrorCount: 1,
		WarnCount:  1,
		InfoCount:  1,
	}

	f := &GitHubFormatter{}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := buf.String()

	// Check for GitHub workflow commands
	if !strings.Contains(output, "::error") {
		t.Error("Output should contain ::error command")
	}
	if !strings.Contains(output, "::warning") {
		t.Error("Output should contain ::warning command")
	}
	if !strings.Contains(output, "::notice") {
		t.Error("Output should contain ::notice command")
	}
	if !strings.Contains(output, "file=test.go") {
		t.Error("Output should contain file parameter")
	}
	if !strings.Contains(output, "line=10") {
		t.Error("Output should contain line parameter")
	}
	if !strings.Contains(output, "::group::") {
		t.Error("Output should contain group command for summary")
	}
}

func TestSARIFFormatter(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:      "TA001",
				RuleName:    "test-rule",
				Severity:    SeverityError,
				Category:    CategoryReliability,
				Message:     "Test message",
				Description: "Test description",
				FilePath:    "test.go",
				LineNumber:  10,
				Fix: &CodeFix{
					Description: "Fix the issue",
					Replacements: []Replacement{{
						FilePath:  "test.go",
						StartLine: 10,
						NewText:   "fixed code",
					}},
				},
			},
		},
		ErrorCount: 1,
	}

	f := &SARIFFormatter{}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify valid JSON
	var report SARIFReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("Invalid SARIF JSON: %v", err)
	}

	if report.Version != "2.1.0" {
		t.Errorf("SARIF version = %q, want %q", report.Version, "2.1.0")
	}
	if len(report.Runs) != 1 {
		t.Fatalf("Runs count = %d, want 1", len(report.Runs))
	}

	run := report.Runs[0]
	if run.Tool.Driver.Name != "temporal-analyzer" {
		t.Errorf("Tool name = %q, want %q", run.Tool.Driver.Name, "temporal-analyzer")
	}
	if len(run.Results) != 1 {
		t.Errorf("Results count = %d, want 1", len(run.Results))
	}
	if len(run.Results[0].Fixes) == 0 {
		t.Error("Expected fixes in result")
	}
}

func TestCheckstyleFormatter(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:     "TA001",
				Severity:   SeverityError,
				Message:    "Test error",
				FilePath:   "test.go",
				LineNumber: 10,
			},
			{
				RuleID:     "TA002",
				Severity:   SeverityWarning,
				Message:    "Test warning",
				FilePath:   "other.go",
				LineNumber: 20,
			},
			{
				RuleID:   "TA003",
				Severity: SeverityInfo,
				Message:  "General issue",
				// No FilePath
			},
		},
	}

	f := &CheckstyleFormatter{}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := buf.String()

	// Check for XML structure
	if !strings.Contains(output, `<?xml version="1.0"`) {
		t.Error("Output should contain XML declaration")
	}
	if !strings.Contains(output, "<checkstyle") {
		t.Error("Output should contain checkstyle element")
	}
	if !strings.Contains(output, `<file name="test.go">`) {
		t.Error("Output should contain file element")
	}
	if !strings.Contains(output, `severity="error"`) {
		t.Error("Output should contain error severity")
	}
	if !strings.Contains(output, `severity="warning"`) {
		t.Error("Output should contain warning severity")
	}
	if !strings.Contains(output, `<file name="general">`) {
		t.Error("Output should contain general file for issues without path")
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"<script>", "&lt;script&gt;"},
		{"a & b", "a &amp; b"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"it's", "it&apos;s"},
		{"<a & b>", "&lt;a &amp; b&gt;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeXML(tt.input)
			if got != tt.want {
				t.Errorf("escapeXML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTextFormatterAllSeverities(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:   "TA001",
				Severity: SeverityError,
				Message:  "Error",
				FilePath: "test.go",
			},
			{
				RuleID:   "TA002",
				Severity: SeverityWarning,
				Message:  "Warning",
				FilePath: "test.go",
			},
			{
				RuleID:   "TA003",
				Severity: SeverityInfo,
				Message:  "Info",
				FilePath: "test.go",
			},
		},
		ErrorCount: 1,
		WarnCount:  1,
		InfoCount:  1,
	}

	f := &TextFormatter{Color: false}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1 error(s)") {
		t.Error("Output should contain error count")
	}
	if !strings.Contains(output, "1 warning(s)") {
		t.Error("Output should contain warning count")
	}
	if !strings.Contains(output, "1 info") {
		t.Error("Output should contain info count")
	}
}

func TestSARIFFormatterMultipleRules(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:   "TA001",
				RuleName: "rule-one",
				Severity: SeverityError,
				Category: CategoryReliability,
				Message:  "First issue",
			},
			{
				RuleID:   "TA001",
				RuleName: "rule-one",
				Severity: SeverityError,
				Category: CategoryReliability,
				Message:  "Second issue (same rule)",
			},
			{
				RuleID:   "TA002",
				RuleName: "rule-two",
				Severity: SeverityWarning,
				Category: CategoryPerformance,
				Message:  "Third issue",
			},
		},
	}

	f := &SARIFFormatter{}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	var report SARIFReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("Invalid SARIF JSON: %v", err)
	}

	// Should have 2 unique rules
	if len(report.Runs[0].Tool.Driver.Rules) != 2 {
		t.Errorf("Rules count = %d, want 2", len(report.Runs[0].Tool.Driver.Rules))
	}

	// Should have 3 results
	if len(report.Runs[0].Results) != 3 {
		t.Errorf("Results count = %d, want 3", len(report.Runs[0].Results))
	}
}

func TestCheckstyleFormatterLineZero(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{
				RuleID:     "TA001",
				Severity:   SeverityError,
				Message:    "Test",
				FilePath:   "test.go",
				LineNumber: 0, // Zero line number
			},
		},
	}

	f := &CheckstyleFormatter{}
	var buf bytes.Buffer
	err := f.Format(result, &buf)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := buf.String()
	// Should default to line 1
	if !strings.Contains(output, `line="1"`) {
		t.Error("Zero line number should default to 1")
	}
}

