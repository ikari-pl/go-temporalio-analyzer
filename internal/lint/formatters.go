package lint

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// Helper functions to suppress errcheck warnings for formatting output.
// These are used for writing to output streams where errors are non-fatal.
func fprintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

func fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}

// Formatter defines the interface for output formatters.
type Formatter interface {
	Format(result *Result, w io.Writer) error
}

// NewFormatter creates a formatter for the given format type.
func NewFormatter(format string) Formatter {
	switch format {
	case "json":
		return &JSONFormatter{}
	case "github":
		return &GitHubFormatter{}
	case "sarif":
		return &SARIFFormatter{}
	case "checkstyle":
		return &CheckstyleFormatter{}
	case "text", "":
		return &TextFormatter{Color: true}
	case "text-no-color":
		return &TextFormatter{Color: false}
	default:
		return &TextFormatter{Color: true}
	}
}

// =============================================================================
// Text Formatter (Human Readable)
// =============================================================================

// TextFormatter outputs human-readable text.
type TextFormatter struct {
	Color bool
}

func (f *TextFormatter) Format(result *Result, w io.Writer) error {
	// ANSI color codes
	red := ""
	yellow := ""
	blue := ""
	reset := ""
	bold := ""
	dim := ""

	if f.Color {
		red = "\033[31m"
		yellow = "\033[33m"
		blue = "\033[34m"
		reset = "\033[0m"
		bold = "\033[1m"
		dim = "\033[2m"
	}

	// Header
	fprintf(w, "\n%s%sTemporal Analyzer - Lint Results%s\n", bold, blue, reset)
	fprintf(w, "%s══════════════════════════════════════════════════════════════════%s\n\n", dim, reset)

	if len(result.Issues) == 0 {
		fprintf(w, "%s✓ No issues found!%s\n\n", bold, reset)
		return nil
	}

	// Group issues by file
	byFile := make(map[string][]Issue)
	noFile := make([]Issue, 0)
	for _, issue := range result.Issues {
		if issue.FilePath != "" {
			byFile[issue.FilePath] = append(byFile[issue.FilePath], issue)
		} else {
			noFile = append(noFile, issue)
		}
	}

	// Print file-grouped issues
	for filePath, issues := range byFile {
		fprintf(w, "%s%s%s\n", bold, filePath, reset)
		for _, issue := range issues {
			severityColor := blue
			severityIcon := "ℹ"
			switch issue.Severity {
			case SeverityError:
				severityColor = red
				severityIcon = "✖"
			case SeverityWarning:
				severityColor = yellow
				severityIcon = "⚠"
			}

			lineInfo := ""
			if issue.LineNumber > 0 {
				lineInfo = fmt.Sprintf("%d:", issue.LineNumber)
			}

			fprintf(w, "  %s%s%s %s%s%s %s%s%s %s\n",
				dim, lineInfo, reset,
				severityColor, severityIcon, reset,
				dim, issue.RuleID, reset,
				issue.Message)

			if issue.Suggestion != "" {
				fprintf(w, "     %s→ %s%s\n", dim, issue.Suggestion, reset)
			}
		}
		fprintln(w)
	}

	// Print non-file issues
	if len(noFile) > 0 {
		fprintf(w, "%sGeneral Issues%s\n", bold, reset)
		for _, issue := range noFile {
			severityColor := blue
			severityIcon := "ℹ"
			switch issue.Severity {
			case SeverityError:
				severityColor = red
				severityIcon = "✖"
			case SeverityWarning:
				severityColor = yellow
				severityIcon = "⚠"
			}

			fprintf(w, "  %s%s%s %s%s%s %s\n",
				severityColor, severityIcon, reset,
				dim, issue.RuleID, reset,
				issue.Message)

			if issue.Suggestion != "" {
				fprintf(w, "     %s→ %s%s\n", dim, issue.Suggestion, reset)
			}
		}
		fprintln(w)
	}

	// Summary
	fprintf(w, "%s──────────────────────────────────────────────────────────────────%s\n", dim, reset)
	summary := []string{}
	if result.ErrorCount > 0 {
		summary = append(summary, fmt.Sprintf("%s%d error(s)%s", red, result.ErrorCount, reset))
	}
	if result.WarnCount > 0 {
		summary = append(summary, fmt.Sprintf("%s%d warning(s)%s", yellow, result.WarnCount, reset))
	}
	if result.InfoCount > 0 {
		summary = append(summary, fmt.Sprintf("%s%d info%s", blue, result.InfoCount, reset))
	}
	fprintf(w, "%s %s\n\n", bold, strings.Join(summary, ", "))

	return nil
}

// =============================================================================
// JSON Formatter
// =============================================================================

