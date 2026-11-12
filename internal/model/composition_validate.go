package model

import "fmt"

type compositionIdentity struct{ id, version string }

func packageIdentity(p ComponentPackage) compositionIdentity {
	return compositionIdentity{p.ID, p.Version}
}
func referenceIdentity(ref ComponentRef) compositionIdentity {
	return compositionIdentity{ref.PackageID, ref.Version}
}

// ValidateComponentBundle resolves only the exact embedded dependency closure.
func ValidateComponentBundle(bundle ComponentBundle) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	if bundle.SchemaVersion != 1 {
		add("schemaVersion", "Only component bundle schema version 1 is supported.")
	}
	issues = append(issues, prefixBehaviorIssues("root", ValidateComponentReference(bundle.Root))...)
	if len(bundle.Packages) < 1 || len(bundle.Packages) > 50 {
		add("packages", "A bundle requires 1–50 package locks.")
		return issues
	}
	indices := make(map[compositionIdentity]int, len(bundle.Packages))
	for i, lock := range bundle.Packages {
		path := fmt.Sprintf("packages[%d]", i)
		local := ValidateComponentPackage(lock.Package)
		issues = append(issues, prefixBehaviorIssues(path+".package", local)...)
		if lock.Package.SchemaVersion != 2 && lock.Package.SchemaVersion != 3 {
			add(path+".package.schemaVersion", "Every bundled definition must be a schema 2 or 3 composite.")
		}
		identity := packageIdentity(lock.Package)
		if _, exists := indices[identity]; exists {
			add(path+".package", "Every package ID and version must be unique.")
		}
		indices[identity] = i
		if !componentDigest.MatchString(lock.Digest) {
			add(path+".digest", "A lock requires a lowercase SHA-256 digest.")
		} else if len(local) == 0 {
			digest, err := ComponentDigest(lock.Package)
			if err != nil || digest != lock.Digest {
				add(path+".digest", "Digest does not match the exact embedded definition.")
			}
		}
	}
	resolve := func(ref ComponentRef, path string) bool {
		index, ok := indices[referenceIdentity(ref)]
		if !ok {
			add(path, "Reference is missing its exact embedded package ID and version.")
			return false
		}
		if bundle.Packages[index].Digest != ref.Digest {
			add(path+".digest", "Reference digest does not match its embedded lock.")
			return false
		}
		return true
	}
	resolve(bundle.Root, "root")
	// Fail before walking oversized or malformed definitions, including their nodes.
	if len(issues) != 0 {
		return issues
	}
	contracts, contractIssues := BundleDataContracts(bundle)
	issues = append(issues, contractIssues...)
	if len(issues) != 0 {
		return issues
	}
	orders := make(map[compositionIdentity][]BehaviorNode, len(indices))
	for i, lock := range bundle.Packages {
		path := fmt.Sprintf("packages[%d].package.behavior", i)
		for j, node := range lock.Package.Behavior.Nodes {
			if node.Component != nil {
				resolve(*node.Component, fmt.Sprintf("%s.nodes[%d].component", path, j))
			}
		}
		order, err := BehaviorNodeOrder(*lock.Package.Behavior)
		if err != nil {
			add(path+".nodes", err.Error())
		} else {
			orders[packageIdentity(lock.Package)] = order
		}
	}
	if len(issues) != 0 {
		return issues
	}
	for i, lock := range bundle.Packages {
		issues = append(issues, validateCompositionTypes(lock.Package, fmt.Sprintf("packages[%d].package.behavior", i), orders[packageIdentity(lock.Package)], bundle.Packages, indices, contracts)...)
	}
	if len(issues) != 0 {
		return issues
	}
	// Count call occurrences, not distinct package identities; repeated reuse can
	// otherwise cause exponential work even in a small acyclic dependency graph.
	seen, stack := make(map[compositionIdentity]bool), make(map[compositionIdentity]bool)
	operations, invocations := 0, 0
	var visit func(compositionIdentity, string, int) bool
	visit = func(key compositionIdentity, path string, depth int) bool {
		if stack[key] {
			add(path, "Component dependencies contain a cycle.")
			return false
		}
		if depth > 8 {
			add(path, "Component nesting depth must not exceed 8, including the root.")
			return false
		}
		invocations++
		if invocations > 256 {
			add(path, "Expanded evaluation must not exceed 256 component invocations.")
			return false
		}
		seen[key], stack[key] = true, true
		defer delete(stack, key)
		for _, node := range orders[key] {
			if node.Component != nil {
				if !visit(referenceIdentity(*node.Component), path+"/"+node.ID, depth+1) {
					return false
				}
			} else {
				operations++
				if operations > 256 {
					add(path+"/"+node.ID, "Expanded evaluation must not exceed 256 primitive executions.")
					return false
				}
			}
		}
		return true
	}
	if !visit(referenceIdentity(bundle.Root), "root", 1) {
		return issues
	}
	for i, lock := range bundle.Packages {
		if !seen[packageIdentity(lock.Package)] {
			add(fmt.Sprintf("packages[%d]", i), "Every bundled definition must be reachable from the root; extra locks are not allowed.")
		}
	}
	return issues
}

