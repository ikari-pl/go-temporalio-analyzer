package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRuntimeParser(t *testing.T) {
	rp := NewRuntimeParser()
	if rp == nil {
		t.Fatal("NewRuntimeParser returned nil")
	}
}

func TestRuntimeParserFindFunction(t *testing.T) {
	rp := NewRuntimeParser()

	// Create a temp directory with test Go files
	tmpDir, err := os.MkdirTemp("", "runtime_parser_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a go.mod file
	goModContent := []byte("module testmodule\n\ngo 1.21\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goModContent, 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create a test Go file with functions
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := []byte(`package main

// TestWorkflow is a test workflow function.
func TestWorkflow() error {
	return nil
}

// ProcessActivity handles processing.
func ProcessActivity() error {
	return nil
}

// Helper is a helper function.
func Helper() {
}
`)
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tests := []struct {
		name         string
		funcName     string
		searchPath   string
		shouldFind   bool
		expectedType string
	}{
		{
			name:         "find workflow function",
			funcName:     "TestWorkflow",
			searchPath:   testFile,
			shouldFind:   true,
			expectedType: "workflow",
		},
		{
			name:         "find activity function",
			funcName:     "ProcessActivity",
			searchPath:   testFile,
			shouldFind:   true,
			expectedType: "activity",
		},
		{
			name:         "find helper function",
			funcName:     "Helper",
			searchPath:   testFile,
			shouldFind:   true,
			expectedType: "function",
		},
		{
			name:       "function not found",
			funcName:   "NonExistent",
			searchPath: testFile,
			shouldFind: false,
		},
		{
			name:       "search in non-existent file outside module",
			funcName:   "CompletelyFakeFunction",
			searchPath: "/nonexistent/path/fake.go",
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rp.FindFunction(tt.funcName, tt.searchPath)

			if tt.shouldFind {
				if result == nil {
					t.Errorf("FindFunction(%q, %q) returned nil, expected to find function", tt.funcName, tt.searchPath)
					return
				}
				if result.Name != tt.funcName {
					t.Errorf("Found function name = %q, want %q", result.Name, tt.funcName)
				}
				if result.Type != tt.expectedType {
					t.Errorf("Found function type = %q, want %q", result.Type, tt.expectedType)
				}
				if result.FilePath != testFile {
					t.Errorf("Found function filePath = %q, want %q", result.FilePath, testFile)
				}
			} else {
				if result != nil {
					t.Errorf("FindFunction(%q, %q) = %+v, expected nil", tt.funcName, tt.searchPath, result)
				}
			}
		})
	}
}

func TestRuntimeParserFindFunctionInDirectory(t *testing.T) {
	rp := NewRuntimeParser()

	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "runtime_parser_dir_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create file1.go
	file1 := filepath.Join(tmpDir, "file1.go")
	if err := os.WriteFile(file1, []byte(`package main
func FunctionInFile1() {}
`), 0644); err != nil {
		t.Fatalf("Failed to write file1.go: %v", err)
	}

	// Create file2.go
	file2 := filepath.Join(tmpDir, "file2.go")
	if err := os.WriteFile(file2, []byte(`package main
func FunctionInFile2() {}
`), 0644); err != nil {
		t.Fatalf("Failed to write file2.go: %v", err)
	}

	// Search for function in directory (not specific file)
	result := rp.FindFunction("FunctionInFile2", tmpDir)
	if result == nil {
		t.Error("Should find FunctionInFile2 when searching directory")
	} else if result.Name != "FunctionInFile2" {
		t.Errorf("Found wrong function: %q", result.Name)
	}

	result = rp.FindFunction("FunctionInFile1", tmpDir)
	if result == nil {
		t.Error("Should find FunctionInFile1 when searching directory")
	}
}

func TestRuntimeParserFindModuleRoot(t *testing.T) {
	rp := NewRuntimeParser()

	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "module_root_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create subdirectory structure
	subDir := filepath.Join(tmpDir, "pkg", "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirs: %v", err)
	}

	// Create go.mod at root
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Test finding module root from subdirectory
	root := rp.findModuleRoot(subDir)
	if root != tmpDir {
		t.Errorf("findModuleRoot(%q) = %q, want %q", subDir, root, tmpDir)
	}

	// Test finding module root from root
	root = rp.findModuleRoot(tmpDir)
	if root != tmpDir {
		t.Errorf("findModuleRoot(%q) = %q, want %q", tmpDir, root, tmpDir)
	}

	// Test with no go.mod - should return empty
	noModDir, err := os.MkdirTemp("", "no_module_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(noModDir) }()

	root = rp.findModuleRoot(noModDir)
	if root != "" {
		t.Errorf("findModuleRoot on dir without go.mod should return empty, got %q", root)
	}
}

func TestRuntimeParserBuildNodeFromFunc(t *testing.T) {
	rp := NewRuntimeParser()

	// Create temp file with documented function
	tmpDir, err := os.MkdirTemp("", "build_node_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	testFile := filepath.Join(tmpDir, "test.go")
	testContent := []byte(`package main

// ProcessOrderWorkflow handles the order processing.
// This is a detailed description that spans multiple lines.
func ProcessOrderWorkflow(ctx context.Context, orderID string, amount int) (string, error) {
	helper()
	return "", nil
}

func helper() {
}
`)
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	node := rp.findFunctionInFile("ProcessOrderWorkflow", testFile)
	if node == nil {
		t.Fatal("Failed to find ProcessOrderWorkflow")
	}

	// Verify node properties
	if node.Name != "ProcessOrderWorkflow" {
		t.Errorf("Name = %q, want %q", node.Name, "ProcessOrderWorkflow")
	}
	if node.Type != "workflow" {
		t.Errorf("Type = %q, want %q", node.Type, "workflow")
	}
	if node.Package != "main" {
		t.Errorf("Package = %q, want %q", node.Package, "main")
	}
	if node.FilePath != testFile {
		t.Errorf("FilePath = %q, want %q", node.FilePath, testFile)
	}
	if node.LineNumber <= 0 {
		t.Error("LineNumber should be positive")
	}
	if node.Description == "" {
		t.Error("Description should be extracted from doc comment")
	}

	// Verify parameters were extracted
	if len(node.Parameters) == 0 {
		t.Error("Parameters should be extracted")
	}

	// Verify return type was extracted
	if node.ReturnType == "" {
		t.Error("ReturnType should be extracted")
	}

	// Verify internal calls were extracted
	if len(node.InternalCalls) == 0 {
		t.Error("InternalCalls should contain helper call")
	}
}

func TestRuntimeParserIsBuiltin(t *testing.T) {
	rp := NewRuntimeParser()

	builtins := []string{
		"append", "cap", "close", "complex", "copy", "delete",
		"imag", "len", "make", "new", "panic", "print",
		"println", "real", "recover",
	}

	for _, b := range builtins {
		if !rp.isBuiltin(b) {
			t.Errorf("isBuiltin(%q) = false, want true", b)
		}
	}

	nonBuiltins := []string{
		"custom", "myFunc", "Process", "Helper",
	}

	for _, nb := range nonBuiltins {
		if rp.isBuiltin(nb) {
			t.Errorf("isBuiltin(%q) = true, want false", nb)
		}
	}
}

func TestRuntimeParserIsBoringCall(t *testing.T) {
	rp := NewRuntimeParser()

	tests := []struct {
		receiver string
		method   string
		expected bool
	}{
		// Error handling
		{"err", "Error", true},
		{"", "Unwrap", true},
		{"", "Is", true},
		{"", "As", true},
		{"", "Wrap", true},
		{"", "Wrapf", true},

		// Context
		{"ctx", "Done", true},
		{"context", "Background", true},

		// Standard library packages
		{"strings", "Contains", true},
		{"fmt", "Sprintf", true},
		{"time", "Now", true},
		{"json", "Marshal", true},
		{"os", "ReadFile", true},

		// Logging
		{"log", "Info", true},
		{"logger", "Debug", true},
		{"l", "Warn", true},
		{"slog", "Error", true},
		{"", "Printf", true},
		{"", "Println", true},

		// Common getters
		{"", "String", true},
		{"", "Int", true},
		{"", "Bool", true},
		{"", "Len", true},
		{"", "Close", true},

		// Non-boring calls
		{"store", "Save", false},
		{"db", "Query", false},
		{"service", "Process", false},
		{"handler", "Handle", false},
		{"repo", "Get", false},
	}

	for _, tt := range tests {
		name := tt.receiver + "." + tt.method
		if tt.receiver == "" {
			name = tt.method
		}
		t.Run(name, func(t *testing.T) {
			result := rp.isBoringCall(tt.receiver, tt.method)
			if result != tt.expected {
				t.Errorf("isBoringCall(%q, %q) = %v, want %v", tt.receiver, tt.method, result, tt.expected)
			}
		})
	}
}

func TestRuntimeParserIsLocalFunction(t *testing.T) {
	rp := NewRuntimeParser()

	tests := []struct {
		receiver string
		expected bool
	}{
		// No receiver - local function call
		{"", true},

		// Single letter receivers (common for methods)
		{"p", true},
		{"s", true},
		{"m", true},
		{"h", true},

		// Common local receiver patterns
		{"self", true},
		{"this", true},
		{"srv", true},
		{"svc", true},
		{"service", true},
		{"handler", true},
		{"repo", true},
		{"store", true},

		// External packages (not local)
		{"fmt", false},
		{"strings", false},
		{"http", false},
		{"db", false},
		{"api", false},
	}

	for _, tt := range tests {
		t.Run(tt.receiver, func(t *testing.T) {
			result := rp.IsLocalFunction(tt.receiver)
			if result != tt.expected {
				t.Errorf("IsLocalFunction(%q) = %v, want %v", tt.receiver, result, tt.expected)
			}
		})
	}
}

func TestRuntimeParserTypeToString(t *testing.T) {
	rp := NewRuntimeParser()

	// Create temp file with various types
	tmpDir, err := os.MkdirTemp("", "type_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	testFile := filepath.Join(tmpDir, "types.go")
	testContent := []byte(`package main

import "context"

func TestBasicTypes(s string, i int, b bool) {}
func TestPointer(p *string) {}
func TestSlice(arr []string) {}
func TestMap(m map[string]int) {}
func TestInterface(i interface{}) {}
func TestPackageType(ctx context.Context) {}
func TestFunc(f func()) {}
`)
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Find functions and check their parameter types
	tests := []struct {
		funcName      string
		paramName     string // Specific parameter name to check
		expectedType  string // Expected type for the parameter
	}{
		{"TestBasicTypes", "s", "string"},
		{"TestPointer", "p", "*string"},
		{"TestSlice", "arr", "[]string"},
		{"TestMap", "m", "map[string]int"},
		{"TestInterface", "i", "interface{}"},
		{"TestPackageType", "ctx", "context.Context"},
		{"TestFunc", "f", "func"},
	}

	for _, tt := range tests {
		t.Run(tt.funcName, func(t *testing.T) {
			node := rp.findFunctionInFile(tt.funcName, testFile)
			if node == nil {
				t.Fatalf("Failed to find function %q", tt.funcName)
			}

			if len(node.Parameters) == 0 {
				t.Fatalf("Function %q has no parameters", tt.funcName)
			}

			// Get the specific parameter type by name
			paramType, ok := node.Parameters[tt.paramName]
			if !ok {
				t.Fatalf("Function %q does not have parameter %q, got %v", tt.funcName, tt.paramName, node.Parameters)
			}

			if paramType != tt.expectedType {
				t.Errorf("Parameter %q type = %q, want %q", tt.paramName, paramType, tt.expectedType)
			}
		})
	}
}

func TestRuntimeParserSkipsTestFiles(t *testing.T) {
	rp := NewRuntimeParser()

	tmpDir, err := os.MkdirTemp("", "skip_test_files")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create test file (should be skipped)
	testFile := filepath.Join(tmpDir, "main_test.go")
	if err := os.WriteFile(testFile, []byte(`package main
func TestOnlyFunction() {}
`), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Search in directory should not find function from test file
	result := rp.findFunctionInDir("TestOnlyFunction", tmpDir)
	if result != nil {
		t.Error("findFunctionInDir should skip _test.go files")
	}
}

func TestRuntimeParserExtractsDescription(t *testing.T) {
	rp := NewRuntimeParser()

	tmpDir, err := os.MkdirTemp("", "description_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	testFile := filepath.Join(tmpDir, "test.go")
	testContent := []byte(`package main

// ShortDescription is a one-liner.
func ShortDescription() {}

// LongDescription has a very long description that exceeds the maximum allowed
// length for descriptions in the analyzer. This is intentionally made very long
// to test the truncation logic that should kick in when descriptions exceed 200
// characters. We need to make sure this description is properly truncated with
// an ellipsis at the end.
func LongDescription() {}

func NoDescription() {}
`)
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test short description
	node := rp.findFunctionInFile("ShortDescription", testFile)
	if node == nil {
		t.Fatal("Failed to find ShortDescription")
	}
	if node.Description == "" {
		t.Error("ShortDescription should have a description")
	}

	// Test long description truncation
	node = rp.findFunctionInFile("LongDescription", testFile)
	if node == nil {
		t.Fatal("Failed to find LongDescription")
	}
	if len(node.Description) > 200 {
		t.Errorf("Description length = %d, should be truncated to 200 or less", len(node.Description))
	}
	if len(node.Description) > 3 && node.Description[len(node.Description)-3:] != "..." {
		t.Error("Truncated description should end with '...'")
	}

	// Test no description
	node = rp.findFunctionInFile("NoDescription", testFile)
	if node == nil {
		t.Fatal("Failed to find NoDescription")
	}
	if node.Description != "" {
		t.Errorf("NoDescription should have empty description, got %q", node.Description)
	}
}

