package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/output"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/tui"

	"github.com/charmbracelet/bubbles/list"
)

func main() {
	// Create config
	cfg := config.NewConfig()

	// Parse command line flags first
	if err := cfg.ParseFlags(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	logger := NewLogger(cfg)

	// Create analyzer
	analyzerInstance := analyzer.NewAnalyzer(logger)

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
		Graph:          graph,
		AllItems:       allItems,
		List:           listModel,
		CurrentView:    cfg.DebugView,
		WindowWidth:    80,
		WindowHeight:   24,
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
