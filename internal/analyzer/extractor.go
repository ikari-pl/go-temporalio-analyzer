package analyzer

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
)

// callExtractor implements the CallExtractor interface.
type callExtractor struct {
	logger *slog.Logger
}

// NewCallExtractor creates a new CallExtractor instance.
func NewCallExtractor(logger *slog.Logger) CallExtractor {
	return &callExtractor{
		logger: logger,
	}
}

// TemporalCallInfo holds detailed information about a Temporal API call.
type TemporalCallInfo struct {
	Type          string // "activity", "child_workflow", "local_activity", "signal", "query", "update", "timer", "version"
	TargetName    string
	LineNumber    int
	FilePath      string
	Options       []string
	SignalDef     *SignalDef
	QueryDef      *QueryDef
	UpdateDef     *UpdateDef
	TimerDef      *TimerDef
	VersionDef    *VersionDef
	SearchAttrDef *SearchAttrDef

	// Signature validation
	ArgumentCount int      // Number of arguments passed (excluding ctx and activity/workflow func)
	ArgumentTypes []string // Types of arguments if determinable
	ResultType    string   // Type used in .Get() call if present

	// Parsed activity/workflow options
	ParsedActivityOpts *ActivityOptions
}

// ExtractCalls finds all temporal workflow and activity calls within a function.
func (e *callExtractor) ExtractCalls(ctx context.Context, fn *ast.FuncDecl, filePath string) ([]CallSite, error) {
	if fn.Body == nil {
		return nil, nil
	}

	var callSites []CallSite
	// Track processed inner calls to avoid duplicates when handling chained .Get() calls
	processedCalls := make(map[*ast.CallExpr]bool)

	// Walk through the function body to find calls
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return false
		default:
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Skip if already processed (inner call of a chained .Get())
		if processedCalls[call] {
			return true
		}

		// Check if this is a .Get() call with a Temporal call as receiver
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if innerCall, isCall := sel.X.(*ast.CallExpr); isCall && sel.Sel.Name == "Get" {
				// Mark inner call as processed to avoid duplicate
				processedCalls[innerCall] = true
			}
		}

		info := e.analyzeCall(call, filePath, nil)
		if info != nil && info.TargetName != "" {
			callSites = append(callSites, CallSite{
				TargetName:         info.TargetName,
				TargetType:         info.Type,
				CallType:           info.Type,
				LineNumber:         info.LineNumber,
				FilePath:           info.FilePath,
				Options:            info.Options,
				ArgumentCount:      info.ArgumentCount,
				ArgumentTypes:      info.ArgumentTypes,
				ResultType:         info.ResultType,
				ParsedActivityOpts: info.ParsedActivityOpts,
			})
		}

		return true
	})

	return callSites, nil
}

// ExtractAllTemporalInfo extracts all Temporal-specific information from a function.
func (e *callExtractor) ExtractAllTemporalInfo(ctx context.Context, fn *ast.FuncDecl, filePath string, fset *token.FileSet) (*TemporalNodeDetails, error) {
	if fn.Body == nil {
		return nil, nil
	}

	details := &TemporalNodeDetails{
		Signals:     []SignalDef{},
		Queries:     []QueryDef{},
		Updates:     []UpdateDef{},
		Timers:      []TimerDef{},
		Versions:    []VersionDef{},
		SearchAttrs: []SearchAttrDef{},
		CallSites:   []CallSite{},
	}

	// Walk through the function body
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		info := e.analyzeCall(call, filePath, fset)
		if info == nil {
			return true
		}

		switch info.Type {
		case "signal":
			if info.SignalDef != nil {
				details.Signals = append(details.Signals, *info.SignalDef)
			}
		case "query":
			if info.QueryDef != nil {
				details.Queries = append(details.Queries, *info.QueryDef)
			}
		case "update":
			if info.UpdateDef != nil {
				details.Updates = append(details.Updates, *info.UpdateDef)
			}
		case "timer":
			if info.TimerDef != nil {
				details.Timers = append(details.Timers, *info.TimerDef)
			}
		case "version":
			if info.VersionDef != nil {
				details.Versions = append(details.Versions, *info.VersionDef)
			}
		case "search_attr":
			if info.SearchAttrDef != nil {
				details.SearchAttrs = append(details.SearchAttrs, *info.SearchAttrDef)
			}
		case "activity", "child_workflow", "local_activity":
			if info.TargetName != "" {
				details.CallSites = append(details.CallSites, CallSite{
					TargetName:         info.TargetName,
					TargetType:         info.Type,
					CallType:           "execute",
					LineNumber:         info.LineNumber,
					FilePath:           info.FilePath,
					Options:            info.Options,
					ArgumentCount:      info.ArgumentCount,
					ArgumentTypes:      info.ArgumentTypes,
					ResultType:         info.ResultType,
					ParsedActivityOpts: info.ParsedActivityOpts,
				})
			}
		}

		return true
	})

	return details, nil
}

