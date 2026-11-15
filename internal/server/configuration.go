package server

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"software-studio/internal/configuration"
	"software-studio/internal/store"
	"strings"
)

func (s *Server) configurationAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/configuration")
	if r.Method == http.MethodGet {
		switch path {
		case "/example":
			ctx, done, ok := s.componentComputation(w, r)
			if !ok {
				return
			}
			defer done()
			_ = ctx
			var p configuration.Project
			var e error
			if r.URL.Query().Get("template") == "hierarchy" {
				p = configuration.HierarchyProject()
			} else {
				p, e = configuration.InspectionProject()
			}
			if e != nil {
				fail(w, 500, e.Error(), nil)
				return
			}
			respond(w, 200, p)
			return
		case "/packages":
			if !s.systemStoreReady(w) {
				return
			}
			ps, e := s.store.ConfigurationPackages(r.Context())
			if s.storageError(w, e, "Configuration packages") {
				return
			}
			respond(w, 200, ps)
			return
		case "/package":
			if !s.systemStoreReady(w) {
				return
			}
			p, e := s.store.ConfigurationPackage(r.Context(), r.URL.Query().Get("id"), r.URL.Query().Get("version"))
			if s.storageError(w, e, "Configuration package") {
				return
			}
			respond(w, 200, p)
			return
		}
	}
	if !method(w, r, http.MethodPost) {
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		fail(w, 415, "Content-Type must be application/json", nil)
		return
	}
	defer r.Body.Close()
	var body struct {
		configuration.EditRequest
		ID       string                   `json:"id"`
		Version  string                   `json:"version"`
		Scope    *configuration.TestScope `json:"scope"`
		Test     bool                     `json:"test"`
		Source   string                   `json:"source"`
		Package  configuration.Package    `json:"package"`
		Previous configuration.Package    `json:"previous"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	d.UseNumber()
	if e := d.Decode(&body); e != nil {
		decodeError(w, e)
		return
	}
	if e := d.Decode(&struct{}{}); e != io.EOF {
		fail(w, 400, "Expected one JSON request", nil)
		return
	}
	ctx, done, ok := s.componentComputation(w, r)
	if !ok {
		return
	}
	defer done()
	var result any
	var e error
	switch path {
	case "/compile":
		result = configuration.Compile(body.Project)
	case "/run":
		result, e = configuration.RunScoped(ctx, body.Project, body.Scope)
	case "/testbench":
		result, e = configuration.NewTestbench(body.EditRequest)
	case "/component":
		result, e = configuration.NewComponent(body.EditRequest)
	case "/extract":
		result, e = configuration.ExtractPrimitive(body.EditRequest)
	case "/organize":
		result, e = configuration.Organize(body.EditRequest)
	case "/format":
		result, e = configuration.Format(body.EditRequest)
	case "/rename":
		result, e = configuration.Rename(body.EditRequest)
	case "/patch":
		result, e = configuration.Patch(body.EditRequest)
	case "/complete":
		result = configuration.Complete(body.Project, body.File, body.Declaration, body.Path, body.Kind)
	case "/definition":
		result, e = configuration.Definition(body.Project, body.File, body.Offset)
	case "/pack":
		result, e = configuration.Pack(ctx, body.ID, body.Version, body.Project, body.Test)
	case "/verify":
		result, e = configuration.VerifyPackage(body.Package)
	case "/replay":
		result, e = configuration.Replay(ctx, body.Package)
	case "/compare":
		result, e = configuration.Compare(body.Previous, body.Package)
	case "/migrate":
		result, e = configuration.MigrateLegacy(body.Source)
	case "/install":
		if !s.systemStoreReady(w) {
			return
		}
		result, e = s.store.InstallConfiguration(ctx, body.Package)
	default:
		fail(w, 404, "Unknown configuration action", nil)
		return
	}
	if e != nil {
		status := 422
		if strings.Contains(e.Error(), "source conflict") || errors.Is(e, store.ErrConfigurationConflict) || errors.Is(e, store.ErrConfigurationCapacity) {
			status = 409
		}
		fail(w, status, e.Error(), nil)
		return
	}
	respond(w, 200, result)
}
