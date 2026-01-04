package tui

import (
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestGraph creates a test graph for unit tests.
func createTestGraph() *analyzer.TemporalGraph {
	return &analyzer.TemporalGraph{
		Nodes: map[string]*analyzer.TemporalNode{
			"MainWorkflow": {
				Name:       "MainWorkflow",
				Type:       "workflow",
				Package:    "workflows",
				FilePath:   "/app/pkg/workflows/main.go",
				LineNumber: 10,
				CallSites: []analyzer.CallSite{
					{TargetName: "ProcessActivity", TargetType: "activity", CallType: "activity", LineNumber: 15},
					{TargetName: "ChildWorkflow", TargetType: "workflow", CallType: "child_workflow", LineNumber: 20},
				},
				Signals: []analyzer.SignalDef{{Name: "UpdateSignal", LineNumber: 25}},
				Queries: []analyzer.QueryDef{{Name: "StatusQuery", LineNumber: 30}},
			},
			"ChildWorkflow": {
				Name:       "ChildWorkflow",
				Type:       "workflow",
				Package:    "workflows",
				FilePath:   "/app/pkg/workflows/child.go",
				LineNumber: 50,
				Parents:    []string{"MainWorkflow"},
				CallSites: []analyzer.CallSite{
					{TargetName: "ProcessActivity", TargetType: "activity", CallType: "activity", LineNumber: 55},
				},
			},
			"ProcessActivity": {
				Name:       "ProcessActivity",
				Type:       "activity",
				Package:    "activities",
				FilePath:   "/app/pkg/activities/process.go",
				LineNumber: 100,
				Parents:    []string{"MainWorkflow", "ChildWorkflow"},
				InternalCalls: []analyzer.InternalCall{
					{TargetName: "helper", CallType: "function", LineNumber: 105},
				},
			},
			"OrphanWorkflow": {
				Name:       "OrphanWorkflow",
				Type:       "workflow",
				Package:    "workflows",
				FilePath:   "/app/pkg/workflows/orphan.go",
				LineNumber: 200,
				// No parents, no call sites - an orphan
			},
		},
		Stats: analyzer.GraphStats{
			TotalWorkflows:  3,
			TotalActivities: 1,
			TotalSignals:    1,
			TotalQueries:    1,
			MaxDepth:        2,
		},
	}
}

// createTestState creates a test state for unit tests.
func createTestState() *State {
	graph := createTestGraph()
	return &State{
		Graph:          graph,
		CurrentView:    ViewList,
		WindowWidth:    80,
		WindowHeight:   24,
		ShowWorkflows:  true,
		ShowActivities: true,
		ShowSignals:    true,
		ShowQueries:    true,
		Navigator:      NewNavigator(),
	}
}

// =============================================================================
// Tree Building Tests
// =============================================================================

func TestFindCommonPrefix(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected string
	}{
		{
			name:     "empty paths",
			paths:    []string{},
			expected: "",
		},
		{
			name:     "single path",
			paths:    []string{"/app/pkg/workflows"},
			expected: "/app/pkg/workflows",
		},
		{
			name:     "common prefix",
			paths:    []string{"/app/pkg/workflows", "/app/pkg/activities", "/app/pkg/handlers"},
			expected: "/app/pkg",
		},
		{
			name:     "no common prefix",
			paths:    []string{"/app/pkg", "/other/path"},
			expected: "",
		},
		{
			name:     "nested paths",
			paths:    []string{"/app/pkg/workflows/main", "/app/pkg/workflows/child"},
			expected: "/app/pkg/workflows",
		},
		{
			name:     "identical paths",
			paths:    []string{"/app/pkg", "/app/pkg", "/app/pkg"},
			expected: "/app/pkg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findCommonPrefix(tt.paths)
			if result != tt.expected {
				t.Errorf("findCommonPrefix(%v) = %q, want %q", tt.paths, result, tt.expected)
			}
		})
	}
}

