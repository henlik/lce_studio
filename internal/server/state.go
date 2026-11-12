package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/state"
	"software-studio/internal/store"
)

func (s *Server) stateExamples(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	respond(w, 200, model.ExampleStateScenarios())
}

func (s *Server) statePrepare(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var machine model.StateMachine
	if !decode(w, r, &machine) {
		return
	}
	if issues := model.ValidateStateMachine(machine); len(issues) > 0 {
		fail(w, 422, "State machine contains invalid definitions.", issues)
		return
	}
	digest, err := model.StateMachineDigest(machine)
	if err != nil {
		fail(w, 500, "Could not encode state machine.", nil)
		return
	}
	respond(w, 200, model.StateMachineLock{Machine: machine, Digest: digest})
}

func (s *Server) stateValidateLock(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var lock model.StateMachineLock
	if !decode(w, r, &lock) {
		return
	}
	issues := model.ValidateStateMachineLock(lock)
	if issues == nil {
		issues = []model.ValidationIssue{}
	}
	var canonical *model.StateMachineLock
	if len(issues) == 0 {
		canonical = &lock
	}
	respond(w, 200, struct {
		Valid   bool                    `json:"valid"`
		Issues  []model.ValidationIssue `json:"issues"`
		Machine *model.StateMachineLock `json:"machine,omitempty"`
	}{len(issues) == 0, issues, canonical})
}

func validStateMachine(w http.ResponseWriter, lock model.StateMachineLock) bool {
	if issues := model.ValidateStateMachineLock(lock); len(issues) > 0 {
		fail(w, 422, "State machine lock is invalid.", issues)
		return false
	}
	return true
}

func (s *Server) stateMachines(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.StateMachines(r.Context())
		if s.storageError(w, err, "State machine catalog") {
			return
		}
		respond(w, 200, records)
	case http.MethodPost:
		var lock model.StateMachineLock
		if !decode(w, r, &lock) || !validStateMachine(w, lock) {
			return
		}
		record, created, err := s.store.InstallStateMachine(r.Context(), lock.Machine)
		if errors.Is(err, store.ErrStateMachineConflict) {
			fail(w, 409, "This state machine ID and version are already installed with different content. Publish a new version instead.", nil)
			return
		}
		if errors.Is(err, store.ErrStateCatalogFull) {
			fail(w, 409, "The state machine catalog supports at most 200 installed versions.", nil)
			return
		}
		if s.storageError(w, err, "State machine catalog") {
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		respond(w, status, record)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, "Method not allowed.", nil)
	}
}

func (s *Server) stateMachine(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	ref := model.ComponentRef{PackageID: q.Get("id"), Version: q.Get("version"), Digest: q.Get("digest")}
	if issues := model.ValidateComponentReference(ref); len(issues) > 0 {
		fail(w, 422, "Invalid exact state machine reference.", issues)
		return
	}
	record, err := s.store.StateMachine(r.Context(), ref.PackageID, ref.Version, ref.Digest)
	if s.storageError(w, err, "State machine") {
		return
	}
	respond(w, 200, record)
}

type stateCheckRequest struct {
	Machine  model.StateMachineLock `json:"machine"`
	Scenario model.StateScenario    `json:"scenario"`
}

func stateExecutionError(w http.ResponseWriter, ctx context.Context, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		fail(w, 408, "State scenario was cancelled or exceeded its execution time limit.", nil)
		return
	}
	if err != nil {
		fail(w, 422, err.Error(), nil)
	}
}

func (s *Server) stateCheck(w http.ResponseWriter, r *http.Request, save bool) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body stateCheckRequest
	if !decode(w, r, &body) {
		return
	}
	// Semantic validation traverses embedded decision bundles, so admission and
	// the deadline must cover validation as well as scenario execution/storage.
	ctx, done, ok := s.componentComputation(w, r)
	if !ok {
		return
	}
	defer done()
	if err := ctx.Err(); err != nil {
		stateExecutionError(w, ctx, err)
		return
	}
	// Scenario validation also validates the complete machine lock.
	issues := model.ValidateStateScenario(body.Machine, body.Scenario)
	if err := ctx.Err(); err != nil {
		stateExecutionError(w, ctx, err)
		return
	}
	if len(issues) > 0 {
		fail(w, 422, "State scenario contains invalid inputs.", issues)
		return
	}
	result, err := state.Run(ctx, body.Machine, body.Scenario)
	if err != nil {
		stateExecutionError(w, ctx, err)
		return
	}
	if err := ctx.Err(); err != nil {
		stateExecutionError(w, ctx, err)
		return
	}
	if !save {
		respond(w, 200, struct {
			Scenario model.StateScenario `json:"scenario"`
			Result   state.Result        `json:"result"`
		}{body.Scenario, result})
		return
	}
	id, err := newID("state-run")
	if err != nil {
		fail(w, 500, "Could not create state run ID.", nil)
		return
	}
	record := store.StateRunRecord{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Machine: body.Machine, Scenario: body.Scenario, Result: result}
	if err = s.store.SaveStateRun(ctx, record); err != nil {
		if ctx.Err() != nil {
			stateExecutionError(w, ctx, err)
		} else {
			s.storageError(w, err, "State run")
		}
		return
	}
	respond(w, 201, record)
}

func (s *Server) stateRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.StateRuns(r.Context())
		if s.storageError(w, err, "State runs") {
			return
		}
		respond(w, 200, records)
	case http.MethodPost:
		s.stateCheck(w, r, true)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, "Method not allowed.", nil)
	}
}

func (s *Server) stateRun(w http.ResponseWriter, r *http.Request, id string) {
	if !validID(id) {
		fail(w, 404, "State run not found.", nil)
		return
	}
	if !method(w, r, http.MethodGet) {
		return
	}
	record, err := s.store.StateRun(r.Context(), id)
	if s.storageError(w, err, "State run") {
		return
	}
	respond(w, 200, record)
}
