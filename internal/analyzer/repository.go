package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// repository implements the Repository interface.
type repository struct {
	logger *slog.Logger
}

// NewRepository creates a new Repository instance.
func NewRepository(logger *slog.Logger) Repository {
	return &repository{
		logger: logger,
	}
}

// SaveGraph persists a temporal graph to storage.
func (r *repository) SaveGraph(ctx context.Context, graph *TemporalGraph, path string) error {
	if graph == nil {
		return fmt.Errorf("graph cannot be nil")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal graph to JSON
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal graph: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	r.logger.Info("Saved temporal graph", "path", path, "nodes", len(graph.Nodes))
	return nil
}

// LoadGraph loads a temporal graph from storage.
func (r *repository) LoadGraph(ctx context.Context, path string) (*TemporalGraph, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", path)
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Unmarshal JSON
	var graph TemporalGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("failed to unmarshal graph: %w", err)
	}

	r.logger.Info("Loaded temporal graph", "path", path, "nodes", len(graph.Nodes))
	return &graph, nil
}
