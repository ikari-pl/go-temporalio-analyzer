package analyzer

import (
	"testing"
)

func TestGetCategory(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		want     NodeCategory
	}{
		{"workflow returns CategoryWorkflow", "workflow", CategoryWorkflow},
		{"activity returns CategoryActivity", "activity", CategoryActivity},
		{"signal returns CategorySignal", "signal", CategorySignal},
		{"signal_handler returns CategorySignal", "signal_handler", CategorySignal},
		{"query returns CategoryQuery", "query", CategoryQuery},
		{"query_handler returns CategoryQuery", "query_handler", CategoryQuery},
		{"update returns CategoryUpdate", "update", CategoryUpdate},
		{"update_handler returns CategoryUpdate", "update_handler", CategoryUpdate},
		{"unknown returns CategoryWorkflow", "unknown", CategoryWorkflow},
		{"empty returns CategoryWorkflow", "", CategoryWorkflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCategory(tt.nodeType); got != tt.want {
				t.Errorf("GetCategory(%q) = %v, want %v", tt.nodeType, got, tt.want)
			}
		})
	}
}

func TestNodeCategories(t *testing.T) {
	// Test that category constants have expected values
	if CategoryWorkflow != "workflow" {
		t.Errorf("CategoryWorkflow = %q, want %q", CategoryWorkflow, "workflow")
	}
	if CategoryActivity != "activity" {
		t.Errorf("CategoryActivity = %q, want %q", CategoryActivity, "activity")
	}
	if CategorySignal != "signal" {
		t.Errorf("CategorySignal = %q, want %q", CategorySignal, "signal")
	}
	if CategoryQuery != "query" {
		t.Errorf("CategoryQuery = %q, want %q", CategoryQuery, "query")
	}
	if CategoryUpdate != "update" {
		t.Errorf("CategoryUpdate = %q, want %q", CategoryUpdate, "update")
	}
}

func TestTemporalNodeStructure(t *testing.T) {
	node := &TemporalNode{
		Name:        "TestWorkflow",
		Type:        "workflow",
		Package:     "test",
		FilePath:    "/path/to/file.go",
		LineNumber:  10,
		Description: "A test workflow",
		Parameters:  map[string]string{"ctx": "workflow.Context"},
		ReturnType:  "error",
		CallSites:   []CallSite{},
		Parents:     []string{"ParentWorkflow"},
		Signals: []SignalDef{
			{Name: "testSignal", LineNumber: 20},
		},
		Queries: []QueryDef{
			{Name: "testQuery", LineNumber: 30},
		},
		Updates: []UpdateDef{
			{Name: "testUpdate", LineNumber: 40},
		},
		Timers: []TimerDef{
			{Duration: "5m", LineNumber: 50, IsSleep: true},
		},
		SearchAttrs: []SearchAttrDef{
			{Name: "testAttr", Type: "keyword", LineNumber: 60},
		},
	}

	if node.Name != "TestWorkflow" {
		t.Errorf("Name = %q, want %q", node.Name, "TestWorkflow")
	}
	if node.Type != "workflow" {
		t.Errorf("Type = %q, want %q", node.Type, "workflow")
	}
	if len(node.Signals) != 1 {
		t.Errorf("len(Signals) = %d, want %d", len(node.Signals), 1)
	}
	if len(node.Queries) != 1 {
		t.Errorf("len(Queries) = %d, want %d", len(node.Queries), 1)
	}
	if len(node.Timers) != 1 {
		t.Errorf("len(Timers) = %d, want %d", len(node.Timers), 1)
	}
}

func TestCallSiteStructure(t *testing.T) {
	cs := CallSite{
		TargetName: "DoActivity",
		TargetType: "activity",
		CallType:   "execute",
		LineNumber: 15,
		FilePath:   "file.go",
		Options:    []string{"WithActivityOptions"},
	}

	if cs.TargetName != "DoActivity" {
		t.Errorf("TargetName = %q, want %q", cs.TargetName, "DoActivity")
	}
	if cs.TargetType != "activity" {
		t.Errorf("TargetType = %q, want %q", cs.TargetType, "activity")
	}
	if len(cs.Options) != 1 {
		t.Errorf("len(Options) = %d, want %d", len(cs.Options), 1)
	}
}

