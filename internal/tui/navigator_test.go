package tui

import (
	"testing"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

func TestNewNavigator(t *testing.T) {
	nav := NewNavigator()
	if nav == nil {
		t.Fatal("NewNavigator returned nil")
	}

	// Verify initial state
	if nav.GetDepth() != 0 {
		t.Error("New navigator should have depth 0")
	}
	if len(nav.GetPath()) != 0 {
		t.Error("New navigator should have empty path")
	}
}

func TestNavigatorPushPopState(t *testing.T) {
	nav := NewNavigator()

	// Pop from empty stack should return false
	_, ok := nav.PopState()
	if ok {
		t.Error("PopState on empty stack should return false")
	}

	// Push a state
	state1 := ViewState{
		View:      ViewList,
		ListIndex: 5,
	}
	nav.PushState(state1)

	if nav.GetDepth() != 1 {
		t.Errorf("GetDepth() = %d, want 1", nav.GetDepth())
	}

	// Push another state
	state2 := ViewState{
		View:         ViewDetails,
		DetailsIndex: 10,
	}
	nav.PushState(state2)

	if nav.GetDepth() != 2 {
		t.Errorf("GetDepth() = %d, want 2", nav.GetDepth())
	}

	// Pop should return state2
	popped, ok := nav.PopState()
	if !ok {
		t.Error("PopState should return true")
	}
	if popped.View != ViewDetails {
		t.Errorf("Popped state view = %q, want %q", popped.View, ViewDetails)
	}
	if popped.DetailsIndex != 10 {
		t.Errorf("Popped state DetailsIndex = %d, want 10", popped.DetailsIndex)
	}

	if nav.GetDepth() != 1 {
		t.Errorf("GetDepth() = %d after pop, want 1", nav.GetDepth())
	}

	// Pop should return state1
	popped, ok = nav.PopState()
	if !ok {
		t.Error("PopState should return true")
	}
	if popped.View != ViewList {
		t.Errorf("Popped state view = %q, want %q", popped.View, ViewList)
	}

	// Stack should be empty now
	if nav.GetDepth() != 0 {
		t.Errorf("GetDepth() = %d after all pops, want 0", nav.GetDepth())
	}
}

func TestNavigatorAddToPath(t *testing.T) {
	nav := NewNavigator()

	// Add nil node should not crash
	nav.AddToPath(nil, DirectionCalls)
	if len(nav.GetPath()) != 0 {
		t.Error("AddToPath with nil node should not add to path")
	}

	// Add first node
	node1 := &analyzer.TemporalNode{Name: "Workflow1", Type: "workflow"}
	nav.AddToPath(node1, DirectionStart)

	path := nav.GetPath()
	if len(path) != 1 {
		t.Errorf("Path length = %d, want 1", len(path))
	}
	if path[0].Node != node1 {
		t.Error("Path[0].Node should be node1")
	}
	if path[0].Direction != DirectionStart {
		t.Errorf("Path[0].Direction = %q, want %q", path[0].Direction, DirectionStart)
	}
	if path[0].DisplayName != "Workflow1" {
		t.Errorf("Path[0].DisplayName = %q, want %q", path[0].DisplayName, "Workflow1")
	}

	// Add second node
	node2 := &analyzer.TemporalNode{Name: "Activity1", Type: "activity"}
	nav.AddToPath(node2, DirectionCalls)

	path = nav.GetPath()
	if len(path) != 2 {
		t.Errorf("Path length = %d, want 2", len(path))
	}
	if path[1].Node != node2 {
		t.Error("Path[1].Node should be node2")
	}
}

func TestNavigatorAddToPathTruncatesLongNames(t *testing.T) {
	nav := NewNavigator()

	longName := "ThisIsAVeryLongNameThatExceedsTwentyCharacters"
	node := &analyzer.TemporalNode{Name: longName, Type: "workflow"}
	nav.AddToPath(node, DirectionStart)

	path := nav.GetPath()
	if len(path) != 1 {
		t.Fatal("Expected one path item")
	}

	// Name should be truncated to 20 chars with "..."
	if len(path[0].DisplayName) > 20 {
		t.Errorf("DisplayName should be truncated, got length %d", len(path[0].DisplayName))
	}
	// Should end with "..."
	if path[0].DisplayName[len(path[0].DisplayName)-3:] != "..." {
		t.Errorf("DisplayName should end with '...', got %q", path[0].DisplayName)
	}
}

func TestNavigatorAddToPathRespectsMaxLength(t *testing.T) {
	nav := NewNavigator()

	// Add more than MaxNavPathLength items
	for i := 0; i < MaxNavPathLength+5; i++ {
		node := &analyzer.TemporalNode{Name: "Node", Type: "workflow"}
		nav.AddToPath(node, DirectionCalls)
	}

	path := nav.GetPath()
	if len(path) > MaxNavPathLength {
		t.Errorf("Path length = %d, should not exceed %d", len(path), MaxNavPathLength)
	}
}

func TestNavigatorGetPath(t *testing.T) {
	nav := NewNavigator()

	// Add some nodes
	node1 := &analyzer.TemporalNode{Name: "Node1", Type: "workflow"}
	node2 := &analyzer.TemporalNode{Name: "Node2", Type: "activity"}
	nav.AddToPath(node1, DirectionStart)
	nav.AddToPath(node2, DirectionCalls)

	// Get path
	path1 := nav.GetPath()
	path2 := nav.GetPath()

	// Verify it returns a copy (not the same slice)
	if &path1[0] == &path2[0] {
		t.Error("GetPath should return a copy, not the original slice")
	}

	// Verify contents are the same
	if len(path1) != len(path2) {
		t.Error("Path copies should have same length")
	}
}

func TestNavigatorClearPath(t *testing.T) {
	nav := NewNavigator()

	// Add some nodes
	node := &analyzer.TemporalNode{Name: "Node", Type: "workflow"}
	nav.AddToPath(node, DirectionStart)
	nav.AddToPath(node, DirectionCalls)

	// Verify path is not empty
	if len(nav.GetPath()) == 0 {
		t.Fatal("Path should not be empty before clear")
	}

	// Clear path
	nav.ClearPath()

	// Verify path is empty
	if len(nav.GetPath()) != 0 {
		t.Error("ClearPath should empty the path")
	}
}

func TestNavigatorRenderPath(t *testing.T) {
	nav := NewNavigator()

	// Empty path
	if nav.RenderPath() != "" {
		t.Error("RenderPath on empty path should return empty string")
	}

	// Add first node
	node1 := &analyzer.TemporalNode{Name: "Workflow", Type: "workflow"}
	nav.AddToPath(node1, DirectionStart)

	rendered := nav.RenderPath()
	if rendered != "Workflow" {
		t.Errorf("RenderPath() = %q, want %q", rendered, "Workflow")
	}

	// Add second node
	node2 := &analyzer.TemporalNode{Name: "Activity", Type: "activity"}
	nav.AddToPath(node2, DirectionCalls)

	rendered = nav.RenderPath()
	// Should contain both names with direction
	if rendered == "" {
		t.Error("RenderPath should not be empty")
	}
	// First item should not have direction prefix
	// Second item should have direction
	if !containsStr(rendered, "Workflow") {
		t.Errorf("RenderPath() = %q, should contain 'Workflow'", rendered)
	}
	if !containsStr(rendered, "Activity") {
		t.Errorf("RenderPath() = %q, should contain 'Activity'", rendered)
	}
}

func TestNavigatorGetDepth(t *testing.T) {
	nav := NewNavigator()

	if nav.GetDepth() != 0 {
		t.Errorf("Initial depth = %d, want 0", nav.GetDepth())
	}

	// Push states and verify depth
	for i := 1; i <= 5; i++ {
		nav.PushState(ViewState{View: ViewList})
		if nav.GetDepth() != i {
			t.Errorf("After %d pushes, depth = %d, want %d", i, nav.GetDepth(), i)
		}
	}

	// Pop and verify depth decreases
	for i := 4; i >= 0; i-- {
		nav.PopState()
		if nav.GetDepth() != i {
			t.Errorf("After pop, depth = %d, want %d", nav.GetDepth(), i)
		}
	}
}

func TestNavigatorClear(t *testing.T) {
	nav := NewNavigator()

	// Add some state
	node := &analyzer.TemporalNode{Name: "Node", Type: "workflow"}
	nav.AddToPath(node, DirectionStart)
	nav.PushState(ViewState{View: ViewList})
	nav.PushState(ViewState{View: ViewDetails})

	// Verify non-empty
	if nav.GetDepth() == 0 {
		t.Fatal("Depth should not be 0 before clear")
	}
	if len(nav.GetPath()) == 0 {
		t.Fatal("Path should not be empty before clear")
	}

	// Clear
	nav.(*navigator).Clear()

	// Verify empty
	if nav.GetDepth() != 0 {
		t.Error("Clear should reset depth to 0")
	}
	if len(nav.GetPath()) != 0 {
		t.Error("Clear should empty the path")
	}
}

func TestNavigatorPeekState(t *testing.T) {
	nav := NewNavigator()

	// Peek on empty stack
	_, ok := nav.(*navigator).PeekState()
	if ok {
		t.Error("PeekState on empty stack should return false")
	}

	// Push a state
	state := ViewState{View: ViewList, ListIndex: 5}
	nav.PushState(state)

	// Peek should return the state without removing it
	peeked, ok := nav.(*navigator).PeekState()
	if !ok {
		t.Error("PeekState should return true after push")
	}
	if peeked.View != ViewList {
		t.Errorf("PeekState view = %q, want %q", peeked.View, ViewList)
	}

	// Depth should still be 1
	if nav.GetDepth() != 1 {
		t.Error("PeekState should not change depth")
	}

	// Peek again should return same state
	peeked2, _ := nav.(*navigator).PeekState()
	if peeked2.View != peeked.View {
		t.Error("Multiple PeekState calls should return same state")
	}
}

func TestNavigatorGetStackSize(t *testing.T) {
	nav := NewNavigator()

	if nav.(*navigator).GetStackSize() != 0 {
		t.Error("Initial stack size should be 0")
	}

	nav.PushState(ViewState{})
	if nav.(*navigator).GetStackSize() != 1 {
		t.Error("Stack size should be 1 after push")
	}

	nav.PushState(ViewState{})
	if nav.(*navigator).GetStackSize() != 2 {
		t.Error("Stack size should be 2 after second push")
	}

	nav.PopState()
	if nav.(*navigator).GetStackSize() != 1 {
		t.Error("Stack size should be 1 after pop")
	}
}

func TestNavigatorPopStateRemovesFromPath(t *testing.T) {
	nav := NewNavigator()

	// Add to path while pushing states
	node1 := &analyzer.TemporalNode{Name: "Node1", Type: "workflow"}
	node2 := &analyzer.TemporalNode{Name: "Node2", Type: "activity"}

	nav.AddToPath(node1, DirectionStart)
	nav.PushState(ViewState{View: ViewList})

	nav.AddToPath(node2, DirectionCalls)
	nav.PushState(ViewState{View: ViewDetails})

	// Path should have 2 items
	if len(nav.GetPath()) != 2 {
		t.Errorf("Path length = %d, want 2", len(nav.GetPath()))
	}

	// Pop should also pop from path
	nav.PopState()
	if len(nav.GetPath()) != 1 {
		t.Errorf("Path length after pop = %d, want 1", len(nav.GetPath()))
	}

	// Pop again
	nav.PopState()
	if len(nav.GetPath()) != 0 {
		t.Errorf("Path length after second pop = %d, want 0", len(nav.GetPath()))
	}
}

func TestNavigatorViewStatePreservation(t *testing.T) {
	nav := NewNavigator()

	// Create a complex state
	node := &analyzer.TemporalNode{Name: "TestNode", Type: "workflow"}
	originalState := ViewState{
		View:         ViewDetails,
		SelectedNode: node,
		ListIndex:    10,
		TreeIndex:    5,
		DetailsIndex: 3,
		NavPath: []PathItem{
			{Node: node, Direction: DirectionStart, DisplayName: "TestNode"},
		},
		ScrollOffset: 100,
	}

	// Push and pop
	nav.PushState(originalState)
	popped, ok := nav.PopState()

	if !ok {
		t.Fatal("PopState should return true")
	}

	// Verify all fields are preserved
	if popped.View != originalState.View {
		t.Errorf("View mismatch: %q vs %q", popped.View, originalState.View)
	}
	if popped.SelectedNode != originalState.SelectedNode {
		t.Error("SelectedNode mismatch")
	}
	if popped.ListIndex != originalState.ListIndex {
		t.Errorf("ListIndex mismatch: %d vs %d", popped.ListIndex, originalState.ListIndex)
	}
	if popped.TreeIndex != originalState.TreeIndex {
		t.Errorf("TreeIndex mismatch: %d vs %d", popped.TreeIndex, originalState.TreeIndex)
	}
	if popped.DetailsIndex != originalState.DetailsIndex {
		t.Errorf("DetailsIndex mismatch: %d vs %d", popped.DetailsIndex, originalState.DetailsIndex)
	}
	if popped.ScrollOffset != originalState.ScrollOffset {
		t.Errorf("ScrollOffset mismatch: %d vs %d", popped.ScrollOffset, originalState.ScrollOffset)
	}
}

