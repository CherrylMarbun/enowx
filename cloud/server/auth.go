package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store"
)

type meKey struct{}

// signedIn serves h to a request with a device's token, who asks in its
// context (asker), and answers 401 to one without.
func (s *Server) signedIn(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			s.fail(w, http.StatusUnauthorized, "sign in first")
			return
		}
		me, err := s.db.Me(r.Context(), token)
		if errors.Is(err, store.ErrNotFound) {
			s.fail(w, http.StatusUnauthorized, "this device is signed out")
			return
		}
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), meKey{}, me)))
	}
}

// asker is who asks, in a handler signedIn serves.
func asker(r *http.Request) api.Me { return r.Context().Value(meKey{}).(api.Me) }

func (s *Server) me(w http.ResponseWriter, r *http.Request) { s.reply(w, asker(r)) }

func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	me := asker(r)
	list, err := s.db.Devices(r.Context(), me.Account.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range list {
		list[i].Current = list[i].ID == me.Device.ID
	}
	s.reply(w, list)
}

// removeDevice signs one of the asker's devices out.
func (s *Server) removeDevice(w http.ResponseWriter, r *http.Request) {
	s.signOutDevice(w, r, r.PathValue("id"))
}

// signOut signs the asking device out.
func (s *Server) signOut(w http.ResponseWriter, r *http.Request) {
	s.signOutDevice(w, r, asker(r).Device.ID)
}

func (s *Server) signOutDevice(w http.ResponseWriter, r *http.Request, id string) {
	err := s.db.RemoveDevice(r.Context(), asker(r).Account.ID, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.fail(w, http.StatusNotFound, "no such device")
	case err != nil:
		s.fail(w, http.StatusInternalServerError, err.Error())
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
