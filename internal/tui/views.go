package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// LIST VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// listView implements the View interface for the list view.
type listView struct {
	styles StyleManager
	filter FilterManager
}

// NewListView creates a new list view.
func NewListView(styles StyleManager, filter FilterManager) View {
	return &listView{
		styles: styles,
		filter: filter,
	}
}

// Name returns the view's name.
func (lv *listView) Name() string {
	return ViewList
}

// Render renders the view with the given model state.
func (lv *listView) Render(state *State) string {
	width := state.WindowWidth
	if width < 40 {
		width = 80
	}

	// Build stunning header
	headerText := "TEMPORAL ANALYZER"
	
	// Build filter status
	var filterStatus []string
	if state.ShowWorkflows {
		filterStatus = append(filterStatus, "⚡Workflows")
	}
	if state.ShowActivities {
		filterStatus = append(filterStatus, "⚙Activities")
	}
	if state.ShowSignals {
		filterStatus = append(filterStatus, "🔔Signals")
	}
	if state.ShowQueries {
		filterStatus = append(filterStatus, "❓Queries")
	}
	
	// Show current view mode
	if !state.ShowActivities && !state.ShowSignals && !state.ShowQueries && state.ShowWorkflows {
		headerText += " │ Top-Level Entry Points"
	} else if len(filterStatus) > 0 {
		headerText += " │ " + strings.Join(filterStatus, " ")
	}

	header := lv.renderHeader(headerText, width)

	// Stats bar (includes filter when active)
	statsBar := lv.renderStatsBar(state, width)

	// Filter bar - always rendered but changes appearance based on state
	filterBar := lv.renderFilterBar(state, width)

	// List content
	listView := state.List.View()

	// Footer with keybindings
	footer := lv.renderFooter(width)

	// Combine all parts - filter bar is always included for stable layout
	var parts []string
	parts = append(parts, header)
	parts = append(parts, statsBar)
	parts = append(parts, filterBar)
	parts = append(parts, listView)
	parts = append(parts, footer)

	return strings.Join(parts, "\n")
}

// renderHeader creates a beautiful header.
func (lv *listView) renderHeader(text string, width int) string {
	// Create gradient header bar
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#161b22")).
		Padding(0, 2).
		Width(width)

	header := headerStyle.Render("⚡ " + text)

	// Add gradient line
	gradient := lv.renderGradient(width)

	return header + "\n" + gradient
}

// renderGradient creates a beautiful gradient line.
func (lv *listView) renderGradient(width int) string {
	colors := []string{"#58a6ff", "#bc8cff", "#79c0ff", "#bc8cff", "#58a6ff"}
	segmentWidth := width / len(colors)
	var gradient strings.Builder

	for i, color := range colors {
		segment := strings.Repeat("▀", segmentWidth)
		if i == len(colors)-1 {
			segment = strings.Repeat("▀", width-i*segmentWidth)
		}
		gradient.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(segment))
	}

	return gradient.String()
}

// renderFilterBar creates the filter input bar - always rendered for stable layout.
func (lv *listView) renderFilterBar(state *State, width int) string {
	if lv.filter.IsActive() {
		// Active filter mode - show input with blinking cursor effect
		style := lipgloss.NewStyle().
			Background(lipgloss.Color("#1f6feb")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true).
			Padding(0, 1).
			Width(width)

		filterText := lv.filter.GetFilterText()
		cursor := "▌" // Block cursor
		
		// Add visual indicator that we're in input mode
		return style.Render("⌨️  FILTER MODE: " + filterText + cursor + "  │  Enter=apply  Esc=cancel  ↑↓=navigate")
	}
	
	// Check if there's an applied filter
	filterText := lv.filter.GetFilterText()
	if filterText != "" {
		// Filter applied but not actively editing
		style := lipgloss.NewStyle().
			Background(lipgloss.Color("#238636")).
			Foreground(lipgloss.Color("#ffffff")).
			Padding(0, 1).
			Width(width)
		
		return style.Render("✓ Filtered: \"" + filterText + "\"  │  / to edit  C to clear all")
	}
	
	// No filter - show hint (subtle)
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#161b22")).
		Foreground(lipgloss.Color("#484f58")).
		Padding(0, 1).
		Width(width)

	return style.Render("   / to search...")
}

// renderStatsBar creates a compact stats summary.
func (lv *listView) renderStatsBar(state *State, width int) string {
	stats := state.Graph.Stats

	// Build stats items
	items := []string{
		fmt.Sprintf("⚡%d workflows", stats.TotalWorkflows),
		fmt.Sprintf("⚙%d activities", stats.TotalActivities),
	}
	if stats.TotalSignals > 0 {
		items = append(items, fmt.Sprintf("🔔%d signals", stats.TotalSignals))
	}
	if stats.TotalQueries > 0 {
		items = append(items, fmt.Sprintf("❓%d queries", stats.TotalQueries))
	}
	items = append(items, fmt.Sprintf("📊 depth:%d", stats.MaxDepth))

	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681")).
		Background(lipgloss.Color("#0d1117")).
		Padding(0, 1)

	return statsStyle.Render(strings.Join(items, "  │  "))
}

// renderFooter creates the footer with keybindings.
func (lv *listView) renderFooter(width int) string {
	bindings := []struct {
		key   string
		label string
	}{
		{"Enter", "Details"},
		{"t", "Tree"},
		{"/", "Filter"},
		{"w", "Workflows"},
		{"a", "Activities"},
		{"?", "Help"},
		{"q", "Quit"},
	}

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58a6ff")).
		Background(lipgloss.Color("#21262d")).
		Padding(0, 1).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681"))

	var parts []string
	for _, b := range bindings {
		parts = append(parts, keyStyle.Render(b.key)+labelStyle.Render(b.label))
	}

	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#161b22")).
		Padding(0, 1).
		Width(width)

	return footerStyle.Render(strings.Join(parts, " "))
}

// Update handles view-specific updates.
func (lv *listView) Update(msg tea.Msg, state *State) (*State, tea.Cmd) {
	// Handle filter input when filter is active
	if lv.filter.IsActive() {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				lv.filter.SetActive(false)
				state.FilterActive = false
				return state, nil
			case "esc":
				lv.filter.SetActive(false)
				state.FilterActive = false
				lv.updateFilteredItemsInView(state)
				return state, nil
			}
		}
		return state, nil
	}

	// Handle enter key for navigation to details when not filtering
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if selected := state.List.SelectedItem(); selected != nil {
				if listItem, ok := selected.(ListItem); ok {
					state.Navigator.PushState(ViewState{
						View:      ViewList,
						ListIndex: state.List.Index(),
						NavPath:   state.Navigator.GetPath(),
					})

					state.SelectedNode = listItem.Node
					state.CurrentView = ViewDetails
					state.DetailsState = nil // Reset to build fresh for new node
					state.Navigator.ClearPath()
					state.Navigator.AddToPath(listItem.Node, DirectionStart)

					return state, nil
				}
			}
		case "g":
			// Go to top
			state.List.Select(0)
			return state, nil
		case "G":
			// Go to bottom
			state.List.Select(len(state.List.Items()) - 1)
			return state, nil
		}
	}

	// Let list handle other updates
	var cmd tea.Cmd
	state.List, cmd = state.List.Update(msg)
	return state, cmd
}

