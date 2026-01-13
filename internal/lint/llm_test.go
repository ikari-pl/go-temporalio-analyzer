package lint

import (
	"context"
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

func TestNewLLMEnhancer_Disabled(t *testing.T) {
	// Test that LLM enhancer is disabled when no API key is provided
	cfg := &LLMConfig{
		APIKey:  "", // No API key
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
	}

	enhancer := NewLLMEnhancer(cfg)

	if enhancer.IsEnabled() {
		t.Error("expected LLM enhancer to be disabled without API key")
	}
}

func TestNewLLMEnhancer_DefaultConfig(t *testing.T) {
	// Test default config creation
	cfg := DefaultLLMConfig()

	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default base URL, got %s", cfg.BaseURL)
	}

	if cfg.Model != "gpt-4o-mini" {
		t.Errorf("expected default model gpt-4o-mini, got %s", cfg.Model)
	}
}

func TestNewLLMEnhancer_NilConfig(t *testing.T) {
	// Test that nil config uses defaults
	enhancer := NewLLMEnhancer(nil)

	// Should not panic and should be disabled (no API key in env during test)
	if enhancer == nil {
		t.Error("expected non-nil enhancer")
	}
}

func TestMCPClient_ReadFileRange(t *testing.T) {
	// Test MCP client file reading (works without language server)
	mcp := &MCPClient{enabled: false}

	// Reading should work even without MCP enabled (falls back to direct file read)
	_, err := mcp.ReadFileRange("/nonexistent/file.go", 1, 10)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestPatternExtractor_Empty(t *testing.T) {
	// Test pattern extraction with empty graph
	extractor := &PatternExtractor{}
	graph := &analyzer.TemporalGraph{
		Nodes: make(map[string]*analyzer.TemporalNode),
	}

	patterns := extractor.ExtractPatterns(graph)
	if patterns != "No existing patterns detected." {
		t.Errorf("expected no patterns message, got: %s", patterns)
	}
}

func TestPatternExtractor_WithPatterns(t *testing.T) {
	// Test pattern extraction with activity options
	extractor := &PatternExtractor{}
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name: "TestWorkflow",
				Type: "workflow",
				CallSites: []analyzer.CallSite{
					{
						TargetName: "MyActivity",
						ParsedActivityOpts: &analyzer.ActivityOptions{
							StartToCloseTimeout: "10m",
							RetryPolicy: &analyzer.RetryPolicy{
								MaximumAttempts: 3,
							},
						},
					},
				},
			},
		},
	}

	patterns := extractor.ExtractPatterns(graph)
	if patterns == "No existing patterns detected." {
		t.Error("expected patterns to be detected")
	}
}

func TestLLMEnhancer_EnhanceIssues_Disabled(t *testing.T) {
	// Test that EnhanceIssues returns original issues when disabled
	cfg := &LLMConfig{APIKey: ""} // Disabled
	enhancer := NewLLMEnhancer(cfg)

	issues := []Issue{
		{
			RuleID:  "TA001",
			Message: "test issue",
		},
	}

	graph := &analyzer.TemporalGraph{
		Nodes: make(map[string]*analyzer.TemporalNode),
	}

	validIssues, filteredIssues := enhancer.EnhanceIssues(context.Background(), issues, graph, true, true)

	if len(validIssues) != 1 {
		t.Errorf("expected 1 valid issue, got %d", len(validIssues))
	}

	if len(filteredIssues) != 0 {
		t.Errorf("expected 0 filtered issues, got %d", len(filteredIssues))
	}
}

func TestVerificationResult_JSON(t *testing.T) {
	// Test VerificationResult struct
	result := VerificationResult{
		Valid:      true,
		Confidence: "high",
		Reason:     "test reason",
		Suggestion: "test suggestion",
	}

	if !result.Valid {
		t.Error("expected Valid to be true")
	}
	if result.Confidence != "high" {
		t.Errorf("expected Confidence 'high', got %s", result.Confidence)
	}
}

func TestEnhancedFix_JSON(t *testing.T) {
	// Test EnhancedFix struct
	fix := EnhancedFix{
		Code:        "ctx = workflow.WithActivityOptions(ctx, ao)",
		Explanation: "Added activity options",
		Imports:     "time",
	}

	if fix.Code == "" {
		t.Error("expected Code to be set")
	}
	if fix.Explanation == "" {
		t.Error("expected Explanation to be set")
	}
}
