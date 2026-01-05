// Package lint provides linting rules for Temporal.io workflow analysis.
// It is designed for CI/CD integration, providing configurable rules and
// multiple output formats.
package lint

import (
	"context"
	"fmt"
	"strings"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

// Severity represents the severity level of a lint issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// SeverityLevel returns the numeric level (higher = more severe)
func (s Severity) Level() int {
	switch s {
	case SeverityError:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// Category represents the category of a lint rule.
type Category string

const (
	CategoryBestPractice Category = "best-practice"
	CategoryReliability  Category = "reliability"
	CategoryPerformance  Category = "performance"
	CategoryMaintenance  Category = "maintenance"
	CategorySecurity     Category = "security"
)

// Issue represents a lint issue found in the codebase.
type Issue struct {
	RuleID      string   `json:"ruleId"`
	RuleName    string   `json:"ruleName"`
	Severity    Severity `json:"severity"`
	Category    Category `json:"category"`
	Message     string   `json:"message"`
	Description string   `json:"description,omitempty"`
	Suggestion  string   `json:"suggestion,omitempty"`
	FilePath    string   `json:"filePath,omitempty"`
	LineNumber  int      `json:"lineNumber,omitempty"`
	EndLine     int      `json:"endLine,omitempty"`
	NodeName    string   `json:"nodeName,omitempty"`
	NodeType    string   `json:"nodeType,omitempty"`
	// Fix contains a suggested code fix that can be applied automatically
	Fix *CodeFix `json:"fix,omitempty"`
}

// CodeFix represents a suggested code change to fix an issue.
type CodeFix struct {
	// Description explains what the fix does
	Description string `json:"description"`
	// Replacements contains the text replacements to apply
	Replacements []Replacement `json:"replacements"`
}

// Replacement represents a single text replacement in a file.
type Replacement struct {
	FilePath  string `json:"filePath"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine,omitempty"`
	// OldText is the text to be replaced (for verification)
	OldText string `json:"oldText,omitempty"`
	// NewText is the replacement text
	NewText string `json:"newText"`
}

// Rule defines a lint rule interface.
type Rule interface {
	// ID returns the unique identifier for this rule (e.g., "TA001")
	ID() string
	// Name returns the human-readable name of the rule
	Name() string
	// Category returns the category of this rule
	Category() Category
	// Severity returns the default severity of this rule
	Severity() Severity
	// Description returns a detailed description of what this rule checks
	Description() string
	// Check executes the rule against the graph and returns any issues found
	Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue
}

// =============================================================================
// Best Practice Rules
// =============================================================================

// ActivityWithoutRetryRule checks for activities without retry policies.
type ActivityWithoutRetryRule struct{}

func (r *ActivityWithoutRetryRule) ID() string         { return "TA001" }
func (r *ActivityWithoutRetryRule) Name() string       { return "activity-without-retry" }
func (r *ActivityWithoutRetryRule) Category() Category { return CategoryReliability }
func (r *ActivityWithoutRetryRule) Severity() Severity { return SeverityWarning }
func (r *ActivityWithoutRetryRule) Description() string {
	return "Network blips, service restarts, and temporary unavailability are common. Without retry policies, every transient error becomes a workflow failure requiring manual intervention."
}

func (r *ActivityWithoutRetryRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if node.Type != "activity" {
			continue
		}
		if node.ActivityOpts == nil || node.ActivityOpts.RetryPolicy == nil {
			issues = append(issues, Issue{
				RuleID:      r.ID(),
				RuleName:    r.Name(),
				Severity:    r.Severity(),
				Category:    r.Category(),
				Message:     fmt.Sprintf("Activity '%s' has no retry policy configured", node.Name),
				Description: r.Description(),
				Suggestion:  "Add a RetryPolicy to activity options for transient failure resilience",
				FilePath:    node.FilePath,
				LineNumber:  node.LineNumber,
				NodeName:    node.Name,
				NodeType:    node.Type,
				Fix: &CodeFix{
					Description: "Add retry policy to activity options",
					Replacements: []Replacement{{
						FilePath:  node.FilePath,
						StartLine: node.LineNumber,
						NewText: `ao := workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
		MaximumAttempts:    3,
	},
}
ctx = workflow.WithActivityOptions(ctx, ao)`,
					}},
				},
			})
		}
	}
	return issues
}

// ActivityWithoutTimeoutRule checks for activities without timeout configuration.
type ActivityWithoutTimeoutRule struct{}

func (r *ActivityWithoutTimeoutRule) ID() string         { return "TA002" }
func (r *ActivityWithoutTimeoutRule) Name() string       { return "activity-without-timeout" }
func (r *ActivityWithoutTimeoutRule) Category() Category { return CategoryReliability }
func (r *ActivityWithoutTimeoutRule) Severity() Severity { return SeverityError }
func (r *ActivityWithoutTimeoutRule) Description() string {
	return "Activities can hang forever due to deadlocked connections, infinite loops, or unresponsive dependencies. Without timeouts, workflows get stuck permanently, consuming resources and blocking business processes."
}

func (r *ActivityWithoutTimeoutRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if node.Type != "activity" {
			continue
		}
		if node.ActivityOpts == nil {
			continue // Handled by parent workflow options
		}
		opts := node.ActivityOpts
		hasTimeout := opts.StartToCloseTimeout != "" ||
			opts.ScheduleToCloseTimeout != "" ||
			opts.ScheduleToStartTimeout != ""
		if !hasTimeout {
			issues = append(issues, Issue{
				RuleID:      r.ID(),
				RuleName:    r.Name(),
				Severity:    r.Severity(),
				Category:    r.Category(),
				Message:     fmt.Sprintf("Activity '%s' has no timeout configured", node.Name),
				Description: r.Description(),
				Suggestion:  "Add StartToCloseTimeout or ScheduleToCloseTimeout to activity options",
				FilePath:    node.FilePath,
				LineNumber:  node.LineNumber,
				NodeName:    node.Name,
				NodeType:    node.Type,
				Fix: &CodeFix{
					Description: "Add timeout to activity options",
					Replacements: []Replacement{{
						FilePath:  node.FilePath,
						StartLine: node.LineNumber,
						NewText: `ao := workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Minute,
}
ctx = workflow.WithActivityOptions(ctx, ao)`,
					}},
				},
			})
		}
	}
	return issues
}

// LongRunningActivityWithoutHeartbeatRule checks for potentially long-running activities without heartbeat.
type LongRunningActivityWithoutHeartbeatRule struct{}

func (r *LongRunningActivityWithoutHeartbeatRule) ID() string { return "TA003" }
func (r *LongRunningActivityWithoutHeartbeatRule) Name() string {
	return "long-activity-without-heartbeat"
}
func (r *LongRunningActivityWithoutHeartbeatRule) Category() Category { return CategoryReliability }
func (r *LongRunningActivityWithoutHeartbeatRule) Severity() Severity { return SeverityWarning }
func (r *LongRunningActivityWithoutHeartbeatRule) Description() string {
	return "Long-running activities should have heartbeats. Without them, if a worker dies (OOMKill, scale-down, SIGKILL), Temporal must wait for the full timeout before retrying. Heartbeats enable fast failure detection."
}

func (r *LongRunningActivityWithoutHeartbeatRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if node.Type != "activity" {
			continue
		}
		if node.ActivityOpts == nil {
			continue
		}
		// Check if activity appears to be long-running (has long timeout or named suggestively)
		isLongRunning := strings.Contains(strings.ToLower(node.Name), "process") ||
			strings.Contains(strings.ToLower(node.Name), "batch") ||
			strings.Contains(strings.ToLower(node.Name), "sync") ||
			strings.Contains(strings.ToLower(node.Name), "import") ||
			strings.Contains(strings.ToLower(node.Name), "export") ||
			strings.Contains(strings.ToLower(node.Name), "migrate")

		if isLongRunning && node.ActivityOpts.HeartbeatTimeout == "" {
			issues = append(issues, Issue{
				RuleID:      r.ID(),
				RuleName:    r.Name(),
				Severity:    r.Severity(),
				Category:    r.Category(),
				Message:     fmt.Sprintf("Potentially long-running activity '%s' has no heartbeat configured", node.Name),
				Description: r.Description(),
				Suggestion:  "Add HeartbeatTimeout and call activity.RecordHeartbeat(ctx, progress) periodically in the activity implementation",
				FilePath:    node.FilePath,
				LineNumber:  node.LineNumber,
				NodeName:    node.Name,
				NodeType:    node.Type,
				Fix: &CodeFix{
					Description: "Add heartbeat timeout to activity options",
					Replacements: []Replacement{{
						FilePath:  node.FilePath,
						StartLine: node.LineNumber,
						NewText: `ao := workflow.ActivityOptions{
	StartToCloseTimeout: 30 * time.Minute,
	HeartbeatTimeout:    30 * time.Second,
}
ctx = workflow.WithActivityOptions(ctx, ao)`,
					}},
				},
			})
		}
	}
	return issues
}

