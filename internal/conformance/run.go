// Package conformance runs a bounded reception fixture against a model and a
// real PostgreSQL implementation. It is not a general code generator.
package conformance

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"software-studio/internal/system"
)

var ErrUnsupportedDesign = errors.New("conformance runner supports only the reception intake reference topology")

type Trace struct {
	Events   []system.Event   `json:"events"`
	Verdicts []system.Verdict `json:"verdicts"`
}
type ScenarioResult struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ExpectedStatus string `json:"expectedStatus"`
	Passed         bool   `json:"passed"`
	Model          Trace  `json:"model"`
	Implementation Trace  `json:"implementation"`
}
type Report struct {
	Experiment       string           `json:"experiment"`
	DesignID         string           `json:"designId"`
	DesignRevision   string           `json:"designRevision"`
	ContractID       string           `json:"contractId"`
	ContractRevision string           `json:"contractRevision"`
	Scenarios        []ScenarioResult `json:"scenarios"`
	Passed           bool             `json:"passed"`
	Limitations      []string         `json:"limitations"`
}

// SupportsDesign compares executable structure; labels and contracts may be edited.
func SupportsDesign(d system.SystemDesign) bool {
	e := system.ReceptionExample()
	d.Components = append([]system.ComponentInstance(nil), d.Components...)
	d.DeploymentPlans = append([]system.DeploymentPlan(nil), d.DeploymentPlans...)
	d.ID = e.ID
	d.Name = e.Name
	d.Revision = e.Revision
	d.Contract = e.Contract
	for i := range d.Components {
		if i < len(e.Components) {
			d.Components[i].Name = e.Components[i].Name
		}
	}
	for i := range d.DeploymentPlans {
		if i < len(e.DeploymentPlans) {
			d.DeploymentPlans[i].Name = e.DeploymentPlans[i].Name
		}
	}
	return reflect.DeepEqual(d, e)
}

type scenario struct {
	id, name, status string
	attempts         int
}

var scenarios = []scenario{
	{"normal", "Normal request", "satisfied", 1},
	{"lost-response", "Discard response and retry", "satisfied", 2},
	{"concurrent-duplicates", "Eight concurrent duplicate requests", "satisfied", 8},
	{"restart", "Recreate service instance after persistence", "satisfied", 2},
	{"database-unavailable", "Closed database connection rejects intake", "satisfied", 1},
	{"faulty-dedup", "Deliberately broken deduplication", "violated", 2},
	{"missing-evidence", "Required persistence evidence removed", "inconclusive", 1},
}

