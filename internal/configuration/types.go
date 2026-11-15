// Package configuration implements the versioned LCE Configuration Language.
// Parsing and evaluation are bounded, deterministic and perform no external I/O.
package configuration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const Version = "1.0"
const CompilerVersion = "lce-configuration-1.0"
const MaxProjectBytes = 4 << 20
const MaxFileBytes = 512 << 10
const MaxFiles = 128
const MaxArtifacts = 512

type Project struct {
	Language   string            `json:"language"`
	Entry      string            `json:"entry"`
	Files      map[string]string `json:"files"`
	Parameters map[string]any    `json:"parameters,omitempty"`
	Lock       *Lockfile         `json:"lock,omitempty"`
}
type Span struct {
	File   string `json:"file"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}
type Diagnostic struct {
	Span
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}
type Expr struct {
	Span
	Kind   string  `json:"kind"`
	Value  any     `json:"value,omitempty"`
	Name   string  `json:"name,omitempty"`
	Items  []*Expr `json:"items,omitempty"`
	Fields []Field `json:"fields,omitempty"`
}
type Field struct {
	Name  string `json:"name"`
	Span  Span   `json:"span"`
	Value *Expr  `json:"value"`
}
type Declaration struct {
	Span
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	NameSpan Span   `json:"nameSpan"`
	Type     string `json:"type,omitempty"`
	Value    *Expr  `json:"value"`
}
type Import struct {
	Span
	Path      string `json:"path"`
	Alias     string `json:"alias"`
	AliasSpan Span   `json:"aliasSpan"`
	Arguments *Expr  `json:"arguments,omitempty"`
}
type Document struct {
	File         string        `json:"file"`
	Module       string        `json:"module"`
	Version      string        `json:"version"`
	Imports      []Import      `json:"imports"`
	Declarations []Declaration `json:"declarations"`
	Comments     []Span        `json:"comments"`
}
type Artifact struct {
	Symbol  string          `json:"symbol"`
	Kind    string          `json:"kind"`
	ID      string          `json:"id"`
	Version string          `json:"version"`
	Digest  string          `json:"digest"`
	Body    json.RawMessage `json:"body"`
	Origin  Span            `json:"origin"`
}
type Target struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}
type Experiment struct {
	Name     string          `json:"name"`
	Target   Target          `json:"target"`
	Scenario json.RawMessage `json:"scenario"`
	Expect   map[string]any  `json:"expect"`
	Origin   Span            `json:"origin"`
}
type Lockfile struct {
	Compiler   string            `json:"compiler"`
	SourceHash string            `json:"sourceHash"`
	Files      map[string]string `json:"files"`
	Artifacts  []Target          `json:"artifacts"`
}
type Compilation struct {
	Language    string       `json:"language"`
	Valid       bool         `json:"valid"`
	SourceHash  string       `json:"sourceHash"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Artifacts   []Artifact   `json:"artifacts"`
	Experiments []Experiment `json:"experiments"`
	Documents   []Document   `json:"documents"`
	Lock        Lockfile     `json:"lock"`
}

func Hash(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func SourceHash(p Project) string { p.Lock = nil; return Hash(p) }
