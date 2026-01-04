package tui

import (
	"fmt"
	"strings"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/tui/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// styleManager implements the StyleManager interface with the new theme system.
type styleManager struct {
	theme  *theme.Theme
	styles *theme.Styles
	
	// Computed styles for rendering
	headerStyle    lipgloss.Style
	footerStyle    lipgloss.Style
	highlightStyle lipgloss.Style
	pathStyle      lipgloss.Style
	errorStyle     lipgloss.Style
	successStyle   lipgloss.Style
	dimStyle       lipgloss.Style
	boxStyle       lipgloss.Style
	titleStyle     lipgloss.Style
	subtitleStyle  lipgloss.Style
	
	// New enhanced styles
	gradientChars []string
	useNerdFonts  bool
}

// NewStyleManager creates a new StyleManager instance with the beautiful theme.
func NewStyleManager() StyleManager {
	t := theme.DefaultTheme()
	s := theme.NewStyles(t)
	
	return &styleManager{
		theme:  t,
		styles: s,
		
		headerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(t.Surface).
			Bold(true).
			Padding(0, 2).
			MarginBottom(0),

		footerStyle: lipgloss.NewStyle().
			Foreground(t.Subtle).
			Background(t.Surface).
			Padding(0, 1),

		highlightStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0d1117")).
			Background(t.Selection).
			Bold(true),

		pathStyle: lipgloss.NewStyle().
			Foreground(t.Subtle).
			Background(t.Overlay).
			Padding(0, 1),

		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(t.Error).
			Bold(true).
			Padding(0, 1),

		successStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(t.Success).
			Bold(true).
			Padding(0, 1),

		dimStyle: lipgloss.NewStyle().
			Foreground(t.Muted),

		boxStyle: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(1, 2),

		titleStyle: lipgloss.NewStyle().
			Foreground(t.Text).
			Bold(true),

		subtitleStyle: lipgloss.NewStyle().
			Foreground(t.Subtle).
			Italic(true),

		gradientChars: []string{"█", "▓", "▒", "░"},
		useNerdFonts:  false, // Default to ASCII-safe icons
	}
}

// GetHeaderStyle returns the style for headers.
func (s *styleManager) GetHeaderStyle() tea.Model {
	return nil
}

// GetFooterStyle returns the style for footers.
func (s *styleManager) GetFooterStyle() tea.Model {
	return nil
}

// GetHighlightStyle returns the style for highlighted items.
func (s *styleManager) GetHighlightStyle() tea.Model {
	return nil
}

// GetPathStyle returns the style for navigation paths.
func (s *styleManager) GetPathStyle() tea.Model {
	return nil
}

// Header renders a stunning header with gradient effect.
func (s *styleManager) Header(text string) string {
	width := 80 // Will be set dynamically in render

	// Create the header line with icon
	icon := "⚡"
	headerText := fmt.Sprintf(" %s %s ", icon, text)

	// Create gradient bar effect
	accent := lipgloss.NewStyle().
		Foreground(s.theme.Primary).
		Bold(true)

	// Build the header
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(s.theme.Surface).
		Bold(true).
		Padding(0, 1).
		Width(width).
		Render(headerText)

	// Add a subtle gradient line below
	gradientLine := s.renderGradientLine(width)

	return header + "\n" + accent.Render(gradientLine)
}

// HeaderWithWidth renders a header with specific width.
func (s *styleManager) HeaderWithWidth(text string, width int) string {
	icon := "⚡"
	headerText := fmt.Sprintf(" %s %s ", icon, text)

	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(s.theme.Surface).
		Bold(true).
		Padding(0, 1).
		Width(width).
		Render(headerText)

	gradientLine := s.renderGradientLine(width)

	return header + "\n" + gradientLine
}

// renderGradientLine creates a beautiful gradient line.
func (s *styleManager) renderGradientLine(width int) string {
	if width <= 0 {
		width = 80
	}

	// Create a gradient effect with the primary/secondary colors
	var b strings.Builder
	colors := []lipgloss.Color{
		s.theme.Primary,
		s.theme.Secondary,
		s.theme.Tertiary,
		s.theme.Secondary,
		s.theme.Primary,
	}

	segmentWidth := width / len(colors)
	for i, color := range colors {
		segment := strings.Repeat("▀", segmentWidth)
		if i == len(colors)-1 {
			// Fill remaining width
			segment = strings.Repeat("▀", width-i*segmentWidth)
		}
		b.WriteString(lipgloss.NewStyle().Foreground(color).Render(segment))
	}

	return b.String()
}

// Footer renders a beautiful footer with key bindings.
func (s *styleManager) Footer(text string) string {
	// Parse the text for key bindings like [key]action
	parts := strings.Split(text, " ")
	var rendered strings.Builder

	keyStyle := lipgloss.NewStyle().
		Foreground(s.theme.Primary).
		Background(s.theme.Overlay).
		Bold(true).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(s.theme.Subtle)

	for _, part := range parts {
		if strings.HasPrefix(part, "[") && strings.Contains(part, "]") {
			// This is a key binding
			idx := strings.Index(part, "]")
			key := part[1:idx]
			action := part[idx+1:]
			rendered.WriteString(keyStyle.Render(key))
			rendered.WriteString(labelStyle.Render(action))
			rendered.WriteString(" ")
		} else {
			rendered.WriteString(labelStyle.Render(part))
			rendered.WriteString(" ")
		}
	}

	return lipgloss.NewStyle().
		Background(s.theme.Surface).
		Padding(0, 1).
		Render(strings.TrimSpace(rendered.String()))
}

// Highlight renders highlighted text.
func (s *styleManager) Highlight(text string) string {
	return s.highlightStyle.Render(text)
}

// Path renders a navigation path.
func (s *styleManager) Path(text string) string {
	return s.pathStyle.Render(text)
}

// Error renders error text.
func (s *styleManager) Error(text string) string {
	return s.errorStyle.Render(text)
}

// Success renders success text.
func (s *styleManager) Success(text string) string {
	return s.successStyle.Render(text)
}

// TreeSelected renders a selected tree item.
func (s *styleManager) TreeSelected(text string) string {
	return s.highlightStyle.Render("▶ " + text)
}

// TreeUnselected renders an unselected tree item.
func (s *styleManager) TreeUnselected(text string) string {
	return "  " + text
}

// DetailsSelected renders a selected details item.
func (s *styleManager) DetailsSelected(text string) string {
	return s.highlightStyle.Render("▶ " + text)
}

// DetailsUnselected renders an unselected details item.
func (s *styleManager) DetailsUnselected(text string) string {
	return text
}

// SelectedItem renders a selected item with highlighting.
func (s *styleManager) SelectedItem(text string) string {
	return s.highlightStyle.Render(text)
}

// DimText renders text with dimmed/grayed out styling.
func (s *styleManager) DimText(text string) string {
	return s.dimStyle.Render(text)
}

// Box renders text in a rounded box.
func (s *styleManager) Box(text string) string {
	return s.boxStyle.Render(text)
}

// Title renders a title.
func (s *styleManager) Title(text string) string {
	return s.titleStyle.Render(text)
}

// Subtitle renders a subtitle.
func (s *styleManager) Subtitle(text string) string {
	return s.subtitleStyle.Render(text)
}

// NodeBadge renders a badge for a node type with appropriate color.
func (s *styleManager) NodeBadge(nodeType string) string {
	var badge lipgloss.Style
	var icon, label string

	switch nodeType {
	case "workflow":
		badge = s.styles.WorkflowBadge
		icon = theme.NodeIcon(nodeType, s.useNerdFonts)
		label = "WORKFLOW"
	case "activity":
		badge = s.styles.ActivityBadge
		icon = theme.NodeIcon(nodeType, s.useNerdFonts)
		label = "ACTIVITY"
	case "signal", "signal_handler":
		badge = s.styles.SignalBadge
		icon = theme.NodeIcon(nodeType, s.useNerdFonts)
		label = "SIGNAL"
	case "query", "query_handler":
		badge = s.styles.QueryBadge
		icon = theme.NodeIcon(nodeType, s.useNerdFonts)
		label = "QUERY"
	case "update", "update_handler":
		badge = s.styles.UpdateBadge
		icon = theme.NodeIcon(nodeType, s.useNerdFonts)
		label = "UPDATE"
	case "timer":
		badge = s.styles.TimerBadge
		icon = theme.NodeIcon(nodeType, s.useNerdFonts)
		label = "TIMER"
	default:
		badge = s.styles.WorkflowBadge
		icon = "?"
		label = strings.ToUpper(nodeType)
	}

	return badge.Render(fmt.Sprintf("%s %s", icon, label))
}

// NodeIcon returns the icon for a node type.
func (s *styleManager) NodeIcon(nodeType string) string {
	return theme.NodeIcon(nodeType, s.useNerdFonts)
}

// ColoredText renders text with the color for a node type.
func (s *styleManager) ColoredText(text string, nodeType string) string {
	color := s.theme.NodeColor(nodeType)
	return lipgloss.NewStyle().Foreground(color).Render(text)
}

// Separator renders a visual separator line.
func (s *styleManager) Separator(width int) string {
	if width <= 0 {
		width = 60
	}
	return lipgloss.NewStyle().
		Foreground(s.theme.Border).
		Render(strings.Repeat("─", width))
}

// DoubleSeparator renders a double line separator.
func (s *styleManager) DoubleSeparator(width int) string {
	if width <= 0 {
		width = 60
	}
	return lipgloss.NewStyle().
		Foreground(s.theme.Border).
		Render(strings.Repeat("═", width))
}

// StatBox renders a statistics box with label and value.
func (s *styleManager) StatBox(label string, value interface{}, color lipgloss.Color) string {
	valueStyle := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(s.theme.Subtle)

	return lipgloss.JoinVertical(
		lipgloss.Center,
		valueStyle.Render(fmt.Sprintf("%v", value)),
		labelStyle.Render(label),
	)
}

// ProgressBar renders a simple progress bar.
func (s *styleManager) ProgressBar(current, total, width int) string {
	if width <= 0 {
		width = 20
	}
	if total <= 0 {
		total = 1
	}

	filled := int(float64(current) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	return lipgloss.NewStyle().
		Foreground(s.theme.Primary).
		Render(bar)
}

// GetStyles returns the underlying theme styles.
func (s *styleManager) GetStyles() *theme.Styles {
	return s.styles
}

// GetTheme returns the underlying theme.
func (s *styleManager) GetTheme() *theme.Theme {
	return s.theme
}

// SetNerdFonts enables or disables Nerd Fonts icons.
func (s *styleManager) SetNerdFonts(enabled bool) {
	s.useNerdFonts = enabled
}
