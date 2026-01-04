package tui

import (
	"strings"
	"testing"
)

func TestNewStyleManager(t *testing.T) {
	sm := NewStyleManager()
	if sm == nil {
		t.Fatal("NewStyleManager returned nil")
	}
}

func TestStyleManagerHeader(t *testing.T) {
	sm := NewStyleManager()

	header := sm.Header("Test Header")
	if header == "" {
		t.Error("Header returned empty string")
	}

	// Should contain the text
	if !strings.Contains(header, "Test Header") {
		t.Error("Header should contain the provided text")
	}
}

func TestStyleManagerHeaderWithWidth(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	header := sm.HeaderWithWidth("Test", 100)
	if header == "" {
		t.Error("HeaderWithWidth returned empty string")
	}
}

func TestStyleManagerFooter(t *testing.T) {
	sm := NewStyleManager()

	tests := []struct {
		name  string
		input string
	}{
		{"simple text", "Press q to quit"},
		{"with key binding", "[q]Quit [?]Help"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			footer := sm.Footer(tt.input)
			// Should not panic and should return something (possibly styled empty)
			_ = footer
		})
	}
}

func TestStyleManagerHighlight(t *testing.T) {
	sm := NewStyleManager()

	result := sm.(*styleManager).Highlight("highlighted text")
	if result == "" {
		t.Error("Highlight returned empty string")
	}
}

func TestStyleManagerPath(t *testing.T) {
	sm := NewStyleManager()

	result := sm.Path("Main > Sub > Item")
	if result == "" {
		t.Error("Path returned empty string")
	}
}

func TestStyleManagerError(t *testing.T) {
	sm := NewStyleManager()

	result := sm.Error("Error message")
	if result == "" {
		t.Error("Error returned empty string")
	}

	if !strings.Contains(result, "Error message") {
		t.Error("Error should contain the error message")
	}
}

func TestStyleManagerSuccess(t *testing.T) {
	sm := NewStyleManager()

	result := sm.Success("Success message")
	if result == "" {
		t.Error("Success returned empty string")
	}

	if !strings.Contains(result, "Success message") {
		t.Error("Success should contain the success message")
	}
}

func TestStyleManagerSelectedItem(t *testing.T) {
	sm := NewStyleManager()

	result := sm.SelectedItem("Selected")
	if result == "" {
		t.Error("SelectedItem returned empty string")
	}
}

func TestStyleManagerDimText(t *testing.T) {
	sm := NewStyleManager()

	result := sm.DimText("Dimmed text")
	if result == "" {
		t.Error("DimText returned empty string")
	}

	if !strings.Contains(result, "Dimmed text") {
		t.Error("DimText should contain the text")
	}
}

func TestStyleManagerBox(t *testing.T) {
	sm := NewStyleManager()

	result := sm.Box("Box content")
	if result == "" {
		t.Error("Box returned empty string")
	}
}

func TestStyleManagerTitle(t *testing.T) {
	sm := NewStyleManager()

	result := sm.Title("Title Text")
	if result == "" {
		t.Error("Title returned empty string")
	}

	if !strings.Contains(result, "Title Text") {
		t.Error("Title should contain the text")
	}
}

func TestStyleManagerSubtitle(t *testing.T) {
	sm := NewStyleManager()

	result := sm.Subtitle("Subtitle Text")
	if result == "" {
		t.Error("Subtitle returned empty string")
	}

	if !strings.Contains(result, "Subtitle Text") {
		t.Error("Subtitle should contain the text")
	}
}

func TestStyleManagerNodeBadge(t *testing.T) {
	sm := NewStyleManager()

	tests := []struct {
		nodeType string
		expected string
	}{
		{"workflow", "WORKFLOW"},
		{"activity", "ACTIVITY"},
		{"signal", "SIGNAL"},
		{"signal_handler", "SIGNAL"},
		{"query", "QUERY"},
		{"query_handler", "QUERY"},
		{"update", "UPDATE"},
		{"update_handler", "UPDATE"},
		{"timer", "TIMER"},
		{"unknown", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			badge := sm.NodeBadge(tt.nodeType)
			if badge == "" {
				t.Errorf("NodeBadge(%q) returned empty string", tt.nodeType)
			}
			if !strings.Contains(badge, tt.expected) {
				t.Errorf("NodeBadge(%q) = %q, should contain %q", tt.nodeType, badge, tt.expected)
			}
		})
	}
}