// CanHandle returns true if this view can handle the given message.
func (lv *listView) CanHandle(msg tea.Msg, state *State) bool {
	return state.CurrentView == ViewList
}

// updateFilteredItemsInView updates the list based on current filters.
func (lv *listView) updateFilteredItemsInView(state *State) {
	filteredItems := make([]list.Item, 0, len(state.AllItems))

	for _, item := range state.AllItems {
		if listItem, ok := item.(ListItem); ok {
			if listItem.Node.Type == "workflow" && !state.ShowWorkflows {
				continue
			}
			if listItem.Node.Type == "activity" && !state.ShowActivities {
				continue
			}

			if lv.filter.IsActive() && lv.filter.GetFilter().Value() != "" {
				filterText := lv.filter.GetFilter().Value()
				filtered := lv.filter.ApplyFilter([]list.Item{item}, filterText)
				if len(filtered) == 0 {
					continue
				}
			}

			filteredItems = append(filteredItems, item)
		}
	}

	state.List.SetItems(filteredItems)
	state.ListState.Items = filteredItems
}

// ═══════════════════════════════════════════════════════════════════════════════
// TREE VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// treeView implements the View interface for the tree view.
type treeView struct {
	styles StyleManager
}

// NewTreeView creates a new tree view.
func NewTreeView(styles StyleManager) View {
	return &treeView{
		styles: styles,
	}
}

// Name returns the view's name.
func (tv *treeView) Name() string {
	return ViewTree
}

// Render renders the view with the given model state.
func (tv *treeView) Render(state *State) string {
	width := state.WindowWidth
	if width < 40 {
		width = 80
	}
	height := state.WindowHeight - 6 // Account for header and footer

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#161b22")).
		Padding(0, 2).
		Width(width)

	selectionInfo := ""
	if state.TreeState != nil && len(state.TreeState.Items) > 0 {
		selectionInfo = fmt.Sprintf(" │ %d/%d", state.TreeState.SelectedIndex+1, len(state.TreeState.Items))
	}

	// Show different title based on grouping mode
	title := "🌳 CALL HIERARCHY"
	if state.TreeState != nil && state.TreeState.GroupBy == "package" {
		title = "📦 BY PACKAGE"
	}

	header := headerStyle.Render(title + selectionInfo)

	// Gradient line
	gradient := tv.renderGradient(width, "#7ee787", "#58a6ff")

	// Tree content with proper scrolling
	content := tv.buildTreeContent(state, height)

	// Footer
	footer := tv.renderFooter(state, width)

	return header + "\n" + gradient + "\n" + content + "\n" + footer
}

// renderGradient creates a gradient line with specified colors.
func (tv *treeView) renderGradient(width int, startColor, endColor string) string {
	colors := []string{startColor, endColor, startColor}
	segmentWidth := width / len(colors)
	var gradient strings.Builder

	for i, color := range colors {
		segment := strings.Repeat("▀", segmentWidth)
		if i == len(colors)-1 {
			segment = strings.Repeat("▀", width-i*segmentWidth)
		}
		gradient.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(segment))
	}

	return gradient.String()
}

// renderFooter creates the footer for tree view.
func (tv *treeView) renderFooter(state *State, width int) string {
	viewMode := "hierarchy"
	if state.TreeState != nil && state.TreeState.GroupBy == "package" {
		viewMode = "package"
	}
	
	bindings := []struct {
		key   string
		label string
	}{
		{"j/k", "Navigate"},
		{"h/l", "±"},
		{"Enter", "Open"},
		{"p", "ByPkg"},
		{"H", "ByCall"},
		{"q", "Back"},
	}
	
	_ = viewMode // Will use for display

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7ee787")).
		Background(lipgloss.Color("#21262d")).
		Padding(0, 1).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681"))

	var parts []string
	for _, b := range bindings {
		parts = append(parts, keyStyle.Render(b.key)+labelStyle.Render(b.label))
	}

	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#161b22")).
		Padding(0, 1).
		Width(width)

	return footerStyle.Render(strings.Join(parts, " "))
}

// Update handles view-specific updates.
func (tv *treeView) Update(msg tea.Msg, state *State) (*State, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "j", "down":
			if state.TreeState != nil && state.TreeState.SelectedIndex < len(state.TreeState.Items)-1 {
				state.TreeState.SelectedIndex++
			}
			return state, nil

		case "k", "up":
			if state.TreeState != nil && state.TreeState.SelectedIndex > 0 {
				state.TreeState.SelectedIndex--
			}
			return state, nil

		case "right", "l":
			if state.TreeState != nil && state.TreeState.SelectedIndex < len(state.TreeState.Items) {
				selectedItem := &state.TreeState.Items[state.TreeState.SelectedIndex]
				if selectedItem.HasChildren && !selectedItem.IsExpanded {
					if state.TreeState.ExpansionStates == nil {
						state.TreeState.ExpansionStates = make(map[string]bool)
					}
					// Get the key for expansion state (node name or display text for packages)
					expansionKey := selectedItem.DisplayText
					if selectedItem.Node != nil {
						expansionKey = selectedItem.Node.Name
					}
					state.TreeState.ExpansionStates[expansionKey] = true
					tv.buildTreeItems(state)
					tv.restoreSelection(state, expansionKey)
				}
			}
			return state, nil

		case "left", "h":
			if state.TreeState != nil && state.TreeState.SelectedIndex < len(state.TreeState.Items) {
				selectedItem := &state.TreeState.Items[state.TreeState.SelectedIndex]
				if selectedItem.HasChildren && selectedItem.IsExpanded {
					if state.TreeState.ExpansionStates == nil {
						state.TreeState.ExpansionStates = make(map[string]bool)
					}
					expansionKey := selectedItem.DisplayText
					if selectedItem.Node != nil {
						expansionKey = selectedItem.Node.Name
					}
					state.TreeState.ExpansionStates[expansionKey] = false
					tv.buildTreeItems(state)
					tv.restoreSelection(state, expansionKey)
				}
			}
			return state, nil

		case "p":
			// Toggle to package view
			if state.TreeState != nil {
				state.TreeState.GroupBy = "package"
				state.TreeState.ExpansionStates = make(map[string]bool)
				state.TreeState.SelectedIndex = 0
				tv.buildTreeItems(state)
				state.StatusMessage = "Grouped by package"
				state.StatusType = "info"
			}
			return state, nil

		case "H":
			// Toggle to hierarchy view
			if state.TreeState != nil {
				state.TreeState.GroupBy = "hierarchy"
				state.TreeState.ExpansionStates = make(map[string]bool)
				state.TreeState.SelectedIndex = 0
				tv.buildTreeItems(state)
				state.StatusMessage = "Call hierarchy view"
				state.StatusType = "info"
			}
			return state, nil

		case "e":
			// Expand all
			if state.TreeState != nil {
				for _, item := range state.TreeState.Items {
					if item.HasChildren {
						key := item.DisplayText
						if item.Node != nil {
							key = item.Node.Name
						}
						state.TreeState.ExpansionStates[key] = true
					}
				}
				tv.buildTreeItems(state)
			}
			return state, nil

		case "c":
			// Collapse all
			if state.TreeState != nil {
				state.TreeState.ExpansionStates = make(map[string]bool)
				tv.buildTreeItems(state)
				state.TreeState.SelectedIndex = 0
			}
			return state, nil

		case "enter":
			if state.TreeState != nil && state.TreeState.SelectedIndex < len(state.TreeState.Items) {
				selectedItem := state.TreeState.Items[state.TreeState.SelectedIndex]

				// For package headers (nil Node), toggle expansion instead
				if selectedItem.Node == nil {
					if selectedItem.HasChildren {
						expansionKey := selectedItem.DisplayText
						state.TreeState.ExpansionStates[expansionKey] = !selectedItem.IsExpanded
						tv.buildTreeItems(state)
					}
					return state, nil
				}

				state.Navigator.PushState(ViewState{
					View:      ViewTree,
					TreeIndex: state.TreeState.SelectedIndex,
					NavPath:   state.Navigator.GetPath(),
				})

				state.SelectedNode = selectedItem.Node
				state.CurrentView = ViewDetails
				state.DetailsState = nil // Reset to build fresh for new node
				state.Navigator.ClearPath()
				state.Navigator.AddToPath(selectedItem.Node, DirectionTree)
			}
			return state, nil
		}
	}

	return state, nil
}