// TemporalNodeDetails holds all extracted Temporal information for a node.
type TemporalNodeDetails struct {
	Signals     []SignalDef
	Queries     []QueryDef
	Updates     []UpdateDef
	Timers      []TimerDef
	Versions    []VersionDef
	SearchAttrs []SearchAttrDef
	CallSites   []CallSite
}

// analyzeCall analyzes a call expression to extract Temporal information.
func (e *callExtractor) analyzeCall(call *ast.CallExpr, filePath string, fset *token.FileSet) *TemporalCallInfo {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		// Check for direct function calls that might be temporal
		if ident, ok := call.Fun.(*ast.Ident); ok {
			if e.isLikelyTemporalFunction(ident.Name) {
				lineNum := e.getLineNumber(call, fset)
				return &TemporalCallInfo{
					Type:       e.inferTypeFromName(ident.Name),
					TargetName: ident.Name,
					LineNumber: lineNum,
					FilePath:   filepath.Base(filePath),
				}
			}
		}
		return nil
	}

	lineNum := e.getLineNumber(call, fset)

	// Handle chained calls like workflow.ExecuteActivity(...).Get(ctx, &result)
	if innerCall, ok := sel.X.(*ast.CallExpr); ok {
		if sel.Sel.Name == "Get" {
			// This is a .Get() call on a Future - analyze the inner call and extract result type
			info := e.analyzeCall(innerCall, filePath, fset)
			if info != nil {
				// Extract result type from .Get(ctx, &result)
				if len(call.Args) >= 2 {
					info.ResultType = e.extractResultType(call.Args[1])
				}
				return info
			}
		}
		return nil
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	// Check if this is a workflow package call
	if ident.Name == "workflow" {
		return e.analyzeWorkflowCall(sel.Sel.Name, call, filePath, lineNum)
	}

	// Check for selector calls that look like temporal functions
	if e.isLikelyTemporalFunction(sel.Sel.Name) {
		return &TemporalCallInfo{
			Type:       e.inferTypeFromName(sel.Sel.Name),
			TargetName: sel.Sel.Name,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
		}
	}

	return nil
}

