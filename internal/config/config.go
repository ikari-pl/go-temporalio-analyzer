// Package config provides configuration management for the temporal analyzer.
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

	flag.Parse()

	return c.Validate()
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate root directory
	absRoot, err := filepath.Abs(c.RootDir)
	if err != nil {
		return fmt.Errorf("invalid root directory %s: %w", c.RootDir, err)
	}
	c.RootDir = absRoot

	if _, err := os.Stat(c.RootDir); os.IsNotExist(err) {
		return fmt.Errorf("root directory does not exist: %s", c.RootDir)
	}

	// Validate output format
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

	return nil
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
