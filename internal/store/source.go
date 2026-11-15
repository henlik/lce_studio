package store

import (
	"context"
	"fmt"
	"software-studio/internal/source"
)

// InstallSourceComponent recompiles untrusted text at the storage boundary.
// Exact versions are immutable. Installation is idempotent per artifact; if a
// later artifact conflicts, earlier installed artifacts remain safe to retry.
func (s *Store) InstallSourceComponent(ctx context.Context, text string) (source.Result, error) {
	r := source.Compile(text)
	if !r.Valid || r.Component == nil {
		return r, fmt.Errorf("valid component source is required")
	}
	p := r.Component
	if _, _, err := s.InstallDataContract(ctx, p.Data.Contract); err != nil {
		return r, err
	}
	if _, _, err := s.installComponents(ctx, p.Packages); err != nil {
		return r, err
	}
	if _, _, err := s.InstallStateMachine(ctx, p.Machine.Machine); err != nil {
		return r, err
	}
	if _, err := s.InstallDefinition(ctx, p.Definition); err != nil {
		return r, err
	}
	return r, nil
}
