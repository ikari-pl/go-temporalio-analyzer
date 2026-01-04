package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

func TestNewExporter(t *testing.T) {
	e := NewExporter()
	if e == nil {
		t.Fatal("NewExporter returned nil")
	}
}

func TestExportJSON(t *testing.T) {
	e := NewExporter()

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
			name: "graph with workflow and activity",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"TestWorkflow": {
						Name:       "TestWorkflow",
						Type:       "workflow",
						Package:    "main",
						FilePath:   "workflow.go",
						LineNumber: 10,
						CallSites: []analyzer.CallSite{
							{
								TargetName: "TestActivity",
								TargetType: "activity",
								CallType:   "activity",
								LineNumber: 15,
							},
						},
					},
					"TestActivity": {
						Name:       "TestActivity",
						Type:       "activity",
						Package:    "main",
						FilePath:   "activity.go",
						LineNumber: 20,
						Parents:    []string{"TestWorkflow"},
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows:  1,
					TotalActivities: 1,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.ExportJSON(tt.graph)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExportJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify it's valid JSON
				var parsed interface{}
				if err := json.Unmarshal(result, &parsed); err != nil {
					t.Errorf("ExportJSON() produced invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestExportDOT(t *testing.T) {
	e := NewExporter()

	tests := []struct {
		name           string
		graph          *analyzer.TemporalGraph
		wantContains   []string
		wantNotContain []string
		wantErr        bool
	}{
		{
			name: "empty graph",
			graph: &analyzer.TemporalGraph{
				Nodes: make(map[string]*analyzer.TemporalNode),
			},
			wantContains: []string{
				"digraph TemporalGraph",
				"graph [rankdir=TB",
				"node [shape=box",
			},
			wantErr: false,
		},
		{
			name: "graph with workflows only",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"WorkflowA": {
						Name:    "WorkflowA",
						Type:    "workflow",
						Package: "main",
					},
					"WorkflowB": {
						Name:    "WorkflowB",
						Type:    "workflow",
						Package: "pkg",
					},
				},
			},
			wantContains: []string{
				"subgraph cluster_workflows",
				"label=\"Workflows\"",
				"WorkflowA",
				"WorkflowB",
				"fillcolor=\"#a371f7\"",
			},
			wantErr: false,
		},
		{
			name: "graph with activities only",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"ActivityA": {
						Name:    "ActivityA",
						Type:    "activity",
						Package: "main",
					},
				},
			},
			wantContains: []string{
				"subgraph cluster_activities",
				"label=\"Activities\"",
				"ActivityA",
				"fillcolor=\"#7ee787\"",
			},
			wantNotContain: []string{
				"subgraph cluster_workflows",
			},
			wantErr: false,
		},
		{
			name: "graph with other node types",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"SignalHandler": {
						Name: "SignalHandler",
						Type: "signal_handler",
					},
				},
			},
			wantContains: []string{
				"SignalHandler",
				"(signal_handler)",
			},
			wantErr: false,
		},
		{
			name: "graph with edges",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Workflow": {
						Name: "Workflow",
						Type: "workflow",
						CallSites: []analyzer.CallSite{
							{TargetName: "Activity", CallType: "activity"},
							{TargetName: "ChildWorkflow", CallType: "child_workflow"},
							{TargetName: "Signal", CallType: "signal"},
							{TargetName: "Query", CallType: "query"},
							{TargetName: "Other", CallType: "other"},
						},
					},
					"Activity":      {Name: "Activity", Type: "activity"},
					"ChildWorkflow": {Name: "ChildWorkflow", Type: "workflow"},
					"Signal":        {Name: "Signal", Type: "signal"},
					"Query":         {Name: "Query", Type: "query"},
					"Other":         {Name: "Other", Type: "other"},
				},
			},
			wantContains: []string{
				"// Edges",
				"->",
				"style=solid, color=\"#7ee787\"",  // activity edge
				"style=bold, color=\"#a371f7\"",   // child_workflow edge
				"style=dashed, color=\"#ffa657\"", // signal edge
				"style=dotted, color=\"#79c0ff\"", // query edge
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.ExportDOT(tt.graph)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExportDOT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("ExportDOT() missing expected content: %q", want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(result, notWant) {
					t.Errorf("ExportDOT() contains unexpected content: %q", notWant)
				}
			}
		})
	}
}

