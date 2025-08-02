package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CallSite represents where a call occurs
type CallSite struct {
	TargetName string `json:"target_name"`
	LineNumber int    `json:"line_number"`
}

// TemporalNode represents a workflow or activity
type TemporalNode struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // "workflow" or "activity"
	FilePath    string            `json:"file_path"`
	Package     string            `json:"package"`
	Children    []string          `json:"children"`   // activities/workflows called by this node (deprecated)
	CallSites   []CallSite        `json:"call_sites"` // actual call locations with line numbers
	Parents     []string          `json:"parents"`    // workflows that call this node
	LineNumber  int               `json:"line_number"`
	Parameters  map[string]string `json:"parameters"` // parameter types
	ReturnType  string            `json:"return_type"`
	Description string            `json:"description"`
}

// TemporalGraph represents the entire workflow/activity graph
type TemporalGraph struct {
	Nodes map[string]*TemporalNode `json:"nodes"`
	Stats AnalysisStats            `json:"stats"`
}

// AnalysisStats contains statistics about the temporal graph
type AnalysisStats struct {
	TotalWorkflows  int `json:"total_workflows"`
	TotalActivities int `json:"total_activities"`
	MaxDepth        int `json:"max_depth"`
	OrphanNodes     int `json:"orphan_nodes"`
}

// CLI configuration
type Config struct {
	OutputFormat string
	OutputFile   string
	FilterType   string
	FilterName   string
	ShowDetails  bool
	Interactive  bool
	RootDir      string
	DisplayGraph bool   // New: display graph with system tools
	GraphTool    string // New: which tool to use (dot, neato, etc.)
}

func main() {
	config := parseFlags()

	if config.Interactive {
		runTUI(config)
	} else if config.DisplayGraph {
		runGraphDisplay(config)
	} else {
		runCLI(config)
	}
}

func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.OutputFormat, "format", "tree", "Output format: tree, json, dot, markdown")
	flag.StringVar(&config.OutputFile, "output", "", "Output file (default: stdout)")
	flag.StringVar(&config.FilterType, "filter-type", "", "Filter by type: workflow, activity")
	flag.StringVar(&config.FilterName, "filter-name", "", "Filter by name pattern (regex)")
	flag.BoolVar(&config.ShowDetails, "details", false, "Show detailed information")
	flag.BoolVar(&config.Interactive, "interactive", true, "Run in interactive TUI mode")
	flag.StringVar(&config.RootDir, "root", ".", "Root directory to analyze")
	flag.BoolVar(&config.DisplayGraph, "display", false, "Generate and display graph using system tools (requires graphviz)")
	flag.StringVar(&config.GraphTool, "graph-tool", "dot", "Graph layout tool: dot, neato, fdp, sfdp, circo, twopi")

	flag.Parse()
	return config
}

func runCLI(config *Config) {
	graph, err := analyzeTemporalGraph(config.RootDir)
	if err != nil {
		log.Fatalf("Error analyzing temporal graph: %v", err)
	}

	output := generateOutput(graph, config)

	if config.OutputFile != "" {
		err := os.WriteFile(config.OutputFile, []byte(output), 0644)
		if err != nil {
			log.Fatalf("Error writing to file: %v", err)
		}
		fmt.Printf("Analysis written to %s\n", config.OutputFile)
	} else {
		fmt.Print(output)
	}
}

// analyzeTemporalGraph scans the codebase and builds the temporal graph
func analyzeTemporalGraph(rootDir string) (*TemporalGraph, error) {
	graph := &TemporalGraph{
		Nodes: make(map[string]*TemporalNode),
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "_test.go") {
			return nil
		}

		// Focus on temporal-related packages
		if !strings.Contains(path, "workflow") && !strings.Contains(path, "activities") && !strings.Contains(path, "temporal") {
			return nil
		}

		return analyzeFile(path, graph)
	})

	if err != nil {
		return nil, err
	}

	// Calculate statistics and relationships
	calculateStats(graph)
	buildRelationships(graph)

	return graph, nil
}

// analyzeFile parses a Go file and extracts temporal workflows and activities
func analyzeFile(filePath string, graph *TemporalGraph) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("error parsing file %s: %v", filePath, err)
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name != nil {
				funcName := x.Name.Name

				// Check if this is a workflow or activity
				if isWorkflowFunction(funcName, x) {
					position := fset.Position(x.Pos())
					temporalNode := &TemporalNode{
						Name:        funcName,
						Type:        "workflow",
						FilePath:    filePath,
						Package:     node.Name.Name,
						LineNumber:  position.Line,
						Children:    extractChildCalls(x),      // Keep for backward compatibility
						CallSites:   extractCallSites(x, fset), // New: actual call sites with line numbers
						Parameters:  extractParameters(x),
						ReturnType:  extractReturnType(x),
						Description: extractDescription(x),
					}
					graph.Nodes[funcName] = temporalNode
				} else if isActivityFunction(funcName, x) {
					position := fset.Position(x.Pos())
					temporalNode := &TemporalNode{
						Name:        funcName,
						Type:        "activity",
						FilePath:    filePath,
						Package:     node.Name.Name,
						LineNumber:  position.Line,
						Children:    []string{},   // Activities don't call other temporal functions
						CallSites:   []CallSite{}, // Activities don't make calls
						Parameters:  extractParameters(x),
						ReturnType:  extractReturnType(x),
						Description: extractDescription(x),
					}
					graph.Nodes[funcName] = temporalNode
				}
			}
		}
		return true
	})

	return nil
}

// isWorkflowFunction determines if a function is a Temporal workflow
func isWorkflowFunction(name string, fn *ast.FuncDecl) bool {
	// Check function name patterns
	if strings.HasSuffix(name, "Workflow") {
		return true
	}

	// Check if first parameter is workflow.Context
	if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
		firstParam := fn.Type.Params.List[0]
		if sel, ok := firstParam.Type.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				return ident.Name == "workflow" && sel.Sel.Name == "Context"
			}
		}
	}

	return false
}

// isActivityFunction determines if a function is a Temporal activity
func isActivityFunction(name string, fn *ast.FuncDecl) bool {
	// Check function name patterns
	if strings.HasSuffix(name, "Activity") {
		return true
	}

	// Check if first parameter is context.Context
	if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
		firstParam := fn.Type.Params.List[0]
		if sel, ok := firstParam.Type.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				return ident.Name == "context" && sel.Sel.Name == "Context"
			}
		}
	}

	return false
}

