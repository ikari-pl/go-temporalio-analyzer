package main

import (
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// addToNavPath adds a new navigation step to the path
func (m *model) addToNavPath(node *TemporalNode, direction string) {
	// Limit path length to prevent overflow
	if len(m.navPath) >= 10 {
		m.navPath = m.navPath[1:] // Remove oldest
	}

	shortName := node.Name
	if len(shortName) > 20 {
		shortName = shortName[:17] + "..."
	}

	m.navPath = append(m.navPath, navPathItem{
		node:        node,
		direction:   direction,
		displayName: shortName,
	})
}

// renderNavPath creates a breadcrumb trail string
func (m model) renderNavPath() string {
	if len(m.navPath) == 0 {
		return ""
	}

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")). // Light gray
		Background(lipgloss.Color("236")). // Dark gray background
		PaddingLeft(1).
		PaddingRight(1)

	var pathParts []string
	for i, item := range m.navPath {
		if i == 0 {
			pathParts = append(pathParts, item.displayName)
		} else {
			pathParts = append(pathParts, item.direction+" "+item.displayName)
		}
	}

	return pathStyle.Render("🧭 Path: " + strings.Join(pathParts, " "))
}
