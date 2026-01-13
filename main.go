package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/lint"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/output"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/tui"

	"github.com/charmbracelet/bubbles/list"
)

// Version information (set via ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Handle --version before anything else (check args directly)
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" || arg == "-v" {
			fmt.Printf("temporal-analyzer %s\n", Version)
			fmt.Printf("Built: %s\n", BuildTime)
			return
		}
	}

	// Handle "lint" subcommand: transform to --lint flag for compatibility
	// This allows: `temporal-analyzer lint [flags] [path]`
	// to work the same as: `temporal-analyzer --lint [flags] [path]`
	os.Args = transformLintSubcommand(os.Args)

	// Create config
	cfg := config.NewConfig()

	// Parse command line flags
	if err := cfg.ParseFlags(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle --lint-rules: list available rules and exit
	if cfg.LintListRules {
		listLintRules()
		return
	}

	// Create logger
	logger := NewLogger(cfg)

	// Create analyzer
	analyzerInstance := analyzer.NewAnalyzer(logger)

	// Handle lint mode separately
	if cfg.LintMode {
		exitCode := runLint(cfg, logger, analyzerInstance)
		os.Exit(exitCode)
	}

	// Create TUI (only needed for tui format)
	var tuiApp tui.TUI
	if cfg.OutputFormat == "tui" || cfg.DebugView != "" {
		tuiApp = tui.NewTUI(logger)
	}

	// Run the application
	if err := run(cfg, logger, analyzerInstance, tuiApp); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// NewLogger creates a new structured logger.
func NewLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelWarn // Default to warn for cleaner output
	if cfg.Debug {
		level = slog.LevelDebug
	} else if cfg.Verbose {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	return slog.New(handler)
}

// run is the main application function.
func run(
	cfg *config.Config,
	logger *slog.Logger,
	analyzerInstance analyzer.Analyzer,
	tuiApp tui.TUI,
) error {
	logger.Info("Starting temporal analyzer",
		"root_dir", cfg.RootDir,
		"format", cfg.OutputFormat)

	// Create analysis options
	opts := cfg.ToAnalysisOptions()

	// Perform analysis
	ctx := context.Background()
	graph, err := analyzerInstance.Analyze(ctx, opts)
	if err != nil {
		logger.Error("Failed to analyze workflows", "error", err)
		return err
	}

	logger.Info("Analysis completed",
		"workflows", graph.Stats.TotalWorkflows,
		"activities", graph.Stats.TotalActivities,
		"total_nodes", len(graph.Nodes))

	// Handle debug view rendering
	if cfg.DebugView != "" {
		return renderDebugView(cfg, graph)
	}

	// Handle different output formats
	switch cfg.OutputFormat {
	case "tui":
		if tuiApp == nil {
			return fmt.Errorf("TUI not initialized")
		}
		return tuiApp.Run(ctx, graph)

	case "json":
		formatter := output.NewJSONFormatter()
		return formatter.Format(ctx, graph, os.Stdout)

	case "dot":
		exporter := output.NewExporter()
		dot, err := exporter.ExportDOT(graph)
		if err != nil {
			return err
		}
		fmt.Println(dot)
		return nil

	case "mermaid":
		exporter := output.NewExporter()
		mermaid, err := exporter.ExportMermaid(graph)
		if err != nil {
			return err
		}
		fmt.Println(mermaid)
		return nil

	case "markdown", "md":
		exporter := output.NewExporter()
		md, err := exporter.ExportMarkdown(graph)
		if err != nil {
			return err
		}
		fmt.Println(md)
		return nil

	default:
		return fmt.Errorf("unsupported output format: %s (supported: tui, json, dot, mermaid, markdown)", cfg.OutputFormat)
	}
}

// renderDebugView renders a single view for debugging without TUI interaction.
func renderDebugView(cfg *config.Config, graph *analyzer.TemporalGraph) error {
	// Create TUI components for debugging
	navigator := tui.NewNavigator()
	styles := tui.NewStyleManager()
	filter := tui.NewFilterManager()
	viewManager := tui.NewViewManager(styles, filter)

	// Create all items list
	allItems := make([]list.Item, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		allItems = append(allItems, tui.ListItem{Node: node})
	}

	// Create initial list items - only top-level workflows (no parents)
	initialItems := make([]list.Item, 0)
	for _, item := range allItems {
		li := item.(tui.ListItem)
		// Show only top-level workflows (no parents)
		if len(li.Node.Parents) == 0 && li.Node.Type == "workflow" {
			initialItems = append(initialItems, item)
		}
	}

	// Fall back to all top-level nodes if no workflows found
	if len(initialItems) == 0 {
		for _, item := range allItems {
			li := item.(tui.ListItem)
			if len(li.Node.Parents) == 0 {
				initialItems = append(initialItems, item)
			}
		}
	}

	// Create list model with initial filtered items
	delegate := list.NewDefaultDelegate()
	listModel := list.New(initialItems, delegate, 80, 20)
	listModel.Title = "Temporal Workflows & Activities"

	state := &tui.State{
		Graph:        graph,
		AllItems:     allItems,
		List:         listModel,
		CurrentView:  cfg.DebugView,
		WindowWidth:  80,
		WindowHeight: 24,
		ListState: &tui.ListViewState{
			Items: initialItems,
		},
		TreeState: &tui.TreeViewState{
			ExpansionStates: make(map[string]bool),
		},
		DetailsState:   nil,
		Navigator:      navigator,
		ShowWorkflows:  true,
		ShowActivities: false, // Initially hide activities (show only top-level workflows)
		ShowSignals:    false,
		ShowQueries:    false,
		ShowUpdates:    false,
		FilterActive:   false,
	}

	// Set up for details view debug
	if cfg.DebugView == "details" && len(graph.Nodes) > 0 {
		for _, node := range graph.Nodes {
			if node.Type == "workflow" {
				state.SelectedNode = node
				break
			}
		}
		if state.SelectedNode == nil {
			for _, node := range graph.Nodes {
				state.SelectedNode = node
				break
			}
		}
	}

	// Get the view and render it
	view := viewManager.GetView(cfg.DebugView)
	if view == nil {
		return fmt.Errorf("unknown debug view: %s (available: list, tree, details, stats, help)", cfg.DebugView)
	}

	// Render the view
	fmt.Println(view.Render(state))
	return nil
}

// runLint executes the linter and returns the exit code.
func runLint(cfg *config.Config, logger *slog.Logger, analyzerInstance analyzer.Analyzer) int {
	logger.Info("Starting temporal analyzer in lint mode",
		"root_dir", cfg.RootDir,
		"format", cfg.LintFormat,
		"strict", cfg.LintStrict,
		"llm_enhance", cfg.LLMEnhance,
		"llm_verify", cfg.LLMVerify)

	// Create analysis options
	opts := cfg.ToAnalysisOptions()

	// Perform analysis
	ctx := context.Background()
	graph, err := analyzerInstance.Analyze(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error analyzing workflows: %v\n", err)
		return 2 // Analysis error
	}
	if graph == nil {
		fmt.Fprintf(os.Stderr, "Error: analyzer returned nil graph\n")
		return 2 // Analysis error
	}

	logger.Info("Analysis completed",
		"workflows", graph.Stats.TotalWorkflows,
		"activities", graph.Stats.TotalActivities,
		"total_nodes", len(graph.Nodes))

	// Create linter config from CLI options
	lintCfg := &lint.Config{
		MinSeverity:   severityFromString(cfg.LintMinSeverity),
		EnabledRules:  cfg.GetLintEnabledRules(),
		DisabledRules: cfg.GetLintDisabledRules(),
		FailOnWarning: cfg.LintStrict,
		Thresholds: lint.Thresholds{
			MaxFanOut:          cfg.LintMaxFanOut,
			MaxCallDepth:       cfg.LintMaxCallDepth,
			VersioningRequired: 5,
		},
		// LLM enhancement options
		LLMEnhance: cfg.LLMEnhance,
		LLMVerify:  cfg.LLMVerify,
		LLMModel:   cfg.LLMModel,
		RootDir:    cfg.RootDir,
	}

	// Create linter and run
	linter := lint.NewLinter(lintCfg)
	result := linter.Run(ctx, graph)

	// Output results in all requested formats
	formats := cfg.LintFormats
	if len(formats) == 0 {
		formats = []string{cfg.LintFormat}
	}

	for i, format := range formats {
		formatter := lint.NewFormatter(format)

		// Determine output destination for this format
		var out *os.File
		var outputPath string

		if i == 0 {
			// First format goes to stdout or the specified output file
			if cfg.OutputFile != "" {
				outputPath = cfg.OutputFile
			}
		} else {
			// Additional formats go to auto-generated files
			baseName := "lint-results"
			if cfg.OutputFile != "" {
				baseName = strings.TrimSuffix(filepath.Base(cfg.OutputFile), filepath.Ext(cfg.OutputFile))
			}
			outputPath = baseName + config.GetLintFormatExtension(format)
		}

		if outputPath != "" {
			f, err := os.Create(outputPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating output file %s: %v\n", outputPath, err)
				return 2
			}
			out = f
			defer func(f *os.File) { _ = f.Close() }(f)

			if len(formats) > 1 {
				logger.Info("Writing output", "format", format, "file", outputPath)
			}
		} else {
			out = os.Stdout
		}

		if err := formatter.Format(result, out); err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting results as %s: %v\n", format, err)
			return 2
		}
	}

	return result.ExitCode
}