// extractChildCalls finds all workflow.ExecuteActivity and workflow.ExecuteChildWorkflow calls
func extractChildCalls(fn *ast.FuncDecl) []string {
	var calls []string

	ast.Inspect(fn, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "workflow" {
					if sel.Sel.Name == "ExecuteActivity" || sel.Sel.Name == "ExecuteChildWorkflow" {
						// Extract the activity/workflow name from the second argument
						if len(callExpr.Args) >= 2 {
							if ident, ok := callExpr.Args[1].(*ast.Ident); ok {
								calls = append(calls, ident.Name)
							} else if sel, ok := callExpr.Args[1].(*ast.SelectorExpr); ok {
								calls = append(calls, sel.Sel.Name)
							}
						}
					}
				}
			}
		}
		return true
	})

	return calls
}

// extractCallSites finds all workflow.ExecuteActivity and workflow.ExecuteChildWorkflow calls WITH line numbers
// Handles multiple patterns:
// 1. workflow.ExecuteActivity(ctx, ActivityName, ...)
// 2. workflow.ExecuteActivity(ctx, activityInstance.Method, ...)
// 3. workflow.ExecuteChildWorkflow(ctx, WorkflowName, ...)
func extractCallSites(fn *ast.FuncDecl, fset *token.FileSet) []CallSite {
	var callSites []CallSite

	ast.Inspect(fn, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "workflow" {
					if sel.Sel.Name == "ExecuteActivity" || sel.Sel.Name == "ExecuteChildWorkflow" {
						// Extract the activity/workflow name from the second argument
						if len(callExpr.Args) >= 2 {
							var targetName string

							// Pattern 1: Direct identifier (ActivityName)
							if ident, ok := callExpr.Args[1].(*ast.Ident); ok {
								targetName = ident.Name
							} else if sel, ok := callExpr.Args[1].(*ast.SelectorExpr); ok {
								// Pattern 2: Method call (activityInstance.Method)
								if _, ok := sel.X.(*ast.Ident); ok {
									// Use the method name as target
									targetName = sel.Sel.Name
								} else {
									// Fallback: just use the method name
									targetName = sel.Sel.Name
								}
							}

							if targetName != "" {
								position := fset.Position(callExpr.Pos())
								callSites = append(callSites, CallSite{
									TargetName: targetName,
									LineNumber: position.Line,
								})
							}
						}
					}
				}
			}
		}
		return true
	})

	return callSites
}

// extractParameters extracts function parameter information
func extractParameters(fn *ast.FuncDecl) map[string]string {
	params := make(map[string]string)

	if fn.Type.Params != nil {
		for i, param := range fn.Type.Params.List {
			for _, name := range param.Names {
				paramType := "unknown"
				if param.Type != nil {
					paramType = fmt.Sprintf("%v", param.Type)
				}
				params[fmt.Sprintf("param_%d_%s", i, name.Name)] = paramType
			}
		}
	}

	return params
}

// extractReturnType extracts function return type information
func extractReturnType(fn *ast.FuncDecl) string {
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		return fmt.Sprintf("%v", fn.Type.Results.List[0].Type)
	}
	return "void"
}

// extractDescription extracts function documentation
func extractDescription(fn *ast.FuncDecl) string {
	if fn.Doc != nil && len(fn.Doc.List) > 0 {
		return strings.TrimSpace(fn.Doc.List[0].Text)
	}
	return ""
}

// buildRelationships builds parent-child relationships in the graph
func buildRelationships(graph *TemporalGraph) {
	for _, node := range graph.Nodes {
		for _, childName := range node.Children {
			if child, exists := graph.Nodes[childName]; exists {
				child.Parents = append(child.Parents, node.Name)
			}
		}
	}
}

// calculateStats computes statistics about the temporal graph
func calculateStats(graph *TemporalGraph) {
	for _, node := range graph.Nodes {
		if node.Type == "workflow" {
			graph.Stats.TotalWorkflows++
		} else {
			graph.Stats.TotalActivities++
		}

		if len(node.Parents) == 0 && len(node.Children) == 0 {
			graph.Stats.OrphanNodes++
		}
	}

	// Calculate max depth (simplified - could be improved)
	graph.Stats.MaxDepth = calculateMaxDepth(graph)
}

// calculateMaxDepth calculates the maximum call depth in the graph
func calculateMaxDepth(graph *TemporalGraph) int {
	maxDepth := 0

	for _, node := range graph.Nodes {
		if len(node.Parents) == 0 { // Root node
			depth := calculateNodeDepth(node, graph, make(map[string]bool))
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}

	return maxDepth
}

// calculateNodeDepth calculates the depth of a specific node
func calculateNodeDepth(node *TemporalNode, graph *TemporalGraph, visited map[string]bool) int {
	if visited[node.Name] {
		return 0 // Avoid cycles
	}

	visited[node.Name] = true
	maxChildDepth := 0

	for _, childName := range node.Children {
		if child, exists := graph.Nodes[childName]; exists {
			depth := calculateNodeDepth(child, graph, visited)
			if depth > maxChildDepth {
				maxChildDepth = depth
			}
		}
	}

	delete(visited, node.Name)
	return 1 + maxChildDepth
}

// generateOutput generates the requested output format
func generateOutput(graph *TemporalGraph, config *Config) string {
	switch config.OutputFormat {
	case "json":
		return generateJSONOutput(graph)
	case "dot":
		return generateDotOutput(graph)
	case "markdown":
		return generateMarkdownOutput(graph)
	default:
		return generateTreeOutput(graph, config)
	}
}

// generateTreeOutput generates a tree-like text output
func generateTreeOutput(graph *TemporalGraph, config *Config) string {
	var output strings.Builder

	output.WriteString("Temporal Workflow/Activity Analysis\n")
	output.WriteString("====================================\n\n")

	output.WriteString(fmt.Sprintf("Statistics:\n"))
	output.WriteString(fmt.Sprintf("- Total Workflows: %d\n", graph.Stats.TotalWorkflows))
	output.WriteString(fmt.Sprintf("- Total Activities: %d\n", graph.Stats.TotalActivities))
	output.WriteString(fmt.Sprintf("- Max Depth: %d\n", graph.Stats.MaxDepth))
	output.WriteString(fmt.Sprintf("- Orphan Nodes: %d\n\n", graph.Stats.OrphanNodes))

	// Find root workflows (those with no parents)
	var rootWorkflows []*TemporalNode
	for _, node := range graph.Nodes {
		if node.Type == "workflow" && len(node.Parents) == 0 {
			rootWorkflows = append(rootWorkflows, node)
		}
	}

	// Sort root workflows by name
	sort.Slice(rootWorkflows, func(i, j int) bool {
		return rootWorkflows[i].Name < rootWorkflows[j].Name
	})

	output.WriteString("Workflow Tree:\n")
	output.WriteString("--------------\n")

	for _, root := range rootWorkflows {
		if matchesFilter(root, config) {
			printNodeTree(root, graph, &output, "", make(map[string]bool), config)
		}
	}

	return output.String()
}

// matchesFilter checks if a node matches the filter criteria
func matchesFilter(node *TemporalNode, config *Config) bool {
	if config.FilterType != "" && node.Type != config.FilterType {
		return false
	}

	if config.FilterName != "" {
		matched, _ := regexp.MatchString(config.FilterName, node.Name)
		if !matched {
			return false
		}
	}

	return true
}

// printNodeTree recursively prints the node tree
func printNodeTree(node *TemporalNode, graph *TemporalGraph, output *strings.Builder, prefix string, visited map[string]bool, config *Config) {
	if visited[node.Name] {
		output.WriteString(fmt.Sprintf("%s%s [%s] (cycle detected)\n", prefix, node.Name, node.Type))
		return
	}

	visited[node.Name] = true

	output.WriteString(fmt.Sprintf("%s%s [%s]", prefix, node.Name, node.Type))
	if config.ShowDetails {
		output.WriteString(fmt.Sprintf(" (%s:%d)", filepath.Base(node.FilePath), node.LineNumber))
	}
	output.WriteString("\n")

	// Print children
	for i, childName := range node.Children {
		if child, exists := graph.Nodes[childName]; exists && matchesFilter(child, config) {
			isLast := i == len(node.Children)-1
			childPrefix := prefix + "  "
			if isLast {
				childPrefix = prefix + "  "
			}
			printNodeTree(child, graph, output, childPrefix+"├── ", visited, config)
		}
	}

	delete(visited, node.Name)
}

// generateJSONOutput generates JSON output
func generateJSONOutput(graph *TemporalGraph) string {
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error generating JSON: %v", err)
	}
	return string(data)
}

