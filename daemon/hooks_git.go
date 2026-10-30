package daemon

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/core/rag"
	"github.com/enowx/enowx/proto"
)

// Hooks on Git: before a commit, push or merge, from the agent (its
// command) or the app's Git panel, which a hook can refuse; and after,
// as the daemon sees the session's folder change: a new commit, a merge,
// a push, files in conflict.

// gitLook is how often folders are looked at, while a hook waits on Git.
const gitLook = 5 * time.Second

// gitCommand finds a commit, push or merge in a command line.
var gitCommand = regexp.MustCompile(`\bgit\b[^|;&\n]*?\s(commit|push|merge)\b`)

// gitOp is the Git operation command runs, "" for none.
func gitOp(command string) string {
	if m := gitCommand.FindStringSubmatch(command); m != nil {
		return m[1]
	}
	return ""
}

// gitBefore runs the hooks before Git operation op in session id's folder,
// done from the app; a refusal comes back as the error.
func (s *Server) gitBefore(id, op string) error {
	sess, err := s.sessions.Get(id)
	if err != nil {
		return nil // nothing to run for; the operation says what's wrong
	}
	d := s.fireHooks(sess.Info(), proto.HookEvent{Event: proto.HookGitBefore, Op: op, Via: proto.HookInApp})
	if d.Block {
		return errors.New(d.Reason)
	}
	return nil
}

// gitState is what a folder's repository was at.
type gitState struct {
	head, upstream string
	conflicted     bool
}

// gitEvents are the events Git watches for.
var gitEvents = []string{proto.HookCommit, proto.HookPush, proto.HookMerge, proto.HookConflict}

// watchGit looks at the folders of live sessions while a hook waits on
// Git there, until done closes.
func (s *Server) watchGit(done <-chan struct{}) {
	last := map[string]gitState{} // folder → what it was at
	t := time.NewTicker(gitLook)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		seen := map[string]bool{}
		for _, sess := range s.sessions.List() {
			info := sess.Info()
			if info.Exited || !userSession(info) || seen[info.Dir] {
				continue
			}
			seen[info.Dir] = true
			wanted := s.ragSettings.Project(rag.ProjectID(sessionProject(info))).On // a commit reindexes
			for _, e := range gitEvents {
				wanted = wanted || s.wants(info, e)
			}
			if !wanted {
				delete(last, info.Dir)
				continue
			}
			now, ok := gitStateOf(info.Dir)
			if !ok {
				continue
			}
			was, known := last[info.Dir]
			last[info.Dir] = now
			if known {
				go s.gitChanged(info, was, now)
			}
		}
		for dir := range last {
			if !seen[dir] {
				delete(last, dir)
			}
		}
	}
}

// gitStateOf is where the repository in dir is at.
func gitStateOf(dir string) (gitState, bool) {
	head, err := git.Run(dir, "rev-parse", "HEAD")
	if err != nil {
		return gitState{}, false
	}
	st := gitState{head: strings.TrimSpace(head)}
	if up, err := git.Run(dir, "rev-parse", "@{upstream}"); err == nil {
		st.upstream = strings.TrimSpace(up)
	}
	if out, err := git.Run(dir, "diff", "--name-only", "--diff-filter=U"); err == nil {
		st.conflicted = strings.TrimSpace(out) != ""
	}
	return st, true
}

// gitChanged runs the hooks on what changed in session info's folder from
// was to now.
func (s *Server) gitChanged(info proto.SessionInfo, was, now gitState) {
	dir := info.Dir
	if now.head != was.head {
		s.ragSoon(sessionProject(info))
		subject, _ := git.Run(dir, "log", "-1", "--format=%s", now.head)
		parents, _ := git.Run(dir, "rev-list", "--parents", "-n", "1", now.head)
		ev := proto.HookEvent{Commit: now.head, Message: strings.TrimSpace(subject), Via: proto.HookOutside}
		ev.Event = proto.HookCommit
		s.fireHooks(info, ev)
		if len(strings.Fields(parents)) > 2 {
			ev.Event = proto.HookMerge
			s.fireHooks(info, ev)
		}
	}
	// The upstream moved to where the branch is: pushed. (A fetch moves it
	// elsewhere.)
	if now.upstream != was.upstream && now.upstream != "" && now.upstream == now.head {
		s.fireHooks(info, proto.HookEvent{Event: proto.HookPush, Commit: now.head, Via: proto.HookOutside})
	}
	if now.conflicted && !was.conflicted {
		out, _ := git.Run(dir, "diff", "--name-only", "--diff-filter=U")
		s.fireHooks(info, proto.HookEvent{Event: proto.HookConflict, Paths: strings.Fields(out), Via: proto.HookOutside})
	}
}
