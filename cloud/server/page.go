package server

import (
	_ "embed"
	"html/template"
	"net/http"

	"github.com/enowx/enowx/cloud/api"
)

// The cloud's pages look like enowx.ai: pure black (white in light mode),
// one card in the middle, the app's logo. No font from elsewhere, and no
// script but the invite page's one, allowed by its hash.

//go:embed assets/logo.svg
var logoSVG []byte

func (s *Server) logo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(logoSVG)
}

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .Title}}{{.Title}} · {{end}}enowX Cloud</title>
<link rel="icon" href="/logo.svg" type="image/svg+xml">
<style>
:root{color-scheme:dark;--bg:#000;--surface:#0a0a0a;--sunken:#050505;--border:#1f1f1f;--border-strong:#2a2a2a;--text:#ededed;--soft:#b4b4b4;--muted:#8f8f8f;--accent:#ededed;--accent-hover:#fff;--on-accent:#000;--hover:#1a1a1a;--ok:#6bd69a;--ok-soft:#0b2015;--bad:#ff8b80;--bad-soft:#2a0e0b;--warn:#f0b955;
--sans:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;--mono:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}
@media (prefers-color-scheme:light){:root{color-scheme:light;--bg:#fff;--surface:#f6f6f6;--sunken:#ececec;--border:#e6e6e6;--border-strong:#d6d6d6;--text:#0a0a0a;--soft:#3f3f3f;--muted:#666;--accent:#0a0a0a;--accent-hover:#2a2a2a;--on-accent:#fff;--hover:#e9e9e9;--ok:#127040;--ok-soft:#e3f4ea;--bad:#b42318;--bad-soft:#fbe7e5;--warn:#8a5300}}
*{box-sizing:border-box}
html,body{margin:0;min-height:100%;background:var(--bg);color:var(--text);font:400 15px/1.55 var(--sans);-webkit-font-smoothing:antialiased}
main{min-height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;padding:48px 16px}
.brand{display:flex;align-items:center;gap:10px;margin-bottom:24px;color:var(--text);text-decoration:none;font-weight:600;letter-spacing:-.01em}
.brand img{width:28px;height:28px;border-radius:7px}
.brand span{color:var(--muted);font-weight:400}
.card{width:100%;max-width:400px;background:var(--surface);border:1px solid var(--border);border-radius:16px;padding:28px}
h1{margin:0 0 6px;font-size:20px;line-height:1.3;font-weight:600;letter-spacing:-.02em}
.lead{margin:0 0 22px;color:var(--soft);font-size:14px}
label{display:block;font-size:12px;font-weight:500;color:var(--muted);margin:0 0 8px;letter-spacing:.02em}
input[type=text]{width:100%;height:52px;padding:0 14px;border:1px solid var(--border-strong);border-radius:12px;background:var(--sunken);color:var(--text);font:600 22px/1 var(--mono);letter-spacing:.18em;text-align:center;text-transform:uppercase;outline:none}
input[type=text]::placeholder{color:var(--muted);opacity:.55}
input[type=text]:focus{border-color:var(--accent)}
.btn{display:flex;align-items:center;justify-content:center;gap:9px;width:100%;height:44px;margin-top:14px;border:0;border-radius:12px;background:var(--accent);color:var(--on-accent);font:600 14px/1 var(--sans);text-decoration:none;cursor:pointer}
.btn:hover{background:var(--accent-hover)}
.btn.ghost{background:transparent;color:var(--soft);border:1px solid var(--border-strong)}
.btn.ghost:hover{background:var(--hover);color:var(--text)}
.btn svg{width:18px;height:18px;fill:currentColor}
.btn:focus-visible,input:focus-visible,.link:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
.notice{display:flex;gap:10px;align-items:flex-start;padding:12px 14px;border-radius:12px;font-size:14px;margin:0 0 18px}
.notice.bad{background:var(--bad-soft);color:var(--bad)}
.notice.ok{background:var(--ok-soft);color:var(--ok)}
.notice svg{flex:none;width:18px;height:18px;margin-top:2px;stroke:currentColor;fill:none;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}
.device{display:flex;gap:14px;align-items:center;padding:14px;border:1px solid var(--border-strong);border-radius:12px;background:var(--sunken);margin:0 0 6px}
.device .icon{flex:none;display:grid;place-items:center;width:40px;height:40px;border-radius:10px;background:var(--hover)}
.device svg{width:20px;height:20px;stroke:var(--text);fill:none;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}
.device b{display:block;font-weight:600;word-break:break-word}
.device small{color:var(--muted);font-size:13px}
.code{font:600 13px/1 var(--mono);letter-spacing:.1em;color:var(--soft)}
.warn{margin:12px 0 0;font-size:13px;color:var(--muted)}
.get{margin:18px 0 0;padding:14px 0 0;border-top:1px solid var(--border);font-size:13px;color:var(--muted)}
.get a{color:var(--text);font-weight:600}
.account{display:flex;align-items:center;gap:10px;width:100%;max-width:400px;margin-top:16px;padding:0 4px;font-size:13px;color:var(--muted)}
.account img{width:24px;height:24px;border-radius:50%;background:var(--hover)}
.account .who{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.account .who b{color:var(--soft);font-weight:500}
.link{background:none;border:0;padding:4px 2px;color:var(--muted);font:inherit;cursor:pointer;text-decoration:underline;text-underline-offset:3px}
.link:hover{color:var(--text)}
form{margin:0}
@media (max-width:440px){.card{padding:22px}}
</style></head><body><main>
<a class="brand" href="/"><img src="/logo.svg" alt="">enowX <span>Cloud</span></a>
<div class="card">
{{if .Error}}<div class="notice bad" role="alert"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/></svg><div>{{.Error}}</div></div>{{end}}
{{if .Done}}<div class="notice ok" role="status"><svg viewBox="0 0 24 24"><path d="M5 12.5l4.5 4.5L19 7.5"/></svg><div>{{.Done}}</div></div>{{end}}
{{if .Title}}<h1>{{.Title}}</h1>{{end}}
{{if .Lead}}<p class="lead">{{.Lead}}</p>{{end}}
{{if .Device}}
<div class="device"><div class="icon">{{if eq .Device.Kind "host"}}<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01"/></svg>{{else}}<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="12" rx="2"/><path d="M2 20h20"/></svg>{{end}}</div>
<div><b>{{.Device.Name}}</b><small>{{if eq .Device.Kind "host"}}A host's daemon{{else}}The enowX app{{end}} · <span class="code">{{.Code}}</span></small></div></div>
<p class="warn">Approve only if you started this on that device just now. It will act as you.</p>
<form method="post" action="/device"><input type="hidden" name="code" value="{{.Code}}"><button class="btn">Approve</button></form>
<a class="btn ghost" href="/device">Not me</a>
{{else if .Invite}}
<div class="device"><div class="icon"><svg viewBox="0 0 24 24"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg></div>
<div><b>{{.Invite.Team}}</b><small>Invited by {{.Invite.By}} · as {{.Invite.Role}}</small></div></div>
{{if .JoinInvite}}<form method="post" action="/invite/accept"><button class="btn">Join {{.Invite.Team}}</button></form>{{end}}
{{else if .AskCode}}
<form method="get" action="/device"><label for="code">Code</label><input type="text" id="code" name="code" value="{{.Code}}" autocomplete="off" autocapitalize="characters" spellcheck="false" autofocus placeholder="XXXX-XXXX" maxlength="9"><button class="btn">Continue</button></form>
{{end}}
{{if .Script}}<form id="open" method="post" action="/invite"><input type="hidden" name="token"></form><noscript><p class="warn">This page needs JavaScript to read the invite. Without it, paste the invite's link in the enowX app: Settings › Cloud › Join a team.</p></noscript><script>{{.Script}}</script>{{end}}
{{if .SignInURL}}<a class="btn" href="{{.SignInURL}}"><svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>Sign in with GitHub</a>{{end}}
{{if .Note}}<p class="warn">{{.Note}}</p>{{end}}
{{if .Download}}<p class="get">Don't have enowX yet? <a href="https://enowx.ai">Download it at enowx.ai</a>, then sign in with the same GitHub account: your teams are there.</p>{{end}}
</div>
{{if .Account}}<div class="account">{{if .Account.Avatar}}<img src="{{.Account.Avatar}}" alt="">{{end}}<div class="who">Signed in as <b>{{.Account.Name}}</b>{{if .Account.GitHub}} · @{{.Account.GitHub}}{{end}}</div><form method="post" action="/signout"><button class="link">Sign out</button></form></div>{{end}}
</main></body></html>`))

// pageData is what a page shows: a title and lead, an error or a success,
// the code to ask for or the device waiting behind it, who is signed in.
type pageData struct {
	Title, Lead string
	Error, Done string
	Account     *api.Account
	Device      *api.PendingDevice
	Code        string
	AskCode     bool
	SignInURL   string
	Note        string
	// An invite: what it's for, and whether the account may join it now.
	Invite     *api.InvitePeek
	JoinInvite bool
	Download   bool        // says where to get the app
	Script     template.JS // inviteScript, the one script a page runs
}

// page answers with page d (nil: an empty one), msg as its error when
// status is one, else as its lead.
func (s *Server) page(w http.ResponseWriter, status int, msg string, d *pageData) {
	if d == nil {
		d = &pageData{}
	}
	switch {
	case msg == "":
	case status >= 400:
		d.Error = msg
	case d.Lead == "":
		d.Lead = msg
	}
	if d.Title == "" && d.Error != "" && !d.AskCode && d.SignInURL == "" {
		d.Title = "Something went wrong"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY") // approving can't be clicked in someone's frame
	csp := "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' https://avatars.githubusercontent.com; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"
	if d.Script != "" {
		csp += "; script-src " + inviteScriptHash
	}
	w.Header().Set("Content-Security-Policy", csp)
	// same-origin: the pages' own forms post with their origin (sameOrigin),
	// other sites learn nothing.
	w.Header().Set("Referrer-Policy", "same-origin")
	w.WriteHeader(status)
	pageTmpl.Execute(w, d)
}