func validateCompositionTypes(p ComponentPackage, path string, order []BehaviorNode, locks []PackageLock, indices map[compositionIdentity]int, contracts []DataContractLock) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	b := p.Behavior
	if p.SchemaVersion == 3 {
		for _, group := range []struct {
			name  string
			ports []BehaviorPort
		}{{"inputs", b.Inputs}, {"outputs", b.Outputs}} {
			for i, port := range group.ports {
				if port.RecordRef != nil {
					if _, err := ResolveDataRecord(*port.RecordRef, contracts); err != nil {
						add(fmt.Sprintf("%s.%s[%d].recordRef", path, group.name, i), err.Error())
					}
				}
			}
		}
		for i, node := range b.Nodes {
			for _, name := range sortedBehaviorKeys(node.Inputs) {
				if literal := node.Inputs[name].Literal; literal != nil {
					issues = append(issues, prefixBehaviorIssues(fmt.Sprintf("%s.nodes[%d].inputs.%s.literal", path, i, name), ValidateStructuredTypedValue(*literal, contracts))...)
				}
			}
		}
		for _, name := range sortedBehaviorKeys(b.Bindings) {
			if literal := b.Bindings[name].Literal; literal != nil {
				issues = append(issues, prefixBehaviorIssues(path+".bindings."+name+".literal", ValidateStructuredTypedValue(*literal, contracts))...)
			}
		}
		testPath := path[:len(path)-len("behavior")] + "tests"
		for i, test := range p.Tests {
			at := fmt.Sprintf("%s[%d]", testPath, i)
			issues = append(issues, prefixBehaviorIssues(at+".inputs", ValidateStructuredBehaviorValues(test.Inputs, b.Inputs, contracts))...)
			issues = append(issues, prefixBehaviorIssues(at+".expected", ValidateStructuredBehaviorValues(test.Expected, b.Outputs, contracts))...)
		}
	}
	inputs, nodeOutputs := make(map[string]BehaviorValueType), make(map[string]map[string]BehaviorValueType)
	for _, port := range b.Inputs {
		inputs[port.Name] = PortBehaviorType(port)
	}
	nodePaths := make(map[string]string, len(b.Nodes))
	for i, node := range b.Nodes {
		nodePaths[node.ID] = fmt.Sprintf("%s.nodes[%d]", path, i)
	}
	bindingType := func(binding BehaviorBinding, at string) BehaviorValueType {
		if binding.Input != "" {
			return inputs[binding.Input]
		}
		if binding.Literal != nil {
			return ValueBehaviorType(*binding.Literal)
		}
		kind, ok := nodeOutputs[binding.Node][binding.Port]
		if !ok {
			add(at+".port", "Source must name an existing output port on its local node.")
		}
		return kind
	}
	for _, node := range order {
		at := nodePaths[node.ID]
		kinds := make(map[string]BehaviorValueType)
		for _, name := range sortedBehaviorKeys(node.Inputs) {
			kinds[name] = bindingType(node.Inputs[name], at+".inputs."+name)
		}
		if node.Component == nil {
			var kind BehaviorValueType
			var err error
			if p.SchemaVersion == 3 {
				kind, err = StructuredPrimitiveOutputType(node, kinds, contracts)
			} else {
				scalars := make(map[string]string, len(kinds))
				for name, typ := range kinds {
					scalars[name] = typ.Type
				}
				var scalar string
				scalar, err = PrimitiveOutputType(node.Op, scalars)
				kind = BehaviorValueType{Type: scalar}
			}
			if err != nil {
				add(at+".inputs", err.Error())
			} else {
				nodeOutputs[node.ID] = map[string]BehaviorValueType{"result": kind}
			}
			continue
		}
		callee := locks[indices[referenceIdentity(*node.Component)]].Package.Behavior
		if p.SchemaVersion == 2 {
			for _, ports := range [][]BehaviorPort{callee.Inputs, callee.Outputs} {
				for _, port := range ports {
					if !scalarType(port.Type) || port.RecordRef != nil {
						add(at+".component", "Schema 2 call nodes require a scalar component interface.")
					}
				}
			}
		}
		expected := make(map[string]BehaviorValueType)
		for _, port := range callee.Inputs {
			expected[port.Name] = PortBehaviorType(port)
		}
		for _, name := range sortedBehaviorKeys(expected) {
			kind, ok := kinds[name]
			if !ok {
				add(at+".inputs."+name, "Every component input requires one binding.")
			} else if !SameBehaviorType(kind, expected[name]) {
				add(at+".inputs."+name, "Binding type must exactly match the called component input.")
			}
		}
		for _, name := range sortedBehaviorKeys(kinds) {
			if _, ok := expected[name]; !ok {
				add(at+".inputs."+name, "Binding does not correspond to a called component input.")
			}
		}
		outputs := make(map[string]BehaviorValueType)
		for _, port := range callee.Outputs {
			outputs[port.Name] = PortBehaviorType(port)
		}
		nodeOutputs[node.ID] = outputs
	}
	outputs := make(map[string]BehaviorValueType)
	for _, port := range b.Outputs {
		outputs[port.Name] = PortBehaviorType(port)
	}
	for _, name := range sortedBehaviorKeys(outputs) {
		if kind := bindingType(b.Bindings[name], path+".bindings."+name); !SameBehaviorType(kind, outputs[name]) {
			add(path+".bindings."+name, "Binding type must exactly match the public output type.")
		}
	}
	// Every local node must contribute transitively to at least one public output.
	nodes := make(map[string]BehaviorNode, len(b.Nodes))
	for _, node := range b.Nodes {
		nodes[node.ID] = node
	}
	observed := make(map[string]bool)
	var mark func(string)
	mark = func(id string) {
		if id == "" || observed[id] {
			return
		}
		observed[id] = true
		for _, name := range sortedBehaviorKeys(nodes[id].Inputs) {
			mark(nodes[id].Inputs[name].Node)
		}
	}
	for _, name := range sortedBehaviorKeys(b.Bindings) {
		mark(b.Bindings[name].Node)
	}
	for _, node := range b.Nodes {
		if !observed[node.ID] {
			add(nodePaths[node.ID], "Every internal node must contribute to a public output.")
		}
	}
	return issues
}

