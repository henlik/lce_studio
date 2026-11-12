// Package server exposes the local Software Studio API and production frontend.
package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/simulation"
	"software-studio/internal/store"
)

const MaxBodyBytes = 2 << 20

type Server struct {
	store  *store.Store
	webDir string
	runs   chan struct{}
}

type errorResponse struct {
	Error  string                  `json:"error"`
	Issues []model.ValidationIssue `json:"issues,omitempty"`
}

func New(db *store.Store, webDir string) *Server {
	return &Server{store: db, webDir: webDir, runs: make(chan struct{}, 2)}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'")
	if r.URL.Path != "/api" && !strings.HasPrefix(r.URL.Path, "/api/") {
		s.serveWeb(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if !sameOrigin(r) {
			fail(w, http.StatusForbidden, "Cross-origin writes are not allowed.", nil)
			return
		}
	}
	path := r.URL.Path
	switch {
	case path == "/api/health":
		if !method(w, r, http.MethodGet) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.store.Ping(ctx); err != nil {
			fail(w, http.StatusServiceUnavailable, "Database unavailable.", nil)
			return
		}
		respond(w, 200, map[string]string{"status": "ok", "version": "0.2.0"})
	case path == "/api/example":
		if !method(w, r, http.MethodGet) {
			return
		}
		switch r.URL.Query().Get("template") {
		case "", "retry":
			respond(w, 200, model.ExampleProject())
		case "continuum":
			respond(w, 200, model.ContinuumExampleProject())
		default:
			fail(w, http.StatusBadRequest, "Unknown example template.", nil)
		}
	case path == "/api/validate":
		if !method(w, r, http.MethodPost) {
			return
		}
		var p model.Project
		if !decode(w, r, &p) {
			return
		}
		issues := model.Validate(p)
		if r.URL.Query().Get("draft") == "true" {
			issues = model.ValidateDraft(p)
		}
		if issues == nil {
			issues = []model.ValidationIssue{}
		}
		respond(w, 200, struct {
			Valid  bool                    `json:"valid"`
			Issues []model.ValidationIssue `json:"issues"`
		}{len(issues) == 0, issues})
	case path == "/api/projects":
		s.projects(w, r)
	case strings.HasPrefix(path, "/api/projects/"):
		id := strings.TrimPrefix(path, "/api/projects/")
		if !validID(id) {
			fail(w, 404, "Project not found.", nil)
			return
		}
		s.project(w, r, id)
	case path == "/api/runs":
		s.runCollection(w, r)
	case strings.HasPrefix(path, "/api/runs/"):
		id := strings.TrimPrefix(path, "/api/runs/")
		if !validID(id) {
			fail(w, 404, "Run not found.", nil)
			return
		}
		if !method(w, r, http.MethodGet) {
			return
		}
		run, err := s.store.Run(r.Context(), id)
		if s.storageError(w, err, "Run") {
			return
		}
		respond(w, 200, run)
	default:
		fail(w, 404, "API route not found.", nil)
	}
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projects, err := s.store.Projects(r.Context())
		if s.storageError(w, err, "Projects") {
			return
		}
		respond(w, 200, projects)
	case http.MethodPost:
		var p model.Project
		if !decode(w, r, &p) {
			return
		}
		if p.ID == "" {
			id, err := newID("project")
			if err != nil {
				fail(w, 500, "Could not create project ID.", nil)
				return
			}
			p.ID = id
		}
		if issues := model.ValidateDraft(p); len(issues) > 0 {
			fail(w, 422, "Project contains invalid fields.", issues)
			return
		}
		if s.storageError(w, s.store.SaveProject(r.Context(), p), "Project") {
			return
		}
		respond(w, 200, p)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, "Method not allowed.", nil)
	}
}

func (s *Server) project(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		p, err := s.store.Project(r.Context(), id)
		if s.storageError(w, err, "Project") {
			return
		}
		respond(w, 200, p)
	case http.MethodDelete:
		if s.storageError(w, s.store.DeleteProject(r.Context(), id), "Project") {
			return
		}
		w.WriteHeader(204)
	default:
		w.Header().Set("Allow", "GET, DELETE")
		fail(w, 405, "Method not allowed.", nil)
	}
}

