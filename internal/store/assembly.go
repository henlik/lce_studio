package store

import (
	"context"
	"errors"
	"software-studio/internal/assembly"
	"software-studio/internal/model"
	"software-studio/internal/system"
)

type assemblySource struct{ *Store }

func (s assemblySource) Definitions(ctx context.Context) ([]assembly.DefinitionRecord, error) {
	rs, e := s.Store.Definitions(ctx)
	out := []assembly.DefinitionRecord{}
	for _, r := range rs {
		out = append(out, assembly.DefinitionRecord{Definition: r.Definition})
	}
	return out, e
}
func (s assemblySource) ComponentPackage(ctx context.Context, id, v string) (model.ComponentPackage, error) {
	r, e := scanComponent(s.db.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM component_packages WHERE id=$1 AND version=$2`, id, v))
	return r.Package, e
}
func (s assemblySource) ComponentBundle(ctx context.Context, r model.ComponentRef) (model.ComponentBundle, error) {
	b, e := s.Store.ComponentBundle(ctx, r)
	if errors.Is(e, ErrNotComposite) || errors.Is(e, ErrNotFound) {
		e = assembly.ErrMissing
	}
	return b, e
}
func (s assemblySource) StateMachine(ctx context.Context, id, v, h string) (model.StateMachineLock, error) {
	r, e := s.Store.StateMachine(ctx, id, v, h)
	return model.StateMachineLock{Machine: r.Machine, Digest: r.Digest}, e
}
func (s assemblySource) DeliveryModel(ctx context.Context, id, v, h string) (model.DeliveryModelLock, error) {
	r, e := s.Store.DeliveryModel(ctx, id, v, h)
	return model.DeliveryModelLock{Model: r.Model, Digest: r.Digest}, e
}
func (s assemblySource) WorkflowPackage(ctx context.Context, id, v, h string) (model.WorkflowLock, error) {
	r, e := s.Store.WorkflowPackage(ctx, id, v, h)
	return model.WorkflowLock{Workflow: r.Workflow, Digest: r.Digest}, e
}

var _ assembly.Source = assemblySource{}
var _ system.SystemDesign
