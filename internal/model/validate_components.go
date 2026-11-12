package model

import "fmt"

// Package locks are portable: resolution never consults the installed catalog.
func validateProjectPackages(p Project) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	type identity struct{ id, version string }
	locks := make(map[identity]PackageLock, len(p.Packages))
	for i, lock := range p.Packages {
		path := fmt.Sprintf("packages[%d]", i)
		for _, issue := range ValidateComponentPackage(lock.Package) {
			add(path+".package."+issue.Path, issue.Message)
		}
		key := identity{lock.Package.ID, lock.Package.Version}
		if _, exists := locks[key]; exists {
			add(path+".package", "Each package ID and version must be unique.")
		}
		locks[key] = lock
		digest, err := ComponentDigest(lock.Package)
		if !componentDigest.MatchString(lock.Digest) || err != nil || lock.Digest != digest {
			add(path+".digest", "Digest must match the lowercase SHA-256 of the typed package definition.")
		}
	}
	for i, node := range p.Nodes {
		if node.Component == nil {
			continue
		}
		path := fmt.Sprintf("nodes[%d]", i)
		ref := node.Component
		lock, ok := locks[identity{ref.PackageID, ref.Version}]
		if !ok {
			add(path+".component", "Component must refer to an exact embedded package ID and version.")
			continue
		}
		if !componentDigest.MatchString(ref.Digest) || ref.Digest != lock.Digest {
			add(path+".component.digest", "Component digest must match its embedded package lock.")
		}
		if node.Type != lock.Package.Kernel {
			add(path+".type", "Node type must match its component package kernel.")
		}
		for _, field := range []struct {
			name      string
			value     int64
			parameter IntegerParameter
		}{
			{"processingMs", node.Config.ProcessingMs, lock.Package.Config.ProcessingMs},
			{"capacity", int64(node.Config.Capacity), lock.Package.Config.Capacity},
			{"memoryMb", ResidentMemoryMB(node.Config), lock.Package.Config.MemoryMB},
		} {
			if field.value < field.parameter.Min || field.value > field.parameter.Max {
				add(path+".config."+field.name, fmt.Sprintf("Effective value must be within the component interval %d..%d.", field.parameter.Min, field.parameter.Max))
			}
		}
	}
	return issues
}
