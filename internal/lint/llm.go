package lint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/analyzer"
)

// MCPClient interfaces with MCP servers for code intelligence.
type MCPClient struct {
	serverCmd string
	enabled   bool
}

// NewMCPClient creates a new MCP client if a language server is available.
func NewMCPClient() *MCPClient {
	// Check for common Go language server MCP configurations
	mcpServer := os.Getenv("MCP_GO_SERVER")
	if mcpServer == "" {
		// Try to find gopls
		if _, err := exec.LookPath("gopls"); err == nil {
			mcpServer = "gopls"
		}
	}

	return &MCPClient{
		serverCmd: mcpServer,
		enabled:   mcpServer != "",
	}
}

// IsEnabled returns true if MCP is available.
func (m *MCPClient) IsEnabled() bool {
	return m.enabled
}

// ReadFileRange reads a range of lines from a file.
func (m *MCPClient) ReadFileRange(filePath string, startLine, endLine int) (string, error) {
	// Use simple file reading - MCP integration would use the language server
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > len(lines) {
		return "", fmt.Errorf("start line %d exceeds file length %d", startLine, len(lines))
	}

	// Get lines with some context (5 lines before and after)
	contextStart := startLine - 5
	if contextStart < 1 {
		contextStart = 1
	}
	contextEnd := endLine + 5
	if contextEnd > len(lines) {
		contextEnd = len(lines)
	}

	var result []string
	for i := contextStart - 1; i < contextEnd; i++ {
		lineNum := i + 1
		prefix := "  "
		if lineNum >= startLine && lineNum <= endLine {
			prefix = "> "
		}
		result = append(result, fmt.Sprintf("%s%4d: %s", prefix, lineNum, lines[i]))
	}

	return strings.Join(result, "\n"), nil
}

// GetDefinition gets the definition of a symbol using gopls.
func (m *MCPClient) GetDefinition(filePath string, line, col int) (string, error) {
	if !m.enabled || m.serverCmd != "gopls" {
		return "", fmt.Errorf("gopls not available")
	}

	// Call gopls definition
	cmd := exec.Command("gopls", "definition", fmt.Sprintf("%s:%d:%d", filePath, line, col))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// GetReferences finds all references to a symbol.
func (m *MCPClient) GetReferences(filePath string, line, col int) ([]string, error) {
	if !m.enabled || m.serverCmd != "gopls" {
		return nil, fmt.Errorf("gopls not available")
	}

	cmd := exec.Command("gopls", "references", fmt.Sprintf("%s:%d:%d", filePath, line, col))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

// LLMEnhancer uses OpenAI to improve lint findings and generate context-aware fixes.
type LLMEnhancer struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	enabled    bool
	mcp        *MCPClient
	rootDir    string
}

// LLMConfig holds configuration for the LLM enhancer.
type LLMConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
	RootDir string
}

