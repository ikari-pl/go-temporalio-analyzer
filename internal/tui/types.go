package tui

import (
	"fmt"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
)

// State represents the complete application state.
type State struct {
	// Core data
	Graph    *analyzer.TemporalGraph
	AllItems []list.Item

	// Current view state
	CurrentView  string
	PreviousView string
	SelectedNode *analyzer.TemporalNode

	// UI components
	List        list.Model
	FilterInput textinput.Model

	// Window dimensions
	WindowWidth  int
	WindowHeight int
	ContentWidth int
	ContentHeight int

	// View-specific state
	ListState    *ListViewState
	TreeState    *TreeViewState
	DetailsState *DetailsViewState
	StatsState   *StatsViewState
	HelpState    *HelpViewState

	// Navigation
	Navigator Navigator

	// Filters
	ShowWorkflows  bool
	ShowActivities bool
	ShowSignals    bool
	ShowQueries    bool
	ShowUpdates    bool
	FilterActive   bool
	FilterText     string

	// UI preferences
	ShowHelp       bool
	ShowStats      bool
	ShowBreadcrumb bool
	CompactMode    bool
	UseNerdFonts   bool

	// Status
	StatusMessage string
	StatusType    string // "info", "success", "warning", "error"
}

// ViewState represents a saved navigation state.
type ViewState struct {
	View         string                 // "list", "details", "tree", "stats", "help"
	SelectedNode *analyzer.TemporalNode // Node being viewed (for details)
	ListIndex    int                    // Selected item in list view
	TreeIndex    int                    // Selected item in tree view
	DetailsIndex int                    // Selected item in details view
	NavPath      []PathItem             // Navigation path at this state
	ScrollOffset int                    // Scroll position
}

// PathItem represents a single step in the navigation path.
type PathItem struct {
	Node        *analyzer.TemporalNode // The node we navigated to
	Direction   string                 // "→" for calls, "←" for called_by, "🌳" for tree
	DisplayName string                 // Short name for display
}

// ListViewState holds state specific to the list view.
type ListViewState struct {
	Items         []list.Item
	SelectedIndex int
	ScrollOffset  int
	SortBy        string // "name", "type", "package", "connections"
	SortAsc       bool
	GroupBy       string // "", "type", "package"
}

// TreeViewState holds state specific to the tree view.
type TreeViewState struct {
	Items           []TreeItem
	SelectedIndex   int
	ScrollOffset    int
	ExpansionStates map[string]bool // Node name -> expanded state
	MaxVisibleDepth int
	ShowOrphans     bool
	GroupBy         string // "hierarchy" (default) or "package"
}

// DetailsViewState holds state specific to the details view.
type DetailsViewState struct {
	Lines           []string
	SelectableItems []SelectableItem
	SelectedIndex   int
	ScrollOffset    int
	Sections        []DetailSection
	ActiveSection   int
}

// DetailSection represents a collapsible section in details view.
type DetailSection struct {
	Title     string
	Content   []string
	Expanded  bool
	Selectable []SelectableItem
}

// StatsViewState holds state for the statistics dashboard.
type StatsViewState struct {
	RefreshInterval int
	LastRefresh     int64
	AnimationFrame  int
	SelectedMetric  int
}

// HelpViewState holds state for the help overlay.
type HelpViewState struct {
	ScrollOffset  int
	ActiveSection int
	Sections      []HelpSection
}

// HelpSection represents a section in the help view.
type HelpSection struct {
	Title    string
	Bindings []KeyBinding
}

// KeyBinding represents a keyboard shortcut.
type KeyBinding struct {
	Key         string
	Description string
	Context     string // "global", "list", "tree", "details"
}

// TreeItem represents an item in the tree view.
type TreeItem struct {
	Node        *analyzer.TemporalNode
	Depth       int    // Indentation level
	DisplayText string // Formatted text with tree graphics
	IsExpanded  bool   // Whether children are shown
	HasChildren bool   // Whether this item has children
	IsOrphan    bool   // Whether this node has no connections
	ChildCount  int    // Number of children
}

// SelectableItem represents a navigable item in details view.
type SelectableItem struct {
	LineIndex    int                    // Which line this item is on
	Node         *analyzer.TemporalNode // The node to navigate to (nil for internal calls)
	InternalCall *analyzer.InternalCall // Internal call info (nil for temporal calls)
	ItemType     string                 // "caller", "callee", "signal", "query", "update", "internal"
	DisplayText  string                 // Text shown for this item
	Section      string                 // Which section this belongs to
	FilePath     string                 // File path for opening
	LineNumber   int                    // Line number for opening
}

// ListItem represents an item in the main list view.
type ListItem struct {
	Node *analyzer.TemporalNode
}

// FilterValue implements list.Item interface.
func (li ListItem) FilterValue() string {
	return li.Node.Name + " " + li.Node.Package + " " + li.Node.FilePath
}

// Title implements list.Item interface.
func (li ListItem) Title() string {
	icon := getNodeIcon(li.Node.Type)
	name := li.Node.Name
	if len(name) > MaxDisplayNameLength {
		return icon + " " + name[:TruncateLength] + EllipsisString
	}
	return icon + " " + name
}

