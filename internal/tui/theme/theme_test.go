package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDefaultTheme(t *testing.T) {
	theme := DefaultTheme()
	if theme == nil {
		t.Fatal("DefaultTheme returned nil")
	}

	// Verify all color fields are set
	tests := []struct {
		name  string
		color lipgloss.Color
	}{
		{"Base", theme.Base},
		{"Surface", theme.Surface},
		{"Overlay", theme.Overlay},
		{"Muted", theme.Muted},
		{"Subtle", theme.Subtle},
		{"Text", theme.Text},
		{"Primary", theme.Primary},
		{"Secondary", theme.Secondary},
		{"Tertiary", theme.Tertiary},
		{"Success", theme.Success},
		{"Warning", theme.Warning},
		{"Error", theme.Error},
		{"Info", theme.Info},
		{"Workflow", theme.Workflow},
		{"Activity", theme.Activity},
		{"Signal", theme.Signal},
		{"Query", theme.Query},
		{"Update", theme.Update},
		{"Timer", theme.Timer},
		{"Border", theme.Border},
		{"Selection", theme.Selection},
		{"Highlight", theme.Highlight},
		{"GradientStart", theme.GradientStart},
		{"GradientEnd", theme.GradientEnd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color == "" {
				t.Errorf("Theme.%s is empty", tt.name)
			}
		})
	}
}

func TestNeonTheme(t *testing.T) {
	theme := NeonTheme()
	if theme == nil {
		t.Fatal("NeonTheme returned nil")
	}

	// Verify all color fields are set
	tests := []struct {
		name  string
		color lipgloss.Color
	}{
		{"Base", theme.Base},
		{"Surface", theme.Surface},
		{"Overlay", theme.Overlay},
		{"Muted", theme.Muted},
		{"Subtle", theme.Subtle},
		{"Text", theme.Text},
		{"Primary", theme.Primary},
		{"Secondary", theme.Secondary},
		{"Tertiary", theme.Tertiary},
		{"Success", theme.Success},
		{"Warning", theme.Warning},
		{"Error", theme.Error},
		{"Info", theme.Info},
		{"Workflow", theme.Workflow},
		{"Activity", theme.Activity},
		{"Signal", theme.Signal},
		{"Query", theme.Query},
		{"Update", theme.Update},
		{"Timer", theme.Timer},
		{"Border", theme.Border},
		{"Selection", theme.Selection},
		{"Highlight", theme.Highlight},
		{"GradientStart", theme.GradientStart},
		{"GradientEnd", theme.GradientEnd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color == "" {
				t.Errorf("Theme.%s is empty", tt.name)
			}
		})
	}
}

func TestThemesDiffer(t *testing.T) {
	defaultTheme := DefaultTheme()
	neonTheme := NeonTheme()

	// At least some colors should differ between themes
	sameCount := 0
	if defaultTheme.Primary == neonTheme.Primary {
		sameCount++
	}
	if defaultTheme.Secondary == neonTheme.Secondary {
		sameCount++
	}
	if defaultTheme.Workflow == neonTheme.Workflow {
		sameCount++
	}
	if defaultTheme.Activity == neonTheme.Activity {
		sameCount++
	}

	// At least some should be different
	if sameCount == 4 {
		t.Error("Themes should have some different colors")
	}
}

func TestNewStylesWithDefaultTheme(t *testing.T) {
	theme := DefaultTheme()
	styles := NewStyles(theme)

	if styles == nil {
		t.Fatal("NewStyles returned nil")
	}

	// Verify the theme is stored
	if styles.GetTheme() != theme {
		t.Error("GetTheme should return the same theme")
	}
}

func TestNewStylesWithNilTheme(t *testing.T) {
	// Should use default theme when nil is passed
	styles := NewStyles(nil)

	if styles == nil {
		t.Fatal("NewStyles returned nil")
	}

	if styles.GetTheme() == nil {
		t.Error("GetTheme should return default theme when initialized with nil")
	}
}

