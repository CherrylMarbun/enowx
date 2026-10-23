package proto

// Review: what a session's branch changed against its base, and what to do
// with it (commit, push, open a pull request, merge, discard the worktree).
const (
	MethodReviewStatus  = "review.status"  // ReviewRef → ReviewStatus
	MethodReviewDiff    = "review.diff"    // ReviewRef → Diff
	MethodReviewCommit  = "review.commit"  // CommitParams → ReviewStatus
	MethodReviewPush    = "review.push"    // ReviewRef → ReviewStatus
	MethodReviewPR      = "review.pr"      // PRParams → PullRequest
	MethodReviewMerge   = "review.merge"   // ReviewRef → ReviewStatus
	MethodReviewDiscard = "review.discard" // DiscardParams
)

// ReviewRef names a session's folder and the branch to compare it with:
// Base, or when empty the session's default base (ReviewStatus.Base);
// "HEAD" compares with the last commit.
type ReviewRef struct {
	Session string `json:"session"`
	Base    string `json:"base,omitempty"`
}

// ReviewStatus is where a session's branch stands.
type ReviewStatus struct {
	Dir    string `json:"dir"`
	Branch string `json:"branch"` // "" when detached
	// Base is the branch compared with: by default the project folder's
	// branch for a worktree, and "" (the last commit, so only what isn't
	// committed) for a session in the project folder.
	Base     string   `json:"base,omitempty"`
	Bases    []string `json:"bases,omitempty"` // other local branches
	Worktree bool     `json:"worktree,omitempty"`
	// Ahead and Behind count the commits of Branch not in Base, and of Base
	// not in Branch.
	Ahead  int `json:"ahead"`
	Behind int `json:"behind"`
	// Uncommitted counts files with changes not committed, untracked ones
	// included.
	Uncommitted int    `json:"uncommitted"`
	Upstream    string `json:"upstream,omitempty"` // the branch it pushes to
	Unpushed    int    `json:"unpushed"`           // commits not on Upstream
	Remote      string `json:"remote,omitempty"`   // the remote pushes go to
	HasGH       bool   `json:"hasGH,omitempty"`    // the gh CLI is installed
	// CanMerge: Branch can be merged into Base in the project folder,
	// which has Base checked out. ProjectDirty says why it can't yet.
	CanMerge     bool `json:"canMerge,omitempty"`
	ProjectDirty bool `json:"projectDirty,omitempty"`
}

// Diff is what changed against a base, uncommitted changes included.
type Diff struct {
	Base      string     `json:"base"` // the commit compared with
	Files     []DiffFile `json:"files"`
	Truncated bool       `json:"truncated,omitempty"` // more files than shown
}

// File statuses of DiffFile.
const (
	FileAdded    = "added"
	FileModified = "modified"
	FileDeleted  = "deleted"
	FileRenamed  = "renamed"
)

// DiffFile is one changed file.
type DiffFile struct {
	Path      string     `json:"path"`
	OldPath   string     `json:"oldPath,omitempty"` // renamed from
	Status    string     `json:"status"`
	Added     int        `json:"added"`
	Removed   int        `json:"removed"`
	Binary    bool       `json:"binary,omitempty"`
	Hunks     []DiffHunk `json:"hunks,omitempty"`
	Truncated bool       `json:"truncated,omitempty"` // too long: hunks cut
}

// DiffHunk is a run of changed lines with their context.
type DiffHunk struct {
	Header string     `json:"header"` // "@@ -1,3 +1,4 @@ func f()"
	Lines  []DiffLine `json:"lines"`
}

// DiffLine is a line of a hunk. Kind is ' ' (context), '+' or '-'; Old and
// New are its line numbers, 0 where it has none.
type DiffLine struct {
	Kind byte   `json:"k"`
	Text string `json:"t"`
	Old  int    `json:"o,omitempty"`
	New  int    `json:"n,omitempty"`
}

// CommitParams commits every change in the session's folder.
type CommitParams struct {
	Session string `json:"session"`
	Message string `json:"message"`
}

// PRParams pushes the session's branch and opens a pull request into Base
// with gh. An empty Title fills title and body from the commits.
type PRParams struct {
	Session string `json:"session"`
	Base    string `json:"base,omitempty"`
	Title   string `json:"title,omitempty"`
	Body    string `json:"body,omitempty"`
}

// PullRequest is an opened pull request.
type PullRequest struct {
	URL string `json:"url"`
}

// DiscardParams ends the sessions in a session's worktree and removes the
// worktree, and its branch with DeleteBranch.
type DiscardParams struct {
	Session      string `json:"session"`
	DeleteBranch bool   `json:"deleteBranch,omitempty"`
}
