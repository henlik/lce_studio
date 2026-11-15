package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"software-studio/internal/store"
	"strings"
	"time"
)

func (s *Server) studioMaintenance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.systemStoreReady(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	path := strings.TrimPrefix(r.URL.Path, "/api/studio/")
	if r.Method == http.MethodGet {
		var result any
		var e error
		switch path {
		case "status":
			result, e = s.store.StudioStatus(ctx)
		case "backups":
			result, e = s.store.StudioBackups(ctx)
		case "backup":
			result, e = s.store.StudioBackup(ctx, r.URL.Query().Get("id"))
		default:
			fail(w, 404, "Unknown studio action", nil)
			return
		}
		if s.storageError(w, e, "Studio backup") {
			return
		}
		respond(w, 200, result)
		return
	}
	if !method(w, r, http.MethodPost) {
		return
	}
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		fail(w, 415, "Content-Type must be application/json", nil)
		return
	}
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxStudioBackupBytes+(1<<20))
	var body struct {
		Epoch        string              `json:"epoch"`
		Browser      store.BrowserBackup `json:"browser"`
		BackupID     string              `json:"backupId"`
		Confirmation string              `json:"confirmation"`
		Backup       *store.StudioBackup `json:"backup"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e = d.Decode(&body); e != nil {
		decodeError(w, e)
		return
	}
	if d.Decode(new(any)) != io.EOF {
		fail(w, 400, "Expected one request", nil)
		return
	}
	var result any
	switch path {
	case "backup":
		result, e = s.store.CreateStudioBackup(ctx, body.Epoch, body.Browser)
	case "validate-backup":
		if body.Backup == nil {
			e = errors.New("choose a backup")
		} else {
			e = store.ValidateStudioBackup(*body.Backup)
			result = map[string]any{"valid": e == nil, "rows": store.BackupRowCounts(*body.Backup)}
		}
	case "reset":
		if body.Confirmation != "RESET" {
			e = errors.New("type RESET to confirm clearing this studio")
		} else {
			result, e = s.store.ReplaceStudio(ctx, body.Epoch, body.BackupID, nil)
		}
	case "restore":
		if body.Confirmation != "RESTORE" || body.Backup == nil {
			e = errors.New("choose a backup and type RESTORE")
		} else {
			result, e = s.store.ReplaceStudio(ctx, body.Epoch, body.BackupID, body.Backup)
		}
	default:
		fail(w, 404, "Unknown studio action", nil)
		return
	}
	if e != nil {
		status := 422
		if errors.Is(e, store.ErrStudioChanged) {
			status = 409
		}
		fail(w, status, e.Error(), nil)
		return
	}
	respond(w, 200, result)
}