func TestStylesHasAllStyles(t *testing.T) {
	styles := NewStyles(DefaultTheme())

	// Test that styles are initialized (by checking they can render without panic)
	testCases := []struct {
		name  string
		style lipgloss.Style
	}{
		{"App", styles.App},
		{"Header", styles.Header},
		{"Footer", styles.Footer},
		{"Content", styles.Content},
		{"Sidebar", styles.Sidebar},
		{"Title", styles.Title},
		{"Subtitle", styles.Subtitle},
		{"Label", styles.Label},
		{"Value", styles.Value},
		{"ListItem", styles.ListItem},
		{"ListItemSelected", styles.ListItemSelected},
		{"ListItemActive", styles.ListItemActive},
		{"WorkflowBadge", styles.WorkflowBadge},
		{"ActivityBadge", styles.ActivityBadge},
		{"SignalBadge", styles.SignalBadge},
		{"QueryBadge", styles.QueryBadge},
		{"UpdateBadge", styles.UpdateBadge},
		{"TimerBadge", styles.TimerBadge},
		{"Success", styles.Success},
		{"Warning", styles.Warning},
		{"Error", styles.Error},
		{"Info", styles.Info},
		{"Muted", styles.Muted},
		{"Breadcrumb", styles.Breadcrumb},
		{"KeyBinding", styles.KeyBinding},
		{"KeyLabel", styles.KeyLabel},
		{"Divider", styles.Divider},
		{"Box", styles.Box},
		{"InsetBox", styles.InsetBox},
		{"TreeBranch", styles.TreeBranch},
		{"TreeLeaf", styles.TreeLeaf},
		{"TreeExpanded", styles.TreeExpanded},
		{"TreeCollapsed", styles.TreeCollapsed},
		{"DetailSection", styles.DetailSection},
		{"DetailLabel", styles.DetailLabel},
		{"DetailValue", styles.DetailValue},
		{"CodeBlock", styles.CodeBlock},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This should not panic
			result := tc.style.Render("test")
			if result == "" {
				t.Errorf("Style %s rendered empty string", tc.name)
			}
		})
	}
}

func TestNodeIconWithNerdFonts(t *testing.T) {
	tests := []struct {
		nodeType string
		expected string
	}{
		{"workflow", Icons.Workflow},
		{"activity", Icons.Activity},
		{"signal", Icons.Signal},
		{"signal_handler", Icons.Signal},
		{"query", Icons.Query},
		{"query_handler", Icons.Query},
		{"update", Icons.Update},
		{"update_handler", Icons.Update},
		{"timer", Icons.Timer},
		{"unknown", Icons.Workflow}, // defaults to workflow
	}

	for _, tt := range tests {
		t.Run(tt.nodeType+"_nerd", func(t *testing.T) {
			result := NodeIcon(tt.nodeType, true)
			if result != tt.expected {
				t.Errorf("NodeIcon(%q, true) = %q, want %q", tt.nodeType, result, tt.expected)
			}
		})
	}
}

func TestNodeIconWithoutNerdFonts(t *testing.T) {
	tests := []struct {
		nodeType string
		expected string
	}{
		{"workflow", FallbackIcons.Workflow},
		{"activity", FallbackIcons.Activity},
		{"signal", FallbackIcons.Signal},
		{"signal_handler", FallbackIcons.Signal},
		{"query", FallbackIcons.Query},
		{"query_handler", FallbackIcons.Query},
		{"update", FallbackIcons.Update},
		{"update_handler", FallbackIcons.Update},
		{"timer", FallbackIcons.Timer},
		{"unknown", FallbackIcons.Workflow}, // defaults to workflow
	}

	for _, tt := range tests {
		t.Run(tt.nodeType+"_fallback", func(t *testing.T) {
			result := NodeIcon(tt.nodeType, false)
			if result != tt.expected {
				t.Errorf("NodeIcon(%q, false) = %q, want %q", tt.nodeType, result, tt.expected)
			}
		})
	}
}

func TestThemeNodeColor(t *testing.T) {
	theme := DefaultTheme()

	tests := []struct {
		nodeType string
		expected lipgloss.Color
	}{
		{"workflow", theme.Workflow},
		{"activity", theme.Activity},
		{"signal", theme.Signal},
		{"signal_handler", theme.Signal},
		{"query", theme.Query},
		{"query_handler", theme.Query},
		{"update", theme.Update},
		{"update_handler", theme.Update},
		{"timer", theme.Timer},
		{"unknown", theme.Primary}, // defaults to primary
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			result := theme.NodeColor(tt.nodeType)
			if result != tt.expected {
				t.Errorf("NodeColor(%q) = %q, want %q", tt.nodeType, result, tt.expected)
			}
		})
	}
}