// extractInternalCalls extracts all internal function calls (non-Temporal) from a function body.
// This includes local function calls, method calls, and package function calls.
func (e *callExtractor) extractInternalCalls(ctx context.Context, fn *ast.FuncDecl, filePath string, fset *token.FileSet) []InternalCall {
	if fn.Body == nil {
		return nil
	}

	var calls []InternalCall
	seen := make(map[string]bool) // Dedupe by target name

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		lineNum := e.getLineNumber(call, fset)
		var callInfo *InternalCall

		switch fun := call.Fun.(type) {
		case *ast.Ident:
			// Direct function call: myFunc()
			name := fun.Name
			// Skip builtins and common standard library functions
			if !e.isBuiltinOrCommon(name) && !seen[name] {
				seen[name] = true
				callInfo = &InternalCall{
					TargetName: name,
					CallType:   "function",
					LineNumber: lineNum,
					FilePath:   filepath.Base(filePath),
				}
			}

		case *ast.SelectorExpr:
			// Method call or package function: obj.Method() or pkg.Func()
			methodName := fun.Sel.Name

			// Get the receiver/package name
			var receiverName string
			switch x := fun.X.(type) {
			case *ast.Ident:
				receiverName = x.Name
			case *ast.SelectorExpr:
				// Chained: obj.field.Method()
				receiverName = e.exprToString(x)
			case *ast.CallExpr:
				// Result of a call: GetFoo().Bar()
				receiverName = "<call>"
			}

			// Skip workflow/activity/temporal package calls (already handled)
			if receiverName == "workflow" || receiverName == "activity" || receiverName == "temporal" {
				return true
			}

			// Skip common non-interesting calls
			if e.isBoringCall(receiverName, methodName) {
				return true
			}

			fullName := methodName
			if receiverName != "" && receiverName != "<call>" {
				fullName = receiverName + "." + methodName
			}

			if !seen[fullName] {
				seen[fullName] = true
				callInfo = &InternalCall{
					TargetName: methodName,
					Receiver:   receiverName,
					CallType:   "method",
					LineNumber: lineNum,
					FilePath:   filepath.Base(filePath),
				}
			}
		}

		if callInfo != nil {
			calls = append(calls, *callInfo)
		}

		return true
	})

	return calls
}

// isBuiltinOrCommon returns true for builtin functions and very common stdlib functions.
func (e *callExtractor) isBuiltinOrCommon(name string) bool {
	builtins := map[string]bool{
		"append": true, "cap": true, "close": true, "complex": true,
		"copy": true, "delete": true, "imag": true, "len": true,
		"make": true, "new": true, "panic": true, "print": true,
		"println": true, "real": true, "recover": true,
	}
	return builtins[name]
}

// isBoringCall returns true for calls that are generally not interesting for analysis.
func (e *callExtractor) isBoringCall(receiver, method string) bool {
	// Skip error handling patterns
	boringMethods := map[string]bool{
		"Error": true, "Unwrap": true, "Is": true, "As": true, "Wrap": true, "Wrapf": true,
		// Logging
		"Info": true, "Debug": true, "Warn": true, "Errorf": true,
		"Infof": true, "Debugf": true, "Warnf": true,
		"InfoContext": true, "DebugContext": true, "WarnContext": true, "ErrorContext": true,
		"Printf": true, "Println": true, "Print": true, "Sprintf": true,
		"Log": true, "Logf": true,
		// Common getters/utilities
		"String": true, "Int": true, "Bool": true, "Float64": true,
		"Bytes": true, "Len": true, "Cap": true, "Close": true,
		"Read": true, "Write": true, "Seek": true, "Flush": true,
	}
	if boringMethods[method] {
		return true
	}

	// Skip standard library packages
	boringReceivers := map[string]bool{
		"ctx": true, "context": true,
		"strings": true, "strconv": true, "fmt": true, "bytes": true,
		"time": true, "sync": true, "atomic": true, "math": true,
		"sort": true, "json": true, "xml": true, "io": true,
		"os": true, "path": true, "filepath": true, "regexp": true,
		"reflect": true, "runtime": true, "unsafe": true,
		"log": true, "slog": true, "logger": true, "l": true,
		"errors": true, "http": true, "net": true, "url": true,
		"bufio": true, "ioutil": true, "testing": true, "flag": true,
		"encoding": true, "crypto": true, "hash": true,
		"ast": true, "token": true, "parser": true, "printer": true,
	}
	return boringReceivers[receiver]
}

