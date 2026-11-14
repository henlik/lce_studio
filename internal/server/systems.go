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
)

func (s *Server) systemsAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/systems")
	switch {
	case path == "/catalog":
		if !method(w, r, http.MethodGet) || !s.systemStoreReady(w) {
			return
		}
		entries, err := s.store.SystemCatalog(r.Context())
		if s.storageError(w, err, "System catalog") {
			return
		}
		respond(w, 200, entries)
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
