package output

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

func TestNewJSONFormatter(t *testing.T) {
	f := NewJSONFormatter()
	if f == nil {
		t.Fatal("NewJSONFormatter returned nil")
	}
}

func TestJSONFormatterName(t *testing.T) {
	f := NewJSONFormatter()
	if f.Name() != "json" {
		t.Errorf("Name() = %q, want %q", f.Name(), "json")
	}
}

func TestJSONFormatterDescription(t *testing.T) {
	f := NewJSONFormatter()
	desc := f.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if desc != "JSON format for programmatic consumption" {
		t.Errorf("Description() = %q, want %q", desc, "JSON format for programmatic consumption")
	}
}

func TestJSONFormatterFormat(t *testing.T) {
	tests := []struct {
		name    string
		graph   *analyzer.TemporalGraph
		wantErr bool
	}{
		{
			name: "empty graph",
			graph: &analyzer.TemporalGraph{
				Nodes: make(map[string]*analyzer.TemporalNode),
				Stats: analyzer.GraphStats{},
			},
			wantErr: false,
		},
		{
			name: "graph with workflow",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"TestWorkflow": {
						Name:       "TestWorkflow",
						Type:       "workflow",
						Package:    "main",
						FilePath:   "workflow.go",
						LineNumber: 10,
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "graph with activity",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"TestActivity": {
						Name:       "TestActivity",
						Type:       "activity",
						Package:    "main",
						FilePath:   "activity.go",
						LineNumber: 20,
						Parents:    []string{"Workflow1"},
					},
				},
				Stats: analyzer.GraphStats{
					TotalActivities: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "graph with call sites",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Workflow": {
						Name: "Workflow",
						Type: "workflow",
						CallSites: []analyzer.CallSite{
							{
								TargetName: "Activity",
								TargetType: "activity",
								CallType:   "activity",
								LineNumber: 15,
								FilePath:   "workflow.go",
							},
						},
					},
					"Activity": {
						Name:    "Activity",
						Type:    "activity",
						Parents: []string{"Workflow"},
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows:  1,
					TotalActivities: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "graph with signals",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Workflow": {
						Name: "Workflow",
						Type: "workflow",
						Signals: []analyzer.SignalDef{
							{Name: "Signal1", LineNumber: 20},
							{Name: "Signal2", LineNumber: 25},
						},
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows: 1,
					TotalSignals:   2,
				},
			},
			wantErr: false,
		},
		{
			name: "graph with queries",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Workflow": {
						Name: "Workflow",
						Type: "workflow",
						Queries: []analyzer.QueryDef{
							{Name: "Query1", LineNumber: 30},
						},
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows: 1,
					TotalQueries:   1,
				},
			},
			wantErr: false,
		},
		{
			name: "graph with updates",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Workflow": {
						Name: "Workflow",
						Type: "workflow",
						Updates: []analyzer.UpdateDef{
							{Name: "Update1", LineNumber: 35},
						},
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows: 1,
					TotalUpdates:   1,
				},
			},
			wantErr: false,
		},
		{
			name: "graph with internal calls",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Workflow": {
						Name: "Workflow",
						Type: "workflow",
						InternalCalls: []analyzer.InternalCall{
							{
								TargetName: "helperFunc",
								CallType:   "function",
								FilePath:   "helpers.go",
								LineNumber: 40,
							},
						},
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows: 1,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewJSONFormatter()
			var buf bytes.Buffer

			err := f.Format(context.Background(), tt.graph, &buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the output is valid JSON
				var parsed interface{}
				if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
					t.Errorf("Format() produced invalid JSON: %v", err)
				}

				// Verify it can be unmarshaled back to TemporalGraph
				var graph analyzer.TemporalGraph
				if err := json.Unmarshal(buf.Bytes(), &graph); err != nil {
					t.Errorf("Format() JSON cannot be unmarshaled to TemporalGraph: %v", err)
				}
			}
		})
	}
}

