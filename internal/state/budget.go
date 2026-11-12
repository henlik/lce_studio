package state

import (
	"encoding/json"
	"fmt"
)

func jsonSize(value any) (int, error) {
	encoded, err := json.Marshal(value)
	return len(encoded), err
}

func commas(count int) int {
	if count == 0 {
		return 0
	}
	return count - 1
}

func instanceSize(instance *ownedInstance) (int, error) {
	// Encode only bounded maps and fixed metadata, never an expanding journal.
	header := InstanceResult{ID: instance.id, Revision: instance.revision, State: emptyValues(), Receipts: []Receipt{}, Effects: []EffectIntent{}}
	base, err := jsonSize(header)
	if err != nil {
		return 0, err
	}
	state, err := jsonSize(instance.values)
	return base + state - 2 + instance.receiptBytes + commas(len(instance.receipts)) + instance.effectBytes + commas(len(instance.effects)), err
}

func stepSize(step StepResult) (int, error) {
	// Account for independently encoded bounded pieces before copying any
	// repeated values into retained evidence. Decision JSON is already <=1MiB.
	header := step
	header.Before, header.After = emptyValues(), emptyValues()
	header.Receipt, header.Decision = nil, nil
	total, err := jsonSize(header)
	if err != nil {
		return 0, err
	}
	for _, values := range []any{step.Before, step.After} {
		n, err := jsonSize(values)
		if err != nil {
			return 0, err
		}
		total += n - 2
	}
	if step.Receipt != nil {
		n, err := jsonSize(step.Receipt)
		if err != nil {
			return 0, err
		}
		total += len(",\"receipt\":") + n
	}
	if step.Decision != nil {
		n, err := jsonSize(step.Decision)
		if err != nil {
			return 0, err
		}
		total += len(",\"decision\":") + n
	}
	return total, nil
}

func (e *engine) checkResultBudget(replaceID string, candidate *ownedInstance, extraStepBytes, extraSteps int) error {
	header := e.result
	header.Instances, header.Steps, header.Expectations = []InstanceResult{}, []StepResult{}, []ExpectationResult{}
	total, err := jsonSize(header)
	if err != nil {
		return fmt.Errorf("encode result metadata: %w", err)
	}
	total += e.stepBytes + extraStepBytes + commas(len(e.result.Steps)+extraSteps) + commas(len(e.order))
	for _, id := range e.order {
		instance := e.instances[id]
		if id == replaceID {
			instance = candidate
		}
		n, err := instanceSize(instance)
		if err != nil {
			return fmt.Errorf("encode instance evidence: %w", err)
		}
		total += n
	}
	for _, expected := range e.result.Expectations {
		n, err := jsonSize(expected)
		if err != nil {
			return fmt.Errorf("encode expectations: %w", err)
		}
		total += n
	}
	total += commas(len(e.result.Expectations))
	if total > MaxResultBytes {
		return fmt.Errorf("%w: serialized result exceeds %d bytes", ErrLimit, MaxResultBytes)
	}
	return nil
}