// CanHandle returns true if this view can handle the given message.
func (tv *treeView) CanHandle(msg tea.Msg, state *State) bool {
	return state.CurrentView == ViewTree
}

// buildTreeContent builds the tree view content with proper styling.
func (tv *treeView) buildTreeContent(state *State, maxHeight int) string {
	if state.TreeState == nil || len(state.TreeState.Items) == 0 {
		tv.buildTreeItems(state)
	}

	if state.TreeState == nil || len(state.TreeState.Items) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e7681")).
			Italic(true).
			Render("  No workflow hierarchy to display")
	}

	var content strings.Builder

	// Calculate visible range
	visibleStart := 0
	visibleEnd := len(state.TreeState.Items)
	if maxHeight > 0 && len(state.TreeState.Items) > maxHeight {
		// Center the selection in view
		halfHeight := maxHeight / 2
		visibleStart = state.TreeState.SelectedIndex - halfHeight
		if visibleStart < 0 {
			visibleStart = 0
		}
		visibleEnd = visibleStart + maxHeight
		if visibleEnd > len(state.TreeState.Items) {
			visibleEnd = len(state.TreeState.Items)
			visibleStart = visibleEnd - maxHeight
			if visibleStart < 0 {
				visibleStart = 0
			}
		}
	}

	for i := visibleStart; i < visibleEnd; i++ {
		item := state.TreeState.Items[i]
		line := tv.renderTreeItem(item, i == state.TreeState.SelectedIndex)
		content.WriteString(line + "\n")
	}

	return content.String()
}

// renderTreeItem renders a single tree item with beautiful styling.
func (tv *treeView) renderTreeItem(item TreeItem, isSelected bool) string {
	// Build indentation with tree graphics
	var indent strings.Builder
	for d := 0; d < item.Depth; d++ {
		indent.WriteString("  ")
	}

	// Tree branch character
	branchChar := "├─"
	if item.Depth > 0 {
		branchChar = "└─"
	}

	// Expansion icon
	var expandIcon string
	if item.HasChildren {
		if item.IsExpanded {
			expandIcon = "▼"
		} else {
			expandIcon = "▶"
		}
	} else {
		expandIcon = "•"
	}

	// Build the line
	var line strings.Builder
	if item.Depth > 0 {
		line.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#30363d")).Render(indent.String()+branchChar))
	}

	// Format: [expand] [icon] name (count)
	expandStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#58a6ff"))
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e6edf3"))
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6e7681"))

	if item.IsExpanded {
		expandStyle = expandStyle.Foreground(lipgloss.Color("#7ee787"))
	}

	// Handle package headers (nil Node) vs regular nodes
	var itemText string
	if item.Node == nil {
		// Package/directory header
		pkgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffa657")).Bold(true)
		displayName := item.DisplayText
		if displayName == "" {
			displayName = "(root)"
		}
		itemText = fmt.Sprintf(" %s 📁 %s",
			expandStyle.Render(expandIcon),
			pkgStyle.Render(displayName))
		if item.HasChildren && item.ChildCount > 0 {
			itemText += countStyle.Render(fmt.Sprintf(" (%d)", item.ChildCount))
		}
	} else {
		// Regular node
		nodeIcon := getNodeIcon(item.Node.Type)
		displayName := item.Node.Name
		if item.DisplayText != "" {
			displayName = item.DisplayText
		}
		itemText = fmt.Sprintf(" %s %s %s",
			expandStyle.Render(expandIcon),
			nodeIcon,
			nameStyle.Render(displayName))
		if item.HasChildren && item.ChildCount > 0 {
			itemText += countStyle.Render(fmt.Sprintf(" (%d)", item.ChildCount))
		}
	}

	line.WriteString(itemText)

	// Apply selection styling
	finalLine := line.String()
	if isSelected {
		selectedStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#388bfd")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true)
		finalLine = selectedStyle.Render("▶" + finalLine)
	} else {
		finalLine = " " + finalLine
	}

	return finalLine
}

// buildTreeItems builds the tree items from the graph.
func (tv *treeView) buildTreeItems(state *State) {
	if state.TreeState == nil {
		state.TreeState = &TreeViewState{
			ExpansionStates: make(map[string]bool),
			GroupBy:         "hierarchy",
		}
	}

	state.TreeState.Items = []TreeItem{}

	if state.TreeState.GroupBy == "package" {
		tv.buildTreeByPackage(state)
	} else {
		tv.buildTreeByHierarchy(state)
	}

	// Ensure SelectedIndex is within bounds
	if len(state.TreeState.Items) > 0 && state.TreeState.SelectedIndex >= len(state.TreeState.Items) {
		state.TreeState.SelectedIndex = 0
	}
}

