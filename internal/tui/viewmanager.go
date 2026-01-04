package tui

import (
	"fmt"
	"sync"
)

// viewManager implements the ViewManager interface.
type viewManager struct {
	views       map[string]View
	currentView string
	styles      StyleManager
	filter      FilterManager
	mu          sync.RWMutex
}

// NewViewManager creates a new ViewManager instance.
func NewViewManager(styles StyleManager, filter FilterManager) ViewManager {
	vm := &viewManager{
		views:       make(map[string]View),
		currentView: ViewList,
		styles:      styles,
		filter:      filter,
	}

	// Register default views
	vm.RegisterView(NewListView(styles, filter))
	vm.RegisterView(NewTreeView(styles))
	vm.RegisterView(NewDetailsView(styles))
	vm.RegisterView(NewStatsView(styles))
	vm.RegisterView(NewHelpView(styles))

	return vm
}

// GetCurrentView returns the currently active view.
func (vm *viewManager) GetCurrentView(state *State) View {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if state == nil {
		return vm.views[vm.currentView]
	}

	// Use state's current view if available
	viewName := state.CurrentView
	if viewName == "" {
		viewName = vm.currentView
	}

	if view, ok := vm.views[viewName]; ok {
		return view
	}

	// Fallback to list view
	return vm.views[ViewList]
}

// SwitchView switches to the specified view.
func (vm *viewManager) SwitchView(viewName string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if _, ok := vm.views[viewName]; !ok {
		return fmt.Errorf("view '%s' not found", viewName)
	}

	vm.currentView = viewName
	return nil
}

// GetView returns a view by name.
func (vm *viewManager) GetView(viewName string) View {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.views[viewName]
}

// RegisterView registers a new view.
func (vm *viewManager) RegisterView(view View) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if view != nil {
		vm.views[view.Name()] = view
	}
}

// GetAllViews returns all registered views.
func (vm *viewManager) GetAllViews() map[string]View {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	// Return a copy to prevent mutation
	views := make(map[string]View, len(vm.views))
	for k, v := range vm.views {
		views[k] = v
	}
	return views
}

// GetCurrentViewName returns the name of the current view.
func (vm *viewManager) GetCurrentViewName() string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.currentView
}

// HasView returns true if a view with the given name exists.
func (vm *viewManager) HasView(viewName string) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	_, ok := vm.views[viewName]
	return ok
}
