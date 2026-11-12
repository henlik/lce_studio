package state

import "fmt"

// ReserveWork shares the existing experiment budget with an outer pure
// orchestration step. Failed evaluations retain their reservation.
func (b *Budget) ReserveWork(operations, invocations int) error {
	if operations < 0 || invocations < 0 || operations > MaxOperations-b.reserved.operations || invocations > MaxInvocations-b.reserved.invocations {
		return fmt.Errorf("%w: global decision work exceeds supported bounds", ErrLimit)
	}
	b.reserved.operations += operations
	b.reserved.invocations += invocations
	return nil
}

// Revision reads only protocol metadata; it does not expose mutable state.
func (s *Session) Revision(instanceID string) (int64, error) {
	if s.failed != nil {
		return 0, s.failed
	}
	instance, ok := s.engine.instances[instanceID]
	if !ok {
		return 0, fmt.Errorf("unknown owned instance %q", instanceID)
	}
	return instance.revision, nil
}