func TestExportMermaid(t *testing.T) {
	e := NewExporter()

	tests := []struct {
		name         string
		graph        *analyzer.TemporalGraph
		wantContains []string
		wantErr      bool
	}{
		{
			name: "empty graph",
			graph: &analyzer.TemporalGraph{
				Nodes: make(map[string]*analyzer.TemporalNode),
			},
			wantContains: []string{
				"```mermaid",
				"flowchart TB",
				"```",
			},
			wantErr: false,
		},
		{
			name: "graph with workflow",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"MyWorkflow": {
						Name: "MyWorkflow",
						Type: "workflow",
					},
				},
			},
			wantContains: []string{
				"⚡ MyWorkflow",
				"class MyWorkflow workflow",
			},
			wantErr: false,
		},
		{
			name: "graph with activity",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"MyActivity": {
						Name: "MyActivity",
						Type: "activity",
					},
				},
			},
			wantContains: []string{
				"⚙ MyActivity",
				"class MyActivity activity",
			},
			wantErr: false,
		},
		{
			name: "graph with signal",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"MySignal": {
						Name: "MySignal",
						Type: "signal",
					},
				},
			},
			wantContains: []string{
				"🔔 MySignal",
				"class MySignal signal",
			},
			wantErr: false,
		},
		{
			name: "graph with signal handler",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"MySignalHandler": {
						Name: "MySignalHandler",
						Type: "signal_handler",
					},
				},
			},
			wantContains: []string{
				"🔔 MySignalHandler",
			},
			wantErr: false,
		},
		{
			name: "graph with query",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"MyQuery": {
						Name: "MyQuery",
						Type: "query",
					},
				},
			},
			wantContains: []string{
				"❓ MyQuery",
				"class MyQuery query",
			},
			wantErr: false,
		},
		{
			name: "graph with query handler",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"MyQueryHandler": {
						Name: "MyQueryHandler",
						Type: "query_handler",
					},
				},
			},
			wantContains: []string{
				"❓ MyQueryHandler",
			},
			wantErr: false,
		},
		{
			name: "graph with other type",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Other": {
						Name: "Other",
						Type: "unknown",
					},
				},
			},
			wantContains: []string{
				"Other",
			},
			wantErr: false,
		},
		{
			name: "graph with edges",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Workflow": {
						Name: "Workflow",
						Type: "workflow",
						CallSites: []analyzer.CallSite{
							{TargetName: "Activity", CallType: "activity"},
							{TargetName: "ChildWorkflow", CallType: "child_workflow"},
							{TargetName: "Signal", CallType: "signal"},
							{TargetName: "Other", CallType: "other"},
						},
					},
					"Activity":      {Name: "Activity", Type: "activity"},
					"ChildWorkflow": {Name: "ChildWorkflow", Type: "workflow"},
					"Signal":        {Name: "Signal", Type: "signal"},
					"Other":         {Name: "Other", Type: "other"},
				},
			},
			wantContains: []string{
				"-->|execute|", // activity
				"==>|child|",   // child_workflow
				"-.->|signal|", // signal
				"-->",          // default
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.ExportMermaid(tt.graph)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExportMermaid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("ExportMermaid() missing expected content: %q", want)
				}
			}
		})
	}
}

