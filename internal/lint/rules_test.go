package lint

import (
	"context"
	"strings"
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

func TestSeverityLevel(t *testing.T) {
	tests := []struct {
		severity Severity
		want     int
	}{
		{SeverityError, 3},
		{SeverityWarning, 2},
		{SeverityInfo, 1},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := tt.severity.Level()
			if got != tt.want {
				t.Errorf("Level() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestActivityWithoutRetryRule(t *testing.T) {
	rule := &ActivityWithoutRetryRule{}

	if rule.ID() != "TA001" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA001")
	}
	if rule.Name() != "activity-without-retry" {
		t.Errorf("Name() = %q, want %q", rule.Name(), "activity-without-retry")
	}
	if rule.Category() != CategoryReliability {
		t.Errorf("Category() = %v, want %v", rule.Category(), CategoryReliability)
	}
	if rule.Severity() != SeverityWarning {
		t.Errorf("Severity() = %v, want %v", rule.Severity(), SeverityWarning)
	}

	// Test with activity without retry
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestActivity": {
				Name:         "TestActivity",
				Type:         "activity",
				ActivityOpts: nil,
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for activity without retry policy")
	}

	// Test with activity with retry
	graph.Nodes["TestActivity"].ActivityOpts = &analyzer.ActivityOptions{
		RetryPolicy: &analyzer.RetryPolicy{MaximumAttempts: 3},
	}
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report issue for activity with retry policy")
	}
}

func TestActivityWithoutTimeoutRule(t *testing.T) {
	rule := &ActivityWithoutTimeoutRule{}

	if rule.ID() != "TA002" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA002")
	}
	if rule.Severity() != SeverityError {
		t.Errorf("Severity() = %v, want %v", rule.Severity(), SeverityError)
	}

	// Test with activity without timeout
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestActivity": {
				Name:         "TestActivity",
				Type:         "activity",
				ActivityOpts: &analyzer.ActivityOptions{},
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for activity without timeout")
	}

	// Test with activity with timeout
	graph.Nodes["TestActivity"].ActivityOpts.StartToCloseTimeout = "5m"
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report issue for activity with timeout")
	}
}

func TestLongRunningActivityWithoutHeartbeatRule(t *testing.T) {
	rule := &LongRunningActivityWithoutHeartbeatRule{}

	if rule.ID() != "TA003" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA003")
	}

	// Test with long-running activity without heartbeat
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"ProcessBatchActivity": {
				Name:         "ProcessBatchActivity",
				Type:         "activity",
				ActivityOpts: &analyzer.ActivityOptions{},
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for long-running activity without heartbeat")
	}

	// Test with heartbeat configured
	graph.Nodes["ProcessBatchActivity"].ActivityOpts.HeartbeatTimeout = "30s"
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report issue for activity with heartbeat")
	}
}

func TestCircularDependencyRule(t *testing.T) {
	rule := &CircularDependencyRule{}

	if rule.ID() != "TA010" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA010")
	}
	if rule.Severity() != SeverityError {
		t.Errorf("Severity() = %v, want %v", rule.Severity(), SeverityError)
	}

	// Test with circular dependency
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"A": {Name: "A", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "B"}}},
			"B": {Name: "B", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "A"}}},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for circular dependency")
	}

	// Test without circular dependency
	graph = &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"A": {Name: "A", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "B"}}},
			"B": {Name: "B", Type: "workflow"},
		},
	}
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report issue without circular dependency")
	}
}

func TestOrphanNodeRule(t *testing.T) {
	rule := &OrphanNodeRule{}

	if rule.ID() != "TA011" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA011")
	}

	ctx := context.Background()

	// Activities are now skipped (may be called cross-repo)
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"OrphanActivity": {Name: "OrphanActivity", Type: "activity"},
		},
	}
	issues := rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Activities should be skipped (may be called from other repos)")
	}

	// Orphan signal handlers should still be reported (exported)
	graph = &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"OrphanSignalHandler": {Name: "OrphanSignalHandler", Type: "signal_handler"},
		},
	}
	issues = rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for orphan signal handler")
	}

	// Unexported (private) methods should be skipped
	graph = &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"privateHelper": {Name: "privateHelper", Type: "signal_handler"},
		},
	}
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Unexported methods should be skipped")
	}

	// Top-level workflows are not orphans
	graph = &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TopWorkflow": {Name: "TopWorkflow", Type: "workflow"},
		},
	}
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report top-level workflows as orphans")
	}
}

