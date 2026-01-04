package tui

import (
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewViewManager(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter)

	if vm == nil {
		t.Fatal("NewViewManager returned nil")
	}

	// Should have all default views registered via GetView
	expectedViews := []string{ViewList, ViewTree, ViewDetails, ViewStats, ViewHelp}
	for _, viewName := range expectedViews {
		if vm.GetView(viewName) == nil {
			t.Errorf("ViewManager should have %s view registered", viewName)
		}
	}
}

func TestViewManagerGetCurrentView(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter)

	// Without state, should return default view (list)
	view := vm.GetCurrentView(nil)
	if view == nil {
		t.Fatal("GetCurrentView(nil) returned nil")
	}
	if view.Name() != ViewList {
		t.Errorf("GetCurrentView(nil) = %q, want %q", view.Name(), ViewList)
	}

	// With state, should use state's current view
	state := &State{CurrentView: ViewTree}
	view = vm.GetCurrentView(state)
	if view.Name() != ViewTree {
		t.Errorf("GetCurrentView(state with tree) = %q, want %q", view.Name(), ViewTree)
	}

	// With empty CurrentView in state, should fallback to manager's current view
	state = &State{CurrentView: ""}
	view = vm.GetCurrentView(state)
	if view.Name() != ViewList {
		t.Errorf("GetCurrentView(state with empty) = %q, want %q", view.Name(), ViewList)
	}

	// With invalid view name, should fallback to list
	state = &State{CurrentView: "invalid"}
	view = vm.GetCurrentView(state)
	if view.Name() != ViewList {
		t.Errorf("GetCurrentView(state with invalid) = %q, want %q", view.Name(), ViewList)
	}
}

func TestViewManagerSwitchView(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter)

	// Switch to valid view
	err := vm.SwitchView(ViewTree)
	if err != nil {
		t.Errorf("SwitchView(tree) error = %v", err)
	}

	// Verify switch by getting current view without state
	view := vm.GetCurrentView(nil)
	if view.Name() != ViewTree {
		t.Errorf("After SwitchView(tree), current = %q, want %q", view.Name(), ViewTree)
	}

	// Switch to invalid view
	err = vm.SwitchView("nonexistent")
	if err == nil {
		t.Error("SwitchView(nonexistent) should return error")
	}
}

func TestViewManagerGetView(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter)

	// Get existing view
	view := vm.GetView(ViewDetails)
	if view == nil {
		t.Error("GetView(details) returned nil")
	}
	if view.Name() != ViewDetails {
		t.Errorf("GetView(details).Name() = %q, want %q", view.Name(), ViewDetails)
	}

	// Get non-existing view
	view = vm.GetView("nonexistent")
	if view != nil {
		t.Error("GetView(nonexistent) should return nil")
	}
}

func TestViewManagerRegisterView(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter)

	// Create a custom mock view
	customView := &mockView{name: "custom"}

	vm.RegisterView(customView)

	// Check via GetView
	if vm.GetView("custom") == nil {
		t.Error("RegisterView should register the custom view")
	}

	// Registering nil should not panic
	vm.RegisterView(nil)
}

func TestViewManagerGetAllViews(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter)

	views := vm.GetAllViews()

	if len(views) != 5 {
		t.Errorf("GetAllViews() returned %d views, want 5", len(views))
	}

	// Verify it's a copy (modifying shouldn't affect manager)
	delete(views, ViewList)
	if vm.GetView(ViewList) == nil {
		t.Error("GetAllViews should return a copy, not the original map")
	}
}

func TestViewManagerHasView(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter).(*viewManager)

	tests := []struct {
		viewName string
		expected bool
	}{
		{ViewList, true},
		{ViewTree, true},
		{ViewDetails, true},
		{ViewStats, true},
		{ViewHelp, true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.viewName, func(t *testing.T) {
			if got := vm.HasView(tt.viewName); got != tt.expected {
				t.Errorf("HasView(%q) = %v, want %v", tt.viewName, got, tt.expected)
			}
		})
	}
}

func TestViewManagerGetCurrentViewName(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter).(*viewManager)

	// Default should be list
	if name := vm.GetCurrentViewName(); name != ViewList {
		t.Errorf("GetCurrentViewName() = %q, want %q", name, ViewList)
	}

	// After switch
	_ = vm.SwitchView(ViewStats)
	if name := vm.GetCurrentViewName(); name != ViewStats {
		t.Errorf("After switch, GetCurrentViewName() = %q, want %q", name, ViewStats)
	}
}

func TestViewManagerConcurrency(t *testing.T) {
	styles := NewStyleManager()
	filter := NewFilterManager()
	vm := NewViewManager(styles, filter).(*viewManager)

	var wg sync.WaitGroup
	views := []string{ViewList, ViewTree, ViewDetails, ViewStats, ViewHelp}

	// Concurrent reads and writes
	for i := 0; i < 100; i++ {
		wg.Add(4)

		go func(idx int) {
			defer wg.Done()
			_ = vm.SwitchView(views[idx%len(views)])
		}(i)

		go func() {
			defer wg.Done()
			_ = vm.GetCurrentView(nil)
		}()

		go func() {
			defer wg.Done()
			_ = vm.GetAllViews()
		}()

		go func() {
			defer wg.Done()
			_ = vm.HasView(ViewList)
		}()
	}

	wg.Wait()
	// Test passes if no race condition detected
}

// mockView is a simple mock for testing view registration.
type mockView struct {
	name string
}

func (m *mockView) Name() string                                   { return m.name }
func (m *mockView) Render(state *State) string                     { return "" }
func (m *mockView) Update(msg tea.Msg, state *State) (*State, tea.Cmd) { return state, nil }
func (m *mockView) CanHandle(msg tea.Msg, state *State) bool       { return false }
