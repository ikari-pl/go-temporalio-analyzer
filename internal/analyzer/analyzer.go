package analyzer

import (
	"context"
	"log/slog"
	"temporal-analyzer/internal/config"
)

// analyzer implements the Analyzer interface and provides the main entry point.
type analyzer struct {
	service Service
	logger  *slog.Logger
}

// NewAnalyzer creates a new Analyzer instance with all dependencies.
func NewAnalyzer(logger *slog.Logger) Analyzer {
	// Create dependencies
	parser := NewParser(logger)
	extractor := NewCallExtractor(logger)
	builder := NewGraphBuilder(logger, extractor)
	repo := NewRepository(logger)
	service := NewService(logger, parser, builder, repo)

	return &analyzer{
		service: service,
		logger:  logger,
	}
}

// Analyze performs a complete analysis of the given directory and returns a temporal graph.
func (a *analyzer) Analyze(ctx context.Context, opts config.AnalysisOptions) (*TemporalGraph, error) {
	return a.service.AnalyzeWorkflows(ctx, opts)
}

// ValidateGraph is a convenience method to access validation through the analyzer.
func (a *analyzer) ValidateGraph(ctx context.Context, graph *TemporalGraph) ([]ValidationIssue, error) {
	return a.service.ValidateGraph(ctx, graph)
}
