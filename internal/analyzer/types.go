// Package analyzer provides functionality for analyzing Temporal.io workflows and activities
// in Go codebases, building dependency graphs, and extracting call relationships.
package analyzer

import (
	"go/ast"
	"go/token"
)

// TemporalNode represents a workflow or activity in the temporal graph.
type TemporalNode struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // "workflow", "activity", "signal", "query", "update"
	Package     string            `json:"package"`
	FilePath    string            `json:"file_path"`
	LineNumber  int               `json:"line_number"`
	Description string            `json:"description,omitempty"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	ReturnType  string            `json:"return_type,omitempty"`

	// Relationship data
	CallSites     []CallSite     `json:"call_sites,omitempty"`
	InternalCalls []InternalCall `json:"internal_calls,omitempty"` // Non-Temporal function calls
	Parents       []string       `json:"parents,omitempty"`

	// Temporal-specific metadata
	Signals       []SignalDef       `json:"signals,omitempty"`
	Queries       []QueryDef        `json:"queries,omitempty"`
	Updates       []UpdateDef       `json:"updates,omitempty"`
	Timers        []TimerDef        `json:"timers,omitempty"`
	SearchAttrs   []SearchAttrDef   `json:"search_attrs,omitempty"`
	WorkflowOpts  *WorkflowOptions  `json:"workflow_opts,omitempty"`
	ActivityOpts  *ActivityOptions  `json:"activity_opts,omitempty"`
	ChildWorkflow []ChildWorkflow   `json:"child_workflows,omitempty"`
	LocalActivity []LocalActivity   `json:"local_activities,omitempty"`
	ContinueAsNew *ContinueAsNewDef `json:"continue_as_new,omitempty"`
	Versioning    []VersionDef      `json:"versioning,omitempty"`
}

// CallSite represents a location where a workflow or activity is called.
type CallSite struct {
	TargetName string   `json:"target_name"`
	TargetType string   `json:"target_type,omitempty"` // "workflow", "activity", "signal", etc.
	CallType   string   `json:"call_type,omitempty"`   // "execute", "signal", "query", "update"
	LineNumber int      `json:"line_number"`
	FilePath   string   `json:"file_path"`
	Options    []string `json:"options,omitempty"` // Activity/workflow options used

	// Signature validation fields
	ArgumentCount int      `json:"argument_count,omitempty"` // Number of arguments passed (excluding ctx and activity func)
	ArgumentTypes []string `json:"argument_types,omitempty"` // Types of arguments if determinable
	ResultType    string   `json:"result_type,omitempty"`    // Type used in .Get() call if present

	// Parsed activity options from the call site
	ParsedActivityOpts *ActivityOptions `json:"parsed_activity_opts,omitempty"`
}

// InternalCall represents a regular Go function/method call within an activity or workflow.
// These are non-Temporal calls that show the internal implementation structure.
type InternalCall struct {
	TargetName string `json:"target_name"`           // Function or method name
	Receiver   string `json:"receiver,omitempty"`    // Receiver type/package (e.g., "store" in store.Save())
	CallType   string `json:"call_type"`             // "function", "method"
	LineNumber int    `json:"line_number"`
	FilePath   string `json:"file_path"`
}

// SignalDef represents a signal definition in a workflow.
type SignalDef struct {
	Name        string            `json:"name"`
	Channel     string            `json:"channel,omitempty"`
	PayloadType string            `json:"payload_type,omitempty"`
	Handler     string            `json:"handler,omitempty"`
	LineNumber  int               `json:"line_number"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	IsExternal  bool              `json:"is_external,omitempty"` // Signal sent from outside
}

// QueryDef represents a query definition in a workflow.
type QueryDef struct {
	Name        string            `json:"name"`
	Handler     string            `json:"handler,omitempty"`
	ReturnType  string            `json:"return_type,omitempty"`
	LineNumber  int               `json:"line_number"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

// UpdateDef represents an update definition in a workflow (Temporal SDK 1.20+).
type UpdateDef struct {
	Name        string            `json:"name"`
	Handler     string            `json:"handler,omitempty"`
	Validator   string            `json:"validator,omitempty"`
	ReturnType  string            `json:"return_type,omitempty"`
	LineNumber  int               `json:"line_number"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

// TimerDef represents a timer used in a workflow.
type TimerDef struct {
	Name       string `json:"name,omitempty"`
	Duration   string `json:"duration"`
	LineNumber int    `json:"line_number"`
	IsSleep    bool   `json:"is_sleep"` // workflow.Sleep vs workflow.NewTimer
}

// SearchAttrDef represents a search attribute used in a workflow.
type SearchAttrDef struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // "keyword", "text", "int", "double", "bool", "datetime"
	LineNumber int    `json:"line_number"`
	Operation  string `json:"operation"` // "upsert", "read"
}

// WorkflowOptions represents workflow execution options.
type WorkflowOptions struct {
	TaskQueue           string `json:"task_queue,omitempty"`
	ExecutionTimeout    string `json:"execution_timeout,omitempty"`
	RunTimeout          string `json:"run_timeout,omitempty"`
	TaskTimeout         string `json:"task_timeout,omitempty"`
	RetryPolicy         *RetryPolicy `json:"retry_policy,omitempty"`
	CronSchedule        string `json:"cron_schedule,omitempty"`
	Memo                bool   `json:"memo,omitempty"`
	SearchAttributes    bool   `json:"search_attributes,omitempty"`
	ParentClosePolicy   string `json:"parent_close_policy,omitempty"`
	WorkflowIDReusePolicy string `json:"workflow_id_reuse_policy,omitempty"`
}

