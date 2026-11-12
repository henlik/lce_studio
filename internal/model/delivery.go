package model

type DeliveryMachineRef struct {
	MachineID string `json:"machineId"`
	Version   string `json:"version"`
	Digest    string `json:"digest"`
}
type DeliveryInstance struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Machine  DeliveryMachineRef `json:"machine"`
	Position Position           `json:"position"`
}
type DeliveryRoute struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	SourceInstanceID string            `json:"sourceInstanceId"`
	Effect           string            `json:"effect"`
	TargetInstanceID string            `json:"targetInstanceId"`
	Command          string            `json:"command"`
	Inputs           map[string]string `json:"inputs"`
}
type DeliveryModel struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	Version       string             `json:"version"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Author        string             `json:"author"`
	License       string             `json:"license"`
	Machines      []StateMachineLock `json:"machines"`
	Instances     []DeliveryInstance `json:"instances"`
	Routes        []DeliveryRoute    `json:"routes"`
}
type DeliveryModelLock struct {
	Model  DeliveryModel `json:"model"`
	Digest string        `json:"digest"`
}
type DeliveryInitial struct {
	InstanceID string                `json:"instanceId"`
	Initial    map[string]TypedValue `json:"initial"`
}
type DeliveryStep struct {
	Kind             string                `json:"kind"`
	InstanceID       string                `json:"instanceId,omitempty"`
	CommandID        string                `json:"commandId,omitempty"`
	Command          string                `json:"command,omitempty"`
	ExpectedRevision *int64                `json:"expectedRevision,omitempty"`
	Inputs           map[string]TypedValue `json:"inputs"`
	Fault            string                `json:"fault,omitempty"`
	RouteID          string                `json:"routeId,omitempty"`
	SourceCommandID  string                `json:"sourceCommandId,omitempty"`
	SourceMessage    string                `json:"sourceMessage,omitempty"`
	Message          string                `json:"message,omitempty"`
}
type DeliveryMessageExpectation struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
	Accepted *bool  `json:"accepted,omitempty"`
}
type DeliveryScenario struct {
	SchemaVersion    int                          `json:"schemaVersion"`
	Name             string                       `json:"name"`
	Initial          []DeliveryInitial            `json:"initial"`
	Steps            []DeliveryStep               `json:"steps"`
	Expected         []StateExpectation           `json:"expected"`
	ExpectedMessages []DeliveryMessageExpectation `json:"expectedMessages"`
}
type DeliveryExample struct {
	Model    DeliveryModelLock `json:"model"`
	Scenario DeliveryScenario  `json:"scenario"`
}
