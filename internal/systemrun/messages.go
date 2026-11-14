package systemrun

import (
	"context"
	"crypto/sha256"
	"fmt"
	"software-studio/internal/delivery"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"strings"
)

const MessageVersion = "system-dataflow-4"

type MessageOwner struct {
	InstanceID string `json:"instanceId"`
	TaskID     string `json:"taskId"`
}
type MessageChannel struct {
	ID       string                   `json:"id"`
	Model    model.DeliveryModelLock  `json:"model"`
	Provider system.ExecutionProvider `json:"provider"`
	Owners   []MessageOwner           `json:"owners"`
}
type MessageCall struct {
	ChannelID        string `json:"channelId"`
	RouteID          string `json:"routeId,omitempty"`
	SourceCommandID  string `json:"sourceCommandId,omitempty"`
	SourceMessage    string `json:"sourceMessage,omitempty"`
	Alias            string `json:"alias,omitempty"`
	ExpectedRevision *int64 `json:"expectedRevision,omitempty"`
	Fault            string `json:"fault,omitempty"`
}
type MessageExpectation struct {
	ChannelID string `json:"channelId"`
	model.DeliveryMessageExpectation
}
type MessageEvidence struct {
	ChannelID string                   `json:"channelId"`
	Provider  system.ExecutionProvider `json:"provider"`
	Step      delivery.StepResult      `json:"step"`
	Messages  []delivery.Message       `json:"messages"`
}
type FinalMessages struct {
	ChannelID string             `json:"channelId"`
	Messages  []delivery.Message `json:"messages"`
}

