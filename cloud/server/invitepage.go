package server

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store"
)

// An invite's page: its link opened in a browser, by someone who may have
// neither the app nor an account yet. The token is after the link's "#",
// so it never reaches the server's logs; the page's one script posts it,
// and the server keeps it in a cookie of the page's own while the visitor
// signs in with GitHub (making their account) and joins. The app, once
// downloaded and signed in with the same GitHub account, finds the team.

const (
	inviteCookie = "enowx_invite"
	downloadURL  = "https://enowx.ai"
)

// inviteScript posts the token after the link's "#" to the server, and
// takes it out of the address bar.
const inviteScript = `(function(){var t=location.hash.slice(1);if(t.indexOf("enxi_")!==0)return;history.replaceState(null,"",location.pathname);var f=document.getElementById("open");f.elements.token.value=t;f.submit();})();`

// inviteScriptHash lets the page run inviteScript, and no other script.
var inviteScriptHash = func() string {
	h := sha256.Sum256([]byte(inviteScript))
	return "'sha256-" + base64.StdEncoding.EncodeToString(h[:]) + "'"
}()

func (s *Server) setInviteCookie(w http.ResponseWriter, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: inviteCookie, Value: token, Path: "/invite", MaxAge: age,
		HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode})
}

// invitePage shows the invite the browser holds: to sign in for, or to
// join; without one, it reads the link's token.
func (s *Server) invitePage(w http.ResponseWriter, r *http.Request) {
	acc := s.webAccount(r)
	d := &pageData{Title: "Join a team on enowX", Account: acc, Download: true}
	c, err := r.Cookie(inviteCookie)
	if err != nil || c.Value == "" {
		d.Lead, d.Script = "Opening your invite…", template.JS(inviteScript)
		s.page(w, http.StatusOK, "", d)
		return
	}
	peek, err := s.db.PeekInvite(r.Context(), c.Value)
	if err != nil {
		s.setInviteCookie(w, "", -1)
		s.page(w, http.StatusNotFound, inviteGone(err), d)
		return
	}
	d.Title, d.Invite = "Join "+peek.Team, &peek
	d.Lead = peek.By + " invites you to the team, as " + roleWord(peek.Role) + "."
	if acc == nil {
		d.SignInURL = s.signInURL("/invite")
		if d.SignInURL == "" {
			s.page(w, http.StatusNotFound, "Signing in on this page isn't set up on this server: paste the invite's link in the enowX app.", d)
			return
		}
		d.Note = "Signing in with GitHub makes your enowX account if you don't have one yet."
	} else {
		d.JoinInvite = true
	}
	s.page(w, http.StatusOK, "", d)
}

// inviteOpen keeps the token the page's script posted, for the page.
func (s *Server) inviteOpen(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		s.page(w, http.StatusForbidden, "This didn't come from this page.", nil)
		return
	}
	token := inviteToken(r.PostFormValue("token"))
	if _, err := s.db.PeekInvite(r.Context(), token); err != nil {
		s.page(w, http.StatusNotFound, inviteGone(err), &pageData{Title: "Join a team on enowX", Account: s.webAccount(r), Download: true})
		return
	}
	s.setInviteCookie(w, token, 3600)
	http.Redirect(w, r, "/invite", http.StatusSeeOther)
}

// inviteAccept joins the invite's team as the browser's account.
func (s *Server) inviteAccept(w http.ResponseWriter, r *http.Request) {
	acc := s.webAccount(r)
	if acc == nil {
		http.Redirect(w, r, "/invite", http.StatusSeeOther)
		return
	}
	if !s.sameOrigin(r) {
		s.page(w, http.StatusForbidden, "This didn't come from this page.", nil)
		return
	}
	c, err := r.Cookie(inviteCookie)
	if err != nil || c.Value == "" {
		s.page(w, http.StatusNotFound, "This browser holds no invite: open its link again.", &pageData{Title: "Join a team on enowX", Account: acc, Download: true})
		return
	}
	peek, _ := s.db.PeekInvite(r.Context(), c.Value)
	t, err := s.db.AcceptInvite(r.Context(), c.Value, acc.ID)
	s.setInviteCookie(w, "", -1)
	if err != nil {
		s.page(w, http.StatusNotFound, inviteGone(err), &pageData{Title: "Join a team on enowX", Account: acc, Download: true})
		return
	}
	s.page(w, http.StatusOK, "", &pageData{Title: "You're in " + t.Name, Done: "You joined " + t.Name + " as " + roleWord(peek.Role) + ".",
		Lead: "Open enowX and sign in with this GitHub account: the team's notes, chat and projects are there.", Account: acc, Download: true})
}

func inviteGone(err error) string {
	if errors.Is(err, store.ErrExpired) {
		return "This invite was used already, or expired: ask for a new one."
	}
	return err.Error()
}

// roleWord is a role as a sentence says it.
func roleWord(role string) string {
	switch role {
	case api.RoleAdmin:
		return "an admin"
	case api.RoleMember:
		return "a member"
	case api.RoleViewer:
		return "a viewer"
	}
	return role
}
