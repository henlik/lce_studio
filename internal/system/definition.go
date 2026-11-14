package system

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"software-studio/internal/model"
	"strings"
)

// ComponentDefinition owns named links to immutable artifact versions. It does
// not change the semantics of those artifacts or merge their execution engines.
type ComponentDefinition struct {
	SchemaVersion int         `json:"schemaVersion"`
	ID            string      `json:"id"`
	Version       string      `json:"version"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	Aspects       []AspectRef `json:"aspects"`
}
type AspectRef struct {
	Slot    string `json:"slot"`
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

var definitionID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,119}$`)
var definitionDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func DefinitionDigest(d ComponentDefinition) (string, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
func ValidateDefinition(d ComponentDefinition) model.Validation {
	issues := []model.ValidationIssue{}
	add := func(path, message string) {
		issues = append(issues, model.ValidationIssue{Path: path, Message: message})
	}
	if d.SchemaVersion != 1 {
		add("schemaVersion", "Only definition schema version 1 is supported")
	}
	if !definitionID.MatchString(d.ID) || !strings.Contains(d.ID, ".") || strings.HasPrefix(d.ID, "builtin.") {
		add("id", "Use a namespaced identifier outside the reserved builtin namespace")
	}
	if !exactVersion.MatchString(d.Version) || len(d.Version) > 32 {
		add("version", "Use an exact major.minor.patch version")
	}
	if strings.TrimSpace(d.Name) == "" || len(d.Name) > 200 {
		add("name", "A name of at most 200 bytes is required")
	}
	if len(d.Description) > 4000 {
		add("description", "Description exceeds 4000 bytes")
	}
	if len(d.Aspects) == 0 || len(d.Aspects) > 32 {
		add("aspects", "A definition must own between 1 and 32 aspect references")
	}
	slots := map[string]bool{}
	for i, a := range d.Aspects {
		p := fmt.Sprintf("aspects[%d]", i)
		if !definitionID.MatchString(a.Slot) || slots[a.Slot] {
			add(p+".slot", "Use a unique nonempty aspect slot")
		}
		slots[a.Slot] = true
		switch a.Kind {
		case "component", "data", "state", "messages", "workflow":
		default:
			add(p+".kind", "Choose behavior (component), data, state, messages or workflow")
		}
		if !definitionID.MatchString(a.ID) || strings.HasPrefix(a.ID, "builtin.") {
			add(p+".id", "Reference an installed artifact outside the builtin namespace")
		}
		if !exactVersion.MatchString(a.Version) || len(a.Version) > 32 {
			add(p+".version", "An exact version is required")
		}
		if !definitionDigest.MatchString(a.Digest) {
			add(p+".digest", "Pin the installed artifact's SHA-256 digest")
		}
	}
	return model.Validation{Valid: len(issues) == 0, Issues: issues}
}

func ResolveDefinition(d ComponentDefinition, catalog []CatalogEntry) model.Validation {
	result := ValidateDefinition(d)
	if !result.Valid {
		return result
	}
	for i, a := range d.Aspects {
		found := false
		for _, e := range catalog {
			if e.Kind == a.Kind && e.ID == a.ID && e.Version == a.Version {
				found = true
				if !e.Ready || e.Digest != a.Digest {
					result.Issues = append(result.Issues, model.ValidationIssue{Path: fmt.Sprintf("aspects[%d]", i), Message: "Artifact is invalid or its digest does not match the pinned version"})
				}
				break
			}
		}
		if !found {
			result.Issues = append(result.Issues, model.ValidationIssue{Path: fmt.Sprintf("aspects[%d]", i), Message: "Install the exact artifact version before installing this definition"})
		}
	}
	result.Valid = len(result.Issues) == 0
	return result
}
