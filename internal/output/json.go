package output

import (
	"context"
	"encoding/json"
	"io"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

// jsonFormatter implements the Formatter interface for JSON output.
type jsonFormatter struct{}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter() Formatter {
	return &jsonFormatter{}
}

// Format formats the given graph and writes it to the writer as JSON.
func (f *jsonFormatter) Format(ctx context.Context, graph *analyzer.TemporalGraph, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(graph)
}

// Name returns the name of the formatter.
func (f *jsonFormatter) Name() string {
	return "json"
}

// Description returns a description of the output format.
func (f *jsonFormatter) Description() string {
	return "JSON format for programmatic consumption"
}
