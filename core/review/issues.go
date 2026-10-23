package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Issues lists dir's repository's open GitHub issues with gh, matching
// search when given (GitHub's search syntax), newest first.
func Issues(dir, search string) ([]proto.Issue, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errors.New("review: listing issues needs the GitHub CLI (gh) on this host")
	}
	args := []string{"issue", "list", "--state", "open", "--limit", "50", "--json", "number,title,body,url,labels,author"}
	if s := strings.TrimSpace(search); s != "" {
		args = append(args, "--search", s)
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	out, err := run(ctx, dir, "gh", args...)
	if err != nil {
		return nil, fmt.Errorf("review: list issues: %w", err)
	}
	var raw []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		URL    string `json:"url"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("review: list issues: %w", err)
	}
	issues := make([]proto.Issue, 0, len(raw))
	for _, r := range raw {
		is := proto.Issue{Number: r.Number, Title: r.Title, Body: clipBody(r.Body), URL: r.URL, Author: r.Author.Login}
		for _, l := range r.Labels {
			is.Labels = append(is.Labels, l.Name)
		}
		issues = append(issues, is)
	}
	return issues, nil
}

// clipBody keeps an issue's body within what a prompt carries well.
func clipBody(s string) string {
	const max = 8000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…"
}