// DefaultLLMConfig returns default LLM configuration.
func DefaultLLMConfig() *LLMConfig {
	return &LLMConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		Model:   getEnvOrDefault("OPENAI_MODEL", "gpt-4o-mini"),
		Timeout: 30 * time.Second,
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// NewLLMEnhancer creates a new LLM enhancer.
func NewLLMEnhancer(cfg *LLMConfig) *LLMEnhancer {
	if cfg == nil {
		cfg = DefaultLLMConfig()
	}

	rootDir := cfg.RootDir
	if rootDir == "" {
		rootDir, _ = os.Getwd()
	}

	return &LLMEnhancer{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		enabled: cfg.APIKey != "",
		mcp:     NewMCPClient(),
		rootDir: rootDir,
	}
}

// IsEnabled returns true if LLM enhancement is available.
func (e *LLMEnhancer) IsEnabled() bool {
	return e.enabled
}

// ChatMessage represents a message in the chat completion API.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// complete sends a chat completion request to OpenAI.
func (e *LLMEnhancer) complete(ctx context.Context, messages []ChatMessage) (string, error) {
	if !e.enabled {
		return "", fmt.Errorf("LLM enhancer not enabled (missing OPENAI_API_KEY)")
	}

	reqBody := ChatRequest{
		Model:       e.model,
		Messages:    messages,
		Temperature: 0.2,
		MaxTokens:   2000,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// VerificationResult holds the result of LLM verification.
type VerificationResult struct {
	Valid       bool   `json:"valid"`
	Confidence  string `json:"confidence"` // "high", "medium", "low"
	Reason      string `json:"reason"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// VerifyFinding uses LLM to verify if a lint finding is valid given the code context.
func (e *LLMEnhancer) VerifyFinding(ctx context.Context, issue Issue, codeContext string) (*VerificationResult, error) {
	if !e.enabled {
		return nil, fmt.Errorf("LLM enhancer not enabled")
	}

	systemPrompt := `You are a Temporal.io workflow expert reviewing lint findings.
Analyze the code context and determine if the lint finding is valid or a false positive.

IMPORTANT Temporal SDK facts:
- Activities have UNLIMITED retries by default (MaximumAttempts=0 means infinite)
- Child workflows do NOT inherit parent's RetryPolicy - they get server defaults
- RetryPolicy: nil means "use server defaults" (unlimited), NOT "no retries"
- Only MaximumAttempts: 1 actually disables retries

Respond with JSON only:
{
  "valid": true/false,
  "confidence": "high"/"medium"/"low",
  "reason": "explanation",
  "suggestion": "optional improvement suggestion"
}`

	userPrompt := fmt.Sprintf(`Lint Finding:
- Rule: %s (%s)
- Severity: %s
- Message: %s
- Description: %s
- File: %s:%d

Code Context:
%s

Is this finding valid? Consider:
1. Is this a real issue or false positive?
2. Does the code context show intentional behavior?
3. Are there project-specific patterns that justify this code?`,
		issue.RuleID, issue.RuleName,
		issue.Severity,
		issue.Message,
		issue.Description,
		issue.FilePath, issue.LineNumber,
		codeContext)

	response, err := e.complete(ctx, []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result VerificationResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w (response: %s)", err, response)
	}

	return &result, nil
}

// EnhancedFix holds an LLM-enhanced code fix.
type EnhancedFix struct {
	Code        string `json:"code"`
	Explanation string `json:"explanation"`
	Imports     string `json:"imports,omitempty"`
}

// EnhanceFix generates a context-aware code fix that matches the project's style.
func (e *LLMEnhancer) EnhanceFix(ctx context.Context, issue Issue, codeContext string, existingPatterns string) (*EnhancedFix, error) {
	if !e.enabled {
		return nil, fmt.Errorf("LLM enhancer not enabled")
	}

	systemPrompt := `You are a Temporal.io workflow expert generating code fixes.
Generate a code fix that:
1. Matches the project's existing code style and patterns
2. Uses any existing helper functions found in the codebase
3. Is minimal and focused on fixing the specific issue
4. Is ready to insert (no placeholder comments)

Respond with JSON only:
{
  "code": "the actual Go code to insert",
  "explanation": "brief explanation of the fix",
  "imports": "any new imports needed (optional)"
}`

	userPrompt := fmt.Sprintf(`Fix needed for:
- Rule: %s (%s)
- Message: %s
- Current suggestion: %s
- File: %s:%d

Code Context (surrounding code):
%s

Existing patterns in this codebase:
%s

Generate a fix that matches this project's style. Use existing helpers if available.`,
		issue.RuleID, issue.RuleName,
		issue.Message,
		issue.Suggestion,
		issue.FilePath, issue.LineNumber,
		codeContext,
		existingPatterns)

	response, err := e.complete(ctx, []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result EnhancedFix
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w (response: %s)", err, response)
	}

	return &result, nil
}

// PatternExtractor extracts common patterns from the codebase for context.
type PatternExtractor struct{}

// ExtractPatterns finds relevant patterns from the analyzed graph.
func (p *PatternExtractor) ExtractPatterns(graph *analyzer.TemporalGraph) string {
	var patterns []string

	// Look for retry policy patterns
	retryPatterns := make(map[string]int)
	timeoutPatterns := make(map[string]int)

	for _, node := range graph.Nodes {
		for _, callSite := range node.CallSites {
			if callSite.ParsedActivityOpts != nil {
				opts := callSite.ParsedActivityOpts
				if opts.RetryPolicy != nil {
					if opts.RetryPolicy.MaximumAttempts > 0 {
						pattern := fmt.Sprintf("MaximumAttempts: %d", opts.RetryPolicy.MaximumAttempts)
						retryPatterns[pattern]++
					}
					if opts.RetryPolicy.InitialInterval != "" {
						pattern := fmt.Sprintf("InitialInterval: %s", opts.RetryPolicy.InitialInterval)
						retryPatterns[pattern]++
					}
				}
				if opts.StartToCloseTimeout != "" {
					timeoutPatterns[opts.StartToCloseTimeout]++
				}
				if opts.HeartbeatTimeout != "" {
					timeoutPatterns["HeartbeatTimeout: "+opts.HeartbeatTimeout]++
				}
			}
		}
	}

	if len(retryPatterns) > 0 {
		patterns = append(patterns, "Retry patterns found:")
		for pattern, count := range retryPatterns {
			patterns = append(patterns, fmt.Sprintf("  - %s (used %d times)", pattern, count))
		}
	}

	if len(timeoutPatterns) > 0 {
		patterns = append(patterns, "Timeout patterns found:")
		for pattern, count := range timeoutPatterns {
			patterns = append(patterns, fmt.Sprintf("  - %s (used %d times)", pattern, count))
		}
	}

	// Look for helper functions
	for name := range graph.Nodes {
		nameLower := strings.ToLower(name)
		if strings.Contains(nameLower, "retry") || strings.Contains(nameLower, "disabled") {
			patterns = append(patterns, fmt.Sprintf("Helper found: %s", name))
		}
	}

	if len(patterns) == 0 {
		return "No existing patterns detected."
	}

	return strings.Join(patterns, "\n")
}

// EnhanceIssues runs LLM enhancement on a list of issues.
func (e *LLMEnhancer) EnhanceIssues(ctx context.Context, issues []Issue, graph *analyzer.TemporalGraph, verify bool, enhance bool) ([]Issue, []Issue) {
	if !e.enabled {
		return issues, nil
	}

	extractor := &PatternExtractor{}
	patterns := extractor.ExtractPatterns(graph)

	var validIssues []Issue
	var filteredIssues []Issue

	for _, issue := range issues {
		select {
		case <-ctx.Done():
			return append(validIssues, issues...), filteredIssues
		default:
		}

		// Get code context for this issue
		codeContext := e.getCodeContext(graph, issue)

		// Verify finding if requested
		if verify {
			result, err := e.VerifyFinding(ctx, issue, codeContext)
			if err == nil && !result.Valid && result.Confidence == "high" {
				issue.Description = fmt.Sprintf("[LLM filtered: %s] %s", result.Reason, issue.Description)
				filteredIssues = append(filteredIssues, issue)
				continue
			}
			if err == nil && result.Suggestion != "" {
				issue.Suggestion = result.Suggestion
			}
		}

		// Enhance fix if requested
		if enhance && issue.Fix != nil {
			enhanced, err := e.EnhanceFix(ctx, issue, codeContext, patterns)
			if err == nil && enhanced.Code != "" {
				issue.Fix = &CodeFix{
					Description:  enhanced.Explanation,
					Replacements: []Replacement{{
						FilePath:  issue.FilePath,
						StartLine: issue.LineNumber,
						NewText:   enhanced.Code,
					}},
				}
			}
		}

		validIssues = append(validIssues, issue)
	}

	return validIssues, filteredIssues
}

// getCodeContext extracts surrounding code for an issue using MCP/file reading.
func (e *LLMEnhancer) getCodeContext(graph *analyzer.TemporalGraph, issue Issue) string {
	var ctx []string

	// Try to read actual source code using MCP
	filePath := issue.FilePath
	if !strings.HasPrefix(filePath, "/") {
		filePath = e.rootDir + "/" + filePath
	}

	// Read the actual source code around the issue
	if e.mcp != nil {
		sourceCode, err := e.mcp.ReadFileRange(filePath, issue.LineNumber, issue.LineNumber)
		if err == nil && sourceCode != "" {
			ctx = append(ctx, "Source code:")
			ctx = append(ctx, sourceCode)
			ctx = append(ctx, "")
		}
	}

	// Add metadata from the graph
	for _, node := range graph.Nodes {
		if node.Name == issue.NodeName || strings.HasSuffix(node.Name, "."+issue.NodeName) {
			ctx = append(ctx, fmt.Sprintf("Node: %s (%s)", node.Name, node.Type))
			ctx = append(ctx, fmt.Sprintf("File: %s:%d", node.FilePath, node.LineNumber))

			if len(node.CallSites) > 0 {
				ctx = append(ctx, fmt.Sprintf("Calls %d activities/workflows:", len(node.CallSites)))
				for _, cs := range node.CallSites {
					if cs.ParsedActivityOpts != nil {
						opts := cs.ParsedActivityOpts
						var optDetails []string
						if opts.StartToCloseTimeout != "" {
							optDetails = append(optDetails, "timeout="+opts.StartToCloseTimeout)
						}
						if opts.HeartbeatTimeout != "" {
							optDetails = append(optDetails, "heartbeat="+opts.HeartbeatTimeout)
						}
						if opts.RetryPolicy != nil {
							if opts.RetryPolicy.MaximumAttempts > 0 {
								optDetails = append(optDetails, fmt.Sprintf("maxAttempts=%d", opts.RetryPolicy.MaximumAttempts))
							} else {
								optDetails = append(optDetails, "maxAttempts=unlimited")
							}
						}
						ctx = append(ctx, fmt.Sprintf("  - %s (%s)", cs.TargetName, strings.Join(optDetails, ", ")))
					} else {
						ctx = append(ctx, fmt.Sprintf("  - %s (no options parsed)", cs.TargetName))
					}
				}
			}

			if len(node.Queries) > 0 {
				ctx = append(ctx, fmt.Sprintf("Query handlers: %d", len(node.Queries)))
			}

			if len(node.Signals) > 0 {
				ctx = append(ctx, fmt.Sprintf("Signal handlers: %d", len(node.Signals)))
			}

			break
		}
	}

	if len(ctx) == 0 {
		return fmt.Sprintf("Issue at %s:%d for %s", issue.FilePath, issue.LineNumber, issue.NodeName)
	}

	return strings.Join(ctx, "\n")
}
