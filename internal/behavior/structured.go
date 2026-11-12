package behavior

import (
	"fmt"

	"software-studio/internal/model"
)

// structuredPrimitive uses only the validated embedded contract union. It never
// resolves a mutable catalog and never edits an input collection or record.
func structuredPrimitive(node model.BehaviorNode, inputs map[string]model.TypedValue, contracts []model.DataContractLock) (model.TypedValue, error) {
	types := make(map[string]model.BehaviorValueType, len(inputs))
	ports := make([]model.BehaviorPort, 0, len(inputs))
	for _, name := range sortedKeys(inputs) {
		value := inputs[name]
		types[name] = model.ValueBehaviorType(value)
		ports = append(ports, model.BehaviorPort{Name: name, Type: value.Type, RecordRef: value.RecordRef})
	}
	// This verifies both individual structures and the aggregate public-map bound
	// before clone/equality work. Signature checking then rejects wrong input names.
	if issues := model.ValidateStructuredBehaviorValues(inputs, ports, contracts); len(issues) != 0 {
		return model.TypedValue{}, fmt.Errorf("invalid primitive input at %s: %s", issues[0].Path, issues[0].Message)
	}
	outputType, err := model.StructuredPrimitiveOutputType(node, types, contracts)
	if err != nil {
		return model.TypedValue{}, err
	}
	var result model.TypedValue
	switch node.Op {
	case "record_make":
		ref := *node.RecordRef
		result = model.TypedValue{Type: "record", RecordRef: &ref, Record: cloneValues(inputs)}
	case "record_get":
		value, found := inputs["value"].Record[node.Field]
		if !found {
			return model.TypedValue{}, fmt.Errorf("record field %q is missing", node.Field)
		}
		result = cloneValue(value)
	case "collection_count":
		result = integer(int64(len(*inputs["values"].Collection)))
	case "collection_at":
		values, index := *inputs["values"].Collection, *inputs["index"].Integer
		if index < 0 || index >= int64(len(values)) {
			return model.TypedValue{}, fmt.Errorf("collection index %d is outside [0,%d)", index, len(values))
		}
		result = cloneValue(values[int(index)])
	case "collection_append":
		values := *inputs["values"].Collection
		if len(values) >= model.MaxCollectionItems {
			return model.TypedValue{}, fmt.Errorf("collection_append result exceeds %d records", model.MaxCollectionItems)
		}
		copied := make([]model.TypedValue, len(values)+1)
		for i, value := range values {
			copied[i] = cloneValue(value)
		}
		copied[len(values)] = cloneValue(inputs["value"])
		ref := *inputs["values"].RecordRef
		result = model.TypedValue{Type: "collection", RecordRef: &ref, Collection: &copied}
	default:
		result, err = primitiveValue(node.Op, inputs)
		if err != nil {
			return model.TypedValue{}, err
		}
	}
	if issues := model.ValidateStructuredTypedValue(result, contracts); len(issues) != 0 {
		return model.TypedValue{}, fmt.Errorf("invalid primitive result at %s: %s", issues[0].Path, issues[0].Message)
	}
	if !model.SameBehaviorType(model.ValueBehaviorType(result), outputType) {
		return model.TypedValue{}, fmt.Errorf("primitive result does not match its declared nominal type")
	}
	return result, nil
}