func TestCollapseSingleChildChains(t *testing.T) {
	tests := []struct {
		name     string
		input    *packageTreeNode
		expected string // Expected name after collapse
	}{
		{
			name: "no collapse needed",
			input: &packageTreeNode{
				name:     "root",
				fullPath: "root",
				children: map[string]*packageTreeNode{
					"a": {name: "a", fullPath: "root/a", children: make(map[string]*packageTreeNode)},
					"b": {name: "b", fullPath: "root/b", children: make(map[string]*packageTreeNode)},
				},
			},
			expected: "root",
		},
		{
			name: "collapse single child",
			input: &packageTreeNode{
				name:     "root",
				fullPath: "root",
				children: map[string]*packageTreeNode{
					"only": {
						name:     "only",
						fullPath: "root/only",
						children: make(map[string]*packageTreeNode),
						nodes:    []*analyzer.TemporalNode{{Name: "Test"}},
					},
				},
			},
			expected: "root/only",
		},
		{
			name: "two level collapse",
			input: &packageTreeNode{
				name:     "",
				fullPath: "",
				children: map[string]*packageTreeNode{
					"a": {
						name:     "a",
						fullPath: "a",
						children: map[string]*packageTreeNode{
							"b": {
								name:     "b",
								fullPath: "a/b",
								children: make(map[string]*packageTreeNode),
								nodes:    []*analyzer.TemporalNode{{Name: "Test"}},
							},
						},
					},
				},
			},
			// When root name is empty, returns the collapsed child directly
			expected: "a/b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collapseSingleChildChains(tt.input)
			if result.name != tt.expected {
				t.Errorf("collapseSingleChildChains() name = %q, want %q", result.name, tt.expected)
			}
		})
	}
}

func TestBuildTreeByHierarchy(t *testing.T) {
	styles := NewStyleManager()
	tv := NewTreeView(styles).(*treeView)
	state := createTestState()

	// Initialize tree state
	state.TreeState = &TreeViewState{
		ExpansionStates: make(map[string]bool),
		GroupBy:         "hierarchy",
	}

	tv.buildTreeByHierarchy(state)

	// Should have root nodes (nodes with no parents)
	if len(state.TreeState.Items) == 0 {
		t.Error("buildTreeByHierarchy should create tree items")
	}

	// Find root nodes - MainWorkflow and OrphanWorkflow have no parents
	rootCount := 0
	for _, item := range state.TreeState.Items {
		if item.Depth == 0 {
			rootCount++
		}
	}

	if rootCount < 2 {
		t.Errorf("Expected at least 2 root nodes (MainWorkflow, OrphanWorkflow), got %d", rootCount)
	}

	// Verify MainWorkflow has children
	for _, item := range state.TreeState.Items {
		if item.Node != nil && item.Node.Name == "MainWorkflow" {
			if !item.HasChildren {
				t.Error("MainWorkflow should have children")
			}
			if item.ChildCount != 2 {
				t.Errorf("MainWorkflow should have 2 children (callsites), got %d", item.ChildCount)
			}
			break
		}
	}
}

func TestBuildTreeByHierarchyExpanded(t *testing.T) {
	styles := NewStyleManager()
	tv := NewTreeView(styles).(*treeView)
	state := createTestState()

	// Initialize tree state with MainWorkflow expanded
	state.TreeState = &TreeViewState{
		ExpansionStates: map[string]bool{"MainWorkflow": true},
		GroupBy:         "hierarchy",
	}

	tv.buildTreeByHierarchy(state)

	// With MainWorkflow expanded, we should see its children
	foundChild := false
	for _, item := range state.TreeState.Items {
		if item.Depth > 0 && item.Node != nil {
			foundChild = true
			break
		}
	}

	if !foundChild {
		t.Error("Expanded MainWorkflow should show children at depth > 0")
	}
}

func TestBuildTreeByPackage(t *testing.T) {
	styles := NewStyleManager()
	tv := NewTreeView(styles).(*treeView)
	state := createTestState()

	// Initialize tree state
	state.TreeState = &TreeViewState{
		ExpansionStates: make(map[string]bool),
		GroupBy:         "package",
	}

	tv.buildTreeByPackage(state)

	// Should have created tree items grouped by package
	if len(state.TreeState.Items) == 0 {
		t.Error("buildTreeByPackage should create tree items")
	}

	// Should have package headers (items with nil Node)
	hasPackageHeader := false
	for _, item := range state.TreeState.Items {
		if item.Node == nil && item.HasChildren {
			hasPackageHeader = true
			break
		}
	}

	if !hasPackageHeader {
		t.Error("buildTreeByPackage should create package header items")
	}
}

