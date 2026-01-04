// Package output provides export functionality for Temporal graphs.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"temporal-analyzer/internal/analyzer"
)

// Exporter provides export functionality for the graph.
type Exporter struct{}

// NewExporter creates a new Exporter instance.
func NewExporter() *Exporter {
	return &Exporter{}
}

// ExportJSON exports the graph as pretty-printed JSON.
func (e *Exporter) ExportJSON(graph *analyzer.TemporalGraph) ([]byte, error) {
	return json.MarshalIndent(graph, "", "  ")
}

// ExportDOT exports the graph as DOT format for Graphviz.
func (e *Exporter) ExportDOT(graph *analyzer.TemporalGraph) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("digraph TemporalGraph {\n")
	buf.WriteString("  // Graph settings\n")
	buf.WriteString("  graph [rankdir=TB, splines=ortho, nodesep=0.8, ranksep=1.0];\n")
	buf.WriteString("  node [shape=box, style=\"rounded,filled\", fontname=\"Helvetica\"];\n")
	buf.WriteString("  edge [fontname=\"Helvetica\", fontsize=10];\n\n")

	// Define color schemes for different node types
	buf.WriteString("  // Node type colors\n")

	// Sort nodes for consistent output
	var nodeNames []string
	for name := range graph.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	// Group nodes by type for subgraphs
	workflows := []string{}
	activities := []string{}
	others := []string{}

	for _, name := range nodeNames {
		node := graph.Nodes[name]
		switch node.Type {
		case "workflow":
			workflows = append(workflows, name)
		case "activity":
			activities = append(activities, name)
		default:
			others = append(others, name)
		}
	}

	// Write workflow subgraph
	if len(workflows) > 0 {
		buf.WriteString("  // Workflows\n")
		buf.WriteString("  subgraph cluster_workflows {\n")
		buf.WriteString("    label=\"Workflows\";\n")
		buf.WriteString("    style=dashed;\n")
		buf.WriteString("    color=\"#a371f7\";\n")
		for _, name := range workflows {
			node := graph.Nodes[name]
			buf.WriteString(fmt.Sprintf("    \"%s\" [label=\"%s\\n%s\", fillcolor=\"#a371f7\", fontcolor=\"white\"];\n",
				e.escapeString(name), e.escapeString(name), node.Package))
		}
		buf.WriteString("  }\n\n")
	}

	// Write activity subgraph
	if len(activities) > 0 {
		buf.WriteString("  // Activities\n")
		buf.WriteString("  subgraph cluster_activities {\n")
		buf.WriteString("    label=\"Activities\";\n")
		buf.WriteString("    style=dashed;\n")
		buf.WriteString("    color=\"#7ee787\";\n")
		for _, name := range activities {
			node := graph.Nodes[name]
			buf.WriteString(fmt.Sprintf("    \"%s\" [label=\"%s\\n%s\", fillcolor=\"#7ee787\", fontcolor=\"black\"];\n",
				e.escapeString(name), e.escapeString(name), node.Package))
		}
		buf.WriteString("  }\n\n")
	}

	// Write other nodes
	for _, name := range others {
		node := graph.Nodes[name]
		color := e.getNodeColor(node.Type)
		buf.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", fillcolor=\"%s\"];\n",
			e.escapeString(name), e.escapeString(name), node.Type, color))
	}

	buf.WriteString("\n  // Edges\n")

	// Write edges
	for _, name := range nodeNames {
		node := graph.Nodes[name]
		for _, call := range node.CallSites {
			edgeStyle := e.getEdgeStyle(call.CallType)
			buf.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [%s];\n",
				e.escapeString(name), e.escapeString(call.TargetName), edgeStyle))
		}
	}

	buf.WriteString("}\n")
	return buf.String(), nil
}

