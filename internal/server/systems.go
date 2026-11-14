package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"software-studio/internal/conformance"
	"software-studio/internal/store"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
)

func (s *Server) systemsAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/systems")
	switch {
	case path == "/execution/prepare" || path == "/execution/run" || path == "/executions" || strings.HasPrefix(path, "/executions/") || strings.HasPrefix(path, "/execution-replays/"):
		s.systemExecutionAPI(w, r, path)
	case path == "/definitions" || path == "/definitions/validate" || path == "/definitions/aspect":
		s.definitionsAPI(w, r, path)
	case path == "/catalog":
		if !method(w, r, http.MethodGet) || !s.systemStoreReady(w) {
			return
		}
		entries, err := s.store.SystemCatalog(r.Context())
		if s.storageError(w, err, "System catalog") {
			return
		}
		respond(w, 200, entries)
	case path == "/interfaces":
		if !method(w, r, http.MethodPost) {
			return
		}
		var design system.SystemDesign
		if !decode(w, r, &design) || !s.systemStoreReady(w) {
			return
		}
		inspection, err := s.store.InspectSystemPorts(r.Context(), design)
		if s.storageError(w, err, "System interfaces") {
			return
		}
		respond(w, 200, inspection)
	case path == "/resolve":
		if !method(w, r, http.MethodPost) {
			return
		}
		var design system.SystemDesign
		if !decode(w, r, &design) || !s.systemStoreReady(w) {
			return
		}
		resolution, err := s.store.ResolveSystem(r.Context(), design)
		if s.storageError(w, err, "System catalog") {
			return
		}
		respond(w, 200, resolution)
	case path == "/m1-example":
		if !method(w, r, http.MethodPost) || !s.systemStoreReady(w) {
			return
		}
		example, err := s.store.InstallM1Example(r.Context())
		if errors.Is(err, store.ErrStateMachineConflict) || errors.Is(err, store.ErrStateCatalogFull) || errors.Is(err, store.ErrDeliveryModelConflict) || errors.Is(err, store.ErrDeliveryCatalogFull) || errors.Is(err, store.ErrDefinitionConflict) || errors.Is(err, store.ErrDefinitionCapacity) || errors.Is(err, store.ErrDataContractConflict) || errors.Is(err, store.ErrDataCatalogFull) || errors.Is(err, store.ErrWorkflowPackageConflict) || errors.Is(err, store.ErrWorkflowCatalogFull) || errors.Is(err, store.ErrComponentConflict) || errors.Is(err, store.ErrCatalogFull) {
			fail(w, 409, "Could not install the full M1 example; existing versions are preserved. Resolve conflicts and retry.", nil)
			return
		}
		if s.storageError(w, err, "M1 acceptance example") {
			return
		}
		respond(w, 200, example)
	case path == "/m1-scenarios":
		if !method(w, r, http.MethodPost) || !s.systemStoreReady(w) {
			return
		}
		var body struct {
			Design system.SystemDesign `json:"design"`
		}
		if !decode(w, r, &body) {
			return
		}
		p, err := s.store.PrepareSystemExecution(r.Context(), body.Design)
		if err != nil {
			fail(w, 422, err.Error(), nil)
			return
		}
		cases, err := systemrun.M1Scenarios(p.Plan)
		if err != nil {
			fail(w, 422, err.Error(), nil)
			return
		}
		respond(w, 200, cases)
	case path == "/message-example":
		if !method(w, r, http.MethodPost) || !s.systemStoreReady(w) {
			return
		}
		example, err := s.store.InstallMessageExample(r.Context())
		if errors.Is(err, store.ErrStateMachineConflict) || errors.Is(err, store.ErrStateCatalogFull) || errors.Is(err, store.ErrDeliveryModelConflict) || errors.Is(err, store.ErrDeliveryCatalogFull) || errors.Is(err, store.ErrDefinitionConflict) || errors.Is(err, store.ErrDefinitionCapacity) {
			fail(w, 409, "Could not install the complete message example; existing versions were preserved. Resolve catalog conflicts and retry.", nil)
			return
		}
		if s.storageError(w, err, "Message example") {
			return
		}
		respond(w, 200, example)
	case path == "/state-execution-example" || path == "/multi-command-example":
		if !method(w, r, http.MethodPost) || !s.systemStoreReady(w) {
			return
		}
		var example system.StateExecutionExamplePack
		var err error
		if path == "/multi-command-example" {
			example, err = s.store.InstallMultiCommandExample(r.Context())
		} else {
			example, err = s.store.InstallStateExecutionExample(r.Context())
		}
		if errors.Is(err, store.ErrComponentConflict) || errors.Is(err, store.ErrCatalogFull) || errors.Is(err, store.ErrStateMachineConflict) || errors.Is(err, store.ErrStateCatalogFull) || errors.Is(err, store.ErrDefinitionConflict) || errors.Is(err, store.ErrDefinitionCapacity) {
			fail(w, 409, "Could not install the complete state example; installed versions were preserved. Resolve catalog conflicts and retry.", nil)
			return
		}
		if s.storageError(w, err, "State execution example") {
			return
		}
		respond(w, 200, example)
	case path == "/connection-example":
		if !method(w, r, http.MethodPost) || !s.systemStoreReady(w) {
			return
		}
		example, err := s.store.InstallConnectionExample(r.Context())
		if errors.Is(err, store.ErrComponentConflict) || errors.Is(err, store.ErrCatalogFull) {
			fail(w, 409, err.Error(), nil)
			return
		}
		if s.storageError(w, err, "Connection example") {
			return
		}
		respond(w, 200, example)
	case path == "/example":
		if method(w, r, http.MethodGet) {
			respond(w, 200, system.ReceptionExample())
		}
	case path == "/validate":
		if !method(w, r, http.MethodPost) {
			return
		}
		var design system.SystemDesign
		if !decode(w, r, &design) {
			return
		}
		respond(w, 200, system.Validate(design))
	case path == "/conformance":
		s.systemConformance(w, r)
	case path == "/evidence":
		if !method(w, r, http.MethodGet) {
			return
		}
		if !s.systemStoreReady(w) {
			return
		}
		records, err := s.store.SystemEvidenceList(r.Context(), r.URL.Query().Get("systemId"))
		if s.storageError(w, err, "System evidence") {
			return
		}
		respond(w, 200, records)
	case strings.HasPrefix(path, "/evidence/"):
		if !method(w, r, http.MethodGet) {
			return
		}
		if !s.systemStoreReady(w) {
			return
		}
		id := strings.TrimPrefix(path, "/evidence/")
		if !validID(id) {
			fail(w, 400, "Invalid evidence identifier.", nil)
			return
		}
		record, err := s.store.SystemEvidence(r.Context(), id)
		if s.storageError(w, err, "System evidence") {
			return
		}
		respond(w, 200, record)
	case path == "":
		if !method(w, r, http.MethodGet) {
			return
		}
		if !s.systemStoreReady(w) {
			return
		}
		designs, err := s.store.Systems(r.Context())
		if s.storageError(w, err, "Systems") {
			return
		}
		respond(w, 200, designs)
	default:
		id := strings.TrimPrefix(path, "/")
		if !validID(id) {
			fail(w, 400, "Invalid system identifier.", nil)
			return
		}
		if !s.systemStoreReady(w) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			design, err := s.store.System(r.Context(), id)
			if s.storageError(w, err, "System") {
				return
			}
			respond(w, 200, design)
		case http.MethodPut:
			var design system.SystemDesign
			if !decode(w, r, &design) {
				return
			}
			if design.ID != id {
				fail(w, 400, "System identifier must match its URL.", nil)
				return
			}
			validation := system.Validate(design)
			if !validation.Valid {
				fail(w, 422, "System design contains invalid definitions.", validation.Issues)
				return
			}
			if s.storageError(w, s.store.SaveSystem(r.Context(), design), "System") {
				return
			}
			respond(w, 200, design)
		default:
			w.Header().Set("Allow", "GET, PUT")
			fail(w, 405, "Method not allowed.", nil)
		}
	}
}

