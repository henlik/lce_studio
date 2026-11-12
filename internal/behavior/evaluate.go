// Package behavior evaluates bounded, pure typed component compositions.
// It has no virtual clock, hardware cost model, or external side effects.
package behavior

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"software-studio/internal/model"
)

const (
	Version        = "0.5.0"
	MaxOperations  = 256
	MaxInvocations = 256
	MaxDepth       = 8
	MaxResultBytes = 1 << 20
	MaxStringBytes = 4096
)

// ErrLimit identifies a rejected bounded evaluation. No successful partial
// result is returned on a limit, cancellation, validation or execution error.
var ErrLimit = errors.New("behavior evaluation limit exceeded")

type Result struct {
	EngineVersion string                      `json:"engineVersion"`
	BundleHash    string                      `json:"bundleHash"`
	Root          model.ComponentRef          `json:"root"`
	Outputs       map[string]model.TypedValue `json:"outputs"`
	Operations    int                         `json:"operations"`
	Invocations   int                         `json:"invocations"`
	Events        []Event                     `json:"events"`
}

type Event struct {
	Sequence  int                         `json:"sequence"`
	Path      string                      `json:"path"`
	Kind      string                      `json:"kind"`
	Op        string                      `json:"op,omitempty"`
	Component *model.ComponentRef         `json:"component,omitempty"`
	Inputs    map[string]model.TypedValue `json:"inputs,omitempty"`
	Outputs   map[string]model.TypedValue `json:"outputs,omitempty"`
}

type preparedComponent struct {
	definition model.ComponentPackage
	order      []model.BehaviorNode
}

type preparedBundle struct {
	root       model.ComponentRef
	hash       string
	components map[model.ComponentRef]preparedComponent
	contracts  []model.DataContractLock
}

type evaluator struct {
	ctx        context.Context
	prepared   *preparedBundle
	result     Result
	eventBytes int
}

// Evaluate validates a portable bundle and evaluates its root. It never
// resolves catalog state or changes the bundle or supplied input values.
func Evaluate(ctx context.Context, bundle model.ComponentBundle, inputs map[string]model.TypedValue) (Result, error) {
	prepared, err := prepare(ctx, bundle)
	if err != nil {
		return Result{}, err
	}
	return evaluatePrepared(ctx, prepared, inputs)
}

func prepare(ctx context.Context, bundle model.ComponentBundle) (*preparedBundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("$: %w", err)
	}
	if issues := model.ValidateComponentBundle(bundle); len(issues) != 0 {
		return nil, fmt.Errorf("$: invalid bundle at %s: %s", issues[0].Path, issues[0].Message)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("$: %w", err)
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("$: encode bundle: %w", err)
	}
	hash := sha256.Sum256(encoded)
	p := &preparedBundle{root: bundle.Root, hash: hex.EncodeToString(hash[:]), components: make(map[model.ComponentRef]preparedComponent, len(bundle.Packages))}
	p.contracts, _ = model.BundleDataContracts(bundle) // already checked by bundle validation
	for _, lock := range bundle.Packages {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("$: %w", err)
		}
		order, err := model.BehaviorNodeOrder(*lock.Package.Behavior)
		if err != nil {
			return nil, fmt.Errorf("$: component %s: %w", lock.Package.ID, err)
		}
		ref := model.ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}
		p.components[ref] = preparedComponent{definition: lock.Package, order: order}
	}
	return p, nil
}

func evaluatePrepared(ctx context.Context, prepared *preparedBundle, inputs map[string]model.TypedValue) (Result, error) {
	e := evaluator{ctx: ctx, prepared: prepared, result: Result{
		EngineVersion: Version, BundleHash: prepared.hash, Root: prepared.root,
		Outputs: map[string]model.TypedValue{}, Events: []Event{},
	}}
	outputs, err := e.call(prepared.root, inputs, "$", 1)
	if err != nil {
		return Result{}, err
	}
	e.result.Outputs = cloneValues(outputs)
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("$: %w", err)
	}
	encoded, err := json.Marshal(e.result)
	if err != nil {
		return Result{}, fmt.Errorf("$: encode result: %w", err)
	}
	if len(encoded) > MaxResultBytes {
		return Result{}, fmt.Errorf("$: %w: serialized result exceeds %d bytes", ErrLimit, MaxResultBytes)
	}
	return e.result, nil
}