// generateDotOutput generates DOT format for Graphviz
func generateDotOutput(graph *TemporalGraph) string {
	var output strings.Builder

	output.WriteString("digraph TemporalGraph {\n")
	output.WriteString("  rankdir=TB;\n")
	output.WriteString("  node [shape=box];\n\n")

	// Define nodes
	for _, node := range graph.Nodes {
		style := "filled"
		color := "lightblue"
		if node.Type == "activity" {
			color = "lightgreen"
		}

		output.WriteString(fmt.Sprintf("  \"%s\" [style=%s, fillcolor=%s, label=\"%s\\n[%s]\"];\n",
			node.Name, style, color, node.Name, node.Type))
	}

	output.WriteString("\n")

	// Define edges
	for _, node := range graph.Nodes {
		for _, childName := range node.Children {
			if _, exists := graph.Nodes[childName]; exists {
				output.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", node.Name, childName))
			}
		}
	}

	output.WriteString("}\n")
	return output.String()
}

// generateMarkdownOutput generates Markdown output
func generateMarkdownOutput(graph *TemporalGraph) string {
	var output strings.Builder

	output.WriteString("# Temporal Workflow/Activity Analysis\n\n")

	output.WriteString("## Statistics\n\n")
	output.WriteString(fmt.Sprintf("- **Total Workflows**: %d\n", graph.Stats.TotalWorkflows))
	output.WriteString(fmt.Sprintf("- **Total Activities**: %d\n", graph.Stats.TotalActivities))
	output.WriteString(fmt.Sprintf("- **Max Depth**: %d\n", graph.Stats.MaxDepth))
	output.WriteString(fmt.Sprintf("- **Orphan Nodes**: %d\n\n", graph.Stats.OrphanNodes))

	output.WriteString("## Workflows\n\n")

	var workflows []*TemporalNode
	for _, node := range graph.Nodes {
		if node.Type == "workflow" {
			workflows = append(workflows, node)
		}
	}

	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].Name < workflows[j].Name
	})

	for _, workflow := range workflows {
		output.WriteString(fmt.Sprintf("### %s\n\n", workflow.Name))
		output.WriteString(fmt.Sprintf("- **File**: `%s:%d`\n", workflow.FilePath, workflow.LineNumber))
		output.WriteString(fmt.Sprintf("- **Package**: `%s`\n", workflow.Package))

		if len(workflow.Children) > 0 {
			output.WriteString("- **Calls**:\n")
			for _, child := range workflow.Children {
				if childNode, exists := graph.Nodes[child]; exists {
					output.WriteString(fmt.Sprintf("  - `%s` [%s]\n", child, childNode.Type))
				} else {
					output.WriteString(fmt.Sprintf("  - `%s` [unknown]\n", child))
				}
			}
		}

		if workflow.Description != "" {
			output.WriteString(fmt.Sprintf("- **Description**: %s\n", workflow.Description))
		}

		output.WriteString("\n")
	}

	return output.String()
}

// runGraphDisplay generates a graph and displays it using system tools
func runGraphDisplay(config *Config) {
	graph, err := analyzeTemporalGraph(config.RootDir)
	if err != nil {
		log.Fatalf("Error analyzing temporal graph: %v", err)
	}

	// Generate DOT format
	dotOutput := generateDotOutput(graph)

	// Create a temporary DOT file
	dotFile := fmt.Sprintf("/tmp/temporal-graph-%d.dot", time.Now().Unix())
	svgFile := fmt.Sprintf("/tmp/temporal-graph-%d.svg", time.Now().Unix())

	err = os.WriteFile(dotFile, []byte(dotOutput), 0644)
	if err != nil {
		log.Fatalf("Error writing DOT file: %v", err)
	}

	fmt.Printf("Generated DOT file: %s\n", dotFile)

	// Generate SVG using specified graph tool
	cmd := exec.Command(config.GraphTool, "-Tsvg", "-o", svgFile, dotFile)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Error generating SVG (is %s installed?): %v", config.GraphTool, err)
	}

	fmt.Printf("Generated SVG file: %s\n", svgFile)

	// Try to open the SVG file with system default
	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", svgFile)
	case "linux":
		openCmd = exec.Command("xdg-open", svgFile)
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", svgFile)
	default:
		fmt.Printf("Generated graph at %s - please open manually\n", svgFile)
		return
	}

	err = openCmd.Start()
	if err != nil {
		fmt.Printf("Could not auto-open %s: %v\n", svgFile, err)
		fmt.Printf("Please open the file manually: %s\n", svgFile)
	} else {
		fmt.Printf("Opening graph visualization...\n")
		fmt.Printf("\n🎨 Graph Statistics:\n")
		fmt.Printf("📊 Total Workflows: %d\n", graph.Stats.TotalWorkflows)
		fmt.Printf("⚙️ Total Activities: %d\n", graph.Stats.TotalActivities)
		fmt.Printf("🔗 Max Depth: %d\n", graph.Stats.MaxDepth)
		fmt.Printf("🏝️ Orphan Nodes: %d\n", graph.Stats.OrphanNodes)
		fmt.Printf("\nFiles generated:\n")
		fmt.Printf("  📄 DOT: %s\n", dotFile)
		fmt.Printf("  🎨 SVG: %s\n", svgFile)
	}
}

