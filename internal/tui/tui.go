package tui

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// tui implements the TUI interface.
type tui struct {
	logger      *slog.Logger
	viewManager ViewManager
	navigator   Navigator
	styles      StyleManager
	filter      FilterManager
}

// NewTUI creates a new TUI instance.
func NewTUI(logger *slog.Logger) TUI {
	navigator := NewNavigator()
	styles := NewStyleManager()
	filter := NewFilterManager()
	viewManager := NewViewManager(styles, filter)

	return &tui{
		logger:      logger,
		viewManager: viewManager,
		navigator:   navigator,
		styles:      styles,
		filter:      filter,
	}
}

// Run starts the TUI with the given graph and blocks until the user exits.
func (t *tui) Run(ctx context.Context, graph *analyzer.TemporalGraph) error {
	if graph == nil {
		return fmt.Errorf("graph cannot be nil")
	}

	// Create initial model
	model := NewModel(graph, t.viewManager, t.navigator, t.styles, t.filter)

	// Create Bubble Tea program with alt screen for full terminal control
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}

// model implements the Model interface and serves as the main application model.
type model struct {
	state       *State
	viewManager ViewManager
	navigator   Navigator
	styles      StyleManager
	filter      FilterManager
	logger      *slog.Logger
}

// NewModel creates a new model instance.
func NewModel(graph *analyzer.TemporalGraph, vm ViewManager, nav Navigator, styles StyleManager, filter FilterManager) Model {
	// Create ALL items for reference (used when filters change)
	allItems := make([]list.Item, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		allItems = append(allItems, ListItem{Node: node})
	}
	
	// Sort all items by name for consistent ordering
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].(ListItem).Node.Name < allItems[j].(ListItem).Node.Name
	})

	// Create initial list items - only top-level workflows (no parents)
	// This shows the entry points into the workflow system
	initialItems := make([]list.Item, 0)
	for _, item := range allItems {
		li := item.(ListItem)
		// Show only top-level nodes (no parents) that are workflows
		if len(li.Node.Parents) == 0 && li.Node.Type == "workflow" {
			initialItems = append(initialItems, item)
		}
	}

	// If no top-level workflows found, fall back to showing all top-level nodes
	if len(initialItems) == 0 {
		for _, item := range allItems {
			li := item.(ListItem)
			if len(li.Node.Parents) == 0 {
				initialItems = append(initialItems, item)
			}
		}
	}

	// Create custom list delegate with beautiful styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(styles.GetTheme().Text).
		Background(styles.GetTheme().Selection).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(styles.GetTheme().Subtle).
		Background(styles.GetTheme().Selection)

	// Create list model with initial (filtered) items
	listModel := list.New(initialItems, delegate, 80, 30)
	listModel.Title = ""
	listModel.SetShowTitle(false)
	listModel.SetShowStatusBar(true)
	listModel.SetFilteringEnabled(false)
	listModel.SetShowHelp(false)

	// Create filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "Type to filter by name, package, or file path..."
	filterInput.CharLimit = 100
	filterInput.Width = 50

	// Create initial state
	state := &State{
		Graph:        graph,
		AllItems:     allItems, // Keep all items for filtering
		CurrentView:  ViewList,
		List:         listModel,
		FilterInput:  filterInput,
		WindowWidth:  80,
		WindowHeight: 30,
		ListState: &ListViewState{
			Items:   initialItems,
			SortBy:  SortByName,
			SortAsc: true,
		},
		TreeState: &TreeViewState{
			ExpansionStates: make(map[string]bool),
		},
		DetailsState:   nil,
		Navigator:      nav,
		ShowWorkflows:  true,
		ShowActivities: false, // Initially hide activities (show only top-level workflows)
		ShowSignals:    false,
		ShowQueries:    false,
		ShowUpdates:    false,
		FilterActive:   false,
		ShowBreadcrumb: true,
		UseNerdFonts:   false,
	}

	return &model{
		state:       state,
		viewManager: vm,
		navigator:   nav,
		styles:      styles,
		filter:      filter,
		logger:      slog.Default(),
	}
}

// Init initializes the model.
func (m *model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowResize(msg)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	default:
		// Handle filter input updates when filter is active
		if m.filter.IsActive() {
			cmd := m.filter.UpdateInput(msg)
			m.updateFilteredItemsWithFilterText(m.filter.GetFilter().Value())
			return m, cmd
		}

		// Let the current view handle other messages
		currentView := m.viewManager.GetCurrentView(m.state)
		if currentView != nil && currentView.CanHandle(msg, m.state) {
			newState, cmd := currentView.Update(msg, m.state)
			m.state = newState
			return m, cmd
		}

		// Update list model if no view handled it
		var cmd tea.Cmd
		m.state.List, cmd = m.state.List.Update(msg)
		return m, cmd
	}
}

