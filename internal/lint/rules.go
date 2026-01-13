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

	// LLM enhancement fields
	Confidence   float64 `json:"confidence,omitempty"`   // LLM confidence in the finding (0.0-1.0)
	LLMReasoning string  `json:"llmReasoning,omitempty"` // LLM explanation for verification/fix
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

// ActivityUnlimitedRetryRule checks for activities with unlimited retry attempts.
// NOTE: Temporal SDK has UNLIMITED retries by default (MaximumAttempts=0).
// This rule warns when activities might retry forever, which may not be desired
// for payment/idempotency-sensitive operations.
type ActivityUnlimitedRetryRule struct{}

func (r *ActivityUnlimitedRetryRule) ID() string         { return "TA001" }
func (r *ActivityUnlimitedRetryRule) Name() string       { return "activity-unlimited-retry" }
func (r *ActivityUnlimitedRetryRule) Category() Category { return CategoryReliability }
func (r *ActivityUnlimitedRetryRule) Severity() Severity { return SeverityWarning }
func (r *ActivityUnlimitedRetryRule) Description() string {
	return "Activities have UNLIMITED retries by default (MaximumAttempts=0). For non-idempotent operations (payments, filings), unlimited retries could cause duplicate processing. Consider setting explicit MaximumAttempts."
}

func (r *ActivityUnlimitedRetryRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue

	// Check activity calls in workflows for unlimited retry policies.
	// Retry policies are configured at the call site (via WithActivityOptions).
	// If no RetryPolicy is set, Temporal uses SERVER DEFAULTS: unlimited retries.
	for _, node := range graph.Nodes {
		// Only check workflow nodes for their activity call sites
		if node.Type != "workflow" {
			continue
		}

		for _, callSite := range node.CallSites {
			// Only check activity and local_activity calls
			if callSite.CallType != "activity" && callSite.CallType != "local_activity" {
				continue
			}

			// Check if retry policy explicitly sets MaximumAttempts
			hasMaxAttempts := false
			if callSite.ParsedActivityOpts != nil && callSite.ParsedActivityOpts.RetryPolicy != nil {
				// MaximumAttempts > 0 means bounded retries
				// MaximumAttempts == 1 means no retries (intentionally disabled)
				hasMaxAttempts = callSite.ParsedActivityOpts.RetryPolicy.MaximumAttempts > 0
			}

			if !hasMaxAttempts {
				issues = append(issues, Issue{
					RuleID:      r.ID(),
					RuleName:    r.Name(),
					Severity:    r.Severity(),
					Category:    r.Category(),
					Message:     fmt.Sprintf("Activity '%s' has unlimited retry attempts (server default)", callSite.TargetName),
					Description: r.Description(),
					Suggestion:  "Consider setting MaximumAttempts in RetryPolicy for bounded retries, especially for non-idempotent operations",
					FilePath:    callSite.FilePath,
					LineNumber:  callSite.LineNumber,
					NodeName:    callSite.TargetName,
					NodeType:    callSite.CallType,
					Fix: &CodeFix{
						Description: "Add bounded retry policy to activity options",
						Replacements: []Replacement{{
							FilePath:  callSite.FilePath,
							StartLine: callSite.LineNumber,
							NewText: `ao := workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
		MaximumAttempts:    3, // Bounded retries prevent infinite loops
	},
}
ctx = workflow.WithActivityOptions(ctx, ao)`,
						}},
					},
				})
			}
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

	// Check activity calls in workflows for missing timeouts.
	// Timeouts are configured at the call site (via WithActivityOptions).
	for _, node := range graph.Nodes {
		if node.Type != "workflow" {
			continue
		}

		for _, callSite := range node.CallSites {
			if callSite.CallType != "activity" && callSite.CallType != "local_activity" {
				continue
			}

			// Check if timeout is configured at this call site
			hasTimeout := false
			if callSite.ParsedActivityOpts != nil {
				opts := callSite.ParsedActivityOpts
				hasTimeout = opts.StartToCloseTimeout != "" ||
					opts.ScheduleToCloseTimeout != "" ||
					opts.ScheduleToStartTimeout != ""
			}

			if !hasTimeout {
				issues = append(issues, Issue{
					RuleID:      r.ID(),
					RuleName:    r.Name(),
					Severity:    r.Severity(),
					Category:    r.Category(),
					Message:     fmt.Sprintf("Activity '%s' has no timeout configured", callSite.TargetName),
					Description: r.Description(),
					Suggestion:  "Add StartToCloseTimeout or ScheduleToCloseTimeout to activity options",
					FilePath:    callSite.FilePath,
					LineNumber:  callSite.LineNumber,
					NodeName:    callSite.TargetName,
					NodeType:    callSite.CallType,
					Fix: &CodeFix{
						Description: "Add timeout to activity options",
						Replacements: []Replacement{{
							FilePath:  callSite.FilePath,
							StartLine: callSite.LineNumber,
							NewText: `ao := workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Minute,
}
ctx = workflow.WithActivityOptions(ctx, ao)`,
						}},
					},
				})
			}
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
	return "Long-running activities should have heartbeats. Without them, if a worker dies (OOMKill, scale-down, SIGKILL), Temporal must wait for the full timeout before retrying. Use background goroutine heartbeats for best results."
}

func (r *LongRunningActivityWithoutHeartbeatRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue

	// Check activity calls in workflows for missing heartbeat timeouts.
	// Heartbeat timeouts are configured at the call site (via WithActivityOptions).
	for _, node := range graph.Nodes {
		if node.Type != "workflow" {
			continue
		}

		for _, callSite := range node.CallSites {
			if callSite.CallType != "activity" && callSite.CallType != "local_activity" {
				continue
			}

			// Check if activity appears to be long-running based on naming
			targetName := strings.ToLower(callSite.TargetName)
			isLongRunning := strings.Contains(targetName, "process") ||
				strings.Contains(targetName, "batch") ||
				strings.Contains(targetName, "sync") ||
				strings.Contains(targetName, "import") ||
				strings.Contains(targetName, "export") ||
				strings.Contains(targetName, "migrate") ||
				strings.Contains(targetName, "generate") ||
				strings.Contains(targetName, "create") ||
				strings.Contains(targetName, "cleanup") ||
				strings.Contains(targetName, "duplicate")

			if !isLongRunning {
				continue
			}

			// Check if heartbeat timeout is configured at this call site
			hasHeartbeat := false
			if callSite.ParsedActivityOpts != nil {
				hasHeartbeat = callSite.ParsedActivityOpts.HeartbeatTimeout != ""
			}

			if !hasHeartbeat {
				issues = append(issues, Issue{
					RuleID:      r.ID(),
					RuleName:    r.Name(),
					Severity:    r.Severity(),
					Category:    r.Category(),
					Message:     fmt.Sprintf("Potentially long-running activity '%s' has no heartbeat configured", callSite.TargetName),
					Description: r.Description(),
					Suggestion:  "Add HeartbeatTimeout and use background goroutine heartbeats (not just per-item heartbeats in loops, which can timeout during slow individual items)",
					FilePath:    callSite.FilePath,
					LineNumber:  callSite.LineNumber,
					NodeName:    callSite.TargetName,
					NodeType:    callSite.CallType,
					Fix: &CodeFix{
						Description: "Add heartbeat timeout to activity options",
						Replacements: []Replacement{{
							FilePath:  callSite.FilePath,
							StartLine: callSite.LineNumber,
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
	}
	return issues
}

// ChildWorkflowUnlimitedRetryRule checks for child workflows with unlimited retry attempts.
// NOTE: Child workflows do NOT inherit RetryPolicy from parent workflows.
// They get Temporal server defaults (unlimited retries) if not explicitly set.
type ChildWorkflowUnlimitedRetryRule struct{}

func (r *ChildWorkflowUnlimitedRetryRule) ID() string         { return "TA004" }
func (r *ChildWorkflowUnlimitedRetryRule) Name() string       { return "child-workflow-unlimited-retry" }
func (r *ChildWorkflowUnlimitedRetryRule) Category() Category { return CategoryReliability }
func (r *ChildWorkflowUnlimitedRetryRule) Severity() Severity { return SeverityWarning }
func (r *ChildWorkflowUnlimitedRetryRule) Description() string {
	return "Child workflows do NOT inherit RetryPolicy from parent workflows. They get Temporal server defaults (UNLIMITED retries). For payment/idempotency-sensitive child workflows, this could cause duplicate processing. Consider setting explicit MaximumAttempts."
}

func (r *ChildWorkflowUnlimitedRetryRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue

	// Check child workflow calls for unlimited retry policies.
	for _, node := range graph.Nodes {
		if node.Type != "workflow" {
			continue
		}

		for _, callSite := range node.CallSites {
			// Only check child_workflow calls
			if callSite.CallType != "child_workflow" {
				continue
			}

			// Child workflows use WorkflowOptions, not ActivityOptions
			// For now, we flag all child workflows without explicit retry configuration
			// since ParsedActivityOpts won't capture ChildWorkflowOptions
			//
			// TODO: Add ChildWorkflowOptions parsing to extractor.go
			hasMaxAttempts := false
			if callSite.ParsedActivityOpts != nil && callSite.ParsedActivityOpts.RetryPolicy != nil {
				hasMaxAttempts = callSite.ParsedActivityOpts.RetryPolicy.MaximumAttempts > 0
			}

			if !hasMaxAttempts {
				issues = append(issues, Issue{
					RuleID:      r.ID(),
					RuleName:    r.Name(),
					Severity:    r.Severity(),
					Category:    r.Category(),
					Message:     fmt.Sprintf("Child workflow '%s' has unlimited retry attempts (does NOT inherit from parent)", callSite.TargetName),
					Description: r.Description(),
					Suggestion:  "Consider setting MaximumAttempts in ChildWorkflowOptions.RetryPolicy for bounded retries",
					FilePath:    callSite.FilePath,
					LineNumber:  callSite.LineNumber,
					NodeName:    callSite.TargetName,
					NodeType:    callSite.CallType,
					Fix: &CodeFix{
						Description: "Add bounded retry policy to child workflow options",
						Replacements: []Replacement{{
							FilePath:  callSite.FilePath,
							StartLine: callSite.LineNumber,
							NewText: `childOpts := workflow.ChildWorkflowOptions{
	WorkflowExecutionTimeout: 1 * time.Hour,
	RetryPolicy: &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    time.Minute,
		MaximumAttempts:    3, // Child workflows do NOT inherit parent's retry policy
	},
}
ctx = workflow.WithChildOptions(ctx, childOpts)`,
						}},
					},
				})
			}
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

// ConsiderQueryHandlerRule suggests using QueryHandlers for workflows with rich heartbeat data.
// QueryHandlers provide on-demand progress queries at the workflow level, which can be
// more efficient than storing detailed progress in activity heartbeats.
type ConsiderQueryHandlerRule struct{}

func (r *ConsiderQueryHandlerRule) ID() string         { return "TA034" }
func (r *ConsiderQueryHandlerRule) Name() string       { return "consider-query-handler" }
func (r *ConsiderQueryHandlerRule) Category() Category { return CategoryBestPractice }
func (r *ConsiderQueryHandlerRule) Severity() Severity { return SeverityInfo }
func (r *ConsiderQueryHandlerRule) Description() string {
	return "Workflows with long-running activities often need progress tracking. QueryHandlers provide on-demand progress queries without the serialization overhead of rich heartbeat payloads. Consider using SetQueryHandler for progress state."
}

func (r *ConsiderQueryHandlerRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue

	for _, node := range graph.Nodes {
		if node.Type != "workflow" {
			continue
		}

		// Check if workflow has activities with heartbeat timeouts but no query handlers
		hasLongActivities := false
		for _, callSite := range node.CallSites {
			if callSite.CallType == "activity" || callSite.CallType == "local_activity" {
				if callSite.ParsedActivityOpts != nil && callSite.ParsedActivityOpts.HeartbeatTimeout != "" {
					hasLongActivities = true
					break
				}
			}
		}

		// Check if workflow already has query handlers
		hasQueryHandler := len(node.Queries) > 0

		// Suggest QueryHandler if workflow has long activities but no query handlers
		if hasLongActivities && !hasQueryHandler {
			issues = append(issues, Issue{
				RuleID:      r.ID(),
				RuleName:    r.Name(),
				Severity:    r.Severity(),
				Category:    r.Category(),
				Message:     fmt.Sprintf("Workflow '%s' has long-running activities but no QueryHandler for progress tracking", node.Name),
				Description: r.Description(),
				Suggestion:  "Consider adding a QueryHandler for progress state instead of or in addition to rich heartbeat payloads",
				FilePath:    node.FilePath,
				LineNumber:  node.LineNumber,
				NodeName:    node.Name,
				NodeType:    node.Type,
				Fix: &CodeFix{
					Description: "Add QueryHandler for progress tracking",
					Replacements: []Replacement{{
						FilePath:  node.FilePath,
						StartLine: node.LineNumber,
						NewText: `err := workflow.SetQueryHandler(ctx, "progress", func() (map[string]interface{}, error) {
	return map[string]interface{}{
		"phase":     currentPhase,
		"processed": itemsProcessed,
		"total":     totalItems,
	}, nil
})
if err != nil {
	return err
}`,
					}},
				},
			})
		}
	}

	return issues
}

// =============================================================================
// Type Safety Rules
// =============================================================================

// ArgumentsMismatchRule checks for activities/workflows called with wrong arguments or return types.
type ArgumentsMismatchRule struct{}

func (r *ArgumentsMismatchRule) ID() string         { return "TA040" }
func (r *ArgumentsMismatchRule) Name() string       { return "arguments-mismatch" }
func (r *ArgumentsMismatchRule) Category() Category { return CategoryReliability }
func (r *ArgumentsMismatchRule) Severity() Severity { return SeverityError }
func (r *ArgumentsMismatchRule) Description() string {
	return "Calling an activity or workflow with wrong number/types of arguments, or reading results into wrong types, causes runtime errors. Temporal deserializes by position and type, so mismatches fail at runtime."
}

func (r *ArgumentsMismatchRule) Check(ctx context.Context, graph *analyzer.TemporalGraph) []Issue {
	var issues []Issue

	for _, node := range graph.Nodes {
		// Check each call site
		for _, callSite := range node.CallSites {
			// Find the target node
			targetNode, exists := graph.Nodes[callSite.TargetName]
			if !exists {
				continue
			}

			// Check argument count mismatch for activity/workflow calls
			// Only check for call types where we extract argument info
			if callSite.CallType == "activity" || callSite.CallType == "child_workflow" || callSite.CallType == "local_activity" {
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

			// Check return type mismatch
			if callSite.ResultType != "" && targetNode.ReturnType != "" {
				if !isTypeCompatible(callSite.ResultType, targetNode.ReturnType) {
					issues = append(issues, Issue{
						RuleID:   r.ID(),
						RuleName: r.Name(),
						Severity: r.Severity(),
						Category: r.Category(),
						Message: fmt.Sprintf(
							"Call to '%s' reads result as '%s', but %s '%s' returns '%s'",
							callSite.TargetName,
							callSite.ResultType,
							targetNode.Type,
							targetNode.Name,
							targetNode.ReturnType,
						),
						Description: r.Description(),
						Suggestion:  fmt.Sprintf("Use a variable of type '%s' to receive the result", targetNode.ReturnType),
						FilePath:    callSite.FilePath,
						LineNumber:  callSite.LineNumber,
						NodeName:    node.Name,
						NodeType:    node.Type,
					})
				}
			}
		}
	}

	return issues
}

// isTypeCompatible checks if the result type is compatible with the expected return type.
func isTypeCompatible(resultType, returnType string) bool {
	// Handle pointer types - result is usually a pointer to the actual type
	resultType = strings.TrimPrefix(resultType, "*")

	// If result type starts with "var:", "call:", or "selector:", we can't determine compatibility
	// These are placeholders for when we couldn't statically determine the type
	if strings.HasPrefix(resultType, "var:") ||
		strings.HasPrefix(resultType, "call:") ||
		strings.HasPrefix(resultType, "selector:") ||
		resultType == "call" ||
		resultType == "indexed" {
		return true // Can't determine, assume compatible
	}

	// "unknown" type means we couldn't determine it, skip check
	if resultType == "unknown" || resultType == "nil" {
		return true
	}

	// Direct match
	if resultType == returnType {
		return true
	}

	// Handle interface{} / any - compatible with anything
	if returnType == "interface{}" || returnType == "any" {
		return true
	}

	// Handle error type specially - it's often the last return value
	// and can be received into error or nil
	if returnType == "error" && (resultType == "error" || resultType == "nil") {
		return true
	}

	return false
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
		// Skip self-referential calls (recursion) - these are not circular dependencies.
		// A method calling itself is valid recursion, not a deadlock-causing cycle.
		// Circular dependencies require at least 2 different nodes (A -> B -> A).
		if callSite.TargetName == node.Name {
			continue
		}

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