// buildTreeByHierarchy builds tree as call hierarchy.
func (tv *treeView) buildTreeByHierarchy(state *State) {
	// Find root nodes (nodes with no parents)
	var rootNodes []*analyzer.TemporalNode
	for _, node := range state.Graph.Nodes {
		if len(node.Parents) == 0 {
			rootNodes = append(rootNodes, node)
		}
	}

	// Sort root nodes by name
	sort.Slice(rootNodes, func(i, j int) bool {
		return rootNodes[i].Name < rootNodes[j].Name
	})

	// Build tree recursively
	visited := make(map[string]bool)
	for _, root := range rootNodes {
		tv.addTreeItemRecursive(state, root, 0, state.TreeState.ExpansionStates, visited)
	}
}

// packageTreeNode represents a node in the package directory tree.
type packageTreeNode struct {
	name     string
	fullPath string
	children map[string]*packageTreeNode
	nodes    []*analyzer.TemporalNode
}

// buildTreeByPackage groups nodes by directory path with FQN hierarchy.
func (tv *treeView) buildTreeByPackage(state *State) {
	// Find common root path
	var allPaths []string
	for _, node := range state.Graph.Nodes {
		if node.FilePath != "" {
			allPaths = append(allPaths, filepath.Dir(node.FilePath))
		}
	}
	commonRoot := findCommonPrefix(allPaths)

	// Build a tree of directories using relative paths
	root := &packageTreeNode{
		name:     "",
		fullPath: "",
		children: make(map[string]*packageTreeNode),
	}

	// Group nodes by directory (using relative paths)
	for _, node := range state.Graph.Nodes {
		if node.FilePath == "" {
			continue
		}
		dir := filepath.Dir(node.FilePath)
		relPath := strings.TrimPrefix(dir, commonRoot)
		relPath = strings.TrimPrefix(relPath, "/")

		// Navigate/create the tree structure
		current := root
		if relPath != "" {
			parts := strings.Split(relPath, "/")
			pathSoFar := ""
			for _, part := range parts {
				if pathSoFar == "" {
					pathSoFar = part
				} else {
					pathSoFar = pathSoFar + "/" + part
				}
				if current.children[part] == nil {
					current.children[part] = &packageTreeNode{
						name:     part,
						fullPath: pathSoFar, // Relative path for cleaner display
						children: make(map[string]*packageTreeNode),
					}
				}
				current = current.children[part]
			}
		}
		current.nodes = append(current.nodes, node)
	}

	// Collapse single-child chains for cleaner display
	root = collapseSingleChildChains(root)

	// Render the tree
	tv.renderPackageTree(state, root, 0)
}

// findCommonPrefix finds the longest common directory prefix.
func findCommonPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		return paths[0]
	}

	// Split first path into parts
	first := strings.Split(paths[0], "/")
	
	// Find common prefix length
	commonLen := len(first)
	for _, p := range paths[1:] {
		parts := strings.Split(p, "/")
		for i := 0; i < commonLen && i < len(parts); i++ {
			if parts[i] != first[i] {
				commonLen = i
				break
			}
		}
		if len(parts) < commonLen {
			commonLen = len(parts)
		}
	}

	return strings.Join(first[:commonLen], "/")
}

// collapseSingleChildChains collapses chains like a/b/c where each has only one child.
func collapseSingleChildChains(node *packageTreeNode) *packageTreeNode {
	// First, recursively collapse children
	for name, child := range node.children {
		node.children[name] = collapseSingleChildChains(child)
	}

	// If this node has exactly one child and no nodes, merge with child
	if len(node.children) == 1 && len(node.nodes) == 0 {
		for childName, child := range node.children {
			if node.name == "" {
				return child
			}
			child.name = node.name + "/" + childName
			return child
		}
	}

	return node
}

// renderPackageTree renders the package tree recursively.
func (tv *treeView) renderPackageTree(state *State, node *packageTreeNode, depth int) {
	// Sort children by name
	var childNames []string
	for name := range node.children {
		childNames = append(childNames, name)
	}
	sort.Strings(childNames)

	// Render children directories
	for _, name := range childNames {
		child := node.children[name]
		totalCount := tv.countNodesInTree(child)
		isExpanded := state.TreeState.ExpansionStates[child.fullPath]

		state.TreeState.Items = append(state.TreeState.Items, TreeItem{
			Node:        nil,
			Depth:       depth,
			DisplayText: child.fullPath, // Full path for expansion key
			HasChildren: totalCount > 0,
			IsExpanded:  isExpanded,
			ChildCount:  totalCount,
		})

		if isExpanded {
			tv.renderPackageTree(state, child, depth+1)
		}
	}

	// Render nodes at this level (if expanded or at root with nodes)
	if len(node.nodes) > 0 {
		// Sort nodes by type, then name
		sort.Slice(node.nodes, func(i, j int) bool {
			typeOrder := map[string]int{"workflow": 0, "activity": 1, "signal": 2, "query": 3, "update": 4}
			ti, tj := typeOrder[node.nodes[i].Type], typeOrder[node.nodes[j].Type]
			if ti != tj {
				return ti < tj
			}
			return node.nodes[i].Name < node.nodes[j].Name
		})

		for _, n := range node.nodes {
			state.TreeState.Items = append(state.TreeState.Items, TreeItem{
				Node:        n,
				Depth:       depth,
				DisplayText: n.Name,
				HasChildren: false,
				IsExpanded:  false,
				ChildCount:  len(n.CallSites),
			})
		}
	}
}

// countNodesInTree counts all nodes in a package tree.
func (tv *treeView) countNodesInTree(node *packageTreeNode) int {
	count := len(node.nodes)
	for _, child := range node.children {
		count += tv.countNodesInTree(child)
	}
	return count
}

// addTreeItemRecursive adds a node and its children to the tree.
func (tv *treeView) addTreeItemRecursive(state *State, node *analyzer.TemporalNode, depth int, expansionStates map[string]bool, visited map[string]bool) {
	// Prevent infinite recursion
	if depth > MaxTreeDepth || visited[node.Name] {
		return
	}
	visited[node.Name] = true
	defer func() { visited[node.Name] = false }()

	hasChildren := len(node.CallSites) > 0
	isExpanded := hasChildren && expansionStates[node.Name]

	item := TreeItem{
		Node:        node,
		Depth:       depth,
		HasChildren: hasChildren,
		IsExpanded:  isExpanded,
		ChildCount:  len(node.CallSites),
	}

	state.TreeState.Items = append(state.TreeState.Items, item)

	// Add children if expanded
	if isExpanded && hasChildren {
		for _, callSite := range node.CallSites {
			for _, targetNode := range state.Graph.Nodes {
				if targetNode.Name == callSite.TargetName {
					tv.addTreeItemRecursive(state, targetNode, depth+1, expansionStates, visited)
					break
				}
			}
		}
	}
}

