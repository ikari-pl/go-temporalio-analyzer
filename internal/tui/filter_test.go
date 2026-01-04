package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

func TestNewFilterManager(t *testing.T) {
	fm := NewFilterManager()
	if fm == nil {
		t.Fatal("NewFilterManager returned nil")
	}

	// Verify initial state
	if fm.IsActive() {
		t.Error("New filter manager should not be active")
	}
	if fm.GetFilterText() != "" {
		t.Error("New filter manager should have empty filter text")
	}
}

func TestFilterManagerApplyFilter(t *testing.T) {
	fm := NewFilterManager()

	// Create test items
	items := []list.Item{
		ListItem{Node: &analyzer.TemporalNode{Name: "OrderWorkflow", Package: "orders", FilePath: "order.go", Type: "workflow", Description: "Handles order processing"}},
		ListItem{Node: &analyzer.TemporalNode{Name: "PaymentActivity", Package: "payments", FilePath: "payment.go", Type: "activity", Description: "Process payment"}},
		ListItem{Node: &analyzer.TemporalNode{Name: "ShippingActivity", Package: "shipping", FilePath: "shipping.go", Type: "activity", Description: "Ship orders"}},
		ListItem{Node: &analyzer.TemporalNode{Name: "NotifyWorkflow", Package: "notifications", FilePath: "notify.go", Type: "workflow", Description: "Send notifications"}},
	}

	tests := []struct {
		name           string
		filter         string
		expectedCount  int
		expectedNames  []string
		notExpected    []string
	}{
		{
			name:          "empty filter returns all",
			filter:        "",
			expectedCount: 4,
		},
		{
			name:          "filter by name prefix",
			filter:        "OrderWor",
			expectedCount: 1,
			expectedNames: []string{"OrderWorkflow"},
		},
		{
			name:          "filter by name case insensitive",
			filter:        "orderwor",
			expectedCount: 1,
			expectedNames: []string{"OrderWorkflow"},
		},
		{
			name:          "filter by package",
			filter:        "payments",
			expectedCount: 1,
			expectedNames: []string{"PaymentActivity"},
		},
		{
			name:          "filter by file path",
			filter:        "shipping.go",
			expectedCount: 1,
			expectedNames: []string{"ShippingActivity"},
		},
		{
			name:          "filter by type",
			filter:        "workflow",
			expectedCount: 2,
			expectedNames: []string{"OrderWorkflow", "NotifyWorkflow"},
			notExpected:   []string{"PaymentActivity", "ShippingActivity"},
		},
		{
			name:          "filter by description",
			filter:        "payment",
			expectedCount: 1,
			expectedNames: []string{"PaymentActivity"},
		},
		{
			name:          "filter returns multiple matches",
			filter:        "Activity",
			expectedCount: 2,
			expectedNames: []string{"PaymentActivity", "ShippingActivity"},
			notExpected:   []string{"OrderWorkflow", "NotifyWorkflow"},
		},
		{
			name:          "filter no matches",
			filter:        "nonexistent",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fm.ApplyFilter(items, tt.filter)

			if len(result) != tt.expectedCount {
				t.Errorf("ApplyFilter(%q) returned %d items, want %d", tt.filter, len(result), tt.expectedCount)
			}

			// Check expected names are present
			resultNames := make(map[string]bool)
			for _, item := range result {
				if li, ok := item.(ListItem); ok {
					resultNames[li.Node.Name] = true
				}
			}

			for _, expectedName := range tt.expectedNames {
				if !resultNames[expectedName] {
					t.Errorf("ApplyFilter(%q) should include %q", tt.filter, expectedName)
				}
			}

			for _, notExpected := range tt.notExpected {
				if resultNames[notExpected] {
					t.Errorf("ApplyFilter(%q) should not include %q", tt.filter, notExpected)
				}
			}
		})
	}
}

func TestFilterManagerIsActive(t *testing.T) {
	fm := NewFilterManager()

	// Initially not active
	if fm.IsActive() {
		t.Error("New filter manager should not be active")
	}

	// Set active
	fm.SetActive(true)
	if !fm.IsActive() {
		t.Error("Filter manager should be active after SetActive(true)")
	}

	// Set inactive
	fm.SetActive(false)
	if fm.IsActive() {
		t.Error("Filter manager should not be active after SetActive(false)")
	}
}

func TestFilterManagerSetActive(t *testing.T) {
	fm := NewFilterManager()

	// Test toggling
	fm.SetActive(true)
	if !fm.IsActive() {
		t.Error("SetActive(true) should activate filter")
	}

	fm.SetActive(false)
	if fm.IsActive() {
		t.Error("SetActive(false) should deactivate filter")
	}

	// Test setting same state multiple times
	fm.SetActive(true)
	fm.SetActive(true)
	if !fm.IsActive() {
		t.Error("Multiple SetActive(true) should keep filter active")
	}
}

func TestFilterManagerGetFilter(t *testing.T) {
	fm := NewFilterManager()
	filter := fm.GetFilter()

	// Verify it returns a textinput model
	if filter.Placeholder == "" {
		t.Error("Filter should have a placeholder")
	}
}