func TestStyleManagerNodeIcon(t *testing.T) {
	sm := NewStyleManager()

	tests := []string{"workflow", "activity", "signal", "query", "update", "timer", "unknown"}

	for _, nodeType := range tests {
		t.Run(nodeType, func(t *testing.T) {
			icon := sm.NodeIcon(nodeType)
			if icon == "" {
				t.Errorf("NodeIcon(%q) returned empty string", nodeType)
			}
		})
	}
}

func TestStyleManagerColoredText(t *testing.T) {
	sm := NewStyleManager()

	tests := []string{"workflow", "activity", "signal", "query", "update", "timer", "unknown"}

	for _, nodeType := range tests {
		t.Run(nodeType, func(t *testing.T) {
			result := sm.ColoredText("Test", nodeType)
			if !strings.Contains(result, "Test") {
				t.Errorf("ColoredText should contain the text")
			}
		})
	}
}

func TestStyleManagerSeparator(t *testing.T) {
	sm := NewStyleManager()

	tests := []struct {
		name  string
		width int
	}{
		{"zero width", 0},
		{"negative width", -10},
		{"normal width", 60},
		{"large width", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sep := sm.Separator(tt.width)
			if sep == "" {
				t.Error("Separator returned empty string")
			}
		})
	}
}

func TestStyleManagerDoubleSeparator(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	tests := []struct {
		name  string
		width int
	}{
		{"zero width", 0},
		{"negative width", -10},
		{"normal width", 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sep := sm.DoubleSeparator(tt.width)
			if sep == "" {
				t.Error("DoubleSeparator returned empty string")
			}
		})
	}
}

func TestStyleManagerStatBox(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	result := sm.StatBox("Workflows", 42, "#58a6ff")
	if result == "" {
		t.Error("StatBox returned empty string")
	}

	if !strings.Contains(result, "42") {
		t.Error("StatBox should contain the value")
	}

	if !strings.Contains(result, "Workflows") {
		t.Error("StatBox should contain the label")
	}
}

func TestStyleManagerProgressBar(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	tests := []struct {
		name    string
		current int
		total   int
		width   int
	}{
		{"zero progress", 0, 100, 20},
		{"half progress", 50, 100, 20},
		{"full progress", 100, 100, 20},
		{"over progress", 150, 100, 20},
		{"zero total", 10, 0, 20},
		{"zero width", 50, 100, 0},
		{"negative width", 50, 100, -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := sm.ProgressBar(tt.current, tt.total, tt.width)
			if bar == "" {
				t.Error("ProgressBar returned empty string")
			}
		})
	}
}

func TestStyleManagerGetStyles(t *testing.T) {
	sm := NewStyleManager()

	styles := sm.GetStyles()
	if styles == nil {
		t.Error("GetStyles returned nil")
	}
}

func TestStyleManagerGetTheme(t *testing.T) {
	sm := NewStyleManager()

	theme := sm.GetTheme()
	if theme == nil {
		t.Error("GetTheme returned nil")
	}
}

func TestStyleManagerSetNerdFonts(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	// Enable Nerd Fonts
	sm.SetNerdFonts(true)
	if !sm.useNerdFonts {
		t.Error("SetNerdFonts(true) should set useNerdFonts to true")
	}

	// Disable Nerd Fonts
	sm.SetNerdFonts(false)
	if sm.useNerdFonts {
		t.Error("SetNerdFonts(false) should set useNerdFonts to false")
	}
}

func TestStyleManagerTreeSelected(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	result := sm.TreeSelected("Item")
	if !strings.Contains(result, "Item") {
		t.Error("TreeSelected should contain the text")
	}
	if !strings.Contains(result, "▶") {
		t.Error("TreeSelected should contain selection indicator")
	}
}

func TestStyleManagerTreeUnselected(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	result := sm.TreeUnselected("Item")
	if !strings.Contains(result, "Item") {
		t.Error("TreeUnselected should contain the text")
	}
}

func TestStyleManagerDetailsSelected(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	result := sm.DetailsSelected("Item")
	if !strings.Contains(result, "Item") {
		t.Error("DetailsSelected should contain the text")
	}
}

func TestStyleManagerDetailsUnselected(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	result := sm.DetailsUnselected("Item")
	if result != "Item" {
		t.Errorf("DetailsUnselected = %q, want %q", result, "Item")
	}
}

func TestRenderGradientLine(t *testing.T) {
	sm := NewStyleManager().(*styleManager)

	tests := []struct {
		name  string
		width int
	}{
		{"zero width", 0},
		{"small width", 10},
		{"normal width", 80},
		{"large width", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sm.renderGradientLine(tt.width)
			if result == "" {
				t.Error("renderGradientLine returned empty string")
			}
		})
	}
}