// ExportMermaid exports the graph as Mermaid diagram format.
func (e *Exporter) ExportMermaid(graph *analyzer.TemporalGraph) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("```mermaid\nflowchart TB\n")

	// Sort nodes for consistent output
	var nodeNames []string
	for name := range graph.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	// Define node styles
	buf.WriteString("\n    %% Node definitions\n")

	for _, name := range nodeNames {
		node := graph.Nodes[name]
		nodeID := e.toMermaidID(name)

		switch node.Type {
		case "workflow":
			buf.WriteString(fmt.Sprintf("    %s[\"⚡ %s\"]\n", nodeID, name))
		case "activity":
			buf.WriteString(fmt.Sprintf("    %s([\"⚙ %s\"])\n", nodeID, name))
		case "signal", "signal_handler":
			buf.WriteString(fmt.Sprintf("    %s{{\"🔔 %s\"}}\n", nodeID, name))
		case "query", "query_handler":
			buf.WriteString(fmt.Sprintf("    %s>\"❓ %s\"]\n", nodeID, name))
		default:
			buf.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, name))
		}
	}

	buf.WriteString("\n    %% Connections\n")

	// Write edges
	for _, name := range nodeNames {
		node := graph.Nodes[name]
		fromID := e.toMermaidID(name)

		for _, call := range node.CallSites {
			toID := e.toMermaidID(call.TargetName)
			
			switch call.CallType {
			case "activity":
				buf.WriteString(fmt.Sprintf("    %s -->|execute| %s\n", fromID, toID))
			case "child_workflow":
				buf.WriteString(fmt.Sprintf("    %s ==>|child| %s\n", fromID, toID))
			case "signal":
				buf.WriteString(fmt.Sprintf("    %s -.->|signal| %s\n", fromID, toID))
			default:
				buf.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
			}
		}
	}

	// Add styling
	buf.WriteString("\n    %% Styles\n")
	buf.WriteString("    classDef workflow fill:#a371f7,stroke:#8b5cf6,color:#fff\n")
	buf.WriteString("    classDef activity fill:#7ee787,stroke:#22c55e,color:#000\n")
	buf.WriteString("    classDef signal fill:#ffa657,stroke:#f97316,color:#000\n")
	buf.WriteString("    classDef query fill:#79c0ff,stroke:#3b82f6,color:#000\n")

	// Apply styles
	workflows := []string{}
	activities := []string{}
	signals := []string{}
	queries := []string{}

	for _, name := range nodeNames {
		node := graph.Nodes[name]
		nodeID := e.toMermaidID(name)

		switch node.Type {
		case "workflow":
			workflows = append(workflows, nodeID)
		case "activity":
			activities = append(activities, nodeID)
		case "signal", "signal_handler":
			signals = append(signals, nodeID)
		case "query", "query_handler":
			queries = append(queries, nodeID)
		}
	}

	if len(workflows) > 0 {
		buf.WriteString(fmt.Sprintf("    class %s workflow\n", strings.Join(workflows, ",")))
	}
	if len(activities) > 0 {
		buf.WriteString(fmt.Sprintf("    class %s activity\n", strings.Join(activities, ",")))
	}
	if len(signals) > 0 {
		buf.WriteString(fmt.Sprintf("    class %s signal\n", strings.Join(signals, ",")))
	}
	if len(queries) > 0 {
		buf.WriteString(fmt.Sprintf("    class %s query\n", strings.Join(queries, ",")))
	}

	buf.WriteString("```\n")
	return buf.String(), nil
}

