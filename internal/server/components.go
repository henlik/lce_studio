package server

import (
	"errors"
	"log"
	"net/http"

	"software-studio/internal/model"
	"software-studio/internal/store"
)

func (s *Server) components(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.Components(r.Context())
		if s.storageError(w, err, "Component catalog") {
			return
		}
		respond(w, http.StatusOK, records)
	case http.MethodPost:
		var p model.ComponentPackage
		if !decode(w, r, &p) {
			return
		}
		if p.SchemaVersion == 2 || p.SchemaVersion == 3 {
			fail(w, http.StatusUnprocessableEntity, "Composite packages require their complete dependency closure. Install through /api/components/bundles.", nil)
			return
		}
		if issues := model.ValidateComponentPackage(p); len(issues) > 0 {
			fail(w, http.StatusUnprocessableEntity, "Component package contains invalid fields.", issues)
			return
		}
		record, created, err := s.store.InstallComponent(r.Context(), p)
		if errors.Is(err, store.ErrComponentConflict) {
			fail(w, http.StatusConflict, "This component ID and version are already installed with different content. Publish a new version instead.", nil)
			return
		}
		if errors.Is(err, store.ErrCatalogFull) {
			fail(w, http.StatusConflict, "The catalog supports at most 200 installed component versions.", nil)
			return
		}
		if s.storageError(w, err, "Component catalog") {
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		respond(w, status, record)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, http.StatusMethodNotAllowed, "Method not allowed.", nil)
	}
}

func (s *Server) componentExamples(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	packages := model.ExampleComponentPackages()
	locks := make([]model.PackageLock, 0, len(packages))
	for _, p := range packages {
		digest, err := model.ComponentDigest(p)
		if err != nil {
			log.Printf("component example digest: %v", err)
			fail(w, http.StatusInternalServerError, "Could not load component examples.", nil)
			return
		}
		locks = append(locks, model.PackageLock{Package: p, Digest: digest})
	}
	respond(w, http.StatusOK, locks)
}