// View renders the current view.
func (m *model) View() string {
	currentView := m.viewManager.GetCurrentView(m.state)
	if currentView == nil {
		return "Error: No view available"
	}

	return currentView.Render(m.state)
}

// handleWindowResize handles window resize messages.
func (m *model) handleWindowResize(msg tea.WindowSizeMsg) {
	m.state.WindowWidth = msg.Width
	m.state.WindowHeight = msg.Height

	// Calculate content dimensions
	headerHeight := 3
	footerHeight := 2
	statsBarHeight := 1
	
	m.state.ContentWidth = msg.Width - 4
	m.state.ContentHeight = msg.Height - headerHeight - footerHeight - statsBarHeight

	// Update list dimensions
	availableHeight := m.state.ContentHeight
	if availableHeight < 10 {
		availableHeight = 10
	}

	m.state.List.SetWidth(msg.Width - 4)
	m.state.List.SetHeight(availableHeight)
}

// handleKeyPress handles key press messages.
func (m *model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Always handle ctrl+c
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	// Filter is only active in List view
	if m.filter.IsActive() && m.state.CurrentView == ViewList {
		switch msg.String() {
		case "esc":
			m.filter.ClearFilter()
			m.state.FilterActive = false
			m.updateFilteredItems()
			return m, nil
		case "enter":
			m.filter.SetActive(false)
			m.state.FilterActive = false
			// Keep the filter text applied
			return m, nil
		case "up", "down", "j", "k":
			// Navigation keys exit filter mode and navigate
			m.filter.SetActive(false)
			m.state.FilterActive = false
			// Fall through to handle navigation
		case "tab":
			// Tab exits filter mode
			m.filter.SetActive(false)
			m.state.FilterActive = false
			return m, nil
		default:
			// Pass key to filter input for typing
			cmd := m.filter.UpdateInput(msg)
			m.updateFilteredItemsWithFilterText(m.filter.GetFilterText())
			return m, cmd
		}
	} else if m.filter.IsActive() {
		// If filter somehow got active in non-list view, deactivate it
		m.filter.SetActive(false)
		m.state.FilterActive = false
	}

	// Global key bindings (only when filter is not active)
	switch msg.String() {
	case "q", "esc":
		return m.handleBackNavigation()

	case "t":
		return m.handleTreeView()

	case "/":
		// Filter only works in list view
		if m.state.CurrentView == ViewList {
			return m.handleFilterToggle()
		}

	case "?":
		return m.handleHelpToggle()

	case "1":
		// Switch to list view
		m.state.PreviousView = m.state.CurrentView
		m.state.CurrentView = ViewList
		m.viewManager.SwitchView(ViewList)
		return m, nil

	case "2":
		// Switch to tree view
		return m.handleTreeView()

	case "3":
		// Switch to stats view
		m.state.PreviousView = m.state.CurrentView
		m.state.CurrentView = ViewStats
		m.viewManager.SwitchView(ViewStats)
		return m, nil

	case "w":
		if m.state.CurrentView == ViewList {
			return m.handleWorkflowToggle()
		}

	case "a":
		if m.state.CurrentView == ViewList {
			return m.handleActivityToggle()
		}

	case "s":
		if m.state.CurrentView == ViewList {
			return m.handleSignalToggle()
		}

	case "C":
		// Clear all filters
		m.state.ShowWorkflows = true
		m.state.ShowActivities = true
		m.state.ShowSignals = true
		m.state.ShowQueries = true
		m.state.ShowUpdates = true
		m.filter.ClearFilter()
		m.updateFilteredItems()
		return m, nil
	}

	// Let the current view handle view-specific keys
	currentView := m.viewManager.GetCurrentView(m.state)
	if currentView != nil && currentView.CanHandle(msg, m.state) {
		newState, cmd := currentView.Update(msg, m.state)
		m.state = newState
		return m, cmd
	}

	// Default list handling if not handled by view
	var cmd tea.Cmd
	m.state.List, cmd = m.state.List.Update(msg)
	return m, cmd
}

// handleBackNavigation handles the back navigation (q/esc).
func (m *model) handleBackNavigation() (tea.Model, tea.Cmd) {
	// If in help view, go back
	if m.state.CurrentView == ViewHelp {
		m.state.CurrentView = m.state.PreviousView
		if m.state.CurrentView == "" {
			m.state.CurrentView = ViewList
		}
		return m, nil
	}

	// Try to pop state from navigator
	if prevState, ok := m.navigator.PopState(); ok {
		m.restoreState(prevState)
		return m, nil
	}

	// No previous state, quit if in list view, otherwise go to list
	if m.state.CurrentView == ViewList {
		return m, tea.Quit
	}

	m.state.CurrentView = ViewList
	m.viewManager.SwitchView(ViewList)
	return m, nil
}