func TestExportMarkdown(t *testing.T) {
	e := NewExporter()

	tests := []struct {
		name         string
		graph        *analyzer.TemporalGraph
		wantContains []string
		wantErr      bool
	}{
		{
			name: "empty graph",
			graph: &analyzer.TemporalGraph{
				Nodes: make(map[string]*analyzer.TemporalNode),
				Stats: analyzer.GraphStats{},
			},
			wantContains: []string{
				"# Temporal Workflow Analysis",
				"## 📊 Statistics",
				"| Metric | Count |",
				"| Workflows | 0 |",
				"| Activities | 0 |",
			},
			wantErr: false,
		},
		{
			name: "graph with workflow",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"TestWorkflow": {
						Name:        "TestWorkflow",
						Type:        "workflow",
						Package:     "main",
						FilePath:    "workflow.go",
						LineNumber:  10,
						Description: "This is a test workflow",
						CallSites: []analyzer.CallSite{
							{TargetName: "Activity", TargetType: "activity"},
						},
					Signals: []analyzer.SignalDef{{Name: "MySignal"}},
					Queries: []analyzer.QueryDef{{Name: "MyQuery"}},
					},
				},
				Stats: analyzer.GraphStats{
					TotalWorkflows: 1,
				},
			},
			wantContains: []string{
				"## ⚡ Workflows",
				"### TestWorkflow",
				"**Package:** `main`",
				"**File:** `workflow.go:10`",
				"**Description:** This is a test workflow",
				"**Calls:**",
				"`Activity` (activity)",
				"**Signals:**",
				"🔔 `MySignal`",
				"**Queries:**",
				"❓ `MyQuery`",
			},
			wantErr: false,
		},
		{
			name: "graph with activity",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"TestActivity": {
						Name:        "TestActivity",
						Type:        "activity",
						Package:     "main",
						FilePath:    "activity.go",
						LineNumber:  20,
						Description: "This is a test activity",
						Parents:     []string{"Workflow1", "Workflow2"},
					},
				},
				Stats: analyzer.GraphStats{
					TotalActivities: 1,
				},
			},
			wantContains: []string{
				"## ⚙️ Activities",
				"### TestActivity",
				"**Package:** `main`",
				"**File:** `activity.go:20`",
				"**Description:** This is a test activity",
				"**Called by:**",
				"`Workflow1`",
				"`Workflow2`",
			},
			wantErr: false,
		},
		{
			name: "graph with stats",
			graph: &analyzer.TemporalGraph{
				Nodes: make(map[string]*analyzer.TemporalNode),
				Stats: analyzer.GraphStats{
					TotalWorkflows:  5,
					TotalActivities: 10,
					TotalSignals:    3,
					TotalQueries:    2,
					TotalUpdates:    1,
					MaxDepth:        4,
					OrphanNodes:     2,
				},
			},
			wantContains: []string{
				"| Workflows | 5 |",
				"| Activities | 10 |",
				"| Signals | 3 |",
				"| Queries | 2 |",
				"| Updates | 1 |",
				"| Max Depth | 4 |",
				"| Orphan Nodes | 2 |",
			},
			wantErr: false,
		},
		{
			name: "graph with embedded mermaid diagram",
			graph: &analyzer.TemporalGraph{
				Nodes: map[string]*analyzer.TemporalNode{
					"Workflow": {
						Name: "Workflow",
						Type: "workflow",
					},
				},
				Stats: analyzer.GraphStats{TotalWorkflows: 1},
			},
			wantContains: []string{
				"## 📈 Dependency Graph",
				"```mermaid",
				"flowchart TB",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.ExportMarkdown(tt.graph)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExportMarkdown() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("ExportMarkdown() missing expected content: %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestEscapeString(t *testing.T) {
	e := NewExporter()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "simple",
			expected: "simple",
		},
		{
			input:    `with"quotes`,
			expected: `with\"quotes`,
		},
		{
			input:    "with\nnewline",
			expected: "with\\nnewline",
		},
		{
			input:    "with\"quotes\nand\nnewlines",
			expected: "with\\\"quotes\\nand\\nnewlines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := e.escapeString(tt.input)
			if result != tt.expected {
				t.Errorf("escapeString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToMermaidID(t *testing.T) {
	e := NewExporter()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "SimpleName",
			expected: "SimpleName",
		},
		{
			input:    "Name_With_Underscore",
			expected: "Name_With_Underscore",
		},
		{
			input:    "Name123",
			expected: "Name123",
		},
		{
			input:    "Name-With-Dashes",
			expected: "NameWithDashes",
		},
		{
			input:    "Name.With.Dots",
			expected: "NameWithDots",
		},
		{
			input:    "Name With Spaces",
			expected: "NameWithSpaces",
		},
		{
			input:    "pkg/path/Function",
			expected: "pkgpathFunction",
		},
		{
			input:    "Special!@#$%^&*()Chars",
			expected: "SpecialChars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := e.toMermaidID(tt.input)
			if result != tt.expected {
				t.Errorf("toMermaidID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetNodeColor(t *testing.T) {
	e := NewExporter()

	tests := []struct {
		nodeType string
		expected string
	}{
		{"workflow", "#a371f7"},
		{"activity", "#7ee787"},
		{"signal", "#ffa657"},
		{"signal_handler", "#ffa657"},
		{"query", "#79c0ff"},
		{"query_handler", "#79c0ff"},
		{"update", "#ff7b72"},
		{"update_handler", "#ff7b72"},
		{"unknown", "#58a6ff"},
		{"", "#58a6ff"},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			result := e.getNodeColor(tt.nodeType)
			if result != tt.expected {
				t.Errorf("getNodeColor(%q) = %q, want %q", tt.nodeType, result, tt.expected)
			}
		})
	}
}

func TestGetEdgeStyle(t *testing.T) {
	e := NewExporter()

	tests := []struct {
		callType string
		expected string
	}{
		{"activity", "style=solid, color=\"#7ee787\""},
		{"child_workflow", "style=bold, color=\"#a371f7\""},
		{"signal", "style=dashed, color=\"#ffa657\""},
		{"query", "style=dotted, color=\"#79c0ff\""},
		{"unknown", "style=solid"},
		{"", "style=solid"},
	}

	for _, tt := range tests {
		t.Run(tt.callType, func(t *testing.T) {
			result := e.getEdgeStyle(tt.callType)
			if result != tt.expected {
				t.Errorf("getEdgeStyle(%q) = %q, want %q", tt.callType, result, tt.expected)
			}
		})
	}
}

// Test for consistent node ordering in exports
func TestExportConsistentOrdering(t *testing.T) {
	e := NewExporter()

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"ZWorkflow": {Name: "ZWorkflow", Type: "workflow"},
			"AWorkflow": {Name: "AWorkflow", Type: "workflow"},
			"MWorkflow": {Name: "MWorkflow", Type: "workflow"},
		},
	}

	// Run export multiple times to verify consistent ordering
	var firstDOT, firstMermaid, firstMarkdown string
	for i := 0; i < 5; i++ {
		dot, _ := e.ExportDOT(graph)
		mermaid, _ := e.ExportMermaid(graph)
		markdown, _ := e.ExportMarkdown(graph)

		if i == 0 {
			firstDOT = dot
			firstMermaid = mermaid
			firstMarkdown = markdown
		} else {
			if dot != firstDOT {
				t.Error("ExportDOT produces inconsistent output across calls")
			}
			if mermaid != firstMermaid {
				t.Error("ExportMermaid produces inconsistent output across calls")
			}
			if markdown != firstMarkdown {
				t.Error("ExportMarkdown produces inconsistent output across calls")
			}
		}
	}

	// Verify alphabetical ordering in DOT output
	dotOutput, _ := e.ExportDOT(graph)
	aIndex := strings.Index(dotOutput, "AWorkflow")
	mIndex := strings.Index(dotOutput, "MWorkflow")
	zIndex := strings.Index(dotOutput, "ZWorkflow")

	// Ensure all nodes are present in output
	if aIndex == -1 || mIndex == -1 || zIndex == -1 {
		t.Errorf("DOT output missing nodes: aIndex=%d, mIndex=%d, zIndex=%d", aIndex, mIndex, zIndex)
	}

	// Verify strict ascending order: A < M < Z (explicit check for all pairs)
	if aIndex >= mIndex || mIndex >= zIndex || aIndex >= zIndex {
		t.Errorf("ExportDOT does not maintain alphabetical ordering: A@%d, M@%d, Z@%d", aIndex, mIndex, zIndex)
	}
}

// Test complex graph structure
func TestExportComplexGraph(t *testing.T) {
	e := NewExporter()

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"MainWorkflow": {
				Name:    "MainWorkflow",
				Type:    "workflow",
				Package: "main",
				CallSites: []analyzer.CallSite{
					{TargetName: "Activity1", TargetType: "activity", CallType: "activity"},
					{TargetName: "Activity2", TargetType: "activity", CallType: "activity"},
					{TargetName: "ChildWorkflow", TargetType: "workflow", CallType: "child_workflow"},
				},
				Signals: []analyzer.SignalDef{
					{Name: "StartSignal"},
					{Name: "StopSignal"},
				},
				Queries: []analyzer.QueryDef{
					{Name: "StatusQuery"},
				},
			},
			"ChildWorkflow": {
				Name:    "ChildWorkflow",
				Type:    "workflow",
				Package: "child",
				Parents: []string{"MainWorkflow"},
				CallSites: []analyzer.CallSite{
					{TargetName: "Activity3", TargetType: "activity", CallType: "activity"},
				},
			},
			"Activity1": {
				Name:    "Activity1",
				Type:    "activity",
				Package: "activities",
				Parents: []string{"MainWorkflow"},
			},
			"Activity2": {
				Name:    "Activity2",
				Type:    "activity",
				Package: "activities",
				Parents: []string{"MainWorkflow"},
			},
			"Activity3": {
				Name:    "Activity3",
				Type:    "activity",
				Package: "activities",
				Parents: []string{"ChildWorkflow"},
			},
		},
		Stats: analyzer.GraphStats{
			TotalWorkflows:  2,
			TotalActivities: 3,
			TotalSignals:    2,
			TotalQueries:    1,
			MaxDepth:        3,
		},
	}

	// Test DOT export
	dotOutput, err := e.ExportDOT(graph)
	if err != nil {
		t.Errorf("ExportDOT failed: %v", err)
	}
	if !strings.Contains(dotOutput, "MainWorkflow") {
		t.Error("DOT output missing MainWorkflow")
	}
	if !strings.Contains(dotOutput, "ChildWorkflow") {
		t.Error("DOT output missing ChildWorkflow")
	}

	// Test Mermaid export
	mermaidOutput, err := e.ExportMermaid(graph)
	if err != nil {
		t.Errorf("ExportMermaid failed: %v", err)
	}
	if !strings.Contains(mermaidOutput, "MainWorkflow") {
		t.Error("Mermaid output missing MainWorkflow")
	}

	// Test Markdown export
	markdownOutput, err := e.ExportMarkdown(graph)
	if err != nil {
		t.Errorf("ExportMarkdown failed: %v", err)
	}
	if !strings.Contains(markdownOutput, "MainWorkflow") {
		t.Error("Markdown output missing MainWorkflow")
	}
	if !strings.Contains(markdownOutput, "| Workflows | 2 |") {
		t.Error("Markdown output has incorrect workflow count")
	}

	// Test JSON export
	jsonOutput, err := e.ExportJSON(graph)
	if err != nil {
		t.Errorf("ExportJSON failed: %v", err)
	}
	var parsed analyzer.TemporalGraph
	if err := json.Unmarshal(jsonOutput, &parsed); err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}
	if len(parsed.Nodes) != 5 {
		t.Errorf("JSON output has %d nodes, expected 5", len(parsed.Nodes))
	}
}

