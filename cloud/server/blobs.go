package server

import (
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/blobstore"
	"github.com/enowx/enowx/cloud/store"
)

// A team's files (api.PathTeamBlob): the database says who sent each and
// may read or delete it, the file store keeps the sealed bytes, under the
// team's ID and the file's, which its sender picked at random.

var blobID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func blobKey(team, id string) string { return "teams/" + team + "/" + id }

// blobRef is the request's team and file, false (answered) when they
// can't be, or the server keeps no files.
func (s *Server) blobRef(w http.ResponseWriter, r *http.Request) (team, id string, ok bool) {
	team, id = r.PathValue("team"), r.PathValue("blob")
	switch {
	case s.cfg.Blobs == nil:
		s.fail(w, http.StatusNotImplemented, "this server keeps no files")
	case !blobID.MatchString(id):
		s.fail(w, http.StatusBadRequest, "a file's ID is 32 hex digits")
	default:
		return team, id, true
	}
	return "", "", false
}

func (s *Server) putBlob(w http.ResponseWriter, r *http.Request) {
	team, id, ok := s.blobRef(w, r)
	if !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, api.MaxBlob))
	if err != nil {
		s.fail(w, http.StatusRequestEntityTooLarge, "a file is 8 MB at most")
		return
	}
	if len(data) == 0 {
		s.fail(w, http.StatusBadRequest, "an empty file")
		return
	}
	me := asker(r).Account.ID
	if err := s.db.AddBlob(r.Context(), team, me, id, int64(len(data))); err != nil {
		s.answer(w, nil, err)
		return
	}
	if err := s.cfg.Blobs.Put(r.Context(), blobKey(team, id), data); err != nil {
		_ = s.db.DeleteBlob(r.Context(), team, me, id) // not kept: may be sent again
		s.internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getBlob(w http.ResponseWriter, r *http.Request) {
	team, id, ok := s.blobRef(w, r)
	if !ok {
		return
	}
	if err := s.db.Blob(r.Context(), team, asker(r).Account.ID, id); err != nil {
		s.answer(w, nil, err)
		return
	}
	data, err := s.cfg.Blobs.Get(r.Context(), blobKey(team, id))
	if errors.Is(err, blobstore.ErrNotFound) {
		s.answer(w, nil, store.ErrNotFound)
		return
	}
	if err != nil {
		s.internal(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}

func (s *Server) deleteBlob(w http.ResponseWriter, r *http.Request) {
	team, id, ok := s.blobRef(w, r)
	if !ok {
		return
	}
	if err := s.db.DeleteBlob(r.Context(), team, asker(r).Account.ID, id); err != nil {
		s.answer(w, nil, err)
		return
	}
	if err := s.cfg.Blobs.Delete(r.Context(), blobKey(team, id)); err != nil {
		s.log.Warn("blobs: a deleted file's content stays", "team", team, "blob", id, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}