// restoreSelection finds and selects the item with the given name.
func (tv *treeView) restoreSelection(state *State, name string) {
	if state.TreeState == nil {
		return
	}

	for i, item := range state.TreeState.Items {
		// Check node name or display text (for package headers)
		if item.Node != nil && item.Node.Name == name {
			state.TreeState.SelectedIndex = i
			return
		}
		if item.DisplayText == name {
			state.TreeState.SelectedIndex = i
			return
		}
	}

	if state.TreeState.SelectedIndex >= len(state.TreeState.Items) {
		if len(state.TreeState.Items) > 0 {
			state.TreeState.SelectedIndex = len(state.TreeState.Items) - 1
		} else {
			state.TreeState.SelectedIndex = 0
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// DETAILS VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// detailsView implements the View interface for the details view.
type detailsView struct {
	styles        StyleManager
	runtimeParser *RuntimeParser
}

// NewDetailsView creates a new details view.
func NewDetailsView(styles StyleManager) View {
	return &detailsView{
		styles:        styles,
		runtimeParser: NewRuntimeParser(),
	}
}

// Name returns the view's name.
func (dv *detailsView) Name() string {
	return ViewDetails
}

// Render renders the view with the given model state.
func (dv *detailsView) Render(state *State) string {
	if state.SelectedNode == nil {
		return "No node selected"
	}

	width := state.WindowWidth
	if width < 40 {
		width = 80
	}

	// Initialize details state if needed
	if state.DetailsState == nil {
		state.DetailsState = dv.buildDetailsState(state)
	}

	node := state.SelectedNode

	// Header with node type badge
	header := dv.renderHeader(node, width)

	// Navigation breadcrumb
	breadcrumb := dv.renderBreadcrumb(state, width)

	// Build content sections
	content := dv.buildContent(state, node, width)

	// Footer (with status)
	footer := dv.renderFooter(state, width)

	return header + "\n" + breadcrumb + "\n" + content + "\n" + footer
}

// renderHeader creates the details header with type badge.
func (dv *detailsView) renderHeader(node *analyzer.TemporalNode, width int) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#161b22")).
		Padding(0, 2).
		Width(width)

	// Type badge
	badgeColor := dv.getTypeColor(node.Type)
	badge := lipgloss.NewStyle().
		Background(badgeColor).
		Foreground(lipgloss.Color("#0d1117")).
		Bold(true).
		Padding(0, 1).
		Render(strings.ToUpper(node.Type))

	icon := getNodeIcon(node.Type)
	header := headerStyle.Render(fmt.Sprintf("%s %s  %s", icon, node.Name, badge))

	// Type-specific gradient
	gradient := dv.renderGradient(width, badgeColor)

	return header + "\n" + gradient
}

// getTypeColor returns the color for a node type.
func (dv *detailsView) getTypeColor(nodeType string) lipgloss.Color {
	switch nodeType {
	case "workflow":
		return lipgloss.Color("#a371f7")
	case "activity":
		return lipgloss.Color("#7ee787")
	case "signal", "signal_handler":
		return lipgloss.Color("#ffa657")
	case "query", "query_handler":
		return lipgloss.Color("#79c0ff")
	case "update", "update_handler":
		return lipgloss.Color("#ff7b72")
	default:
		return lipgloss.Color("#58a6ff")
	}
}

// renderGradient creates a gradient from the type color.
func (dv *detailsView) renderGradient(width int, accentColor lipgloss.Color) string {
	var gradient strings.Builder
	segment := strings.Repeat("▀", width)
	gradient.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render(segment))
	return gradient.String()
}

// renderBreadcrumb renders the navigation path.
func (dv *detailsView) renderBreadcrumb(state *State, width int) string {
	path := state.Navigator.GetPath()
	if len(path) == 0 {
		return ""
	}

	pathStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#21262d")).
		Foreground(lipgloss.Color("#6e7681")).
		Padding(0, 1)

	var parts []string
	for _, item := range path {
		parts = append(parts, item.DisplayName)
	}

	return pathStyle.Render("📍 " + strings.Join(parts, " → "))
}

// buildContent builds the main content sections.
func (dv *detailsView) buildContent(state *State, node *analyzer.TemporalNode, width int) string {
	var sections []string

	// Info section
	sections = append(sections, dv.renderInfoSection(node, width))

	// Always show Calls section (Temporal SDK calls)
	sections = append(sections, dv.renderCallsSection(state, node, width))

	// Always show Called by section
	sections = append(sections, dv.renderCallersSection(state, node, width))

	// Internal calls section (non-Temporal function calls)
	if len(node.InternalCalls) > 0 {
		sections = append(sections, dv.renderInternalCallsSection(state, node, width))
	}

	// Signals section (if any)
	if len(node.Signals) > 0 {
		sections = append(sections, dv.renderSignalsSection(node, width))
	}

	// Queries section (if any)
	if len(node.Queries) > 0 {
		sections = append(sections, dv.renderQueriesSection(node, width))
	}

	// Timers section (if any)
	if len(node.Timers) > 0 {
		sections = append(sections, dv.renderTimersSection(node, width))
	}

	return strings.Join(sections, "\n")
}

// renderInfoSection renders the node information section.
func (dv *detailsView) renderInfoSection(node *analyzer.TemporalNode, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#30363d")).
		Padding(0, 1).
		Width(width - 4)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58a6ff")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681")).
		Width(12)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#e6edf3"))

	var content strings.Builder
	content.WriteString(titleStyle.Render("📋 Information") + "\n\n")
	content.WriteString(labelStyle.Render("📁 File:") + valueStyle.Render(node.FilePath) + "\n")
	content.WriteString(labelStyle.Render("📦 Package:") + valueStyle.Render(node.Package) + "\n")
	if node.LineNumber > 0 {
		content.WriteString(labelStyle.Render("📍 Line:") + valueStyle.Render(fmt.Sprintf("%d", node.LineNumber)) + "\n")
	}
	if node.Description != "" {
		content.WriteString(labelStyle.Render("📄 Desc:") + valueStyle.Render(node.Description) + "\n")
	}

	return boxStyle.Render(content.String())
}

// renderCallsSection renders the outgoing calls section.
func (dv *detailsView) renderCallsSection(state *State, node *analyzer.TemporalNode, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7ee787")).
		Padding(0, 1).
		Width(width - 4)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7ee787")).
		Bold(true)

	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681")).
		Italic(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render(fmt.Sprintf("📞 Calls (%d)", len(node.CallSites))) + "\n\n")

	if len(node.CallSites) == 0 {
		content.WriteString(emptyStyle.Render("  No outgoing calls") + "\n")
				} else {
		for i, call := range node.CallSites {
			isSelected := state.DetailsState != nil &&
				i < len(state.DetailsState.SelectableItems) &&
				state.DetailsState.SelectedIndex == i

			line := dv.renderCallItem(state, call, isSelected)
			content.WriteString(line + "\n")
		}
	}

	return boxStyle.Render(content.String())
}