// =============================================================================
// Reliability Rules
// =============================================================================

// CircularDependencyRule checks for circular dependencies in the workflow graph.
type CircularDependencyRule struct{}

func (r *CircularDependencyRule) ID() string         { return "TA010" }
func (r *CircularDependencyRule) Name() string       { return "circular-dependency" }
func (r *CircularDependencyRule) Category() Category { return CategoryReliability }
func (r *CircularDependencyRule) Severity() Severity { return SeverityError }
func (r *CircularDependencyRule) Description() string {
	return "Workflow A waiting for B while B waits for A creates a deadlock that never resolves. These are hard to debug in production, waste resources, and can cascade into system-wide issues."
}

func (r *CircularDependencyRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	cycles := findCircularDependencies(ctx, graph)
	for _, cycle := range cycles {
		issues = append(issues, Issue{
			RuleID:      r.ID(),
			RuleName:    r.Name(),
			Severity:    r.Severity(),
			Category:    r.Category(),
			Message:     fmt.Sprintf("Circular dependency detected: %s", cycle),
			Description: r.Description(),
			Suggestion:  "Refactor to eliminate circular dependencies, consider using signals or child workflows",
		})
	}
	return issues
}

// OrphanNodeRule checks for nodes that are never called.
type OrphanNodeRule struct{}