func TestInternalCallStructure(t *testing.T) {
	ic := InternalCall{
		TargetName: "helperFunc",
		Receiver:   "store",
		CallType:   "method",
		LineNumber: 25,
		FilePath:   "file.go",
	}

	if ic.TargetName != "helperFunc" {
		t.Errorf("TargetName = %q, want %q", ic.TargetName, "helperFunc")
	}
	if ic.Receiver != "store" {
		t.Errorf("Receiver = %q, want %q", ic.Receiver, "store")
	}
	if ic.CallType != "method" {
		t.Errorf("CallType = %q, want %q", ic.CallType, "method")
	}
}

func TestSignalDefStructure(t *testing.T) {
	sd := SignalDef{
		Name:        "updateSignal",
		Channel:     "updateChan",
		PayloadType: "UpdatePayload",
		Handler:     "handleUpdate",
		LineNumber:  35,
		Parameters:  map[string]string{"payload": "UpdatePayload"},
		IsExternal:  true,
	}

	if sd.Name != "updateSignal" {
		t.Errorf("Name = %q, want %q", sd.Name, "updateSignal")
	}
	if !sd.IsExternal {
		t.Error("IsExternal = false, want true")
	}
}

func TestQueryDefStructure(t *testing.T) {
	qd := QueryDef{
		Name:       "getState",
		Handler:    "handleGetState",
		ReturnType: "State",
		LineNumber: 45,
		Parameters: map[string]string{},
	}

	if qd.Name != "getState" {
		t.Errorf("Name = %q, want %q", qd.Name, "getState")
	}
	if qd.ReturnType != "State" {
		t.Errorf("ReturnType = %q, want %q", qd.ReturnType, "State")
	}
}

func TestUpdateDefStructure(t *testing.T) {
	ud := UpdateDef{
		Name:       "updateValue",
		Handler:    "handleUpdateValue",
		Validator:  "validateUpdate",
		ReturnType: "error",
		LineNumber: 55,
		Parameters: map[string]string{"value": "int"},
	}

	if ud.Name != "updateValue" {
		t.Errorf("Name = %q, want %q", ud.Name, "updateValue")
	}
	if ud.Validator != "validateUpdate" {
		t.Errorf("Validator = %q, want %q", ud.Validator, "validateUpdate")
	}
}

func TestTimerDefStructure(t *testing.T) {
	td := TimerDef{
		Name:       "waitTimer",
		Duration:   "10s",
		LineNumber: 65,
		IsSleep:    false,
	}

	if td.Duration != "10s" {
		t.Errorf("Duration = %q, want %q", td.Duration, "10s")
	}
	if td.IsSleep {
		t.Error("IsSleep = true, want false")
	}
}

func TestSearchAttrDefStructure(t *testing.T) {
	sa := SearchAttrDef{
		Name:       "customerId",
		Type:       "keyword",
		LineNumber: 75,
		Operation:  "upsert",
	}

	if sa.Name != "customerId" {
		t.Errorf("Name = %q, want %q", sa.Name, "customerId")
	}
	if sa.Operation != "upsert" {
		t.Errorf("Operation = %q, want %q", sa.Operation, "upsert")
	}
}

func TestWorkflowOptionsStructure(t *testing.T) {
	rp := &RetryPolicy{
		InitialInterval:    "1s",
		BackoffCoefficient: 2.0,
		MaximumInterval:    "1m",
		MaximumAttempts:    5,
		NonRetryableErrors: []string{"PermanentError"},
	}

	wo := &WorkflowOptions{
		TaskQueue:             "test-queue",
		ExecutionTimeout:      "1h",
		RunTimeout:            "30m",
		TaskTimeout:           "10s",
		RetryPolicy:           rp,
		CronSchedule:          "0 * * * *",
		Memo:                  true,
		SearchAttributes:      true,
		ParentClosePolicy:     "TERMINATE",
		WorkflowIDReusePolicy: "ALLOW_DUPLICATE",
	}

	if wo.TaskQueue != "test-queue" {
		t.Errorf("TaskQueue = %q, want %q", wo.TaskQueue, "test-queue")
	}
	if wo.RetryPolicy.MaximumAttempts != 5 {
		t.Errorf("RetryPolicy.MaximumAttempts = %d, want %d", wo.RetryPolicy.MaximumAttempts, 5)
	}
}

