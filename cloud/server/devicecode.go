package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store"
)

// maxBody bounds a request's JSON: none of the API's is large.
const maxBody = 64 << 10

// decode reads r's JSON body into v, answering 400 itself when it can't.
func (s *Server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(v); err != nil {
		s.fail(w, http.StatusBadRequest, "can't read the request: "+err.Error())
		return false
	}
	return true
}

// deviceCode starts signing a device in with a code.
func (s *Server) deviceCode(w http.ResponseWriter, r *http.Request) {
	var p api.DeviceCodeParams
	if !s.decode(w, r, &p) {
		return
	}
	dc, err := s.db.StartDeviceCode(r.Context(), p)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err.Error())
		return
	}
	// VerifyURL stays empty until the cloud has a page to approve codes on
	// (it comes with signing in with GitHub): an app signed in approves.
	s.reply(w, dc)
}

// deviceToken gives a device its token once its code is approved.
func (s *Server) deviceToken(w http.ResponseWriter, r *http.Request) {
	var p api.DeviceTokenParams
	if !s.decode(w, r, &p) {
		return
	}
	d, token, accID, err := s.db.ClaimDeviceCode(r.Context(), p.DeviceCode)
	switch {
	case errors.Is(err, store.ErrPending):
		s.fail(w, http.StatusBadRequest, api.ErrPending)
		return
	case errors.Is(err, store.ErrExpired):
		s.fail(w, http.StatusBadRequest, api.ErrExpired)
		return
	case err != nil:
		s.fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	acc, err := s.db.Account(r.Context(), accID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	d.Current = true
	s.reply(w, api.SignedIn{Token: token, Me: api.Me{Account: acc, Device: d}})
}

// pendingDevice says what waits behind a user code, before it's approved.
func (s *Server) pendingDevice(w http.ResponseWriter, r *http.Request) {
	d, err := s.db.PendingDevice(r.Context(), r.PathValue("code"))
	switch {
	case errors.Is(err, store.ErrExpired):
		s.fail(w, http.StatusNotFound, "no device waits with this code: it expired, or was mistyped")
	case err != nil:
		s.fail(w, http.StatusInternalServerError, err.Error())
	default:
		s.reply(w, d)
	}
}

// approveDevice signs the device showing a user code in to the asker's
// account.
func (s *Server) approveDevice(w http.ResponseWriter, r *http.Request) {
	var p api.DeviceApproveParams
	if !s.decode(w, r, &p) {
		return
	}
	err := s.db.ApproveDeviceCode(r.Context(), p.UserCode, asker(r).Account.ID)
	switch {
	case errors.Is(err, store.ErrExpired):
		s.fail(w, http.StatusNotFound, "no device waits with this code: it expired, or was mistyped")
	case err != nil:
		s.fail(w, http.StatusInternalServerError, err.Error())
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
