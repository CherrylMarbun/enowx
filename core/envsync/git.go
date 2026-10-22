package envsync

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"

	"github.com/enowx/enowx/proto"
)

// The git identity is synced so commits made by agents on a server carry
// the user's name.

type gitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func readGitIdentity() (gitIdentity, bool) {
	get := func(key string) string {
		out, _ := exec.Command("git", "config", "--global", "--get", key).Output()
		return strings.TrimSpace(string(out))
	}
	id := gitIdentity{Name: get("user.name"), Email: get("user.email")}
	return id, id.Name != "" || id.Email != ""
}

func scanGit(s source) []proto.EnvItem {
	id, ok := readGitIdentity()
	if !ok {
		return nil
	}
	detail := id.Name
	if id.Email != "" {
		detail += " <" + id.Email + ">"
	}
	it := newItem(s, "identity", strings.TrimSpace(detail))
	raw, _ := json.Marshal(id)
	it.Hash = hashParts(raw)
	return []proto.EnvItem{it}
}

func exportGit() ([]byte, error) {
	id, ok := readGitIdentity()
	if !ok {
		return nil, errors.New("envsync: no git identity")
	}
	return json.Marshal(id)
}

func importGit(p proto.EnvPayload) (proto.EnvImportResult, error) {
	var res proto.EnvImportResult
	var id gitIdentity
	if err := json.Unmarshal(p.Value, &id); err != nil {
		return res, err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return res, errors.New("envsync: git is not installed")
	}
	for key, v := range map[string]string{"user.name": id.Name, "user.email": id.Email} {
		if v == "" {
			continue
		}
		if out, err := exec.Command("git", "config", "--global", key, v).CombinedOutput(); err != nil {
			return res, errors.New("envsync: git config " + key + ": " + strings.TrimSpace(string(out)))
		}
	}
	return res, nil
}

// programWarnings says when the program an MCP server runs is missing.
func programWarnings(server, program string) []string {
	if program == "" {
		return nil
	}
	if _, err := exec.LookPath(program); err != nil {
		return []string{"MCP server " + server + " runs " + program + ", which isn't installed here"}
	}
	return nil
}
