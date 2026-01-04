# Changelog

## [1.0.0] - 2026-01-04

First public release with production-ready features.

### Added
- **CI/CD Lint Mode** - Non-interactive analysis for pipelines
  - Multiple output formats: text, JSON, GitHub Actions, SARIF, Checkstyle
  - Configurable rules with enable/disable options
  - Strict mode to fail on warnings
  - Code fix suggestions in JSON/SARIF output
- **TUI Improvements**
  - Package grouping view with FQN hierarchy
  - Internal call navigation within activities
  - Enhanced signal/query/update handler detection
  - k9s-inspired visual design

### Fixed
- Handler detection for Temporal signals, queries, and updates
- Selection and extraction bugs in AST parsing

## [0.1.0] - 2025-08-02

Initial release.

### Added
- Static analysis of Go codebases for Temporal workflows and activities
- Interactive TUI for exploring workflow graphs
- Export formats: JSON, DOT (Graphviz), Mermaid, Markdown
- Call relationship mapping between workflows and activities
- Statistics: depth, fan-out, orphan detection