// PrepareComponentBundle pins one root and a dependency-only closure, preserving
// provided order and rejecting inconsistent identities instead of replacing them.
func PrepareComponentBundle(p ComponentPackage, dependencies []PackageLock) (ComponentBundle, []ValidationIssue) {
	if len(dependencies) > 49 {
		return ComponentBundle{}, []ValidationIssue{{"dependencies", "At most 49 dependency locks are supported."}}
	}
	issues := prefixBehaviorIssues("package", ValidateComponentPackage(p))
	if p.SchemaVersion != 2 && p.SchemaVersion != 3 {
		issues = append(issues, ValidationIssue{"package.schemaVersion", "The prepared root must be a schema 2 or 3 composite."})
	}
	if len(issues) != 0 {
		return ComponentBundle{}, issues
	}
	digest, err := ComponentDigest(p)
	if err != nil {
		return ComponentBundle{}, []ValidationIssue{{"package", "The package could not be encoded."}}
	}
	bundle := ComponentBundle{SchemaVersion: 1, Root: ComponentRef{PackageID: p.ID, Version: p.Version, Digest: digest}, Packages: make([]PackageLock, 0, len(dependencies)+1)}
	bundle.Packages = append(bundle.Packages, PackageLock{Package: p, Digest: digest})
	bundle.Packages = append(bundle.Packages, dependencies...)
	return bundle, ValidateComponentBundle(bundle)
}