func (r *OrphanNodeRule) ID() string         { return "TA011" }
func (r *OrphanNodeRule) Name() string       { return "orphan-node" }
func (r *OrphanNodeRule) Category() Category { return CategoryMaintenance }
func (r *OrphanNodeRule) Severity() Severity { return SeverityWarning }
func (r *OrphanNodeRule) Description() string {
	return "Unused workflows/activities add maintenance burden, confuse developers, and may indicate incomplete migrations or forgotten features. Dead code should be removed to keep the codebase clean."
}

func (r *OrphanNodeRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		// Skip top-level workflows (they're entry points, expected to have no parents)
		if node.Type == "workflow" && len(node.Parents) == 0 {
			continue
		}

		// Skip activities entirely - they may be called from other repositories
		// or registered dynamically with workers. This check produces too many
		// false positives for activities.
		if node.Type == "activity" {
			continue
		}

		// Extract the method/function name (after the last dot if qualified)
		name := node.Name
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}

		// Skip unexported (private) methods - these are likely helper methods,
		// not meant to be called as activities/workflows from outside
		if len(name) > 0 && name[0] >= 'a' && name[0] <= 'z' {
			continue
		}

		// Check if node is never called (has no parents)
		if len(node.Parents) == 0 {
			issues = append(issues, Issue{
				RuleID:      r.ID(),
				RuleName:    r.Name(),
				Severity:    r.Severity(),
				Category:    r.Category(),
				Message:     fmt.Sprintf("%s '%s' appears to be unused (never called)", node.Type, node.Name),
				Description: r.Description(),
				Suggestion:  "Consider removing unused code, or verify it's called from another repository or registered with a worker",
				FilePath:    node.FilePath,
				LineNumber:  node.LineNumber,
				NodeName:    node.Name,
				NodeType:    node.Type,
			})
		}
	}
	return issues
}

// =============================================================================
// Performance Rules
// =============================================================================

// HighFanOutRule checks for workflows with too many direct calls.
type HighFanOutRule struct {
	Threshold int
}

func NewHighFanOutRule(threshold int) *HighFanOutRule {
	if threshold <= 0 {
		threshold = 15 // Default
	}
	return &HighFanOutRule{Threshold: threshold}
}

func (r *HighFanOutRule) ID() string         { return "TA020" }
func (r *HighFanOutRule) Name() string       { return "high-fan-out" }
func (r *HighFanOutRule) Category() Category { return CategoryPerformance }
func (r *HighFanOutRule) Severity() Severity { return SeverityWarning }
func (r *HighFanOutRule) Description() string {
	return "High fan-out creates blast radius issues: one change affects many dependencies. It makes testing harder, increases coupling, and often indicates a missing abstraction layer or orchestration pattern."
}

func (r *HighFanOutRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if len(node.CallSites) > r.Threshold {
			issues = append(issues, Issue{
				RuleID:      r.ID(),
				RuleName:    r.Name(),
				Severity:    r.Severity(),
				Category:    r.Category(),
				Message:     fmt.Sprintf("%s '%s' has %d direct calls (threshold: %d)", node.Type, node.Name, len(node.CallSites), r.Threshold),
				Description: r.Description(),
				Suggestion:  "Consider breaking down into smaller, more focused workflows or using sub-workflows",
				FilePath:    node.FilePath,
				LineNumber:  node.LineNumber,
				NodeName:    node.Name,
				NodeType:    node.Type,
			})
		}
	}
	return issues
}

