package system

import "strings"

// Capability names are versioned semantic promises, not claims about live services.
func ProviderCapabilities(p ExecutionProvider) []string {
	if p.Version != "1.0.0" {
		return nil
	}
	switch p.Kind {
	case "modeled-state":
		return []string{"typed-commands", "atomic-state", "revision-checks", "durable-receipts", "effect-intents", "owner-restart"}
	case "modeled-broker":
		return []string{"typed-routes", "frozen-envelopes", "explicit-delivery", "receipt-acknowledgement", "durable-restart"}
	case "modeled-workflow":
		return []string{"typed-inputs", "shared-owners", "decisions", "command-routing", "correlated-responses", "signals", "bounded-loops", "durable-checkpoint"}
	}
	return nil
}
func RequiredCapabilities(requirement string) []string {
	kind := strings.SplitN(requirement, "/", 2)[0]
	switch kind {
	case "state":
		return ProviderCapabilities(ExecutionProvider{Kind: "modeled-state", Version: "1.0.0"})
	case "messages":
		return ProviderCapabilities(ExecutionProvider{Kind: "modeled-broker", Version: "1.0.0"})
	case "workflow":
		return ProviderCapabilities(ExecutionProvider{Kind: "modeled-workflow", Version: "1.0.0"})
	}
	return nil
}

type ProviderCheck struct {
	ComponentID string   `json:"componentId"`
	Requirement string   `json:"requirement"`
	ProviderID  string   `json:"providerId"`
	Required    []string `json:"required"`
	Provided    []string `json:"provided"`
	Satisfied   bool     `json:"satisfied"`
}
type WorkflowOwnerBinding struct {
	Slot        string `json:"slot"`
	OwnerID     string `json:"ownerId"`
	ComponentID string `json:"componentId"`
	StateSlot   string `json:"stateSlot"`
}

// Adapter node completion is routed exclusively from its correlated broker receipt.
type WorkflowMessageBinding struct {
	Slot               string            `json:"slot"`
	NodeID             string            `json:"nodeId"`
	ChannelComponentID string            `json:"channelComponentId"`
	ChannelSlot        string            `json:"channelSlot"`
	RouteID            string            `json:"routeId"`
	SourceNodeID       string            `json:"sourceNodeId"`
	Response           map[string]string `json:"response"`
}
