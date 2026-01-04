# Temporal Analyzer

A **beautiful**, **production-ready** CLI/TUI tool for analyzing and visualizing Temporal.io workflow and activity connections in Go codebases.

![Demo](https://img.shields.io/badge/TUI-Beautiful-blueviolet?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go)
![Temporal](https://img.shields.io/badge/Temporal-SDK-FF6B6B?style=for-the-badge)
[![codecov](https://codecov.io/gh/ikari-pl/go-temporalio-analyzer/branch/main/graph/badge.svg)](https://codecov.io/gh/ikari-pl/go-temporalio-analyzer)
[![CI](https://github.com/ikari-pl/go-temporalio-analyzer/actions/workflows/ci.yml/badge.svg)](https://github.com/ikari-pl/go-temporalio-analyzer/actions/workflows/ci.yml)

If you find this tool useful, please consider buying me a coffee!

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/A0A4GUGRG)

## 🌈🦄 Beauty ✨

https://github.com/user-attachments/assets/c8424a87-9bf9-492a-9f08-109331338c0a



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

### 🔧 CI/CD Lint Mode
- **Lint Mode** - Non-interactive analysis with exit codes for CI
- **Multiple Formats** - Text, JSON, GitHub Actions, SARIF, Checkstyle
- **Configurable Rules** - Enable/disable specific checks
- **Strict Mode** - Fail on warnings for strict pipelines

## 📦 Installation

### Using Make (Recommended)

```bash
# Clone the repository
git clone https://github.com/ikari-pl/go-temporalio-analyzer.git
cd go-temporalio-analyzer

# Build the binary
make build

# Install to ~/.local/bin (make sure it's in your PATH)
make install

# Or install globally (requires sudo)
make install-global
```

### Using Go Install

```bash
go install github.com/ikari-pl/go-temporalio-analyzer@latest
```

### From Source

```bash
go build -o temporal-analyzer .
```

## 🎯 Usage

### Interactive TUI Mode (Default)

```bash
# Analyze current directory
temporal-analyzer

# Analyze a specific project (positional argument)
temporal-analyzer /path/to/your/project

# Or use the --root flag
temporal-analyzer --root /path/to/your/project
```

Opens a beautiful terminal interface where you can:
- Browse workflows, activities, signals, and queries
- Navigate the call hierarchy in tree view
- View detailed node information
- Search and filter by name

### CLI Export Modes

```bash
# Export to JSON
temporal-analyzer --format json > graph.json

# Generate Graphviz DOT file
temporal-analyzer --format dot > temporal.dot
dot -Tpng temporal.dot -o temporal-graph.png

# Generate Mermaid diagram
temporal-analyzer --format mermaid > diagram.md

# Generate Markdown documentation
temporal-analyzer --format markdown > TEMPORAL.md

# Combine positional path with export format
temporal-analyzer /path/to/project --format mermaid
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

### 🔧 Lint Mode (CI/CD Integration)

The lint mode provides non-interactive analysis with proper exit codes for CI/CD pipelines:

```bash
# Run lint analysis (exit code 0 if no errors, 1 otherwise)
temporal-analyzer --lint

# Lint a specific project
temporal-analyzer --lint /path/to/project

# Strict mode - treat warnings as errors
temporal-analyzer --lint --lint-strict

# List all available lint rules
temporal-analyzer --lint-rules

# Output in different formats
temporal-analyzer --lint --lint-format text      # Human-readable (default)
temporal-analyzer --lint --lint-format json      # Machine-parseable JSON
temporal-analyzer --lint --lint-format github    # GitHub Actions annotations
temporal-analyzer --lint --lint-format sarif     # SARIF format (GitHub Code Scanning)
temporal-analyzer --lint --lint-format checkstyle # Checkstyle XML

# Disable specific rules
temporal-analyzer --lint --lint-disable TA001,TA002

# Only run specific rules
temporal-analyzer --lint --lint-enable TA010,TA020

# Set minimum severity level
temporal-analyzer --lint --lint-level warning   # error, warning, info

# Configure thresholds
temporal-analyzer --lint --lint-max-fan-out 20 --lint-max-depth 15

# Output to file
temporal-analyzer --lint --lint-format sarif --output results.sarif
```

#### GitHub Actions Example

```yaml
name: Temporal Workflow Analysis
on: [push, pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Install Temporal Analyzer
        run: go install github.com/ikari-pl/go-temporalio-analyzer@latest
      
      - name: Run Lint
        run: temporal-analyzer --lint --lint-format github --lint-strict .
```

#### Available Lint Rules

| ID | Name | Severity | Description | Fix |
|----|------|----------|-------------|-----|
| TA001 | activity-without-retry | warning | Transient failures (network, restarts) become permanent failures without retries | ✅ |
| TA002 | activity-without-timeout | error | Hung activities block workflows forever, wasting resources | ✅ |
| TA003 | long-activity-without-heartbeat | warning | Worker crashes (OOMKill, scale-down) cause slow retries without heartbeats | ✅ |
| TA010 | circular-dependency | error | A↔B deadlocks never resolve and cascade into system-wide issues | |
| TA011 | orphan-node | warning | Dead code adds maintenance burden and confuses developers | |
| TA020 | high-fan-out | warning | High coupling increases blast radius and indicates missing abstractions | |
| TA021 | deep-call-chain | warning | Deep chains hurt debugging, latency, and comprehension | |
| TA030 | workflow-without-versioning | info | Deploying changes can break long-running workflows mid-execution | 📝 |
| TA031 | signal-without-handler | warning | Unhandled signals are silently dropped—a hidden failure mode | |
| TA032 | query-without-return | info | Queries that return nothing defeat their inspection purpose | |
| TA033 | continue-as-new-risk | info | Without termination conditions, workflows run forever | |

✅ = insertable code fix, 📝 = code template

#### Code Fix Suggestions

Rules marked with ✅ include insertable code fixes in JSON and SARIF output. Rules marked with 📝 provide code templates. These can be used by:
- **GitHub Code Scanning** - SARIF format includes fixes that GitHub can display
- **IDE integrations** - JSON output includes fix suggestions for editor plugins  
- **Custom tooling** - Parse the JSON/SARIF output to apply fixes programmatically

Example fix in JSON output:
```json
{
  "fix": {
    "description": "Add retry policy to activity options",
    "replacements": [{
      "filePath": "workflow.go",
      "startLine": 42,
      "newText": "ao := workflow.ActivityOptions{\n\tStartToCloseTimeout: 10 * time.Minute,\n\tRetryPolicy: &temporal.RetryPolicy{\n\t\tMaximumAttempts: 3,\n\t},\n}\nctx = workflow.WithActivityOptions(ctx, ao)"
    }]
  }
}
```

### Advanced Options

```bash
# Analyze specific directory (positional or flag)
temporal-analyzer ./pkg/workflows
temporal-analyzer --root ./pkg/workflows

# Include test files
temporal-analyzer --include-tests

# Filter by package name (regex)
temporal-analyzer --package ".*workflow.*"

# Filter by function name (regex)
temporal-analyzer --name ".*Employee.*"

# Verbose logging
temporal-analyzer --verbose

# Debug mode
temporal-analyzer --debug

# Version info
temporal-analyzer --version
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
├── lint/            # CI/CD lint mode
│   ├── linter.go    # Lint orchestrator
│   ├── rules.go     # Lint rule definitions
│   └── formatters.go # Output formatters (JSON, GitHub, SARIF, etc.)
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
