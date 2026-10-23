package client

import "github.com/enowx/enowx/proto"

// ReviewStatus tells where session id's branch stands against base ("" for
// its default base).
func (c *Client) ReviewStatus(id, base string) (proto.ReviewStatus, error) {
	var st proto.ReviewStatus
	err := c.Call(proto.MethodReviewStatus, proto.ReviewRef{Session: id, Base: base}, &st)
	return st, err
}

// Diff returns what session id changed against base.
func (c *Client) Diff(id, base string) (proto.Diff, error) {
	var d proto.Diff
	err := c.Call(proto.MethodReviewDiff, proto.ReviewRef{Session: id, Base: base}, &d)
	return d, err
}

// Commit commits every change in session id's folder.
func (c *Client) Commit(id, message string) (proto.ReviewStatus, error) {
	var st proto.ReviewStatus
	err := c.Call(proto.MethodReviewCommit, proto.CommitParams{Session: id, Message: message}, &st)
	return st, err
}

// Push pushes session id's branch.
func (c *Client) Push(id, base string) (proto.ReviewStatus, error) {
	var st proto.ReviewStatus
	err := c.Call(proto.MethodReviewPush, proto.ReviewRef{Session: id, Base: base}, &st)
	return st, err
}

// PullRequest pushes session id's branch and opens a pull request.
func (c *Client) PullRequest(p proto.PRParams) (proto.PullRequest, error) {
	var pr proto.PullRequest
	err := c.Call(proto.MethodReviewPR, p, &pr)
	return pr, err
}

// Merge merges session id's branch into base in the project folder.
func (c *Client) Merge(id, base string) (proto.ReviewStatus, error) {
	var st proto.ReviewStatus
	err := c.Call(proto.MethodReviewMerge, proto.ReviewRef{Session: id, Base: base}, &st)
	return st, err
}

// Discard ends the sessions in session id's worktree and removes it.
func (c *Client) Discard(id string, deleteBranch bool) error {
	return c.Call(proto.MethodReviewDiscard, proto.DiscardParams{Session: id, DeleteBranch: deleteBranch}, nil)
}

// Conflicts finds the files two running sessions of project both change.
func (c *Client) Conflicts(project string) ([]proto.Conflict, error) {
	var out []proto.Conflict
	err := c.Call(proto.MethodProjectConflicts, proto.ProjectRef{Path: project}, &out)
	return out, err
}

// Issues lists project's open GitHub issues matching search ("" for the
// newest).
func (c *Client) Issues(project, search string) ([]proto.Issue, error) {
	var out []proto.Issue
	err := c.Call(proto.MethodProjectIssues, proto.IssuesParams{Project: project, Search: search}, &out)
	return out, err
}
