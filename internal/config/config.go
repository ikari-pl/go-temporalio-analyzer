// Package config provides configuration management for the temporal analyzer.
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	// Analysis options
	RootDir       string   `json:"root_dir"`
	ExcludeDirs   []string `json:"exclude_dirs,omitempty"`
	IncludeTests  bool     `json:"include_tests"`
	FilterPackage string   `json:"filter_package,omitempty"`
	FilterName    string   `json:"filter_name,omitempty"`

	// Output options
	OutputFormat string `json:"output_format"` // "tui", "json", "tree", "dot"
	OutputFile   string `json:"output_file,omitempty"`
	GraphTool    string `json:"graph_tool"` // "dot", "fdp", "neato", "circo"

	// UI options
	ShowWorkflows  bool `json:"show_workflows"`
	ShowActivities bool `json:"show_activities"`

	// Debug options
	Verbose   bool   `json:"verbose"`
	Debug     bool   `json:"debug"`
	DebugView string `json:"debug_view,omitempty"` // "list", "tree", "details" - render single view and exit

	// Lint options
	LintMode          bool   `json:"lint_mode"`           // Enable lint mode for CI
	LintFormat        string `json:"lint_format"`         // "text", "json", "github", "sarif", "checkstyle"
	LintStrict        bool   `json:"lint_strict"`         // Treat warnings as errors
	LintMinSeverity   string `json:"lint_min_severity"`   // "error", "warning", "info"
	LintDisabledRules string `json:"lint_disabled_rules"` // Comma-separated rule IDs to disable
	LintEnabledRules  string `json:"lint_enabled_rules"`  // Comma-separated rule IDs to enable (exclusive)
	LintListRules     bool   `json:"lint_list_rules"`     // List available lint rules and exit

	// Lint thresholds
	LintMaxFanOut    int `json:"lint_max_fan_out"`    // Max allowed fan-out before warning
	LintMaxCallDepth int `json:"lint_max_call_depth"` // Max call chain depth before warning
}

// NewConfig creates a new configuration with default values.
func NewConfig() *Config {
	return &Config{
		RootDir:        ".",
		ExcludeDirs:    []string{"vendor", ".git", "node_modules"},
		IncludeTests:   false,
		OutputFormat:   "tui",
		GraphTool:      "dot",
		ShowWorkflows:  true,
		ShowActivities: true,
		Verbose:        false,
		Debug:          false,

		// Lint defaults
		LintMode:          false,
		LintFormat:        "text",
		LintStrict:        false,
		LintMinSeverity:   "info",
		LintDisabledRules: "",
		LintEnabledRules:  "",
		LintListRules:     false,
		LintMaxFanOut:     15,
		LintMaxCallDepth:  10,
	}
}