// TUI Implementation starts here

// viewState represents a navigation state that can be returned to
type viewState struct {
	view         string        // "list", "details", "tree"
	selectedNode *TemporalNode // Node being viewed (for details)
	listIndex    int           // Selected item in list view
	treeIndex    int           // Selected item in tree view
	detailsIndex int           // Selected item in details view
	navPath      []navPathItem // Navigation path at this state
}

// navPathItem represents a single step in the navigation path
type navPathItem struct {
	node        *TemporalNode // The node we navigated to
	direction   string        // "→" for calls, "←" for called_by, "🌳" for tree
	displayName string        // Short name for display
}

type model struct {
	graph          *TemporalGraph
	list           list.Model
	filterInput    textinput.Model
	showDetails    bool
	currentView    string // "list", "details", "tree"
	selectedNode   *TemporalNode
	allItems       []list.Item // Keep original items
	showWorkflows  bool
	showActivities bool
	filterActive   bool // Whether filter input has focus
	windowWidth    int
	windowHeight   int
	// Navigation state stack for proper back button behavior
	stateStack []viewState // Stack of previous states
	// Navigation path tracking for breadcrumb display
	navPath []navPathItem // Breadcrumb trail of navigation
	// Details navigation - make callers/callees directly selectable
	detailsLines           []string         // All lines in details view
	detailsSelectableItems []selectableItem // Items that can be navigated to
	detailsSelected        int              // Currently selected line
	detailsScrollOffset    int              // For scrolling in details
	// Tree view
	treeItems        []treeItem // Hierarchical tree items
	treeSelected     int        // Currently selected tree item
	treeScrollOffset int        // For scrolling in tree
}

// selectableItem represents a navigable item in details view
type selectableItem struct {
	lineIndex   int           // Which line this item is on
	node        *TemporalNode // The node to navigate to
	itemType    string        // "caller", "callee"
	displayText string        // Text shown for this item
}

// pushState saves the current state to the stack before navigating
func (m *model) pushState() {
	// Make a copy of the current navigation path
	navPathCopy := make([]navPathItem, len(m.navPath))
	copy(navPathCopy, m.navPath)

	currentState := viewState{
		view:         m.currentView,
		selectedNode: m.selectedNode,
		listIndex:    m.list.Index(),
		treeIndex:    m.treeSelected,
		detailsIndex: m.detailsSelected,
		navPath:      navPathCopy,
	}
	m.stateStack = append(m.stateStack, currentState)
}

// popState returns to the previous state from the stack
func (m *model) popState() bool {
	if len(m.stateStack) == 0 {
		return false // No state to pop
	}

	// Get the previous state
	prevState := m.stateStack[len(m.stateStack)-1]
	m.stateStack = m.stateStack[:len(m.stateStack)-1] // Pop from stack

	// Restore the previous state
	m.currentView = prevState.view
	m.selectedNode = prevState.selectedNode
	// Restore navigation path
	m.navPath = make([]navPathItem, len(prevState.navPath))
	copy(m.navPath, prevState.navPath)

	switch prevState.view {
	case "list":
		m.list.Select(prevState.listIndex)
	case "details":
		if m.selectedNode != nil {
			m.buildDetailsView()
			m.detailsSelected = prevState.detailsIndex
		}
	case "tree":
		m.buildTreeView()
		m.treeSelected = prevState.treeIndex
	}

	return true
}

// treeItem represents an item in the tree view
type treeItem struct {
	node        *TemporalNode
	depth       int    // Indentation level
	displayText string // Formatted text with tree graphics
	isExpanded  bool   // Whether children are shown
	hasChildren bool   // Whether this item has children
}

type item struct {
	node *TemporalNode
}

func (i item) FilterValue() string { return i.node.Name }
func (i item) Title() string {
	// Add emoji prefix for instant visual recognition
	var icon string
	if i.node.Type == "workflow" {
		icon = "🔄 "
	} else {
		icon = "⚙️ "
	}

	name := i.node.Name
	if len(name) > 75 { // Account for emoji space
		return icon + name[:72] + "..."
	}
	return icon + name
}
func (i item) Description() string {
	// Second line: darker, no emoji, just info
	desc := fmt.Sprintf("%s 📦 %s", i.node.Type, i.node.Package)
	if len(i.node.Children) > 0 {
		desc += fmt.Sprintf(" →%d", len(i.node.Children))
	}
	if len(i.node.Parents) > 0 {
		desc += fmt.Sprintf(" ←%d", len(i.node.Parents))
	}
	return desc
}

