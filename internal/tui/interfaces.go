// Package tui provides a terminal user interface for browsing and analyzing
// Temporal.io workflow and activity graphs.
package tui

import (
	"context"

	"temporal-analyzer/internal/analyzer"
	"temporal-analyzer/internal/tui/theme"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// TUI provides the main terminal user interface.
type TUI interface {
	// Run starts the TUI with the given graph and blocks until the user exits.
	Run(ctx context.Context, graph *analyzer.TemporalGraph) error
}

// Model represents the application state for the TUI.
type Model interface {
	// Init initializes the model.
	Init() tea.Cmd

	// Update handles messages and updates the model.
	Update(tea.Msg) (tea.Model, tea.Cmd)

	// View renders the current view.
	View() string
}

// ViewManager manages different views in the TUI.
type ViewManager interface {
	// GetCurrentView returns the currently active view.
	GetCurrentView(state *State) View

	// SwitchView switches to the specified view.
	SwitchView(viewName string) error

	// GetView returns a view by name.
	GetView(viewName string) View

	// RegisterView registers a new view.
	RegisterView(view View)

	// GetAllViews returns all registered views.
	GetAllViews() map[string]View
}

// View represents a single view in the TUI.
type View interface {
	// Name returns the view's name.
	Name() string

	// Render renders the view with the given model state.
	Render(state *State) string

	// Update handles view-specific updates.
	Update(msg tea.Msg, state *State) (*State, tea.Cmd)

	// CanHandle returns true if this view can handle the given message.
	CanHandle(msg tea.Msg, state *State) bool
}

// Navigator manages navigation state and history.
type Navigator interface {
	// PushState saves the current state to the navigation stack.
	PushState(state ViewState)

	// PopState returns to the previous state from the navigation stack.
	PopState() (ViewState, bool)

	// AddToPath adds a new navigation step to the breadcrumb path.
	AddToPath(node *analyzer.TemporalNode, direction string)

	// GetPath returns the current navigation path.
	GetPath() []PathItem

	// ClearPath clears the navigation path.
	ClearPath()

	// RenderPath renders the navigation path as a string.
	RenderPath() string

	// GetDepth returns the current navigation depth.
	GetDepth() int
}

// StyleManager provides consistent styling across the TUI.
type StyleManager interface {
	// GetHeaderStyle returns the style for headers.
	GetHeaderStyle() tea.Model

	// GetFooterStyle returns the style for footers.
	GetFooterStyle() tea.Model

	// GetHighlightStyle returns the style for highlighted items.
	GetHighlightStyle() tea.Model

	// GetPathStyle returns the style for navigation paths.
	GetPathStyle() tea.Model

	// Header renders a header with the given text.
	Header(text string) string

	// Footer renders a footer with the given text.
	Footer(text string) string

	// SelectedItem renders a selected item with highlighting.
	SelectedItem(text string) string

	// Path renders a navigation path.
	Path(text string) string

	// Error renders error text.
	Error(text string) string

	// Success renders success text.
	Success(text string) string

	// DimText renders text with dimmed/grayed out styling.
	DimText(text string) string

	// Box renders text in a box.
	Box(text string) string

	// Title renders a title.
	Title(text string) string

	// Subtitle renders a subtitle.
	Subtitle(text string) string

	// NodeBadge renders a badge for a node type.
	NodeBadge(nodeType string) string

	// NodeIcon returns the icon for a node type.
	NodeIcon(nodeType string) string

	// ColoredText renders text with the color for a node type.
	ColoredText(text string, nodeType string) string

	// Separator renders a visual separator.
	Separator(width int) string

	// GetStyles returns the underlying theme styles.
	GetStyles() *theme.Styles

	// GetTheme returns the underlying theme.
	GetTheme() *theme.Theme

	// SetNerdFonts enables or disables Nerd Fonts.
	SetNerdFonts(enabled bool)
}

// FilterManager handles filtering and searching functionality.
type FilterManager interface {
	// ApplyFilter applies the given filter to the items.
	ApplyFilter(items []list.Item, filter string) []list.Item

	// IsActive returns true if filtering is currently active.
	IsActive() bool

	// GetFilter returns the current filter input.
	GetFilter() textinput.Model

	// SetActive sets the filter active state.
	SetActive(active bool)

	// UpdateInput updates the filter input model and returns a command.
	UpdateInput(msg tea.Msg) tea.Cmd

	// ClearFilter clears the current filter.
	ClearFilter()

	// GetFilterText returns the current filter text.
	GetFilterText() string

	// SetFilterText sets the filter text.
	SetFilterText(text string)
}

// Exporter provides export functionality for the graph.
type Exporter interface {
	// ExportJSON exports the graph as JSON.
	ExportJSON(graph *analyzer.TemporalGraph) ([]byte, error)

	// ExportDOT exports the graph as DOT format for Graphviz.
	ExportDOT(graph *analyzer.TemporalGraph) (string, error)

	// ExportMermaid exports the graph as Mermaid diagram.
	ExportMermaid(graph *analyzer.TemporalGraph) (string, error)

	// ExportMarkdown exports the graph as Markdown documentation.
	ExportMarkdown(graph *analyzer.TemporalGraph) (string, error)
}