func ownerID(id string) string { return fmt.Sprintf("owner%x", sha256.Sum256([]byte(id)))[:37] }
func messageStep(step ScenarioStep) model.DeliveryStep {
	c := step.Message
	k := step.Kind
	if k == "retry" {
		k = "deliver"
	}
	return model.DeliveryStep{Kind: k, RouteID: c.RouteID, SourceCommandID: c.SourceCommandID, SourceMessage: c.SourceMessage, Message: c.Alias, ExpectedRevision: c.ExpectedRevision, Fault: c.Fault}
}
func validateChannels(tasks map[string]Task, channels []MessageChannel) []model.ValidationIssue {
	issues := []model.ValidationIssue{}
	add := func(path, message string) {
		issues = append(issues, model.ValidationIssue{Path: path, Message: message})
	}
	if len(channels) > 16 {
		add("channels", "At most 16 message channels")
		return issues
	}
	ids := map[string]bool{}
	for i, c := range channels {
		path := fmt.Sprintf("channels[%d]", i)
		if c.ID == "" || ids[c.ID] {
			add(path, "Unique channel ID required")
		}
		ids[c.ID] = true
		if c.Provider.ID == "" || c.Provider.Kind != "modeled-broker" || c.Provider.Version != "1.0.0" {
			add(path, "Bind messages to modeled-broker@1.0.0")
		}
		for _, issue := range model.ValidateDeliveryModelLock(c.Model) {
			add(path+".model."+issue.Path, issue.Message)
		}
		owners := map[string]bool{}
		usedTasks := map[string]bool{}
		if len(c.Owners) != len(c.Model.Model.Instances) {
			add(path, "Map every message actor to one existing state owner")
		}
		for _, o := range c.Owners {
			t, ok := tasks[o.TaskID]
			match := false
			if ok && t.State != nil {
				for _, a := range c.Model.Model.Instances {
					if a.ID == o.InstanceID && a.Machine.MachineID == t.State.Machine.Machine.ID && a.Machine.Version == t.State.Machine.Machine.Version && a.Machine.Digest == t.State.Machine.Digest {
						match = true
					}
				}
			}
			if !match || owners[o.InstanceID] || usedTasks[o.TaskID] {
				add(path+".owners", "Actors must map one-to-one to state owners with exactly matching machine ID, version and digest")
			}
			owners[o.InstanceID] = true
			usedTasks[o.TaskID] = true
		}
	}
	return issues
}
func prepareMessages(ctx context.Context, plan Plan, scenario Scenario, runtime *scenarioRuntime) error {
	runtime.brokers = map[string]*delivery.SharedSession{}
	transport := map[string]model.DeliveryScenario{}
	for _, c := range plan.Channels {
		s := model.DeliveryScenario{SchemaVersion: 1, Name: scenario.Name, Initial: []model.DeliveryInitial{}, Steps: []model.DeliveryStep{}, Expected: []model.StateExpectation{}, ExpectedMessages: []model.DeliveryMessageExpectation{}}
		owners := map[string]delivery.SharedOwner{}
		for _, o := range c.Owners {
			owners[o.InstanceID] = delivery.SharedOwner{Session: runtime.sessions[o.TaskID], InstanceID: ownerID(o.TaskID)}
			s.Initial = append(s.Initial, model.DeliveryInitial{InstanceID: o.InstanceID, Initial: map[string]model.TypedValue{}})
		}
		broker, err := delivery.NewSharedSession(ctx, c.Model, c.ID, owners)
		if err != nil {
			return err
		}
		runtime.brokers[c.ID] = broker
		transport[c.ID] = s
	}
	for i, step := range scenario.Steps {
		isMessage := step.Kind == "dispatch" || step.Kind == "deliver" || step.Kind == "retry" || step.Kind == "broker_restart"
		invalid := func(message string) error {
			return &Invalid{[]model.ValidationIssue{{Path: fmt.Sprintf("scenario.steps[%d]", i), Message: message}}}
		}
		if len(plan.Channels) > 0 {
			for _, call := range step.Calls {
				if strings.HasPrefix(call.CommandID, "msg_") {
					return invalid("The msg_ command identity prefix is reserved for broker deliveries")
				}
			}
		}
		if !isMessage {
			if step.Message != nil || step.ExpectedMessageStatus != "" {
				return invalid("Only message steps accept transport fields")
			}
			continue
		}
		if step.Message == nil || runtime.brokers[step.Message.ChannelID] == nil || step.TaskID != "" || len(step.Inputs) > 0 || len(step.Calls) > 0 || len(step.ExpectedStatuses) > 0 {
			return invalid("Choose an existing channel; message steps cannot contain graph or owner calls")
		}
		if step.Kind == "broker_restart" {
			c := *step.Message
			c.ChannelID = ""
			if c != (MessageCall{}) {
				return invalid("Broker restart accepts only a channel ID")
			}
			if step.ExpectedMessageStatus != "" && step.ExpectedMessageStatus != "broker_restarted" {
				return invalid("Expected broker restart outcome must be broker_restarted")
			}
			continue
		}
		switch step.ExpectedMessageStatus {
		case "", "dispatched", "dispatch_replayed", "dispatch_conflict", "missing_intent", "missing_source_message", "missing_message", "acknowledged", "acknowledgement_lost", "recipient_unacknowledged", "dropped", "already_acknowledged":
		default:
			return invalid("Unknown transport outcome")
		}
		s := transport[step.Message.ChannelID]
		s.Steps = append(s.Steps, messageStep(step))
		transport[step.Message.ChannelID] = s
	}
	for _, e := range scenario.ExpectedMessages {
		s, ok := transport[e.ChannelID]
		if !ok {
			return &Invalid{[]model.ValidationIssue{{Path: "scenario.expectedMessages", Message: "Unknown channel"}}}
		}
		if len(plan.Workflows) == 0 || !strings.HasPrefix(e.Name, "wfs_") {
			s.ExpectedMessages = append(s.ExpectedMessages, e.DeliveryMessageExpectation)
		}
		transport[e.ChannelID] = s
	}
	for _, c := range plan.Channels {
		s := transport[c.ID]
		if len(s.Steps) == 0 {
			if len(s.ExpectedMessages) > 0 {
				return fmt.Errorf("message assertions require a transport step")
			}
			continue
		}
		if issues := model.ValidateDeliveryScenario(c.Model, s); len(issues) > 0 {
			return &Invalid{issues}
		}
	}
	return nil
}
func runMessageStep(ctx context.Context, plan Plan, step ScenarioStep, runtime *scenarioRuntime) ([]Event, string, error) {
	var channel MessageChannel
	for _, c := range plan.Channels {
		if c.ID == step.Message.ChannelID {
			channel = c
		}
	}
	broker := runtime.brokers[channel.ID]
	result := delivery.StepResult{Index: runtime.index, Kind: step.Kind, Status: "broker_restarted"}
	if step.Kind != "broker_restart" {
		var err error
		result, err = broker.Apply(runtime.index, messageStep(step))
		if err != nil {
			return nil, "", err
		}
	}
	evidence := &MessageEvidence{ChannelID: channel.ID, Provider: channel.Provider, Step: result, Messages: broker.Messages()}
	source, target := "", ""
	for _, m := range evidence.Messages {
		if m.Name == step.Message.Alias {
			for _, o := range channel.Owners {
				if o.InstanceID == m.SourceInstanceID {
					source = o.TaskID
				}
				if o.InstanceID == m.TargetInstanceID {
					target = o.TaskID
				}
			}
		}
	}
	if source == "" {
		for _, r := range channel.Model.Model.Routes {
			if r.ID == step.Message.RouteID {
				for _, o := range channel.Owners {
					if o.InstanceID == r.SourceInstanceID {
						source = o.TaskID
					}
					if o.InstanceID == r.TargetInstanceID {
						target = o.TaskID
					}
				}
			}
		}
	}
	events := []Event{}
	emit := func(kind string) {
		events = append(events, Event{Kind: kind, TaskID: source, Target: target, Message: evidence, Values: map[string]model.TypedValue{}})
	}
	if result.State != nil {
		emit("delivered")
		if result.State.Status == "committed" {
			emit("recipient_committed")
			if result.State.Receipt != nil && len(result.State.Receipt.EffectIDs) > 0 {
				events = append(events, Event{Kind: "intent_created", TaskID: target, IntentIDs: result.State.Receipt.EffectIDs, Values: map[string]model.TypedValue{}})
			}
		} else if result.State.Status == "replayed" {
			emit("recipient_replayed")
		}
	}
	emit(result.Status)
	return events, result.Status, ctx.Err()
}