// ParseFlags parses command line flags and updates the config.
func (c *Config) ParseFlags() error {
	flag.StringVar(&c.RootDir, "root", c.RootDir, "Root directory to analyze")
	flag.StringVar(&c.FilterPackage, "package", c.FilterPackage, "Filter by package name (regex)")
	flag.StringVar(&c.FilterName, "name", c.FilterName, "Filter by function name (regex)")
	flag.StringVar(&c.OutputFormat, "format", c.OutputFormat, "Output format (tui, json, tree, dot)")
	flag.StringVar(&c.OutputFile, "output", c.OutputFile, "Output file (defaults to stdout)")
	flag.StringVar(&c.GraphTool, "graph-tool", c.GraphTool, "Graph layout tool (dot, fdp, neato, circo)")
	flag.BoolVar(&c.IncludeTests, "include-tests", c.IncludeTests, "Include test files in analysis")
	flag.BoolVar(&c.ShowWorkflows, "workflows", c.ShowWorkflows, "Show workflows")
	flag.BoolVar(&c.ShowActivities, "activities", c.ShowActivities, "Show activities")
	flag.BoolVar(&c.Verbose, "verbose", c.Verbose, "Verbose output")
	flag.BoolVar(&c.Debug, "debug", c.Debug, "Debug output")
	flag.StringVar(&c.DebugView, "debug-view", c.DebugView, "Debug view rendering (list, tree, details)")

	// Lint flags
	flag.BoolVar(&c.LintMode, "lint", c.LintMode, "Enable lint mode for CI (non-interactive)")
	flag.StringVar(&c.LintFormat, "lint-format", c.LintFormat, "Lint output format (text, json, github, sarif, checkstyle)")
	flag.BoolVar(&c.LintStrict, "lint-strict", c.LintStrict, "Treat warnings as errors (useful for CI)")
	flag.StringVar(&c.LintMinSeverity, "lint-level", c.LintMinSeverity, "Minimum severity to report (error, warning, info)")
	flag.StringVar(&c.LintDisabledRules, "lint-disable", c.LintDisabledRules, "Comma-separated rule IDs to disable")
	flag.StringVar(&c.LintEnabledRules, "lint-enable", c.LintEnabledRules, "Comma-separated rule IDs to enable (exclusive)")
	flag.BoolVar(&c.LintListRules, "lint-rules", c.LintListRules, "List all available lint rules and exit")
	flag.IntVar(&c.LintMaxFanOut, "lint-max-fan-out", c.LintMaxFanOut, "Max fan-out before warning (default: 15)")
	flag.IntVar(&c.LintMaxCallDepth, "lint-max-depth", c.LintMaxCallDepth, "Max call chain depth before warning (default: 10)")

	flag.Parse()

	return c.Validate()
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Skip some validations if just listing rules
	if c.LintListRules {
		return nil
	}

	// Validate root directory
	absRoot, err := filepath.Abs(c.RootDir)
	if err != nil {
		return fmt.Errorf("invalid root directory %s: %w", c.RootDir, err)
	}
	c.RootDir = absRoot

	if _, err := os.Stat(c.RootDir); os.IsNotExist(err) {
		return fmt.Errorf("root directory does not exist: %s", c.RootDir)
	}

	// Validate output format (unless in lint mode)
	if !c.LintMode {
		validFormats := map[string]bool{
			"tui":      true,
			"json":     true,
			"tree":     true,
			"dot":      true,
			"mermaid":  true,
			"markdown": true,
			"md":       true,
		}
		if !validFormats[c.OutputFormat] {
			return fmt.Errorf("invalid output format: %s (valid: tui, json, dot, mermaid, markdown)", c.OutputFormat)
		}
	}

	// Validate graph tool
	validTools := map[string]bool{
		"dot":   true,
		"fdp":   true,
		"neato": true,
		"circo": true,
	}
	if !validTools[c.GraphTool] {
		return fmt.Errorf("invalid graph tool: %s", c.GraphTool)
	}

	// Ensure at least one type is shown
	if !c.ShowWorkflows && !c.ShowActivities {
		return fmt.Errorf("at least one of workflows or activities must be shown")
	}

	// Validate lint options
	if c.LintMode {
		validLintFormats := map[string]bool{
			"text":          true,
			"text-no-color": true,
			"json":          true,
			"github":        true,
			"sarif":         true,
			"checkstyle":    true,
		}
		if !validLintFormats[c.LintFormat] {
			return fmt.Errorf("invalid lint format: %s (valid: text, json, github, sarif, checkstyle)", c.LintFormat)
		}

		validSeverities := map[string]bool{
			"error":   true,
			"warning": true,
			"info":    true,
		}
		if !validSeverities[c.LintMinSeverity] {
			return fmt.Errorf("invalid lint severity: %s (valid: error, warning, info)", c.LintMinSeverity)
		}
	}

	return nil
}

// GetLintDisabledRules returns the disabled rules as a slice.
func (c *Config) GetLintDisabledRules() []string {
	if c.LintDisabledRules == "" {
		return nil
	}
	rules := strings.Split(c.LintDisabledRules, ",")
	for i := range rules {
		rules[i] = strings.TrimSpace(rules[i])
	}
	return rules
}

// GetLintEnabledRules returns the enabled rules as a slice.
func (c *Config) GetLintEnabledRules() []string {
	if c.LintEnabledRules == "" {
		return nil
	}
	rules := strings.Split(c.LintEnabledRules, ",")
	for i := range rules {
		rules[i] = strings.TrimSpace(rules[i])
	}
	return rules
}

// ToAnalysisOptions converts the config to analyzer options.
func (c *Config) ToAnalysisOptions() AnalysisOptions {
	return AnalysisOptions{
		RootDir:       c.RootDir,
		ExcludeDirs:   c.ExcludeDirs,
		IncludeTests:  c.IncludeTests,
		FilterPackage: c.FilterPackage,
		FilterName:    c.FilterName,
	}
}

// AnalysisOptions represents options for the temporal analysis.
type AnalysisOptions struct {
	RootDir       string   `json:"root_dir"`
	ExcludeDirs   []string `json:"exclude_dirs,omitempty"`
	IncludeTests  bool     `json:"include_tests"`
	FilterPackage string   `json:"filter_package,omitempty"`
	FilterName    string   `json:"filter_name,omitempty"`
}
