package server

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store"
)

// The cloud's few pages, for a browser: signing in with GitHub and
// approving a device's code. Everything else is the API.

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>enowX Cloud</title>
<style>
:root{color-scheme:light dark;--bg:#fff;--fg:#111;--muted:#666;--line:#ddd;--accent:#111;--on:#fff}
@media (prefers-color-scheme:dark){:root{--bg:#000;--fg:#eee;--muted:#999;--line:#333;--accent:#eee;--on:#000}}
body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.5 system-ui,sans-serif}
main{max-width:420px;margin:12vh auto;padding:0 16px}
h1{font-size:20px;margin:0 0 16px}p{margin:0 0 14px}.muted{color:var(--muted);font-size:13px}
.box{border:1px solid var(--line);border-radius:10px;padding:14px 16px;margin:0 0 16px}
input{font:600 20px/1 ui-monospace,monospace;letter-spacing:.12em;text-transform:uppercase;width:100%;box-sizing:border-box;padding:10px 12px;border:1px solid var(--line);border-radius:8px;background:transparent;color:var(--fg)}
button,a.button{display:inline-block;font:inherit;font-weight:600;padding:9px 16px;border-radius:8px;border:0;background:var(--accent);color:var(--on);text-decoration:none;cursor:pointer;margin-top:12px}
form.inline{display:inline}button.plain{background:none;color:var(--muted);padding:0;font-weight:400;text-decoration:underline}
</style></head><body><main>
<h1>enowX Cloud</h1>
{{if .Message}}<p>{{.Message}}</p>{{end}}
{{if .Device}}<div class="box"><p><b>{{.Device.Name}}</b> ({{if eq .Device.Kind "host"}}a host's daemon{{else}}the enowX app{{end}}) asks to sign in to your account.</p>
<p class="muted">Approve it only if you started this on that device, just now.</p>
<form method="post" action="/device"><input type="hidden" name="code" value="{{.Code}}"><button>Approve</button></form></div>
{{else if .AskCode}}<form method="get" action="/device"><p>Type the code the device shows.</p><input name="code" value="{{.Code}}" autocomplete="off" autofocus placeholder="XXXX-XXXX"><button>Continue</button></form>{{end}}
{{if .SignInURL}}<a class="button" href="{{.SignInURL}}">Sign in with GitHub</a>{{end}}
{{if .Account}}<p class="muted">Signed in as {{.Account.Name}}{{if .Account.GitHub}} (@{{.Account.GitHub}}){{end}} · <form class="inline" method="post" action="/signout"><button class="plain">Sign out</button></form></p>{{end}}
</main></body></html>`))

type pageData struct {
	Message   string
	Account   *api.Account
	Device    *api.PendingDevice
	Code      string
	AskCode   bool
	SignInURL string
}

// page answers with a page saying msg, its data d (nil: the message only).
func (s *Server) page(w http.ResponseWriter, status int, msg string, d *pageData) {
	if d == nil {
		d = &pageData{}
	}
	d.Message = msg
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY") // approving can't be clicked in someone's frame
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	pageTmpl.Execute(w, d)
}

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
		s.page(w, http.StatusOK, "Teams in enowX: their members, shared notes and canvases, and hosts. Sign in from the app, or here.", &pageData{SignInURL: s.signInURL("/")})
		return
	}
	s.page(w, http.StatusOK, "You're signed in. Approve a device at /device with the code it shows.", &pageData{Account: acc})
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
	d := &pageData{Account: acc, Code: code, AskCode: true}
	if code == "" {
		s.page(w, http.StatusOK, "", d)
		return
	}
	pending, err := s.db.PendingDevice(r.Context(), code)
	if err != nil {
		s.page(w, http.StatusNotFound, "No device waits with this code: it expired, or was mistyped.", d)
		return
	}
	d.Device = &pending
	s.page(w, http.StatusOK, "", d)
}

// approve signs the device of a code in to the browser's account.
func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	acc := s.webAccount(r)
	if acc == nil {
		s.page(w, http.StatusUnauthorized, "Sign in first.", &pageData{SignInURL: s.signInURL("/device")})
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
		s.page(w, http.StatusNotFound, "No device waits with this code: it expired, or was mistyped.", &pageData{Account: acc, AskCode: true})
	case err != nil:
		s.page(w, http.StatusInternalServerError, err.Error(), nil)
	default:
		s.page(w, http.StatusOK, pending.Name+" is signed in to your account. You can go back to it.", &pageData{Account: acc})
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