// analyzeWorkflowCall analyzes workflow.* calls.
func (e *callExtractor) analyzeWorkflowCall(method string, call *ast.CallExpr, filePath string, lineNum int) *TemporalCallInfo {
	switch method {
	case "ExecuteActivity":
		target, argCount, argTypes := e.extractTemporalTargetWithArgs(call)
		return &TemporalCallInfo{
			Type:               "activity",
			TargetName:         target,
			LineNumber:         lineNum,
			FilePath:           filepath.Base(filePath),
			Options:            e.extractOptions(call),
			ArgumentCount:      argCount,
			ArgumentTypes:      argTypes,
			ParsedActivityOpts: e.extractActivityOptions(call),
		}

	case "ExecuteChildWorkflow":
		target, argCount, argTypes := e.extractTemporalTargetWithArgs(call)
		return &TemporalCallInfo{
			Type:               "child_workflow",
			TargetName:         target,
			LineNumber:         lineNum,
			FilePath:           filepath.Base(filePath),
			Options:            e.extractOptions(call),
			ArgumentCount:      argCount,
			ArgumentTypes:      argTypes,
			ParsedActivityOpts: e.extractActivityOptions(call),
		}

	case "ExecuteLocalActivity":
		target, argCount, argTypes := e.extractTemporalTargetWithArgs(call)
		return &TemporalCallInfo{
			Type:               "local_activity",
			TargetName:         target,
			LineNumber:         lineNum,
			FilePath:           filepath.Base(filePath),
			Options:            e.extractOptions(call),
			ArgumentCount:      argCount,
			ArgumentTypes:      argTypes,
			ParsedActivityOpts: e.extractActivityOptions(call),
		}

	case "SetSignalHandler":
		signalDef := e.extractSignalHandler(call, lineNum)
		return &TemporalCallInfo{
			Type:       "signal",
			TargetName: signalDef.Name,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			SignalDef:  &signalDef,
		}

	case "GetSignalChannel":
		signalDef := e.extractSignalChannel(call, lineNum)
		return &TemporalCallInfo{
			Type:       "signal",
			TargetName: signalDef.Name,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			SignalDef:  &signalDef,
		}

	case "SetQueryHandler":
		queryDef := e.extractQueryHandler(call, lineNum)
		return &TemporalCallInfo{
			Type:       "query",
			TargetName: queryDef.Name,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			QueryDef:   &queryDef,
		}

	case "SetUpdateHandler":
		updateDef := e.extractUpdateHandler(call, lineNum)
		return &TemporalCallInfo{
			Type:       "update",
			TargetName: updateDef.Name,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			UpdateDef:  &updateDef,
		}

	case "Sleep", "NewTimer":
		timerDef := e.extractTimer(call, method, lineNum)
		return &TemporalCallInfo{
			Type:       "timer",
			TargetName: fmt.Sprintf("timer_%d", lineNum),
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			TimerDef:   &timerDef,
		}

	case "GetVersion":
		versionDef := e.extractVersion(call, lineNum)
		return &TemporalCallInfo{
			Type:       "version",
			TargetName: versionDef.ChangeID,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			VersionDef: &versionDef,
		}

	case "UpsertSearchAttributes":
		searchAttrDef := e.extractSearchAttr(call, lineNum)
		return &TemporalCallInfo{
			Type:          "search_attr",
			TargetName:    searchAttrDef.Name,
			LineNumber:    lineNum,
			FilePath:      filepath.Base(filePath),
			SearchAttrDef: &searchAttrDef,
		}

	case "NewContinueAsNewError":
		return &TemporalCallInfo{
			Type:       "continue_as_new",
			TargetName: "continue_as_new",
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
		}
	}

	return nil
}

// extractSignalHandler extracts signal handler information.
func (e *callExtractor) extractSignalHandler(call *ast.CallExpr, lineNum int) SignalDef {
	signalDef := SignalDef{LineNumber: lineNum}

	if len(call.Args) >= 1 {
		// First arg is signal name
		if lit, ok := call.Args[0].(*ast.BasicLit); ok {
			signalDef.Name = strings.Trim(lit.Value, `"`)
		}
	}

	if len(call.Args) >= 2 {
		// Second arg is handler function
		if ident, ok := call.Args[1].(*ast.Ident); ok {
			signalDef.Handler = ident.Name
		}
	}

	return signalDef
}

// extractSignalChannel extracts signal channel information.
func (e *callExtractor) extractSignalChannel(call *ast.CallExpr, lineNum int) SignalDef {
	signalDef := SignalDef{LineNumber: lineNum}

	if len(call.Args) >= 2 {
		// Second arg is signal name (first is ctx)
		if lit, ok := call.Args[1].(*ast.BasicLit); ok {
			signalDef.Name = strings.Trim(lit.Value, `"`)
		}
	}

	return signalDef
}

