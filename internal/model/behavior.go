package model

type TypedValue struct {
	Type    string  `json:"type"`
	Boolean *bool   `json:"boolean,omitempty"`
	Integer *int64  `json:"integer,omitempty"`
	String  *string `json:"string,omitempty"`
}

type BehaviorPort struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type BehaviorBinding struct {
	Input   string      `json:"input,omitempty"`
	Node    string      `json:"node,omitempty"`
	Port    string      `json:"port,omitempty"`
	Literal *TypedValue `json:"literal,omitempty"`
}

type BehaviorNode struct {
	ID        string                     `json:"id"`
	Op        string                     `json:"op,omitempty"`
	Component *ComponentRef              `json:"component,omitempty"`
	Inputs    map[string]BehaviorBinding `json:"inputs"`
	Position  Position                   `json:"position"`
}

type Behavior struct {
	Inputs   []BehaviorPort             `json:"inputs"`
	Outputs  []BehaviorPort             `json:"outputs"`
	Nodes    []BehaviorNode             `json:"nodes"`
	Bindings map[string]BehaviorBinding `json:"bindings"`
}

type BehaviorCase struct {
	Name     string                `json:"name"`
	Inputs   map[string]TypedValue `json:"inputs"`
	Expected map[string]TypedValue `json:"expected"`
}

type ComponentBundle struct {
	SchemaVersion int           `json:"schemaVersion"`
	Root          ComponentRef  `json:"root"`
	Packages      []PackageLock `json:"packages"`
}

type PrimitiveSpec struct {
	Op          string         `json:"op"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Inputs      []BehaviorPort `json:"inputs"`
	Outputs     []BehaviorPort `json:"outputs"`
}

const MaxBehaviorInteger int64 = 9007199254740991
