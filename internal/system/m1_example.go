package system

import "software-studio/internal/model"

type M1ExamplePack struct {
	MessageExamplePack
	Workflow model.WorkflowLock     `json:"workflow"`
	Behavior model.ComponentPackage `json:"behavior"`
}

func M1Example() M1ExamplePack {
	e := MessageExecutionExample()
	stock := e.Model.Model.Machines[0]
	request := stock.Machine.Commands[0].Inputs[0]
	behavior := model.ComponentPackage{SchemaVersion: 3, ID: "studio.reservation-input", Version: "1.0.0", Name: "Typed reservation input", Description: "Reusable record pass-through for unified engineering acceptance", Author: "LCE Studio", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, DataContracts: stock.Machine.DataContracts, Behavior: &model.Behavior{Inputs: []model.BehaviorPort{request}, Outputs: []model.BehaviorPort{request}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{"request": {Input: "request"}}}}
	bundle, issues := model.PrepareComponentBundle(behavior, []model.PackageLock{})
	if len(issues) > 0 {
		panic(issues[0].Message)
	}
	no := false
	zero := int64(0)
	tenant, resource := "tenantDefault", "resourceDefault"
	initial := model.TypedValue{Type: "record", RecordRef: request.RecordRef, Record: map[string]model.TypedValue{"tenantId": {Type: "string", String: &tenant}, "resourceId": {Type: "string", String: &resource}, "quantity": {Type: "integer", Integer: &zero}}}
	boolean := func(name string) model.WorkflowVariable {
		return model.WorkflowVariable{Name: name, Type: "boolean", Initial: model.TypedValue{Type: "boolean", Boolean: &no}}
	}
	output := map[string]model.WorkflowBinding{"remaining": {Variable: "remaining"}, "recorded": {Variable: "acknowledged"}}
	end := func(id, outcome string) model.WorkflowNode {
		return model.WorkflowNode{ID: id, Name: outcome, Kind: "end", End: &model.WorkflowEnd{Outcome: outcome, Outputs: output}}
	}
	w := model.WorkflowDefinition{SchemaVersion: 1, ID: "studio.reservation-orchestration", Version: "1.0.0", Name: "Reservation orchestration", Description: "Review, reserve, deliver and resume on a correlated ledger acknowledgement", Author: "LCE Studio", License: "MIT", DataContracts: stock.Machine.DataContracts, Decisions: []model.ComponentBundle{bundle}, Machines: []model.StateMachineLock{stock}, Owners: []model.WorkflowOwner{{ID: "stock", Name: "Shared stock owner", Machine: model.DeliveryMachineRef{MachineID: stock.Machine.ID, Version: stock.Machine.Version, Digest: stock.Digest}, Initial: map[string]model.WorkflowBinding{}}}, Inputs: []model.BehaviorPort{request}, Outputs: []model.BehaviorPort{{Name: "remaining", Type: "integer"}, {Name: "recorded", Type: "boolean"}}, Variables: []model.WorkflowVariable{boolean("proceed"), boolean("accepted"), boolean("acknowledged"), {Name: "remaining", Type: "integer", Initial: model.TypedValue{Type: "integer", Integer: &zero}}, {Name: "request", Type: "record", RecordRef: request.RecordRef, Initial: initial}}, Entry: "review", Signals: []model.WorkflowSignal{{Name: "cancel", Inputs: []model.BehaviorPort{}, Set: map[string]model.WorkflowBinding{}, Target: "cancelled"}}, Nodes: []model.WorkflowNode{
		{ID: "review", Name: "Review request", Kind: "task", Stage: "Arrival", Task: &model.WorkflowTask{Kind: "human", Title: "Review reservation", Instructions: "Proceed to reserve, or return to review. Cancel is available while waiting.", Request: map[string]model.WorkflowBinding{"request": {Input: "request"}}, Response: []model.BehaviorPort{{Name: "proceed", Type: "boolean"}}, Assign: map[string]string{"proceed": "proceed"}}, Branch: &model.WorkflowBranch{Variable: "proceed", Then: "validate", Else: "review"}},
		{ID: "validate", Name: "Typed input decision", Kind: "decision", Stage: "Reserve", Decision: &model.WorkflowDecision{Component: bundle.Root, Inputs: map[string]model.WorkflowBinding{"request": {Input: "request"}}, Assign: map[string]string{"request": "request"}}, Next: "reserve"},
		{ID: "reserve", Name: "Reserve shared stock", Kind: "command", Stage: "Reserve", Command: &model.WorkflowCommand{Owner: "stock", Command: "reserve", Inputs: map[string]model.WorkflowBinding{"request": {Variable: "request"}}, Assign: map[string]string{"remaining": "remaining"}, AcceptedVariable: "accepted", ErrorNext: "failed"}, Branch: &model.WorkflowBranch{Variable: "accepted", Then: "handoff", Else: "rejected"}},
		{ID: "handoff", Name: "Await ledger acknowledgement", Kind: "task", Stage: "Ledger", Task: &model.WorkflowTask{Kind: "adapter", Title: "Record reservation", Instructions: "Only the correlated broker receipt can complete this task.", Request: map[string]model.WorkflowBinding{}, Response: []model.BehaviorPort{{Name: "accepted", Type: "boolean"}}, Assign: map[string]string{"acknowledged": "accepted"}}, Branch: &model.WorkflowBranch{Variable: "acknowledged", Then: "complete", Else: "ledgerRejected"}},
		end("complete", "reservationRecorded"), end("rejected", "reservationRejected"), end("failed", "reservationFailed"), end("cancelled", "reservationCancelled"), end("ledgerRejected", "ledgerRejected"),
	}}
	digest, err := model.WorkflowDigest(w)
	if err != nil {
		panic(err)
	}
	e.Design.ID = "m1-unified-reservation"
	e.Design.Name = "M1 acceptance · unified reservation"
	e.Design.Revision = "2.0.0"
	e.Definitions[0].Version = "2.0.0"
	e.Definitions[0].Aspects = append(e.Definitions[0].Aspects, AspectRef{Slot: "flow", Kind: "workflow", ID: w.ID, Version: w.Version, Digest: digest}, AspectRef{Slot: "input", Kind: "component", ID: bundle.Root.PackageID, Version: bundle.Root.Version, Digest: bundle.Root.Digest})
	data := stock.Machine.DataContracts[0]
	e.Definitions[0].Aspects = append(e.Definitions[0].Aspects, AspectRef{Slot: "data", Kind: "data", ID: data.Contract.ID, Version: data.Contract.Version, Digest: data.Digest})
	c := &e.Design.Components[0]
	c.Definition.Version = "2.0.0"
	c.Bindings = append(c.Bindings, Binding{Requirement: "workflow/flow", ResourceID: "workflow-model"})
	c.WorkflowOwners = []WorkflowOwnerBinding{{Slot: "flow", OwnerID: "stock", ComponentID: "stock", StateSlot: "stock"}}
	c.WorkflowMessages = []WorkflowMessageBinding{{Slot: "flow", NodeID: "handoff", ChannelComponentID: "stock", ChannelSlot: "notices", RouteID: "reservationToLedger", SourceNodeID: "reserve", Response: map[string]string{"accepted": "accepted"}}}
	e.Design.ExecutionProviders = append(e.Design.ExecutionProviders, ExecutionProvider{ID: "workflow-model", Kind: "modeled-workflow", Version: "1.0.0"})
	return M1ExamplePack{MessageExamplePack: e, Workflow: model.WorkflowLock{Workflow: w, Digest: digest}, Behavior: behavior}
}