func (s *Server) systemStoreReady(w http.ResponseWriter) bool {
	if s.store == nil {
		fail(w, 503, "PostgreSQL storage is unavailable.", nil)
		return false
	}
	return true
}
func (s *Server) systemConformance(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var request struct {
		Design system.SystemDesign `json:"design"`
	}
	if !decode(w, r, &request) {
		return
	}
	validation := system.Validate(request.Design)
	if !validation.Valid {
		fail(w, 422, "System design contains invalid definitions.", validation.Issues)
		return
	}
	if !s.systemStoreReady(w) {
		return
	}
	select {
	case s.runs <- struct{}{}:
		defer func() { <-s.runs }()
	default:
		fail(w, 429, "Experiment capacity is busy; retry shortly.", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	resolution, err := s.store.ResolveSystem(ctx, request.Design)
	if err != nil {
		log.Printf("system catalog resolution: %v", err)
		fail(w, 503, "System catalog is unavailable. Conformance execution did not start.", nil)
		return
	}
	if !resolution.Valid {
		fail(w, 422, "Resolve missing or invalid definitions before conformance execution.", resolution.Issues)
		return
	}
	report, err := s.store.RunSystemConformance(ctx, request.Design)
	if err != nil {
		if errors.Is(err, conformance.ErrUnsupportedDesign) {
			fail(w, 422, "This experiment only supports the reception intake fixture. Restore its component definitions and bindings to run it.", nil)
			return
		}
		log.Printf("system conformance: %v", err)
		fail(w, 503, "Conformance execution unavailable. PostgreSQL must be reachable and permit an isolated experiment schema.", nil)
		return
	}
	id, err := newID("evidence")
	if err != nil {
		fail(w, 500, "Could not identify experiment.", nil)
		return
	}
	evidence, err := store.NewSystemEvidence(id, request.Design, report)
	if err != nil {
		fail(w, 500, "Could not encode experiment evidence.", nil)
		return
	}
	evidence.Resolution = &resolution
	if s.storageError(w, s.store.SaveSystemEvidence(ctx, evidence), "System evidence") {
		return
	}
	respond(w, 200, evidence)
}
