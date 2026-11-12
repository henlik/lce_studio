package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"software-studio/internal/behavior"
	"software-studio/internal/model"
	"software-studio/internal/store"
)

func (s *Server) componentPrimitives(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	respond(w, http.StatusOK, model.PrimitiveSpecs())
}

func (s *Server) compositeExamples(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	respond(w, http.StatusOK, model.ExampleComponentBundles())
}

func (s *Server) componentPrepare(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Package      model.ComponentPackage `json:"package"`
		Dependencies []model.PackageLock    `json:"dependencies"`
	}
	if !decode(w, r, &body) {
		return
	}
	bundle, issues := model.PrepareComponentBundle(body.Package, body.Dependencies)
	if len(issues) > 0 {
		fail(w, http.StatusUnprocessableEntity, "Component bundle contains invalid definitions.", issues)
		return
	}
	respond(w, http.StatusOK, bundle)
}

func (s *Server) componentValidateBundle(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var bundle model.ComponentBundle
	if !decode(w, r, &bundle) {
		return
	}
	issues := model.ValidateComponentBundle(bundle)
	if issues == nil {
		issues = []model.ValidationIssue{}
	}
	// Return the exact typed representation validated by Go. A browser must not
	// reinterpret the original JSON, whose field casing or duplicate fields may
	// be handled differently by another decoder.
	var canonical *model.ComponentBundle
	if len(issues) == 0 {
		canonical = &bundle
	}
	respond(w, http.StatusOK, struct {
		Valid  bool                    `json:"valid"`
		Issues []model.ValidationIssue `json:"issues"`
		Bundle *model.ComponentBundle  `json:"bundle,omitempty"`
	}{len(issues) == 0, issues, canonical})
}

func (s *Server) componentBundles(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var bundle model.ComponentBundle
	if !decode(w, r, &bundle) || !validBundle(w, bundle) {
		return
	}
	records, created, err := s.store.InstallComponentBundle(r.Context(), bundle)
	if errors.Is(err, store.ErrComponentConflict) {
		fail(w, http.StatusConflict, "A bundle version conflicts with already installed content. No versions were installed.", nil)
		return
	}
	if errors.Is(err, store.ErrCatalogFull) {
		fail(w, http.StatusConflict, "The complete bundle would exceed the catalog's 200-version limit. No versions were installed.", nil)
		return
	}
	if s.storageError(w, err, "Component bundle") {
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	respond(w, status, struct {
		Bundle    model.ComponentBundle   `json:"bundle"`
		Installed []store.ComponentRecord `json:"installed"`
	}{bundle, records})
}

func (s *Server) componentBundle(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	query := r.URL.Query()
	ref := model.ComponentRef{PackageID: query.Get("id"), Version: query.Get("version"), Digest: query.Get("digest")}
	if issues := model.ValidateComponentReference(ref); len(issues) > 0 {
		fail(w, http.StatusUnprocessableEntity, "Invalid exact component reference.", issues)
		return
	}
	bundle, err := s.store.ComponentBundle(r.Context(), ref)
	if errors.Is(err, store.ErrNotComposite) {
		fail(w, http.StatusUnprocessableEntity, "This reference is a simulation profile, not a composite definition.", nil)
		return
	}
	if s.storageError(w, err, "Component bundle") {
		return
	}
	respond(w, http.StatusOK, bundle)
}

func validBundle(w http.ResponseWriter, bundle model.ComponentBundle) bool {
	if issues := model.ValidateComponentBundle(bundle); len(issues) > 0 {
		fail(w, http.StatusUnprocessableEntity, "Component bundle contains invalid definitions.", issues)
		return false
	}
	return true
}

// Pure component evaluation and tests share the existing simulation admission
// channel, so the server never starts more than two computations in total.
func (s *Server) componentComputation(w http.ResponseWriter, r *http.Request) (context.Context, func(), bool) {
	select {
	case s.runs <- struct{}{}:
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		return ctx, func() { cancel(); <-s.runs }, true
	default:
		fail(w, http.StatusTooManyRequests, "Two computations are already running. Please try again shortly.", nil)
		return nil, nil, false
	}
}

func componentExecutionError(w http.ResponseWriter, ctx context.Context, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		fail(w, http.StatusRequestTimeout, "Component evaluation was cancelled or exceeded its execution time limit.", nil)
		return
	}
	fail(w, http.StatusUnprocessableEntity, err.Error(), nil)
}

func (s *Server) componentEvaluate(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Bundle model.ComponentBundle       `json:"bundle"`
		Inputs map[string]model.TypedValue `json:"inputs"`
	}
	if !decode(w, r, &body) || !validBundle(w, body.Bundle) {
		return
	}
	ctx, done, ok := s.componentComputation(w, r)
	if !ok {
		return
	}
	defer done()
	result, err := behavior.Evaluate(ctx, body.Bundle, body.Inputs)
	if err != nil {
		componentExecutionError(w, ctx, err)
		return
	}
	if err := ctx.Err(); err != nil {
		componentExecutionError(w, ctx, err)
		return
	}
	id, err := newID("evaluation")
	if err != nil {
		fail(w, http.StatusInternalServerError, "Could not create evaluation ID.", nil)
		return
	}
	record := store.EvaluationRecord{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Bundle: body.Bundle, Inputs: body.Inputs, Result: result}
	if err := s.store.SaveEvaluation(ctx, record); err != nil {
		if ctx.Err() != nil {
			componentExecutionError(w, ctx, err)
		} else {
			s.storageError(w, err, "Component evaluation")
		}
		return
	}
	respond(w, http.StatusCreated, record)
}

func (s *Server) componentTest(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Bundle model.ComponentBundle `json:"bundle"`
	}
	if !decode(w, r, &body) || !validBundle(w, body.Bundle) {
		return
	}
	ctx, done, ok := s.componentComputation(w, r)
	if !ok {
		return
	}
	defer done()
	result, err := behavior.Test(ctx, body.Bundle)
	if err != nil {
		componentExecutionError(w, ctx, err)
		return
	}
	respond(w, http.StatusOK, result)
}

func (s *Server) componentEvaluations(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	records, err := s.store.Evaluations(r.Context())
	if s.storageError(w, err, "Component evaluations") {
		return
	}
	respond(w, http.StatusOK, records)
}

func (s *Server) componentEvaluation(w http.ResponseWriter, r *http.Request, id string) {
	if !validID(id) {
		fail(w, http.StatusNotFound, "Component evaluation not found.", nil)
		return
	}
	if !method(w, r, http.MethodGet) {
		return
	}
	record, err := s.store.Evaluation(r.Context(), id)
	if s.storageError(w, err, "Component evaluation") {
		return
	}
	respond(w, http.StatusOK, record)
}