func TestActivityOptionsStructure(t *testing.T) {
	rp := &RetryPolicy{
		MaximumAttempts: 3,
	}

	ao := &ActivityOptions{
		TaskQueue:              "activity-queue",
		ScheduleToStartTimeout: "5m",
		StartToCloseTimeout:    "10m",
		HeartbeatTimeout:       "30s",
		ScheduleToCloseTimeout: "15m",
		RetryPolicy:            rp,
		WaitForCancellation:    true,
	}

	if ao.HeartbeatTimeout != "30s" {
		t.Errorf("HeartbeatTimeout = %q, want %q", ao.HeartbeatTimeout, "30s")
	}
	if !ao.WaitForCancellation {
		t.Error("WaitForCancellation = false, want true")
	}
}

func TestChildWorkflowStructure(t *testing.T) {
	cw := ChildWorkflow{
		Name:              "ChildWorkflow",
		LineNumber:        85,
		Options:           &WorkflowOptions{TaskQueue: "child-queue"},
		ParentClosePolicy: "REQUEST_CANCEL",
	}

	if cw.Name != "ChildWorkflow" {
		t.Errorf("Name = %q, want %q", cw.Name, "ChildWorkflow")
	}
	if cw.ParentClosePolicy != "REQUEST_CANCEL" {
		t.Errorf("ParentClosePolicy = %q, want %q", cw.ParentClosePolicy, "REQUEST_CANCEL")
	}
}

func TestLocalActivityStructure(t *testing.T) {
	la := LocalActivity{
		Name:       "LocalActivity",
		LineNumber: 95,
		Options:    &ActivityOptions{StartToCloseTimeout: "5s"},
	}

	if la.Name != "LocalActivity" {
		t.Errorf("Name = %q, want %q", la.Name, "LocalActivity")
	}
}

func TestContinueAsNewDefStructure(t *testing.T) {
	can := &ContinueAsNewDef{
		LineNumber: 105,
		Arguments:  map[string]string{"iteration": "int"},
	}

	if can.LineNumber != 105 {
		t.Errorf("LineNumber = %d, want %d", can.LineNumber, 105)
	}
}

func TestVersionDefStructure(t *testing.T) {
	vd := VersionDef{
		ChangeID:   "add-feature-x",
		MinVersion: 1,
		MaxVersion: 2,
		LineNumber: 115,
	}

	if vd.ChangeID != "add-feature-x" {
		t.Errorf("ChangeID = %q, want %q", vd.ChangeID, "add-feature-x")
	}
	if vd.MinVersion != 1 || vd.MaxVersion != 2 {
		t.Errorf("Versions = (%d, %d), want (%d, %d)", vd.MinVersion, vd.MaxVersion, 1, 2)
	}
}

func TestTemporalGraphStructure(t *testing.T) {
	graph := &TemporalGraph{
		Nodes: map[string]*TemporalNode{
			"TestWorkflow": {Name: "TestWorkflow", Type: "workflow"},
			"TestActivity": {Name: "TestActivity", Type: "activity"},
		},
		Stats: GraphStats{
			TotalWorkflows:   1,
			TotalActivities:  1,
			TotalSignals:     2,
			TotalQueries:     1,
			TotalUpdates:     0,
			TotalTimers:      3,
			MaxDepth:         5,
			OrphanNodes:      0,
			CircularDeps:     0,
			TotalConnections: 10,
			AvgFanOut:        2.5,
			MaxFanOut:        5,
		},
	}

	if len(graph.Nodes) != 2 {
		t.Errorf("len(Nodes) = %d, want %d", len(graph.Nodes), 2)
	}
	if graph.Stats.TotalWorkflows != 1 {
		t.Errorf("TotalWorkflows = %d, want %d", graph.Stats.TotalWorkflows, 1)
	}
	if graph.Stats.AvgFanOut != 2.5 {
		t.Errorf("AvgFanOut = %f, want %f", graph.Stats.AvgFanOut, 2.5)
	}
}

func TestValidationIssueStructure(t *testing.T) {
	vi := ValidationIssue{
		Type:       "warning",
		Message:    "Activity has no timeout",
		NodeName:   "TestActivity",
		Severity:   5,
		Suggestion: "Add a timeout to the activity options",
	}

	if vi.Type != "warning" {
		t.Errorf("Type = %q, want %q", vi.Type, "warning")
	}
	if vi.Severity != 5 {
		t.Errorf("Severity = %d, want %d", vi.Severity, 5)
	}
}