// JSONFormatter outputs JSON.
type JSONFormatter struct{}

// JSONOutput is the structure for JSON output.
type JSONOutput struct {
	Version    string   `json:"version"`
	Timestamp  string   `json:"timestamp"`
	TotalNodes int      `json:"totalNodes"`
	Summary    Summary  `json:"summary"`
	Issues     []Issue  `json:"issues"`
	ExitCode   int      `json:"exitCode"`
}

type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
	Total    int `json:"total"`
}

func (f *JSONFormatter) Format(result *Result, w io.Writer) error {
	output := JSONOutput{
		Version:    "1.0",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		TotalNodes: result.TotalNodes,
		Summary: Summary{
			Errors:   result.ErrorCount,
			Warnings: result.WarnCount,
			Info:     result.InfoCount,
			Total:    len(result.Issues),
		},
		Issues:   result.Issues,
		ExitCode: result.ExitCode,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// =============================================================================
// GitHub Actions Formatter
// =============================================================================

// GitHubFormatter outputs GitHub Actions workflow commands.
type GitHubFormatter struct{}

func (f *GitHubFormatter) Format(result *Result, w io.Writer) error {
	// Track which rules we've explained (include description only once per rule)
	explainedRules := make(map[string]bool)

	for _, issue := range result.Issues {
		level := "notice"
		switch issue.Severity {
		case SeverityError:
			level = "error"
		case SeverityWarning:
			level = "warning"
		}

		// GitHub workflow command format:
		// ::error file={name},line={line},title={title}::{message}
		params := []string{}
		if issue.FilePath != "" {
			params = append(params, fmt.Sprintf("file=%s", issue.FilePath))
		}
		if issue.LineNumber > 0 {
			params = append(params, fmt.Sprintf("line=%d", issue.LineNumber))
		}
		params = append(params, fmt.Sprintf("title=%s (%s)", issue.RuleName, issue.RuleID))

		// Build message: include description (the "why") only on first occurrence of each rule
		message := issue.Message
		if !explainedRules[issue.RuleID] && issue.Description != "" {
			message += " Why: " + issue.Description
			explainedRules[issue.RuleID] = true
		}
		if issue.Suggestion != "" {
			message += " Suggestion: " + issue.Suggestion
		}

		// Format: ::{level} {params}::{message}
		if len(params) > 0 {
			fprintf(w, "::%s %s::%s\n", level, strings.Join(params, ","), message)
		} else {
			fprintf(w, "::%s::%s\n", level, message)
		}
	}

	// Summary annotation
	fprintf(w, "::group::Lint Summary\n")
	fprintf(w, "Total: %d issue(s) - %d error(s), %d warning(s), %d info\n",
		len(result.Issues), result.ErrorCount, result.WarnCount, result.InfoCount)
	fprintf(w, "::endgroup::\n")

	return nil
}

// =============================================================================
// SARIF Formatter (Static Analysis Results Interchange Format)
// =============================================================================

// SARIFFormatter outputs SARIF format for Azure DevOps, GitHub Code Scanning, etc.
type SARIFFormatter struct{}

// SARIF structures
type SARIFReport struct {
	Schema  string      `json:"$schema"`
	Version string      `json:"version"`
	Runs    []SARIFRun  `json:"runs"`
}

type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

type SARIFDriver struct {
	Name            string      `json:"name"`
	Version         string      `json:"version"`
	InformationURI  string      `json:"informationUri"`
	Rules           []SARIFRule `json:"rules"`
}

type SARIFRule struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	ShortDescription SARIFMessage         `json:"shortDescription"`
	FullDescription  SARIFMessage         `json:"fullDescription,omitempty"`
	DefaultConfig    SARIFRuleConfig      `json:"defaultConfiguration"`
	Properties       SARIFRuleProperties  `json:"properties,omitempty"`
}

type SARIFRuleConfig struct {
	Level string `json:"level"`
}

type SARIFRuleProperties struct {
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type SARIFResult struct {
	RuleID    string           `json:"ruleId"`
	Level     string           `json:"level"`
	Message   SARIFMessage     `json:"message"`
	Locations []SARIFLocation  `json:"locations,omitempty"`
	Fixes     []SARIFFix       `json:"fixes,omitempty"`
}

type SARIFMessage struct {
	Text string `json:"text"`
}

type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           *SARIFRegion          `json:"region,omitempty"`
}

type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

type SARIFRegion struct {
	StartLine int `json:"startLine"`
	EndLine   int `json:"endLine,omitempty"`
}

// SARIFFix represents a suggested fix for an issue
type SARIFFix struct {
	Description     SARIFMessage              `json:"description"`
	ArtifactChanges []SARIFArtifactChange     `json:"artifactChanges"`
}

type SARIFArtifactChange struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Replacements     []SARIFReplacement    `json:"replacements"`
}

