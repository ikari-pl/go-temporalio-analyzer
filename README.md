# Temporal Analyzer

A beautiful CLI/TUI tool for analyzing and visualizing Temporal.io workflow and activity connections in Go codebases.

## Features

- 🔍 **Automatic Discovery**: Scans your codebase to find all Temporal workflows and activities
- 🌳 **Relationship Mapping**: Maps the connections between workflows and activities
- 📊 **Multiple Output Formats**: Tree view, JSON, DOT (Graphviz), and Markdown
- 🎨 **Interactive TUI**: Beautiful terminal user interface for navigation
- 📈 **Statistics**: Provides insights about your temporal graph
- 🔎 **Search & Filter**: Find specific workflows or activities quickly

## Installation

From the ff repository root:

```bash
cd cmd/temporal-analyzer
go mod tidy
go build -o temporal-analyzer
```

## Usage

### Interactive TUI Mode (Default)

```bash
./temporal-analyzer
```

This opens an interactive terminal interface where you can:
- Browse all workflows and activities
- Search by name (press `/`)
- View detailed information (press `Enter` on a selected item)
- Toggle details view (press `d`)

### CLI Mode

```bash
# Generate tree output
./temporal-analyzer -interactive=false

# Export to JSON
./temporal-analyzer -interactive=false -format=json -output=temporal-graph.json

# Generate Graphviz DOT file
./temporal-analyzer -interactive=false -format=dot -output=temporal-graph.dot

# Generate Markdown documentation
./temporal-analyzer -interactive=false -format=markdown -output=temporal-docs.md

# Filter by type
./temporal-analyzer -interactive=false -filter-type=workflow

# Filter by name pattern (regex)
./temporal-analyzer -interactive=false -filter-name=".*Employee.*"

# Show detailed information
./temporal-analyzer -interactive=false -details

# Analyze specific directory
./temporal-analyzer -interactive=false -root=./pkg/temporal/workflows
```

## Output Formats

### Tree Format (Default)
```
Temporal Workflow/Activity Analysis
====================================

Statistics:
- Total Workflows: 45
- Total Activities: 78
- Max Depth: 5
- Orphan Nodes: 3

Workflow Tree:
--------------
EmployeeFilingsProcessingWorkflow [workflow]
├── GetClientReconStatus [activity]
├── GenerateEmployeeFilingData [activity]
├── RunDataAudits [activity]
└── CreateDocGenWorkflow [workflow]
    ├── GenerateDocuments [workflow]
    └── EmployeeFilingsDocGenerationActivity [activity]
```

### JSON Format
Complete machine-readable representation of the temporal graph with all metadata.

### DOT Format
Generate Graphviz diagrams:
```bash
./temporal-analyzer -format=dot -output=graph.dot
dot -Tpng graph.dot -o temporal-graph.png
```

### Markdown Format
Documentation-ready format with statistics, workflow descriptions, and call relationships.

## How It Works

The analyzer:

1. **Scans Go Files**: Recursively searches for `.go` files in workflow/activity related packages
2. **Parses AST**: Uses Go's AST parser to analyze function signatures and calls
3. **Identifies Patterns**: Recognizes Temporal workflows and activities by:
   - Function names ending in "Workflow" or "Activity"
   - Functions with `workflow.Context` or `context.Context` as first parameter
4. **Extracts Calls**: Finds `workflow.ExecuteActivity` and `workflow.ExecuteChildWorkflow` calls
5. **Builds Graph**: Creates a directed graph of all relationships
6. **Generates Output**: Provides multiple visualization and export options

## TUI Navigation

- **Arrow Keys / j,k**: Navigate list
- **Enter**: View details of selected item
- **/**: Search mode
- **d**: Toggle details in list view
- **Esc**: Go back / exit search
- **q / Ctrl+C**: Quit

## Examples

### Find All Document Generation Workflows
```bash
./temporal-analyzer -filter-name=".*[Dd]ocument.*" -details
```

### Export Employee Filing Workflows
```bash
./temporal-analyzer -filter-name=".*Employee.*" -format=json -output=employee-workflows.json
```

### Generate Visual Graph
```bash
./temporal-analyzer -format=dot -output=temporal.dot
dot -Tpng temporal.dot -o temporal-graph.png
```

## Statistics Explained

- **Total Workflows**: Number of functions identified as Temporal workflows
- **Total Activities**: Number of functions identified as Temporal activities  
- **Max Depth**: Maximum call chain depth in the workflow graph
- **Orphan Nodes**: Workflows/activities with no parents or children (potential issues)

## Supported Patterns

The analyzer recognizes these Temporal patterns:

```go
// Workflows
func SomeWorkflow(ctx workflow.Context, param SomeParam) (SomeResponse, error)
func (w *WorkflowStruct) AnotherWorkflow(ctx workflow.Context, param Param) error

// Activities  
func SomeActivity(ctx context.Context, param SomeParam) (SomeResponse, error)
func (a *ActivityStruct) AnotherActivity(ctx context.Context, param Param) error

// Calls
workflow.ExecuteActivity(ctx, activity.SomeActivity, params)
workflow.ExecuteChildWorkflow(ctx, SomeChildWorkflow, params)
```

## Contributing

To extend the analyzer:

1. **Add New Patterns**: Modify the `isWorkflowFunction` and `isActivityFunction` functions
2. **Improve Call Extraction**: Enhance `extractChildCalls` for complex call patterns
3. **Add Output Formats**: Implement new generators in the `generateOutput` function
4. **Enhance TUI**: Add new views or navigation features in the `model` struct

## Troubleshooting

### No Workflows Found
- Ensure you're running from the correct directory
- Check that your workflows follow the naming conventions
- Use `-root` flag to specify the correct path

### Missing Relationships
- The analyzer looks for direct `workflow.ExecuteActivity` calls
- Dynamic activity names or indirect calls may not be detected
- Complex reflection-based patterns may require manual annotation

### Performance Issues
- Large codebases may take time to analyze
- Use `-filter-type` or `-filter-name` to limit scope
- Consider analyzing specific subdirectories with `-root`