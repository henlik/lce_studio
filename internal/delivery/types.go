// Package delivery executes explicit typed message experiments without external
// I/O, hidden routing, retries, clocks or delivery guarantees outside the model.
package delivery

import (
	"errors"

	"software-studio/internal/model"
	"software-studio/internal/state"
)

const Version = "0.7.0"
const MaxMessages = 32
const MaxResultBytes = 2 << 20

var ErrLimit = errors.New("delivery execution limit exceeded")

type Message struct {
	Name             string                      `json:"name"`
	ID               string                      `json:"id"`
	RouteID          string                      `json:"routeId"`
	SourceIntentID   string                      `json:"sourceIntentId"`
	SourceInstanceID string                      `json:"sourceInstanceId"`
	SourceCommandID  string                      `json:"sourceCommandId"`
	Effect           string                      `json:"effect"`
	TargetInstanceID string                      `json:"targetInstanceId"`
	Command          string                      `json:"command"`
	ExpectedRevision int64                       `json:"expectedRevision"`
	Inputs           map[string]model.TypedValue `json:"inputs"`
	Status           string                      `json:"status"`
	Attempts         int                         `json:"attempts"`
	Acknowledgement  *state.Receipt              `json:"acknowledgement,omitempty"`
}

type StepResult struct {
	Index          int               `json:"index"`
	Kind           string            `json:"kind"`
	Status         string            `json:"status"`
	Message        string            `json:"message"`
	InstanceID     string            `json:"instanceId,omitempty"`
	MessageName    string            `json:"messageName,omitempty"`
	MessageID      string            `json:"messageId,omitempty"`
	FaultApplied   bool              `json:"faultApplied"`
	BeforeStatus   string            `json:"beforeStatus"`
	AfterStatus    string            `json:"afterStatus"`
	BeforeAttempts int               `json:"beforeAttempts"`
	AfterAttempts  int               `json:"afterAttempts"`
	State          *state.StepResult `json:"state,omitempty"`
}

type MessageExpectationResult struct {
	Name   string                  `json:"name"`
	Passed bool                    `json:"passed"`
	Issues []model.ValidationIssue `json:"issues"`
}

type Result struct {
	EngineVersion       string                     `json:"engineVersion"`
	ModelHash           string                     `json:"modelHash"`
	ScenarioHash        string                     `json:"scenarioHash"`
	Operations          int                        `json:"operations"`
	Invocations         int                        `json:"invocations"`
	Instances           []state.InstanceResult     `json:"instances"`
	Messages            []Message                  `json:"messages"`
	Steps               []StepResult               `json:"steps"`
	Verified            bool                       `json:"verified"`
	Passed              bool                       `json:"passed"`
	Expectations        []state.ExpectationResult  `json:"expectations"`
	MessageExpectations []MessageExpectationResult `json:"messageExpectations"`
}