// Run uses only synthetic requests in a fresh schema. No project tables or
// production patient records are read. Caller owns db and controls credentials.
func Run(ctx context.Context, db *sql.DB, design system.SystemDesign) (Report, error) {
	report := Report{Experiment: "reception-intake-v1", DesignID: design.ID, DesignRevision: design.Revision, ContractID: design.Contract.ID, ContractRevision: design.Contract.Revision, Scenarios: []ScenarioResult{}, Passed: true, Limitations: []string{
		"Runs the fixed reception intake reference implementation, not arbitrary generated system code.",
		"Restart recreates the Go service instance; it does not kill an operating-system process.",
		"Lost-response injection discards a returned response; database unavailability uses a closed SQL connection.",
		"Checks bounded synthetic traces; no claim of formal proof, real network partition testing, or production readiness.",
	}}
	if validation := system.Validate(design); !validation.Valid {
		return report, fmt.Errorf("invalid experiment design: %s", validation.Issues[0].Message)
	}
	if !SupportsDesign(design) {
		return report, ErrUnsupportedDesign
	}
	if db == nil {
		return report, errors.New("PostgreSQL database is required")
	}
	schema := "lce_conformance_" + token()
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		return report, fmt.Errorf("create isolated experiment schema: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db.ExecContext(cleanup, "DROP SCHEMA IF EXISTS "+quoted+" CASCADE")
	}()
	correct := quoted + ".encounters"
	faulty := quoted + ".faulty_encounters"
	for _, table := range []string{correct, faulty} {
		constraint := ""
		if table == correct {
			constraint = ", UNIQUE (tenant_id, request_id)"
		}
		if _, err := db.ExecContext(ctx, "CREATE TABLE "+table+" (tenant_id text NOT NULL, request_id text NOT NULL, encounter_id text PRIMARY KEY"+constraint+")"); err != nil {
			return report, err
		}
	}
	for _, s := range scenarios {
		collector := &events{}
		svc := &intake{db: db, table: correct, events: collector}
		req := s.id
		var unavailable *sql.Conn
		if s.id == "database-unavailable" {
			var err error
			unavailable, err = db.Conn(ctx)
			if err != nil {
				return report, err
			}
			unavailable.Close()
			svc.db = unavailable
		}
		if s.id == "faulty-dedup" {
			svc.table = faulty
			svc.faulty = true
		}
		type outcome struct {
			id  string
			err error
		}
		outcomes := make(chan outcome, s.attempts)
		var wg sync.WaitGroup
		for i := 0; i < s.attempts; i++ {
			if s.id == "restart" && i > 0 {
				svc = &intake{db: db, table: correct, events: collector}
			}
			if s.id == "concurrent-duplicates" {
				wg.Add(1)
				go func() {
					defer wg.Done()
					id, err := svc.receive(ctx, "synthetic-hospital", req)
					outcomes <- outcome{id, err}
				}()
			} else {
				svc.discardResponse = s.id == "lost-response" && i == 0
				id, err := svc.receive(ctx, "synthetic-hospital", req)
				outcomes <- outcome{id, err}
			}
		}
		wg.Wait()
		close(outcomes)
		original := ""
		for out := range outcomes {
			if s.id == "database-unavailable" {
				if out.err == nil || out.id != "" {
					return report, errors.New("unavailable database unexpectedly returned success")
				}
				continue
			}
			if out.err != nil {
				return report, fmt.Errorf("scenario %s intake failed: %w", s.id, out.err)
			}
			if out.id == "" {
				return report, fmt.Errorf("scenario %s returned no encounter", s.id)
			}
			if original != "" && original != out.id && s.id != "faulty-dedup" {
				return report, fmt.Errorf("scenario %s retry returned a different encounter", s.id)
			}
			original = out.id
		}
		actual := collector.snapshot()
		// Verify traces against persisted state, rather than trusting emitted successes.
		if s.id != "database-unavailable" {
			rows, err := db.QueryContext(ctx, "SELECT encounter_id FROM "+svc.table+" WHERE tenant_id=$1 AND request_id=$2 ORDER BY encounter_id", "synthetic-hospital", req)
			if err != nil {
				return report, err
			}
			count := 0
			for rows.Next() {
				var id string
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					return report, err
				}
				count++
				actual = append(actual, system.Event{Kind: "persisted", RequestID: req, EncounterID: id, Detail: "Independent database readback"})
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return report, err
			}
			if count == 0 {
				return report, errors.New("experiment produced no durable encounter")
			}
		}
		if s.id == "missing-evidence" {
			filtered := []system.Event{}
			for _, e := range actual {
				if e.Kind != "persisted" {
					filtered = append(filtered, e)
				}
			}
			actual = append(filtered, system.Event{Kind: "evidence_gap", RequestID: req, Detail: "Persistence observations deliberately removed"})
		}
		renumber(actual)
		modeled := modelTrace(s)
		result := ScenarioResult{ID: s.id, Name: s.name, ExpectedStatus: s.status, Model: Trace{modeled, system.Evaluate(design.Contract, modeled)}, Implementation: Trace{actual, system.Evaluate(design.Contract, actual)}}
		result.Passed = aggregate(result.Model.Verdicts) == "satisfied" && aggregate(result.Implementation.Verdicts) == s.status
		report.Passed = report.Passed && result.Passed
		report.Scenarios = append(report.Scenarios, result)
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
		return report, fmt.Errorf("clean up experiment schema: %w", err)
	}
	return report, nil
}

// modelTrace is an independent specification adapter: a successful logical
// request has one durable record regardless of retries and execution details.
func modelTrace(s scenario) []system.Event {
	events := []system.Event{{Kind: "request_received", RequestID: s.id}}
	if s.id == "database-unavailable" {
		events = append(events, system.Event{Kind: "response_failure", RequestID: s.id})
	} else {
		events = append(events, system.Event{Kind: "persisted", RequestID: s.id, EncounterID: "model-encounter"}, system.Event{Kind: "response_success", RequestID: s.id, EncounterID: "model-encounter"})
	}
	renumber(events)
	return events
}
func aggregate(v []system.Verdict) string {
	if len(v) == 0 {
		return "inconclusive"
	}
	status := "satisfied"
	for _, x := range v {
		if x.Status == "violated" {
			return "violated"
		}
		if x.Status != "satisfied" {
			status = "inconclusive"
		}
	}
	return status
}
func renumber(es []system.Event) {
	for i := range es {
		es[i].Sequence = i + 1
	}
}
func token() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

type querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type intake struct {
	discardResponse bool
	db              querier
	table           string
	faulty          bool
	events          *events
}

func (s *intake) receive(ctx context.Context, tenant, request string) (string, error) {
	s.events.add(system.Event{Kind: "request_received", RequestID: request})
	q := "INSERT INTO " + s.table + " (tenant_id,request_id,encounter_id) VALUES ($1,$2,$3)"
	if !s.faulty {
		q += " ON CONFLICT (tenant_id,request_id) DO UPDATE SET request_id=EXCLUDED.request_id"
	}
	q += " RETURNING encounter_id"
	var id string
	err := s.db.QueryRowContext(ctx, q, tenant, request, token()).Scan(&id)
	if err != nil {
		s.events.add(system.Event{Kind: "response_failure", RequestID: request, Detail: "Persistence unavailable; no success returned"})
		return "", err
	}
	s.events.add(system.Event{Kind: "persisted", RequestID: request, EncounterID: id})
	if !s.discardResponse {
		s.events.add(system.Event{Kind: "response_success", RequestID: request, EncounterID: id})
	}
	return id, nil
}

type events struct {
	mu    sync.Mutex
	items []system.Event
}

func (e *events) add(v system.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v.Sequence = len(e.items) + 1
	e.items = append(e.items, v)
}
func (e *events) snapshot() []system.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]system.Event{}, e.items...)
}