type SARIFReplacement struct {
	DeletedRegion   SARIFRegion       `json:"deletedRegion"`
	InsertedContent SARIFTextContent  `json:"insertedContent"`
}

type SARIFTextContent struct {
	Text string `json:"text"`
}

func (f *SARIFFormatter) Format(result *Result, w io.Writer) error {
	// Build unique rules from issues
	ruleMap := make(map[string]*SARIFRule)
	for _, issue := range result.Issues {
		if _, exists := ruleMap[issue.RuleID]; !exists {
			level := "note"
			switch issue.Severity {
			case SeverityError:
				level = "error"
			case SeverityWarning:
				level = "warning"
			}

			ruleMap[issue.RuleID] = &SARIFRule{
				ID:               issue.RuleID,
				Name:             issue.RuleName,
				ShortDescription: SARIFMessage{Text: issue.Description},
				DefaultConfig:    SARIFRuleConfig{Level: level},
				Properties: SARIFRuleProperties{
					Category: string(issue.Category),
					Tags:     []string{"temporal", string(issue.Category)},
				},
			}
		}
	}

	rules := make([]SARIFRule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rules = append(rules, *rule)
	}

	// Build results
	results := make([]SARIFResult, 0, len(result.Issues))
	for _, issue := range result.Issues {
		level := "note"
		switch issue.Severity {
		case SeverityError:
			level = "error"
		case SeverityWarning:
			level = "warning"
		}

		r := SARIFResult{
			RuleID:  issue.RuleID,
			Level:   level,
			Message: SARIFMessage{Text: issue.Message},
		}

		if issue.FilePath != "" {
			location := SARIFLocation{
				PhysicalLocation: SARIFPhysicalLocation{
					ArtifactLocation: SARIFArtifactLocation{
						URI: filepath.ToSlash(issue.FilePath),
					},
				},
			}
			if issue.LineNumber > 0 {
				location.PhysicalLocation.Region = &SARIFRegion{
					StartLine: issue.LineNumber,
				}
			}
			r.Locations = []SARIFLocation{location}
		}

		// Add fix information if available
		if issue.Fix != nil && len(issue.Fix.Replacements) > 0 {
			sarifFix := SARIFFix{
				Description: SARIFMessage{Text: issue.Fix.Description},
			}
			for _, repl := range issue.Fix.Replacements {
				change := SARIFArtifactChange{
					ArtifactLocation: SARIFArtifactLocation{
						URI: filepath.ToSlash(repl.FilePath),
					},
					Replacements: []SARIFReplacement{{
						DeletedRegion: SARIFRegion{
							StartLine: repl.StartLine,
							EndLine:   repl.StartLine, // Single line replacement
						},
						InsertedContent: SARIFTextContent{
							Text: repl.NewText,
						},
					}},
				}
				sarifFix.ArtifactChanges = append(sarifFix.ArtifactChanges, change)
			}
			r.Fixes = []SARIFFix{sarifFix}
		}

		results = append(results, r)
	}

	report := SARIFReport{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:           "temporal-analyzer",
						Version:        "1.0.0",
						InformationURI: "https://github.com/ikari-pl/go-temporalio-analyzer",
						Rules:          rules,
					},
				},
				Results: results,
			},
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// =============================================================================
// Checkstyle Formatter (XML)
// =============================================================================

// CheckstyleFormatter outputs Checkstyle XML format.
type CheckstyleFormatter struct{}

func (f *CheckstyleFormatter) Format(result *Result, w io.Writer) error {
	fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fprintln(w, `<checkstyle version="4.3">`)

	// Group by file
	byFile := make(map[string][]Issue)
	for _, issue := range result.Issues {
		path := issue.FilePath
		if path == "" {
			path = "general"
		}
		byFile[path] = append(byFile[path], issue)
	}

	for filePath, issues := range byFile {
		fprintf(w, `  <file name="%s">`+"\n", escapeXML(filePath))
		for _, issue := range issues {
			severity := "info"
			switch issue.Severity {
			case SeverityError:
				severity = "error"
			case SeverityWarning:
				severity = "warning"
			}

			line := issue.LineNumber
			if line <= 0 {
				line = 1
			}

			fprintf(w, `    <error line="%d" severity="%s" message="%s" source="%s"/>`+"\n",
				line, severity, escapeXML(issue.Message), escapeXML(issue.RuleID))
		}
		fprintln(w, `  </file>`)
	}

	fprintln(w, `</checkstyle>`)
	return nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

