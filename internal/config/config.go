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
	LintMode          bool     `json:"lint_mode"`           // Enable lint mode for CI
	LintFormat        string   `json:"lint_format"`         // "text", "json", "github", "sarif", "checkstyle" (comma-separated for multiple)
	LintFormats       []string `json:"-"`                   // Parsed list of formats
	LintStrict        bool     `json:"lint_strict"`         // Treat warnings as errors
	LintMinSeverity   string `json:"lint_min_severity"`   // "error", "warning", "info"
	LintDisabledRules string `json:"lint_disabled_rules"` // Comma-separated rule IDs to disable
	LintEnabledRules  string `json:"lint_enabled_rules"`  // Comma-separated rule IDs to enable (exclusive)
	LintListRules     bool   `json:"lint_list_rules"`     // List available lint rules and exit

	// Lint thresholds
	LintMaxFanOut    int `json:"lint_max_fan_out"`    // Max allowed fan-out before warning
	LintMaxCallDepth int `json:"lint_max_call_depth"` // Max call chain depth before warning

	// LLM enhancement options
	LLMEnhance bool   `json:"llm_enhance"` // Use LLM to generate context-aware fixes
	LLMVerify  bool   `json:"llm_verify"`  // Use LLM to verify/filter findings
	LLMModel   string `json:"llm_model"`   // Override OpenAI model (default: gpt-4o-mini)
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

		// LLM defaults
		LLMEnhance: false,
		LLMVerify:  false,
		LLMModel:   "", // Empty means use default (gpt-4o-mini)
	}
}

// ParseFlags parses command line flags and updates the config.
// Supports optional positional argument anywhere: temporal-analyzer [flags] [path] [flags]
// The path can appear before, after, or between flags.
func (c *Config) ParseFlags() error {
	// Pre-process args to extract positional path argument from anywhere in the command line
	// This allows: `temporal-analyzer --lint . --format json` to work correctly
	args, positionalPath := extractPositionalPath(os.Args[1:])

	// Create a new flag set for clean parsing
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Track if --root was explicitly set
	rootSet := false

	fs.StringVar(&c.RootDir, "root", c.RootDir, "Root directory to analyze (alternative: positional arg)")
	fs.StringVar(&c.FilterPackage, "package", c.FilterPackage, "Filter by package name (regex)")
	fs.StringVar(&c.FilterName, "name", c.FilterName, "Filter by function name (regex)")
	fs.StringVar(&c.OutputFormat, "format", c.OutputFormat, "Output format (tui, json, tree, dot)")
	fs.StringVar(&c.OutputFile, "output", c.OutputFile, "Output file (defaults to stdout)")
	fs.StringVar(&c.GraphTool, "graph-tool", c.GraphTool, "Graph layout tool (dot, fdp, neato, circo)")
	fs.BoolVar(&c.IncludeTests, "include-tests", c.IncludeTests, "Include test files in analysis")
	fs.BoolVar(&c.ShowWorkflows, "workflows", c.ShowWorkflows, "Show workflows")
	fs.BoolVar(&c.ShowActivities, "activities", c.ShowActivities, "Show activities")
	fs.BoolVar(&c.Verbose, "verbose", c.Verbose, "Verbose output")
	fs.BoolVar(&c.Debug, "debug", c.Debug, "Debug output")
	fs.StringVar(&c.DebugView, "debug-view", c.DebugView, "Debug view rendering (list, tree, details)")

	// Lint flags
	fs.BoolVar(&c.LintMode, "lint", c.LintMode, "Enable lint mode for CI (non-interactive)")
	fs.StringVar(&c.LintFormat, "lint-format", c.LintFormat, "Lint output format (text, json, github, sarif, checkstyle)")
	fs.BoolVar(&c.LintStrict, "lint-strict", c.LintStrict, "Treat warnings as errors (useful for CI)")
	fs.StringVar(&c.LintMinSeverity, "lint-level", c.LintMinSeverity, "Minimum severity to report (error, warning, info)")
	fs.StringVar(&c.LintDisabledRules, "lint-disable", c.LintDisabledRules, "Comma-separated rule IDs to disable")
	fs.StringVar(&c.LintEnabledRules, "lint-enable", c.LintEnabledRules, "Comma-separated rule IDs to enable (exclusive)")
	fs.BoolVar(&c.LintListRules, "lint-rules", c.LintListRules, "List all available lint rules and exit")
	fs.IntVar(&c.LintMaxFanOut, "lint-max-fan-out", c.LintMaxFanOut, "Max fan-out before warning (default: 15)")
	fs.IntVar(&c.LintMaxCallDepth, "lint-max-depth", c.LintMaxCallDepth, "Max call chain depth before warning (default: 10)")

	// LLM enhancement flags
	fs.BoolVar(&c.LLMEnhance, "llm-enhance", c.LLMEnhance, "Use LLM to generate context-aware code fixes (requires OPENAI_API_KEY)")
	fs.BoolVar(&c.LLMVerify, "llm-verify", c.LLMVerify, "Use LLM to verify findings and reduce false positives (requires OPENAI_API_KEY)")
	fs.StringVar(&c.LLMModel, "llm-model", c.LLMModel, "Override OpenAI model (default: gpt-4o-mini)")

	// Custom usage message
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [path] [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Analyze Temporal.io workflows and activities in a Go project.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  path\n")
		fmt.Fprintf(os.Stderr, "        Path to the project to analyze (default: current directory)\n")
		fmt.Fprintf(os.Stderr, "        Can appear anywhere in the command line\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if --root was explicitly provided
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "root" {
			rootSet = true
		}
	})

	// Use positional path if found and --root wasn't explicitly set
	if positionalPath != "" && !rootSet {
		c.RootDir = positionalPath
	}

	return c.Validate()
}

