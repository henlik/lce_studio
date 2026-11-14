package systemrun

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/workflow"
)

const UnifiedVersion = "system-dataflow-5"

type WorkflowOwner struct {
	OwnerID string `json:"ownerId"`
	TaskID  string `json:"taskId"`
}
type WorkflowRoute struct {
	NodeID       string            `json:"nodeId"`
	ChannelID    string            `json:"channelId"`
	RouteID      string            `json:"routeId"`
	SourceNodeID string            `json:"sourceNodeId"`
	Response     map[string]string `json:"response"`
}
type WorkflowTask struct {
	ID       string                   `json:"id"`
	Name     string                   `json:"name"`
	Lock     model.WorkflowLock       `json:"lock"`
	Provider system.ExecutionProvider `json:"provider"`
	Owners   []WorkflowOwner          `json:"owners"`
	Routes   []WorkflowRoute          `json:"routes"`
}
type WorkflowCall struct {
	WorkflowID string                      `json:"workflowId,omitempty"`
	InstanceID string                      `json:"instanceId"`
	Inputs     map[string]model.TypedValue `json:"inputs"`
	Faults     map[string]string           `json:"faults,omitempty"`
	TaskID     string                      `json:"taskId,omitempty"`
	Fault      string                      `json:"fault,omitempty"`
	Event      *model.WorkflowEvent        `json:"event,omitempty"`
}
type WorkflowEvidence struct {
	WorkflowID  string                      `json:"workflowId"`
	InstanceID  string                      `json:"instanceId"`
	Status      string                      `json:"status"`
	Outcome     string                      `json:"outcome"`
	CurrentNode string                      `json:"currentNode"`
	Task        *workflow.Task              `json:"task,omitempty"`
	Step        *workflow.StepResult        `json:"step,omitempty"`
	Event       *workflow.EventResult       `json:"event,omitempty"`
	Variables   map[string]model.TypedValue `json:"variables"`
	Outputs     map[string]model.TypedValue `json:"outputs"`
}
type FinalWorkflow struct {
	WorkflowID string          `json:"workflowId"`
	InstanceID string          `json:"instanceId"`
	Result     workflow.Result `json:"result"`
}
type WorkflowExpectation struct {
	InstanceID     string                      `json:"instanceId"`
	Status         string                      `json:"status"`
	Outcome        string                      `json:"outcome"`
	ActiveTaskNode string                      `json:"activeTaskNode"`
	Variables      map[string]model.TypedValue `json:"variables"`
	Outputs        map[string]model.TypedValue `json:"outputs"`
}
type pendingWorkflow struct {
	route  WorkflowRoute
	alias  string
	taskID string
}
type runningWorkflow struct {
	task        WorkflowTask
	session     *workflow.SharedSession
	fingerprint string
	pending     map[string]pendingWorkflow
	emitted     int
}

var workflowInstanceID = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