// DeepCallChainRule checks for excessively deep call chains.
type DeepCallChainRule struct {
	MaxDepth int
}

func NewDeepCallChainRule(maxDepth int) *DeepCallChainRule {
	if maxDepth <= 0 {
		maxDepth = 10 // Default
	}
	return &DeepCallChainRule{MaxDepth: maxDepth}
}

func (r *DeepCallChainRule) ID() string         { return "TA021" }
func (r *DeepCallChainRule) Name() string       { return "deep-call-chain" }
func (r *DeepCallChainRule) Category() Category { return CategoryPerformance }
func (r *DeepCallChainRule) Severity() Severity { return SeverityWarning }
func (r *DeepCallChainRule) Description() string {
	return "Deep call chains make stack traces hard to read, increase end-to-end latency, and make it difficult to understand the business flow. Consider flattening or using child workflows for clarity."
}

func (r *DeepCallChainRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if len(node.Parents) == 0 { // Root node
			depth := calculateChainDepth(ctx, node, graph, make(map[string]bool))
			if depth > r.MaxDepth {
				issues = append(issues, Issue{
					RuleID:      r.ID(),
					RuleName:    r.Name(),
					Severity:    r.Severity(),
					Category:    r.Category(),
					Message:     fmt.Sprintf("Call chain starting from '%s' has depth %d (max: %d)", node.Name, depth, r.MaxDepth),
					Description: r.Description(),
					Suggestion:  "Consider flattening the workflow structure or using child workflows strategically",
					FilePath:    node.FilePath,
					LineNumber:  node.LineNumber,
					NodeName:    node.Name,
					NodeType:    node.Type,
				})
			}
		}
	}
	return issues
}

// =============================================================================
// Maintenance Rules
// =============================================================================

// WorkflowWithoutVersioningRule checks for complex workflows without versioning.
type WorkflowWithoutVersioningRule struct {
	ComplexityThreshold int
}

func NewWorkflowWithoutVersioningRule(threshold int) *WorkflowWithoutVersioningRule {
	if threshold <= 0 {
		threshold = 5 // Default: workflows with 5+ activities should consider versioning
	}
	return &WorkflowWithoutVersioningRule{ComplexityThreshold: threshold}
}

func (r *WorkflowWithoutVersioningRule) ID() string         { return "TA030" }
func (r *WorkflowWithoutVersioningRule) Name() string       { return "workflow-without-versioning" }
func (r *WorkflowWithoutVersioningRule) Category() Category { return CategoryMaintenance }
func (r *WorkflowWithoutVersioningRule) Severity() Severity { return SeverityInfo }
func (r *WorkflowWithoutVersioningRule) Description() string {
	return "Long-running workflows may execute for days or weeks. Without versioning, deploying logic changes can break in-flight executions mid-run, causing failures and data inconsistencies."
}

func (r *WorkflowWithoutVersioningRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if node.Type != "workflow" {
			continue
		}
		// Check if workflow is complex (has many calls)
		isComplex := len(node.CallSites) >= r.ComplexityThreshold
		hasVersioning := len(node.Versioning) > 0

		if isComplex && !hasVersioning {
			issues = append(issues, Issue{
				RuleID:      r.ID(),
				RuleName:    r.Name(),
				Severity:    r.Severity(),
				Category:    r.Category(),
				Message:     fmt.Sprintf("Complex workflow '%s' (%d calls) has no versioning", node.Name, len(node.CallSites)),
				Description: r.Description(),
				Suggestion:  "Consider using workflow.GetVersion() for safe deployments with running workflows",
				FilePath:    node.FilePath,
				LineNumber:  node.LineNumber,
				NodeName:    node.Name,
				NodeType:    node.Type,
				Fix: &CodeFix{
					Description: "Add workflow versioning for safe deployments",
					Replacements: []Replacement{{
						FilePath:  node.FilePath,
						StartLine: node.LineNumber,
						NewText: `v := workflow.GetVersion(ctx, "initial-version", workflow.DefaultVersion, 1)
if v == workflow.DefaultVersion {
	// existing logic
} else {
	// new logic
}`,
					}},
				},
			})
		}
	}
	return issues
}

