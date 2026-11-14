package store

import (
	"context"
	"software-studio/internal/system"
)

func (s *Store) InstallMessageExample(ctx context.Context) (system.MessageExamplePack, error) {
	e := system.MessageExecutionExample()
	for _, m := range e.Model.Model.Machines {
		if _, _, err := s.InstallStateMachine(ctx, m.Machine); err != nil {
			return e, err
		}
	}
	if _, _, err := s.InstallDeliveryModel(ctx, e.Model.Model); err != nil {
		return e, err
	}
	for _, d := range e.Definitions {
		if _, err := s.InstallDefinition(ctx, d); err != nil {
			return e, err
		}
	}
	return e, nil
}
