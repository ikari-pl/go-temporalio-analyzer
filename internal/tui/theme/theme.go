// Package theme provides a beautiful, cohesive visual theme for the TUI.
// Inspired by Catppuccin Mocha with custom temporal-themed accents.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme represents the complete visual theme for the application.
type Theme struct {
	// Base colors
	Base       lipgloss.Color
	Surface    lipgloss.Color
	Overlay    lipgloss.Color
	Muted      lipgloss.Color
	Subtle     lipgloss.Color
	Text       lipgloss.Color
	
	// Accent colors
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Tertiary   lipgloss.Color
	
	// Semantic colors
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Info       lipgloss.Color
	
	// Temporal-specific colors
	Workflow   lipgloss.Color
	Activity   lipgloss.Color
	Signal     lipgloss.Color
	Query      lipgloss.Color
	Update     lipgloss.Color
	Timer      lipgloss.Color
	
	// UI element colors
	Border     lipgloss.Color
	Selection  lipgloss.Color
	Highlight  lipgloss.Color
	
	// Gradient colors for effects
	GradientStart lipgloss.Color
	GradientEnd   lipgloss.Color
}

// DefaultTheme returns the default dark theme (Temporal Midnight).
func DefaultTheme() *Theme {
	return &Theme{
		// Deep space base palette
		Base:    lipgloss.Color("#0d1117"),
		Surface: lipgloss.Color("#161b22"),
		Overlay: lipgloss.Color("#21262d"),
		Muted:   lipgloss.Color("#484f58"),
		Subtle:  lipgloss.Color("#6e7681"),
		Text:    lipgloss.Color("#e6edf3"),
		
		// Vibrant accent colors
		Primary:   lipgloss.Color("#58a6ff"), // Electric blue
		Secondary: lipgloss.Color("#bc8cff"), // Soft purple
		Tertiary:  lipgloss.Color("#79c0ff"), // Sky blue
		
		// Semantic colors
		Success: lipgloss.Color("#3fb950"),
		Warning: lipgloss.Color("#d29922"),
		Error:   lipgloss.Color("#f85149"),
		Info:    lipgloss.Color("#58a6ff"),
		
		// Temporal type colors - distinct and beautiful
		Workflow: lipgloss.Color("#a371f7"), // Purple for workflows
		Activity: lipgloss.Color("#7ee787"), // Green for activities
		Signal:   lipgloss.Color("#ffa657"), // Orange for signals
		Query:    lipgloss.Color("#79c0ff"), // Blue for queries
		Update:   lipgloss.Color("#ff7b72"), // Red for updates
		Timer:    lipgloss.Color("#d2a8ff"), // Light purple for timers
		
		// UI elements
		Border:     lipgloss.Color("#30363d"),
		Selection:  lipgloss.Color("#388bfd"),
		Highlight:  lipgloss.Color("#1f6feb"),
		
		// Gradients
		GradientStart: lipgloss.Color("#58a6ff"),
		GradientEnd:   lipgloss.Color("#bc8cff"),
	}
}

// NeonTheme returns a vibrant neon theme.
func NeonTheme() *Theme {
	return &Theme{
		Base:    lipgloss.Color("#0a0a0f"),
		Surface: lipgloss.Color("#12121a"),
		Overlay: lipgloss.Color("#1a1a24"),
		Muted:   lipgloss.Color("#3a3a4a"),
		Subtle:  lipgloss.Color("#5a5a6a"),
		Text:    lipgloss.Color("#f0f0f5"),
		
		Primary:   lipgloss.Color("#00ffff"), // Cyan
		Secondary: lipgloss.Color("#ff00ff"), // Magenta
		Tertiary:  lipgloss.Color("#00ff88"), // Mint
		
		Success: lipgloss.Color("#00ff88"),
		Warning: lipgloss.Color("#ffff00"),
		Error:   lipgloss.Color("#ff0055"),
		Info:    lipgloss.Color("#00ffff"),
		
		Workflow: lipgloss.Color("#ff00ff"),
		Activity: lipgloss.Color("#00ff88"),
		Signal:   lipgloss.Color("#ffff00"),
		Query:    lipgloss.Color("#00ffff"),
		Update:   lipgloss.Color("#ff0055"),
		Timer:    lipgloss.Color("#ff88ff"),
		
		Border:     lipgloss.Color("#2a2a3a"),
		Selection:  lipgloss.Color("#00ffff"),
		Highlight:  lipgloss.Color("#0088aa"),
		
		GradientStart: lipgloss.Color("#00ffff"),
		GradientEnd:   lipgloss.Color("#ff00ff"),
	}
}