// ExportMarkdown exports the graph as Markdown documentation.
func (e *Exporter) ExportMarkdown(graph *analyzer.TemporalGraph) (string, error) {
	var buf bytes.Buffer

	// Title
	buf.WriteString("# Temporal Workflow Analysis\n\n")

	// Statistics
	buf.WriteString("## 📊 Statistics\n\n")
	buf.WriteString("| Metric | Count |\n")
	buf.WriteString("|--------|-------|\n")
	buf.WriteString(fmt.Sprintf("| Workflows | %d |\n", graph.Stats.TotalWorkflows))
	buf.WriteString(fmt.Sprintf("| Activities | %d |\n", graph.Stats.TotalActivities))
	buf.WriteString(fmt.Sprintf("| Signals | %d |\n", graph.Stats.TotalSignals))
	buf.WriteString(fmt.Sprintf("| Queries | %d |\n", graph.Stats.TotalQueries))
	buf.WriteString(fmt.Sprintf("| Updates | %d |\n", graph.Stats.TotalUpdates))
	buf.WriteString(fmt.Sprintf("| Max Depth | %d |\n", graph.Stats.MaxDepth))
	buf.WriteString(fmt.Sprintf("| Orphan Nodes | %d |\n", graph.Stats.OrphanNodes))
	buf.WriteString("\n")

	// Sort nodes
	var nodeNames []string
	for name := range graph.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	// Workflows section
	buf.WriteString("## ⚡ Workflows\n\n")
	for _, name := range nodeNames {
		node := graph.Nodes[name]
		if node.Type != "workflow" {
			continue
		}

		buf.WriteString(fmt.Sprintf("### %s\n\n", name))
		buf.WriteString(fmt.Sprintf("- **Package:** `%s`\n", node.Package))
		buf.WriteString(fmt.Sprintf("- **File:** `%s:%d`\n", node.FilePath, node.LineNumber))

		if node.Description != "" {
			buf.WriteString(fmt.Sprintf("- **Description:** %s\n", node.Description))
		}

		if len(node.CallSites) > 0 {
			buf.WriteString("\n**Calls:**\n")
			for _, call := range node.CallSites {
				buf.WriteString(fmt.Sprintf("- `%s` (%s)\n", call.TargetName, call.TargetType))
			}
		}

		if len(node.Signals) > 0 {
			buf.WriteString("\n**Signals:**\n")
			for _, sig := range node.Signals {
				buf.WriteString(fmt.Sprintf("- 🔔 `%s`\n", sig.Name))
			}
		}

		if len(node.Queries) > 0 {
			buf.WriteString("\n**Queries:**\n")
			for _, q := range node.Queries {
				buf.WriteString(fmt.Sprintf("- ❓ `%s`\n", q.Name))
			}
		}

		buf.WriteString("\n")
	}

	// Activities section
	buf.WriteString("## ⚙️ Activities\n\n")
	for _, name := range nodeNames {
		node := graph.Nodes[name]
		if node.Type != "activity" {
			continue
		}

		buf.WriteString(fmt.Sprintf("### %s\n\n", name))
		buf.WriteString(fmt.Sprintf("- **Package:** `%s`\n", node.Package))
		buf.WriteString(fmt.Sprintf("- **File:** `%s:%d`\n", node.FilePath, node.LineNumber))

		if node.Description != "" {
			buf.WriteString(fmt.Sprintf("- **Description:** %s\n", node.Description))
		}

		if len(node.Parents) > 0 {
			buf.WriteString("\n**Called by:**\n")
			for _, parent := range node.Parents {
				buf.WriteString(fmt.Sprintf("- `%s`\n", parent))
			}
		}

		buf.WriteString("\n")
	}

	// Add Mermaid diagram
	mermaid, _ := e.ExportMermaid(graph)
	buf.WriteString("## 📈 Dependency Graph\n\n")
	buf.WriteString(mermaid)

	return buf.String(), nil
}

// Helper functions

func (e *Exporter) escapeString(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

func (e *Exporter) toMermaidID(name string) string {
	// Convert to valid Mermaid ID (alphanumeric and underscore only)
	result := strings.Builder{}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func (e *Exporter) getNodeColor(nodeType string) string {
	switch nodeType {
	case "workflow":
		return "#a371f7"
	case "activity":
		return "#7ee787"
	case "signal", "signal_handler":
		return "#ffa657"
	case "query", "query_handler":
		return "#79c0ff"
	case "update", "update_handler":
		return "#ff7b72"
	default:
		return "#58a6ff"
	}
}

func (e *Exporter) getEdgeStyle(callType string) string {
	switch callType {
	case "activity":
		return "style=solid, color=\"#7ee787\""
	case "child_workflow":
		return "style=bold, color=\"#a371f7\""
	case "signal":
		return "style=dashed, color=\"#ffa657\""
	case "query":
		return "style=dotted, color=\"#79c0ff\""
	default:
		return "style=solid"
	}
}

