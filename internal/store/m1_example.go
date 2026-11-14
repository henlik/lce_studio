package store

import (
	"context"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
)

type M1ExamplePack struct {
	system.M1ExamplePack
	Scenarios []systemrun.NamedScenario `json:"scenarios"`
}

func (s *Store) InstallM1Example(ctx context.Context) (M1ExamplePack, error) {
	e := M1ExamplePack{M1ExamplePack: system.M1Example()}
	if _, _, err := s.InstallDataContract(ctx, e.Model.Model.Machines[0].Machine.DataContracts[0].Contract); err != nil {
		return e, err
	}
	if _, _, err := s.installComponents(ctx, []model.ComponentPackage{e.Behavior}); err != nil {
		return e, err
	}
	for _, m := range e.Model.Model.Machines {
		if _, _, err := s.InstallStateMachine(ctx, m.Machine); err != nil {
			return e, err
		}
	}
	if _, _, err := s.InstallDeliveryModel(ctx, e.Model.Model); err != nil {
		return e, err
	}
	if _, _, err := s.InstallWorkflowPackage(ctx, e.Workflow.Workflow); err != nil {
		return e, err
	}
	for _, d := range e.Definitions {
		if _, err := s.InstallDefinition(ctx, d); err != nil {
			return e, err
		}
	}
	p, err := s.PrepareSystemExecution(ctx, e.Design)
	if err != nil {
		return e, err
	}
	e.Scenarios, err = systemrun.M1Scenarios(p.Plan)
	return e, err
}