// extractPositionalPath separates flags from a positional path argument.
// It identifies the first argument that looks like a path (doesn't start with -)
// and isn't a value for a flag that takes a value.
// Returns the filtered args (flags only) and the extracted path.
func extractPositionalPath(args []string) ([]string, string) {
	if len(args) == 0 {
		return args, ""
	}

	// Flags that take a value (need to skip their next arg)
	// NOTE: Keep this map in sync with flag definitions in loadFromFlags()
	flagsWithValue := map[string]bool{
		"-root": true, "--root": true,
		"-package": true, "--package": true,
		"-name": true, "--name": true,
		"-format": true, "--format": true,
		"-output": true, "--output": true,
		"-graph-tool": true, "--graph-tool": true,
		"-debug-view": true, "--debug-view": true,
		"-lint-format": true, "--lint-format": true,
		"-lint-level": true, "--lint-level": true,
		"-lint-disable": true, "--lint-disable": true,
		"-lint-enable": true, "--lint-enable": true,
		"-lint-max-fan-out": true, "--lint-max-fan-out": true,
		"-lint-max-depth": true, "--lint-max-depth": true,
		"-llm-model": true, "--llm-model": true,
	}

	// Pre-allocate with capacity hint for efficiency
	filtered := make([]string, 0, len(args))
	var positionalPath string
	skipNext := false

	for i, arg := range args {
		if skipNext {
			filtered = append(filtered, arg)
			skipNext = false
			continue
		}

		// Check if this is a flag
		if strings.HasPrefix(arg, "-") {
			filtered = append(filtered, arg)

			// Check if this flag takes a value (and value isn't using = syntax)
			if flagsWithValue[arg] && !strings.Contains(arg, "=") {
				skipNext = true
			}
			continue
		}

		// This is a non-flag argument - treat as path if we haven't found one yet
		if positionalPath == "" {
			positionalPath = arg
		} else {
			// Multiple positional args - keep subsequent ones (shouldn't happen, but be safe)
			_ = i // silence unused warning
		}
	}

	return filtered, positionalPath
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

		// Parse comma-separated formats
		c.LintFormats = nil
		for _, f := range strings.Split(c.LintFormat, ",") {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			if !validLintFormats[f] {
				return fmt.Errorf("invalid lint format: %s (valid: text, json, github, sarif, checkstyle)", f)
			}
			c.LintFormats = append(c.LintFormats, f)
		}
		if len(c.LintFormats) == 0 {
			c.LintFormats = []string{"text"}
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

// GetLintFormatExtension returns the file extension for a lint format.
func GetLintFormatExtension(format string) string {
	switch format {
	case "json":
		return ".json"
	case "sarif":
		return ".sarif"
	case "checkstyle":
		return ".xml"
	case "github":
		return ".txt" // GitHub annotations are text-based
	default:
		return ".txt"
	}
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