// extractQueryHandler extracts query handler information.
func (e *callExtractor) extractQueryHandler(call *ast.CallExpr, lineNum int) QueryDef {
	queryDef := QueryDef{LineNumber: lineNum}

	if len(call.Args) >= 1 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok {
			queryDef.Name = strings.Trim(lit.Value, `"`)
		}
	}

	if len(call.Args) >= 2 {
		if ident, ok := call.Args[1].(*ast.Ident); ok {
			queryDef.Handler = ident.Name
		}
	}

	return queryDef
}

// extractUpdateHandler extracts update handler information.
func (e *callExtractor) extractUpdateHandler(call *ast.CallExpr, lineNum int) UpdateDef {
	updateDef := UpdateDef{LineNumber: lineNum}

	if len(call.Args) >= 1 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok {
			updateDef.Name = strings.Trim(lit.Value, `"`)
		}
	}

	if len(call.Args) >= 2 {
		if ident, ok := call.Args[1].(*ast.Ident); ok {
			updateDef.Handler = ident.Name
		}
	}

	return updateDef
}

// extractTimer extracts timer information.
func (e *callExtractor) extractTimer(call *ast.CallExpr, method string, lineNum int) TimerDef {
	timerDef := TimerDef{
		LineNumber: lineNum,
		IsSleep:    method == "Sleep",
	}

	// Extract duration from first non-context arg
	for i, arg := range call.Args {
		if i == 0 {
			continue // Skip context
		}
		timerDef.Duration = e.exprToString(arg)
		break
	}

	return timerDef
}

// extractVersion extracts versioning information.
func (e *callExtractor) extractVersion(call *ast.CallExpr, lineNum int) VersionDef {
	versionDef := VersionDef{LineNumber: lineNum}

	// GetVersion(ctx, changeID, minSupported, maxSupported)
	if len(call.Args) >= 2 {
		if lit, ok := call.Args[1].(*ast.BasicLit); ok {
			versionDef.ChangeID = strings.Trim(lit.Value, `"`)
		}
	}
	if len(call.Args) >= 3 {
		if lit, ok := call.Args[2].(*ast.BasicLit); ok {
			if v, err := strconv.Atoi(lit.Value); err == nil {
				versionDef.MinVersion = v
			}
		}
	}
	if len(call.Args) >= 4 {
		if lit, ok := call.Args[3].(*ast.BasicLit); ok {
			if v, err := strconv.Atoi(lit.Value); err == nil {
				versionDef.MaxVersion = v
			}
		}
	}

	return versionDef
}

// extractSearchAttr extracts search attribute information.
func (e *callExtractor) extractSearchAttr(call *ast.CallExpr, lineNum int) SearchAttrDef {
	def := SearchAttrDef{
		LineNumber: lineNum,
		Operation:  "upsert",
	}

	// Try to extract the search attribute name from the call arguments
	// UpsertSearchAttributes takes a map, try to extract keys
	if len(call.Args) > 0 {
		// Check if it's a composite literal (map)
		if comp, ok := call.Args[0].(*ast.CompositeLit); ok {
			var names []string
			for _, elt := range comp.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if key, ok := kv.Key.(*ast.BasicLit); ok {
						// Remove quotes from string literal
						name := strings.Trim(key.Value, "\"")
						names = append(names, name)
					} else if key, ok := kv.Key.(*ast.Ident); ok {
						names = append(names, key.Name)
					}
				}
			}
			if len(names) > 0 {
				def.Name = strings.Join(names, ", ")
				return def
			}
		}
		// Try to extract from identifier or selector
		def.Name = e.exprToString(call.Args[0])
		if def.Name == "" {
			def.Name = "search_attributes"
		}
	} else {
		def.Name = "search_attributes"
	}

	return def
}