func TestJSONFormatterFormatIndentation(t *testing.T) {
	f := NewJSONFormatter()
	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"Test": {Name: "Test", Type: "workflow"},
		},
		Stats: analyzer.GraphStats{TotalWorkflows: 1},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), graph, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Verify indentation (2 spaces)
	if !bytes.Contains(buf.Bytes(), []byte("  ")) {
		t.Error("Format() should produce indented JSON")
	}

	// Verify output is not minified (should have newlines)
	if !bytes.Contains(buf.Bytes(), []byte("\n")) {
		t.Error("Format() should produce pretty-printed JSON with newlines")
	}

	// Verify it's valid JSON
	var parsed interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("Format() produced invalid JSON: %v", err)
	}
}

func TestJSONFormatterContextCancellation(t *testing.T) {
	f := NewJSONFormatter()
	graph := &analyzer.TemporalGraph{
		Nodes: make(map[string]*analyzer.TemporalNode),
		Stats: analyzer.GraphStats{},
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var buf bytes.Buffer
	// Note: The current implementation doesn't actually check context,
	// so this should still succeed
	err := f.Format(ctx, graph, &buf)
	// This will likely succeed since the implementation doesn't check context
	// But we're verifying the interface allows for context passing
	_ = err
}

func TestJSONFormatterRoundTrip(t *testing.T) {
	f := NewJSONFormatter()

	original := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"MainWorkflow": {
				Name:        "MainWorkflow",
				Type:        "workflow",
				Package:     "main",
				FilePath:    "workflow.go",
				LineNumber:  10,
				Description: "Main workflow description",
				CallSites: []analyzer.CallSite{
					{
						TargetName: "Activity1",
						TargetType: "activity",
						CallType:   "activity",
						LineNumber: 15,
					},
				},
				Signals: []analyzer.SignalDef{
					{Name: "MySignal", LineNumber: 20},
				},
				Queries: []analyzer.QueryDef{
					{Name: "MyQuery", LineNumber: 25},
				},
				Updates: []analyzer.UpdateDef{
					{Name: "MyUpdate", LineNumber: 30},
				},
				Parents: []string{},
			},
			"Activity1": {
				Name:       "Activity1",
				Type:       "activity",
				Package:    "activities",
				FilePath:   "activity.go",
				LineNumber: 50,
				Parents:    []string{"MainWorkflow"},
			},
		},
		Stats: analyzer.GraphStats{
			TotalWorkflows:  1,
			TotalActivities: 1,
			TotalSignals:    1,
			TotalQueries:    1,
			TotalUpdates:    1,
			MaxDepth:        2,
			OrphanNodes:     0,
		},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), original, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Unmarshal back
	var roundTripped analyzer.TemporalGraph
	if err := json.Unmarshal(buf.Bytes(), &roundTripped); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify key fields
	if len(roundTripped.Nodes) != len(original.Nodes) {
		t.Errorf("Node count mismatch: got %d, want %d", len(roundTripped.Nodes), len(original.Nodes))
	}

	if roundTripped.Stats.TotalWorkflows != original.Stats.TotalWorkflows {
		t.Errorf("TotalWorkflows mismatch: got %d, want %d", roundTripped.Stats.TotalWorkflows, original.Stats.TotalWorkflows)
	}

	if roundTripped.Stats.TotalActivities != original.Stats.TotalActivities {
		t.Errorf("TotalActivities mismatch: got %d, want %d", roundTripped.Stats.TotalActivities, original.Stats.TotalActivities)
	}

	// Verify node data
	mainWorkflow, ok := roundTripped.Nodes["MainWorkflow"]
	if !ok {
		t.Fatal("MainWorkflow not found in round-tripped graph")
	}
	if mainWorkflow.Name != "MainWorkflow" {
		t.Errorf("MainWorkflow.Name = %q, want %q", mainWorkflow.Name, "MainWorkflow")
	}
	if mainWorkflow.Type != "workflow" {
		t.Errorf("MainWorkflow.Type = %q, want %q", mainWorkflow.Type, "workflow")
	}
	if mainWorkflow.Description != "Main workflow description" {
		t.Errorf("MainWorkflow.Description = %q, want %q", mainWorkflow.Description, "Main workflow description")
	}
	if len(mainWorkflow.CallSites) != 1 {
		t.Errorf("MainWorkflow.CallSites length = %d, want 1", len(mainWorkflow.CallSites))
	}
}