func workflowNamespace(id string) string {
	return fmt.Sprintf("wfs_%x_", sha256.Sum256([]byte(id)))[:37]
}
func PrepareUnified(tasks []Task, links []Link, channels []MessageChannel, flows []WorkflowTask) (Plan, error) {
	p, err := Prepare(tasks, links, channels...)
	if len(tasks) == 0 && len(flows) > 0 && len(links) == 0 {
		p = Plan{EngineVersion: "system-pure-1", Tasks: []Task{}, Links: []Link{}, Order: []string{}, Inputs: []RequiredInput{}, Channels: channels}
		err = nil
		if issues := validateChannels(map[string]Task{}, channels); len(issues) > 0 {
			err = &Invalid{issues}
		}
	}
	if err != nil {
		return p, err
	}
	if len(flows) == 0 {
		return p, nil
	}
	issues := []model.ValidationIssue{}
	add := func(path, message string) {
		issues = append(issues, model.ValidationIssue{Path: path, Message: message})
	}
	if len(flows) > 16 {
		add("workflows", "At most 16 workflow aspects")
	}
	ids := map[string]bool{}
	for i, f := range flows {
		path := fmt.Sprintf("workflows[%d]", i)
		if f.ID == "" || ids[f.ID] {
			add(path, "Unique workflow identity required")
		}
		ids[f.ID] = true
		if f.Provider.ID == "" || f.Provider.Kind != "modeled-workflow" || f.Provider.Version != "1.0.0" {
			add(path+".provider", "Bind workflow to modeled-workflow@1.0.0")
		}
		if v := model.ValidateWorkflowLock(f.Lock); len(v) > 0 {
			for _, issue := range v {
				add(path+".lock."+issue.Path, issue.Message)
			}
			continue
		}
		if len(f.Owners) != len(f.Lock.Workflow.Owners) {
			add(path+".owners", "Map every workflow owner to an existing system owner")
		}
		seen := map[string]bool{}
		used := map[string]bool{}
		for _, o := range f.Owners {
			match := false
			for _, decl := range f.Lock.Workflow.Owners {
				if decl.ID == o.OwnerID {
					if len(decl.Initial) > 0 {
						add(path+".owners", "Shared owners cannot be reinitialized by a workflow")
					}
					for _, t := range tasks {
						if t.ID == o.TaskID && t.State != nil && decl.Machine.MachineID == t.State.Machine.Machine.ID && decl.Machine.Version == t.State.Machine.Machine.Version && decl.Machine.Digest == t.State.Machine.Digest {
							match = true
						}
					}
				}
			}
			if !match || seen[o.OwnerID] || used[o.TaskID] {
				add(path+".owners", "Workflow owners require one-to-one exact machine pins")
			}
			seen[o.OwnerID] = true
			used[o.TaskID] = true
		}
		nodes := map[string]model.WorkflowNode{}
		for _, n := range f.Lock.Workflow.Nodes {
			nodes[n.ID] = n
		}
		routed := map[string]bool{}
		for _, r := range f.Routes {
			node := nodes[r.NodeID]
			source := nodes[r.SourceNodeID]
			if routed[r.NodeID] || node.Task == nil || node.Task.Kind != "adapter" || source.Command == nil {
				add(path+".routes", "Bind each adapter task once, using a declared source command node")
				continue
			}
			routed[r.NodeID] = true
			if len(node.Task.Request) > 0 {
				add(path+".routes", "Broker task payload comes from the committed intent; task request bindings must be empty")
			}
			var channel *MessageChannel
			for j := range channels {
				if channels[j].ID == r.ChannelID {
					channel = &channels[j]
				}
			}
			if channel == nil {
				add(path+".routes", "Choose an existing message channel")
				continue
			}
			var route *model.DeliveryRoute
			for j := range channel.Model.Model.Routes {
				if channel.Model.Model.Routes[j].ID == r.RouteID {
					route = &channel.Model.Model.Routes[j]
				}
			}
			if route == nil {
				add(path+".routes", "Choose an existing message route")
				continue
			}
			sourceTask := ""
			for _, o := range f.Owners {
				if o.OwnerID == source.Command.Owner {
					sourceTask = o.TaskID
				}
			}
			routeSource := ""
			for _, o := range channel.Owners {
				if o.InstanceID == route.SourceInstanceID {
					routeSource = o.TaskID
				}
			}
			if sourceTask == "" || sourceTask != routeSource {
				add(path+".routes", "Workflow command and message source must use the same state owner")
			}
			produces := false
			for _, t := range tasks {
				if t.ID == sourceTask && t.State != nil {
					for _, c := range t.State.Machine.Machine.Commands {
						if c.Name == source.Command.Command {
							for _, effect := range c.Effects {
								if effect.Effect == route.Effect {
									produces = true
								}
							}
						}
					}
				}
			}
			if !produces {
				add(path+".routes", "Selected command must declare the route's source effect")
			}
			ports := []model.BehaviorPort{}
			for _, a := range channel.Model.Model.Instances {
				if a.ID == route.TargetInstanceID {
					for _, m := range channel.Model.Model.Machines {
						if m.Digest == a.Machine.Digest {
							for _, c := range m.Machine.Commands {
								if c.Name == route.Command {
									ports = c.Outputs
								}
							}
						}
					}
				}
			}
			if len(r.Response) != len(node.Task.Response) {
				add(path+".routes.response", "Map every response field from an acknowledged recipient output")
			}
			for _, port := range node.Task.Response {
				match := false
				for _, p := range ports {
					if p.Name == r.Response[port.Name] && model.SameBehaviorType(model.PortBehaviorType(p), model.PortBehaviorType(port)) {
						match = true
					}
				}
				if !match {
					add(path+".routes.response", "Response fields must match exact recipient output types")
				}
			}
		}
		for _, n := range f.Lock.Workflow.Nodes {
			if n.Task != nil && n.Task.Kind == "adapter" && !routed[n.ID] {
				add(path+".routes", "Every adapter task needs a typed provider route: "+n.ID)
			}
		}
	}
	if len(issues) > 0 {
		return p, &Invalid{issues}
	}
	p.Workflows = flows
	p.EngineVersion = UnifiedVersion
	b, err := json.Marshal(p)
	if err != nil {
		return p, err
	}
	if len(b) > MaxBytes {
		return p, &Invalid{[]model.ValidationIssue{{Path: "workflows", Message: "Portable plan exceeds 4 MiB"}}}
	}
	return p, nil
}
func workflowByID(p Plan, id string) (WorkflowTask, bool) {
	for _, f := range p.Workflows {
		if f.ID == id {
			return f, true
		}
	}
	return WorkflowTask{}, false
}
func validateWorkflowSteps(p Plan, s Scenario) error {
	started := map[string]WorkflowTask{}
	invalid := func(path, message string) error {
		return &Invalid{[]model.ValidationIssue{{Path: path, Message: message}}}
	}
	for i, step := range s.Steps {
		path := fmt.Sprintf("scenario.steps[%d]", i)
		c := step.Workflow
		is := step.Kind == "workflow_start" || step.Kind == "workflow_event" || step.Kind == "workflow_deliver" || step.Kind == "workflow_retry" || step.Kind == "workflow_restart"
		if len(p.Workflows) > 0 {
			for _, call := range step.Calls {
				if len(call.CommandID) >= 4 && call.CommandID[:4] == "wfs_" {
					return invalid(path, "wfs_ identities are reserved for workflow commands")
				}
			}
		}
		if !is {
			if c != nil || step.ExpectedWorkflowStatus != "" {
				return invalid(path, "Workflow fields require a workflow step")
			}
			continue
		}
		if c == nil || !workflowInstanceID.MatchString(c.InstanceID) || step.Message != nil || step.TaskID != "" || len(step.Calls) > 0 || len(step.Inputs) > 0 || len(step.ExpectedStatuses) > 0 || step.ExpectedMessageStatus != "" {
			return invalid(path, "Workflow step requires a valid instance identity and only workflow fields")
		}
		if step.ExpectedWorkflowStatus != "" && step.ExpectedWorkflowStatus != "waiting" && step.ExpectedWorkflowStatus != "completed" && step.ExpectedWorkflowStatus != "failed" {
			return invalid(path, "Expected workflow status must be waiting, completed or failed")
		}
		if step.Kind == "workflow_start" {
			f, ok := workflowByID(p, c.WorkflowID)
			if !ok || c.TaskID != "" || c.Fault != "" || c.Event != nil {
				return invalid(path, "Select a pinned workflow and supply only start inputs/faults")
			}
			validation := model.WorkflowScenario{SchemaVersion: 1, Name: s.Name, Inputs: c.Inputs, Events: []model.WorkflowEvent{}}
			if issues := model.ValidateWorkflowScenario(f.Lock, validation); len(issues) > 0 {
				return &Invalid{issues}
			}
			for node, fault := range c.Faults {
				valid := false
				for _, n := range f.Lock.Workflow.Nodes {
					if n.ID == node && n.Command != nil && fault == "before_commit" {
						valid = true
					}
				}
				if !valid {
					return invalid(path, "Workflow command faults must name command nodes and use before_commit")
				}
			}
			if old, ok := started[c.InstanceID]; ok && old.ID != f.ID {
				return invalid(path, "Instance identity cannot name another workflow")
			}
			started[c.InstanceID] = f
		} else {
			if _, ok := started[c.InstanceID]; !ok {
				return invalid(path, "Start this workflow instance in an earlier step")
			}
			if c.WorkflowID != "" || c.Inputs != nil || len(c.Faults) > 0 {
				return invalid(path, "Only start steps supply definitions and inputs")
			}
			switch step.Kind {
			case "workflow_event":
				if c.Event == nil || c.TaskID != "" || c.Fault != "" {
					return invalid(path, "Supply only a workflow event")
				}
				if len(c.Event.ID) > 64 || !workflowInstanceID.MatchString(c.Event.ID) || len(c.Event.ID) >= 4 && c.Event.ID[:4] == "ack_" {
					return invalid(path, "Event ID must be local; ack_ is reserved")
				}
				if c.Event.Kind != "response" && c.Event.Kind != "signal" {
					return invalid(path, "Choose response or signal")
				}
			case "workflow_restart":
				if c.Event != nil || c.TaskID != "" || c.Fault != "" {
					return invalid(path, "Restart accepts only the instance identity")
				}
			default:
				if c.Event != nil || c.TaskID == "" {
					return invalid(path, "Delivery requires the exact pending workflow task ID")
				}
				if c.Fault != "" && c.Fault != "drop_before_receive" && c.Fault != "before_recipient_commit" && c.Fault != "after_recipient_commit_before_ack" {
					return invalid(path, "Unsupported delivery fault")
				}
			}
		}
	}
	messageExpectations := map[string]bool{}
	for _, e := range s.ExpectedMessages {
		key := e.ChannelID + "/" + e.Name
		if messageExpectations[key] {
			return invalid("scenario.expectedMessages", "Message expectations must be unique per channel and alias")
		}
		messageExpectations[key] = true
		if len(p.Workflows) > 0 && len(e.Name) >= 4 && e.Name[:4] == "wfs_" {
			known := false
			for id, f := range started {
				prefix := workflowNamespace(f.ID+"/"+id) + "task"
				for n := 1; n <= 64; n++ {
					if e.Name == fmt.Sprintf("%s%d", prefix, n) {
						known = true
					}
				}
			}
			valid := known && e.Attempts >= 0 && e.Attempts <= 64
			switch e.Status {
			case "absent":
				valid = valid && e.Attempts == 0 && e.Accepted == nil
			case "pending":
				valid = valid && e.Accepted == nil
			case "acknowledged":
				valid = valid && e.Attempts > 0
			default:
				valid = false
			}
			if !valid {
				return invalid("scenario.expectedMessages", "Workflow message assertions require a known task alias, valid status and explicit attempts")
			}
		}
	}
	if len(started) > 8 {
		return invalid("scenario.steps", "At most 8 workflow instances")
	}
	seen := map[string]bool{}
	for _, e := range s.ExpectedWorkflows {
		f, ok := started[e.InstanceID]
		if !ok || seen[e.InstanceID] {
			return invalid("scenario.expectedWorkflows", "Expect one entry per started workflow instance")
		}
		seen[e.InstanceID] = true
		if e.Status != "waiting" && e.Status != "completed" && e.Status != "failed" {
			return invalid("scenario.expectedWorkflows", "Choose a terminal or waiting status")
		}
		ctx, _ := model.BuildWorkflowContext(f.Lock.Workflow)
		variables := []model.BehaviorPort{}
		for _, v := range f.Lock.Workflow.Variables {
			if _, ok := e.Variables[v.Name]; ok {
				variables = append(variables, model.BehaviorPort{Name: v.Name, Type: v.Type, RecordRef: v.RecordRef})
			}
		}
		outputs := []model.BehaviorPort{}
		for _, v := range f.Lock.Workflow.Outputs {
			if _, ok := e.Outputs[v.Name]; ok {
				outputs = append(outputs, v)
			}
		}
		for _, pair := range []struct {
			values map[string]model.TypedValue
			ports  []model.BehaviorPort
		}{{e.Variables, variables}, {e.Outputs, outputs}} {
			if issues := model.ValidateStructuredBehaviorValues(pair.values, pair.ports, ctx.Contracts); len(issues) > 0 {
				return &Invalid{issues}
			}
		}
	}
	return nil
}
func workflowEvidence(id string, running *runningWorkflow, r workflow.Result) *WorkflowEvidence {
	return &WorkflowEvidence{WorkflowID: running.task.ID, InstanceID: id, Status: r.Status, Outcome: r.Outcome, CurrentNode: r.CurrentNode, Task: r.Task, Variables: r.Variables, Outputs: r.Outputs}
}
func emitWorkflow(id string, running *runningWorkflow, r workflow.Result, kind string, events *[]Event, p Plan) {
	for _, step := range r.Steps[running.emitted:] {
		evidence := workflowEvidence(id, running, r)
		v := step
		evidence.Step = &v
		event := Event{Kind: "workflow_" + step.Kind, TaskID: running.task.ID, Workflow: evidence, Values: step.Assignments}
		if step.State != nil {
			event.Kind = "completed"
			for _, o := range running.task.Owners {
				if ownerID(o.TaskID) == step.State.InstanceID {
					event.TaskID = o.TaskID
					for _, t := range p.Tasks {
						if t.ID == o.TaskID {
							for _, owner := range r.Owners {
								if owner.ID == step.State.InstanceID {
									event.State = &StateEvidence{Provider: t.State.Provider, Step: *step.State, Instance: owner, Command: workflowCommandName(running.task, step.NodeID)}
								}
							}
						}
					}
				}
			}
		}
		*events = append(*events, event)
	}
	running.emitted = len(r.Steps)
	evidence := workflowEvidence(id, running, r)
	if kind == "workflow_event" || kind == "workflow_resumed" {
		if len(r.Events) > 0 {
			last := r.Events[len(r.Events)-1]
			evidence.Event = &last
		}
	}
	*events = append(*events, Event{Kind: kind, TaskID: running.task.ID, Workflow: evidence, Values: r.Outputs})
}
func runWorkflowStep(ctx context.Context, p Plan, step ScenarioStep, runtime *scenarioRuntime) ([]Event, workflow.Result, error) {
	c := step.Workflow
	events := []Event{}
	running := runtime.workflows[c.InstanceID]
	kind := step.Kind
	if kind == "workflow_start" {
		encoded, _ := json.Marshal(c)
		fingerprint := string(encoded)
		if running != nil {
			if running.fingerprint != fingerprint {
				return nil, workflow.Result{}, fmt.Errorf("workflow instance identity conflict")
			}
			r, err := running.session.Snapshot(ctx)
			if err != nil {
				return nil, workflow.Result{}, err
			}
			emitWorkflow(c.InstanceID, running, r, "workflow_replayed", &events, p)
			return events, r, err
		}
		f, _ := workflowByID(p, c.WorkflowID)
		owners := map[string]workflow.SharedOwner{}
		for _, o := range f.Owners {
			owners[o.OwnerID] = workflow.SharedOwner{Session: runtime.sessions[o.TaskID], InstanceID: ownerID(o.TaskID)}
		}
		session, err := workflow.NewSharedSession(ctx, f.Lock, c.Inputs, owners, runtime.budget, workflowNamespace(f.ID+"/"+c.InstanceID), c.Faults)
		if err != nil {
			return nil, workflow.Result{}, err
		}
		running = &runningWorkflow{task: f, session: session, fingerprint: fingerprint, pending: map[string]pendingWorkflow{}}
		runtime.workflows[c.InstanceID] = running
		runtime.workflowOrder = append(runtime.workflowOrder, c.InstanceID)
	} else if kind == "workflow_restart" {
		if _, err := running.session.Restart(ctx); err != nil {
			return nil, workflow.Result{}, err
		}
	} else if kind == "workflow_event" {
		if c.Event.Kind == "response" {
			if _, bound := running.pending[c.Event.TaskID]; bound {
				return nil, workflow.Result{}, &Invalid{[]model.ValidationIssue{{Path: "scenario.workflow.event", Message: "Broker tasks can only be completed by their correlated acknowledgement"}}}
			}
		}
		if _, err := running.session.Apply(ctx, *c.Event); err != nil {
			return nil, workflow.Result{}, err
		}
	} else {
		pending, ok := running.pending[c.TaskID]
		if !ok {
			return nil, workflow.Result{}, &Invalid{[]model.ValidationIssue{{Path: "scenario.workflow.taskId", Message: "No correlated dispatch exists for this workflow task"}}}
		}
		m := ScenarioStep{Kind: "deliver", Message: &MessageCall{ChannelID: pending.route.ChannelID, Alias: pending.alias, Fault: c.Fault}}
		part, _, err := runMessageStep(ctx, p, m, runtime)
		if err != nil {
			return nil, workflow.Result{}, err
		}
		events = append(events, part...)
		current, err := running.session.Snapshot(ctx)
		if err != nil {
			return nil, workflow.Result{}, err
		}
		for _, msg := range runtime.brokers[pending.route.ChannelID].Messages() {
			if msg.Name == pending.alias && msg.Acknowledgement != nil && current.Task != nil && current.Task.ID == pending.taskID {
				values := map[string]model.TypedValue{}
				for target, source := range pending.route.Response {
					values[target] = msg.Acknowledgement.Outputs[source]
				}
				event := model.WorkflowEvent{ID: "ack_" + msg.ID, Kind: "response", TaskID: pending.taskID, Values: values}
				if _, err = running.session.Apply(ctx, event); err != nil {
					return nil, workflow.Result{}, err
				}
				kind = "workflow_resumed"
			}
		}
	}
	result, err := running.session.Snapshot(ctx)
	if err != nil {
		return nil, workflow.Result{}, err
	}
	emitWorkflow(c.InstanceID, running, result, kind, &events, p)
	if result.Task != nil {
		for _, route := range running.task.Routes {
			if route.NodeID != result.Task.NodeID {
				continue
			}
			if _, ok := running.pending[result.Task.ID]; ok {
				continue
			}
			sourceID := ""
			for _, s := range result.Steps {
				if s.NodeID == route.SourceNodeID && s.State != nil {
					sourceID = s.State.CommandID
				}
			}
			if sourceID == "" {
				return nil, workflow.Result{}, fmt.Errorf("workflow route source command has not executed")
			}
			revision := int64(0)
			for _, channel := range p.Channels {
				if channel.ID == route.ChannelID {
					for _, r := range channel.Model.Model.Routes {
						if r.ID == route.RouteID {
							for _, o := range channel.Owners {
								if o.InstanceID == r.TargetInstanceID {
									revision, err = runtime.sessions[o.TaskID].Revision(ownerID(o.TaskID))
									if err != nil {
										return nil, workflow.Result{}, err
									}
								}
							}
						}
					}
				}
			}
			alias := workflowNamespace(running.task.ID+"/"+c.InstanceID) + result.Task.ID
			part, status, err := runMessageStep(ctx, p, ScenarioStep{Kind: "dispatch", Message: &MessageCall{ChannelID: route.ChannelID, RouteID: route.RouteID, SourceCommandID: sourceID, Alias: alias, ExpectedRevision: &revision}}, runtime)
			if err != nil {
				return nil, workflow.Result{}, err
			}
			events = append(events, part...)
			if status != "dispatched" && status != "dispatch_replayed" {
				return nil, workflow.Result{}, fmt.Errorf("workflow dispatch failed: %s", status)
			}
			running.pending[result.Task.ID] = pendingWorkflow{route: route, alias: alias, taskID: result.Task.ID}
		}
	}
	return events, result, nil
}
func matchWorkflowExpectation(e WorkflowExpectation, r workflow.Result) bool {
	node := ""
	if r.Task != nil {
		node = r.Task.NodeID
	}
	if r.Status != e.Status || r.Outcome != e.Outcome || node != e.ActiveTaskNode {
		return false
	}
	for k, v := range e.Variables {
		if !reflect.DeepEqual(v, r.Variables[k]) {
			return false
		}
	}
	for k, v := range e.Outputs {
		if !reflect.DeepEqual(v, r.Outputs[k]) {
			return false
		}
	}
	return true
}

func workflowCommandName(t WorkflowTask, id string) string {
	for _, n := range t.Lock.Workflow.Nodes {
		if n.ID == id && n.Command != nil {
			return n.Command.Command
		}
	}
	return ""
}
