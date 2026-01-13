package lint

import (
	"context"
	"sort"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

// Config holds linter configuration.
type Config struct {
	// MinSeverity is the minimum severity level to report
	MinSeverity Severity
	// EnabledRules contains the IDs of rules to enable (empty means all)
	EnabledRules []string
	// DisabledRules contains the IDs of rules to disable
	DisabledRules []string
	// FailOnWarning treats warnings as failures for CI
	FailOnWarning bool
	// MaxIssues is the maximum number of issues to report (0 = unlimited)
	MaxIssues int
	// CustomThresholds allows overriding default rule thresholds
	Thresholds Thresholds

	// LLM enhancement options
	LLMEnhance bool   // Use LLM to generate context-aware code fixes
	LLMVerify  bool   // Use LLM to verify/filter findings
	LLMModel   string // Override OpenAI model (default: gpt-4o-mini)
	RootDir    string // Project root for file reading
}

// Thresholds contains configurable thresholds for various rules.
type Thresholds struct {
	MaxFanOut          int `json:"maxFanOut"`
	MaxCallDepth       int `json:"maxCallDepth"`
	VersioningRequired int `json:"versioningRequired"` // Activities count to require versioning
}

// DefaultConfig returns a default linter configuration.
func DefaultConfig() *Config {
	return &Config{
		MinSeverity:   SeverityInfo,
		EnabledRules:  nil, // All rules enabled
		DisabledRules: nil,
		FailOnWarning: false,
		MaxIssues:     0, // Unlimited
		Thresholds: Thresholds{
			MaxFanOut:          15,
			MaxCallDepth:       10,
			VersioningRequired: 5,
		},
	}
}

// StrictConfig returns a strict configuration for CI.
func StrictConfig() *Config {
	cfg := DefaultConfig()
	cfg.FailOnWarning = true
	cfg.MinSeverity = SeverityWarning
	return cfg
}

// Result holds the results of a lint run.
type Result struct {
	Issues     []Issue `json:"issues"`
	ErrorCount int     `json:"errorCount"`
	WarnCount  int     `json:"warningCount"`
	InfoCount  int     `json:"infoCount"`
	TotalNodes int     `json:"totalNodes"`
	ExitCode   int     `json:"exitCode"`
}

// Passed returns true if the lint run passed (no errors, and no warnings if strict).
func (r *Result) Passed(strict bool) bool {
	if r.ErrorCount > 0 {
		return false
	}
	if strict && r.WarnCount > 0 {
		return false
	}
	return true
}

// Summary returns a summary string of the results.
func (r *Result) Summary() string {
	if r.ErrorCount == 0 && r.WarnCount == 0 && r.InfoCount == 0 {
		return "No issues found"
	}
	return ""
}

// Linter orchestrates lint rule execution.
type Linter struct {
	config *Config
	rules  []Rule
	llm    *LLMEnhancer
}

// NewLinter creates a new linter with the given configuration.
func NewLinter(cfg *Config) *Linter {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	l := &Linter{
		config: cfg,
		rules:  make([]Rule, 0),
	}

	// Initialize LLM enhancer if enabled
	if cfg.LLMEnhance || cfg.LLMVerify {
		llmCfg := DefaultLLMConfig()
		if cfg.LLMModel != "" {
			llmCfg.Model = cfg.LLMModel
		}
		llmCfg.RootDir = cfg.RootDir
		l.llm = NewLLMEnhancer(llmCfg)
	}

	// Register all rules
	l.registerRules()

	return l
}

// registerRules registers all available lint rules.
func (l *Linter) registerRules() {
	// Reliability Rules (TA001-TA004)
	l.rules = append(l.rules, &ActivityUnlimitedRetryRule{})
	l.rules = append(l.rules, &ActivityWithoutTimeoutRule{})
	l.rules = append(l.rules, &LongRunningActivityWithoutHeartbeatRule{})
	l.rules = append(l.rules, &ChildWorkflowUnlimitedRetryRule{})

	// Structural Rules (TA010-TA011)
	l.rules = append(l.rules, &CircularDependencyRule{})
	l.rules = append(l.rules, &OrphanNodeRule{})

	// Performance Rules (TA020-TA021)
	l.rules = append(l.rules, NewHighFanOutRule(l.config.Thresholds.MaxFanOut))
	l.rules = append(l.rules, NewDeepCallChainRule(l.config.Thresholds.MaxCallDepth))

	// Maintenance Rules (TA030-TA034)
	l.rules = append(l.rules, NewWorkflowWithoutVersioningRule(l.config.Thresholds.VersioningRequired))
	l.rules = append(l.rules, &SignalWithoutHandlerRule{})
	l.rules = append(l.rules, &QueryWithoutReturnRule{})
	l.rules = append(l.rules, &ContinueAsNewWithoutConditionRule{})
	l.rules = append(l.rules, &ConsiderQueryHandlerRule{})

	// Type Safety Rules (TA040+)
	l.rules = append(l.rules, &ArgumentsMismatchRule{})
}

// isRuleEnabled checks if a rule should be executed.
func (l *Linter) isRuleEnabled(ruleID string) bool {
	// Check if explicitly disabled
	for _, disabled := range l.config.DisabledRules {
		if disabled == ruleID {
			return false
		}
	}

	// If specific rules are enabled, check if this one is in the list
	if len(l.config.EnabledRules) > 0 {
		for _, enabled := range l.config.EnabledRules {
			if enabled == ruleID {
				return true
			}
		}
		return false
	}

	return true
}

// shouldReport checks if an issue meets the minimum severity threshold.
func (l *Linter) shouldReport(issue Issue) bool {
	return issue.Severity.Level() >= l.config.MinSeverity.Level()
}

// Run executes all enabled lint rules against the graph.
func (l *Linter) Run(ctx context.Context, graph *analyzer.TemporalGraph) *Result {
	result := &Result{
		Issues:     make([]Issue, 0),
		TotalNodes: len(graph.Nodes),
	}

	// Collect all issues from rules first
	var allIssues []Issue

	// Execute each enabled rule
	for _, rule := range l.rules {
		select {
		case <-ctx.Done():
			return result
		default:
		}

		if !l.isRuleEnabled(rule.ID()) {
			continue
		}

		issues := rule.Check(ctx, graph)
		for _, issue := range issues {
			if !l.shouldReport(issue) {
				continue
			}
			allIssues = append(allIssues, issue)
		}
	}

	// Apply LLM verification and/or fix enhancement if enabled
	if l.llm != nil && l.llm.IsEnabled() && (l.config.LLMVerify || l.config.LLMEnhance) {
		allIssues, _ = l.llm.EnhanceIssues(ctx, allIssues, graph, l.config.LLMVerify, l.config.LLMEnhance)
	}

	// Count and limit issues
	for _, issue := range allIssues {
		result.Issues = append(result.Issues, issue)

		// Count by severity
		switch issue.Severity {
		case SeverityError:
			result.ErrorCount++
		case SeverityWarning:
			result.WarnCount++
		case SeverityInfo:
			result.InfoCount++
		}

		// Check max issues limit
		if l.config.MaxIssues > 0 && len(result.Issues) >= l.config.MaxIssues {
			break
		}
	}

	// Sort issues by severity (most severe first), then by file/line
	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].Severity.Level() != result.Issues[j].Severity.Level() {
			return result.Issues[i].Severity.Level() > result.Issues[j].Severity.Level()
		}
		if result.Issues[i].FilePath != result.Issues[j].FilePath {
			return result.Issues[i].FilePath < result.Issues[j].FilePath
		}
		return result.Issues[i].LineNumber < result.Issues[j].LineNumber
	})

	// Determine exit code
	if result.ErrorCount > 0 {
		result.ExitCode = 1
	} else if l.config.FailOnWarning && result.WarnCount > 0 {
		result.ExitCode = 1
	}

	return result
}

// ListRules returns all available rules.
func (l *Linter) ListRules() []RuleInfo {
	info := make([]RuleInfo, 0, len(l.rules))
	for _, rule := range l.rules {
		info = append(info, RuleInfo{
			ID:          rule.ID(),
			Name:        rule.Name(),
			Category:    rule.Category(),
			Severity:    rule.Severity(),
			Description: rule.Description(),
			Enabled:     l.isRuleEnabled(rule.ID()),
		})
	}
	return info
}

// RuleInfo provides information about a lint rule.
type RuleInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
}


