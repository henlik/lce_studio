package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/store"
	"software-studio/internal/workflow"
)

func (s *Server) workflowExamples(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	respond(w, 200, model.ExampleWorkflowScenarios())
}

func (s *Server) workflowPrepare(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var definition model.WorkflowDefinition
	if !decode(w, r, &definition) {
		return
	}
	if issues := model.ValidateWorkflowDefinition(definition); len(issues) > 0 {
		fail(w, 422, "Workflow package contains invalid definitions.", issues)
		return
	}
	digest, err := model.WorkflowDigest(definition)
	if err != nil {
		fail(w, 500, "Could not encode workflow package.", nil)
		return
	}
	respond(w, 200, model.WorkflowLock{Workflow: definition, Digest: digest})
}

func (s *Server) workflowValidateLock(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var lock model.WorkflowLock
	if !decode(w, r, &lock) {
		return
	}
	issues := model.ValidateWorkflowLock(lock)
	if issues == nil {
		issues = []model.ValidationIssue{}
	}
	var canonical *model.WorkflowLock
	if len(issues) == 0 {
		canonical = &lock
	}
	respond(w, 200, struct {
		Valid    bool                    `json:"valid"`
		Issues   []model.ValidationIssue `json:"issues"`
		Workflow *model.WorkflowLock     `json:"workflow,omitempty"`
	}{len(issues) == 0, issues, canonical})
}

func validWorkflow(w http.ResponseWriter, lock model.WorkflowLock) bool {
	if issues := model.ValidateWorkflowLock(lock); len(issues) > 0 {
		fail(w, 422, "Workflow package lock is invalid.", issues)
		return false
	}
	return true
}

func (s *Server) workflowPackages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.WorkflowPackages(r.Context())
		if s.storageError(w, err, "Workflow package catalog") {
			return
		}
		respond(w, 200, records)
	case http.MethodPost:
		var lock model.WorkflowLock
		if !decode(w, r, &lock) || !validWorkflow(w, lock) {
			return
		}
		record, created, err := s.store.InstallWorkflowPackage(r.Context(), lock.Workflow)
		if errors.Is(err, store.ErrWorkflowPackageConflict) {
			fail(w, 409, "This workflow package ID and version are already installed with different content. Publish a new version instead.", nil)
			return
		}
		if errors.Is(err, store.ErrWorkflowCatalogFull) {
			fail(w, 409, "The workflow package catalog supports at most 200 installed versions.", nil)
			return
		}
		if s.storageError(w, err, "Workflow package catalog") {
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

func (s *Server) workflowPackage(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	ref := model.ComponentRef{PackageID: q.Get("id"), Version: q.Get("version"), Digest: q.Get("digest")}
	if issues := model.ValidateComponentReference(ref); len(issues) > 0 {
		fail(w, 422, "Invalid exact workflow package reference.", issues)
		return
	}
	record, err := s.store.WorkflowPackage(r.Context(), ref.PackageID, ref.Version, ref.Digest)
	if s.storageError(w, err, "Workflow package") {
		return
	}
	respond(w, 200, record)
}

type workflowCheckRequest struct {
	Workflow model.WorkflowLock     `json:"workflow"`
	Scenario model.WorkflowScenario `json:"scenario"`
}

func workflowExecutionError(w http.ResponseWriter, ctx context.Context, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		fail(w, 408, "Workflow scenario was cancelled or exceeded its execution time limit.", nil)
		return
	}
	if err != nil {
		fail(w, 422, err.Error(), nil)
	}
}

func (s *Server) workflowCheck(w http.ResponseWriter, r *http.Request, save bool) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body workflowCheckRequest
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
		workflowExecutionError(w, ctx, err)
		return
	}
	// Scenario validation also validates the complete definition lock.
	issues := model.ValidateWorkflowScenario(body.Workflow, body.Scenario)
	if err := ctx.Err(); err != nil {
		workflowExecutionError(w, ctx, err)
		return
	}
	if len(issues) > 0 {
		fail(w, 422, "Workflow scenario contains invalid inputs.", issues)
		return
	}
	result, err := workflow.Run(ctx, body.Workflow, body.Scenario)
	if err != nil {
		workflowExecutionError(w, ctx, err)
		return
	}
	if err := ctx.Err(); err != nil {
		workflowExecutionError(w, ctx, err)
		return
	}
	if !save {
		respond(w, 200, struct {
			Scenario model.WorkflowScenario `json:"scenario"`
			Result   workflow.Result        `json:"result"`
		}{body.Scenario, result})
		return
	}
	id, err := newID("workflow-run")
	if err != nil {
		fail(w, 500, "Could not create workflow run ID.", nil)
		return
	}
	record := store.WorkflowRunRecord{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Workflow: body.Workflow, Scenario: body.Scenario, Result: result}
	if err = s.store.SaveWorkflowRun(ctx, record); err != nil {
		if ctx.Err() != nil {
			workflowExecutionError(w, ctx, err)
		} else {
			s.storageError(w, err, "Workflow run")
		}
		return
	}
	respond(w, 201, record)
}

func (s *Server) workflowRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.WorkflowRuns(r.Context())
		if s.storageError(w, err, "Workflow runs") {
			return
		}
		respond(w, 200, records)
	case http.MethodPost:
		s.workflowCheck(w, r, true)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, "Method not allowed.", nil)
	}
}

func (s *Server) workflowRun(w http.ResponseWriter, r *http.Request, id string) {
	if !validID(id) {
		fail(w, 404, "Workflow run not found.", nil)
		return
	}
	if !method(w, r, http.MethodGet) {
		return
	}
	record, err := s.store.WorkflowRun(r.Context(), id)
	if s.storageError(w, err, "Workflow run") {
		return
	}
	respond(w, 200, record)
}