func TestHighFanOutRule(t *testing.T) {
	rule := NewHighFanOutRule(0) // Should use default threshold

	if rule.ID() != "TA020" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA020")
	}
	if rule.Threshold != 15 {
		t.Errorf("Threshold = %d, want 15 (default)", rule.Threshold)
	}

	// Test with custom threshold
	rule = NewHighFanOutRule(5)
	if rule.Threshold != 5 {
		t.Errorf("Threshold = %d, want 5", rule.Threshold)
	}

	// Test with high fan-out
	callSites := make([]analyzer.CallSite, 10)
	for i := range callSites {
		callSites[i] = analyzer.CallSite{TargetName: "Activity" + string(rune('A'+i))}
	}
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"HighFanOutWorkflow": {
				Name:      "HighFanOutWorkflow",
				Type:      "workflow",
				CallSites: callSites,
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for high fan-out")
	}
}

func TestDeepCallChainRule(t *testing.T) {
	rule := NewDeepCallChainRule(0) // Should use default

	if rule.ID() != "TA021" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA021")
	}
	if rule.MaxDepth != 10 {
		t.Errorf("MaxDepth = %d, want 10 (default)", rule.MaxDepth)
	}

	// Test with custom depth
	rule = NewDeepCallChainRule(3)
	if rule.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d, want 3", rule.MaxDepth)
	}

	// Create deep call chain
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"A": {Name: "A", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "B"}}},
			"B": {Name: "B", Type: "workflow", Parents: []string{"A"}, CallSites: []analyzer.CallSite{{TargetName: "C"}}},
			"C": {Name: "C", Type: "workflow", Parents: []string{"B"}, CallSites: []analyzer.CallSite{{TargetName: "D"}}},
			"D": {Name: "D", Type: "workflow", Parents: []string{"C"}, CallSites: []analyzer.CallSite{{TargetName: "E"}}},
			"E": {Name: "E", Type: "workflow", Parents: []string{"D"}},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for deep call chain")
	}
}

func TestWorkflowWithoutVersioningRule(t *testing.T) {
	rule := NewWorkflowWithoutVersioningRule(0) // Should use default

	if rule.ID() != "TA030" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA030")
	}
	if rule.ComplexityThreshold != 5 {
		t.Errorf("ComplexityThreshold = %d, want 5 (default)", rule.ComplexityThreshold)
	}

	// Test with custom threshold
	rule = NewWorkflowWithoutVersioningRule(3)

	// Create complex workflow without versioning
	callSites := make([]analyzer.CallSite, 5)
	for i := range callSites {
		callSites[i] = analyzer.CallSite{TargetName: "Activity" + string(rune('A'+i))}
	}
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"ComplexWorkflow": {
				Name:      "ComplexWorkflow",
				Type:      "workflow",
				CallSites: callSites,
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for complex workflow without versioning")
	}

	// Test with versioning
	graph.Nodes["ComplexWorkflow"].Versioning = []analyzer.VersionDef{{ChangeID: "v1"}}
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report issue for workflow with versioning")
	}
}

func TestSignalWithoutHandlerRule(t *testing.T) {
	rule := &SignalWithoutHandlerRule{}

	if rule.ID() != "TA031" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA031")
	}

	// Test with signal without handler
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name: "TestWorkflow",
				Type: "workflow",
				Signals: []analyzer.SignalDef{
					{Name: "cancelSignal", Handler: ""},
				},
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for signal without handler")
	}

	// Test with handler
	graph.Nodes["TestWorkflow"].Signals[0].Handler = "handleCancel"
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report issue for signal with handler")
	}
}

func TestQueryWithoutReturnRule(t *testing.T) {
	rule := &QueryWithoutReturnRule{}

	if rule.ID() != "TA032" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA032")
	}

	// Test with query without return type
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name: "TestWorkflow",
				Type: "workflow",
				Queries: []analyzer.QueryDef{
					{Name: "getStatus", ReturnType: ""},
				},
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for query without return type")
	}

	// Test with return type
	graph.Nodes["TestWorkflow"].Queries[0].ReturnType = "string"
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report issue for query with return type")
	}
}