// extractOptions extracts workflow/activity options from a call.
func (e *callExtractor) extractOptions(call *ast.CallExpr) []string {
	var options []string

	if len(call.Args) > 0 {
		// Check first arg for WithActivityOptions or similar
		if innerCall, ok := call.Args[0].(*ast.CallExpr); ok {
			if sel, ok := innerCall.Fun.(*ast.SelectorExpr); ok {
				if strings.HasPrefix(sel.Sel.Name, "With") {
					options = append(options, sel.Sel.Name)
				}
			}
		}
	}

	return options
}

// extractActivityOptions extracts and parses ActivityOptions from a workflow.ExecuteActivity call.
// It looks for workflow.WithActivityOptions(ctx, opts) and parses the opts struct.
func (e *callExtractor) extractActivityOptions(call *ast.CallExpr) *ActivityOptions {
	if len(call.Args) == 0 {
		return nil
	}

	// Check if first arg is workflow.WithActivityOptions(ctx, opts)
	innerCall, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return nil
	}

	sel, ok := innerCall.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Check for WithActivityOptions or WithLocalActivityOptions
	if sel.Sel.Name != "WithActivityOptions" && sel.Sel.Name != "WithLocalActivityOptions" {
		return nil
	}

	// The opts should be the second argument to WithActivityOptions
	if len(innerCall.Args) < 2 {
		return nil
	}

	optsArg := innerCall.Args[1]
	return e.parseActivityOptionsExpr(optsArg)
}

// parseActivityOptionsExpr parses an expression that represents ActivityOptions.
// It handles composite literals and tries to extract RetryPolicy and timeout fields.
func (e *callExtractor) parseActivityOptionsExpr(expr ast.Expr) *ActivityOptions {
	switch t := expr.(type) {
	case *ast.CompositeLit:
		return e.parseActivityOptionsLiteral(t)
	case *ast.UnaryExpr:
		// Handle &workflow.ActivityOptions{...}
		if t.Op.String() == "&" {
			if lit, ok := t.X.(*ast.CompositeLit); ok {
				return e.parseActivityOptionsLiteral(lit)
			}
		}
	case *ast.Ident:
		// Variable reference - we can't trace it statically without more context,
		// but we'll mark it as "options present but not parsed"
		return &ActivityOptions{
			// Mark that options were provided via variable (can't parse contents)
			optionsProvided: true,
		}
	}
	return nil
}

// parseActivityOptionsLiteral parses a workflow.ActivityOptions{...} composite literal.
func (e *callExtractor) parseActivityOptionsLiteral(lit *ast.CompositeLit) *ActivityOptions {
	opts := &ActivityOptions{
		optionsProvided: true,
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "RetryPolicy":
			// RetryPolicy is present - parse it if possible
			opts.RetryPolicy = e.parseRetryPolicy(kv.Value)
		case "StartToCloseTimeout":
			opts.StartToCloseTimeout = e.extractDurationString(kv.Value)
		case "ScheduleToCloseTimeout":
			opts.ScheduleToCloseTimeout = e.extractDurationString(kv.Value)
		case "ScheduleToStartTimeout":
			opts.ScheduleToStartTimeout = e.extractDurationString(kv.Value)
		case "HeartbeatTimeout":
			opts.HeartbeatTimeout = e.extractDurationString(kv.Value)
		}
	}

	return opts
}

// parseRetryPolicy parses a temporal.RetryPolicy struct literal.
func (e *callExtractor) parseRetryPolicy(expr ast.Expr) *RetryPolicy {
	// Handle &temporal.RetryPolicy{...}
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op.String() == "&" {
		expr = unary.X
	}

	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		// It's set to something (variable, function call, etc.) - mark as present
		return &RetryPolicy{policyProvided: true}
	}

	policy := &RetryPolicy{policyProvided: true}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "InitialInterval":
			policy.InitialInterval = e.extractDurationString(kv.Value)
		case "BackoffCoefficient":
			policy.BackoffCoefficient = e.extractFloatString(kv.Value)
		case "MaximumInterval":
			policy.MaximumInterval = e.extractDurationString(kv.Value)
		case "MaximumAttempts":
			policy.MaximumAttempts = e.extractIntValue(kv.Value)
		}
	}

	return policy
}