// SignalWithoutHandlerRule checks for signal definitions without handlers.
type SignalWithoutHandlerRule struct{}

func (r *SignalWithoutHandlerRule) ID() string         { return "TA031" }
func (r *SignalWithoutHandlerRule) Name() string       { return "signal-without-handler" }
func (r *SignalWithoutHandlerRule) Category() Category { return CategoryReliability }
func (r *SignalWithoutHandlerRule) Severity() Severity { return SeverityWarning }
func (r *SignalWithoutHandlerRule) Description() string {
	return "Unhandled signals are silently dropped. External systems sending signals believe they're communicating with the workflow, but the data goes nowhere—a silent failure that's hard to debug."
}

func (r *SignalWithoutHandlerRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if node.Type != "workflow" {
			continue
		}
		for _, signal := range node.Signals {
			if signal.Handler == "" {
				issues = append(issues, Issue{
					RuleID:      r.ID(),
					RuleName:    r.Name(),
					Severity:    r.Severity(),
					Category:    r.Category(),
					Message:     fmt.Sprintf("Signal '%s' in workflow '%s' has no handler", signal.Name, node.Name),
					Description: r.Description(),
					Suggestion:  "Add a signal handler using workflow.SetSignalHandler()",
					FilePath:    node.FilePath,
					LineNumber:  signal.LineNumber,
					NodeName:    node.Name,
					NodeType:    node.Type,
				})
			}
		}
	}
	return issues
}

// QueryWithoutReturnRule checks for query handlers that might not return values.
type QueryWithoutReturnRule struct{}

func (r *QueryWithoutReturnRule) ID() string         { return "TA032" }
func (r *QueryWithoutReturnRule) Name() string       { return "query-without-return" }
func (r *QueryWithoutReturnRule) Category() Category { return CategoryBestPractice }
func (r *QueryWithoutReturnRule) Severity() Severity { return SeverityInfo }
func (r *QueryWithoutReturnRule) Description() string {
	return "Queries exist so external systems can inspect workflow state without affecting execution. A query that returns nothing defeats its purpose and leaves callers without the insight they need."
}

func (r *QueryWithoutReturnRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if node.Type != "workflow" {
			continue
		}
		for _, query := range node.Queries {
			if query.ReturnType == "" || query.ReturnType == "interface{}" {
				issues = append(issues, Issue{
					RuleID:      r.ID(),
					RuleName:    r.Name(),
					Severity:    r.Severity(),
					Category:    r.Category(),
					Message:     fmt.Sprintf("Query '%s' in workflow '%s' has no typed return", query.Name, node.Name),
					Description: r.Description(),
					Suggestion:  "Define a concrete return type for better type safety",
					FilePath:    node.FilePath,
					LineNumber:  query.LineNumber,
					NodeName:    node.Name,
					NodeType:    node.Type,
				})
			}
		}
	}
	return issues
}

// ContinueAsNewWithoutConditionRule checks for continue-as-new that might run indefinitely.
type ContinueAsNewWithoutConditionRule struct{}

func (r *ContinueAsNewWithoutConditionRule) ID() string         { return "TA033" }
func (r *ContinueAsNewWithoutConditionRule) Name() string       { return "continue-as-new-risk" }
func (r *ContinueAsNewWithoutConditionRule) Category() Category { return CategoryReliability }
func (r *ContinueAsNewWithoutConditionRule) Severity() Severity { return SeverityInfo }
func (r *ContinueAsNewWithoutConditionRule) Description() string {
	return "Without termination conditions, continue-as-new workflows run forever, accumulating costs and never completing. Every long-running workflow should have a defined end state or maximum iterations."
}

func (r *ContinueAsNewWithoutConditionRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue
	for _, node := range graph.Nodes {
		if node.Type != "workflow" {
			continue
		}
		if node.ContinueAsNew != nil {
			issues = append(issues, Issue{
				RuleID:      r.ID(),
				RuleName:    r.Name(),
				Severity:    r.Severity(),
				Category:    r.Category(),
				Message:     fmt.Sprintf("Workflow '%s' uses continue-as-new", node.Name),
				Description: r.Description(),
				Suggestion:  "Ensure there's a clear termination condition to prevent infinite continuation",
				FilePath:    node.FilePath,
				LineNumber:  node.ContinueAsNew.LineNumber,
				NodeName:    node.Name,
				NodeType:    node.Type,
			})
		}
	}
	return issues
}

// =============================================================================
// Type Safety Rules
// =============================================================================

