package model

import "fmt"

// StructuredPrimitiveSpecs is separate from the stable eleven-scalar catalog.
// record_make inputs and record_get output are resolved from their metadata.
func StructuredPrimitiveSpecs() []PrimitiveSpec {
	specs := PrimitiveSpecs()
	specs[1].Description = "Compares values of exactly the same nominal type; record field order is ignored and collection order is preserved."
	specs[7].Description = "Eagerly selects one of two computed values of exactly the same nominal type."
	port := func(name, kind string) BehaviorPort { return BehaviorPort{Name: name, Type: kind} }
	specs = append(specs,
		PrimitiveSpec{Op: "record_make", Name: "Make record", Description: "Constructs the exact referenced record from every declared scalar field.", Inputs: []BehaviorPort{}, Outputs: []BehaviorPort{port("result", "record")}},
		PrimitiveSpec{Op: "record_get", Name: "Record field", Description: "Reads the selected declared scalar field from an exact typed record.", Inputs: []BehaviorPort{port("value", "record")}, Outputs: []BehaviorPort{port("result", "T")}},
		PrimitiveSpec{Op: "collection_count", Name: "Collection count", Description: "Returns the number of records in the ordered collection.", Inputs: []BehaviorPort{port("values", "collection")}, Outputs: []BehaviorPort{port("result", "integer")}},
		PrimitiveSpec{Op: "collection_at", Name: "Collection item", Description: "Returns the record at a zero-based integer index; invalid indices fail.", Inputs: []BehaviorPort{port("values", "collection"), port("index", "integer")}, Outputs: []BehaviorPort{port("result", "record")}},
		PrimitiveSpec{Op: "collection_append", Name: "Append record", Description: "Returns a new bounded ordered collection with one exact typed record appended.", Inputs: []BehaviorPort{port("values", "collection"), port("value", "record")}, Outputs: []BehaviorPort{port("result", "collection")}},
	)
	return specs
}

func validateStructuredNodeMetadata(node BehaviorNode) []ValidationIssue {
	issues := []ValidationIssue{}
	if node.Op == "record_make" && node.Component == nil {
		if node.RecordRef == nil {
			issues = append(issues, ValidationIssue{"recordRef", "record_make requires an exact record reference."})
		} else {
			issues = append(issues, prefixBehaviorIssues("recordRef", ValidateDataTypeRef(*node.RecordRef))...)
		}
	} else if node.RecordRef != nil {
		issues = append(issues, ValidationIssue{"recordRef", "Only record_make accepts recordRef node metadata."})
	}
	if node.Op == "record_get" && node.Component == nil {
		if !behaviorName.MatchString(node.Field) {
			issues = append(issues, ValidationIssue{"field", "record_get requires a declared scalar field name."})
		}
	} else if node.Field != "" {
		issues = append(issues, ValidationIssue{"field", "Only record_get accepts field node metadata."})
	}
	return issues
}

func StructuredPrimitiveOutputType(node BehaviorNode, inputs map[string]BehaviorValueType, locks []DataContractLock) (BehaviorValueType, error) {
	invalid := func(message string) (BehaviorValueType, error) {
		return BehaviorValueType{}, fmt.Errorf("%s: %s", node.Op, message)
	}
	if issues := validateStructuredNodeMetadata(node); len(issues) > 0 {
		return invalid(issues[0].Message)
	}
	if len(inputs) > 16 {
		return invalid("at most 16 inputs are supported")
	}
	for _, name := range sortedBehaviorKeys(inputs) {
		typ := inputs[name]
		if issues := validateBehaviorValueType(typ); len(issues) > 0 {
			return invalid(fmt.Sprintf("input %s: %s", name, issues[0].Message))
		}
		if typ.RecordRef != nil {
			if _, err := ResolveDataRecord(*typ.RecordRef, locks); err != nil {
				return invalid(fmt.Sprintf("input %s: %s", name, err))
			}
		}
	}
	require := func(names ...string) error {
		if len(inputs) != len(names) {
			return fmt.Errorf("requires exactly %d inputs", len(names))
		}
		for _, name := range names {
			if _, ok := inputs[name]; !ok {
				return fmt.Errorf("missing input %s", name)
			}
		}
		return nil
	}
	switch node.Op {
	case "record_make":
		record, err := ResolveDataRecord(*node.RecordRef, locks)
		if err != nil {
			return invalid(err.Error())
		}
		if len(inputs) != len(record.Fields) {
			return invalid("requires one input for every declared scalar record field")
		}
		for _, field := range record.Fields {
			value, ok := inputs[field.Name]
			if !ok || value.Type != field.Type || value.RecordRef != nil {
				return invalid("record field inputs must exactly match their declared scalar types")
			}
		}
		return BehaviorValueType{"record", node.RecordRef}, nil
	case "record_get":
		if err := require("value"); err != nil {
			return invalid(err.Error())
		}
		value := inputs["value"]
		if value.Type != "record" {
			return invalid("value input must be a record")
		}
		record, err := ResolveDataRecord(*value.RecordRef, locks)
		if err != nil {
			return invalid(err.Error())
		}
		for _, field := range record.Fields {
			if field.Name == node.Field {
				return BehaviorValueType{Type: field.Type}, nil
			}
		}
		return invalid("selected field is not declared by the exact record")
	case "collection_count", "collection_at", "collection_append":
		names := []string{"values"}
		if node.Op == "collection_at" {
			names = append(names, "index")
		}
		if node.Op == "collection_append" {
			names = append(names, "value")
		}
		if err := require(names...); err != nil {
			return invalid(err.Error())
		}
		values := inputs["values"]
		if values.Type != "collection" {
			return invalid("values input must be a collection")
		}
		if node.Op == "collection_count" {
			return BehaviorValueType{Type: "integer"}, nil
		}
		if node.Op == "collection_at" {
			if inputs["index"].Type != "integer" {
				return invalid("index input must be an integer")
			}
			return BehaviorValueType{"record", values.RecordRef}, nil
		}
		if !SameBehaviorType(inputs["value"], BehaviorValueType{"record", values.RecordRef}) {
			return invalid("appended record must match the collection's exact nominal reference")
		}
		return values, nil
	}
	for _, spec := range PrimitiveSpecs() {
		if spec.Op != node.Op {
			continue
		}
		if len(inputs) != len(spec.Inputs) {
			return invalid(fmt.Sprintf("requires exactly %d inputs", len(spec.Inputs)))
		}
		var unified *BehaviorValueType
		for _, port := range spec.Inputs {
			typ, ok := inputs[port.Name]
			if !ok {
				return invalid("missing input " + port.Name)
			}
			if port.Type == "T" {
				if unified != nil && !SameBehaviorType(*unified, typ) {
					return invalid("generic input nominal types must match exactly")
				}
				copy := typ
				unified = &copy
			} else if typ.Type != port.Type || typ.RecordRef != nil {
				return invalid("input " + port.Name + " must be " + port.Type)
			}
		}
		if spec.Outputs[0].Type == "T" {
			return *unified, nil
		}
		return BehaviorValueType{Type: spec.Outputs[0].Type}, nil
	}
	return invalid("unsupported pure primitive")
}
