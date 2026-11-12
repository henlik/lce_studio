package delivery

import (
	"software-studio/internal/model"
	"software-studio/internal/state"
)

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
func cloneReceipt(receipt state.Receipt) *state.Receipt {
	receipt.Outputs = cloneValues(receipt.Outputs)
	receipt.EffectIDs = append([]string{}, receipt.EffectIDs...)
	return &receipt
}
func cloneMessage(message Message) Message {
	message.Inputs = cloneValues(message.Inputs)
	if message.Acknowledgement != nil {
		message.Acknowledgement = cloneReceipt(*message.Acknowledgement)
	}
	return message
}
