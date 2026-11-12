package state

import (
	"sort"

	"software-studio/internal/model"
)

// All callers validate bounded, flat structured values before copying.
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
		items := make([]model.TypedValue, len(*value.Collection))
		for i, item := range *value.Collection {
			items[i] = cloneValue(item)
		}
		copy.Collection = &items
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

func references(values map[string]model.TypedValue) map[string]model.TypedValue {
	copy := make(map[string]model.TypedValue, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}

func cloneReceipt(receipt Receipt) Receipt {
	receipt.Outputs = cloneValues(receipt.Outputs)
	receipt.EffectIDs = append([]string{}, receipt.EffectIDs...)
	return receipt
}

func cloneEffect(effect EffectIntent) EffectIntent {
	effect.Payload = cloneValues(effect.Payload)
	return effect
}

func cloneStep(step StepResult) StepResult {
	step.Before, step.After = cloneValues(step.Before), cloneValues(step.After)
	if step.Receipt != nil {
		receipt := cloneReceipt(*step.Receipt)
		step.Receipt = &receipt
	}
	// behavior.Evaluate already returns independently owned output and event
	// snapshots. State, receipt and effect copies never expose their pointers.
	return step
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