func TestJSONFormatterInterface(t *testing.T) {
	// Verify that jsonFormatter implements Formatter interface
	var _ Formatter = NewJSONFormatter()
}

func TestJSONFormatterEmptyWriter(t *testing.T) {
	f := NewJSONFormatter()
	graph := &analyzer.TemporalGraph{
		Nodes: make(map[string]*analyzer.TemporalNode),
		Stats: analyzer.GraphStats{},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), graph, &buf)
	if err != nil {
		t.Errorf("Format() error = %v", err)
	}

	// Verify output is not empty
	if buf.Len() == 0 {
		t.Error("Format() produced empty output")
	}
}

// Test with complex nested structures
func TestJSONFormatterComplexGraph(t *testing.T) {
	f := NewJSONFormatter()

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"RootWorkflow": {
				Name:    "RootWorkflow",
				Type:    "workflow",
				Package: "root",
				CallSites: []analyzer.CallSite{
					{TargetName: "ChildWorkflow1", TargetType: "workflow", CallType: "child_workflow"},
					{TargetName: "ChildWorkflow2", TargetType: "workflow", CallType: "child_workflow"},
					{TargetName: "SharedActivity", TargetType: "activity", CallType: "activity"},
				},
				Signals: []analyzer.SignalDef{{Name: "Signal1"}, {Name: "Signal2"}},
				Queries: []analyzer.QueryDef{{Name: "Query1"}},
			},
			"ChildWorkflow1": {
				Name:    "ChildWorkflow1",
				Type:    "workflow",
				Package: "child",
				Parents: []string{"RootWorkflow"},
				CallSites: []analyzer.CallSite{
					{TargetName: "SharedActivity", TargetType: "activity", CallType: "activity"},
					{TargetName: "Activity1", TargetType: "activity", CallType: "activity"},
				},
			},
			"ChildWorkflow2": {
				Name:    "ChildWorkflow2",
				Type:    "workflow",
				Package: "child",
				Parents: []string{"RootWorkflow"},
				CallSites: []analyzer.CallSite{
					{TargetName: "SharedActivity", TargetType: "activity", CallType: "activity"},
					{TargetName: "Activity2", TargetType: "activity", CallType: "activity"},
				},
			},
			"SharedActivity": {
				Name:    "SharedActivity",
				Type:    "activity",
				Package: "activities",
				Parents: []string{"RootWorkflow", "ChildWorkflow1", "ChildWorkflow2"},
			},
			"Activity1": {
				Name:    "Activity1",
				Type:    "activity",
				Package: "activities",
				Parents: []string{"ChildWorkflow1"},
			},
			"Activity2": {
				Name:    "Activity2",
				Type:    "activity",
				Package: "activities",
				Parents: []string{"ChildWorkflow2"},
			},
		},
		Stats: analyzer.GraphStats{
			TotalWorkflows:  3,
			TotalActivities: 3,
			TotalSignals:    2,
			TotalQueries:    1,
			MaxDepth:        3,
		},
	}

	var buf bytes.Buffer
	err := f.Format(context.Background(), graph, &buf)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Verify valid JSON
	var parsed analyzer.TemporalGraph
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify structure
	if len(parsed.Nodes) != 6 {
		t.Errorf("Expected 6 nodes, got %d", len(parsed.Nodes))
	}

	// Verify shared activity has correct parents
	sharedActivity, ok := parsed.Nodes["SharedActivity"]
	if !ok {
		t.Fatal("SharedActivity not found")
	}
	if len(sharedActivity.Parents) != 3 {
		t.Errorf("SharedActivity should have 3 parents, got %d", len(sharedActivity.Parents))
	}
}

