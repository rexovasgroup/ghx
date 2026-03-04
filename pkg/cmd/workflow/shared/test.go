package shared

// AWorkflow is a test fixture representing an active workflow.
var AWorkflow = Workflow{
	Name:  "a workflow",
	ID:    123,
	Path:  ".github/workflows/flow.yml",
	State: Active,
}

// AWorkflowContent is a test fixture with base64-encoded content for AWorkflow.
var AWorkflowContent = `{"content":"bmFtZTogYSB3b3JrZmxvdwo="}`

// DisabledWorkflow is a test fixture representing a manually disabled workflow.
var DisabledWorkflow = Workflow{
	Name:  "a disabled workflow",
	ID:    456,
	Path:  ".github/workflows/disabled.yml",
	State: DisabledManually,
}

// DisabledInactivityWorkflow is a test fixture representing a workflow disabled due to inactivity.
var DisabledInactivityWorkflow = Workflow{
	Name:  "a disabled inactivity workflow",
	ID:    1206,
	Path:  ".github/workflows/disabledInactivity.yml",
	State: DisabledInactivity,
}

// AnotherDisabledWorkflow is a test fixture representing a second manually disabled workflow.
var AnotherDisabledWorkflow = Workflow{
	Name:  "a disabled workflow",
	ID:    1213,
	Path:  ".github/workflows/anotherDisabled.yml",
	State: DisabledManually,
}

// UniqueDisabledWorkflow is a test fixture representing a disabled workflow with a unique name.
var UniqueDisabledWorkflow = Workflow{
	Name:  "terrible workflow",
	ID:    1314,
	Path:  ".github/workflows/terrible.yml",
	State: DisabledManually,
}

// AnotherWorkflow is a test fixture representing a second active workflow.
var AnotherWorkflow = Workflow{
	Name:  "another workflow",
	ID:    789,
	Path:  ".github/workflows/another.yml",
	State: Active,
}

// AnotherWorkflowContent is a test fixture with base64-encoded content for AnotherWorkflow.
var AnotherWorkflowContent = `{"content":"bmFtZTogYW5vdGhlciB3b3JrZmxvdwo="}`

// YetAnotherWorkflow is a test fixture representing a third active workflow with a duplicate name.
var YetAnotherWorkflow = Workflow{
	Name:  "another workflow",
	ID:    1011,
	Path:  ".github/workflows/yetanother.yml",
	State: Active,
}