// ArgumentCountMismatchRule checks for activities/workflows called with wrong number of arguments.
type ArgumentCountMismatchRule struct{}

func (r *ArgumentCountMismatchRule) ID() string         { return "TA040" }
func (r *ArgumentCountMismatchRule) Name() string       { return "argument-count-mismatch" }
func (r *ArgumentCountMismatchRule) Category() Category { return CategoryReliability }
func (r *ArgumentCountMismatchRule) Severity() Severity { return SeverityError }
func (r *ArgumentCountMismatchRule) Description() string {
	return "Calling an activity or workflow with the wrong number of arguments will cause a runtime error. Temporal deserializes arguments by position, so mismatches cause failures that are hard to debug."
}

func (r *ArgumentCountMismatchRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue

	for _, node := range graph.Nodes {
		// Check each call site
		for _, callSite := range node.CallSites {
			// Skip if no argument count was captured
			if callSite.ArgumentCount == 0 && len(callSite.ArgumentTypes) == 0 {
				continue
			}

			// Find the target node
			targetNode, exists := graph.Nodes[callSite.TargetName]
			if !exists {
				continue
			}

			// Count expected parameters (excluding context)
			expectedCount := countNonContextParams(targetNode.Parameters)

			if callSite.ArgumentCount != expectedCount {
				issues = append(issues, Issue{
					RuleID:   r.ID(),
					RuleName: r.Name(),
					Severity: r.Severity(),
					Category: r.Category(),
					Message: fmt.Sprintf(
						"Call to '%s' passes %d argument(s), but %s '%s' expects %d",
						callSite.TargetName,
						callSite.ArgumentCount,
						targetNode.Type,
						targetNode.Name,
						expectedCount,
					),
					Description: r.Description(),
					Suggestion:  fmt.Sprintf("Update the call to pass exactly %d argument(s) matching the %s signature", expectedCount, targetNode.Type),
					FilePath:    callSite.FilePath,
					LineNumber:  callSite.LineNumber,
					NodeName:    node.Name,
					NodeType:    node.Type,
				})
			}
		}
	}

	return issues
}

// countNonContextParams counts parameters that aren't context.Context or workflow.Context.
func countNonContextParams(params map[string]string) int {
	count := 0
	for _, paramType := range params {
		// Skip context parameters
		if paramType == "context.Context" || paramType == "workflow.Context" {
			continue
		}
		count++
	}
	return count
}

// =============================================================================
// Helper Functions
// =============================================================================

func findCircularDependencies(ctx context.Context, graph *analyzer.TemporalGraph) []string {
	var cycles []string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for _, node := range graph.Nodes {
		select {
		case <-ctx.Done():
			return cycles
		default:
		}

		if !visited[node.Name] {
			if cycle := detectCycle(ctx, node, graph, visited, recStack, []string{}); cycle != "" {
				cycles = append(cycles, cycle)
			}
		}
	}

	return cycles
}

func detectCycle(ctx context.Context, node *analyzer.TemporalNode, graph *analyzer.TemporalGraph, visited, recStack map[string]bool, path []string) string {
	select {
	case <-ctx.Done():
		return ""
	default:
	}

	visited[node.Name] = true
	recStack[node.Name] = true
	path = append(path, node.Name)

	for _, callSite := range node.CallSites {
		if childNode, exists := graph.Nodes[callSite.TargetName]; exists {
			if !visited[childNode.Name] {
				if cycle := detectCycle(ctx, childNode, graph, visited, recStack, path); cycle != "" {
					return cycle
				}
			} else if recStack[childNode.Name] {
				cycleStart := -1
				for i, name := range path {
					if name == childNode.Name {
						cycleStart = i
						break
					}
				}
				if cycleStart != -1 {
					cyclePath := append(path[cycleStart:], childNode.Name)
					return strings.Join(cyclePath, " -> ")
				}
			}
		}
	}

	recStack[node.Name] = false
	return ""
}

func calculateChainDepth(ctx context.Context, node *analyzer.TemporalNode, graph *analyzer.TemporalGraph, visited map[string]bool) int {
	select {
	case <-ctx.Done():
		return 0
	default:
	}

	if visited[node.Name] {
		return 0
	}

	visited[node.Name] = true
	defer func() { visited[node.Name] = false }()

	maxDepth := 0
	for _, callSite := range node.CallSites {
		if childNode, exists := graph.Nodes[callSite.TargetName]; exists {
			depth := 1 + calculateChainDepth(ctx, childNode, graph, visited)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}

	return maxDepth
}
