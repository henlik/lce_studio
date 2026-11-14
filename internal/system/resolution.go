package system

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"software-studio/internal/model"
)

// CatalogEntry identifies a checked catalog definition, not a deployed binary.
type CatalogEntry struct {
	Kind    string      `json:"kind"`
	ID      string      `json:"id"`
	Version string      `json:"version"`
	Name    string      `json:"name"`
	Digest  string      `json:"digest"`
	Source  string      `json:"source"`
	Ready   bool        `json:"ready"`
	Issue   string      `json:"issue,omitempty"`
	Aspects []AspectRef `json:"aspects,omitempty"`
}
type ResolvedReference struct {
	Path string `json:"path"`
	CatalogEntry
}
type Resolution struct {
	Valid         bool                    `json:"valid"`
	Issues        []model.ValidationIssue `json:"issues"`
	References    []ResolvedReference     `json:"references"`
	Ports         []InstancePort          `json:"ports,omitempty"`
	Dependencies  []Dependency            `json:"dependencies,omitempty"`
	Connections   []ConnectionCheck       `json:"connections,omitempty"`
	Compatibility string                  `json:"compatibility,omitempty"`
}

// BuiltinCatalog is a reserved adapter namespace. Its digest identifies the
// adapter declaration, not the source code, executable, or build provenance.
func BuiltinCatalog() []CatalogEntry {
	declarations := []struct{ ID, Name, Semantics string }{
		{"builtin.intake-client", "Intake client", "Submit a tenant-scoped synthetic intake request and retry the same request identifier."},
		{"builtin.durable-intake", "Durable intake", "Persist one encounter per tenant and request using a PostgreSQL unique key; return the same encounter on retries."},
	}
	result := []CatalogEntry{}
	for _, declaration := range declarations {
		data, _ := json.Marshal(struct{ ID, Version, Semantics string }{declaration.ID, "1.0.0", declaration.Semantics})
		sum := sha256.Sum256(data)
		result = append(result, CatalogEntry{Kind: "component", ID: declaration.ID, Version: "1.0.0", Name: declaration.Name, Digest: hex.EncodeToString(sum[:]), Source: "builtin-adapter", Ready: true})
	}
	return result
}

// Resolve verifies direct references only. It does not establish port
// compatibility, provider capability, or implementation availability.
func Resolve(design SystemDesign, installed []CatalogEntry) Resolution {
	validation := Validate(design)
	result := Resolution{Valid: validation.Valid, Issues: validation.Issues, References: []ResolvedReference{}}
	if !validation.Valid {
		return result
	}
	index := map[string]CatalogEntry{}
	key := func(kind, id, version string) string { return kind + "\x00" + id + "\x00" + version }
	for _, entry := range installed {
		if strings.HasPrefix(entry.ID, "builtin.") {
			continue
		}
		k := key(entry.Kind, entry.ID, entry.Version)
		if _, exists := index[k]; exists {
			entry.Ready = false
			entry.Issue = "Ambiguous installed catalog identity"
		}
		index[k] = entry
	}
	for _, entry := range BuiltinCatalog() {
		index[key(entry.Kind, entry.ID, entry.Version)] = entry
	}
	resolve := func(path, kind, id, version string) {
		entry, exists := index[key(kind, id, version)]
		if !exists {
			result.Issues = append(result.Issues, model.ValidationIssue{Path: path, Message: fmt.Sprintf("Missing %s definition %s@%s. Install this exact version; no newer version is substituted.", kind, id, version)})
			return
		}
		if !entry.Ready {
			result.Issues = append(result.Issues, model.ValidationIssue{Path: path, Message: "Installed definition failed integrity or validation checks: " + entry.Issue})
			return
		}
		result.References = append(result.References, ResolvedReference{Path: path, CatalogEntry: entry})
		if kind == "definition" {
			for j, a := range entry.Aspects {
				child, ok := index[key(a.Kind, a.ID, a.Version)]
				childPath := fmt.Sprintf("%s.aspects[%d]", path, j)
				if !ok || !child.Ready || child.Digest != a.Digest {
					result.Issues = append(result.Issues, model.ValidationIssue{Path: childPath, Message: "Owned aspect is missing, invalid or differs from its pinned digest"})
					continue
				}
				result.References = append(result.References, ResolvedReference{Path: childPath, CatalogEntry: child})
			}
		}
	}
	for i, component := range design.Components {
		kind := component.Definition.Kind
		if kind == "" {
			kind = "component"
		}
		resolve(fmt.Sprintf("components[%d].definition", i), kind, component.Definition.ID, component.Definition.Version)
		for j, artifact := range component.Artifacts {
			resolve(fmt.Sprintf("components[%d].artifacts[%d]", i, j), artifact.Kind, artifact.ID, artifact.Version)
		}
	}
	result.Valid = len(result.Issues) == 0
	return result
}
