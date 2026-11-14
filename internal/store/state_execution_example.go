package store

import (
	"context"
	"software-studio/internal/system"
)

// Installation is idempotent per immutable artifact. A failure may leave earlier
// artifacts installed; retry is safe and no draft is replaced until all succeed.
func (s *Store) InstallStateExecutionExample(ctx context.Context) (system.StateExecutionExamplePack, error) {
	e := system.StateExecutionExample()
	if _, _, err := s.installComponents(ctx, e.Packages); err != nil {
		return e, err
	}
	if _, _, err := s.InstallStateMachine(ctx, e.Machine.Machine); err != nil {
		return e, err
	}
	_, err := s.InstallDefinition(ctx, e.Definition)
	return e, err
}