// listLintRules prints all available lint rules.
func listLintRules() {
	linter := lint.NewLinter(lint.DefaultConfig())
	rules := linter.ListRules()

	fmt.Println("\nTemporal Analyzer - Available Lint Rules")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Group by category
	categories := make(map[lint.Category][]lint.RuleInfo)
	for _, rule := range rules {
		categories[rule.Category] = append(categories[rule.Category], rule)
	}

	categoryOrder := []lint.Category{
		lint.CategoryReliability,
		lint.CategoryBestPractice,
		lint.CategoryPerformance,
		lint.CategoryMaintenance,
		lint.CategorySecurity,
	}

	for _, cat := range categoryOrder {
		catRules, ok := categories[cat]
		if !ok || len(catRules) == 0 {
			continue
		}

		fmt.Printf("  %s\n", categoryTitle(cat))
		fmt.Println("  " + strings.Repeat("─", 60))
		for _, rule := range catRules {
			severityIcon := "ℹ"
			switch rule.Severity {
			case lint.SeverityError:
				severityIcon = "✖"
			case lint.SeverityWarning:
				severityIcon = "⚠"
			}
			fmt.Printf("    %s %-8s %-30s %s\n", severityIcon, rule.ID, rule.Name, rule.Severity)
			fmt.Printf("              %s\n", rule.Description)
			fmt.Println()
		}
	}

	fmt.Println("Usage:")
	fmt.Println("  temporal-analyzer --lint                    # Run all rules")
	fmt.Println("  temporal-analyzer --lint --lint-strict      # Fail on warnings")
	fmt.Println("  temporal-analyzer --lint --lint-disable TA001,TA002")
	fmt.Println("  temporal-analyzer --lint --lint-format github  # For GitHub Actions")
	fmt.Println()
}

