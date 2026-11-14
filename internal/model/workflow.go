package model

// Workflow definitions orchestrate pure decisions, owned commands and explicit
// human/adapter responses. They do not introduce external I/O or a real clock.
type WorkflowBinding struct {
	Input    string      `json:"input,omitempty"`
	Variable string      `json:"variable,omitempty"`
	Literal  *TypedValue `json:"literal,omitempty"`
}
type WorkflowVariable struct {
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	RecordRef *DataTypeRef `json:"recordRef,omitempty"`
	Initial   TypedValue   `json:"initial"`
}
type WorkflowOwner struct {
	ID      string                     `json:"id"`
	Name    string                     `json:"name"`
	Machine DeliveryMachineRef         `json:"machine"`
	Initial map[string]WorkflowBinding `json:"initial"`
}
type WorkflowBranch struct {
	Variable string `json:"variable"`
	Then     string `json:"then"`
	Else     string `json:"else"`
}
type WorkflowTask struct {
	Kind         string                     `json:"kind"`
	Title        string                     `json:"title"`
	Instructions string                     `json:"instructions"`
	Request      map[string]WorkflowBinding `json:"request"`
	Response     []BehaviorPort             `json:"response"`
	Assign       map[string]string          `json:"assign"`
}
type WorkflowDecision struct {
	Component ComponentRef               `json:"component"`
	Inputs    map[string]WorkflowBinding `json:"inputs"`
	Assign    map[string]string          `json:"assign"`
}
type WorkflowCommand struct {
	Owner            string                     `json:"owner"`
	Command          string                     `json:"command"`
	Inputs           map[string]WorkflowBinding `json:"inputs"`
	Assign           map[string]string          `json:"assign"`
	AcceptedVariable string                     `json:"acceptedVariable"`
	ErrorNext        string                     `json:"errorNext"`
}
type WorkflowEnd struct {
	Outcome string                     `json:"outcome"`
	Outputs map[string]WorkflowBinding `json:"outputs"`
}
type WorkflowNode struct {
	// Stage is a presentation path, not an execution boundary.
	Stage    string            `json:"stage,omitempty"`
	Color    string            `json:"color,omitempty"`
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Kind     string            `json:"kind"`
	Position Position          `json:"position"`
	Task     *WorkflowTask     `json:"task,omitempty"`
	Decision *WorkflowDecision `json:"decision,omitempty"`
	Command  *WorkflowCommand  `json:"command,omitempty"`
	End      *WorkflowEnd      `json:"end,omitempty"`
	Next     string            `json:"next,omitempty"`
	Branch   *WorkflowBranch   `json:"branch,omitempty"`
}
type WorkflowSignal struct {
	Name   string                     `json:"name"`
	Inputs []BehaviorPort             `json:"inputs"`
	When   string                     `json:"when,omitempty"`
	Set    map[string]WorkflowBinding `json:"set"`
	Target string                     `json:"target"`
}
type WorkflowDefinition struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	Version       string             `json:"version"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Author        string             `json:"author"`
	License       string             `json:"license"`
	DataContracts []DataContractLock `json:"dataContracts"`
	Decisions     []ComponentBundle  `json:"decisions"`
	Machines      []StateMachineLock `json:"machines"`
	Owners        []WorkflowOwner    `json:"owners"`
	Inputs        []BehaviorPort     `json:"inputs"`
	Outputs       []BehaviorPort     `json:"outputs"`
	Variables     []WorkflowVariable `json:"variables"`
	Entry         string             `json:"entry"`
	Nodes         []WorkflowNode     `json:"nodes"`
	Signals       []WorkflowSignal   `json:"signals"`
}
type WorkflowLock struct {
	Workflow WorkflowDefinition `json:"workflow"`
	Digest   string             `json:"digest"`
}
type WorkflowEvent struct {
	ID     string                `json:"id"`
	Kind   string                `json:"kind"`
	TaskID string                `json:"taskId,omitempty"`
	Signal string                `json:"signal,omitempty"`
	Values map[string]TypedValue `json:"values"`
}
type WorkflowExpectation struct {
	Status         string                `json:"status"`
	Outcome        string                `json:"outcome"`
	ActiveTaskNode string                `json:"activeTaskNode"`
	Variables      map[string]TypedValue `json:"variables"`
	Owners         []StateExpectation    `json:"owners"`
	Tasks          int                   `json:"tasks"`
	CompletedTasks int                   `json:"completedTasks"`
	CancelledTasks int                   `json:"cancelledTasks"`
	EventReceipts  int                   `json:"eventReceipts"`
}
type WorkflowScenario struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Name          string                `json:"name"`
	Inputs        map[string]TypedValue `json:"inputs"`
	Events        []WorkflowEvent       `json:"events"`
	Expected      *WorkflowExpectation  `json:"expected,omitempty"`
}
type WorkflowExample struct {
	Workflow WorkflowLock     `json:"workflow"`
	Scenario WorkflowScenario `json:"scenario"`
}
