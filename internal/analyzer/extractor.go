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
}

// ExtractCalls finds all temporal workflow and activity calls within a function.
func (e *callExtractor) ExtractCalls(ctx context.Context, fn *ast.FuncDecl, filePath string) ([]CallSite, error) {
	if fn.Body == nil {
		return nil, nil
	}

	var callSites []CallSite

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

		info := e.analyzeCall(call, filePath, nil)
		if info != nil && info.TargetName != "" {
			callSites = append(callSites, CallSite{
				TargetName: info.TargetName,
				TargetType: info.Type,
				CallType:   info.Type,
				LineNumber: info.LineNumber,
				FilePath:   info.FilePath,
				Options:    info.Options,
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
					TargetName: info.TargetName,
					TargetType: info.Type,
					CallType:   "execute",
					LineNumber: info.LineNumber,
					FilePath:   info.FilePath,
					Options:    info.Options,
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

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	lineNum := e.getLineNumber(call, fset)

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
	if boringReceivers[receiver] {
		return true
	}

	return false
}

// analyzeWorkflowCall analyzes workflow.* calls.
func (e *callExtractor) analyzeWorkflowCall(method string, call *ast.CallExpr, filePath string, lineNum int) *TemporalCallInfo {
	switch method {
	case "ExecuteActivity":
		target := e.extractTemporalTarget(call)
		return &TemporalCallInfo{
			Type:       "activity",
			TargetName: target,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			Options:    e.extractOptions(call),
		}

	case "ExecuteChildWorkflow":
		target := e.extractTemporalTarget(call)
		return &TemporalCallInfo{
			Type:       "child_workflow",
			TargetName: target,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			Options:    e.extractOptions(call),
		}

	case "ExecuteLocalActivity":
		target := e.extractTemporalTarget(call)
		return &TemporalCallInfo{
			Type:       "local_activity",
			TargetName: target,
			LineNumber: lineNum,
			FilePath:   filepath.Base(filePath),
			Options:    e.extractOptions(call),
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

// extractTemporalTarget extracts the target function name from a Temporal API call.
func (e *callExtractor) extractTemporalTarget(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}

	var targetArg ast.Expr

	// Check if first argument is a call to workflow.WithActivityOptions
	if len(call.Args) > 0 {
		if firstCall, ok := call.Args[0].(*ast.CallExpr); ok {
			if e.isWithOptionsCall(firstCall) {
				// Pattern 2: target is second argument
				if len(call.Args) > 1 {
					targetArg = call.Args[1]
				}
			} else {
				// Pattern 1: target is second argument (first is context)
				if len(call.Args) > 1 {
					targetArg = call.Args[1]
				}
			}
		} else {
			// Pattern 1: target is second argument (first is context)
			if len(call.Args) > 1 {
				targetArg = call.Args[1]
			}
		}
	}

	if targetArg == nil {
		return ""
	}

	return e.extractFunctionReference(targetArg)
}

// isWithOptionsCall checks if a call is workflow.WithActivityOptions or similar.
func (e *callExtractor) isWithOptionsCall(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name == "workflow" &&
				(sel.Sel.Name == "WithActivityOptions" ||
					sel.Sel.Name == "WithChildWorkflowOptions" ||
					sel.Sel.Name == "WithLocalActivityOptions")
		}
	}
	return false
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
				TargetName: info.TargetName,
				TargetType: info.Type,
				CallType:   info.Type,
				LineNumber: info.LineNumber,
				FilePath:   info.FilePath,
				Options:    info.Options,
			})
		}

		return true
	})

	return callSites, nil
}