func categoryTitle(cat lint.Category) string {
	switch cat {
	case lint.CategoryReliability:
		return "🔒 Reliability"
	case lint.CategoryBestPractice:
		return "✨ Best Practices"
	case lint.CategoryPerformance:
		return "⚡ Performance"
	case lint.CategoryMaintenance:
		return "🔧 Maintenance"
	case lint.CategorySecurity:
		return "🛡️  Security"
	default:
		return string(cat)
	}
}

func severityFromString(s string) lint.Severity {
	switch s {
	case "error":
		return lint.SeverityError
	case "warning":
		return lint.SeverityWarning
	case "info":
		return lint.SeverityInfo
	default:
		return lint.SeverityInfo
	}
}

// transformLintSubcommand transforms "lint" subcommand style into flag style.
// This allows: `temporal-analyzer lint --format=github ./...`
// to work the same as: `temporal-analyzer --lint --lint-format=github ./...`
func transformLintSubcommand(args []string) []string {
	if len(args) < 2 {
		return args
	}

	// Check if first argument after program name is "lint"
	if args[1] != "lint" {
		return args
	}

	// Transform the arguments:
	// 1. Remove "lint" subcommand and add --lint flag
	// 2. Transform --format= to --lint-format= (common mistake)
	// 3. Transform -format= to -lint-format=
	newArgs := make([]string, 0, len(args))
	newArgs = append(newArgs, args[0]) // program name
	newArgs = append(newArgs, "--lint")

	for i := 2; i < len(args); i++ {
		arg := args[i]

		// Transform --format=X or -format=X to --lint-format=X in lint mode
		if strings.HasPrefix(arg, "--format=") {
			arg = "--lint-format=" + strings.TrimPrefix(arg, "--format=")
		} else if strings.HasPrefix(arg, "-format=") {
			arg = "--lint-format=" + strings.TrimPrefix(arg, "-format=")
		} else if arg == "--format" || arg == "-format" {
			// Handle --format X (space-separated) form
			arg = "--lint-format"
		}

		// Transform github-actions to github (the actual valid format name)
		if strings.HasSuffix(arg, "=github-actions") {
			arg = strings.TrimSuffix(arg, "=github-actions") + "=github"
		}

		newArgs = append(newArgs, arg)
	}

	return newArgs
}