func TestContinueAsNewWithoutConditionRule(t *testing.T) {
	rule := &ContinueAsNewWithoutConditionRule{}

	if rule.ID() != "TA033" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA033")
	}

	// Test with continue-as-new
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"TestWorkflow": {
				Name:          "TestWorkflow",
				Type:          "workflow",
				ContinueAsNew: &analyzer.ContinueAsNewDef{LineNumber: 10},
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) == 0 {
		t.Error("Expected issue for workflow with continue-as-new")
	}

	// Test without continue-as-new
	graph.Nodes["TestWorkflow"].ContinueAsNew = nil
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should not report issue for workflow without continue-as-new")
	}
}

func TestFindCircularDependencies(t *testing.T) {
	ctx := context.Background()

	// Test with cycle
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"A": {Name: "A", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "B"}}},
			"B": {Name: "B", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "C"}}},
			"C": {Name: "C", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "A"}}},
		},
	}

	cycles := findCircularDependencies(ctx, graph)
	if len(cycles) == 0 {
		t.Error("Expected to find circular dependency")
	}

	// Test without cycle
	graph = &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"A": {Name: "A", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "B"}}},
			"B": {Name: "B", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "C"}}},
			"C": {Name: "C", Type: "workflow"},
		},
	}

	cycles = findCircularDependencies(ctx, graph)
	if len(cycles) != 0 {
		t.Error("Should not find circular dependency")
	}
}

func TestCalculateChainDepth(t *testing.T) {
	ctx := context.Background()

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"A": {Name: "A", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "B"}}},
			"B": {Name: "B", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "C"}}},
			"C": {Name: "C", Type: "workflow", CallSites: []analyzer.CallSite{{TargetName: "D"}}},
			"D": {Name: "D", Type: "workflow"},
		},
	}

	depth := calculateChainDepth(ctx, graph.Nodes["A"], graph, make(map[string]bool))
	if depth != 3 {
		t.Errorf("calculateChainDepth = %d, want 3", depth)
	}
}

func TestIssueFields(t *testing.T) {
	issue := Issue{
		RuleID:      "TA001",
		RuleName:    "test-rule",
		Severity:    SeverityWarning,
		Category:    CategoryReliability,
		Message:     "Test message",
		Description: "Test description",
		Suggestion:  "Test suggestion",
		FilePath:    "test.go",
		LineNumber:  10,
		NodeName:    "TestNode",
		NodeType:    "workflow",
		Fix: &CodeFix{
			Description: "Fix description",
			Replacements: []Replacement{{
				FilePath:  "test.go",
				StartLine: 10,
				NewText:   "new code",
			}},
		},
	}

	if issue.RuleID != "TA001" {
		t.Errorf("RuleID = %q, want %q", issue.RuleID, "TA001")
	}
	if issue.Fix == nil {
		t.Error("Fix should not be nil")
	}
	if len(issue.Fix.Replacements) != 1 {
		t.Error("Expected 1 replacement")
	}
}