func runTUI(config *Config) {
	graph, err := analyzeTemporalGraph(config.RootDir)
	if err != nil {
		log.Fatalf("Error analyzing temporal graph: %v", err)
	}

	// Convert nodes to list items
	var items []list.Item
	var nodes []*TemporalNode
	for _, node := range graph.Nodes {
		nodes = append(nodes, node)
	}

	// Sort nodes by name
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	for _, node := range nodes {
		items = append(items, item{node: node})
	}

	// Initialize list with custom delegate for better display
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2) // Give more space for each item
	delegate.SetSpacing(1)

	// Colorful, emoji-rich styling for maximum readability
	// Colorful normal items: different colors for workflows vs activities
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(lipgloss.Color("39")). // Bright blue for better readability
		PaddingLeft(0).
		PaddingRight(0)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(lipgloss.Color("240")). // Darker gray for second line
		PaddingLeft(0).
		PaddingRight(0)

	// Selected item: gentler highlight
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("15")). // White text
		Background(lipgloss.Color("25")). // Gentle blue background
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("250")). // Light gray text
		Background(lipgloss.Color("25")).  // Gentle blue background
		PaddingLeft(1).
		PaddingRight(1)

	// Dimmed items (if any)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.
		Foreground(lipgloss.Color("240")).
		PaddingLeft(0)
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.
		Foreground(lipgloss.Color("238")).
		PaddingLeft(0)

	// Start with reasonable defaults - will be resized in WindowSizeMsg
	l := list.New(items, delegate, 80, 30)
	l.Title = ""
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)

	// Status bar with SUPER BRIGHT YELLOW active page indicator
	l.Styles.StatusBar = l.Styles.StatusBar.
		Foreground(lipgloss.Color("15")).  // White text
		Background(lipgloss.Color("236")). // Dark gray background
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)
	l.Styles.StatusBarActiveFilter = l.Styles.StatusBarActiveFilter.
		Foreground(lipgloss.Color("226")). // SUPER BRIGHT YELLOW for active page
		Background(lipgloss.Color("236")).
		Bold(true)
	l.Styles.StatusBarFilterCount = l.Styles.StatusBarFilterCount.
		Foreground(lipgloss.Color("226")). // SUPER BRIGHT YELLOW for active indicators
		Background(lipgloss.Color("236")).
		Bold(true)

	// Clean title styling
	l.Styles.Title = l.Styles.Title.
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("0")).
		Bold(false).
		PaddingLeft(0).
		PaddingRight(0)

	// Initialize filter input
	fi := textinput.New()
	fi.Placeholder = "Type to filter by name, package, or file path..."
	fi.Width = 80

	m := model{
		graph:          graph,
		list:           l,
		filterInput:    fi,
		currentView:    "list",
		allItems:       items,
		windowWidth:    120,
		windowHeight:   30,
		showWorkflows:  true,
		showActivities: true,
		filterActive:   false,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// filterItems applies current filters (search term, workflow/activity toggles)
func (m *model) filterItems() {
	var filteredItems []list.Item
	searchTerm := strings.TrimSpace(m.filterInput.Value())

	for _, listItem := range m.allItems {
		node := listItem.(item).node

		// Apply type filters
		if !m.showWorkflows && node.Type == "workflow" {
			continue
		}
		if !m.showActivities && node.Type == "activity" {
			continue
		}

		// Apply search filter
		if searchTerm != "" {
			searchLower := strings.ToLower(searchTerm)
			nodeName := strings.ToLower(node.Name)
			nodePackage := strings.ToLower(node.Package)
			nodeFilePath := strings.ToLower(node.FilePath)

			// Debug: Check what we're actually matching against
			nameMatch := strings.Contains(nodeName, searchLower)
			packageMatch := strings.Contains(nodePackage, searchLower)
			fileMatch := strings.Contains(nodeFilePath, searchLower)

			// Must match at least one field
			if !nameMatch && !packageMatch && !fileMatch {
				continue
			}

			// Only include if at least one field matches
		}

		filteredItems = append(filteredItems, listItem)
	}

	m.list.SetItems(filteredItems)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.currentView {
		case "list":
			// Handle filter mode separately - when filter is active, most keys go to input
			if m.filterActive {
				switch msg.String() {
				case "ctrl+c":
					return m, tea.Quit
				case "q":
					// Exit filter mode, don't quit app
					m.filterActive = false
					return m, nil
				case "enter":
					// Exit filter mode and return focus to list
					m.filterActive = false
					return m, nil
				case "esc":
					// Exit filter mode
					m.filterActive = false
					return m, nil
				default:
					// All other keys go to the filter input (including 'f', 'w', 'a', etc.)
					// This will be handled in the component update section below
				}
			} else {
				// Normal list mode - handle shortcuts
				switch msg.String() {
				case "ctrl+c", "q":
					return m, tea.Quit
				case "enter":
					if selected := m.list.SelectedItem(); selected != nil {
						m.pushState() // Save current state
						m.selectedNode = selected.(item).node
						// Initialize navigation path with the first item
						m.navPath = nil                     // Reset path
						m.addToNavPath(m.selectedNode, "📁") // Starting point
						m.currentView = "details"
						m.buildDetailsView() // Build details view with selectable items
						m.detailsSelected = 0
						return m, nil // Don't pass to list when changing views
					}
				case "/", "f":
					// Focus on filter input
					m.filterActive = true
					return m, m.filterInput.Focus()
				case "w":
					m.showWorkflows = !m.showWorkflows
					m.filterItems()
					return m, nil
				case "a":
					m.showActivities = !m.showActivities
					m.filterItems()
					return m, nil
				case "r":
					// Reset all filters
					m.showWorkflows = true
					m.showActivities = true
					m.filterInput.SetValue("")
					m.filterItems()
					return m, nil
				case "t":
					// Switch to tree view
					m.pushState() // Save current state
					m.currentView = "tree"
					m.buildTreeView()
					m.treeSelected = 0
					return m, nil
				}
			}
		case "details":
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "q", "esc":
				if !m.popState() {
					// No state to pop, go to main list
					m.currentView = "list"
					m.selectedNode = nil
					m.detailsSelectableItems = nil
					m.detailsSelected = 0
				}
				return m, nil
			case "j", "down":
				if len(m.detailsSelectableItems) > 0 {
					m.detailsSelected = (m.detailsSelected + 1) % len(m.detailsSelectableItems)
					// Update scroll offset to keep selection visible
					visibleHeight := m.windowHeight - 4
					if visibleHeight < 10 {
						visibleHeight = 10
					}
					// Calculate which line the selected item appears on (approximate)
					selectedLine := 8 + m.detailsSelected*2 // Header + basic info + spacing
					if selectedLine >= m.detailsScrollOffset+visibleHeight {
						m.detailsScrollOffset = selectedLine - visibleHeight + 1
					}
				}
				return m, nil
			case "k", "up":
				if len(m.detailsSelectableItems) > 0 {
					m.detailsSelected = (m.detailsSelected - 1 + len(m.detailsSelectableItems)) % len(m.detailsSelectableItems)
					// Update scroll offset to keep selection visible
					selectedLine := 8 + m.detailsSelected*2 // Header + basic info + spacing
					if selectedLine < m.detailsScrollOffset {
						m.detailsScrollOffset = selectedLine
						if m.detailsScrollOffset < 0 {
							m.detailsScrollOffset = 0
						}
					}
				}
				return m, nil
			case "enter":
				if len(m.detailsSelectableItems) > 0 && m.detailsSelected < len(m.detailsSelectableItems) {
					selectedItem := m.detailsSelectableItems[m.detailsSelected]
					if selectedItem.node != nil {
						m.pushState() // Save current state
						// Determine direction based on item type
						direction := "→" // Default to "calls"
						if selectedItem.itemType == "caller" {
							direction = "←" // "called by"
						}
						m.addToNavPath(selectedItem.node, direction)
						m.selectedNode = selectedItem.node
						m.buildDetailsView() // Rebuild for new node
						m.detailsSelected = 0
					}
				}
				return m, nil
			case "f":
				// Allow filtering from details view
				m.currentView = "list"
				m.filterActive = true
				return m, m.filterInput.Focus()
			case "t":
				// Switch to tree view
				m.pushState() // Save current state
				m.currentView = "tree"
				m.buildTreeView()
				m.treeSelected = 0
				return m, nil
			}
		case "tree":
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "q", "esc":
				if !m.popState() {
					// No state to pop, go to main list
					m.currentView = "list"
				}
				return m, nil
			case "j", "down":
				if len(m.treeItems) > 0 {
					// Move down but don't wrap around - stop at end
					if m.treeSelected < len(m.treeItems)-1 {
						m.treeSelected++
						// Update scroll offset to keep selection visible
						visibleHeight := m.windowHeight - 6
						if visibleHeight < 5 {
							visibleHeight = 5
						}
						if m.treeSelected >= m.treeScrollOffset+visibleHeight {
							m.treeScrollOffset = m.treeSelected - visibleHeight + 1
						}
					}
				}
				return m, nil
			case "k", "up":
				if len(m.treeItems) > 0 {
					// Move up but don't wrap around - stop at beginning
					if m.treeSelected > 0 {
						m.treeSelected--
						// Update scroll offset to keep selection visible
						if m.treeSelected < m.treeScrollOffset {
							m.treeScrollOffset = m.treeSelected
						}
					}
				}
				return m, nil
			case "right", "l":
				// Expand node if it has children and is collapsed
				if len(m.treeItems) > 0 && m.treeSelected < len(m.treeItems) {
					selectedItem := m.treeItems[m.treeSelected]
					if selectedItem.node != nil && selectedItem.hasChildren && !selectedItem.isExpanded {
						// Expand the node
						m.treeItems[m.treeSelected].isExpanded = true
						// Rebuild tree to show children
						m.buildTreeView()
						// Keep selection on the same item
						for i, item := range m.treeItems {
							if item.node != nil && item.node.Name == selectedItem.node.Name && item.depth == selectedItem.depth {
								m.treeSelected = i
								break
							}
						}
					}
				}
				return m, nil
			case "left", "h":
				// Collapse node if it has children and is expanded
				if len(m.treeItems) > 0 && m.treeSelected < len(m.treeItems) {
					selectedItem := m.treeItems[m.treeSelected]
					if selectedItem.node != nil && selectedItem.hasChildren && selectedItem.isExpanded {
						// Collapse the node
						m.treeItems[m.treeSelected].isExpanded = false
						// Rebuild tree to hide children
						m.buildTreeView()
						// Keep selection on the same item
						for i, item := range m.treeItems {
							if item.node != nil && item.node.Name == selectedItem.node.Name && item.depth == selectedItem.depth {
								m.treeSelected = i
								break
							}
						}
					}
				}
				return m, nil
			case "enter":
				// Navigate to details of any selected node
				if len(m.treeItems) > 0 && m.treeSelected < len(m.treeItems) {
					selectedItem := m.treeItems[m.treeSelected]
					if selectedItem.node != nil {
						// Navigate to details
						m.pushState() // Save current state
						m.selectedNode = selectedItem.node
						// Initialize or extend navigation path from tree
						if len(m.navPath) == 0 {
							m.addToNavPath(m.selectedNode, "🌳") // From tree
						} else {
							m.addToNavPath(m.selectedNode, "🌳") // Tree navigation
						}
						m.currentView = "details"
						m.buildDetailsView()
						m.detailsSelected = 0
					}
				}
				return m, nil
			case "f":
				// Allow filtering from tree view
				m.currentView = "list"
				m.filterActive = true
				return m, m.filterInput.Focus()
			}
		}

	case tea.WindowSizeMsg:
		// Full-width UI with proper margins
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

		headerHeight := 3
		footerHeight := 2
		availableHeight := msg.Height - headerHeight - footerHeight
		if availableHeight < 10 {
			availableHeight = 10
		}

		// Make it truly full width - much more aggressive sizing
		margin := 0 // No margin for maximum width
		if msg.Width > 120 {
			margin = 1 // Tiny margin only on very wide terminals
		}
		m.list.SetSize(msg.Width-(margin*2), availableHeight)
		m.filterInput.Width = msg.Width - 15 // More room for filter input
	}

	// Only pass messages to components if we haven't handled them above
	var cmd tea.Cmd
	switch m.currentView {
	case "list":
		if m.filterActive {
			// Update filter input and apply filters in real-time
			var filterCmd tea.Cmd
			m.filterInput, filterCmd = m.filterInput.Update(msg)
			m.filterItems() // Apply filters as user types
			return m, filterCmd
		} else {
			m.list, cmd = m.list.Update(msg)
		}
	case "details":
		// Details view doesn't need to update any components
		cmd = nil
	}

	return m, cmd
}

