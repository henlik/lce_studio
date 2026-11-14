package system

import "software-studio/internal/model"

type MessageExamplePack struct {
	Design      SystemDesign            `json:"design"`
	Model       model.DeliveryModelLock `json:"model"`
	Definitions []ComponentDefinition   `json:"definitions"`
}

func MessageExecutionExample() MessageExamplePack {
	lock := model.ExampleDeliveryScenarios()[0].Model
	stock := lock.Model.Machines[0]
	stock.Machine.ID = "studio.message-stock-owner"
	five := int64(5)
	for i := range stock.Machine.State {
		if stock.Machine.State[i].Name == "available" {
			stock.Machine.State[i].Initial = model.TypedValue{Type: "integer", Integer: &five}
		}
	}
	stock.Digest, _ = model.StateMachineDigest(stock.Machine)
	lock.Model.Machines[0] = stock
	lock.Model.ID = "studio.stock-ledger-delivery"
	lock.Model.Instances[0].Machine = model.DeliveryMachineRef{MachineID: stock.Machine.ID, Version: stock.Machine.Version, Digest: stock.Digest}
	lock.Digest, _ = model.DeliveryModelDigest(lock.Model)
	ledger := lock.Model.Machines[1]
	defs := []ComponentDefinition{
		{SchemaVersion: 1, ID: "studio.message-stock", Version: "1.0.0", Name: "Stock with reservation messages", Aspects: []AspectRef{
			{Slot: "stock", Kind: "state", ID: stock.Machine.ID, Version: stock.Machine.Version, Digest: stock.Digest},
			{Slot: "notices", Kind: "messages", ID: lock.Model.ID, Version: lock.Model.Version, Digest: lock.Digest}}},
		{SchemaVersion: 1, ID: "studio.message-ledger", Version: "1.0.0", Name: "Reservation ledger", Aspects: []AspectRef{{Slot: "ledger", Kind: "state", ID: ledger.Machine.ID, Version: ledger.Machine.Version, Digest: ledger.Digest}}},
	}
	d := SystemDesign{SchemaVersion: 1, ID: "stock-ledger-messages", Name: "Stock owner → reservation ledger", Revision: "1.0.0", Connections: []Connection{}, DeploymentPlans: []DeploymentPlan{}, ExecutionProviders: []ExecutionProvider{{ID: "owned-state", Kind: "modeled-state", Version: "1.0.0"}, {ID: "reservation-broker", Kind: "modeled-broker", Version: "1.0.0"}}, Contract: Contract{ID: "stock-ledger", Revision: "1", Claims: []Claim{{ID: "evidence", Kind: "required_evidence", Description: "Declared only; this model experiment checks scenario expectations, not contract claims"}}, Assumptions: []Assumption{}}, Components: []ComponentInstance{
		{ID: "stock", Name: "Stock owner", Definition: DefinitionRef{Kind: "definition", ID: defs[0].ID, Version: "1.0.0"}, Artifacts: []ArtifactRef{}, Bindings: []Binding{{Requirement: "state/stock", ResourceID: "owned-state"}, {Requirement: "messages/notices", ResourceID: "reservation-broker"}}, MessageOwners: []MessageOwner{{Slot: "notices", InstanceID: "stock", ComponentID: "stock", StateSlot: "stock"}, {Slot: "notices", InstanceID: "ledger", ComponentID: "ledger", StateSlot: "ledger"}}},
		{ID: "ledger", Name: "Reservation ledger", Definition: DefinitionRef{Kind: "definition", ID: defs[1].ID, Version: "1.0.0"}, Artifacts: []ArtifactRef{}, Bindings: []Binding{{Requirement: "state/ledger", ResourceID: "owned-state"}}},
	}}
	return MessageExamplePack{Design: d, Model: lock, Definitions: defs}
}