func TestFilterManagerClearFilter(t *testing.T) {
	fm := NewFilterManager()

	// Set up filter with some state
	fm.SetActive(true)
	fm.SetFilterText("test")

	// Clear filter
	fm.ClearFilter()

	// Verify state is cleared
	if fm.IsActive() {
		t.Error("ClearFilter should deactivate filter")
	}
	if fm.GetFilterText() != "" {
		t.Error("ClearFilter should clear filter text")
	}
}

func TestFilterManagerGetFilterText(t *testing.T) {
	fm := NewFilterManager()

	// Initially empty
	if fm.GetFilterText() != "" {
		t.Error("New filter manager should have empty filter text")
	}

	// After setting text
	fm.SetFilterText("test query")
	if fm.GetFilterText() != "test query" {
		t.Errorf("GetFilterText() = %q, want %q", fm.GetFilterText(), "test query")
	}
}

func TestFilterManagerSetFilterText(t *testing.T) {
	fm := NewFilterManager()

	tests := []string{
		"simple",
		"with spaces",
		"UPPERCASE",
		"MixedCase",
		"special!@#$%",
		"",
	}

	for _, text := range tests {
		fm.SetFilterText(text)
		if got := fm.GetFilterText(); got != text {
			t.Errorf("SetFilterText(%q) => GetFilterText() = %q", text, got)
		}
	}
}

func TestFilterManagerReset(t *testing.T) {
	fm := NewFilterManager()

	// Set up some state
	fm.SetActive(true)
	fm.SetFilterText("test")

	// Reset
	fm.(*filterManager).Reset()

	// Verify state is reset
	if fm.IsActive() {
		t.Error("Reset should deactivate filter")
	}
	if fm.GetFilterText() != "" {
		t.Error("Reset should clear filter text")
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		s        string
		pattern  string
		expected bool
	}{
		// Empty pattern always matches
		{"anything", "", true},
		{"", "", true},

		// Substring matches
		{"OrderWorkflow", "Order", true},
		{"OrderWorkflow", "Workflow", true},
		{"OrderWorkflow", "low", true},
		{"OrderWorkflow", "work", true},

		// Case insensitive
		{"OrderWorkflow", "order", true},
		{"OrderWorkflow", "ORDER", true},
		{"orderworkflow", "ORDER", true},

		// Fuzzy matching (all characters appear in order)
		{"OrderWorkflow", "ow", true},
		{"OrderWorkflow", "OW", true},
		{"OrderWorkflow", "ofl", true},
		{"OrderWorkflow", "owf", true},

		// No match
		{"OrderWorkflow", "xyz", false},
		{"OrderWorkflow", "Payment", false},
		{"Order", "OrderWorkflow", false}, // pattern longer than string

		// Edge cases
		{"", "a", false},
		{"a", "a", true},
		{"ab", "ba", false}, // fuzzy requires order
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.pattern, func(t *testing.T) {
			result := FuzzyMatch(tt.s, tt.pattern)
			if result != tt.expected {
				t.Errorf("FuzzyMatch(%q, %q) = %v, want %v", tt.s, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestHighlightMatches(t *testing.T) {
	highlightFn := func(s string) string {
		return "[" + s + "]"
	}

	tests := []struct {
		text     string
		pattern  string
		expected string
	}{
		// Empty pattern returns original
		{"hello world", "", "hello world"},

		// Simple match
		{"hello world", "world", "hello [world]"},
		{"hello world", "hello", "[hello] world"},

		// Case insensitive match
		{"Hello World", "world", "Hello [World]"},
		{"Hello World", "HELLO", "[Hello] World"},

		// Pattern not found
		{"hello world", "xyz", "hello world"},

		// Match at beginning
		{"abc", "a", "[a]bc"},

		// Match at end
		{"abc", "c", "ab[c]"},

		// Full match
		{"test", "test", "[test]"},
	}

	for _, tt := range tests {
		t.Run(tt.text+"_"+tt.pattern, func(t *testing.T) {
			result := HighlightMatches(tt.text, tt.pattern, highlightFn)
			if result != tt.expected {
				t.Errorf("HighlightMatches(%q, %q) = %q, want %q", tt.text, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestFilterManagerWithNonListItems(t *testing.T) {
	fm := NewFilterManager()

	// This should not panic and should return empty
	items := []list.Item{}
	result := fm.ApplyFilter(items, "test")
	if len(result) != 0 {
		t.Error("ApplyFilter on empty slice should return empty slice")
	}
}

func TestFilterManagerApplyFilterWithPartialMatches(t *testing.T) {
	fm := NewFilterManager()

	items := []list.Item{
		ListItem{Node: &analyzer.TemporalNode{Name: "ProcessOrderWorkflow", Package: "order", FilePath: "process_order.go", Type: "workflow"}},
		ListItem{Node: &analyzer.TemporalNode{Name: "ProcessPaymentWorkflow", Package: "payment", FilePath: "process_payment.go", Type: "workflow"}},
	}

	// Filter by common prefix
	result := fm.ApplyFilter(items, "Process")
	if len(result) != 2 {
		t.Errorf("ApplyFilter('Process') should return 2 items, got %d", len(result))
	}

	// Filter by unique suffix
	result = fm.ApplyFilter(items, "Order")
	if len(result) != 1 {
		t.Errorf("ApplyFilter('Order') should return 1 item, got %d", len(result))
	}
}

