package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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
	dc, err := s.db.StartDeviceCode(r.Context(), p, clientAddr(r), r.Header.Get("CF-IPCountry"))
	if err != nil {
		s.answer(w, nil, err)
		return
	}
	// Approved on the cloud's page, signed in with GitHub, or from an app
	// already signed in.
	if s.githubOn() {
		dc.VerifyURL = s.base() + "/device"
		dc.VerifyURLComplete = dc.VerifyURL + "?code=" + dc.UserCode
	}
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
		s.internal(w, err)
		return
	}
	acc, err := s.db.Account(r.Context(), accID)
	if err != nil {
		s.internal(w, err)
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
		s.answer(w, nil, err)
	default:
		d.SameNetwork = d.From != "" && d.From == clientAddr(r)
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
		s.internal(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// deviceOrigin says when and where a device's sign-in started.
func deviceOrigin(d api.PendingDevice) string {
	age := time.Since(d.Created).Round(time.Second)
	out := "Started " + age.String() + " ago"
	if age >= time.Minute {
		out = "Started " + strconv.Itoa(int(age.Minutes())) + " min ago"
	}
	if d.From != "" {
		out += " from " + d.From
		if d.Place != "" && d.Place != "XX" {
			out += " (" + d.Place + ")"
		}
	}
	return out
}