// renderCallItem renders a single call item.
func (dv *detailsView) renderCallItem(state *State, call analyzer.CallSite, isSelected bool) string {
	icon := getNodeIcon(call.TargetType)
	
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e6edf3"))
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6e7681"))

	line := fmt.Sprintf("  %s %s %s",
		icon,
		nameStyle.Render(call.TargetName),
		metaStyle.Render(fmt.Sprintf("(%s:%d)", call.FilePath, call.LineNumber)))

			if isSelected {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#388bfd")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true).
			Render("▶" + line)
	}

	return " " + line
}

// renderCallersSection renders the incoming callers section.
func (dv *detailsView) renderCallersSection(state *State, node *analyzer.TemporalNode, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#ffa657")).
		Padding(0, 1).
		Width(width - 4)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffa657")).
		Bold(true)

	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681")).
		Italic(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render(fmt.Sprintf("📤 Called By (%d)", len(node.Parents))) + "\n\n")

	if len(node.Parents) == 0 {
		content.WriteString(emptyStyle.Render("  No incoming calls (top-level entry point)") + "\n")
	} else {
		callsOffset := len(node.CallSites)

		for i, parentName := range node.Parents {
			isSelected := state.DetailsState != nil &&
				callsOffset+i < len(state.DetailsState.SelectableItems) &&
				state.DetailsState.SelectedIndex == callsOffset+i

			// Find parent node type
			parentType := "workflow"
			for _, n := range state.Graph.Nodes {
				if n.Name == parentName {
					parentType = n.Type
					break
				}
			}

			icon := getNodeIcon(parentType)
			nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e6edf3"))

			line := fmt.Sprintf("  %s %s", icon, nameStyle.Render(parentName))

			if isSelected {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#388bfd")).
					Foreground(lipgloss.Color("#ffffff")).
					Bold(true).
					Render("▶" + line)
			} else {
				line = " " + line
		}

		content.WriteString(line + "\n")
		}
	}

	return boxStyle.Render(content.String())
}

// renderInternalCallsSection renders the internal (non-Temporal) function calls section.
func (dv *detailsView) renderInternalCallsSection(state *State, node *analyzer.TemporalNode, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8b949e")).
		Padding(0, 1).
		Width(width - 4)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8b949e")).
		Bold(true)

	funcStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d2a8ff"))

	methodStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#79c0ff"))

	receiverStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7ee787"))

	lineNumStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681"))

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#388bfd")).
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render(fmt.Sprintf("🔧 Internal Calls (%d)", len(node.InternalCalls))) + "  ")
	content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6e7681")).Italic(true).Render("Enter to drill in"))
	content.WriteString("\n\n")

	// Calculate offset for internal calls in selectable items
	// (calls + parents come before internal calls)
	internalOffset := len(node.CallSites) + len(node.Parents)

	for i, call := range node.InternalCalls {
		isSelected := state.DetailsState != nil &&
			state.DetailsState.SelectedIndex == internalOffset+i

		var line string
		if call.Receiver != "" {
			// Method call: receiver.Method()
			line = fmt.Sprintf("  • %s.%s()",
				receiverStyle.Render(call.Receiver),
				methodStyle.Render(call.TargetName))
			} else {
			// Function call: Function()
			line = fmt.Sprintf("  • %s()", funcStyle.Render(call.TargetName))
			}
		line += lineNumStyle.Render(fmt.Sprintf("  :%d", call.LineNumber))

		if isSelected {
			content.WriteString(selectedStyle.Render("▶" + line) + "\n")
		} else {
			content.WriteString(" " + line + "\n")
		}
	}

	return boxStyle.Render(content.String())
}

// renderSignalsSection renders the signals section.
func (dv *detailsView) renderSignalsSection(node *analyzer.TemporalNode, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#ffa657")).
		Padding(0, 1).
		Width(width - 4)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffa657")).
		Bold(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render(fmt.Sprintf("🔔 Signals (%d)", len(node.Signals))) + "\n\n")

	for _, signal := range node.Signals {
		content.WriteString(fmt.Sprintf("  • %s (handler: %s)\n", signal.Name, signal.Handler))
	}

	return boxStyle.Render(content.String())
}

// renderQueriesSection renders the queries section.
func (dv *detailsView) renderQueriesSection(node *analyzer.TemporalNode, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#79c0ff")).
		Padding(0, 1).
		Width(width - 4)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#79c0ff")).
		Bold(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render(fmt.Sprintf("❓ Queries (%d)", len(node.Queries))) + "\n\n")

	for _, query := range node.Queries {
		content.WriteString(fmt.Sprintf("  • %s (handler: %s)\n", query.Name, query.Handler))
	}

	return boxStyle.Render(content.String())
}

// renderTimersSection renders the timers section.
func (dv *detailsView) renderTimersSection(node *analyzer.TemporalNode, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#d2a8ff")).
		Padding(0, 1).
		Width(width - 4)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d2a8ff")).
		Bold(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render(fmt.Sprintf("⏱ Timers (%d)", len(node.Timers))) + "\n\n")

	for _, timer := range node.Timers {
		timerType := "Timer"
		if timer.IsSleep {
			timerType = "Sleep"
		}
		content.WriteString(fmt.Sprintf("  • %s: %s\n", timerType, timer.Duration))
	}

	return boxStyle.Render(content.String())
}

// renderFooter creates the footer for details view.
func (dv *detailsView) renderFooter(state *State, width int) string {
	bindings := []struct {
		key   string
		label string
	}{
		{"j/k", "Navigate"},
		{"Enter", "Drill In"},
		{"t", "Tree"},
		{"q", "Back"},
	}

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a371f7")).
		Background(lipgloss.Color("#21262d")).
		Padding(0, 1).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681"))

	var parts []string
	for _, b := range bindings {
		parts = append(parts, keyStyle.Render(b.key)+labelStyle.Render(b.label))
	}

	footerContent := strings.Join(parts, " ")
	
	// Show status message if present
	if state.StatusMessage != "" {
		statusColor := "#6e7681"
		switch state.StatusType {
		case "success":
			statusColor = "#7ee787"
		case "warning":
			statusColor = "#d29922"
		case "error":
			statusColor = "#f85149"
		}
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(statusColor)).
			Italic(true)
		footerContent = footerContent + "  " + statusStyle.Render(state.StatusMessage)
	}

	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#161b22")).
		Padding(0, 1).
		Width(width)

	return footerStyle.Render(footerContent)
}