// Styles holds all pre-configured styles for the UI.
type Styles struct {
	theme *Theme
	
	// Layout styles
	App           lipgloss.Style
	Header        lipgloss.Style
	Footer        lipgloss.Style
	Content       lipgloss.Style
	Sidebar       lipgloss.Style
	
	// Component styles
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	Label         lipgloss.Style
	Value         lipgloss.Style
	
	// List styles
	ListItem          lipgloss.Style
	ListItemSelected  lipgloss.Style
	ListItemActive    lipgloss.Style
	
	// Node type styles
	WorkflowBadge  lipgloss.Style
	ActivityBadge  lipgloss.Style
	SignalBadge    lipgloss.Style
	QueryBadge     lipgloss.Style
	UpdateBadge    lipgloss.Style
	TimerBadge     lipgloss.Style
	
	// Status styles
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Error    lipgloss.Style
	Info     lipgloss.Style
	Muted    lipgloss.Style
	
	// Special styles
	Breadcrumb    lipgloss.Style
	KeyBinding    lipgloss.Style
	KeyLabel      lipgloss.Style
	Divider       lipgloss.Style
	Box           lipgloss.Style
	InsetBox      lipgloss.Style
	
	// Tree styles
	TreeBranch    lipgloss.Style
	TreeLeaf      lipgloss.Style
	TreeExpanded  lipgloss.Style
	TreeCollapsed lipgloss.Style
	
	// Details styles
	DetailSection lipgloss.Style
	DetailLabel   lipgloss.Style
	DetailValue   lipgloss.Style
	CodeBlock     lipgloss.Style
}