// handleTreeView handles switching to tree view.
func (m *model) handleTreeView() (tea.Model, tea.Cmd) {
	// Save current state
	m.navigator.PushState(m.getCurrentViewState())

	// Switch to tree view
	m.state.PreviousView = m.state.CurrentView
	m.state.CurrentView = ViewTree
	m.viewManager.SwitchView(ViewTree)

	// Initialize tree state if needed
	if len(m.state.TreeState.Items) == 0 {
		m.buildTreeItems()
	}

	return m, nil
}

// handleFilterToggle handles toggling the filter.
func (m *model) handleFilterToggle() (tea.Model, tea.Cmd) {
	isActive := m.filter.IsActive()
	m.filter.SetActive(!isActive)
	m.state.FilterActive = !isActive

	if isActive {
		// Deactivating filter - restore all items based on current toggles
		m.updateFilteredItems()
	}

	return m, nil
}

// handleHelpToggle handles toggling the help view.
func (m *model) handleHelpToggle() (tea.Model, tea.Cmd) {
	if m.state.CurrentView == ViewHelp {
		m.state.CurrentView = m.state.PreviousView
		if m.state.CurrentView == "" {
			m.state.CurrentView = ViewList
		}
	} else {
		m.state.PreviousView = m.state.CurrentView
		m.state.CurrentView = ViewHelp
	}
	return m, nil
}

// handleWorkflowToggle handles toggling workflow display.
func (m *model) handleWorkflowToggle() (tea.Model, tea.Cmd) {
	m.state.ShowWorkflows = !m.state.ShowWorkflows
	m.updateFilteredItems()
	return m, nil
}

// handleActivityToggle handles toggling activity display.
func (m *model) handleActivityToggle() (tea.Model, tea.Cmd) {
	m.state.ShowActivities = !m.state.ShowActivities
	m.updateFilteredItems()
	return m, nil
}

// handleSignalToggle handles toggling signal display.
func (m *model) handleSignalToggle() (tea.Model, tea.Cmd) {
	m.state.ShowSignals = !m.state.ShowSignals
	m.updateFilteredItems()
	return m, nil
}

// getCurrentViewState returns the current view state for navigation.
func (m *model) getCurrentViewState() ViewState {
	var detailsIndex int
	if m.state.DetailsState != nil {
		detailsIndex = m.state.DetailsState.SelectedIndex
	}

	return ViewState{
		View:         m.state.CurrentView,
		SelectedNode: m.state.SelectedNode,
		ListIndex:    m.state.List.Index(),
		TreeIndex:    m.state.TreeState.SelectedIndex,
		DetailsIndex: detailsIndex,
		NavPath:      m.navigator.GetPath(),
	}
}

// restoreState restores a previous view state.
func (m *model) restoreState(viewState ViewState) {
	m.state.CurrentView = viewState.View
	m.state.SelectedNode = viewState.SelectedNode

	// Restore navigation path
	m.navigator.ClearPath()
	for _, pathItem := range viewState.NavPath {
		m.navigator.AddToPath(pathItem.Node, pathItem.Direction)
	}

	// Switch to the appropriate view
	m.viewManager.SwitchView(viewState.View)

	// Restore view-specific state
	switch viewState.View {
	case ViewList:
		m.state.List.Select(viewState.ListIndex)
	case ViewTree:
		m.state.TreeState.SelectedIndex = viewState.TreeIndex
	case ViewDetails:
		if m.state.DetailsState != nil {
			m.state.DetailsState.SelectedIndex = viewState.DetailsIndex
		}
		if m.state.SelectedNode != nil {
			m.buildDetailsItems()
		}
	}
}

// buildTreeItems initializes the tree items for the tree view.
// Note: The actual tree building logic is in treeView.buildTreeItems
func (m *model) buildTreeItems() {
	if m.state.TreeState == nil {
		m.state.TreeState = &TreeViewState{
			ExpansionStates: make(map[string]bool),
			GroupBy:         "hierarchy",
		}
	}

	// Use simplified hierarchy build for initial state
	m.state.TreeState.Items = []TreeItem{}

	var rootNodes []*analyzer.TemporalNode
	for _, node := range m.state.Graph.Nodes {
		if len(node.Parents) == 0 {
			rootNodes = append(rootNodes, node)
		}
	}

	sort.Slice(rootNodes, func(i, j int) bool {
		return rootNodes[i].Name < rootNodes[j].Name
	})

	visited := make(map[string]bool)
	for _, root := range rootNodes {
		m.addTreeItemRecursive(root, 0, m.state.TreeState.ExpansionStates, visited)
	}
}