func (m model) View() string {
	// Minimal margins for maximum width utilization
	margin := 0
	if m.windowWidth > 140 {
		margin = 1 // Only add margin on very wide terminals
	}

	// Create margin style
	marginStyle := lipgloss.NewStyle().PaddingLeft(margin).PaddingRight(margin)

	switch m.currentView {
	case "details":
		return marginStyle.Render(m.detailsView())
	case "tree":
		return marginStyle.Render(m.treeView())
	default:
		// Build the main view with persistent filter bar and margins
		header := m.headerView()
		filterBar := m.filterBarView()
		listView := m.list.View()

		content := header + "\n" + filterBar + "\n" + listView
		return marginStyle.Render(content)
	}
}

func (m model) headerView() string {
	// Ultra-bright header for maximum impact
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")). // Bright white text
		Background(lipgloss.Color("19")). // Deep blue background
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1).
		MarginBottom(0)

	currentCount := len(m.list.Items())
	totalCount := m.graph.Stats.TotalWorkflows + m.graph.Stats.TotalActivities

	// Super colorful, emoji-rich header for maximum readability
	stats := fmt.Sprintf("⚡ TEMPORAL ANALYZER | 📊 Showing %d/%d | 🔄 WF:%d | ⚙️ ACT:%d | 🔍 [f]Filter 🌳 [t]Tree 🔄 [w]Workflows ⚙️ [a]Activities 🔄 [r]Reset ❌ [q]Quit",
		currentCount,
		totalCount,
		m.graph.Stats.TotalWorkflows,
		m.graph.Stats.TotalActivities)

	return style.Render(stats)
}

func (m model) filterBarView() string {
	// Super colorful filter bar with maximum visual impact
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("90")). // Bright purple background
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("235")). // Dark gray
		PaddingLeft(1).
		PaddingRight(1)

	if m.filterActive {
		inputStyle = inputStyle.
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("226")) // Bright yellow when active
	}

	// Emoji-rich status indicators
	var status []string
	if m.showWorkflows {
		status = append(status, "🔄 ON")
	} else {
		status = append(status, "🔄 OFF")
	}

	if m.showActivities {
		status = append(status, "⚙️ ON")
	} else {
		status = append(status, "⚙️ OFF")
	}

	filterValue := m.filterInput.Value()
	if filterValue != "" {
		status = append(status, fmt.Sprintf("🔍 '%s'", filterValue))
	}

	statusText := strings.Join(status, " | ")

	// Colorful prompts with emojis
	var prompt string
	if m.filterActive {
		prompt = "🔍 FILTER ▶ "
	} else {
		prompt = "🔍 filter ▶ "
	}

	return fmt.Sprintf("%s │ %s",
		statusStyle.Render(statusText),
		inputStyle.Render(prompt+m.filterInput.View()))
}

