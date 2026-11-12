package behavior

import (
	"fmt"

	"software-studio/internal/model"
)

func boolean(value bool) model.TypedValue  { return model.TypedValue{Type: "boolean", Boolean: &value} }
func integer(value int64) model.TypedValue { return model.TypedValue{Type: "integer", Integer: &value} }
func textValue(value string) model.TypedValue {
	return model.TypedValue{Type: "string", String: &value}
}

// primitive is deliberately closed over the eleven documented scalar kernels.
// All inputs are validated before pointer access; there is no coercion or
// fallback decision when a value or attribute is missing.
func primitive(op string, inputs map[string]model.TypedValue) (model.TypedValue, error) {
	types := make(map[string]string, len(inputs))
	for _, name := range sortedKeys(inputs) {
		value := inputs[name]
		if issues := model.ValidateTypedValue(value); len(issues) != 0 {
			return model.TypedValue{}, fmt.Errorf("input %s: %s: %s", name, issues[0].Path, issues[0].Message)
		}
		types[name] = value.Type
	}
	if _, err := model.PrimitiveOutputType(op, types); err != nil {
		return model.TypedValue{}, err
	}
	return primitiveValue(op, inputs)
}

// primitiveValue runs the shared pure operations after their caller has
// validated values and signatures for its schema version.
func primitiveValue(op string, inputs map[string]model.TypedValue) (model.TypedValue, error) {
	left, right := inputs["left"], inputs["right"]
	switch op {
	case "identity":
		return cloneValue(inputs["value"]), nil
	case "equal":
		return boolean(equalValue(left, right)), nil
	case "less_than":
		return boolean(*left.Integer < *right.Integer), nil
	case "less_or_equal":
		return boolean(*left.Integer <= *right.Integer), nil
	case "and":
		return boolean(*left.Boolean && *right.Boolean), nil
	case "or":
		return boolean(*left.Boolean || *right.Boolean), nil
	case "not":
		return boolean(!*inputs["value"].Boolean), nil
	case "select":
		if *inputs["condition"].Boolean {
			return cloneValue(inputs["trueValue"]), nil
		}
		return cloneValue(inputs["falseValue"]), nil
	case "add", "subtract":
		// Each operand is within ±(2^53−1), so intermediate int64 arithmetic
		// cannot overflow. The narrower browser-exact range is then enforced.
		value := *left.Integer + *right.Integer
		if op == "subtract" {
			value = *left.Integer - *right.Integer
		}
		if value < -model.MaxBehaviorInteger || value > model.MaxBehaviorInteger {
			return model.TypedValue{}, fmt.Errorf("%s result exceeds the safe integer range ±%d", op, model.MaxBehaviorInteger)
		}
		return integer(value), nil
	case "concat":
		if len(*left.String) > MaxStringBytes-len(*right.String) {
			return model.TypedValue{}, fmt.Errorf("concat result exceeds %d UTF-8 bytes", MaxStringBytes)
		}
		return textValue(*left.String + *right.String), nil
	}
	return model.TypedValue{}, fmt.Errorf("unsupported primitive %q", op)
}

func equalValue(left, right model.TypedValue) bool {
	if left.Type != right.Type {
		return false
	}
	switch left.Type {
	case "boolean":
		return left.Boolean != nil && right.Boolean != nil && *left.Boolean == *right.Boolean
	case "integer":
		return left.Integer != nil && right.Integer != nil && *left.Integer == *right.Integer
	case "string":
		return left.String != nil && right.String != nil && *left.String == *right.String
	case "record":
		return sameDataReference(left.RecordRef, right.RecordRef) && left.Record != nil && right.Record != nil && equalValues(left.Record, right.Record)
	case "collection":
		if !sameDataReference(left.RecordRef, right.RecordRef) || left.Collection == nil || right.Collection == nil || len(*left.Collection) != len(*right.Collection) {
			return false
		}
		for i := range *left.Collection {
			if !equalValue((*left.Collection)[i], (*right.Collection)[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func sameDataReference(left, right *model.DataTypeRef) bool {
	return left != nil && right != nil && *left == *right
}
