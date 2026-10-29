package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store"
)

// The cloud's few pages, for a browser: signing in with GitHub and
// approving a device's code (page.go draws them). Everything else is the
// API.

// webAccount is the account the browser is signed in as, nil when none.
func (s *Server) webAccount(r *http.Request) *api.Account {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	acc, err := s.db.WebSessionAccount(r.Context(), c.Value)
	if err != nil {
		return nil
	}
	return &acc
}

func (s *Server) signInURL(next string) string {
	if !s.githubOn() {
		return ""
	}
	return pathGitHub + "?next=" + url.QueryEscape(next)
}

// home says who is signed in.
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.page(w, http.StatusNotFound, "Nothing here.", nil)
		return
	}
	acc := s.webAccount(r)
	if acc == nil {
		s.page(w, http.StatusOK, "", &pageData{Title: "Sign in", Lead: "Your teams in enowX: their members, the notes and canvases they share, and their hosts.", SignInURL: s.signInURL("/")})
		return
	}
	s.page(w, http.StatusOK, "", &pageData{Title: "Sign in a device", Lead: "Type the code the enowX app or your host shows.", Account: acc, AskCode: true})
}

// device shows what waits behind a code, to approve; signing in first.
func (s *Server) device(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	acc := s.webAccount(r)
	if acc == nil {
		next := "/device"
		if code != "" {
			next += "?code=" + url.QueryEscape(code)
		}
		if u := s.signInURL(next); u != "" {
			http.Redirect(w, r, u, http.StatusFound)
			return
		}
		s.page(w, http.StatusNotFound, "Signing in on this page isn't set up on this server: approve the device from the enowX app.", nil)
		return
	}
	d := &pageData{Title: "Sign in a device", Lead: "Type the code the enowX app or your host shows.", Account: acc, Code: code, AskCode: true}
	if code == "" {
		s.page(w, http.StatusOK, "", d)
		return
	}
	pending, err := s.db.PendingDevice(r.Context(), code)
	if err != nil {
		s.page(w, http.StatusNotFound, "No device waits with this code: it expired, or was mistyped.", d)
		return
	}
	d.Title, d.Lead, d.Device = "Approve this device?", "It asks to sign in to your enowX Cloud account.", &pending
	if c := store.NormalUserCode(code); len(c) == 8 {
		d.Code = c[:4] + "-" + c[4:] // as the device shows it
	}
	s.page(w, http.StatusOK, "", d)
}

// approve signs the device of a code in to the browser's account.
func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	acc := s.webAccount(r)
	if acc == nil {
		s.page(w, http.StatusUnauthorized, "Sign in first.", &pageData{Title: "Sign in a device", SignInURL: s.signInURL("/device")})
		return
	}
	if !s.sameOrigin(r) {
		s.page(w, http.StatusForbidden, "This approval didn't come from this page.", nil)
		return
	}
	code := r.PostFormValue("code")
	pending, err := s.db.PendingDevice(r.Context(), code)
	if err == nil {
		err = s.db.ApproveDeviceCode(r.Context(), code, acc.ID)
	}
	switch {
	case errors.Is(err, store.ErrExpired):
		s.page(w, http.StatusNotFound, "No device waits with this code: it expired, or was mistyped.", &pageData{Title: "Sign in a device", Account: acc, AskCode: true})
	case err != nil:
		s.page(w, http.StatusInternalServerError, err.Error(), nil)
	default:
		s.page(w, http.StatusOK, "", &pageData{Title: "Device signed in", Done: pending.Name + " is signed in to your account.", Lead: "You can close this page and go back to it.", Account: acc})
	}
}

// webSignOut signs the browser out of the cloud's pages.
func (s *Server) webSignOut(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		s.page(w, http.StatusForbidden, "This didn't come from this page.", nil)
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.db.EndWebSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// sameOrigin says a form was posted from the server's own pages: the
// session cookie is SameSite=Lax already, this is the second lock.
func (s *Server) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	if origin == "" {
		return false
	}
	want := s.base()
	if want == "" {
		want = "http://" + r.Host
	}
	return origin == want || strings.HasPrefix(origin, want+"/")
}
