package model

import "fmt"

// PrimitiveSpecs returns fresh metadata; T is invocation-local scalar unification.
func PrimitiveSpecs() []PrimitiveSpec {
	port := func(name, kind string) BehaviorPort { return BehaviorPort{Name: name, Type: kind} }
	binary := func(kind string) []BehaviorPort { return []BehaviorPort{port("left", kind), port("right", kind)} }
	spec := func(op, name, description string, inputs []BehaviorPort, output string) PrimitiveSpec {
		return PrimitiveSpec{Op: op, Name: name, Description: description, Inputs: inputs, Outputs: []BehaviorPort{port("result", output)}}
	}
	return []PrimitiveSpec{
		spec("identity", "Identity", "Returns its typed input unchanged.", []BehaviorPort{port("value", "T")}, "T"),
		spec("equal", "Equal", "Compares two values of exactly the same scalar type.", binary("T"), "boolean"),
		spec("less_than", "Less than", "Tests whether the left integer is smaller than the right integer.", binary("integer"), "boolean"),
		spec("less_or_equal", "Less or equal", "Tests whether the left integer is no greater than the right integer.", binary("integer"), "boolean"),
		spec("and", "And", "Returns true when both boolean inputs are true.", binary("boolean"), "boolean"),
		spec("or", "Or", "Returns true when at least one boolean input is true.", binary("boolean"), "boolean"),
		spec("not", "Not", "Inverts a boolean input.", []BehaviorPort{port("value", "boolean")}, "boolean"),
		spec("select", "Select", "Eagerly selects one of two already-computed values of the same scalar type.", []BehaviorPort{port("condition", "boolean"), port("trueValue", "T"), port("falseValue", "T")}, "T"),
		spec("add", "Add", "Adds two integers; results outside the browser-safe integer range fail.", binary("integer"), "integer"),
		spec("subtract", "Subtract", "Subtracts the right integer from the left; out-of-range results fail.", binary("integer"), "integer"),
		spec("concat", "Concatenate", "Joins two strings; results exceeding 4096 bytes fail.", binary("string"), "string"),
	}
}

func scalarType(kind string) bool { return kind == "boolean" || kind == "integer" || kind == "string" }

// PrimitiveOutputType verifies the exact input signature and resolves generic T.
func PrimitiveOutputType(op string, inputs map[string]string) (string, error) {
	for _, spec := range PrimitiveSpecs() {
		if spec.Op != op {
			continue
		}
		if len(inputs) != len(spec.Inputs) {
			return "", fmt.Errorf("%s requires exactly %d inputs", op, len(spec.Inputs))
		}
		unified := ""
		for _, port := range spec.Inputs {
			kind, ok := inputs[port.Name]
			if !ok {
				return "", fmt.Errorf("%s is missing input %s", op, port.Name)
			}
			if !scalarType(kind) {
				return "", fmt.Errorf("%s input %s must have a scalar type", op, port.Name)
			}
			if port.Type == "T" {
				if unified != "" && unified != kind {
					return "", fmt.Errorf("%s generic input types must match exactly", op)
				}
				unified = kind
			} else if kind != port.Type {
				return "", fmt.Errorf("%s input %s must be %s", op, port.Name, port.Type)
			}
		}
		if spec.Outputs[0].Type == "T" {
			return unified, nil
		}
		return spec.Outputs[0].Type, nil
	}
	return "", fmt.Errorf("unsupported primitive %q", op)
}