func (e *evaluator) call(ref model.ComponentRef, inputs map[string]model.TypedValue, path string, depth int) (map[string]model.TypedValue, error) {
	if err := e.ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if depth > MaxDepth {
		return nil, fmt.Errorf("%s: %w: nesting depth exceeds %d", path, ErrLimit, MaxDepth)
	}
	if e.result.Invocations >= MaxInvocations {
		return nil, fmt.Errorf("%s: %w: component invocations exceed %d", path, ErrLimit, MaxInvocations)
	}
	component, found := e.prepared.components[ref]
	if !found {
		return nil, fmt.Errorf("%s: unresolved component %s@%s", path, ref.PackageID, ref.Version)
	}
	b := component.definition.Behavior
	if issues := validateCallValues(component.definition.SchemaVersion, inputs, b.Inputs, e.prepared.contracts); len(issues) != 0 {
		return nil, fmt.Errorf("%s: invalid inputs at %s: %s", path, issues[0].Path, issues[0].Message)
	}
	inputs = cloneValues(inputs)
	e.result.Invocations++
	if err := e.emit(Event{Path: path, Kind: "component_enter", Component: refCopy(ref), Inputs: cloneValues(inputs)}); err != nil {
		return nil, err
	}
	values := make(map[string]map[string]model.TypedValue, len(component.order))
	for _, node := range component.order {
		nodePath := path + "/" + node.ID
		if err := e.ctx.Err(); err != nil {
			return nil, fmt.Errorf("%s: %w", nodePath, err)
		}
		args, err := resolveBindings(node.Inputs, inputs, values)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", nodePath, err)
		}
		var outputs map[string]model.TypedValue
		if node.Component != nil {
			outputs, err = e.call(*node.Component, args, nodePath, depth+1)
		} else {
			if e.result.Operations >= MaxOperations {
				return nil, fmt.Errorf("%s: %w: primitive executions exceed %d", nodePath, ErrLimit, MaxOperations)
			}
			e.result.Operations++
			var value model.TypedValue
			if component.definition.SchemaVersion == 3 {
				value, err = structuredPrimitive(node, args, e.prepared.contracts)
			} else {
				value, err = primitive(node.Op, args)
			}
			if err != nil {
				return nil, fmt.Errorf("%s: %w", nodePath, err)
			}
			outputs = map[string]model.TypedValue{"result": value}
			err = e.emit(Event{Path: nodePath, Kind: "primitive", Op: node.Op, Inputs: cloneValues(args), Outputs: cloneValues(outputs)})
		}
		if err != nil {
			return nil, err
		}
		values[node.ID] = outputs
	}
	outputs, err := resolveBindings(b.Bindings, inputs, values)
	if err != nil {
		return nil, fmt.Errorf("%s: outputs: %w", path, err)
	}
	if issues := validateCallValues(component.definition.SchemaVersion, outputs, b.Outputs, e.prepared.contracts); len(issues) != 0 {
		return nil, fmt.Errorf("%s: invalid outputs at %s: %s", path, issues[0].Path, issues[0].Message)
	}
	if err := e.emit(Event{Path: path, Kind: "component_exit", Component: refCopy(ref), Outputs: cloneValues(outputs)}); err != nil {
		return nil, err
	}
	return outputs, nil
}

// emit checks the exact serialized size so far without repeatedly encoding the
// accumulated event list. Final public outputs are checked again before return.
func (e *evaluator) emit(event Event) error {
	if err := e.ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", event.Path, err)
	}
	event.Sequence = len(e.result.Events) + 1
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("%s: encode event: %w", event.Path, err)
	}
	metadata := e.result
	metadata.Events = []Event{}
	header, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("%s: encode result metadata: %w", event.Path, err)
	}
	// Header includes the empty array brackets; one comma separates each pair.
	size := len(header) + e.eventBytes + len(encoded) + len(e.result.Events)
	if size > MaxResultBytes {
		return fmt.Errorf("%s: %w: serialized result exceeds %d bytes", event.Path, ErrLimit, MaxResultBytes)
	}
	e.eventBytes += len(encoded)
	e.result.Events = append(e.result.Events, event)
	return nil
}

func resolveBindings(bindings map[string]model.BehaviorBinding, inputs map[string]model.TypedValue, values map[string]map[string]model.TypedValue) (map[string]model.TypedValue, error) {
	outputs := make(map[string]model.TypedValue, len(bindings))
	for _, name := range sortedKeys(bindings) {
		binding := bindings[name]
		var value model.TypedValue
		var found bool
		switch {
		case binding.Literal != nil:
			value, found = *binding.Literal, true
		case binding.Input != "":
			value, found = inputs[binding.Input]
		case binding.Node != "" && binding.Port != "":
			value, found = values[binding.Node][binding.Port]
		}
		if !found {
			return nil, fmt.Errorf("binding %s has an unresolved source", name)
		}
		// Bindings are internal read-only references. The caller validates the
		// aggregate map before a call/kernel copies values, preventing repeated
		// bindings from amplifying allocations before the public-map bound.
		// Every result and event snapshot is independently deep-copied.
		outputs[name] = value
	}
	return outputs, nil
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func refCopy(ref model.ComponentRef) *model.ComponentRef { return &ref }

func validateCallValues(schema int, values map[string]model.TypedValue, ports []model.BehaviorPort, contracts []model.DataContractLock) []model.ValidationIssue {
	if schema == 3 {
		return model.ValidateStructuredBehaviorValues(values, ports, contracts)
	}
	return model.ValidateBehaviorValues(values, ports)
}

func cloneValue(value model.TypedValue) model.TypedValue {
	copy := value
	if value.Boolean != nil {
		v := *value.Boolean
		copy.Boolean = &v
	}
	if value.Integer != nil {
		v := *value.Integer
		copy.Integer = &v
	}
	if value.String != nil {
		v := *value.String
		copy.String = &v
	}
	if value.RecordRef != nil {
		ref := *value.RecordRef
		copy.RecordRef = &ref
	}
	if value.Record != nil {
		copy.Record = cloneValues(value.Record)
	}
	if value.Collection != nil {
		values := make([]model.TypedValue, len(*value.Collection))
		for i, item := range *value.Collection {
			values[i] = cloneValue(item)
		}
		copy.Collection = &values
	}
	return copy
}

func cloneValues(values map[string]model.TypedValue) map[string]model.TypedValue {
	copy := make(map[string]model.TypedValue, len(values))
	for key, value := range values {
		copy[key] = cloneValue(value)
	}
	return copy
}
