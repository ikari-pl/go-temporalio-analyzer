package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()
	if cfg == nil {
		t.Fatal("NewConfig returned nil")
	}

	// Check default values
	if cfg.RootDir != "." {
		t.Errorf("RootDir = %q, want %q", cfg.RootDir, ".")
	}
	if len(cfg.ExcludeDirs) != 3 {
		t.Errorf("ExcludeDirs length = %d, want 3", len(cfg.ExcludeDirs))
	}
	if cfg.OutputFormat != "tui" {
		t.Errorf("OutputFormat = %q, want %q", cfg.OutputFormat, "tui")
	}
	if cfg.GraphTool != "dot" {
		t.Errorf("GraphTool = %q, want %q", cfg.GraphTool, "dot")
	}
	if !cfg.ShowWorkflows {
		t.Error("ShowWorkflows should be true by default")
	}
	if !cfg.ShowActivities {
		t.Error("ShowActivities should be true by default")
	}
	if cfg.LintMode {
		t.Error("LintMode should be false by default")
	}
	if cfg.LintFormat != "text" {
		t.Errorf("LintFormat = %q, want %q", cfg.LintFormat, "text")
	}
	if cfg.LintMaxFanOut != 15 {
		t.Errorf("LintMaxFanOut = %d, want 15", cfg.LintMaxFanOut)
	}
	if cfg.LintMaxCallDepth != 10 {
		t.Errorf("LintMaxCallDepth = %d, want 10", cfg.LintMaxCallDepth)
	}
}

func TestValidate(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		setup   func(*Config)
		wantErr bool
	}{
		{
			name: "valid config",
			setup: func(c *Config) {
				c.RootDir = tmpDir
			},
			wantErr: false,
		},
		{
			name: "non-existent directory",
			setup: func(c *Config) {
				c.RootDir = "/non/existent/path/xyz123"
			},
			wantErr: true,
		},
		{
			name: "invalid output format",
			setup: func(c *Config) {
				c.RootDir = tmpDir
				c.OutputFormat = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid graph tool",
			setup: func(c *Config) {
				c.RootDir = tmpDir
				c.GraphTool = "invalid"
			},
			wantErr: true,
		},
		{
			name: "neither workflows nor activities",
			setup: func(c *Config) {
				c.RootDir = tmpDir
				c.ShowWorkflows = false
				c.ShowActivities = false
			},
			wantErr: true,
		},
		{
			name: "lint mode with invalid format",
			setup: func(c *Config) {
				c.RootDir = tmpDir
				c.LintMode = true
				c.LintFormat = "invalid"
			},
			wantErr: true,
		},
		{
			name: "lint mode with invalid severity",
			setup: func(c *Config) {
				c.RootDir = tmpDir
				c.LintMode = true
				c.LintMinSeverity = "invalid"
			},
			wantErr: true,
		},
		{
			name: "lint list rules skips validation",
			setup: func(c *Config) {
				c.RootDir = "/non/existent"
				c.LintListRules = true
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOutputFormats(t *testing.T) {
	tmpDir := t.TempDir()

	validFormats := []string{"tui", "json", "tree", "dot", "mermaid", "markdown", "md"}

	for _, format := range validFormats {
		t.Run("format_"+format, func(t *testing.T) {
			cfg := NewConfig()
			cfg.RootDir = tmpDir
			cfg.OutputFormat = format

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() error for format %q: %v", format, err)
			}
		})
	}
}

func TestValidateLintFormats(t *testing.T) {
	tmpDir := t.TempDir()

	validFormats := []string{"text", "text-no-color", "json", "github", "sarif", "checkstyle"}

	for _, format := range validFormats {
		t.Run("lint_format_"+format, func(t *testing.T) {
			cfg := NewConfig()
			cfg.RootDir = tmpDir
			cfg.LintMode = true
			cfg.LintFormat = format

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() error for lint format %q: %v", format, err)
			}
		})
	}
}

func TestValidateLintSeverities(t *testing.T) {
	tmpDir := t.TempDir()

	validSeverities := []string{"error", "warning", "info"}

	for _, severity := range validSeverities {
		t.Run("severity_"+severity, func(t *testing.T) {
			cfg := NewConfig()
			cfg.RootDir = tmpDir
			cfg.LintMode = true
			cfg.LintMinSeverity = severity

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() error for severity %q: %v", severity, err)
			}
		})
	}
}

func TestValidateGraphTools(t *testing.T) {
	tmpDir := t.TempDir()

	validTools := []string{"dot", "fdp", "neato", "circo"}

	for _, tool := range validTools {
		t.Run("tool_"+tool, func(t *testing.T) {
			cfg := NewConfig()
			cfg.RootDir = tmpDir
			cfg.GraphTool = tool

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() error for graph tool %q: %v", tool, err)
			}
		})
	}
}

