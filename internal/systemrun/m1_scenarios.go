package systemrun

import (
	"fmt"
	"software-studio/internal/model"
)

type NamedScenario struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scenario    Scenario `json:"scenario"`
}

// M1Scenarios are authored expectations, not observations used as an oracle.
func M1Scenarios(p Plan) ([]NamedScenario, error) {
	var f WorkflowTask
	for _, candidate := range p.Workflows {
		if candidate.Lock.Workflow.ID == "studio.reservation-orchestration" {
			f = candidate
		}
	}
	if f.ID == "" {
		return nil, fmt.Errorf("install and prepare the M1 acceptance example")
	}
	stock, ledger := Key("stock", "stock"), Key("ledger", "ledger")
	channel := Key("stock", "notices")
	instance := "reservation"
	integer := func(v int64) model.TypedValue { return model.TypedValue{Type: "integer", Integer: &v} }
	boolean := func(v bool) model.TypedValue { return model.TypedValue{Type: "boolean", Boolean: &v} }
	text := func(v string) model.TypedValue { return model.TypedValue{Type: "string", String: &v} }
	start := func(q int64, fault bool, status string) ScenarioStep {
		faults := map[string]string{}
		if fault {
			faults["reserve"] = "before_commit"
		}
		return ScenarioStep{Kind: "workflow_start", Workflow: &WorkflowCall{WorkflowID: f.ID, InstanceID: instance, Inputs: map[string]model.TypedValue{"request": {Type: "record", RecordRef: f.Lock.Workflow.Inputs[0].RecordRef, Record: map[string]model.TypedValue{"tenantId": text("tenantDefault"), "resourceId": text("resourceDefault"), "quantity": integer(q)}}}, Faults: faults}, ExpectedWorkflowStatus: status}
	}
	response := func(id, task string, proceed bool, status string) ScenarioStep {
		return ScenarioStep{Kind: "workflow_event", Workflow: &WorkflowCall{InstanceID: instance, Event: &model.WorkflowEvent{ID: id, Kind: "response", TaskID: task, Values: map[string]model.TypedValue{"proceed": boolean(proceed)}}}, ExpectedWorkflowStatus: status}
	}
	deliver := func(kind, task, fault, status string) ScenarioStep {
		return ScenarioStep{Kind: kind, Workflow: &WorkflowCall{InstanceID: instance, TaskID: task, Fault: fault}, ExpectedWorkflowStatus: status}
	}
	restart := func(status string) ScenarioStep {
		return ScenarioStep{Kind: "workflow_restart", Workflow: &WorkflowCall{InstanceID: instance}, ExpectedWorkflowStatus: status}
	}
	expected := func(available, total int64, stockReceipts, stockEffects, ledgerReceipts, stockCounter, ledgerCounter int) []FinalExpectation {
		one := int64(stockEffects)
		other := int64(ledgerReceipts)
		zero := 0
		return []FinalExpectation{
			{TaskID: stock, Revision: &one, Receipts: &stockReceipts, Effects: &stockEffects, State: map[string]model.TypedValue{"tenantId": text("tenantDefault"), "resourceId": text("resourceDefault"), "available": integer(available), "acceptedSinceRestart": integer(int64(stockCounter))}},
			{TaskID: ledger, Revision: &other, Receipts: &ledgerReceipts, Effects: &zero, State: map[string]model.TypedValue{"tenantId": text("tenantDefault"), "resourceId": text("resourceDefault"), "totalReserved": integer(total), "acceptedSinceRestart": integer(int64(ledgerCounter))}}}
	}
	final := func(status, outcome, node string, remaining int64, accepted, recorded bool) []WorkflowExpectation {
		outputs := map[string]model.TypedValue{}
		if status == "completed" {
			outputs = map[string]model.TypedValue{"remaining": integer(remaining), "recorded": boolean(recorded)}
		}
		return []WorkflowExpectation{{InstanceID: instance, Status: status, Outcome: outcome, ActiveTaskNode: node, Variables: map[string]model.TypedValue{"remaining": integer(remaining), "accepted": boolean(accepted), "acknowledged": boolean(recorded)}, Outputs: outputs}}
	}
	message := func(task, status string, attempts int, accepted *bool) []MessageExpectation {
		return []MessageExpectation{{ChannelID: channel, DeliveryMessageExpectation: model.DeliveryMessageExpectation{Name: workflowNamespace(f.ID+"/"+instance) + task, Status: status, Attempts: attempts, Accepted: accepted}}}
	}
	yes := true
	cases := []NamedScenario{
		{Name: "Complete M1 journey", Description: "All five aspects, shared owners, automatic dispatch, faults, correlated acknowledgement, durable restarts and replay", Scenario: Scenario{Name: "M1 complete journey", Steps: []ScenarioStep{start(2, false, "waiting"), response("approve", "task1", true, "waiting"), deliver("workflow_deliver", "task2", "drop_before_receive", "waiting"), deliver("workflow_retry", "task2", "before_recipient_commit", "waiting"), deliver("workflow_retry", "task2", "after_recipient_commit_before_ack", "waiting"), {Kind: "restart", TaskID: ledger}, {Kind: "restart", TaskID: stock}, {Kind: "broker_restart", Message: &MessageCall{ChannelID: channel}}, restart("waiting"), deliver("workflow_retry", "task2", "", "completed"), deliver("workflow_retry", "task2", "", "completed"), restart("completed"), start(2, false, "completed")}, Expected: expected(3, 2, 1, 1, 1, 0, 0), ExpectedWorkflows: final("completed", "reservationRecorded", "", 3, true, true), ExpectedMessages: message("task2", "acknowledged", 4, &yes)}},
		{Name: "Lost acknowledgement stays waiting", Description: "The recipient commits, but the workflow cannot claim completion without its correlated receipt", Scenario: Scenario{Name: "M1 pending acknowledgement", Steps: []ScenarioStep{start(2, false, "waiting"), response("approve", "task1", true, "waiting"), deliver("workflow_deliver", "task2", "after_recipient_commit_before_ack", "waiting")}, Expected: expected(3, 2, 1, 1, 1, 1, 1), ExpectedWorkflows: final("waiting", "", "handoff", 3, true, false), ExpectedMessages: message("task2", "pending", 1, nil)}},
		{Name: "Stock rejection", Description: "Business rejection follows the declared branch and creates no delivery", Scenario: Scenario{Name: "M1 stock rejection", Steps: []ScenarioStep{start(99, false, "waiting"), response("approve", "task1", true, "completed")}, Expected: expected(5, 0, 1, 0, 0, 0, 0), ExpectedWorkflows: final("completed", "reservationRejected", "", 5, false, false)}},
		{Name: "Atomic rollback", Description: "A fault before the source commit follows the error branch, with no state write, receipt or intent", Scenario: Scenario{Name: "M1 rollback", Steps: []ScenarioStep{start(2, true, "waiting"), response("approve", "task1", true, "completed")}, Expected: expected(5, 0, 0, 0, 0, 0, 0), ExpectedWorkflows: final("completed", "reservationFailed", "", 0, false, false)}},
		{Name: "Review loop and duplicate event", Description: "A human decision loops to a new task; replaying the previous response does not complete the new task", Scenario: Scenario{Name: "M1 bounded review loop", Steps: []ScenarioStep{start(2, false, "waiting"), response("reviewAgain", "task1", false, "waiting"), response("reviewAgain", "task1", false, "waiting"), response("approve", "task2", true, "waiting"), deliver("workflow_deliver", "task3", "", "completed")}, Expected: expected(3, 2, 1, 1, 1, 1, 1), ExpectedWorkflows: final("completed", "reservationRecorded", "", 3, true, true), ExpectedMessages: message("task3", "acknowledged", 1, &yes)}},
		{Name: "Cancel and ignore late acknowledgement", Description: "A signal cancels the waiting workflow. Its existing intent can still be delivered, but the late receipt cannot reopen the cancelled workflow", Scenario: Scenario{Name: "M1 cancellation correlation", Steps: []ScenarioStep{start(2, false, "waiting"), response("approve", "task1", true, "waiting"), {Kind: "workflow_event", Workflow: &WorkflowCall{InstanceID: instance, Event: &model.WorkflowEvent{ID: "cancel", Kind: "signal", Signal: "cancel", Values: map[string]model.TypedValue{}}}, ExpectedWorkflowStatus: "completed"}, deliver("workflow_deliver", "task2", "", "completed")}, Expected: expected(3, 2, 1, 1, 1, 1, 1), ExpectedWorkflows: final("completed", "reservationCancelled", "", 3, true, false), ExpectedMessages: message("task2", "acknowledged", 1, &yes)}},
	}
	return cases, nil
}