func TestCountNodesInTree(t *testing.T) {
	styles := NewStyleManager()
	tv := NewTreeView(styles).(*treeView)

	root := &packageTreeNode{
		name:     "root",
		fullPath: "root",
		children: map[string]*packageTreeNode{
			"child1": {
				name:     "child1",
				fullPath: "root/child1",
				children: make(map[string]*packageTreeNode),
				nodes:    []*analyzer.TemporalNode{{Name: "A"}, {Name: "B"}},
			},
			"child2": {
				name:     "child2",
				fullPath: "root/child2",
				children: make(map[string]*packageTreeNode),
				nodes:    []*analyzer.TemporalNode{{Name: "C"}},
			},
		},
		nodes: []*analyzer.TemporalNode{{Name: "Root"}},
	}

	count := tv.countNodesInTree(root)
	if count != 4 {
		t.Errorf("countNodesInTree = %d, want 4", count)
	}
}

func TestRestoreSelection(t *testing.T) {
	styles := NewStyleManager()
	tv := NewTreeView(styles).(*treeView)
	state := createTestState()

	// Set up tree items
	state.TreeState = &TreeViewState{
		Items: []TreeItem{
			{Node: &analyzer.TemporalNode{Name: "First"}, Depth: 0},
			{Node: &analyzer.TemporalNode{Name: "Second"}, Depth: 0},
			{Node: &analyzer.TemporalNode{Name: "Third"}, Depth: 0},
		},
		SelectedIndex: 0,
	}

	// Restore selection to Second
	tv.restoreSelection(state, "Second")
	if state.TreeState.SelectedIndex != 1 {
		t.Errorf("restoreSelection should select index 1 for 'Second', got %d", state.TreeState.SelectedIndex)
	}

	// Restore selection to non-existent - should stay in bounds
	state.TreeState.SelectedIndex = 100
	tv.restoreSelection(state, "NonExistent")
	if state.TreeState.SelectedIndex >= len(state.TreeState.Items) {
		t.Errorf("restoreSelection should keep index in bounds, got %d", state.TreeState.SelectedIndex)
	}
}

// =============================================================================
// Details View Tests
// =============================================================================

func TestBuildDetailsState(t *testing.T) {
	styles := NewStyleManager()
	dv := NewDetailsView(styles).(*detailsView)
	state := createTestState()

	// Select MainWorkflow
	state.SelectedNode = state.Graph.Nodes["MainWorkflow"]

	detailsState := dv.buildDetailsState(state)

	if detailsState == nil {
		t.Fatal("buildDetailsState returned nil")
	}

	// MainWorkflow has 2 call sites (ProcessActivity, ChildWorkflow)
	// Plus 0 parents (it's a root)
	// Plus 0 internal calls
	// So total should be 2 selectable items
	expectedCallees := 2
	calleeCount := 0
	for _, item := range detailsState.SelectableItems {
		if item.ItemType == "callee" {
			calleeCount++
		}
	}

	if calleeCount != expectedCallees {
		t.Errorf("Expected %d callee items, got %d", expectedCallees, calleeCount)
	}
}

func TestBuildDetailsStateWithParents(t *testing.T) {
	styles := NewStyleManager()
	dv := NewDetailsView(styles).(*detailsView)
	state := createTestState()

	// Select ProcessActivity which has 2 parents
	state.SelectedNode = state.Graph.Nodes["ProcessActivity"]

	detailsState := dv.buildDetailsState(state)

	// ProcessActivity has 0 call sites, 2 parents, 1 internal call
	callerCount := 0
	internalCount := 0
	for _, item := range detailsState.SelectableItems {
		switch item.ItemType {
		case "caller":
			callerCount++
		case "internal":
			internalCount++
		}
	}

	if callerCount != 2 {
		t.Errorf("Expected 2 caller items, got %d", callerCount)
	}

	if internalCount != 1 {
		t.Errorf("Expected 1 internal call item, got %d", internalCount)
	}
}

func TestBuildDetailsStateEmpty(t *testing.T) {
	styles := NewStyleManager()
	dv := NewDetailsView(styles).(*detailsView)
	state := createTestState()

	// No selected node
	state.SelectedNode = nil

	detailsState := dv.buildDetailsState(state)

	if detailsState == nil {
		t.Fatal("buildDetailsState should not return nil even for nil node")
	}

	if len(detailsState.SelectableItems) != 0 {
		t.Errorf("Expected 0 selectable items for nil node, got %d", len(detailsState.SelectableItems))
	}
}

