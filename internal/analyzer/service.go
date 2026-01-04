package analyzer

import (
	"context"
	"fmt"
	"log/slog"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
)

// service implements the Service interface.
type service struct {
	logger     *slog.Logger
	parser     Parser
	builder    GraphBuilder
	repository Repository
}

// NewService creates a new Service instance.
func NewService(logger *slog.Logger, parser Parser, builder GraphBuilder, repo Repository) Service {
	return &service{
		logger:     logger,
		parser:     parser,
		builder:    builder,
		repository: repo,
	}
}

// AnalyzeWorkflows performs a complete workflow analysis.
func (s *service) AnalyzeWorkflows(ctx context.Context, opts config.AnalysisOptions) (*TemporalGraph, error) {
	s.logger.Info("Starting temporal analysis", "root_dir", opts.RootDir)

	// Parse directory
	nodes, err := s.parser.ParseDirectory(ctx, opts.RootDir, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory: %w", err)
	}

	if len(nodes) == 0 {
		s.logger.Warn("No temporal workflows or activities found", "root_dir", opts.RootDir)
		return &TemporalGraph{
			Nodes: make(map[string]*TemporalNode),
			Stats: GraphStats{},
		}, nil
	}

	// Build graph
	graph, err := s.builder.BuildGraph(ctx, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}

	s.logger.Info("Analysis complete",
		"workflows", graph.Stats.TotalWorkflows,
		"activities", graph.Stats.TotalActivities,
		"total_nodes", len(graph.Nodes))

	return graph, nil
}

// ValidateGraph checks the graph for common issues or anti-patterns.
func (s *service) ValidateGraph(ctx context.Context, graph *TemporalGraph) ([]ValidationIssue, error) {
	var issues []ValidationIssue

	// Check for orphan nodes
	for _, node := range graph.Nodes {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return issues, ctx.Err()
		default:
		}

		if len(node.Parents) == 0 && len(node.CallSites) == 0 {
			issues = append(issues, ValidationIssue{
				Type:       "warning",
				Message:    fmt.Sprintf("Node '%s' has no connections (orphan)", node.Name),
				NodeName:   node.Name,
				Severity:   3,
				Suggestion: "Consider removing unused code or adding connections",
			})
		}
	}

	// Check for circular dependencies
	circularDeps := s.findCircularDependencies(ctx, graph)
	for _, cycle := range circularDeps {
		issues = append(issues, ValidationIssue{
			Type:       "error",
			Message:    fmt.Sprintf("Circular dependency detected: %s", cycle),
			Severity:   8,
			Suggestion: "Refactor to eliminate circular dependencies",
		})
	}

	// Check for deep call chains
	for _, node := range graph.Nodes {
		if len(node.Parents) == 0 { // Root node
			depth := s.calculateChainDepth(ctx, node, graph, make(map[string]bool))
			if depth > 10 {
				issues = append(issues, ValidationIssue{
					Type:       "warning",
					Message:    fmt.Sprintf("Deep call chain starting from '%s' (depth: %d)", node.Name, depth),
					NodeName:   node.Name,
					Severity:   5,
					Suggestion: "Consider breaking down complex workflows",
				})
			}
		}
	}

	// Check for nodes with too many dependencies
	for _, node := range graph.Nodes {
		if len(node.CallSites) > 20 {
			issues = append(issues, ValidationIssue{
				Type:       "warning",
				Message:    fmt.Sprintf("Node '%s' has many dependencies (%d)", node.Name, len(node.CallSites)),
				NodeName:   node.Name,
				Severity:   4,
				Suggestion: "Consider splitting into smaller, more focused workflows",
			})
		}
	}

	s.logger.Info("Graph validation complete", "issues_found", len(issues))
	return issues, nil
}

// findCircularDependencies detects circular dependencies in the graph.
func (s *service) findCircularDependencies(ctx context.Context, graph *TemporalGraph) []string {
	var cycles []string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for _, node := range graph.Nodes {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return cycles
		default:
		}

		if !visited[node.Name] {
			if cycle := s.detectCycle(ctx, node, graph, visited, recStack, []string{}); cycle != "" {
				cycles = append(cycles, cycle)
			}
		}
	}

	return cycles
}

// detectCycle performs DFS to detect cycles in the graph.
func (s *service) detectCycle(ctx context.Context, node *TemporalNode, graph *TemporalGraph, visited, recStack map[string]bool, path []string) string {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ""
	default:
	}

	visited[node.Name] = true
	recStack[node.Name] = true
	path = append(path, node.Name)

	for _, callSite := range node.CallSites {
		if childNode, exists := graph.Nodes[callSite.TargetName]; exists {
			if !visited[childNode.Name] {
				if cycle := s.detectCycle(ctx, childNode, graph, visited, recStack, path); cycle != "" {
					return cycle
				}
			} else if recStack[childNode.Name] {
				// Found a cycle
				cycleStart := -1
				for i, name := range path {
					if name == childNode.Name {
						cycleStart = i
						break
					}
				}
				if cycleStart != -1 {
					cyclePath := append(path[cycleStart:], childNode.Name)
					return fmt.Sprintf("%v", cyclePath)
				}
			}
		}
	}

	recStack[node.Name] = false
	return ""
}

// calculateChainDepth calculates the maximum depth of a call chain starting from a node.
func (s *service) calculateChainDepth(ctx context.Context, node *TemporalNode, graph *TemporalGraph, visited map[string]bool) int {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return 0
	default:
	}

	if visited[node.Name] {
		return 0 // Avoid infinite recursion
	}

	visited[node.Name] = true
	defer func() { visited[node.Name] = false }()

	maxDepth := 0
	for _, callSite := range node.CallSites {
		if childNode, exists := graph.Nodes[callSite.TargetName]; exists {
			depth := 1 + s.calculateChainDepth(ctx, childNode, graph, visited)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}

	return maxDepth
}
