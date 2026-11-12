package model

// Stateful machines are separate immutable artifacts. Their fields never alter
// the serialization or evaluation semantics of pure component packages.
type StateSlot struct {
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	RecordRef  *DataTypeRef `json:"recordRef,omitempty"`
	Durability string       `json:"durability"`
	Initial    TypedValue   `json:"initial"`
}
type StateEffect struct {
	Name    string         `json:"name"`
	Payload []BehaviorPort `json:"payload"`
}
type StateBinding struct {
	State   string      `json:"state,omitempty"`
	Input   string      `json:"input,omitempty"`
	Literal *TypedValue `json:"literal,omitempty"`
}
type StateEffectBinding struct {
	Effect     string            `json:"effect"`
	WhenOutput string            `json:"whenOutput,omitempty"`
	Payload    map[string]string `json:"payload"`
}
type StateCommand struct {
	Name           string                  `json:"name"`
	Inputs         []BehaviorPort          `json:"inputs"`
	Outputs        []BehaviorPort          `json:"outputs"`
	Decision       ComponentBundle         `json:"decision"`
	Bindings       map[string]StateBinding `json:"bindings"`
	AcceptedOutput string                  `json:"acceptedOutput"`
	Writes         map[string]string       `json:"writes"`
	Results        map[string]string       `json:"results"`
	Effects        []StateEffectBinding    `json:"effects"`
}
type StateMachine struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	Version       string             `json:"version"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Author        string             `json:"author"`
	License       string             `json:"license"`
	DataContracts []DataContractLock `json:"dataContracts"`
	State         []StateSlot        `json:"state"`
	Effects       []StateEffect      `json:"effects"`
	Commands      []StateCommand     `json:"commands"`
}
type StateMachineLock struct {
	Machine StateMachine `json:"machine"`
	Digest  string       `json:"digest"`
}
type StateInstance struct {
	ID      string                `json:"id"`
	Name    string                `json:"name"`
	Initial map[string]TypedValue `json:"initial"`
}
type StateStep struct {
	Kind             string                `json:"kind"`
	InstanceID       string                `json:"instanceId"`
	CommandID        string                `json:"commandId,omitempty"`
	Command          string                `json:"command,omitempty"`
	ExpectedRevision *int64                `json:"expectedRevision,omitempty"`
	Inputs           map[string]TypedValue `json:"inputs"`
	Fault            string                `json:"fault,omitempty"`
}
type StateExpectation struct {
	InstanceID string                `json:"instanceId"`
	State      map[string]TypedValue `json:"state"`
	Revision   int64                 `json:"revision"`
	Receipts   int                   `json:"receipts"`
	Effects    int                   `json:"effects"`
}
type StateScenario struct {
	SchemaVersion int                `json:"schemaVersion"`
	Name          string             `json:"name"`
	Instances     []StateInstance    `json:"instances"`
	Steps         []StateStep        `json:"steps"`
	Expected      []StateExpectation `json:"expected"`
}
type StateExample struct {
	Machine  StateMachineLock `json:"machine"`
	Scenario StateScenario    `json:"scenario"`
}