// Test workflow without description
func TestExportWorkflowWithoutDescription(t *testing.T) {
	e := NewExporter()

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"NoDescWorkflow": {
				Name:        "NoDescWorkflow",
				Type:        "workflow",
				Package:     "main",
				FilePath:    "workflow.go",
				LineNumber:  10,
				Description: "", // Empty description
			},
		},
		Stats: analyzer.GraphStats{TotalWorkflows: 1},
	}

	result, err := e.ExportMarkdown(graph)
	if err != nil {
		t.Errorf("ExportMarkdown failed: %v", err)
	}

	// Should not contain "Description:" when description is empty
	if strings.Contains(result, "**Description:**") {
		t.Error("Markdown output should not contain Description field when description is empty")
	}
}

// Test activity without description
func TestExportActivityWithoutDescription(t *testing.T) {
	e := NewExporter()

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"NoDescActivity": {
				Name:        "NoDescActivity",
				Type:        "activity",
				Package:     "main",
				FilePath:    "activity.go",
				LineNumber:  10,
				Description: "", // Empty description
			},
		},
		Stats: analyzer.GraphStats{TotalActivities: 1},
	}

	result, err := e.ExportMarkdown(graph)
	if err != nil {
		t.Errorf("ExportMarkdown failed: %v", err)
	}

	// Should not contain "Description:" when description is empty
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(line, "NoDescActivity") && strings.Contains(line, "**Description:**") {
			t.Error("Markdown output should not contain Description field for activity when description is empty")
		}
	}
}