// buildDetailsView creates the details view with directly selectable callers/callees
func (m *model) buildDetailsView() {
	if m.selectedNode == nil {
		return
	}

	m.detailsSelectableItems = nil

	// Build selectable items from calls first, then parents
	for _, callSite := range m.selectedNode.CallSites {
		if childNode, exists := m.graph.Nodes[callSite.TargetName]; exists {
			m.detailsSelectableItems = append(m.detailsSelectableItems, selectableItem{
				lineIndex:   len(m.detailsSelectableItems), // Use index in selectable list
				node:        childNode,
				itemType:    "callee",
				displayText: fmt.Sprintf("%s [%s]", callSite.TargetName, childNode.Type),
			})
		}
	}

	// Add parents to selectable items
	for _, parent := range m.selectedNode.Parents {
		if parentNode, exists := m.graph.Nodes[parent]; exists {
			m.detailsSelectableItems = append(m.detailsSelectableItems, selectableItem{
				lineIndex:   len(m.detailsSelectableItems), // Use index in selectable list
				node:        parentNode,
				itemType:    "caller",
				displayText: fmt.Sprintf("%s [%s]", parent, parentNode.Type),
			})
		}
	}
}

func (m model) detailsView() string {
	if m.selectedNode == nil {
		return "No node selected"
	}

	var allLines []string

	// Super bright details header for maximum visibility
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).  // Black text
		Background(lipgloss.Color("51")). // Bright cyan background
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	allLines = append(allLines, headerStyle.Render(fmt.Sprintf(" %s [%s] ", m.selectedNode.Name, m.selectedNode.Type)))
	allLines = append(allLines, "")

	// Add navigation path if it exists
	if navPath := m.renderNavPath(); navPath != "" {
		allLines = append(allLines, navPath)
		allLines = append(allLines, "")
	}

	// Basic info with colorful emojis for visual richness
	allLines = append(allLines, fmt.Sprintf("📁 File: %s:%d", m.selectedNode.FilePath, m.selectedNode.LineNumber))
	allLines = append(allLines, fmt.Sprintf("📦 Package: %s", m.selectedNode.Package))

	if m.selectedNode.Description != "" {
		allLines = append(allLines, fmt.Sprintf("Description: %s", m.selectedNode.Description))
	}

	allLines = append(allLines, "")

	// Parameters with colorful emoji
	if len(m.selectedNode.Parameters) > 0 {
		allLines = append(allLines, "🎯 Parameters:")
		for param, paramType := range m.selectedNode.Parameters {
			allLines = append(allLines, fmt.Sprintf("  %s: %s", param, paramType))
		}
		allLines = append(allLines, "")
	}

	// Track current selectable item for highlighting
	selectableIndex := 0

	// Children (calls) with colorful emoji for visual distinction
	if len(m.selectedNode.CallSites) > 0 {
		allLines = append(allLines, "📞 Calls:")

		for _, callSite := range m.selectedNode.CallSites {
			if childNode, exists := m.graph.Nodes[callSite.TargetName]; exists {
				// Colorful emojis for instant visual recognition
				typeIndicator := "⚙️ activity"
				if childNode.Type == "workflow" {
					typeIndicator = "🔄 workflow"
				}
				// Call location display with emoji - show where this call happens
				callLocation := fmt.Sprintf(" 📍 at %s:%d", filepath.Base(m.selectedNode.FilePath), callSite.LineNumber)

				baseText := fmt.Sprintf("  %s [%s]%s", callSite.TargetName, typeIndicator, callLocation)

				// Check if this is the currently selected item
				if selectableIndex == m.detailsSelected && len(m.detailsSelectableItems) > 0 {
					// Highlight this line
					highlightStyle := lipgloss.NewStyle().
						Foreground(lipgloss.Color("0")).
						Background(lipgloss.Color("226")). // Bright yellow
						Bold(true)
					allLines = append(allLines, highlightStyle.Render("▶ "+strings.TrimPrefix(baseText, "  ")))
				} else {
					allLines = append(allLines, baseText)
				}

				selectableIndex++
			} else {
				allLines = append(allLines, fmt.Sprintf("  %s [❓ unknown] 📍 at %s:%d", callSite.TargetName, filepath.Base(m.selectedNode.FilePath), callSite.LineNumber))
			}
		}
		allLines = append(allLines, "")
	}

	// Parents (called by) with colorful emoji and highlighting
	if len(m.selectedNode.Parents) > 0 {
		allLines = append(allLines, "📤 Called by:")
		for _, parent := range m.selectedNode.Parents {
			if parentNode, exists := m.graph.Nodes[parent]; exists {
				// Colorful emojis for parent type distinction
				typeIndicator := "⚙️ activity"
				if parentNode.Type == "workflow" {
					typeIndicator = "🔄 workflow"
				}

				// Find where in the parent this function is called
				var callLocation string
				for _, callSite := range parentNode.CallSites {
					if callSite.TargetName == m.selectedNode.Name {
						callLocation = fmt.Sprintf(" 📍 at %s:%d", filepath.Base(parentNode.FilePath), callSite.LineNumber)
						break
					}
				}
				if callLocation == "" {
					// Fallback if we can't find the call site
					callLocation = fmt.Sprintf(" 📍 in %s", filepath.Base(parentNode.FilePath))
				}

				baseText := fmt.Sprintf("  %s [%s]%s", parent, typeIndicator, callLocation)

				// Check if this is the currently selected item
				if selectableIndex == m.detailsSelected && len(m.detailsSelectableItems) > 0 {
					// Highlight this line
					highlightStyle := lipgloss.NewStyle().
						Foreground(lipgloss.Color("0")).
						Background(lipgloss.Color("226")). // Bright yellow
						Bold(true)
					allLines = append(allLines, highlightStyle.Render("▶ "+strings.TrimPrefix(baseText, "  ")))
				} else {
					allLines = append(allLines, baseText)
				}

				selectableIndex++
			} else {
				allLines = append(allLines, fmt.Sprintf("  %s [unknown]", parent))
			}
		}
		allLines = append(allLines, "")
	}

	// Add viewport scrolling - calculate visible window
	visibleHeight := m.windowHeight - 4 // Account for margins and footer
	if visibleHeight < 10 {
		visibleHeight = 10
	}

	// Apply scrolling to show the visible portion
	var visibleLines []string
	if len(allLines) <= visibleHeight {
		// All lines fit, show them all
		visibleLines = allLines
	} else {
		// Need scrolling - show visible window
		startIdx := m.detailsScrollOffset
		endIdx := startIdx + visibleHeight
		if endIdx > len(allLines) {
			endIdx = len(allLines)
		}
		if startIdx >= len(allLines) {
			startIdx = len(allLines) - visibleHeight
			if startIdx < 0 {
				startIdx = 0
			}
		}
		visibleLines = allLines[startIdx:endIdx]
	}

	// Colorful footer with navigation instructions and scroll indicator
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).  // Bright white text
		Background(lipgloss.Color("240")). // Dark gray background
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	visibleLines = append(visibleLines, "")
	navInstructions := "⬅️ [q]Back 🌳 [t]Tree 🔍 [f]Filter ❌ [Ctrl+C]Quit"
	if len(m.detailsSelectableItems) > 0 {
		scrollInfo := ""
		if len(allLines) > visibleHeight {
			scrollInfo = fmt.Sprintf(" | Lines %d-%d/%d", m.detailsScrollOffset+1, m.detailsScrollOffset+len(visibleLines)-1, len(allLines))
		}
		navInstructions = fmt.Sprintf("▶️ [j/k]Navigate [Enter]Go (%d/%d)%s ⬅️ [q]Back 🌳 [t]Tree 🔍 [f]Filter", m.detailsSelected+1, len(m.detailsSelectableItems), scrollInfo)
	}
	visibleLines = append(visibleLines, footerStyle.Render(navInstructions))

	return strings.Join(visibleLines, "\n")
}

