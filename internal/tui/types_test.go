package tui

import (
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

func TestListItemFilterValue(t *testing.T) {
	tests := []struct {
		name     string
		node     *analyzer.TemporalNode
		expected string
	}{
		{
			name: "basic workflow",
			node: &analyzer.TemporalNode{
				Name:     "MyWorkflow",
				Package:  "main",
				FilePath: "workflow.go",
			},
			expected: "MyWorkflow main workflow.go",
		},
		{
			name: "activity with long path",
			node: &analyzer.TemporalNode{
				Name:     "ProcessActivity",
				Package:  "activities",
				FilePath: "internal/activities/process.go",
			},
			expected: "ProcessActivity activities internal/activities/process.go",
		},
		{
			name: "empty fields",
			node: &analyzer.TemporalNode{
				Name:     "EmptyNode",
				Package:  "",
				FilePath: "",
			},
			expected: "EmptyNode  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := ListItem{Node: tt.node}
			result := item.FilterValue()
			if result != tt.expected {
				t.Errorf("FilterValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestListItemTitle(t *testing.T) {
	tests := []struct {
		name       string
		node       *analyzer.TemporalNode
		contains   []string
		notContain []string
	}{
		{
			name:     "workflow",
			node:     &analyzer.TemporalNode{Name: "TestWorkflow", Type: "workflow"},
			contains: []string{"⚡", "TestWorkflow"},
		},
		{
			name:     "activity",
			node:     &analyzer.TemporalNode{Name: "TestActivity", Type: "activity"},
			contains: []string{"⚙", "TestActivity"},
		},
		{
			name:     "signal",
			node:     &analyzer.TemporalNode{Name: "TestSignal", Type: "signal"},
			contains: []string{"🔔", "TestSignal"},
		},
		{
			name:     "signal_handler",
			node:     &analyzer.TemporalNode{Name: "TestHandler", Type: "signal_handler"},
			contains: []string{"🔔", "TestHandler"},
		},
		{
			name:     "query",
			node:     &analyzer.TemporalNode{Name: "TestQuery", Type: "query"},
			contains: []string{"❓", "TestQuery"},
		},
		{
			name:     "query_handler",
			node:     &analyzer.TemporalNode{Name: "TestQueryHandler", Type: "query_handler"},
			contains: []string{"❓", "TestQueryHandler"},
		},
		{
			name:     "update",
			node:     &analyzer.TemporalNode{Name: "TestUpdate", Type: "update"},
			contains: []string{"🔄", "TestUpdate"},
		},
		{
			name:     "update_handler",
			node:     &analyzer.TemporalNode{Name: "TestUpdateHandler", Type: "update_handler"},
			contains: []string{"🔄", "TestUpdateHandler"},
		},
		{
			name:     "timer",
			node:     &analyzer.TemporalNode{Name: "TestTimer", Type: "timer"},
			contains: []string{"⏱", "TestTimer"},
		},
		{
			name:     "unknown type",
			node:     &analyzer.TemporalNode{Name: "Unknown", Type: "something"},
			contains: []string{"•", "Unknown"},
		},
		{
			name: "truncated long name",
			node: &analyzer.TemporalNode{
				Name: "ThisIsAVeryLongWorkflowNameThatShouldBeTruncatedBecauseItExceedsTheMaxDisplayLength",
				Type: "workflow",
			},
			contains:   []string{"⚡", "..."},
			notContain: []string{"MaxDisplayLength"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := ListItem{Node: tt.node}
			result := item.Title()

			for _, want := range tt.contains {
				if !containsStr(result, want) {
					t.Errorf("Title() = %q, should contain %q", result, want)
				}
			}

			for _, notWant := range tt.notContain {
				if containsStr(result, notWant) {
					t.Errorf("Title() = %q, should not contain %q", result, notWant)
				}
			}
		})
	}
}

func TestListItemDescription(t *testing.T) {
	tests := []struct {
		name     string
		node     *analyzer.TemporalNode
		contains []string
	}{
		{
			name: "basic workflow",
			node: &analyzer.TemporalNode{
				Name:    "Test",
				Type:    "workflow",
				Package: "main",
			},
			contains: []string{"workflow", "main"},
		},
		{
			name: "workflow with connections",
			node: &analyzer.TemporalNode{
				Name:    "Test",
				Type:    "workflow",
				Package: "main",
				CallSites: []analyzer.CallSite{
					{TargetName: "Activity1"},
					{TargetName: "Activity2"},
				},
				Parents: []string{"ParentWorkflow"},
			},
			contains: []string{"workflow", "main", "3 connections"},
		},
		{
			name: "workflow with signals",
			node: &analyzer.TemporalNode{
				Name:    "Test",
				Type:    "workflow",
				Package: "main",
				Signals: []analyzer.SignalDef{
					{Name: "Signal1"},
					{Name: "Signal2"},
				},
			},
			contains: []string{"2 signals"},
		},
		{
			name: "workflow with queries",
			node: &analyzer.TemporalNode{
				Name:    "Test",
				Type:    "workflow",
				Package: "main",
				Queries: []analyzer.QueryDef{
					{Name: "Query1"},
				},
			},
			contains: []string{"1 queries"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := ListItem{Node: tt.node}
			result := item.Description()

			for _, want := range tt.contains {
				if !containsStr(result, want) {
					t.Errorf("Description() = %q, should contain %q", result, want)
				}
			}
		})
	}
}

func TestGetNodeIcon(t *testing.T) {
	tests := []struct {
		nodeType string
		expected string
	}{
		{"workflow", "⚡"},
		{"activity", "⚙"},
		{"signal", "🔔"},
		{"signal_handler", "🔔"},
		{"query", "❓"},
		{"query_handler", "❓"},
		{"update", "🔄"},
		{"update_handler", "🔄"},
		{"timer", "⏱"},
		{"unknown", "•"},
		{"", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			result := getNodeIcon(tt.nodeType)
			if result != tt.expected {
				t.Errorf("getNodeIcon(%q) = %q, want %q", tt.nodeType, result, tt.expected)
			}
		})
	}
}

func TestDefaultKeyBindings(t *testing.T) {
	bindings := DefaultKeyBindings()

	if len(bindings) == 0 {
		t.Error("DefaultKeyBindings() returned empty slice")
	}

	// Verify expected sections exist
	sectionNames := make(map[string]bool)
	for _, section := range bindings {
		sectionNames[section.Title] = true
	}

	expectedSections := []string{"Navigation", "Views", "Filtering", "Tree View", "Details View", "Export"}
	for _, expected := range expectedSections {
		if !sectionNames[expected] {
			t.Errorf("DefaultKeyBindings() missing expected section: %q", expected)
		}
	}

	// Verify each section has bindings
	for _, section := range bindings {
		if len(section.Bindings) == 0 {
			t.Errorf("Section %q has no bindings", section.Title)
		}

		// Verify each binding has required fields
		for _, binding := range section.Bindings {
			if binding.Key == "" {
				t.Errorf("Section %q has binding with empty key", section.Title)
			}
			if binding.Description == "" {
				t.Errorf("Section %q has binding with empty description (key: %q)", section.Title, binding.Key)
			}
		}
	}
}

func TestViewConstants(t *testing.T) {
	// Verify view constants are defined and unique
	views := []string{ViewList, ViewDetails, ViewTree, ViewStats, ViewHelp, ViewGraph}
	seen := make(map[string]bool)

	for _, view := range views {
		if view == "" {
			t.Error("View constant is empty")
		}
		if seen[view] {
			t.Errorf("Duplicate view constant: %q", view)
		}
		seen[view] = true
	}
}

func TestDirectionConstants(t *testing.T) {
	// Verify direction constants are defined
	directions := []string{
		DirectionCalls,
		DirectionCalledBy,
		DirectionTree,
		DirectionStart,
		DirectionSignal,
		DirectionQuery,
		DirectionUpdate,
	}

	for _, dir := range directions {
		if dir == "" {
			t.Error("Direction constant is empty")
		}
	}
}

func TestIconConstants(t *testing.T) {
	// Verify icon constants are defined
	icons := []string{
		IconExpanded,
		IconCollapsed,
		IconLeaf,
		IconWorkflow,
		IconActivity,
		IconSignal,
		IconQuery,
		IconUpdate,
		IconTimer,
	}

	for _, icon := range icons {
		if icon == "" {
			t.Error("Icon constant is empty")
		}
	}
}

func TestDisplayConstants(t *testing.T) {
	// Verify display constants have sensible values
	if MaxDisplayNameLength <= 0 {
		t.Error("MaxDisplayNameLength should be positive")
	}
	if TruncateLength <= 0 {
		t.Error("TruncateLength should be positive")
	}
	if TruncateLength >= MaxDisplayNameLength {
		t.Error("TruncateLength should be less than MaxDisplayNameLength")
	}
	if EllipsisString == "" {
		t.Error("EllipsisString should not be empty")
	}
	if MaxNavPathLength <= 0 {
		t.Error("MaxNavPathLength should be positive")
	}
	if MaxTreeDepth <= 0 {
		t.Error("MaxTreeDepth should be positive")
	}
	if DefaultPageSize <= 0 {
		t.Error("DefaultPageSize should be positive")
	}
}

func TestSortConstants(t *testing.T) {
	// Verify sort constants are defined and unique
	sorts := []string{SortByName, SortByType, SortByPackage, SortByConnections}
	seen := make(map[string]bool)

	for _, s := range sorts {
		if s == "" {
			t.Error("Sort constant is empty")
		}
		if seen[s] {
			t.Errorf("Duplicate sort constant: %q", s)
		}
		seen[s] = true
	}
}

func TestGroupConstants(t *testing.T) {
	// Verify group constants are defined
	groups := []string{GroupByNone, GroupByType, GroupByPackage}

	// GroupByNone can be empty, but others should be unique
	seen := make(map[string]int)
	for _, g := range groups {
		seen[g]++
	}

	if seen[GroupByType] > 1 {
		t.Error("Duplicate group constant")
	}
	if seen[GroupByPackage] > 1 {
		t.Error("Duplicate group constant")
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify status constants are defined and unique
	statuses := []string{StatusInfo, StatusSuccess, StatusWarning, StatusError}
	seen := make(map[string]bool)

	for _, s := range statuses {
		if s == "" {
			t.Error("Status constant is empty")
		}
		if seen[s] {
			t.Errorf("Duplicate status constant: %q", s)
		}
		seen[s] = true
	}
}

// Helper function
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

