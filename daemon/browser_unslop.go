package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

// Unslop in the Browser: the page a session's Browser shows, the
// project's own running on localhost, is read for what makes it look
// AI-made: its HTML and every stylesheet it loaded (a bundle's, a CSS
// framework's), with the colors it really shows. Started, a branch of its
// own is made in the session's folder, so the redesign is undone by going
// back, and the session's agent is told to redesign the project's code
// pass by pass, a commit each; it reads the page again at each, as the dev
// server reloads it.

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
	var branch, base string
	if p.Start {
		if branch, base, err = s.unslopBranch(p.Session); err != nil {
			return out, err
		}
		out.Ask = browserAsk(page.URL, score, len(findings), theme, branch, base)
	}
	run, ok, err := s.browserUnslops.update(p.Session, browserPage, proto.CanvasUnslopParams{Start: p.Start, Pass: p.Pass}, score, findings, theme)
	if err != nil {
		return out, err
	}
	if ok && p.Start {
		run = s.browserUnslops.setBranch(p.Session, branch, base)
	}
	if ok {
		s.broadcast(proto.EventBrowserUnslop, run)
		out.Run = &run
	}
	return out, nil
}

// browserAsk is what the session's agent is told to redesign the page at
// u in the project's code, on branch (from base; "" outside git).
func browserAsk(u string, score, found int, theme, branch, base string) string {
	cmd := `"$ENOWX_BIN" browser unslop`
	keep := "Keep its look's direction: its theme, brand colors and mood stay; unslop changes its care, not its style."
	if theme != "" {
		keep = "It is a " + theme + " page and stays " + theme + ": never turn its theme. " + keep
	}
	where := "This folder isn't a git repository, so nothing is undone by a branch: change only what the passes call for."
	commit := ""
	if branch != "" {
		where = fmt.Sprintf("You are on branch %s, made for this from %s: the whole redesign goes there, so going back to %s undoes it.", branch, base, base)
		commit = ` After each pass, commit what it changed on this branch, the pass's name as the message ("Unslop: 03 Spacing"), so each can be undone alone.`
	}
	return fmt.Sprintf(`Unslop the page in your Browser (%s): it scores %d/100 with %d findings of what makes a design look AI-made. The user watches the page live. %s
1. Run: %s — it prints the findings by pass, in the order to fix them, each with what to avoid, and the elements.
2. Redesign it in this project's code (its components, styles, tokens, copy), one pass at a time, in that order, keeping its content, purpose and behavior; never invent data, and follow the project's own design system where it has one. %s
3. Begin each pass with: %s --pass N (N its number, 1 to 12): it reads the page again, as the dev server reloaded it, and the app shows the pass under way. Change the code a part at a time, so each change shows.%s
4. When what it still finds is what you'd keep on purpose: %s --pass done, then sum up what you changed, file by file, and how to undo it.`,
		u, score, found, where, cmd, keep, cmd, commit, cmd)
}

// unslopBranch makes a branch of its own for a redesign of session id's
// code and checks it out: its name, and the branch it started from. A
// folder outside git gets none; one with changes not committed is refused,
// so the branch holds the redesign alone.
func (s *Server) unslopBranch(id string) (branch, base string, err error) {
	sess, err := s.sessions.Get(id)
	if err != nil {
		return "", "", err
	}
	dir := sess.Info().Dir
	if _, err := git.TopLevel(dir); err != nil {
		return "", "", nil
	}
	if status, err := git.Run(dir, "status", "--porcelain"); err != nil {
		return "", "", fmt.Errorf("browser: read the project's changes: %w", err)
	} else if status != "" {
		return "", "", errors.New("browser: the project has changes not committed: commit or stash them first, so the unslop's branch holds its changes alone and is undone cleanly")
	}
	base = git.CurrentBranch(dir)
	if base == "" {
		base, _ = git.Run(dir, "rev-parse", "--short", "HEAD")
	}
	branch = "unslop/" + time.Now().Format("20060102-1504")
	for n := 2; git.BranchExists(dir, branch); n++ {
		branch = fmt.Sprintf("unslop/%s-%d", time.Now().Format("20060102-1504"), n)
	}
	if _, err := git.Run(dir, "switch", "-c", branch); err != nil {
		return "", "", fmt.Errorf("browser: make the unslop's branch: %w", err)
	}
	return branch, base, nil
}
