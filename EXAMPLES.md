# Temporal Analyzer Examples

## Quick Start

### 1. Interactive Mode (Recommended)
```bash
cd cmd/temporal-analyzer
go build -o temporal-analyzer
./temporal-analyzer
```

This opens a beautiful TUI where you can browse all 610+ workflows and activities in your codebase!

### 2. Generate Complete Analysis Report
```bash
./temporal-analyzer -interactive=false -format=markdown -output=temporal-analysis.md
```

### 3. Find Specific Workflows
```bash
# Find all employee-related workflows
./temporal-analyzer -interactive=false -filter-name=".*[Ee]mployee.*"

# Find all document generation workflows  
./temporal-analyzer -interactive=false -filter-name=".*[Dd]ocument.*"

# Show only activities
./temporal-analyzer -interactive=false -filter-type=activity
```

### 4. Generate Visual Graphs
```bash
# Create a Graphviz diagram of employee workflows
./temporal-analyzer -interactive=false -root=./pkg/employee_filings/workflows -format=dot -output=employee-workflows.dot

# Convert to PNG (requires graphviz installed)
dot -Tpng employee-workflows.dot -o employee-workflows.png
```

## Real Examples from Your Codebase

### Current Statistics
Based on analysis of your Filing Factory codebase:

- **Total Workflows**: 294
- **Total Activities**: 316  
- **Max Call Depth**: 6 levels
- **Orphan Nodes**: 433 (isolated functions)

### Complex Workflow Examples

#### EmployeeFilingsProcessingWorkflow
This is one of your most complex workflows with multiple stages:

```
EmployeeFilingsProcessingWorkflow [workflow]
├── GetClientReconStatus [activity]
├── GenerateEmployeeFilingData [activity] 
├── RunDataAudits [activity]
├── GenerateW2Explanations [activity]
├── GetEmployeesExplanationDataNotPresent [activity]
├── ParseRplmExplanationData [activity]
├── CompareW2Explanations [activity]
├── GetTemplateIds [activity]
├── CreateDocGenWorkflow [workflow]
│   ├── GenerateDocuments [workflow]
│   └── EmployeeFilingsDocGenerationActivity [activity]
├── GetClientSyncDataActivity [activity]
└── CreateDocComparisonWorkflow [workflow]
    └── EmployeeFilingsComparisonWorkflow [workflow]
```

#### Document Generation Pipeline
```
BulkGenerateDocsWorkflow [workflow]
├── CreateBulkGenResultInDbActivity [activity]
└── FuDocGenAndStatusUpdateWorkflow [workflow]
    ├── CompareDocsActivity [activity]
    └── CompareDocsAndUpdateStatus [workflow]
```

### Search Examples

Find workflows that might have performance issues:
```bash
./temporal-analyzer -interactive=false -filter-name=".*Bulk.*|.*Batch.*"
```

Find all payment-related workflows:
```bash  
./temporal-analyzer -interactive=false -filter-name=".*[Pp]ayment.*"
```

Find potential long-running workflows:
```bash
./temporal-analyzer -interactive=false -filter-name=".*Process.*|.*Generation.*"
```

### Export Data for Analysis

Export everything to JSON for custom analysis:
```bash
./temporal-analyzer -interactive=false -format=json -output=full-temporal-graph.json
```

Generate documentation for your team:
```bash
./temporal-analyzer -interactive=false -format=markdown -details -output=temporal-documentation.md
```

## Integration Tips

### 1. CI/CD Integration
Add to your build pipeline to track workflow complexity:
```bash
# Generate metrics
./temporal-analyzer -format=json -output=temporal-metrics.json

# Alert if complexity exceeds thresholds
if [ $(jq '.stats.max_depth' temporal-metrics.json) -gt 8 ]; then
  echo "WARNING: Workflow depth exceeds recommended limit"
fi
```

### 2. Code Review Usage
Before adding new workflows, check current patterns:
```bash
./temporal-analyzer -filter-name=".*$(echo $NEW_WORKFLOW_NAME | cut -d' ' -f1).*"
```

### 3. Refactoring Analysis
Find orphan workflows that might be dead code:
```bash
./temporal-analyzer -interactive=false | grep -A 1000 "Orphan Nodes:" | grep "\[workflow\]"
```

### 4. Architecture Documentation
Generate architecture diagrams for different domains:
```bash
# Employee workflows
./temporal-analyzer -root=./pkg/employee_filings -format=dot -output=employee-arch.dot

# Document generation workflows  
./temporal-analyzer -filter-name=".*[Dd]ocument.*" -format=dot -output=document-arch.dot

# Payment workflows
./temporal-analyzer -filter-name=".*[Pp]ayment.*" -format=dot -output=payment-arch.dot
```

## Troubleshooting Workflow Issues

### Find Deeply Nested Workflows
```bash
./temporal-analyzer -interactive=false -details | grep -B 2 -A 5 "Max Depth: [6789]"
```

### Identify Potential Bottlenecks
Look for workflows with many children (high fan-out):
```bash
./temporal-analyzer -format=json | jq '.nodes[] | select(.children | length > 10) | {name, child_count: (.children | length)}'
```

### Find Missing Activity Implementations
```bash
./temporal-analyzer -format=json | jq '.nodes[] | select(.children[] | inside("unknown"))'
```