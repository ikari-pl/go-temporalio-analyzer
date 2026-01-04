// Package output provides various output formats for temporal analysis results.
package output

import (
	"context"
	"io"
	"temporal-analyzer/internal/analyzer"
)

// Formatter provides methods for formatting temporal graphs into different output formats.
type Formatter interface {
	// Format formats the given graph and writes it to the writer.
	Format(ctx context.Context, graph *analyzer.TemporalGraph, w io.Writer) error

	// Name returns the name of the formatter.
	Name() string

	// Description returns a description of the output format.
	Description() string
}

// Manager manages multiple output formatters.
type Manager interface {
	// RegisterFormatter registers a new formatter.
	RegisterFormatter(formatter Formatter)

	// GetFormatter returns a formatter by name.
	GetFormatter(name string) (Formatter, error)

	// ListFormatters returns all available formatter names.
	ListFormatters() []string

	// Format formats the graph using the specified formatter.
	Format(ctx context.Context, formatName string, graph *analyzer.TemporalGraph, w io.Writer) error
}
