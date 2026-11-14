package systemrun

import (
	"context"
	"crypto/sha256"
	"fmt"
	"software-studio/internal/model"
)

func validateCall(call StateCall) string {
	if call.CommandID == "" || len(call.CommandID) > 64 || call.ExpectedRevision == nil || *call.ExpectedRevision < 0 || *call.ExpectedRevision > model.MaxBehaviorInteger {
		return "Supply command identity and a nonnegative safe expected revision"
	}
	if call.Fault != "" && call.Fault != "before_commit" && call.Fault != "after_commit_before_reply" {
		return "Unsupported fault"
	}
	return ""
}
func namedCommand(task Task, name string) (model.StateCommand, bool) {
	if task.State != nil {
		for _, c := range task.State.Machine.Machine.Commands {
			if name != "" && c.Name == name {
				return c, true
			}
		}
	}
	return model.StateCommand{}, false
}

// A command step explicitly targets one existing owner. It shares that owner's
// receipt/revision namespace with graph invocations, but never drives data links.
func runOwnerCommand(ctx context.Context, task Task, step ScenarioStep, runtime *scenarioRuntime) ([]Event, TaskResult, error) {
	call, values := step.Calls[0], step.Inputs[0].Values
	owner := fmt.Sprintf("owner%x", sha256.Sum256([]byte(task.ID)))[:37]
	session := runtime.sessions[task.ID]
	transition, err := session.Apply(ctx, runtime.index, model.StateStep{Kind: "command", InstanceID: owner, Command: call.Command, CommandID: call.CommandID, ExpectedRevision: call.ExpectedRevision, Fault: call.Fault, Inputs: values}, nil)
	if err != nil {
		return nil, TaskResult{}, err
	}
	snapshots, err := session.Snapshot(ctx)
	if err != nil {
		return nil, TaskResult{}, err
	}
	result := TaskResult{TaskID: task.ID, Status: transition.Status, BundleHash: task.State.Machine.Digest, Outputs: map[string]model.TypedValue{}, State: &StateEvidence{Command: call.Command, Provider: task.State.Provider, Step: transition, Instance: snapshots[0]}}
	if transition.ReplyLost {
		result.Status = "reply_lost"
	} else if transition.Receipt != nil {
		result.Outputs = transition.Receipt.Outputs
	}
	if transition.Decision != nil {
		result.Operations, result.Invocations = transition.Decision.Operations, transition.Decision.Invocations
	}
	runtime.operations += result.Operations
	runtime.invocations += result.Invocations
	if runtime.operations > 4096 || runtime.invocations > 4096 {
		return nil, TaskResult{}, fmt.Errorf("scenario work budget exceeded")
	}
	return []Event{{Kind: "started", TaskID: task.ID, Values: values}, {Kind: "completed", TaskID: task.ID, Values: result.Outputs, State: result.State}}, result, nil
}
