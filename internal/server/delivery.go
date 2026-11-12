package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"software-studio/internal/delivery"
	"software-studio/internal/model"
	"software-studio/internal/store"
)

func (s *Server) deliveryExamples(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	respond(w, 200, model.ExampleDeliveryScenarios())
}

func (s *Server) deliveryPrepare(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var definition model.DeliveryModel
	if !decode(w, r, &definition) {
		return
	}
	if issues := model.ValidateDeliveryModel(definition); len(issues) > 0 {
		fail(w, 422, "Delivery model contains invalid definitions.", issues)
		return
	}
	digest, err := model.DeliveryModelDigest(definition)
	if err != nil {
		fail(w, 500, "Could not encode delivery model.", nil)
		return
	}
	respond(w, 200, model.DeliveryModelLock{Model: definition, Digest: digest})
}

func (s *Server) deliveryValidateLock(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var lock model.DeliveryModelLock
	if !decode(w, r, &lock) {
		return
	}
	issues := model.ValidateDeliveryModelLock(lock)
	if issues == nil {
		issues = []model.ValidationIssue{}
	}
	var canonical *model.DeliveryModelLock
	if len(issues) == 0 {
		canonical = &lock
	}
	respond(w, 200, struct {
		Valid  bool                     `json:"valid"`
		Issues []model.ValidationIssue  `json:"issues"`
		Model  *model.DeliveryModelLock `json:"model,omitempty"`
	}{len(issues) == 0, issues, canonical})
}

func validDeliveryModel(w http.ResponseWriter, lock model.DeliveryModelLock) bool {
	if issues := model.ValidateDeliveryModelLock(lock); len(issues) > 0 {
		fail(w, 422, "Delivery model lock is invalid.", issues)
		return false
	}
	return true
}

func (s *Server) deliveryModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.DeliveryModels(r.Context())
		if s.storageError(w, err, "Delivery model catalog") {
			return
		}
		respond(w, 200, records)
	case http.MethodPost:
		var lock model.DeliveryModelLock
		if !decode(w, r, &lock) || !validDeliveryModel(w, lock) {
			return
		}
		record, created, err := s.store.InstallDeliveryModel(r.Context(), lock.Model)
		if errors.Is(err, store.ErrDeliveryModelConflict) {
			fail(w, 409, "This delivery model ID and version are already installed with different content. Publish a new version instead.", nil)
			return
		}
		if errors.Is(err, store.ErrDeliveryCatalogFull) {
			fail(w, 409, "The delivery model catalog supports at most 200 installed versions.", nil)
			return
		}
		if s.storageError(w, err, "Delivery model catalog") {
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

func (s *Server) deliveryModel(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	ref := model.ComponentRef{PackageID: q.Get("id"), Version: q.Get("version"), Digest: q.Get("digest")}
	if issues := model.ValidateComponentReference(ref); len(issues) > 0 {
		fail(w, 422, "Invalid exact delivery model reference.", issues)
		return
	}
	record, err := s.store.DeliveryModel(r.Context(), ref.PackageID, ref.Version, ref.Digest)
	if s.storageError(w, err, "Delivery model") {
		return
	}
	respond(w, 200, record)
}

type deliveryCheckRequest struct {
	Model    model.DeliveryModelLock `json:"model"`
	Scenario model.DeliveryScenario  `json:"scenario"`
}

func deliveryExecutionError(w http.ResponseWriter, ctx context.Context, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		fail(w, 408, "Delivery scenario was cancelled or exceeded its execution time limit.", nil)
		return
	}
	if err != nil {
		fail(w, 422, err.Error(), nil)
	}
}

func (s *Server) deliveryCheck(w http.ResponseWriter, r *http.Request, save bool) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body deliveryCheckRequest
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
		deliveryExecutionError(w, ctx, err)
		return
	}
	// Scenario validation also validates the complete definition lock.
	issues := model.ValidateDeliveryScenario(body.Model, body.Scenario)
	if err := ctx.Err(); err != nil {
		deliveryExecutionError(w, ctx, err)
		return
	}
	if len(issues) > 0 {
		fail(w, 422, "Delivery scenario contains invalid inputs.", issues)
		return
	}
	result, err := delivery.Run(ctx, body.Model, body.Scenario)
	if err != nil {
		deliveryExecutionError(w, ctx, err)
		return
	}
	if err := ctx.Err(); err != nil {
		deliveryExecutionError(w, ctx, err)
		return
	}
	if !save {
		respond(w, 200, struct {
			Scenario model.DeliveryScenario `json:"scenario"`
			Result   delivery.Result        `json:"result"`
		}{body.Scenario, result})
		return
	}
	id, err := newID("delivery-run")
	if err != nil {
		fail(w, 500, "Could not create delivery run ID.", nil)
		return
	}
	record := store.DeliveryRunRecord{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Model: body.Model, Scenario: body.Scenario, Result: result}
	if err = s.store.SaveDeliveryRun(ctx, record); err != nil {
		if ctx.Err() != nil {
			deliveryExecutionError(w, ctx, err)
		} else {
			s.storageError(w, err, "Delivery run")
		}
		return
	}
	respond(w, 201, record)
}

func (s *Server) deliveryRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.DeliveryRuns(r.Context())
		if s.storageError(w, err, "Delivery runs") {
			return
		}
		respond(w, 200, records)
	case http.MethodPost:
		s.deliveryCheck(w, r, true)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, "Method not allowed.", nil)
	}
}

func (s *Server) deliveryRun(w http.ResponseWriter, r *http.Request, id string) {
	if !validID(id) {
		fail(w, 404, "Delivery run not found.", nil)
		return
	}
	if !method(w, r, http.MethodGet) {
		return
	}
	record, err := s.store.DeliveryRun(r.Context(), id)
	if s.storageError(w, err, "Delivery run") {
		return
	}
	respond(w, 200, record)
}