func TestIconsAreDefined(t *testing.T) {
	// Verify all icon fields are defined and non-empty
	tests := []struct {
		name string
		icon string
	}{
		{"Workflow", Icons.Workflow},
		{"Activity", Icons.Activity},
		{"Signal", Icons.Signal},
		{"Query", Icons.Query},
		{"Update", Icons.Update},
		{"Timer", Icons.Timer},
		{"Package", Icons.Package},
		{"File", Icons.File},
		{"Line", Icons.Line},
		{"Arrow", Icons.Arrow},
		{"ArrowRight", Icons.ArrowRight},
		{"ArrowLeft", Icons.ArrowLeft},
		{"ArrowDown", Icons.ArrowDown},
		{"TreeBranch", Icons.TreeBranch},
		{"TreeLeaf", Icons.TreeLeaf},
		{"TreeExpand", Icons.TreeExpand},
		{"TreeCollapse", Icons.TreeCollapse},
		{"Check", Icons.Check},
		{"Cross", Icons.Cross},
		{"Warning", Icons.Warning},
		{"Info", Icons.Info},
		{"Search", Icons.Search},
		{"Filter", Icons.Filter},
		{"Stats", Icons.Stats},
		{"Help", Icons.Help},
		{"Settings", Icons.Settings},
		{"Refresh", Icons.Refresh},
		{"Exit", Icons.Exit},
		{"Back", Icons.Back},
		{"Connection", Icons.Connection},
		{"Depth", Icons.Depth},
		{"Clock", Icons.Clock},
		{"Play", Icons.Play},
		{"Pause", Icons.Pause},
		{"Stop", Icons.Stop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.icon == "" {
				t.Errorf("Icons.%s is empty", tt.name)
			}
		})
	}
}

func TestFallbackIconsAreDefined(t *testing.T) {
	// Verify all fallback icon fields are defined and non-empty
	tests := []struct {
		name string
		icon string
	}{
		{"Workflow", FallbackIcons.Workflow},
		{"Activity", FallbackIcons.Activity},
		{"Signal", FallbackIcons.Signal},
		{"Query", FallbackIcons.Query},
		{"Update", FallbackIcons.Update},
		{"Timer", FallbackIcons.Timer},
		{"Package", FallbackIcons.Package},
		{"File", FallbackIcons.File},
		{"Line", FallbackIcons.Line},
		{"Arrow", FallbackIcons.Arrow},
		{"ArrowRight", FallbackIcons.ArrowRight},
		{"ArrowLeft", FallbackIcons.ArrowLeft},
		{"TreeBranch", FallbackIcons.TreeBranch},
		{"TreeLeaf", FallbackIcons.TreeLeaf},
		{"TreeExpand", FallbackIcons.TreeExpand},
		{"TreeCollapse", FallbackIcons.TreeCollapse},
		{"Check", FallbackIcons.Check},
		{"Cross", FallbackIcons.Cross},
		{"Warning", FallbackIcons.Warning},
		{"Info", FallbackIcons.Info},
		{"Search", FallbackIcons.Search},
		{"Filter", FallbackIcons.Filter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.icon == "" {
				t.Errorf("FallbackIcons.%s is empty", tt.name)
			}
		})
	}
}

func TestStylesRenderWithContent(t *testing.T) {
	styles := NewStyles(DefaultTheme())
	testText := "Hello, World!"

	// Test that various styles can render actual content
	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"WorkflowBadge", styles.WorkflowBadge},
		{"ActivityBadge", styles.ActivityBadge},
		{"SignalBadge", styles.SignalBadge},
		{"Success", styles.Success},
		{"Error", styles.Error},
		{"Box", styles.Box},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.style.Render(testText)
			// Result should contain the original text (after styling)
			// We can't check exact output due to ANSI codes, but it should be non-empty
			if len(result) < len(testText) {
				t.Errorf("Rendered content shorter than input")
			}
		})
	}
}

func TestGetTheme(t *testing.T) {
	theme := DefaultTheme()
	styles := NewStyles(theme)

	retrieved := styles.GetTheme()
	if retrieved != theme {
		t.Error("GetTheme should return the same theme instance")
	}

	// Test with neon theme
	neonTheme := NeonTheme()
	neonStyles := NewStyles(neonTheme)

	if neonStyles.GetTheme() != neonTheme {
		t.Error("GetTheme should return the neon theme instance")
	}
}

func TestStylesWithDifferentThemes(t *testing.T) {
	defaultStyles := NewStyles(DefaultTheme())
	neonStyles := NewStyles(NeonTheme())

	// Styles should use their respective theme colors
	// We can't easily compare lipgloss styles directly, but we can verify
	// they're both initialized
	if defaultStyles.GetTheme() == neonStyles.GetTheme() {
		t.Error("Different styles should have different themes")
	}
}

