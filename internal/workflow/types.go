// Package workflow executes finite interactive experiments with explicit
// human/adapter responses. It performs no external I/O or clinical assessment.
package workflow

import (
	"errors"
	"software-studio/internal/behavior"
	"software-studio/internal/model"
	"software-studio/internal/state"
)

const Version = "0.8.0"
const MaxTransitions = 256
const MaxTasks = 64
const MaxResultBytes = 4 << 20

var ErrLimit = errors.New("workflow execution limit exceeded")

type Task struct {
	ID           string                      `json:"id"`
	NodeID       string                      `json:"nodeId"`
	Kind         string                      `json:"kind"`
	Title        string                      `json:"title"`
	Instructions string                      `json:"instructions"`
	Request      map[string]model.TypedValue `json:"request"`
	Response     []model.BehaviorPort        `json:"response"`
	Status       string                      `json:"status"`
	CompletedBy  string                      `json:"completedBy"`
	Values       map[string]model.TypedValue `json:"values"`
}
type EventReceipt struct {
	ID          string `json:"id"`
	RequestHash string `json:"requestHash"`
	Kind        string `json:"kind"`
	TaskID      string `json:"taskId,omitempty"`
	Signal      string `json:"signal,omitempty"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	NextTaskID  string `json:"nextTaskId"`
	Outcome     string `json:"outcome"`
}
type EventResult struct {
	Index        int           `json:"index"`
	ID           string        `json:"id"`
	Status       string        `json:"status"`
	Message      string        `json:"message"`
	BeforeTaskID string        `json:"beforeTaskId"`
	AfterTaskID  string        `json:"afterTaskId"`
	Receipt      *EventReceipt `json:"receipt,omitempty"`
}
type StepResult struct {
	Index       int                         `json:"index"`
	NodeID      string                      `json:"nodeId"`
	Kind        string                      `json:"kind"`
	Status      string                      `json:"status"`
	Message     string                      `json:"message"`
	Next        string                      `json:"next"`
	TaskID      string                      `json:"taskId,omitempty"`
	Assignments map[string]model.TypedValue `json:"assignments"`
	Decision    *behavior.Result            `json:"decision,omitempty"`
	State       *state.StepResult           `json:"state,omitempty"`
}
type Result struct {
	EngineVersion string                      `json:"engineVersion"`
	WorkflowHash  string                      `json:"workflowHash"`
	ScenarioHash  string                      `json:"scenarioHash"`
	Status        string                      `json:"status"`
	Outcome       string                      `json:"outcome"`
	CurrentNode   string                      `json:"currentNode"`
	Variables     map[string]model.TypedValue `json:"variables"`
	Outputs       map[string]model.TypedValue `json:"outputs"`
	Task          *Task                       `json:"task,omitempty"`
	Tasks         []Task                      `json:"tasks"`
	Owners        []state.InstanceResult      `json:"owners"`
	Receipts      []EventReceipt              `json:"receipts"`
	Events        []EventResult               `json:"events"`
	Steps         []StepResult                `json:"steps"`
	Operations    int                         `json:"operations"`
	Invocations   int                         `json:"invocations"`
	Verified      bool                        `json:"verified"`
	Passed        bool                        `json:"passed"`
	Issues        []model.ValidationIssue     `json:"issues"`
}
