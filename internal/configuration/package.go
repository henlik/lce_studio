package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"software-studio/internal/model"
	"strings"
)

const PackageVersion = 1

type Package struct {
	SchemaVersion int        `json:"schemaVersion"`
	ID            string     `json:"id"`
	Version       string     `json:"version"`
	Project       Project    `json:"project"`
	Artifacts     []Artifact `json:"artifacts"`
	Report        *Report    `json:"report,omitempty"`
	Digest        string     `json:"digest"`
}

func PackageDigest(p Package) string { p.Digest = ""; return Hash(p) }
func Pack(ctx context.Context, id, version string, p Project, test bool) (Package, error) {
	c := Compile(p)
	if !c.Valid {
		return Package{}, fmt.Errorf("compile configuration before packaging")
	}
	if !moduleID.MatchString(id) || !strings.Contains(id, ".") || !versionID.MatchString(version) {
		return Package{}, fmt.Errorf("package requires safe namespaced ID and exact version")
	}
	p.Lock = &c.Lock
	pkg := Package{SchemaVersion: PackageVersion, ID: id, Version: version, Project: p, Artifacts: c.Artifacts}
	if test {
		r, e := Run(ctx, p)
		if e != nil {
			return pkg, e
		}
		pkg.Report = &r
	}
	pkg.Digest = PackageDigest(pkg)
	return pkg, nil
}
func VerifyPackage(p Package) (Compilation, error) {
	if p.SchemaVersion != PackageVersion || p.Digest != PackageDigest(p) || !moduleID.MatchString(p.ID) || !strings.Contains(p.ID, ".") || !versionID.MatchString(p.Version) {
		return Compilation{}, fmt.Errorf("unsupported or corrupted configuration package")
	}
	if p.Project.Lock == nil {
		return Compilation{}, fmt.Errorf("package must include an exact lock")
	}
	c := Compile(p.Project)
	if !c.Valid || Hash(c.Artifacts) != Hash(p.Artifacts) {
		return c, fmt.Errorf("package source, lock and compiled artifacts do not agree")
	}
	if p.Report != nil && (p.Report.Scope != nil || p.Report.SourceHash != c.SourceHash || Hash(p.Report.Lock) != Hash(c.Lock) || p.Report.Compiler != CompilerVersion) {
		return c, fmt.Errorf("evidence provenance differs from package source")
	}
	return c, nil
}
func Replay(ctx context.Context, p Package) (Report, error) {
	if _, e := VerifyPackage(p); e != nil {
		return Report{}, e
	}
	if p.Report == nil {
		return Report{}, fmt.Errorf("package contains no test evidence to reproduce")
	}
	r, e := Run(ctx, p.Project)
	if e != nil {
		return r, e
	}
	if Hash(r) != Hash(*p.Report) {
		return r, fmt.Errorf("fresh execution differs from packaged evidence")
	}
	return r, nil
}

type Change struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}
type Compatibility struct {
	Compatible bool     `json:"compatible"`
	Changes    []Change `json:"changes"`
}

// Behavioral changes need a conformance review; no finite test suite proves all
// future behavior. Data contracts use the existing directional compatibility rules.
func Compare(old, new Package) (Compatibility, error) {
	a, e := VerifyPackage(old)
	if e != nil {
		return Compatibility{}, e
	}
	b, e := VerifyPackage(new)
	if e != nil {
		return Compatibility{}, e
	}
	r := Compatibility{Compatible: true, Changes: []Change{}}
	used := map[int]bool{}
	for _, x := range a.Artifacts {
		found := -1
		for i, y := range b.Artifacts {
			if x.Kind == y.Kind && x.ID == y.ID {
				found = i
				if x.Version == y.Version {
					break
				}
			}
		}
		ch := Change{Kind: x.Kind, ID: x.ID, From: x.Version}
		if found < 0 {
			ch.Status = "removed"
			ch.Detail = "Consumers must explicitly migrate"
			r.Compatible = false
		} else {
			used[found] = true
			y := b.Artifacts[found]
			ch.To = y.Version
			if x.Digest == y.Digest {
				ch.Status = "unchanged"
			} else if x.Version == y.Version {
				ch.Status = "immutable-conflict"
				ch.Detail = "Changed content requires a new version"
				r.Compatible = false
			} else if x.Kind == "data" {
				var d1, d2 model.DataContract
				_ = json.Unmarshal(x.Body, &d1)
				_ = json.Unmarshal(y.Body, &d2)
				cmp, err := model.CompareDataContracts(model.DataContractLock{Contract: d1, Digest: x.Digest}, model.DataContractLock{Contract: d2, Digest: y.Digest})
				if err != nil {
					return r, err
				}
				if cmp.Compatible {
					ch.Status = "compatible-data"
				} else {
					ch.Status = "breaking-data"
					r.Compatible = false
				}
				ch.Detail = cmp.Direction
			} else {
				ch.Status = "review-required"
				ch.Detail = "Run consumer conformance tests before upgrading the exact pin"
				r.Compatible = false
			}
		}
		r.Changes = append(r.Changes, ch)
	}
	for i, y := range b.Artifacts {
		if !used[i] {
			r.Changes = append(r.Changes, Change{Kind: y.Kind, ID: y.ID, To: y.Version, Status: "added"})
		}
	}
	return r, nil
}