// Update handles view-specific updates.
func (dv *detailsView) Update(msg tea.Msg, state *State) (*State, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "j", "down":
			if state.DetailsState != nil && len(state.DetailsState.SelectableItems) > 0 {
				if state.DetailsState.SelectedIndex < len(state.DetailsState.SelectableItems)-1 {
					state.DetailsState.SelectedIndex++
				}
			}
			return state, nil

		case "k", "up":
			if state.DetailsState != nil && len(state.DetailsState.SelectableItems) > 0 {
				if state.DetailsState.SelectedIndex > 0 {
					state.DetailsState.SelectedIndex--
				}
			}
			return state, nil

		case "enter":
			if state.DetailsState != nil && len(state.DetailsState.SelectableItems) > 0 &&
				state.DetailsState.SelectedIndex < len(state.DetailsState.SelectableItems) {
				selected := state.DetailsState.SelectableItems[state.DetailsState.SelectedIndex]

				// Handle internal calls - dynamically parse the source
				if selected.ItemType == "internal" {
					// Get the target function name and receiver
					var targetName, receiver string
					if selected.InternalCall != nil {
						targetName = selected.InternalCall.TargetName
						receiver = selected.InternalCall.Receiver
					} else {
						// Fallback: extract from DisplayText (e.g., "recv.funcName()" -> "funcName")
						targetName = strings.TrimSuffix(selected.DisplayText, "()")
						if idx := strings.LastIndex(targetName, "."); idx >= 0 {
							receiver = targetName[:idx]
							targetName = targetName[idx+1:]
						}
					}

					if targetName != "" && state.SelectedNode != nil {
						searchPath := state.SelectedNode.FilePath
						callerNode := state.SelectedNode // Remember who called this

						// Try to find the function in the source
						foundNode := dv.runtimeParser.FindFunction(targetName, searchPath)
						if foundNode != nil {
							// Add the caller to Parents so "Called By" shows correctly
							foundNode.Parents = append(foundNode.Parents, callerNode.Name)

							// Push current state for back navigation
							state.Navigator.PushState(ViewState{
								View:         ViewDetails,
								SelectedNode: callerNode,
								DetailsIndex: state.DetailsState.SelectedIndex,
								NavPath:      state.Navigator.GetPath(),
							})

							// Navigate to the found function
							state.SelectedNode = foundNode
							state.Navigator.AddToPath(foundNode, "→ internal")

							// Build new details state for the found function
							state.DetailsState = dv.buildDetailsState(state)
							
							state.StatusMessage = fmt.Sprintf("→ %s", foundNode.Name)
							state.StatusType = "success"
							return state, nil
						} else {
							// Couldn't find the function - show helpful message
							displayName := targetName
							if receiver != "" {
								displayName = receiver + "." + targetName
							}
							state.StatusMessage = fmt.Sprintf("'%s' not found in package (line %d)", displayName, selected.LineNumber)
							state.StatusType = "warning"
						}
					}
					// If we couldn't navigate, stay on the same view
					return state, nil
				}

				// Navigate to the selected node (for calls/callers)
				if selected.Node != nil {
					state.Navigator.PushState(ViewState{
						View:         ViewDetails,
						SelectedNode: state.SelectedNode,
						DetailsIndex: state.DetailsState.SelectedIndex,
						NavPath:      state.Navigator.GetPath(),
					})

					state.SelectedNode = selected.Node
					var direction string
					if selected.ItemType == "callee" {
						direction = DirectionCalls
					} else {
						direction = DirectionCalledBy
					}
					state.Navigator.AddToPath(selected.Node, direction)

					state.DetailsState = dv.buildDetailsState(state)
				} else if selected.ItemType == "caller" {
					// Try to find the caller via runtime parser (for runtime-parsed callers)
					callerNode := dv.runtimeParser.FindFunction(selected.DisplayText, state.SelectedNode.FilePath)
					if callerNode != nil {
						state.Navigator.PushState(ViewState{
							View:         ViewDetails,
							SelectedNode: state.SelectedNode,
							DetailsIndex: state.DetailsState.SelectedIndex,
							NavPath:      state.Navigator.GetPath(),
						})

						state.SelectedNode = callerNode
						state.Navigator.AddToPath(callerNode, DirectionCalledBy)
						state.DetailsState = dv.buildDetailsState(state)
						
						state.StatusMessage = fmt.Sprintf("← %s", callerNode.Name)
						state.StatusType = "success"
					}
				}
			}
			return state, nil
		}
	}

	return state, nil
}

// CanHandle returns true if this view can handle the given message.
func (dv *detailsView) CanHandle(msg tea.Msg, state *State) bool {
	return state.CurrentView == ViewDetails
}

// buildDetailsState builds the details view state.
func (dv *detailsView) buildDetailsState(state *State) *DetailsViewState {
	if state.SelectedNode == nil {
		return &DetailsViewState{}
	}

	var selectableItems []SelectableItem
	node := state.SelectedNode

	// Add call sites as selectable items
	for _, call := range node.CallSites {
			for _, targetNode := range state.Graph.Nodes {
				if targetNode.Name == call.TargetName {
					selectableItems = append(selectableItems, SelectableItem{
					LineIndex:   len(selectableItems),
						Node:        targetNode,
						ItemType:    "callee",
						DisplayText: call.TargetName,
					Section:     "calls",
					FilePath:    targetNode.FilePath,
					LineNumber:  targetNode.LineNumber,
					})
					break
				}
			}
	}

	// Add parents as selectable items
	for _, parentName := range node.Parents {
		var parentNode *analyzer.TemporalNode
		var filePath string
		var lineNum int
		
		// Try to find parent in graph
		for _, pn := range state.Graph.Nodes {
			if pn.Name == parentName {
				parentNode = pn
				filePath = pn.FilePath
				lineNum = pn.LineNumber
				break
			}
		}
		
		// Add even if not in graph (for runtime-parsed callers)
		selectableItems = append(selectableItems, SelectableItem{
			LineIndex:   len(selectableItems),
			Node:        parentNode, // May be nil for runtime callers
			ItemType:    "caller",
			DisplayText: parentName,
			Section:     "callers",
			FilePath:    filePath,
			LineNumber:  lineNum,
		})
	}

	// Add internal calls as selectable items
	for i := range node.InternalCalls {
		call := &node.InternalCalls[i]
		displayText := call.TargetName
		if call.Receiver != "" {
			displayText = call.Receiver + "." + call.TargetName
		}
		selectableItems = append(selectableItems, SelectableItem{
			LineIndex:    len(selectableItems),
			InternalCall: call,
			ItemType:     "internal",
			DisplayText:  displayText + "()",
			Section:      "internal",
			FilePath:     node.FilePath, // Internal calls are in the same file
			LineNumber:   call.LineNumber,
		})
	}

	return &DetailsViewState{
		SelectableItems: selectableItems,
		SelectedIndex:   0,
		ScrollOffset:    0,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATS VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// statsView implements the View interface for the statistics dashboard.
type statsView struct {
	styles StyleManager
}

// NewStatsView creates a new stats view.
func NewStatsView(styles StyleManager) View {
	return &statsView{
		styles: styles,
	}
}

// Name returns the view's name.
func (sv *statsView) Name() string {
	return ViewStats
}

// Render renders the statistics dashboard.
func (sv *statsView) Render(state *State) string {
	width := state.WindowWidth
	if width < 40 {
		width = 80
	}

	stats := state.Graph.Stats

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#161b22")).
		Padding(0, 2).
		Width(width)

	header := headerStyle.Render("📊 STATISTICS DASHBOARD")
	gradient := sv.renderGradient(width)

	// Stats boxes
	boxWidth := (width - 8) / 4

	workflowBox := sv.renderStatBox("⚡ Workflows", stats.TotalWorkflows, "#a371f7", boxWidth)
	activityBox := sv.renderStatBox("⚙ Activities", stats.TotalActivities, "#7ee787", boxWidth)
	signalBox := sv.renderStatBox("🔔 Signals", stats.TotalSignals, "#ffa657", boxWidth)
	depthBox := sv.renderStatBox("📏 Max Depth", stats.MaxDepth, "#79c0ff", boxWidth)

	statsRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		workflowBox, " ",
		activityBox, " ",
		signalBox, " ",
		depthBox,
	)

	// Additional stats
	detailsBox := sv.renderDetailsBox(stats, width-4)

	// Footer
	footer := sv.renderFooter(width)

	return header + "\n" + gradient + "\n\n" + statsRow + "\n\n" + detailsBox + "\n" + footer
}

// renderGradient creates a beautiful gradient line.
func (sv *statsView) renderGradient(width int) string {
	colors := []string{"#58a6ff", "#a371f7", "#7ee787", "#ffa657", "#58a6ff"}
	segmentWidth := width / len(colors)
	var gradient strings.Builder

	for i, color := range colors {
		segment := strings.Repeat("▀", segmentWidth)
		if i == len(colors)-1 {
			segment = strings.Repeat("▀", width-i*segmentWidth)
		}
		gradient.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(segment))
	}

	return gradient.String()
}

