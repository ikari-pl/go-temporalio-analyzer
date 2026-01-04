# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Overview

Temporal Analyzer is a CLI/TUI tool for analyzing and visualizing Temporal.io workflow and activity connections in Go codebases. It parses Go source code to identify Temporal workflows and activities, builds a dependency graph, and provides multiple visualization options including an interactive terminal UI.

This is a standalone command-line tool within the larger Filing Factory (ff) monorepo, located in `cmd/temporal-analyzer`.

## Development Commands

### Build
```bash
# Build the binary
cd cmd/temporal-analyzer
go build -o temporal-analyzer

# Or build from ff root
go build -o temporal-analyzer ./cmd/temporal-analyzer
```

### Run
```bash
# Run interactively (default TUI mode)
./temporal-analyzer

# Run in CLI mode with JSON output
./temporal-analyzer -interactive=false -format=json

# Debug a specific view
./temporal-analyzer -debug-view=list
./temporal-analyzer -debug-view=details
./temporal-analyzer -debug-view=tree
```

### Test
```bash
# Run tests from temporal-analyzer directory
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -v ./internal/analyzer -run TestParseDirectory
```

### Dependencies
```bash
# Install/update dependencies
go mod tidy

# Download dependencies
go mod download
```

## Architecture

### Dependency Injection with fx
The application uses [uber-go/fx](https://uber-go.github.io/fx/) for dependency injection and lifecycle management. All components are wired together in `main.go`:
- Configuration is parsed once and injected into all components
- Logger, analyzer components, output formatters, and TUI components are all provided via fx
- The `run` function is invoked by fx to execute the main application logic

### Core Layers

#### 1. Analyzer Layer (`internal/analyzer/`)
Responsible for parsing Go source files and building the Temporal workflow graph.

**Key Components:**
- **Parser**: Parses Go files using Go's AST package, identifies workflow and activity functions based on:
  - Function names ending in "Workflow" or "Activity"
  - First parameter is `workflow.Context` or `context.Context`
- **CallExtractor**: Extracts temporal calls (`workflow.ExecuteActivity`, `workflow.ExecuteChildWorkflow`) from function bodies
- **GraphBuilder**: Constructs a directed graph of nodes (workflows/activities) and their call relationships
- **Repository**: Handles persistence (saving/loading graphs to/from JSON files)
- **Service**: High-level business logic wrapper around analysis operations
- **Analyzer**: Main orchestrator that coordinates all analyzer components

**Data Model:**
- `TemporalNode`: Represents a single workflow or activity with metadata (name, type, package, file location, parameters, call sites)
- `TemporalGraph`: Complete graph with all nodes and computed statistics
- `CallSite`: Represents where a workflow/activity is called from

#### 2. TUI Layer (`internal/tui/`)
Provides an interactive terminal user interface using the Bubble Tea framework.

**Key Components:**
- **TUI**: Main Bubble Tea application that manages the event loop
- **ViewManager**: Manages different views (list, tree, details) and handles view-specific rendering
- **Navigator**: Handles navigation history and view transitions (back/forward navigation)
- **FilterManager**: Manages filtering of workflows/activities by type or name
- **StyleManager**: Centralizes all styling and theming for consistent UI appearance

**View Types:**
- **List View**: Shows all workflows/activities in a searchable list
- **Tree View**: Hierarchical visualization of workflow call chains
- **Details View**: Shows detailed information about a selected node including callers and callees

**State Management:**
- Single `State` struct contains all application state
- View-specific state is tracked in `ListViewState`, `TreeViewState`, and `DetailsViewState`
- Navigation history maintained by the Navigator component

#### 3. Output Layer (`internal/output/`)
Formats analysis results for non-interactive output.

**Supported Formats:**
- **JSON**: Machine-readable format with complete graph data
- Future formats can be added by implementing the `Formatter` interface

#### 4. Config Layer (`internal/config/`)
Handles command-line flags and configuration parsing. Converts config to `AnalysisOptions` for the analyzer.

### Key Interfaces

All major components are defined by interfaces to enable dependency injection and testing:
- `analyzer.Analyzer`: Main analysis interface
- `analyzer.Parser`: AST parsing
- `analyzer.CallExtractor`: Call extraction
- `analyzer.GraphBuilder`: Graph construction
- `tui.TUI`: Terminal UI
- `tui.View`: Individual view rendering
- `output.Formatter`: Output formatting

## Temporal Workflow Patterns

The analyzer recognizes standard Temporal.io patterns used in the Filing Factory codebase:

**Workflow Functions:**
```go
func SomeWorkflow(ctx workflow.Context, param SomeParam) (SomeResponse, error)
func (w *WorkflowStruct) AnotherWorkflow(ctx workflow.Context, param Param) error
```

**Activity Functions:**
```go
func SomeActivity(ctx context.Context, param SomeParam) (SomeResponse, error)
func (a *ActivityStruct) AnotherActivity(ctx context.Context, param Param) error
```

**Recognized Calls:**
```go
workflow.ExecuteActivity(ctx, activity.SomeActivity, params)
workflow.ExecuteChildWorkflow(ctx, SomeChildWorkflow, params)
```

## Code Organization

```
cmd/temporal-analyzer/
├── main.go                 # Entry point, fx wiring, logger setup
├── go.mod                  # Module dependencies
├── internal/
│   ├── analyzer/          # Core analysis logic
│   │   ├── analyzer.go    # Main analyzer implementation
│   │   ├── parser.go      # Go AST parsing
│   │   ├── extractor.go   # Call extraction
│   │   ├── graph.go       # Graph builder
│   │   ├── repository.go  # Graph persistence
│   │   ├── service.go     # Business logic wrapper
│   │   ├── types.go       # Data structures
│   │   └── interfaces.go  # Component interfaces
│   ├── tui/              # Terminal UI
│   │   ├── tui.go        # Main Bubble Tea app
│   │   ├── views.go      # View implementations
│   │   ├── viewmanager.go # View management
│   │   ├── navigator.go  # Navigation history
│   │   ├── filter.go     # Filtering logic
│   │   ├── styles.go     # UI styling
│   │   ├── types.go      # State structures
│   │   └── interfaces.go # TUI interfaces
│   ├── output/           # Output formatting
│   │   ├── json.go       # JSON formatter
│   │   └── interfaces.go # Formatter interface
│   └── config/           # Configuration
│       └── config.go     # Config parsing
└── README.md             # User documentation
```

## Important Implementation Details

### AST Parsing Strategy
The parser uses Go's `go/ast` and `go/parser` packages to analyze source code statically. It:
1. Recursively walks directories to find `.go` files
2. Parses each file into an AST
3. Walks the AST to find function declarations
4. Checks function signatures against Temporal patterns
5. Extracts call sites by walking function bodies looking for `ExecuteActivity` and `ExecuteChildWorkflow` calls

### Graph Building
The graph builder:
1. Creates nodes for all discovered workflows and activities
2. Links nodes based on extracted call sites
3. Calculates parent-child relationships (who calls whom)
4. Computes statistics (max depth, orphan nodes, totals)

### TUI Event Loop
The Bubble Tea framework provides a Model-View-Update (MVU) architecture:
- `Init()`: Initializes the model
- `Update(msg)`: Handles events (keypresses, window resizes) and updates state
- `View()`: Renders the current state to a string

The ViewManager delegates rendering to specific view implementations based on the current view state.

## Common Development Patterns

### Adding a New View
1. Define view-specific state struct in `internal/tui/types.go`
2. Implement the `View` interface in `internal/tui/views.go`
3. Register the view in `ViewManager.GetView()`
4. Add navigation logic to reach the new view

### Adding a New Output Format
1. Create a new formatter file in `internal/output/`
2. Implement the `Formatter` interface
3. Provide the formatter via fx in `main.go`
4. Add format handling in the main `run()` function

### Extending Pattern Recognition
Modify `internal/analyzer/parser.go`:
- Update `IsWorkflow()` or `IsActivity()` to recognize new patterns
- Update `ExtractCalls()` to handle new call patterns
- Consider edge cases like method receivers, pointer types, interface implementations

## Dependencies

Key external dependencies:
- **github.com/charmbracelet/bubbletea**: TUI framework (Elm-inspired architecture)
- **github.com/charmbracelet/bubbles**: Reusable TUI components (list, text input)
- **github.com/charmbracelet/lipgloss**: Terminal styling library
- **go.uber.org/fx**: Dependency injection framework

All dependencies use semantic versioning and are pinned in `go.mod`.

## Testing Considerations

When writing tests:
- Use fx for dependency injection in tests
- Mock interfaces (Parser, CallExtractor, etc.) for unit testing
- Test AST parsing with small, focused Go code snippets
- Test graph building with synthetic node data
- Test TUI components by calling `Update()` with mock messages and verifying state changes

## Relationship to Filing Factory

This tool is part of the Filing Factory monorepo but is self-contained:
- It analyzes Temporal workflows in the larger ff codebase (run from `../../` to analyze the entire monorepo)
- It has its own `go.mod` and can be built independently
- It does not depend on ff packages - it only parses Go source code generically
- The parent ff codebase uses Temporal extensively for workflow orchestration (see `pkg/temporal/` in the root)
