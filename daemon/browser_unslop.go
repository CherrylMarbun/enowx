package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

// Unslop in the Browser: the page a session's Browser shows, the
// project's own running on localhost, is read for what makes it look
// AI-made: its HTML and every stylesheet it loaded (a bundle's, a CSS
// framework's), with the colors it really shows. Started, the session's
// agent is told to redesign the project's code pass by pass; it reads the
// page again at each, as the dev server reloads it.

// browserPage is what a run calls the page it reads, whatever its URL.
const browserPage = "page"

// pageScript gives the page shown as HTML whose styles are all of its
// stylesheets' rules, and its URL. The background it shows and its text
// color go in as the body's, so its theme reads true; the app's own
// marks are left out.
const pageScript = `(function(){var css=[];
for(var i=0;i<document.styleSheets.length;i++){try{var rs=document.styleSheets[i].cssRules;for(var j=0;j<rs.length;j++)css.push(rs[j].cssText)}catch(e){}}
var bg='',e=document.elementFromPoint(innerWidth/2,Math.min(innerHeight/2,200));
for(;e;e=e.parentElement){var c=getComputedStyle(e).backgroundColor;if(c&&c!=='transparent'&&!/rgba\([^)]*,\s*0\)$/.test(c)){bg=c;break}}
if(!bg)bg=getComputedStyle(document.body).backgroundColor;
css.push('body{background-color:'+bg+';color:'+getComputedStyle(document.body).color+'}');
var b=document.body.cloneNode(true);b.querySelectorAll('script,#__enx_scan,#__enx_regions').forEach(function(x){x.remove()});
return JSON.stringify({url:location.href,html:'<style>'+css.join('\n')+'</style>'+b.outerHTML})})()`

// localPage says page URL u is on this device: localhost, a loopback
// address, or a .localhost name.
func localPage(u string) bool {
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") {
		return false
	}
	h := strings.ToLower(p.Hostname())
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// browserUnslop reads the page of session p.Session's Browser.
func (s *Server) browserUnslop(p proto.BrowserUnslopParams) (proto.CanvasUnslop, error) {
	if p.Status {
		run, ok := s.browserUnslops.status(p.Session)
		if !ok {
			return proto.CanvasUnslop{}, errors.New("browser: no unslop under way in this session's Browser")
		}
		return proto.CanvasUnslop{Score: run.Score, Findings: run.Findings, Run: &run}, nil
	}
	res, err := s.browserAct(proto.BrowserAction{Session: p.Session, Action: proto.BrowserEval, Text: pageScript})
	if err != nil {
		return proto.CanvasUnslop{}, err
	}
	var page struct{ URL, HTML string }
	if err := json.Unmarshal([]byte(res.Text), &page); err != nil {
		return proto.CanvasUnslop{}, fmt.Errorf("browser: the page didn't give its HTML: %w", err)
	}
	if !localPage(page.URL) {
		return proto.CanvasUnslop{}, fmt.Errorf("browser: unslop reads a page of the project running here (localhost), not %s", page.URL)
	}
	score, findings := unslop.Scan(page.HTML, proto.CanvasMockup)
	theme := unslop.Theme(page.HTML)
	if !p.Start {
		theme = s.browserUnslops.theme(p.Session, browserPage)
	}
	if fd, turned := unslop.ThemeFinding(theme, page.HTML); turned {
		findings = append([]proto.UnslopFinding{fd}, findings...)
		score = max(score-15, 0)
	}
	out := proto.CanvasUnslop{Frame: page.URL, Score: score, Findings: findings}
	if p.Start {
		out.Ask = browserAsk(page.URL, score, len(findings), theme)
	}
	run, ok, err := s.browserUnslops.update(p.Session, browserPage, proto.CanvasUnslopParams{Start: p.Start, Pass: p.Pass}, score, findings, theme)
	if err != nil {
		return out, err
	}
	if ok {
		s.broadcast(proto.EventBrowserUnslop, run)
		out.Run = &run
	}
	return out, nil
}

// browserAsk is what the session's agent is told to redesign the page at
// u in the project's code.
func browserAsk(u string, score, found int, theme string) string {
	cmd := `"$ENOWX_BIN" browser unslop`
	keep := "Keep its look's direction: its theme, brand colors and mood stay; unslop changes its care, not its style."
	if theme != "" {
		keep = "It is a " + theme + " page and stays " + theme + ": never turn its theme. " + keep
	}
	return fmt.Sprintf(`Unslop the page in your Browser (%s): it scores %d/100 with %d findings of what makes a design look AI-made. The user watches the page live.
1. Run: %s — it prints the findings by pass, in the order to fix them, each with what to avoid, and the elements.
2. Redesign it in this project's code (its components, styles, tokens, copy), one pass at a time, in that order, keeping its content, purpose and behavior; never invent data, and follow the project's own design system where it has one. %s
3. Begin each pass with: %s --pass N (N its number, 1 to 12): it reads the page again, as the dev server reloaded it, and the app shows the pass under way. Change the code a part at a time, so each change shows.
4. When what it still finds is what you'd keep on purpose: %s --pass done, then sum up what you changed, file by file.`,
		u, score, found, cmd, keep, cmd, cmd)
}