// renderStatBox renders a single statistics box.
func (sv *statsView) renderStatBox(label string, value int, color string, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(color)).
		Padding(1, 2).
		Width(width).
		Align(lipgloss.Center)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681"))

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		valueStyle.Render(fmt.Sprintf("%d", value)),
		labelStyle.Render(label),
	)

	return boxStyle.Render(content)
}

// renderDetailsBox renders additional statistics details.
func (sv *statsView) renderDetailsBox(stats analyzer.GraphStats, width int) string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#30363d")).
		Padding(1, 2).
		Width(width)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58a6ff")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681")).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#e6edf3"))

	var content strings.Builder
	content.WriteString(titleStyle.Render("📈 Additional Metrics") + "\n\n")
	content.WriteString(labelStyle.Render("Orphan Nodes:") + valueStyle.Render(fmt.Sprintf("%d", stats.OrphanNodes)) + "\n")
	content.WriteString(labelStyle.Render("Total Connections:") + valueStyle.Render(fmt.Sprintf("%d", stats.TotalConnections)) + "\n")
	content.WriteString(labelStyle.Render("Queries:") + valueStyle.Render(fmt.Sprintf("%d", stats.TotalQueries)) + "\n")
	content.WriteString(labelStyle.Render("Updates:") + valueStyle.Render(fmt.Sprintf("%d", stats.TotalUpdates)) + "\n")
	content.WriteString(labelStyle.Render("Timers:") + valueStyle.Render(fmt.Sprintf("%d", stats.TotalTimers)) + "\n")

	if stats.AvgFanOut > 0 {
		content.WriteString(labelStyle.Render("Avg Fan-Out:") + valueStyle.Render(fmt.Sprintf("%.2f", stats.AvgFanOut)) + "\n")
	}
	if stats.MaxFanOut > 0 {
		content.WriteString(labelStyle.Render("Max Fan-Out:") + valueStyle.Render(fmt.Sprintf("%d", stats.MaxFanOut)) + "\n")
	}

	return boxStyle.Render(content.String())
}

// renderFooter creates the footer for stats view.
func (sv *statsView) renderFooter(width int) string {
	bindings := []struct {
		key   string
		label string
	}{
		{"1", "List"},
		{"2", "Tree"},
		{"r", "Refresh"},
		{"E", "Export"},
		{"q", "Back"},
	}

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58a6ff")).
		Background(lipgloss.Color("#21262d")).
		Padding(0, 1).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681"))

	var parts []string
	for _, b := range bindings {
		parts = append(parts, keyStyle.Render(b.key)+labelStyle.Render(b.label))
	}

	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#161b22")).
		Padding(0, 1).
		Width(width)

	return footerStyle.Render(strings.Join(parts, " "))
}

// Update handles view-specific updates.
func (sv *statsView) Update(msg tea.Msg, state *State) (*State, tea.Cmd) {
	return state, nil
}

// CanHandle returns true if this view can handle the given message.
func (sv *statsView) CanHandle(msg tea.Msg, state *State) bool {
	return state.CurrentView == ViewStats
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELP VIEW
// ═══════════════════════════════════════════════════════════════════════════════

// helpView implements the View interface for the help overlay.
type helpView struct {
	styles StyleManager
}

// NewHelpView creates a new help view.
func NewHelpView(styles StyleManager) View {
	return &helpView{
		styles: styles,
	}
}

// Name returns the view's name.
func (hv *helpView) Name() string {
	return ViewHelp
}

// Render renders the help overlay.
func (hv *helpView) Render(state *State) string {
	width := state.WindowWidth
	if width < 40 {
		width = 80
	}
	if width > 100 {
		width = 100
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#161b22")).
		Padding(0, 2).
		Width(width)

	header := headerStyle.Render("❓ KEYBOARD SHORTCUTS")

	// Help sections
	sections := DefaultKeyBindings()
	var content strings.Builder

	for _, section := range sections {
		sectionStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#58a6ff")).
			Bold(true).
			MarginTop(1)

		content.WriteString(sectionStyle.Render(section.Title) + "\n")

		for _, binding := range section.Bindings {
			keyStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7ee787")).
				Width(16)

			descStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#e6edf3"))

			content.WriteString(fmt.Sprintf("  %s %s\n",
				keyStyle.Render(binding.Key),
				descStyle.Render(binding.Description)))
		}
	}

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#30363d")).
		Padding(1, 2).
		Width(width - 4)

	// Footer
	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#161b22")).
		Foreground(lipgloss.Color("#6e7681")).
		Padding(0, 1).
		Width(width)

	footer := footerStyle.Render("Press ? or Esc to close help")

	return header + "\n" + boxStyle.Render(content.String()) + "\n" + footer
}

// Update handles view-specific updates.
func (hv *helpView) Update(msg tea.Msg, state *State) (*State, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "?", "esc", "q":
			state.CurrentView = state.PreviousView
			if state.CurrentView == "" {
				state.CurrentView = ViewList
			}
			return state, nil
		}
	}
	return state, nil
}

// CanHandle returns true if this view can handle the given message.
func (hv *helpView) CanHandle(msg tea.Msg, state *State) bool {
	return state.CurrentView == ViewHelp
}