// NewStyles creates a new Styles instance from a theme.
func NewStyles(theme *Theme) *Styles {
	if theme == nil {
		theme = DefaultTheme()
	}
	
	s := &Styles{theme: theme}
	
	// Layout styles
	s.App = lipgloss.NewStyle().
		Background(theme.Base)
	
	s.Header = lipgloss.NewStyle().
		Foreground(theme.Text).
		Background(theme.Surface).
		Bold(true).
		Padding(0, 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(theme.Border)
	
	s.Footer = lipgloss.NewStyle().
		Foreground(theme.Subtle).
		Background(theme.Surface).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(theme.Border)
	
	s.Content = lipgloss.NewStyle().
		Background(theme.Base).
		Padding(1, 2)
	
	s.Sidebar = lipgloss.NewStyle().
		Background(theme.Surface).
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true).
		BorderForeground(theme.Border).
		Padding(1, 1)
	
	// Component styles
	s.Title = lipgloss.NewStyle().
		Foreground(theme.Text).
		Bold(true).
		MarginBottom(1)
	
	s.Subtitle = lipgloss.NewStyle().
		Foreground(theme.Subtle).
		Italic(true)
	
	s.Label = lipgloss.NewStyle().
		Foreground(theme.Muted)
	
	s.Value = lipgloss.NewStyle().
		Foreground(theme.Text)
	
	// List styles
	s.ListItem = lipgloss.NewStyle().
		Foreground(theme.Text).
		Padding(0, 1)
	
	s.ListItemSelected = lipgloss.NewStyle().
		Foreground(theme.Text).
		Background(theme.Selection).
		Bold(true).
		Padding(0, 1)
	
	s.ListItemActive = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Padding(0, 1)
	
	// Node type badges
	s.WorkflowBadge = lipgloss.NewStyle().
		Foreground(theme.Base).
		Background(theme.Workflow).
		Padding(0, 1).
		Bold(true)
	
	s.ActivityBadge = lipgloss.NewStyle().
		Foreground(theme.Base).
		Background(theme.Activity).
		Padding(0, 1).
		Bold(true)
	
	s.SignalBadge = lipgloss.NewStyle().
		Foreground(theme.Base).
		Background(theme.Signal).
		Padding(0, 1).
		Bold(true)
	
	s.QueryBadge = lipgloss.NewStyle().
		Foreground(theme.Base).
		Background(theme.Query).
		Padding(0, 1).
		Bold(true)
	
	s.UpdateBadge = lipgloss.NewStyle().
		Foreground(theme.Base).
		Background(theme.Update).
		Padding(0, 1).
		Bold(true)
	
	s.TimerBadge = lipgloss.NewStyle().
		Foreground(theme.Base).
		Background(theme.Timer).
		Padding(0, 1).
		Bold(true)
	
	// Status styles
	s.Success = lipgloss.NewStyle().
		Foreground(theme.Success)
	
	s.Warning = lipgloss.NewStyle().
		Foreground(theme.Warning)
	
	s.Error = lipgloss.NewStyle().
		Foreground(theme.Error)
	
	s.Info = lipgloss.NewStyle().
		Foreground(theme.Info)
	
	s.Muted = lipgloss.NewStyle().
		Foreground(theme.Muted)
	
	// Special styles
	s.Breadcrumb = lipgloss.NewStyle().
		Foreground(theme.Subtle).
		Background(theme.Overlay).
		Padding(0, 1)
	
	s.KeyBinding = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Background(theme.Overlay).
		Padding(0, 1).
		Bold(true)
	
	s.KeyLabel = lipgloss.NewStyle().
		Foreground(theme.Subtle)
	
	s.Divider = lipgloss.NewStyle().
		Foreground(theme.Border)
	
	s.Box = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2)
	
	s.InsetBox = lipgloss.NewStyle().
		Background(theme.Overlay).
		Padding(1, 2)
	
	// Tree styles
	s.TreeBranch = lipgloss.NewStyle().
		Foreground(theme.Border)
	
	s.TreeLeaf = lipgloss.NewStyle().
		Foreground(theme.Subtle)
	
	s.TreeExpanded = lipgloss.NewStyle().
		Foreground(theme.Primary)
	
	s.TreeCollapsed = lipgloss.NewStyle().
		Foreground(theme.Muted)
	
	// Details styles
	s.DetailSection = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1).
		MarginBottom(1)
	
	s.DetailLabel = lipgloss.NewStyle().
		Foreground(theme.Subtle).
		Width(16)
	
	s.DetailValue = lipgloss.NewStyle().
		Foreground(theme.Text)
	
	s.CodeBlock = lipgloss.NewStyle().
		Background(theme.Overlay).
		Foreground(theme.Text).
		Padding(1, 2)
	
	return s
}

// GetTheme returns the underlying theme.
func (s *Styles) GetTheme() *Theme {
	return s.theme
}