// addTreeItemRecursive adds a node and its children to the tree.
func (m *model) addTreeItemRecursive(node *analyzer.TemporalNode, depth int, expansionStates map[string]bool, visited map[string]bool) {
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

	m.state.TreeState.Items = append(m.state.TreeState.Items, item)

	// Add children if expanded
	if isExpanded && hasChildren {
		for _, callSite := range node.CallSites {
			for _, targetNode := range m.state.Graph.Nodes {
				if targetNode.Name == callSite.TargetName {
					m.addTreeItemRecursive(targetNode, depth+1, expansionStates, visited)
					break
				}
			}
		}
	}
}

// buildDetailsItems builds the details items for the details view.
func (m *model) buildDetailsItems() {
	if m.state.SelectedNode == nil {
		m.state.DetailsState = &DetailsViewState{}
		return
	}

	var selectableItems []SelectableItem
	node := m.state.SelectedNode

	// Add calls section
	for _, call := range node.CallSites {
		for _, targetNode := range m.state.Graph.Nodes {
			if targetNode.Name == call.TargetName {
				selectableItems = append(selectableItems, SelectableItem{
					LineIndex:   len(selectableItems),
					Node:        targetNode,
					ItemType:    "callee",
					DisplayText: call.TargetName,
				})
				break
			}
		}
	}

	// Add called by section
	for _, parentName := range node.Parents {
		for _, parentNode := range m.state.Graph.Nodes {
			if parentNode.Name == parentName {
				selectableItems = append(selectableItems, SelectableItem{
					LineIndex:   len(selectableItems),
					Node:        parentNode,
					ItemType:    "caller",
					DisplayText: parentName,
				})
				break
			}
		}
	}

	m.state.DetailsState = &DetailsViewState{
		SelectableItems: selectableItems,
		SelectedIndex:   0,
		ScrollOffset:    0,
	}
}

// updateFilteredItems updates the list based on current filter and toggle settings.
func (m *model) updateFilteredItems() {
	filteredItems := make([]list.Item, 0, len(m.state.AllItems))

	// Check if we're in "top-level only" mode (only workflows shown, nothing else)
	topLevelOnly := m.state.ShowWorkflows && !m.state.ShowActivities && 
		!m.state.ShowSignals && !m.state.ShowQueries && !m.state.ShowUpdates

	for _, item := range m.state.AllItems {
		if listItem, ok := item.(ListItem); ok {
			// Apply type filters
			switch listItem.Node.Type {
			case "workflow":
				if !m.state.ShowWorkflows {
					continue
				}
				// In top-level only mode, only show workflows with no parents
				if topLevelOnly && len(listItem.Node.Parents) > 0 {
					continue
				}
			case "activity":
				if !m.state.ShowActivities {
					continue
				}
			case "signal", "signal_handler":
				if !m.state.ShowSignals {
					continue
				}
			case "query", "query_handler":
				if !m.state.ShowQueries {
					continue
				}
			case "update", "update_handler":
				if !m.state.ShowUpdates {
					continue
				}
			}

			// Apply text filter if active
			if m.state.FilterActive && m.state.FilterInput.Value() != "" {
				filterText := m.state.FilterInput.Value()
				filtered := m.filter.ApplyFilter([]list.Item{item}, filterText)
				if len(filtered) == 0 {
					continue
				}
			}

			filteredItems = append(filteredItems, item)
		}
	}

	m.state.List.SetItems(filteredItems)
	m.state.ListState.Items = filteredItems
}

// updateFilteredItemsWithFilterText updates the list with a specific filter text.
func (m *model) updateFilteredItemsWithFilterText(filterText string) {
	filteredItems := make([]list.Item, 0, len(m.state.AllItems))

	// Check if we're in "top-level only" mode
	topLevelOnly := m.state.ShowWorkflows && !m.state.ShowActivities && 
		!m.state.ShowSignals && !m.state.ShowQueries && !m.state.ShowUpdates

	for _, item := range m.state.AllItems {
		if listItem, ok := item.(ListItem); ok {
			// Apply type filters
			switch listItem.Node.Type {
			case "workflow":
				if !m.state.ShowWorkflows {
					continue
				}
				// In top-level only mode, only show workflows with no parents
				if topLevelOnly && len(listItem.Node.Parents) > 0 {
					continue
				}
			case "activity":
				if !m.state.ShowActivities {
					continue
				}
			case "signal", "signal_handler":
				if !m.state.ShowSignals {
					continue
				}
			case "query", "query_handler":
				if !m.state.ShowQueries {
					continue
				}
			case "update", "update_handler":
				if !m.state.ShowUpdates {
					continue
				}
			}

			// Apply text filter if provided
			if filterText != "" {
				filtered := m.filter.ApplyFilter([]list.Item{item}, filterText)
				if len(filtered) == 0 {
					continue
				}
			}

			filteredItems = append(filteredItems, item)
		}
	}

	m.state.List.SetItems(filteredItems)
	m.state.ListState.Items = filteredItems
}
