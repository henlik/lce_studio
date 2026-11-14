package server

import (
	"errors"
	"net/http"
	"software-studio/internal/store"
	"software-studio/internal/system"
)

func (s *Server) definitionsAPI(w http.ResponseWriter, r *http.Request, path string) {
	if !s.systemStoreReady(w) {
		return
	}
	if path == "/definitions" && r.Method == http.MethodGet {
		records, err := s.store.Definitions(r.Context())
		if s.storageError(w, err, "Component definitions") {
			return
		}
		respond(w, 200, records)
		return
	}
	if !method(w, r, http.MethodPost) {
		return
	}
	if path == "/definitions/aspect" {
		var aspect system.AspectRef
		if !decode(w, r, &aspect) {
			return
		}
		result, err := s.store.AspectEditor(r.Context(), aspect)
		if s.definitionError(w, err) {
			return
		}
		respond(w, 200, result)
		return
	}
	var d system.ComponentDefinition
	if !decode(w, r, &d) {
		return
	}
	if path == "/definitions/validate" {
		checked, err := s.store.CheckDefinition(r.Context(), d)
		if s.storageError(w, err, "Definition compatibility") {
			return
		}
		respond(w, 200, checked)
		return
	}
	result, err := s.store.InstallDefinition(r.Context(), d)
	if s.definitionError(w, err) {
		return
	}
	respond(w, 200, result)
}
func (s *Server) definitionError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var invalid *store.DefinitionValidationError
	if errors.As(err, &invalid) {
		fail(w, 422, invalid.Error(), invalid.Issues)
		return true
	}
	if errors.Is(err, store.ErrDefinitionConflict) || errors.Is(err, store.ErrDefinitionCapacity) {
		fail(w, 409, err.Error(), nil)
		return true
	}
	if errors.Is(err, store.ErrNotComposite) {
		fail(w, 422, "This timing profile has no behavior editor. Choose a composite behavior package to edit.", nil)
		return true
	}
	return s.storageError(w, err, "Component definition")
}