// Description implements list.Item interface.
func (li ListItem) Description() string {
	var extra string
	
	// Count connections
	connections := len(li.Node.CallSites) + len(li.Node.Parents)
	if connections > 0 {
		extra = fmt.Sprintf(" │ %d connections", connections)
	}
	
	// Add signal/query/update counts if present
	if len(li.Node.Signals) > 0 {
		extra += fmt.Sprintf(" │ %d signals", len(li.Node.Signals))
	}
	if len(li.Node.Queries) > 0 {
		extra += fmt.Sprintf(" │ %d queries", len(li.Node.Queries))
	}
	
	return li.Node.Type + " │ " + li.Node.Package + extra
}

// getNodeIcon returns an icon for the node type.
func getNodeIcon(nodeType string) string {
	switch nodeType {
	case "workflow":
		return "⚡"
	case "activity":
		return "⚙"
	case "signal", "signal_handler":
		return "🔔"
	case "query", "query_handler":
		return "❓"
	case "update", "update_handler":
		return "🔄"
	case "timer":
		return "⏱"
	default:
		return "•"
	}
}

// Constants for view names.
const (
	ViewList    = "list"
	ViewDetails = "details"
	ViewTree    = "tree"
	ViewStats   = "stats"
	ViewHelp    = "help"
	ViewGraph   = "graph"
)

// Constants for navigation directions.
const (
	DirectionCalls    = "→"
	DirectionCalledBy = "←"
	DirectionTree     = "🌳"
	DirectionStart    = "📁"
	DirectionSignal   = "📡"
	DirectionQuery    = "❓"
	DirectionUpdate   = "🔄"
)

// Constants for tree expansion icons.
const (
	IconExpanded   = "▼"
	IconCollapsed  = "▶"
	IconLeaf       = "•"
	IconWorkflow   = "⚡"
	IconActivity   = "⚙"
	IconSignal     = "🔔"
	IconQuery      = "❓"
	IconUpdate     = "🔄"
	IconTimer      = "⏱"
)

// Constants for display limits.
const (
	MaxDisplayNameLength = 75
	TruncateLength       = 72
	EllipsisString       = "..."
	MaxNavPathLength     = 10
	MaxTreeDepth         = 50
	DefaultPageSize      = 20
)

// Constants for sort options.
const (
	SortByName        = "name"
	SortByType        = "type"
	SortByPackage     = "package"
	SortByConnections = "connections"
)

// Constants for group options.
const (
	GroupByNone    = ""
	GroupByType    = "type"
	GroupByPackage = "package"
)

// StatusType constants
const (
	StatusInfo    = "info"
	StatusSuccess = "success"
	StatusWarning = "warning"
	StatusError   = "error"
)

// DefaultKeyBindings returns the default set of key bindings.
func DefaultKeyBindings() []HelpSection {
	return []HelpSection{
		{
			Title: "Navigation",
			Bindings: []KeyBinding{
				{Key: "j/↓", Description: "Move down", Context: "global"},
				{Key: "k/↑", Description: "Move up", Context: "global"},
				{Key: "Enter", Description: "Select / Open details", Context: "global"},
				{Key: "Esc/q", Description: "Go back / Quit", Context: "global"},
				{Key: "g", Description: "Go to top", Context: "list"},
				{Key: "G", Description: "Go to bottom", Context: "list"},
			},
		},
		{
			Title: "Views",
			Bindings: []KeyBinding{
				{Key: "1", Description: "List view", Context: "global"},
				{Key: "2", Description: "Tree view", Context: "global"},
				{Key: "3", Description: "Stats dashboard", Context: "global"},
				{Key: "t", Description: "Toggle tree view", Context: "list"},
				{Key: "?", Description: "Help", Context: "global"},
			},
		},
		{
			Title: "Filtering",
			Bindings: []KeyBinding{
				{Key: "/", Description: "Search / Filter", Context: "global"},
				{Key: "w", Description: "Toggle workflows", Context: "list"},
				{Key: "a", Description: "Toggle activities", Context: "list"},
				{Key: "s", Description: "Toggle signals", Context: "list"},
				{Key: "C", Description: "Clear filters", Context: "global"},
			},
		},
		{
			Title: "Tree View",
			Bindings: []KeyBinding{
				{Key: "h/←", Description: "Collapse node", Context: "tree"},
				{Key: "l/→", Description: "Expand node", Context: "tree"},
				{Key: "e", Description: "Expand all", Context: "tree"},
				{Key: "c", Description: "Collapse all", Context: "tree"},
			},
		},
		{
			Title: "Details View",
			Bindings: []KeyBinding{
				{Key: "Tab", Description: "Next section", Context: "details"},
				{Key: "Shift+Tab", Description: "Previous section", Context: "details"},
				{Key: "o", Description: "Open file in editor", Context: "details"},
				{Key: "y", Description: "Copy name to clipboard", Context: "details"},
			},
		},
		{
			Title: "Export",
			Bindings: []KeyBinding{
				{Key: "E", Description: "Export menu", Context: "global"},
				{Key: "Ctrl+e", Description: "Quick export to JSON", Context: "global"},
			},
		},
	}
}