func TestGetTypeColor(t *testing.T) {
	styles := NewStyleManager()
	dv := NewDetailsView(styles).(*detailsView)

	tests := []struct {
		nodeType string
		notEmpty bool // Just verify we get a color back
	}{
		{"workflow", true},
		{"activity", true},
		{"signal", true},
		{"signal_handler", true},
		{"query", true},
		{"query_handler", true},
		{"update", true},
		{"update_handler", true},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			color := dv.getTypeColor(tt.nodeType)
			if string(color) == "" && tt.notEmpty {
				t.Errorf("getTypeColor(%q) returned empty color", tt.nodeType)
			}
		})
	}
}

// =============================================================================
// View Render Tests (non-fragile - just test structure)
// =============================================================================

func TestListViewRender(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	lv := NewListView(styles, filter)

	state := createTestState()

	output := lv.Render(state)

	if output == "" {
		t.Error("ListView.Render should not return empty string")
	}

	// Just verify it doesn't panic and returns something
	if len(output) < 10 {
		t.Error("ListView.Render output too short")
	}
}

func TestTreeViewRender(t *testing.T) {
	styles := NewStyleManager()
	tv := NewTreeView(styles)

	state := createTestState()
	state.CurrentView = ViewTree

	output := tv.Render(state)

	if output == "" {
		t.Error("TreeView.Render should not return empty string")
	}
}

func TestDetailsViewRender(t *testing.T) {
	styles := NewStyleManager()
	dv := NewDetailsView(styles)

	state := createTestState()
	state.CurrentView = ViewDetails
	state.SelectedNode = state.Graph.Nodes["MainWorkflow"]

	output := dv.Render(state)

	if output == "" {
		t.Error("DetailsView.Render should not return empty string")
	}
}

func TestDetailsViewRenderNoNode(t *testing.T) {
	styles := NewStyleManager()
	dv := NewDetailsView(styles)

	state := createTestState()
	state.CurrentView = ViewDetails
	state.SelectedNode = nil

	output := dv.Render(state)

	if output != "No node selected" {
		t.Errorf("DetailsView.Render with nil node = %q, want 'No node selected'", output)
	}
}

func TestStatsViewRender(t *testing.T) {
	styles := NewStyleManager()
	sv := NewStatsView(styles)

	state := createTestState()
	state.CurrentView = ViewStats

	output := sv.Render(state)

	if output == "" {
		t.Error("StatsView.Render should not return empty string")
	}
}

func TestHelpViewRender(t *testing.T) {
	styles := NewStyleManager()
	hv := NewHelpView(styles)

	state := createTestState()
	state.CurrentView = ViewHelp

	output := hv.Render(state)

	if output == "" {
		t.Error("HelpView.Render should not return empty string")
	}
}

// =============================================================================
// View Name Tests
// =============================================================================

func TestViewNames(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()

	tests := []struct {
		view     View
		expected string
	}{
		{NewListView(styles, filter), ViewList},
		{NewTreeView(styles), ViewTree},
		{NewDetailsView(styles), ViewDetails},
		{NewStatsView(styles), ViewStats},
		{NewHelpView(styles), ViewHelp},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if name := tt.view.Name(); name != tt.expected {
				t.Errorf("View.Name() = %q, want %q", name, tt.expected)
			}
		})
	}
}

// =============================================================================
// CanHandle Tests
// =============================================================================

func TestViewCanHandle(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()

	views := map[string]View{
		ViewList:    NewListView(styles, filter),
		ViewTree:    NewTreeView(styles),
		ViewDetails: NewDetailsView(styles),
		ViewStats:   NewStatsView(styles),
		ViewHelp:    NewHelpView(styles),
	}

	for viewName, view := range views {
		t.Run(viewName, func(t *testing.T) {
			state := createTestState()
			state.CurrentView = viewName

			if !view.CanHandle(nil, state) {
				t.Errorf("%s.CanHandle should return true when CurrentView is %s", viewName, viewName)
			}

			// Should not handle other views
			state.CurrentView = "other"
			if view.CanHandle(nil, state) {
				t.Errorf("%s.CanHandle should return false when CurrentView is 'other'", viewName)
			}
		})
	}
}

