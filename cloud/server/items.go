package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store"
)

// A team's items: what changed after a sequence number, and writing one
// at its next version (docs/cloud.md, API).

func (s *Server) items(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	list, err := s.db.Items(r.Context(), r.PathValue("team"), asker(r).Account.ID, after)
	s.answer(w, list, err)
}

func (s *Server) putItem(w http.ResponseWriter, r *http.Request) {
	var p api.PutItemParams
	// Base64 makes the sealed content a third bigger.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, api.MaxItemSize/3*4+4096)).Decode(&p); err != nil {
		s.fail(w, http.StatusBadRequest, "can't read the item: "+err.Error())
		return
	}
	me := asker(r)
	it, err := s.db.PutItem(r.Context(), r.PathValue("team"), me.Account.ID, me.Device.ID, r.PathValue("id"), p)
	var conflict *store.ConflictError
	if errors.As(err, &conflict) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(conflict.Item)
		return
	}
	s.answer(w, it, err)
}
