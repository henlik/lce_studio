package server

import (
	"net/http"
	sourceexamples "software-studio/examples/source"
	"software-studio/internal/source"
)

func (s *Server) sourceAPI(w http.ResponseWriter, r *http.Request, path string) {
	if path == "/source/example" {
		if !method(w, r, http.MethodGet) {
			return
		}
		respond(w, 200, map[string]string{"language": source.Version, "source": sourceexamples.M1})
		return
	}
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Source string `json:"source"`
	}
	if !decode(w, r, &body) {
		return
	}
	// Compilation is pure. Catalog resolution and execution use the existing APIs.
	respond(w, 200, source.Compile(body.Source))
}
