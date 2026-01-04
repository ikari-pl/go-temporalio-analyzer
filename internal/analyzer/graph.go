package analyzer

import (
	"context"
	"fmt"
	"go/ast"
	"log/slog"
)

// graphBuilder implements the GraphBuilder interface.
type graphBuilder struct {
	logger        *slog.Logger
	callExtractor CallExtractor
}

// NewGraphBuilder creates a new GraphBuilder instance.
func NewGraphBuilder(logger *slog.Logger, extractor CallExtractor) GraphBuilder {
	return &graphBuilder{
		logger:        logger,
		callExtractor: extractor,
	}
}

// BuildGraph creates a temporal graph from the given parsed nodes.
func (g *graphBuilder) BuildGraph(ctx context.Context, nodes []NodeMatch) (*TemporalGraph, error) {
	graph := &TemporalGraph{
		Nodes: make(map[string]*TemporalNode),
		Stats: GraphStats{},
	}

	// First pass: create nodes
	for _, match := range nodes {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		node, err := g.createNodeFromMatch(ctx, match)
		if err != nil {
			g.logger.Warn("Failed to create node from match", "error", err)
			continue
		}

		graph.Nodes[node.Name] = node
	}

	// Second pass: build relationships and extract temporal info
	for _, match := range nodes {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		err := g.buildRelationships(ctx, match, graph)
		if err != nil {
			fn := match.Node.(*ast.FuncDecl)
			g.logger.Warn("Failed to build relationships", "node", fn.Name.Name, "error", err)
		}
	}

	// Calculate statistics
	if err := g.CalculateStats(ctx, graph); err != nil {
		return nil, fmt.Errorf("failed to calculate stats: %w", err)
	}

	g.logger.Info("Built temporal graph",
		"workflows", graph.Stats.TotalWorkflows,
		"activities", graph.Stats.TotalActivities,
		"signals", graph.Stats.TotalSignals,
		"queries", graph.Stats.TotalQueries,
		"max_depth", graph.Stats.MaxDepth)

	return graph, nil
}

// createNodeFromMatch creates a TemporalNode from a NodeMatch.
func (g *graphBuilder) createNodeFromMatch(ctx context.Context, match NodeMatch) (*TemporalNode, error) {
	fn, ok := match.Node.(*ast.FuncDecl)
	if !ok {
		return nil, fmt.Errorf("expected *ast.FuncDecl, got %T", match.Node)
	}

	if fn.Name == nil {
		return nil, fmt.Errorf("function declaration has no name")
	}

	// Get position information
	pos := match.FileSet.Position(fn.Pos())

	// Extract parameters
	parameters := g.callExtractor.ExtractParameters(fn)

	// Extract description from comments
	description := g.extractDescription(fn)

	// Extract return type
	returnType := g.extractReturnType(fn)

	node := &TemporalNode{
		Name:        fn.Name.Name,
		Type:        match.NodeType,
		Package:     match.Package,
		FilePath:    match.FilePath,
		LineNumber:  pos.Line,
		Description: description,
		Parameters:  parameters,
		ReturnType:  returnType,
		CallSites:   []CallSite{},
		Parents:     []string{},
		Signals:     []SignalDef{},
		Queries:     []QueryDef{},
		Updates:     []UpdateDef{},
		Timers:      []TimerDef{},
		SearchAttrs: []SearchAttrDef{},
		Versioning:  []VersionDef{},
	}

	return node, nil
}

// buildRelationships builds call relationships between nodes.
func (g *graphBuilder) buildRelationships(ctx context.Context, match NodeMatch, graph *TemporalGraph) error {
	fn, ok := match.Node.(*ast.FuncDecl)
	if !ok {
		return fmt.Errorf("expected *ast.FuncDecl, got %T", match.Node)
	}

	if fn.Name == nil {
		return fmt.Errorf("function declaration has no name")
	}

	nodeName := fn.Name.Name
	node, exists := graph.Nodes[nodeName]
	if !exists {
		return fmt.Errorf("node %s not found in graph", nodeName)
	}

	// Use the enhanced extractor if available
	if extractor, ok := g.callExtractor.(*callExtractor); ok {
		// Extract all temporal information
		details, err := extractor.ExtractAllTemporalInfo(ctx, fn, match.FilePath, match.FileSet)
		if err != nil {
			return fmt.Errorf("failed to extract temporal info: %w", err)
		}

		if details != nil {
			node.CallSites = details.CallSites
			node.Signals = details.Signals
			node.Queries = details.Queries
			node.Updates = details.Updates
			node.Timers = details.Timers
			node.Versioning = details.Versions
			node.SearchAttrs = details.SearchAttrs

			// Build parent relationships
			for _, callSite := range details.CallSites {
				if targetNode, exists := graph.Nodes[callSite.TargetName]; exists {
					targetNode.Parents = g.addUniqueParent(targetNode.Parents, nodeName)
				}
			}
		}

		// Extract internal (non-Temporal) function calls
		internalCalls := extractor.extractInternalCalls(ctx, fn, match.FilePath, match.FileSet)
		if len(internalCalls) > 0 {
			node.InternalCalls = internalCalls
		}
	} else {
		// Fallback to the basic extractor
		callSites, err := g.callExtractor.ExtractCalls(ctx, fn, match.FilePath)
		if err != nil {
			return fmt.Errorf("failed to extract calls: %w", err)
		}

		node.CallSites = callSites

		for _, callSite := range callSites {
			if targetNode, exists := graph.Nodes[callSite.TargetName]; exists {
				targetNode.Parents = g.addUniqueParent(targetNode.Parents, nodeName)
			}
		}
	}

	return nil
}

