# Temporal Analyzer

A **beautiful**, **production-ready** CLI/TUI tool for analyzing and visualizing Temporal.io workflow and activity connections in Go codebases.

![Demo](https://img.shields.io/badge/TUI-Beautiful-blueviolet?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)
![Temporal](https://img.shields.io/badge/Temporal-SDK-FF6B6B?style=for-the-badge)

## ✨ Features

### 🔍 Complete Temporal SDK Analysis
- **Workflows** - Detect and analyze all Temporal workflows
- **Activities** - Find activities and their callers
- **Signals** - Discover signal handlers and signal channels
- **Queries** - Identify query handlers
- **Updates** - Find update handlers (Temporal SDK 1.20+)
- **Timers** - Track `workflow.Sleep` and `workflow.NewTimer` calls
- **Versioning** - Detect `workflow.GetVersion` usage
- **Search Attributes** - Find `UpsertSearchAttributes` calls
- **Continue-as-New** - Identify workflow continuation patterns

### 🎨 Beautiful Terminal UI
- **Modern Design** - Inspired by popular terminal aesthetics
- **Gradient Headers** - Eye-catching visual hierarchy
- **Color-coded Types** - Instantly identify node types
- **Rounded Borders** - Polished, professional look
- **Responsive Layout** - Adapts to terminal size

### 📊 Multiple Views
- **List View** - Browse all workflows and activities
- **Tree View** - Visualize call hierarchy with expandable nodes
- **Details View** - Deep-dive into node connections
- **Stats Dashboard** - At-a-glance metrics
- **Help Overlay** - In-app keyboard reference

### 🚀 Export Formats
- **JSON** - Machine-readable full graph export
- **DOT** - Graphviz format for visual diagrams
- **Mermaid** - Embed diagrams in Markdown
- **Markdown** - Documentation-ready format

## 📦 Installation

```bash
cd cmd/temporal-analyzer
go build -o temporal-analyzer
```

## 🎯 Usage

### Interactive TUI Mode (Default)

```bash
./temporal-analyzer
```

Opens a beautiful terminal interface where you can:
- Browse workflows, activities, signals, and queries
- Navigate the call hierarchy in tree view
- View detailed node information
- Search and filter by name

### CLI Export Modes

```bash
# Export to JSON
./temporal-analyzer -format=json > graph.json

# Generate Graphviz DOT file
./temporal-analyzer -format=dot > temporal.dot
dot -Tpng temporal.dot -o temporal-graph.png

# Generate Mermaid diagram
./temporal-analyzer -format=mermaid > diagram.md

# Generate Markdown documentation
./temporal-analyzer -format=markdown > TEMPORAL.md
```

### Debug View Modes (No Interaction)

```bash
# Preview list view
./temporal-analyzer -debug-view=list

# Preview tree view
./temporal-analyzer -debug-view=tree

# Preview stats dashboard
./temporal-analyzer -debug-view=stats

# Preview help screen
./temporal-analyzer -debug-view=help
```

### Advanced Options

```bash
# Analyze specific directory
./temporal-analyzer -root=./pkg/workflows

# Include test files
./temporal-analyzer -include-tests

# Filter by package name (regex)
./temporal-analyzer -package=".*workflow.*"

# Filter by function name (regex)
./temporal-analyzer -name=".*Employee.*"

# Verbose logging
./temporal-analyzer -verbose

# Debug mode
./temporal-analyzer -debug
```

## ⌨️ Keyboard Shortcuts

### Navigation
| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Select / Open details |
| `Esc` / `q` | Go back / Quit |
| `g` | Go to top |
| `G` | Go to bottom |

### Views
| Key | Action |
|-----|--------|
| `1` | List view |
| `2` | Tree view |
| `3` | Stats dashboard |
| `t` | Toggle tree view |
| `?` | Help |

### Filtering
| Key | Action |
|-----|--------|
| `/` | Search / Filter |
| `w` | Toggle workflows |
| `a` | Toggle activities |
| `s` | Toggle signals |
| `C` | Clear all filters |

### Tree View
| Key | Action |
|-----|--------|
| `h` / `←` | Collapse node |
| `l` / `→` | Expand node |
| `e` | Expand all |
| `c` | Collapse all |

### Details View
| Key | Action |
|-----|--------|
| `j` / `k` | Navigate items |
| `Enter` | Go to selected |

## 🎨 Theme

The analyzer uses a beautiful dark theme inspired by modern terminal aesthetics:

- **Base**: Deep space dark (`#0d1117`)
- **Workflows**: Purple (`#a371f7`)
- **Activities**: Green (`#7ee787`)
- **Signals**: Orange (`#ffa657`)
- **Queries**: Blue (`#79c0ff`)
- **Updates**: Red (`#ff7b72`)

## 📈 Statistics

The stats dashboard shows:

| Metric | Description |
|--------|-------------|
| **Workflows** | Total Temporal workflows |
| **Activities** | Total Temporal activities |
| **Signals** | Signal handlers and channels |
| **Queries** | Query handlers |
| **Max Depth** | Deepest call chain |
| **Orphans** | Disconnected nodes |
| **Fan-Out** | Average connections per node |

## 🔬 Detection Patterns

The analyzer recognizes these Temporal patterns:

```go
// Workflows (detected by name or context type)
func SomeWorkflow(ctx workflow.Context, ...) (Result, error)

// Activities (detected by name or context type)
func SomeActivity(ctx context.Context, ...) (Result, error)

// Signal Handlers
workflow.SetSignalHandler(ctx, "signal-name", handler)
workflow.GetSignalChannel(ctx, "signal-name")

// Query Handlers
workflow.SetQueryHandler(ctx, "query-name", handler)

// Update Handlers (SDK 1.20+)
workflow.SetUpdateHandler(ctx, "update-name", handler)

// Timers
workflow.Sleep(ctx, duration)
workflow.NewTimer(ctx, duration)

// Versioning
workflow.GetVersion(ctx, "change-id", minVersion, maxVersion)

// Child Workflows
workflow.ExecuteChildWorkflow(ctx, ChildWorkflow, args)

// Activities
workflow.ExecuteActivity(ctx, SomeActivity, args)
workflow.ExecuteLocalActivity(ctx, LocalActivity, args)
```

## 🏗️ Architecture

```
internal/
├── analyzer/        # Core analysis engine
│   ├── analyzer.go  # Main analyzer interface
│   ├── parser.go    # Go AST parsing
│   ├── extractor.go # Temporal pattern extraction
│   ├── graph.go     # Dependency graph builder
│   ├── types.go     # Data structures
│   └── service.go   # Business logic
├── config/          # Configuration management
├── output/          # Export formatters
│   ├── json.go      # JSON export
│   └── exporter.go  # DOT, Mermaid, Markdown
└── tui/             # Terminal UI
    ├── theme/       # Color theme system
    ├── views.go     # View implementations
    ├── tui.go       # Main TUI controller
    ├── styles.go    # Styling system
    └── types.go     # UI data structures
```

## 🤝 Contributing

1. **Add New Patterns**: Extend `parser.go` for new detection patterns
2. **Improve UI**: Add new views in `views.go`
3. **Add Exports**: Create new formatters in `output/`
4. **Enhance Theme**: Modify `theme/theme.go` for styling

## 📄 License

MIT
