package analyzer

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ikari-pl/go-temporalio-analyzer/internal/config"
)

// RegistrationInfo holds information about Temporal registrations found in the codebase.
type RegistrationInfo struct {
	// Activities maps function/method names to their registration info.
	// Key is either "FunctionName" for direct registrations or "TypeName" for struct registrations.
	Activities map[string]*Registration

	// Workflows maps function names to their registration info.
	Workflows map[string]*Registration

	// RegisteredTypes maps type names to their registration type ("activity" or "workflow").
	// When a struct is registered, all its exported methods become activities/workflows.
	RegisteredTypes map[string]string
}

// Registration holds details about a single registration call.
type Registration struct {
	Name       string // Function or type name
	Type       string // "activity", "workflow", "local_activity"
	FilePath   string
	LineNumber int
	IsStruct   bool   // True if this is a struct registration (all methods)
	TypeName   string // For struct registrations, the type name
}

// registrationScanner scans for worker.Register* calls.
type registrationScanner struct {
	logger *slog.Logger
}

// NewRegistrationScanner creates a new registration scanner.
func NewRegistrationScanner(logger *slog.Logger) *registrationScanner {
	return &registrationScanner{
		logger: logger,
	}
}

// ScanDirectory scans all Go files in a directory for Temporal registrations.
func (s *registrationScanner) ScanDirectory(ctx context.Context, rootDir string, opts config.AnalysisOptions) (*RegistrationInfo, error) {
	info := &RegistrationInfo{
		Activities:      make(map[string]*Registration),
		Workflows:       make(map[string]*Registration),
		RegisteredTypes: make(map[string]string),
	}

	fset := token.NewFileSet()

	err := filepath.Walk(rootDir, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			s.logger.Warn("Error accessing path during registration scan", "path", path, "error", err)
			return nil // Continue walking
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if fileInfo.IsDir() {
			for _, excludeDir := range opts.ExcludeDirs {
				if fileInfo.Name() == excludeDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if !opts.IncludeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse the file
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			s.logger.Warn("Error parsing file for registrations", "path", path, "error", err)
			return nil
		}

		// Scan for registration calls
		s.scanFile(ctx, file, fset, path, info)

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.logger.Info("Scanned for registrations",
		"activities", len(info.Activities),
		"workflows", len(info.Workflows),
		"types", len(info.RegisteredTypes))

	return info, nil
}

// scanFile scans a single file for registration calls.
func (s *registrationScanner) scanFile(ctx context.Context, file *ast.File, fset *token.FileSet, filePath string, info *RegistrationInfo) {
	ast.Inspect(file, func(n ast.Node) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for worker.RegisterActivity, worker.RegisterWorkflow, etc.
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		if ident.Name != "worker" {
			return true
		}

		lineNum := fset.Position(call.Pos()).Line

		switch sel.Sel.Name {
		case "RegisterActivity", "RegisterActivityWithOptions":
			s.extractRegistration(call, filePath, lineNum, "activity", info)
		case "RegisterWorkflow", "RegisterWorkflowWithOptions":
			s.extractRegistration(call, filePath, lineNum, "workflow", info)
		}

		return true
	})
}

// extractRegistration extracts registration info from a Register* call.
func (s *registrationScanner) extractRegistration(call *ast.CallExpr, filePath string, lineNum int, regType string, info *RegistrationInfo) {
	if len(call.Args) == 0 {
		return
	}

	arg := call.Args[0]

	// Handle different argument patterns:
	// 1. worker.RegisterActivity(MyActivity) - direct function
	// 2. worker.RegisterActivity(&MyActivities{}) - struct pointer literal
	// 3. worker.RegisterActivity(new(MyActivities)) - new() call
	// 4. worker.RegisterActivity(activities) - variable (struct instance)

	reg := &Registration{
		Type:       regType,
		FilePath:   filePath,
		LineNumber: lineNum,
	}

	switch expr := arg.(type) {
	case *ast.Ident:
		// Could be a function or a variable holding a struct
		// For functions, the name is direct
		// For variables, we'd need type analysis (beyond AST)
		reg.Name = expr.Name
		s.addRegistration(reg, info)

	case *ast.SelectorExpr:
		// pkg.Function or receiver.Method
		if ident, ok := expr.X.(*ast.Ident); ok {
			reg.Name = ident.Name + "." + expr.Sel.Name
		} else {
			reg.Name = expr.Sel.Name
		}
		s.addRegistration(reg, info)

	case *ast.UnaryExpr:
		// &MyActivities{} or &myActivities
		if expr.Op.String() == "&" {
			s.handlePointerArg(expr.X, reg, info)
		}

	case *ast.CallExpr:
		// new(MyActivities) or SomeFunction()
		if ident, ok := expr.Fun.(*ast.Ident); ok && ident.Name == "new" {
			if len(expr.Args) > 0 {
				if typeIdent, ok := expr.Args[0].(*ast.Ident); ok {
					reg.Name = typeIdent.Name
					reg.TypeName = typeIdent.Name
					reg.IsStruct = true
					info.RegisteredTypes[typeIdent.Name] = regType
					s.addRegistration(reg, info)
				}
			}
		}
	}
}

// handlePointerArg handles &Something expressions.
func (s *registrationScanner) handlePointerArg(expr ast.Expr, reg *Registration, info *RegistrationInfo) {
	switch x := expr.(type) {
	case *ast.CompositeLit:
		// &MyActivities{} - struct literal
		if typeExpr, ok := x.Type.(*ast.Ident); ok {
			reg.Name = typeExpr.Name
			reg.TypeName = typeExpr.Name
			reg.IsStruct = true
			info.RegisteredTypes[typeExpr.Name] = reg.Type
			s.addRegistration(reg, info)
		} else if sel, ok := x.Type.(*ast.SelectorExpr); ok {
			// &pkg.MyActivities{}
			if pkgIdent, ok := sel.X.(*ast.Ident); ok {
				reg.Name = pkgIdent.Name + "." + sel.Sel.Name
				reg.TypeName = sel.Sel.Name
				reg.IsStruct = true
				info.RegisteredTypes[sel.Sel.Name] = reg.Type
				s.addRegistration(reg, info)
			}
		}

	case *ast.Ident:
		// &myActivities - pointer to variable
		// We can't determine if it's a struct without type analysis,
		// but we'll assume it's a struct registration
		reg.Name = x.Name
		reg.IsStruct = true
		s.addRegistration(reg, info)
	}
}

// addRegistration adds a registration to the appropriate map.
func (s *registrationScanner) addRegistration(reg *Registration, info *RegistrationInfo) {
	switch reg.Type {
	case "activity", "local_activity":
		info.Activities[reg.Name] = reg
	case "workflow":
		info.Workflows[reg.Name] = reg
	}

	s.logger.Debug("Found registration",
		"type", reg.Type,
		"name", reg.Name,
		"isStruct", reg.IsStruct,
		"file", reg.FilePath,
		"line", reg.LineNumber)
}

// IsRegisteredActivity checks if a function name is registered as an activity.
// It checks both direct function registrations and method registrations via struct types.
func (info *RegistrationInfo) IsRegisteredActivity(funcName string, receiverType string) bool {
	// Check direct registration
	if _, ok := info.Activities[funcName]; ok {
		return true
	}

	// Check if the receiver type is registered as an activity struct
	if receiverType != "" {
		// Remove pointer prefix if present
		cleanType := strings.TrimPrefix(receiverType, "*")
		if regType, ok := info.RegisteredTypes[cleanType]; ok && regType == "activity" {
			return true
		}
	}

	return false
}

// IsRegisteredWorkflow checks if a function name is registered as a workflow.
func (info *RegistrationInfo) IsRegisteredWorkflow(funcName string) bool {
	_, ok := info.Workflows[funcName]
	return ok
}

// IsRegisteredType checks if a type name is registered (for struct registrations).
func (info *RegistrationInfo) IsRegisteredType(typeName string) (string, bool) {
	regType, ok := info.RegisteredTypes[typeName]
	return regType, ok
}
