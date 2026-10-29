package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/proto"
)

// A project's rules: Markdown every session in it reads first, in its
// guide, whatever its agent. The user's own project keeps them in the
// synced settings (settings/rules/), by the project's Git remote, so a
// clone on another of their hosts has them too; a team's project in the
// team's space (rules/<project>.md), the same for every member, edited by
// its members or only its admins.

// maxRules is the most a project's rules hold: they go in every session's
// guide.
const maxRules = 32 << 10

func (s *Server) ownRulesFile(project string) string {
	key := s.remoteRef(project) // the same however the remote is written
	if key == "" {
		key = "path:" + project
	}
	h := sha256.Sum256([]byte(key))
	return filepath.Join(s.dataDir, "settings", "rules", hex.EncodeToString(h[:8])+".md")
}

func (s *Server) teamRulesFile(team, project string) string {
	return filepath.Join(s.teamsDir(), team, "rules", project+".md")
}

// projectRules are the rules of the project at folder path.
func (s *Server) projectRules(path string) (proto.ProjectRules, error) {
	if path == "" {
		return proto.ProjectRules{}, errors.New("rules: name a project's folder")
	}
	if sp, tp, ok := s.teamOfFolder(path); ok {
		data, _ := os.ReadFile(s.teamRulesFile(sp.Team, tp.ID))
		editors := tp.RulesBy
		if editors == "" {
			editors = proto.RulesByMembers
		}
		admin := sp.Role == api.RoleOwner || sp.Role == api.RoleAdmin
		return proto.ProjectRules{Path: path, Rules: string(data), Team: sp.Team, Project: tp.Name, Editors: editors,
			CanEdit: admin || sp.Role == api.RoleMember && editors == proto.RulesByMembers}, nil
	}
	data, _ := os.ReadFile(s.ownRulesFile(path))
	return proto.ProjectRules{Path: path, Rules: string(data), CanEdit: true}, nil
}

// saveProjectRules writes a project's rules, as the user may.
func (s *Server) saveProjectRules(p proto.ProjectRules) (proto.ProjectRules, error) {
	cur, err := s.projectRules(p.Path)
	if err != nil {
		return proto.ProjectRules{}, err
	}
	rules := strings.TrimSpace(p.Rules)
	if len(rules) > maxRules {
		return proto.ProjectRules{}, fmt.Errorf("rules: %d KB at most, they go in every session's guide", maxRules>>10)
	}
	if err := secretsIn(map[string][]byte{"the rules": []byte(rules)}); err != nil {
		return proto.ProjectRules{}, errors.New("rules: they look like they hold a secret: agents read them, keep credentials in the vault")
	}
	if cur.Team == "" {
		file := s.ownRulesFile(p.Path)
		if rules == "" {
			os.Remove(file)
		} else if err := writeTeamFile(file, []byte(rules+"\n")); err != nil {
			return proto.ProjectRules{}, err
		}
		return s.projectRules(p.Path)
	}
	sp, tp, _ := s.teamOfFolder(p.Path)
	admin := sp.Role == api.RoleOwner || sp.Role == api.RoleAdmin
	if p.Editors != "" && p.Editors != cur.Editors {
		if !admin {
			return proto.ProjectRules{}, fmt.Errorf("rules: only %s's admins say who edits a project's rules", sp.Name)
		}
		if p.Editors != proto.RulesByMembers && p.Editors != proto.RulesByAdmins {
			return proto.ProjectRules{}, fmt.Errorf("rules: %q edit, or %q", proto.RulesByMembers, proto.RulesByAdmins)
		}
		list := s.readTeamProjects(sp.Team)
		if i := slices.IndexFunc(list, func(x storedTeamProject) bool { return x.ID == tp.ID }); i >= 0 {
			list[i].RulesBy = p.Editors
			if err := s.writeTeamProjects(sp.Team, list); err != nil {
				return proto.ProjectRules{}, err
			}
		}
		cur.CanEdit = admin
	}
	if rules != strings.TrimSpace(cur.Rules) {
		if !cur.CanEdit {
			return proto.ProjectRules{}, fmt.Errorf("rules: %s's rules are edited by its admins", tp.Name)
		}
		file := s.teamRulesFile(sp.Team, tp.ID)
		if rules == "" {
			os.Remove(file)
		} else if err := writeTeamFile(file, []byte(rules+"\n")); err != nil {
			return proto.ProjectRules{}, err
		}
		go s.recordAudit(sp.Team, api.AuditParams{Action: api.AuditRulesChanged, Detail: tp.Name})
	}
	s.teamSync.kick()
	return s.projectRules(p.Path)
}

// recordAudit records an action in a team's audit log, best effort.
func (s *Server) recordAudit(team string, p api.AuditParams) {
	st, err := s.cloud.store.Get()
	if err != nil || !st.SignedIn() {
		return
	}
	c, err := cloudclient.New(st.URL, st.Token)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudTimeout)
	defer cancel()
	if err := c.RecordAudit(ctx, team, p); err != nil {
		s.logger().Warn("cloud: audit", "team", team, "err", err)
	}
}

// rulesGuide is a project's rules as a session's guide gives them.
func (s *Server) rulesGuide(project string) string {
	r, err := s.projectRules(project)
	if err != nil || strings.TrimSpace(r.Rules) == "" {
		return ""
	}
	whose := "the user's"
	if r.Team != "" {
		whose = "the team's"
	}
	return "\n\n## This project's rules\n\nThese are " + whose + " rules for this project: follow them in all you do here.\n\n" + strings.TrimSpace(r.Rules) + "\n"
}