// ActivityOptions represents activity execution options.
type ActivityOptions struct {
	TaskQueue              string       `json:"task_queue,omitempty"`
	ScheduleToStartTimeout string       `json:"schedule_to_start_timeout,omitempty"`
	StartToCloseTimeout    string       `json:"start_to_close_timeout,omitempty"`
	HeartbeatTimeout       string       `json:"heartbeat_timeout,omitempty"`
	ScheduleToCloseTimeout string       `json:"schedule_to_close_timeout,omitempty"`
	RetryPolicy            *RetryPolicy `json:"retry_policy,omitempty"`
	WaitForCancellation    bool         `json:"wait_for_cancellation,omitempty"`

	// optionsProvided indicates that activity options were specified (even if we couldn't parse them)
	optionsProvided bool
}

// OptionsProvided returns true if activity options were specified in the code.
func (ao *ActivityOptions) OptionsProvided() bool {
	return ao != nil && ao.optionsProvided
}

// HasRetryPolicy returns true if a retry policy was specified.
func (ao *ActivityOptions) HasRetryPolicy() bool {
	if ao == nil || ao.RetryPolicy == nil {
		return false
	}
	rp := ao.RetryPolicy
	// Return true if policyProvided flag is set, OR if any retry policy fields have values
	return rp.policyProvided ||
		rp.InitialInterval != "" ||
		rp.BackoffCoefficient != "" ||
		rp.MaximumInterval != "" ||
		rp.MaximumAttempts > 0 ||
		len(rp.NonRetryableErrors) > 0
}

// RetryPolicy represents a retry policy configuration.
type RetryPolicy struct {
	InitialInterval    string   `json:"initial_interval,omitempty"`
	BackoffCoefficient string   `json:"backoff_coefficient,omitempty"` // Stored as string to preserve source format
	MaximumInterval    string   `json:"maximum_interval,omitempty"`
	MaximumAttempts    int      `json:"maximum_attempts,omitempty"`
	NonRetryableErrors []string `json:"non_retryable_errors,omitempty"`

	// policyProvided indicates that a retry policy was specified (even if we couldn't parse details)
	policyProvided bool
}

// PolicyProvided returns true if a retry policy was specified in the code.
func (rp *RetryPolicy) PolicyProvided() bool {
	return rp != nil && rp.policyProvided
}

// ChildWorkflow represents a child workflow execution.
type ChildWorkflow struct {
	Name            string           `json:"name"`
	LineNumber      int              `json:"line_number"`
	Options         *WorkflowOptions `json:"options,omitempty"`
	ParentClosePolicy string         `json:"parent_close_policy,omitempty"`
}

// LocalActivity represents a local activity execution.
type LocalActivity struct {
	Name       string           `json:"name"`
	LineNumber int              `json:"line_number"`
	Options    *ActivityOptions `json:"options,omitempty"`
}

// ContinueAsNewDef represents a continue-as-new call in a workflow.
type ContinueAsNewDef struct {
	LineNumber int               `json:"line_number"`
	Arguments  map[string]string `json:"arguments,omitempty"`
}

// VersionDef represents workflow versioning information.
type VersionDef struct {
	ChangeID   string `json:"change_id"`
	MinVersion int    `json:"min_version"`
	MaxVersion int    `json:"max_version"`
	LineNumber int    `json:"line_number"`
}

// TemporalGraph represents the complete graph of temporal workflows and activities.
type TemporalGraph struct {
	Nodes map[string]*TemporalNode `json:"nodes"`
	Stats GraphStats               `json:"stats"`
}

// GraphStats contains statistics about the temporal graph.
type GraphStats struct {
	TotalWorkflows   int `json:"total_workflows"`
	TotalActivities  int `json:"total_activities"`
	TotalSignals     int `json:"total_signals"`
	TotalQueries     int `json:"total_queries"`
	TotalUpdates     int `json:"total_updates"`
	TotalTimers      int `json:"total_timers"`
	MaxDepth         int `json:"max_depth"`
	OrphanNodes      int `json:"orphan_nodes"`
	CircularDeps     int `json:"circular_deps"`
	TotalConnections int `json:"total_connections"`
	AvgFanOut        float64 `json:"avg_fan_out"`
	MaxFanOut        int `json:"max_fan_out"`
}

// NodeMatch represents a parsed AST node with its metadata.
type NodeMatch struct {
	Node     ast.Node
	FileSet  *token.FileSet
	FilePath string
	Package  string
	NodeType string // "workflow", "activity", "signal_handler", "query_handler", "update_handler"
}

// NodeCategory groups node types for display purposes.
type NodeCategory string

const (
	CategoryWorkflow  NodeCategory = "workflow"
	CategoryActivity  NodeCategory = "activity"
	CategorySignal    NodeCategory = "signal"
	CategoryQuery     NodeCategory = "query"
	CategoryUpdate    NodeCategory = "update"
)

// GetCategory returns the category of a node type.
func GetCategory(nodeType string) NodeCategory {
	switch nodeType {
	case "workflow":
		return CategoryWorkflow
	case "activity":
		return CategoryActivity
	case "signal", "signal_handler":
		return CategorySignal
	case "query", "query_handler":
		return CategoryQuery
	case "update", "update_handler":
		return CategoryUpdate
	default:
		return CategoryWorkflow
	}
}
