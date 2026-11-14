package delivery

import (
	"context"
	"fmt"
	"software-studio/internal/model"
	"software-studio/internal/state"
)

// SharedOwner attaches transport to an existing state session; it never creates
// another copy of the business owner. InstanceID is the session's private ID.
type SharedOwner struct {
	Session    *state.Session
	InstanceID string
}
type SharedSession struct {
	engine *engine
	owners map[string]SharedOwner
}

func NewSharedSession(ctx context.Context, lock model.DeliveryModelLock, namespace string, owners map[string]SharedOwner) (*SharedSession, error) {
	if issues := model.ValidateDeliveryModelLock(lock); len(issues) > 0 {
		return nil, fmt.Errorf("invalid message model: %s", issues[0].Message)
	}
	contracts, _ := model.DeliveryModelDataContracts(lock.Model)
	e := &engine{namespace: namespace, ctx: ctx, model: lock.Model, contracts: contracts, owners: map[string]*host{}, commands: map[string]map[string]model.StateCommand{}, routes: map[string]model.DeliveryRoute{}, byName: map[string]int{}, byIntentRoute: map[intentRoute]int{}, instanceIDs: map[string]string{}, result: Result{EngineVersion: Version, ModelHash: lock.Digest}}
	if len(owners) != len(lock.Model.Instances) {
		return nil, fmt.Errorf("every message actor requires exactly one shared owner")
	}
	for _, actor := range lock.Model.Instances {
		owner, ok := owners[actor.ID]
		if !ok || owner.Session == nil || owner.InstanceID == "" {
			return nil, fmt.Errorf("missing shared owner %s", actor.ID)
		}
		snapshots, err := owner.Session.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		found := false
		for _, s := range snapshots {
			found = found || s.ID == owner.InstanceID
		}
		if !found {
			return nil, fmt.Errorf("shared owner instance absent")
		}
		h := &host{session: owner.Session}
		e.hosts = append(e.hosts, h)
		e.owners[actor.ID] = h
		e.instanceIDs[actor.ID] = owner.InstanceID
		commands := map[string]model.StateCommand{}
		for _, machine := range lock.Model.Machines {
			if machine.Digest == actor.Machine.Digest {
				for _, c := range machine.Machine.Commands {
					commands[c.Name] = c
				}
			}
		}
		e.commands[actor.ID] = commands
	}
	for _, r := range lock.Model.Routes {
		e.routes[r.ID] = r
	}
	return &SharedSession{engine: e, owners: owners}, nil
}

// Apply receives a step from a previously validated whole transport scenario.
// Capacity checks refresh shared-owner sizes after intervening graph commands.
func (s *SharedSession) Apply(index int, step model.DeliveryStep) (StepResult, error) {
	e := s.engine
	if index < 0 || index >= 64 || len(e.result.Steps) >= 64 {
		return StepResult{}, fmt.Errorf("transport step limit")
	}
	if step.Kind != "dispatch" && step.Kind != "deliver" {
		return StepResult{}, fmt.Errorf("transport requires dispatch or deliver")
	}
	if step.Kind == "dispatch" {
		if _, ok := e.routes[step.RouteID]; !ok || step.ExpectedRevision == nil {
			return StepResult{}, fmt.Errorf("invalid dispatch route or revision")
		}
	}
	for _, h := range e.hosts {
		n, err := h.session.InstanceBytes()
		if err != nil {
			return StepResult{}, err
		}
		h.bytes = n
	}
	if err := e.step(index, step); err != nil {
		return StepResult{}, err
	}
	return e.result.Steps[len(e.result.Steps)-1], nil
}
func (s *SharedSession) Messages() []Message {
	out := []Message{}
	for _, m := range s.engine.messages {
		out = append(out, cloneMessage(m))
	}
	return out
}
func (e *engine) stateInstance(actor string) string {
	if id := e.instanceIDs[actor]; id != "" {
		return id
	}
	return actor
}
