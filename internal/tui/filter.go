package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// filterManager implements the FilterManager interface.
type filterManager struct {
	input    textinput.Model
	active   bool
	lastText string
}

// NewFilterManager creates a new FilterManager instance.
func NewFilterManager() FilterManager {
	input := textinput.New()
	input.Placeholder = "Search workflows, activities, signals..."
	input.CharLimit = 100
	input.Width = 50
	input.Prompt = ""

	return &filterManager{
		input:  input,
		active: false,
	}
}

// ApplyFilter applies the given filter to the items.
func (fm *filterManager) ApplyFilter(items []list.Item, filter string) []list.Item {
	if filter == "" {
		return items
	}

	filter = strings.ToLower(filter)
	var filtered []list.Item

	for _, item := range items {
		if li, ok := item.(ListItem); ok {
			// Check name
			if strings.Contains(strings.ToLower(li.Node.Name), filter) {
				filtered = append(filtered, item)
				continue
			}

			// Check package
			if strings.Contains(strings.ToLower(li.Node.Package), filter) {
				filtered = append(filtered, item)
				continue
			}

			// Check file path
			if strings.Contains(strings.ToLower(li.Node.FilePath), filter) {
				filtered = append(filtered, item)
				continue
			}

			// Check type
			if strings.Contains(strings.ToLower(li.Node.Type), filter) {
				filtered = append(filtered, item)
				continue
			}

			// Check description
			if strings.Contains(strings.ToLower(li.Node.Description), filter) {
				filtered = append(filtered, item)
				continue
			}
		}
	}

	return filtered
}

// IsActive returns true if filtering is currently active.
func (fm *filterManager) IsActive() bool {
	return fm.active
}

// GetFilter returns the current filter input.
func (fm *filterManager) GetFilter() textinput.Model {
	return fm.input
}

// SetActive sets the filter active state.
func (fm *filterManager) SetActive(active bool) {
	fm.active = active
	if active {
		fm.input.Focus()
	} else {
		fm.input.Blur()
	}
}

// UpdateInput updates the filter input model and returns a command.
func (fm *filterManager) UpdateInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	fm.input, cmd = fm.input.Update(msg)
	fm.lastText = fm.input.Value()
	return cmd
}

// ClearFilter clears the current filter.
func (fm *filterManager) ClearFilter() {
	fm.input.SetValue("")
	fm.lastText = ""
	fm.active = false
	fm.input.Blur()
}

// GetFilterText returns the current filter text.
func (fm *filterManager) GetFilterText() string {
	return fm.input.Value()
}

// SetFilterText sets the filter text.
func (fm *filterManager) SetFilterText(text string) {
	fm.input.SetValue(text)
	fm.lastText = text
}

// Reset resets the filter to its initial state.
func (fm *filterManager) Reset() {
	fm.input.SetValue("")
	fm.lastText = ""
	fm.active = false
	fm.input.Blur()
}

// FuzzyMatch performs a simple fuzzy matching against a string.
func FuzzyMatch(s, pattern string) bool {
	s = strings.ToLower(s)
	pattern = strings.ToLower(pattern)

	if pattern == "" {
		return true
	}

	// Simple substring match
	if strings.Contains(s, pattern) {
		return true
	}

	// Simple fuzzy match - all characters in pattern appear in order in s
	patternIdx := 0
	for _, c := range s {
		if patternIdx < len(pattern) && byte(c) == pattern[patternIdx] {
			patternIdx++
		}
	}

	return patternIdx == len(pattern)
}

// HighlightMatches wraps matching parts of text with highlighting.
func HighlightMatches(text, pattern string, highlightFn func(string) string) string {
	if pattern == "" {
		return text
	}

	lowerText := strings.ToLower(text)
	lowerPattern := strings.ToLower(pattern)

	idx := strings.Index(lowerText, lowerPattern)
	if idx == -1 {
		return text
	}

	// Highlight the matching part
	before := text[:idx]
	match := text[idx : idx+len(pattern)]
	after := text[idx+len(pattern):]

	return before + highlightFn(match) + after
}