func TestGetLintDisabledRules(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // number of rules
	}{
		{
			name:  "empty",
			input: "",
			want:  0,
		},
		{
			name:  "single rule",
			input: "TA001",
			want:  1,
		},
		{
			name:  "multiple rules",
			input: "TA001,TA002,TA003",
			want:  3,
		},
		{
			name:  "rules with spaces",
			input: "TA001, TA002, TA003",
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.LintDisabledRules = tt.input

			rules := cfg.GetLintDisabledRules()
			if len(rules) != tt.want {
				t.Errorf("GetLintDisabledRules() length = %d, want %d", len(rules), tt.want)
			}

			// Verify rules are trimmed
			for _, rule := range rules {
				if rule == "" || rule[0] == ' ' || rule[len(rule)-1] == ' ' {
					t.Errorf("Rule %q not properly trimmed", rule)
				}
			}
		})
	}
}

func TestGetLintEnabledRules(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // number of rules
	}{
		{
			name:  "empty",
			input: "",
			want:  0,
		},
		{
			name:  "single rule",
			input: "TA001",
			want:  1,
		},
		{
			name:  "multiple rules",
			input: "TA001,TA002,TA003",
			want:  3,
		},
		{
			name:  "rules with spaces",
			input: "TA001, TA002, TA003",
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.LintEnabledRules = tt.input

			rules := cfg.GetLintEnabledRules()
			if len(rules) != tt.want {
				t.Errorf("GetLintEnabledRules() length = %d, want %d", len(rules), tt.want)
			}

			// Verify rules are trimmed
			for _, rule := range rules {
				if rule == "" || rule[0] == ' ' || rule[len(rule)-1] == ' ' {
					t.Errorf("Rule %q not properly trimmed", rule)
				}
			}
		})
	}
}

func TestToAnalysisOptions(t *testing.T) {
	cfg := NewConfig()
	cfg.RootDir = "/test/path"
	cfg.ExcludeDirs = []string{"vendor"}
	cfg.IncludeTests = true
	cfg.FilterPackage = "mypackage"
	cfg.FilterName = "MyFunc.*"

	opts := cfg.ToAnalysisOptions()

	if opts.RootDir != cfg.RootDir {
		t.Errorf("RootDir = %q, want %q", opts.RootDir, cfg.RootDir)
	}
	if len(opts.ExcludeDirs) != len(cfg.ExcludeDirs) {
		t.Errorf("ExcludeDirs length = %d, want %d", len(opts.ExcludeDirs), len(cfg.ExcludeDirs))
	}
	if opts.IncludeTests != cfg.IncludeTests {
		t.Errorf("IncludeTests = %v, want %v", opts.IncludeTests, cfg.IncludeTests)
	}
	if opts.FilterPackage != cfg.FilterPackage {
		t.Errorf("FilterPackage = %q, want %q", opts.FilterPackage, cfg.FilterPackage)
	}
	if opts.FilterName != cfg.FilterName {
		t.Errorf("FilterName = %q, want %q", opts.FilterName, cfg.FilterName)
	}
}

func TestValidateRootDirAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory to test relative path conversion
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Change to tmpDir and use relative path
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	cfg := NewConfig()
	cfg.RootDir = "subdir"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	// Root dir should be converted to absolute path
	if !filepath.IsAbs(cfg.RootDir) {
		t.Errorf("RootDir should be absolute, got %q", cfg.RootDir)
	}
}

func TestAnalysisOptions(t *testing.T) {
	opts := AnalysisOptions{
		RootDir:       "/test",
		ExcludeDirs:   []string{"vendor", ".git"},
		IncludeTests:  true,
		FilterPackage: "pkg",
		FilterName:    "Func",
	}

	if opts.RootDir != "/test" {
		t.Errorf("RootDir = %q, want %q", opts.RootDir, "/test")
	}
	if len(opts.ExcludeDirs) != 2 {
		t.Errorf("ExcludeDirs length = %d, want 2", len(opts.ExcludeDirs))
	}
	if !opts.IncludeTests {
		t.Error("IncludeTests should be true")
	}
	if opts.FilterPackage != "pkg" {
		t.Errorf("FilterPackage = %q, want %q", opts.FilterPackage, "pkg")
	}
	if opts.FilterName != "Func" {
		t.Errorf("FilterName = %q, want %q", opts.FilterName, "Func")
	}
}

func TestExtractPositionalPath(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantFiltered []string
		wantPath     string
	}{
		{
			name:         "empty args",
			args:         []string{},
			wantFiltered: []string{},
			wantPath:     "",
		},
		{
			name:         "path first",
			args:         []string{".", "--lint"},
			wantFiltered: []string{"--lint"},
			wantPath:     ".",
		},
		{
			name:         "path in middle",
			args:         []string{"--lint", ".", "--format", "json"},
			wantFiltered: []string{"--lint", "--format", "json"},
			wantPath:     ".",
		},
		{
			name:         "path last",
			args:         []string{"--lint", "--format", "json", "."},
			wantFiltered: []string{"--lint", "--format", "json"},
			wantPath:     ".",
		},
		{
			name:         "absolute path",
			args:         []string{"--lint", "/path/to/project", "--verbose"},
			wantFiltered: []string{"--lint", "--verbose"},
			wantPath:     "/path/to/project",
		},
		{
			name:         "relative path with subdirs",
			args:         []string{"./pkg/workflows", "--lint"},
			wantFiltered: []string{"--lint"},
			wantPath:     "./pkg/workflows",
		},
		{
			name:         "flags only - no path",
			args:         []string{"--lint", "--verbose", "--format", "json"},
			wantFiltered: []string{"--lint", "--verbose", "--format", "json"},
			wantPath:     "",
		},
		{
			name:         "flag with = syntax",
			args:         []string{"--format=json", ".", "--lint"},
			wantFiltered: []string{"--format=json", "--lint"},
			wantPath:     ".",
		},
		{
			name:         "short flags",
			args:         []string{"-v", ".", "-d"},
			wantFiltered: []string{"-v", "-d"},
			wantPath:     ".",
		},
		{
			name:         "flag value not confused with path",
			args:         []string{"--format", "json", "."},
			wantFiltered: []string{"--format", "json"},
			wantPath:     ".",
		},
		{
			name:         "root flag value preserved",
			args:         []string{"--root", "/other/path", "."},
			wantFiltered: []string{"--root", "/other/path"},
			wantPath:     ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, path := extractPositionalPath(tt.args)

			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}

			if len(filtered) != len(tt.wantFiltered) {
				t.Errorf("filtered length = %d, want %d", len(filtered), len(tt.wantFiltered))
				t.Errorf("filtered = %v, want %v", filtered, tt.wantFiltered)
				return
			}

			for i, f := range filtered {
				if f != tt.wantFiltered[i] {
					t.Errorf("filtered[%d] = %q, want %q", i, f, tt.wantFiltered[i])
				}
			}
		})
	}
}