// buildTreeView creates a hierarchical tree view with callers as parents
func (m *model) buildTreeView() {
	// Save current expansion states before clearing
	expansionStates := make(map[string]bool)
	for _, item := range m.treeItems {
		if item.node != nil {
			key := fmt.Sprintf("%s:%d", item.node.Name, item.depth)
			expansionStates[key] = item.isExpanded
		}
	}

	m.treeItems = nil

	// Find root nodes (workflows/activities that aren't called by others)
	rootNodes := make([]*TemporalNode, 0)
	for _, node := range m.graph.Nodes {
		if len(node.Parents) == 0 {
			rootNodes = append(rootNodes, node)
		}
	}

	// Sort root nodes by name
	sort.Slice(rootNodes, func(i, j int) bool {
		return rootNodes[i].Name < rootNodes[j].Name
	})

	// Build tree starting from root nodes
	for _, rootNode := range rootNodes {
		m.addTreeItemRecursive(rootNode, 0, make(map[string]bool), true, expansionStates) // Root nodes are always "expanded"
	}
}

// addTreeItemRecursive recursively adds a node and its callees to the tree
func (m *model) addTreeItemRecursive(node *TemporalNode, depth int, visited map[string]bool, parentExpanded bool, expansionStates map[string]bool) {
	// Prevent infinite recursion
	if visited[node.Name] {
		return
	}
	visited[node.Name] = true

	hasChildren := len(node.CallSites) > 0

	// Create tree graphics with proper indentation
	indent := ""
	for i := 0; i < depth; i++ {
		if i == depth-1 {
			indent += "├── " // ├──
		} else {
			indent += "│   " // │
		}
	}

	// Node type icon
	var icon string
	if node.Type == "workflow" {
		icon = "🔄"
	} else {
		icon = "⚙️"
	}

	// Get expansion state from saved states, or default based on depth
	key := fmt.Sprintf("%s:%d", node.Name, depth)
	isExpanded, exists := expansionStates[key]
	if !exists {
		// Default: root nodes with children start expanded
		isExpanded = depth == 0 && hasChildren
	}

	// Expansion icon based on state
	var expandIcon string
	if hasChildren {
		if isExpanded {
			expandIcon = "[-]" // Expanded (showing children)
		} else {
			expandIcon = "[+]" // Collapsed (can expand)
		}
	} else {
		expandIcon = " • " // Leaf node (no children)
	}

	displayText := fmt.Sprintf("%s%s %s %s", indent, expandIcon, icon, node.Name)

	// Add this item to the tree only if parent is expanded (or this is a root node)
	if parentExpanded || depth == 0 {
		m.treeItems = append(m.treeItems, treeItem{
			node:        node,
			depth:       depth,
			displayText: displayText,
			isExpanded:  isExpanded,
			hasChildren: hasChildren,
		})
	}

	// Add children (callees) recursively only if this node is expanded
	if hasChildren && isExpanded && (parentExpanded || depth == 0) {
		// Sort children by name for consistent display
		childNodes := make([]*TemporalNode, 0)
		for _, callSite := range node.CallSites {
			if childNode, exists := m.graph.Nodes[callSite.TargetName]; exists {
				childNodes = append(childNodes, childNode)
			}
		}

		sort.Slice(childNodes, func(i, j int) bool {
			return childNodes[i].Name < childNodes[j].Name
		})

		// Add children recursively with increased depth
		for _, child := range childNodes {
			// Use a copy of visited map for each branch to allow nodes to appear in multiple places
			branchVisited := make(map[string]bool)
			for k, v := range visited {
				branchVisited[k] = v
			}

			// Add child with parentExpanded=true since we're expanded
			m.addTreeItemRecursive(child, depth+1, branchVisited, true, expansionStates)
		}
	}

	// Remove from visited for this branch
	delete(visited, node.Name)
}

// treeView renders the tree view
func (m model) treeView() string {
	if len(m.treeItems) == 0 {
		return "No tree items to display"
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("17")).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	// Count nodes with children for debug
	nodesWithChildren := 0
	for _, item := range m.treeItems {
		if item.hasChildren {
			nodesWithChildren++
		}
	}

	lines := []string{
		headerStyle.Render(fmt.Sprintf("🌳 Tree - %d nodes (%d parents) | ←→/hl = expand/collapse, Enter = details", len(m.treeItems), nodesWithChildren)),
		"",
	}

	// Calculate visible window with scrolling
	visibleHeight := m.windowHeight - 6 // Account for header, footer, margins
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	// Use current scroll offset (managed in Update function)

	// Render visible tree items
	for i := m.treeScrollOffset; i < len(m.treeItems) && i < m.treeScrollOffset+visibleHeight; i++ {
		item := m.treeItems[i]
		if i == m.treeSelected {
			// Highlight selected item
			highlightStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("226")). // Bright yellow
				Bold(true)
			lines = append(lines, highlightStyle.Render("▶ "+item.displayText))
		} else {
			lines = append(lines, "  "+item.displayText)
		}
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("240")).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	lines = append(lines, "")

	// Add scroll indicator to footer
	footerText := fmt.Sprintf("🌳 [j/k]Navigate [→/l]Expand [←/h]Collapse [Enter]Details ⬅️ [q]Back 🔍 [f]Filter | Item %d/%d", m.treeSelected+1, len(m.treeItems))
	lines = append(lines, footerStyle.Render(footerText))

	return strings.Join(lines, "\n")
}
