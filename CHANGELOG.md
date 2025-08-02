# Temporal Analyzer Changelog

## v1.1.0 - UI/UX Improvements

### 🎨 **TUI Display Fixes**

#### 1. **Fixed Narrow Display for Long Workflow Names**
- **Before**: Names were truncated aggressively, showing only fragments
- **After**: Names display up to 80 characters, only truncating when absolutely necessary
- **Benefit**: You can now see full workflow names like `EmployeeFilingsProcessingWorkflow`

#### 2. **Fixed Navigation Bug** 
- **Before**: Pressing `q` or `Esc` in details view would quit the entire application
- **After**: 
  - `q` or `Esc` in details view → Goes back to list
  - `Ctrl+C` anywhere → Quits application
  - Clear visual hints show proper navigation
- **Benefit**: Natural navigation flow, no accidental quits

#### 3. **Enhanced Child Workflow Display**
- **Before**: Only showed immediate children with minimal info
- **After**: 
  - Shows child workflows with file locations `(filename:line)`
  - Displays nested children (2 levels deep) with tree structure
  - Uses icons: 🔄 for workflows, ⚙️ for activities
  - Shows call counts when there are many children
- **Benefit**: Better understanding of workflow hierarchies

### 🎯 **Visual Enhancements**

#### Better List Display
- Increased item height for better readability
- Added parent/child call counts in descriptions
- Improved spacing and margins

#### Enhanced Details View
```
📞 Calls:
  🔄 CreateDocGenWorkflow [workflow] (employee_filings_processor.go:373)
    └─ 🔄 GenerateDocuments [workflow]
    └─ ⚙️ EmployeeFilingsDocGenerationActivity [activity]
  ⚙️ GetClientSyncDataActivity [activity] (get_client_sync_data_activity.go:25)

📤 Called by:
  🔄 SomeParentWorkflow [workflow] (parent_workflow.go:42)

🔙 q/Esc: Back to list • 🔍 /: Search • ⚠️ Ctrl+C: Quit
```

#### Improved Header
- Added emoji indicators for better visual scanning
- Color-coded header with blue background
- Clearer instruction text

### 🔍 **Search & Navigation**
- Better search feedback with filtered results
- Maintained search state during navigation
- Clear search instructions and cancellation

### 📊 **Statistics Display**
- More comprehensive call relationship tracking
- Better orphan node detection
- Enhanced depth calculation

## Usage Examples

### Navigate the Complex EmployeeFilingsProcessingWorkflow
```bash
./temporal-analyzer
# Search for "EmployeeFilings"
# Press Enter to see the full hierarchy with nested workflows
```

### Generate Documentation with New Details
```bash
./temporal-analyzer -interactive=false -details -format=markdown -output=workflows.md
```

The tool now provides a much better experience for exploring your 610+ temporal workflows and activities!