// Test workflow without calls
func TestExportWorkflowWithoutCalls(t *testing.T) {
	e := NewExporter()

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"NoCallsWorkflow": {
				Name:       "NoCallsWorkflow",
				Type:       "workflow",
				Package:    "main",
				FilePath:   "workflow.go",
				LineNumber: 10,
				CallSites:  nil, // No calls
			},
		},
		Stats: analyzer.GraphStats{TotalWorkflows: 1},
	}

	result, err := e.ExportMarkdown(graph)
	if err != nil {
		t.Errorf("ExportMarkdown failed: %v", err)
	}

	// Should not contain "Calls:" when there are no calls
	if strings.Contains(result, "**Calls:**") {
		t.Error("Markdown output should not contain Calls section when there are no calls")
	}
}

// Test activity without parents
func TestExportActivityWithoutParents(t *testing.T) {
	e := NewExporter()

	graph := &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"OrphanActivity": {
				Name:       "OrphanActivity",
				Type:       "activity",
				Package:    "main",
				FilePath:   "activity.go",
				LineNumber: 10,
				Parents:    nil, // No parents (orphan)
			},
		},
		Stats: analyzer.GraphStats{TotalActivities: 1, OrphanNodes: 1},
	}

	result, err := e.ExportMarkdown(graph)
	if err != nil {
		t.Errorf("ExportMarkdown failed: %v", err)
	}

	// Should not contain "Called by:" when there are no parents
	if strings.Contains(result, "**Called by:**") {
		t.Error("Markdown output should not contain Called by section when there are no parents")
	}
}

