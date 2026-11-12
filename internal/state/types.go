// Package state executes bounded, deterministic owned-state experiments.
// Durable values survive modeled restarts, not an external process lifetime.
// Effect intents are evidence only; this package performs no external effects.
package state

import (
	"errors"

	"software-studio/internal/behavior"
	"software-studio/internal/model"
)

const (
	Version        = "0.6.0"
	MaxOperations  = 4096
	MaxInvocations = 4096
	MaxReceipts    = 64
	MaxEffects     = 128
	MaxResultBytes = 2 << 20
)

var ErrLimit = errors.New("state execution limit exceeded")

type Receipt struct {
	CommandID   string                      `json:"commandId"`
	RequestHash string                      `json:"requestHash"`
	Command     string                      `json:"command"`
	Accepted    bool                        `json:"accepted"`
	Revision    int64                       `json:"revision"`
	Outputs     map[string]model.TypedValue `json:"outputs"`
	EffectIDs   []string                    `json:"effectIds"`
}

type EffectIntent struct {
	ID         string                      `json:"id"`
	InstanceID string                      `json:"instanceId"`
	CommandID  string                      `json:"commandId"`
	Effect     string                      `json:"effect"`
	Revision   int64                       `json:"revision"`
	Payload    map[string]model.TypedValue `json:"payload"`
}

type InstanceResult struct {
	ID       string                      `json:"id"`
	Revision int64                       `json:"revision"`
	State    map[string]model.TypedValue `json:"state"`
	Receipts []Receipt                   `json:"receipts"`
	Effects  []EffectIntent              `json:"effects"`
}

type StepResult struct {
	Index          int                         `json:"index"`
	Kind           string                      `json:"kind"`
	InstanceID     string                      `json:"instanceId"`
	CommandID      string                      `json:"commandId,omitempty"`
	Status         string                      `json:"status"`
	ReplyLost      bool                        `json:"replyLost"`
	FaultApplied   bool                        `json:"faultApplied"`
	Message        string                      `json:"message"`
	BeforeRevision int64                       `json:"beforeRevision"`
	AfterRevision  int64                       `json:"afterRevision"`
	Before         map[string]model.TypedValue `json:"before"`
	After          map[string]model.TypedValue `json:"after"`
	Receipt        *Receipt                    `json:"receipt,omitempty"`
	Decision       *behavior.Result            `json:"decision,omitempty"`
}

type ExpectationResult struct {
	InstanceID string                  `json:"instanceId"`
	Passed     bool                    `json:"passed"`
	Issues     []model.ValidationIssue `json:"issues"`
}

type Result struct {
	EngineVersion string              `json:"engineVersion"`
	MachineHash   string              `json:"machineHash"`
	ScenarioHash  string              `json:"scenarioHash"`
	Operations    int                 `json:"operations"`
	Invocations   int                 `json:"invocations"`
	Instances     []InstanceResult    `json:"instances"`
	Steps         []StepResult        `json:"steps"`
	Verified      bool                `json:"verified"`
	Passed        bool                `json:"passed"`
	Expectations  []ExpectationResult `json:"expectations"`
}