func TestArgumentsMismatchRule(t *testing.T) {
	rule := &ArgumentsMismatchRule{}

	if rule.ID() != "TA040" {
		t.Errorf("ID() = %q, want %q", rule.ID(), "TA040")
	}
	if rule.Name() != "arguments-mismatch" {
		t.Errorf("Name() = %q, want %q", rule.Name(), "arguments-mismatch")
	}
	if rule.Severity() != SeverityError {
		t.Errorf("Severity() = %v, want %v", rule.Severity(), SeverityError)
	}

	// Test with wrong argument count
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"MyWorkflow": {
				Name: "MyWorkflow",
				Type: "workflow",
				CallSites: []analyzer.CallSite{
					{
						TargetName:    "SendEmailActivity",
						TargetType:    "activity",
						ArgumentCount: 1, // Only passing 1 arg
						LineNumber:    10,
						FilePath:      "workflow.go",
					},
				},
			},
			"SendEmailActivity": {
				Name: "SendEmailActivity",
				Type: "activity",
				Parameters: map[string]string{
					"ctx":     "context.Context",
					"to":      "string",
					"subject": "string",
					"body":    "string",
				}, // Expects 3 args (excluding ctx)
			},
		},
	}

	ctx := context.Background()
	issues := rule.Check(ctx, graph)
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue for argument count mismatch, got %d", len(issues))
	}
	if len(issues) > 0 && issues[0].RuleID != "TA040" {
		t.Errorf("Issue RuleID = %q, want %q", issues[0].RuleID, "TA040")
	}

	// Test with correct argument count
	graph.Nodes["MyWorkflow"].CallSites[0].ArgumentCount = 3 // Correct: to, subject, body
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Errorf("Should not report issue for correct argument count, got %d", len(issues))
	}

	// Test with zero arguments (skip check)
	graph.Nodes["MyWorkflow"].CallSites[0].ArgumentCount = 0
	issues = rule.Check(ctx, graph)
	if len(issues) != 0 {
		t.Error("Should skip check when ArgumentCount is 0")
	}

	// Test return type mismatch
	graphWithReturnType := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"ProcessOrderWorkflow": {
				Name: "ProcessOrderWorkflow",
				Type: "workflow",
				CallSites: []analyzer.CallSite{
					{
						TargetName:    "CalculateTotalActivity",
						TargetType:    "activity",
						ArgumentCount: 1,
						ResultType:    "string", // Wrong type - activity returns int
						LineNumber:    25,
						FilePath:      "workflow.go",
					},
				},
			},
			"CalculateTotalActivity": {
				Name: "CalculateTotalActivity",
				Type: "activity",
				Parameters: map[string]string{
					"ctx":   "context.Context",
					"order": "Order",
				},
				ReturnType: "int", // Returns int, but caller expects string
			},
		},
	}

	issues = rule.Check(ctx, graphWithReturnType)
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue for return type mismatch, got %d", len(issues))
	}
	if len(issues) > 0 {
		if !strings.Contains(issues[0].Message, "result as 'string'") {
			t.Errorf("Expected message to mention wrong result type 'string', got: %s", issues[0].Message)
		}
		if !strings.Contains(issues[0].Message, "returns 'int'") {
			t.Errorf("Expected message to mention correct return type 'int', got: %s", issues[0].Message)
		}
	}

	// Test with matching return type (should not report issue)
	graphWithReturnType.Nodes["ProcessOrderWorkflow"].CallSites[0].ResultType = "int"
	issues = rule.Check(ctx, graphWithReturnType)
	if len(issues) != 0 {
		t.Errorf("Should not report issue for matching return type, got %d", len(issues))
	}

	// Test with unknown result type (skip check)
	graphWithReturnType.Nodes["ProcessOrderWorkflow"].CallSites[0].ResultType = "unknown"
	issues = rule.Check(ctx, graphWithReturnType)
	if len(issues) != 0 {
		t.Errorf("Should skip check when ResultType is 'unknown', got %d", len(issues))
	}

	// Test with var: prefix (cannot determine, skip check)
	graphWithReturnType.Nodes["ProcessOrderWorkflow"].CallSites[0].ResultType = "var:result"
	issues = rule.Check(ctx, graphWithReturnType)
	if len(issues) != 0 {
		t.Errorf("Should skip check when ResultType has 'var:' prefix (type unknown), got %d", len(issues))
	}
}

func TestCountNonContextParams(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]string
		want   int
	}{
		{
			name:   "empty params",
			params: map[string]string{},
			want:   0,
		},
		{
			name: "only context",
			params: map[string]string{
				"ctx": "context.Context",
			},
			want: 0,
		},
		{
			name: "workflow context only",
			params: map[string]string{
				"ctx": "workflow.Context",
			},
			want: 0,
		},
		{
			name: "context plus params",
			params: map[string]string{
				"ctx":    "context.Context",
				"input":  "string",
				"count":  "int",
			},
			want: 2,
		},
		{
			name: "no context",
			params: map[string]string{
				"input": "string",
				"count": "int",
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countNonContextParams(tt.params)
			if got != tt.want {
				t.Errorf("countNonContextParams() = %d, want %d", got, tt.want)
			}
		})
	}
}