// Icons provides Unicode icons for different node types and UI elements.
var Icons = struct {
	Workflow    string
	Activity    string
	Signal      string
	Query       string
	Update      string
	Timer       string
	Package     string
	File        string
	Line        string
	Arrow       string
	ArrowRight  string
	ArrowLeft   string
	ArrowDown   string
	TreeBranch  string
	TreeLeaf    string
	TreeExpand  string
	TreeCollapse string
	Check       string
	Cross       string
	Warning     string
	Info        string
	Search      string
	Filter      string
	Stats       string
	Help        string
	Settings    string
	Refresh     string
	Exit        string
	Back        string
	Connection  string
	Depth       string
	Clock       string
	Play        string
	Pause       string
	Stop        string
}{
	Workflow:     "󰒕",  // nf-md-rotate_right
	Activity:     "󰙨",  // nf-md-cog
	Signal:       "󰍡",  // nf-md-bell
	Query:        "󰘦",  // nf-md-help_circle
	Update:       "󰁮",  // nf-md-update
	Timer:        "󰔛",  // nf-md-timer
	Package:      "󰏗",  // nf-md-package
	File:         "󰈙",  // nf-md-file
	Line:         "󰯂",  // nf-md-numeric
	Arrow:        "→",
	ArrowRight:   "▶",
	ArrowLeft:    "◀",
	ArrowDown:    "▼",
	TreeBranch:   "├─",
	TreeLeaf:     "└─",
	TreeExpand:   "▶",
	TreeCollapse: "▼",
	Check:        "✓",
	Cross:        "✗",
	Warning:      "⚠",
	Info:         "ℹ",
	Search:       "󰍉",
	Filter:       "󰈲",
	Stats:        "󰄪",
	Help:         "󰋖",
	Settings:     "󰒓",
	Refresh:      "󰑐",
	Exit:         "󰗼",
	Back:         "󰁍",
	Connection:   "󰌘",
	Depth:        "󰹻",
	Clock:        "󰥔",
	Play:         "▶",
	Pause:        "⏸",
	Stop:         "⏹",
}

// FallbackIcons provides ASCII fallback icons when Nerd Fonts aren't available.
var FallbackIcons = struct {
	Workflow    string
	Activity    string
	Signal      string
	Query       string
	Update      string
	Timer       string
	Package     string
	File        string
	Line        string
	Arrow       string
	ArrowRight  string
	ArrowLeft   string
	TreeBranch  string
	TreeLeaf    string
	TreeExpand  string
	TreeCollapse string
	Check       string
	Cross       string
	Warning     string
	Info        string
	Search      string
	Filter      string
}{
	Workflow:     "⚡",
	Activity:     "⚙",
	Signal:       "🔔",
	Query:        "?",
	Update:       "↻",
	Timer:        "⏱",
	Package:      "📦",
	File:         "📄",
	Line:         "#",
	Arrow:        "→",
	ArrowRight:   ">",
	ArrowLeft:    "<",
	TreeBranch:   "├─",
	TreeLeaf:     "└─",
	TreeExpand:   "+",
	TreeCollapse: "-",
	Check:        "✓",
	Cross:        "✗",
	Warning:      "!",
	Info:         "i",
	Search:       "/",
	Filter:       "~",
}

// NodeIcon returns the appropriate icon for a node type.
func NodeIcon(nodeType string, nerdFonts bool) string {
	if nerdFonts {
		switch nodeType {
		case "workflow":
			return Icons.Workflow
		case "activity":
			return Icons.Activity
		case "signal", "signal_handler":
			return Icons.Signal
		case "query", "query_handler":
			return Icons.Query
		case "update", "update_handler":
			return Icons.Update
		case "timer":
			return Icons.Timer
		default:
			return Icons.Workflow
		}
	}
	
	switch nodeType {
	case "workflow":
		return FallbackIcons.Workflow
	case "activity":
		return FallbackIcons.Activity
	case "signal", "signal_handler":
		return FallbackIcons.Signal
	case "query", "query_handler":
		return FallbackIcons.Query
	case "update", "update_handler":
		return FallbackIcons.Update
	case "timer":
		return FallbackIcons.Timer
	default:
		return FallbackIcons.Workflow
	}
}

// NodeColor returns the color for a node type from the theme.
func (t *Theme) NodeColor(nodeType string) lipgloss.Color {
	switch nodeType {
	case "workflow":
		return t.Workflow
	case "activity":
		return t.Activity
	case "signal", "signal_handler":
		return t.Signal
	case "query", "query_handler":
		return t.Query
	case "update", "update_handler":
		return t.Update
	case "timer":
		return t.Timer
	default:
		return t.Primary
	}
}

