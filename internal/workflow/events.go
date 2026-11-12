package workflow

import (
	"encoding/json"
	"fmt"
	"software-studio/internal/model"
)

func eventFingerprint(event model.WorkflowEvent) (string, error) {
	request := struct {
		Kind   string                      `json:"kind"`
		TaskID string                      `json:"taskId"`
		Signal string                      `json:"signal"`
		Values map[string]model.TypedValue `json:"values"`
	}{event.Kind, event.TaskID, event.Signal, event.Values}
	encoded, err := json.Marshal(request)
	return hash(encoded), err
}

func (e *engine) event(index int, event model.WorkflowEvent) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	fingerprint, err := eventFingerprint(event)
	if err != nil {
		return err
	}
	result := EventResult{Index: index, ID: event.ID, BeforeTaskID: e.taskID(), AfterTaskID: e.taskID()}
	if prior, found := e.receipts[event.ID]; found {
		receipt := e.result.Receipts[prior]
		if receipt.RequestHash == fingerprint {
			result.Status = "replayed"
			result.Message = "Historical event receipt replayed."
			result.Receipt = &receipt
		} else {
			result.Status = "id_conflict"
			result.Message = "Event ID was already used for a different request."
		}
		p := e.project()
		n := size(result)
		p.events++
		p.eventBytes += n
		if err := e.admit(p); err != nil {
			return err
		}
		e.result.Events = append(e.result.Events, result)
		e.eventBytes += n
		return nil
	}
	if len(e.result.Receipts) >= 64 {
		return fmt.Errorf("%w: at most 64 event receipts", ErrLimit)
	}
	receipt := EventReceipt{ID: event.ID, RequestHash: fingerprint, Kind: event.Kind, TaskID: event.TaskID, Signal: event.Signal, Status: "rejected"}
	// Reserve an upper bound on the fixed event envelope before any ownership
	// copies or state commits caused by this event. Values are never duplicated
	// into receipts. No reservation depends on an unbounded accumulated journal.
	longestOutcome := ""
	for _, node := range e.definition.Nodes {
		if node.End != nil && size(node.End.Outcome) > size(longestOutcome) {
			longestOutcome = node.End.Outcome
		}
	}
	reservedReceipt := receipt
	reservedReceipt.NextTaskID = "task64"
	reservedReceipt.Outcome = longestOutcome
	reservedReceipt.Message = "Task response does not match the declared response contract."
	reservedResult := result
	reservedResult.Status = "accepted"
	reservedResult.Message = reservedReceipt.Message
	reservedResult.AfterTaskID = "task64"
	reservedResult.Receipt = &reservedReceipt
	e.pendingEventBytes = size(reservedReceipt) + size(reservedResult) + 2
	if err := e.admit(e.project()); err != nil {
		return err
	}
	message := ""
	if e.result.Status != "waiting" {
		message = "Workflow is terminal; the event cannot reopen it."
	} else if event.Kind == "response" {
		message, err = e.respond(event)
	} else {
		message, err = e.signal(event)
	}
	if err != nil {
		return err
	}
	if message == "" {
		receipt.Status = "accepted"
		receipt.Message = "Event accepted."
		if err := e.drain(); err != nil {
			return err
		}
	} else {
		receipt.Message = message
	}
	receipt.NextTaskID, receipt.Outcome = e.taskID(), e.result.Outcome
	result.Status, result.Message, result.AfterTaskID, result.Receipt = receipt.Status, receipt.Message, e.taskID(), &receipt
	e.pendingEventBytes = 0
	rn, en := size(receipt), size(result)
	p := e.project()
	p.receipts++
	p.receiptBytes += rn
	p.events++
	p.eventBytes += en
	if err := e.admit(p); err != nil {
		return err
	}
	e.receipts[event.ID] = len(e.result.Receipts)
	e.result.Receipts = append(e.result.Receipts, receipt)
	e.receiptBytes += rn
	e.result.Events = append(e.result.Events, result)
	e.eventBytes += en
	return nil
}

func (e *engine) respond(event model.WorkflowEvent) (string, error) {
	if e.active < 0 || event.TaskID != e.taskID() {
		return "Response does not name the current pending task.", nil
	}
	task := e.result.Tasks[e.active]
	if issues := model.ValidateStructuredBehaviorValues(event.Values, task.Response, e.prepared.Contracts); len(issues) > 0 {
		return "Task response does not match the declared response contract.", nil
	}
	node := e.prepared.Nodes[task.NodeID]
	changes := assignments(node.Task.Assign, event.Values)
	values := stagedValues(e.result.Variables, changes)
	if err := e.validateContext(values); err != nil {
		return "", err
	}
	task.Status, task.CompletedBy, task.Values = "completed", event.ID, event.Values
	n := size(task)
	p := e.project()
	p.variables = size(values)
	p.taskBytes += n - e.taskSizes[e.active]
	p.activeTaskBytes = 0
	p.status = "running"
	p.node = nextNode(node, values)
	if err := e.admit(p); err != nil {
		return "", err
	}
	e.taskBytes = p.taskBytes
	e.taskSizes[e.active] = n
	e.result.Tasks[e.active] = copyTask(task)
	e.active = -1
	e.result.Variables = copyValues(values)
	e.result.Status = "running"
	e.result.CurrentNode = p.node
	return "", nil
}

func (e *engine) signal(event model.WorkflowEvent) (string, error) {
	signal, found := e.prepared.Signals[event.Signal]
	if !found {
		return "Signal is not declared by this workflow.", nil
	}
	if issues := model.ValidateStructuredBehaviorValues(event.Values, signal.Inputs, e.prepared.Contracts); len(issues) > 0 {
		return "Signal values do not match the declared input contract.", nil
	}
	if signal.When != "" && !*e.result.Variables[signal.When].Boolean {
		return "Signal guard is false.", nil
	}
	changes := e.resolve(signal.Set, event.Values, e.result.Variables)
	values := stagedValues(e.result.Variables, changes)
	if err := e.validateContext(values); err != nil {
		return "", err
	}
	task := e.result.Tasks[e.active]
	task.Status, task.CompletedBy = "cancelled", event.ID
	n := size(task)
	p := e.project()
	p.variables = size(values)
	p.taskBytes += n - e.taskSizes[e.active]
	p.activeTaskBytes = 0
	p.status = "running"
	p.node = signal.Target
	if err := e.admit(p); err != nil {
		return "", err
	}
	e.taskBytes = p.taskBytes
	e.taskSizes[e.active] = n
	e.result.Tasks[e.active] = copyTask(task)
	e.active = -1
	e.result.Variables = copyValues(values)
	e.result.Status = "running"
	e.result.CurrentNode = signal.Target
	return "", nil
}