// extractDurationString extracts a duration expression as a string.
func (e *callExtractor) extractDurationString(expr ast.Expr) string {
	return e.exprToString(expr)
}

// extractFloatString extracts a float expression as a string.
func (e *callExtractor) extractFloatString(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok {
		return lit.Value
	}
	return e.exprToString(expr)
}

// extractIntValue extracts an integer value from an expression.
func (e *callExtractor) extractIntValue(expr ast.Expr) int {
	if lit, ok := expr.(*ast.BasicLit); ok {
		if val, err := strconv.Atoi(lit.Value); err == nil {
			return val
		}
	}
	return 0
}

// ExtractParameters extracts parameter information from a function declaration.
func (e *callExtractor) ExtractParameters(fn *ast.FuncDecl) map[string]string {
	params := make(map[string]string)

	if fn.Type.Params == nil {
		return params
	}

	for i, field := range fn.Type.Params.List {
		paramType := e.typeToString(field.Type)

		// Handle multiple names for the same type (e.g., a, b int)
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				params[name.Name] = paramType
			}
		} else {
			// Anonymous parameter
			params[fmt.Sprintf("param_%d", i)] = paramType
		}
	}

	return params
}

// extractTemporalTargetWithArgs extracts the target function name and argument info from a Temporal API call.
// Returns: target name, argument count (excluding ctx and target func), argument types
func (e *callExtractor) extractTemporalTargetWithArgs(call *ast.CallExpr) (string, int, []string) {
	// In both patterns, the target is the second argument and activity/workflow args start at index 2:
	// Pattern 1: ExecuteActivity(ctx, MyActivity, args...)
	// Pattern 2: ExecuteActivity(workflow.WithActivityOptions(ctx, opts), MyActivity, args...)
	if len(call.Args) < 2 {
		return "", 0, nil
	}

	targetArg := call.Args[1]
	argsStartIndex := 2

	targetName := e.extractFunctionReference(targetArg)

	// Count and extract types of remaining arguments
	argCount := 0
	var argTypes []string

	if argsStartIndex < len(call.Args) {
		argCount = len(call.Args) - argsStartIndex
		for i := argsStartIndex; i < len(call.Args); i++ {
			argType := e.inferExprType(call.Args[i])
			argTypes = append(argTypes, argType)
		}
	}

	return targetName, argCount, argTypes
}

// inferExprType attempts to infer the type of an expression.
// Returns a type hint or "unknown" if type cannot be determined.
func (e *callExtractor) inferExprType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.BasicLit:
		// Literal values have obvious types
		switch t.Kind.String() {
		case "INT":
			return "int"
		case "FLOAT":
			return "float64"
		case "STRING":
			return "string"
		case "CHAR":
			return "rune"
		}
	case *ast.Ident:
		// Could be a variable or constant
		// Check for common type names used as values
		switch t.Name {
		case "true", "false":
			return "bool"
		case "nil":
			return "nil"
		}
		return "var:" + t.Name // Variable, type unknown
	case *ast.SelectorExpr:
		// pkg.Const or obj.Field
		return "selector:" + e.exprToString(t)
	case *ast.UnaryExpr:
		// &x, *x, etc
		if t.Op.String() == "&" {
			innerType := e.inferExprType(t.X)
			return "*" + innerType
		}
		return e.inferExprType(t.X)
	case *ast.CompositeLit:
		// Type{...} literal
		if t.Type != nil {
			return e.typeToString(t.Type)
		}
	case *ast.CallExpr:
		// Function call result - type depends on function
		return "call:" + e.exprToString(t.Fun)
	}
	return "unknown"
}