func (s *Server) runCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projectID := r.URL.Query().Get("projectId")
		if projectID != "" && !validID(projectID) {
			fail(w, 400, "Invalid projectId.", nil)
			return
		}
		runs, err := s.store.Runs(r.Context(), projectID)
		if s.storageError(w, err, "Runs") {
			return
		}
		respond(w, 200, runs)
	case http.MethodPost:
		var body struct {
			Project model.Project `json:"project"`
		}
		if !decode(w, r, &body) {
			return
		}
		if issues := model.Validate(body.Project); len(issues) > 0 {
			fail(w, 422, "Model cannot be simulated.", issues)
			return
		}
		select {
		case s.runs <- struct{}{}:
			defer func() { <-s.runs }()
		default:
			fail(w, 429, "Two simulations are already running. Please try again shortly.", nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		run, err := simulation.Run(ctx, body.Project)
		if err != nil {
			if ctx.Err() != nil {
				fail(w, 408, "Simulation was cancelled or exceeded its execution time limit.", nil)
			} else {
				fail(w, 422, err.Error(), nil)
			}
			return
		}
		id, err := newID("run")
		if err != nil {
			fail(w, 500, "Could not create run ID.", nil)
			return
		}
		run.ID = id
		run.ProjectID = body.Project.ID
		run.Project = body.Project
		run.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		run.ModelHash = ModelHash(body.Project)
		if run.Events == nil {
			run.Events = []model.TraceEvent{}
		}
		if run.Verdicts == nil {
			run.Verdicts = []model.Verdict{}
		}
		if s.storageError(w, s.store.SaveRun(r.Context(), run), "Run") {
			return
		}
		respond(w, http.StatusCreated, run)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, "Method not allowed.", nil)
	}
}

// ModelHash identifies the full typed JSON snapshot, including presentation,
// names, requirements and scenario. It is not a behavioural-equivalence hash.
// Struct field order is fixed; array order is significant.
func ModelHash(p model.Project) string {
	body, _ := json.Marshal(p) // JSON decoding and model validation exclude nonfinite numbers.
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (s *Server) storageError(w http.ResponseWriter, err error, entity string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, store.ErrNotFound) {
		fail(w, 404, entity+" not found.", nil)
	} else {
		log.Printf("storage: %v", err)
		fail(w, 500, "Could not access local storage.", nil)
	}
	return true
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

func validID(id string) bool { return idPattern.MatchString(id) }

func newID(prefix string) (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes[:]), nil
}

func sameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	} // CLI clients are permitted; browser writes use JSON.
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && strings.EqualFold(u.Host, r.Host)
}

func decode(w http.ResponseWriter, r *http.Request, dest any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		fail(w, 415, "Content-Type must be application/json.", nil)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dest); err != nil {
		decodeError(w, err)
		return false
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		if err == nil {
			fail(w, 400, "Request body must contain exactly one JSON value.", nil)
		} else {
			decodeError(w, err)
		}
		return false
	}
	return true
}

func decodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		fail(w, 413, "Request body exceeds 2 MiB.", nil)
		return
	}
	fail(w, 400, "Invalid JSON: "+err.Error(), nil)
}

func method(w http.ResponseWriter, r *http.Request, allowed string) bool {
	if r.Method == allowed {
		return true
	}
	w.Header().Set("Allow", allowed)
	fail(w, 405, "Method not allowed.", nil)
	return false
}

func fail(w http.ResponseWriter, status int, message string, issues []model.ValidationIssue) {
	respond(w, status, errorResponse{message, issues})
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func (s *Server) serveWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if s.webDir == "" {
		http.Error(w, "Frontend unavailable. Run npm --prefix web run build.", 503)
		return
	}
	clean := filepath.Clean("/" + r.URL.Path)
	file := filepath.Join(s.webDir, strings.TrimPrefix(clean, "/"))
	info, err := os.Stat(file)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, file)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") || filepath.Ext(r.URL.Path) != "" {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(s.webDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.Error(w, "Frontend unavailable. Run npm --prefix web run build.", 503)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, index)
}

// AddrForLog makes wildcard-bound addresses usable in the startup message.
func AddrForLog(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return fmt.Sprintf("http://%s", addr)
}