// CalculateStats computes statistics for the given graph.
func (g *graphBuilder) CalculateStats(ctx context.Context, graph *TemporalGraph) error {
	stats := GraphStats{}
	
	var totalFanOut int
	var nodeCount int

	for _, node := range graph.Nodes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Count by type
		switch node.Type {
		case "workflow":
			stats.TotalWorkflows++
		case "activity":
			stats.TotalActivities++
		case "signal", "signal_handler":
			stats.TotalSignals++
		case "query", "query_handler":
			stats.TotalQueries++
		case "update", "update_handler":
			stats.TotalUpdates++
		}

		// Count signals, queries, updates, timers within nodes
		stats.TotalSignals += len(node.Signals)
		stats.TotalQueries += len(node.Queries)
		stats.TotalUpdates += len(node.Updates)
		stats.TotalTimers += len(node.Timers)

		// Count connections
		fanOut := len(node.CallSites)
		stats.TotalConnections += fanOut
		totalFanOut += fanOut
		nodeCount++

		// Track max fan-out
		if fanOut > stats.MaxFanOut {
			stats.MaxFanOut = fanOut
		}

		// Count orphan nodes (no parents and no children)
		if len(node.Parents) == 0 && len(node.CallSites) == 0 {
			stats.OrphanNodes++
		}
	}

	// Calculate average fan-out
	if nodeCount > 0 {
		stats.AvgFanOut = float64(totalFanOut) / float64(nodeCount)
	}

	// Calculate maximum depth
	stats.MaxDepth = g.calculateMaxDepth(ctx, graph)

	graph.Stats = stats
	return nil
}

// calculateMaxDepth calculates the maximum depth of the call graph.
func (g *graphBuilder) calculateMaxDepth(ctx context.Context, graph *TemporalGraph) int {
	maxDepth := 0
	visited := make(map[string]bool)

	// Start from root nodes (nodes with no parents)
	for _, node := range graph.Nodes {
		select {
		case <-ctx.Done():
			return maxDepth
		default:
		}

		if len(node.Parents) == 0 {
			depth := g.calculateNodeDepth(ctx, node, graph, visited, 0)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}

	return maxDepth
}

// calculateNodeDepth calculates the depth of a specific node in the call graph.
func (g *graphBuilder) calculateNodeDepth(ctx context.Context, node *TemporalNode, graph *TemporalGraph, visited map[string]bool, currentDepth int) int {
	select {
	case <-ctx.Done():
		return currentDepth
	default:
	}

	// Prevent infinite recursion
	if visited[node.Name] {
		return currentDepth
	}

	visited[node.Name] = true
	defer func() { visited[node.Name] = false }()

	maxChildDepth := currentDepth

	for _, callSite := range node.CallSites {
		if childNode, exists := graph.Nodes[callSite.TargetName]; exists {
			childDepth := g.calculateNodeDepth(ctx, childNode, graph, visited, currentDepth+1)
			if childDepth > maxChildDepth {
				maxChildDepth = childDepth
			}
		}
	}

	return maxChildDepth
}

// extractDescription extracts documentation from function comments.
func (g *graphBuilder) extractDescription(fn *ast.FuncDecl) string {
	if fn.Doc == nil {
		return ""
	}

	var comments []string
	for _, comment := range fn.Doc.List {
		text := comment.Text
		// Remove comment markers
		if len(text) > 2 && text[:2] == "//" {
			text = text[2:]
		}
		if len(text) > 0 && text[0] == ' ' {
			text = text[1:]
		}
		comments = append(comments, text)
	}

	if len(comments) == 0 {
		return ""
	}

	// Return first non-empty comment line
	for _, comment := range comments {
		if comment != "" {
			return comment
		}
	}

	return ""
}

// extractReturnType extracts the return type from a function declaration.
func (g *graphBuilder) extractReturnType(fn *ast.FuncDecl) string {
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return ""
	}

	// Get the first return type (usually the main return value before error)
	if len(fn.Type.Results.List) > 0 {
		return g.typeToString(fn.Type.Results.List[0].Type)
	}

	return ""
}

// typeToString converts an AST type to a string.
func (g *graphBuilder) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.StarExpr:
		return "*" + g.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + g.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + g.typeToString(t.Key) + "]" + g.typeToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "unknown"
	}
}

// addUniqueParent adds a parent to the list if it's not already present.
func (g *graphBuilder) addUniqueParent(parents []string, parent string) []string {
	for _, p := range parents {
		if p == parent {
			return parents
		}
	}
	return append(parents, parent)
}
