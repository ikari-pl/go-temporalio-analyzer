package tui

import (
	"strings"
	"temporal-analyzer/internal/analyzer"
)

// navigator implements the Navigator interface.
type navigator struct {
	stack []ViewState
	path  []PathItem
}

// NewNavigator creates a new Navigator instance.
func NewNavigator() Navigator {
	return &navigator{
		stack: make([]ViewState, 0),
		path:  make([]PathItem, 0),
	}
}

// PushState saves the current state to the navigation stack.
func (n *navigator) PushState(state ViewState) {
	n.stack = append(n.stack, state)
}

// PopState returns to the previous state from the navigation stack.
func (n *navigator) PopState() (ViewState, bool) {
	if len(n.stack) == 0 {
		return ViewState{}, false
	}

	// Pop from stack
	last := n.stack[len(n.stack)-1]
	n.stack = n.stack[:len(n.stack)-1]

	// Pop from path if not empty
	if len(n.path) > 0 {
		n.path = n.path[:len(n.path)-1]
	}

	return last, true
}

// AddToPath adds a new navigation step to the breadcrumb path.
func (n *navigator) AddToPath(node *analyzer.TemporalNode, direction string) {
	if node == nil {
		return
	}

	// Truncate name if too long
	displayName := node.Name
	if len(displayName) > 20 {
		displayName = displayName[:17] + "..."
	}

	pathItem := PathItem{
		Node:        node,
		Direction:   direction,
		DisplayName: displayName,
	}

	// Limit path length
	if len(n.path) >= MaxNavPathLength {
		// Remove oldest item
		n.path = n.path[1:]
	}

	n.path = append(n.path, pathItem)
}

// GetPath returns the current navigation path.
func (n *navigator) GetPath() []PathItem {
	return n.path
}

// ClearPath clears the navigation path.
func (n *navigator) ClearPath() {
	n.path = make([]PathItem, 0)
}

// RenderPath renders the navigation path as a formatted string.
func (n *navigator) RenderPath() string {
	if len(n.path) == 0 {
		return ""
	}

	var parts []string
	for i, item := range n.path {
		part := item.DisplayName
		if i > 0 {
			part = item.Direction + " " + part
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, " ")
}

// GetDepth returns the current navigation depth.
func (n *navigator) GetDepth() int {
	return len(n.stack)
}

// Clear clears both the stack and path.
func (n *navigator) Clear() {
	n.stack = make([]ViewState, 0)
	n.path = make([]PathItem, 0)
}

// PeekState returns the top state without removing it.
func (n *navigator) PeekState() (ViewState, bool) {
	if len(n.stack) == 0 {
		return ViewState{}, false
	}
	return n.stack[len(n.stack)-1], true
}

// GetStackSize returns the size of the navigation stack.
func (n *navigator) GetStackSize() int {
	return len(n.stack)
}