// extractResultType extracts the type from a result pointer expression passed to .Get().
// Handles patterns like: &result, result, &MyType{}
func (e *callExtractor) extractResultType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.UnaryExpr:
		// &result or &MyType{}
		if t.Op.String() == "&" {
			switch inner := t.X.(type) {
			case *ast.Ident:
				// &result - variable name, type unknown statically
				return "var:" + inner.Name
			case *ast.CompositeLit:
				// &MyType{} - composite literal with explicit type
				if inner.Type != nil {
					return e.typeToString(inner.Type)
				}
			case *ast.IndexExpr:
				// &slice[i] - indexed expression
				return "indexed"
			}
		}
	case *ast.Ident:
		// result - variable (usually already a pointer)
		return "var:" + t.Name
	case *ast.CompositeLit:
		// MyType{} - composite literal (rare in .Get() but handle it)
		if t.Type != nil {
			return e.typeToString(t.Type)
		}
	case *ast.CallExpr:
		// new(MyType) pattern
		if ident, ok := t.Fun.(*ast.Ident); ok && ident.Name == "new" {
			if len(t.Args) > 0 {
				return e.typeToString(t.Args[0])
			}
		}
		return "call"
	}
	return "unknown"
}

// isLikelyTemporalFunction checks if a function name suggests it's a temporal function.
func (e *callExtractor) isLikelyTemporalFunction(name string) bool {
	return strings.HasSuffix(name, "Workflow") ||
		strings.HasSuffix(name, "Activity") ||
		strings.HasSuffix(name, "Signal") ||
		strings.HasSuffix(name, "Query")
}

// inferTypeFromName infers the node type from function name.
func (e *callExtractor) inferTypeFromName(name string) string {
	switch {
	case strings.HasSuffix(name, "Workflow"):
		return "workflow"
	case strings.HasSuffix(name, "Activity"):
		return "activity"
	case strings.HasSuffix(name, "Signal"):
		return "signal"
	case strings.HasSuffix(name, "Query"):
		return "query"
	default:
		return "activity" // Default to activity
	}
}

// extractFunctionReference extracts the function name from various expression types.
func (e *callExtractor) extractFunctionReference(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		// For selector expressions like handler.MethodName, include the receiver
		// This helps distinguish between different receivers calling methods with the same name
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name + "." + e.Sel.Name
		}
		return e.Sel.Name
	case *ast.FuncLit:
		return ""
	default:
		return ""
	}
}

// getLineNumber extracts line number from a call expression.
func (e *callExtractor) getLineNumber(call *ast.CallExpr, fset *token.FileSet) int {
	if fset == nil {
		return int(call.Pos())
	}
	return fset.Position(call.Pos()).Line
}

// exprToString converts an expression to a string representation.
func (e *callExtractor) exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.BasicLit:
		return t.Value
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return e.exprToString(t.X) + "." + t.Sel.Name
	case *ast.BinaryExpr:
		return e.exprToString(t.X) + " " + t.Op.String() + " " + e.exprToString(t.Y)
	default:
		return "<expr>"
	}
}

// typeToString converts an AST type expression to a string representation.
func (e *callExtractor) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.StarExpr:
		return "*" + e.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + e.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + e.typeToString(t.Key) + "]" + e.typeToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	case *ast.FuncType:
		return "func"
	case *ast.ChanType:
		return "chan " + e.typeToString(t.Value)
	case *ast.Ellipsis:
		return "..." + e.typeToString(t.Elt)
	default:
		return "unknown"
	}
}

// ExtractCallsWithFileSet extracts calls with proper position information using a file set.
func (e *callExtractor) ExtractCallsWithFileSet(ctx context.Context, fn *ast.FuncDecl, filePath string, fset *token.FileSet) ([]CallSite, error) {
	if fn.Body == nil {
		return nil, nil
	}

	var callSites []CallSite

	// Walk through the function body to find calls
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		info := e.analyzeCall(call, filePath, fset)
		if info != nil && info.TargetName != "" {
			callSites = append(callSites, CallSite{
				TargetName:         info.TargetName,
				TargetType:         info.Type,
				CallType:           info.Type,
				LineNumber:         info.LineNumber,
				FilePath:           info.FilePath,
				Options:            info.Options,
				ArgumentCount:      info.ArgumentCount,
				ArgumentTypes:      info.ArgumentTypes,
				ResultType:         info.ResultType,
				ParsedActivityOpts: info.ParsedActivityOpts,
			})
		}

		return true
	})

	return callSites, nil
}
