package review

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

// Limits keep a diff something the app can show: a file longer than
// maxFileLines shows its first lines, and past maxLines or maxFiles the
// rest is left out.
const (
	maxFiles     = 500
	maxLines     = 30000
	maxFileLines = 4000
	// An untracked file larger than this shows as changed, not line by line.
	maxUntracked = 512 << 10
)

// emptyTree is git's tree with nothing in it: what a repository without
// commits is compared with.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Diff returns what dir changed against base: committed, staged and not,
// and untracked files. An empty base compares with the last commit.
func Diff(dir, base string) (proto.Diff, error) {
	from, err := mergeBase(dir, base)
	if err != nil {
		return proto.Diff{}, err
	}
	out, err := git.Run(dir, "-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff", "-M", "-U3", from, "--")
	if err != nil {
		return proto.Diff{}, err
	}
	d := proto.Diff{Base: from}
	d.Files = parseDiff(out)
	untracked, err := git.Run(dir, "-c", "core.quotePath=false", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return proto.Diff{}, err
	}
	if untracked != "" {
		for _, path := range strings.Split(untracked, "\n") {
			d.Files = append(d.Files, untrackedFile(dir, path))
		}
	}
	limit(&d)
	return d, nil
}

// mergeBase is the commit base and HEAD last had in common, so that what
// base did since doesn't show as undone; HEAD for an empty base.
func mergeBase(dir, base string) (string, error) {
	if _, err := git.Run(dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return emptyTree, nil // no commit yet
	}
	if base == "" {
		return "HEAD", nil
	}
	return git.Run(dir, "merge-base", "HEAD", base)
}

// parseDiff reads git's unified diff.
func parseDiff(out string) []proto.DiffFile {
	var files []proto.DiffFile
	var f *proto.DiffFile
	var h *proto.DiffHunk
	oldNo, newNo := 0, 0
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			files = append(files, proto.DiffFile{Status: proto.FileModified})
			f, h = &files[len(files)-1], nil
			// Binary files have no ---/+++ lines to name them.
			if i := strings.Index(line, " b/"); i >= 0 {
				f.Path = line[i+3:]
			}
		case f == nil:
		case h == nil && strings.HasPrefix(line, "new file mode"):
			f.Status = proto.FileAdded
		case h == nil && strings.HasPrefix(line, "deleted file mode"):
			f.Status = proto.FileDeleted
		case h == nil && strings.HasPrefix(line, "rename from "):
			f.Status, f.OldPath = proto.FileRenamed, strings.TrimPrefix(line, "rename from ")
		case h == nil && strings.HasPrefix(line, "rename to "):
			f.Path = strings.TrimPrefix(line, "rename to ")
		case h == nil && strings.HasPrefix(line, "Binary files "):
			f.Binary = true
		case h == nil && strings.HasPrefix(line, "--- "):
			if p := strings.TrimPrefix(line, "--- "); p != "/dev/null" && f.Status == proto.FileDeleted {
				f.Path = strings.TrimPrefix(p, "a/")
			}
		case h == nil && strings.HasPrefix(line, "+++ "):
			if p := strings.TrimPrefix(line, "+++ "); p != "/dev/null" {
				f.Path = strings.TrimPrefix(p, "b/")
			}
		case strings.HasPrefix(line, "@@"):
			f.Hunks = append(f.Hunks, proto.DiffHunk{Header: line})
			h = &f.Hunks[len(f.Hunks)-1]
			oldNo, newNo = hunkStarts(line)
		case h == nil:
		case strings.HasPrefix(line, "+"):
			h.Lines = append(h.Lines, proto.DiffLine{Kind: '+', Text: line[1:], New: newNo})
			newNo++
			f.Added++
		case strings.HasPrefix(line, "-"):
			h.Lines = append(h.Lines, proto.DiffLine{Kind: '-', Text: line[1:], Old: oldNo})
			oldNo++
			f.Removed++
		case strings.HasPrefix(line, " "), line == "":
			h.Lines = append(h.Lines, proto.DiffLine{Kind: ' ', Text: strings.TrimPrefix(line, " "), Old: oldNo, New: newNo})
			oldNo++
			newNo++
		}
		// "\ No newline at end of file" and anything else is left out.
	}
	return files
}

// hunkStarts reads the first line numbers of "@@ -a,b +c,d @@".
func hunkStarts(header string) (old, new int) {
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return 0, 0
	}
	num := func(s string) int {
		s, _, _ = strings.Cut(s[1:], ",")
		n, _ := strconv.Atoi(s)
		return n
	}
	return num(fields[1]), num(fields[2])
}

// untrackedFile shows a file git doesn't track yet as added whole.
func untrackedFile(dir, path string) proto.DiffFile {
	f := proto.DiffFile{Path: path, Status: proto.FileAdded}
	info, err := os.Stat(filepath.Join(dir, path))
	if err != nil || info.Size() > maxUntracked {
		return f
	}
	data, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		return f
	}
	if bytes.IndexByte(data, 0) >= 0 {
		f.Binary = true
		return f
	}
	if len(data) == 0 {
		return f
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	h := proto.DiffHunk{Header: "@@ -0,0 +1," + strconv.Itoa(len(lines)) + " @@"}
	for i, l := range lines {
		h.Lines = append(h.Lines, proto.DiffLine{Kind: '+', Text: l, New: i + 1})
	}
	f.Added, f.Hunks = len(lines), []proto.DiffHunk{h}
	return f
}

// limit cuts d down to the limits.
func limit(d *proto.Diff) {
	if len(d.Files) > maxFiles {
		d.Files, d.Truncated = d.Files[:maxFiles], true
	}
	total := 0
	for i := range d.Files {
		f := &d.Files[i]
		n, budget := 0, min(maxFileLines, maxLines-total)
		for j := range f.Hunks {
			h := &f.Hunks[j]
			if n+len(h.Lines) > budget {
				h.Lines = h.Lines[:max(budget-n, 0)]
				f.Hunks, f.Truncated = f.Hunks[:j+1], true
				if len(h.Lines) == 0 {
					f.Hunks = f.Hunks[:j]
				}
				n = budget
				break
			}
			n += len(h.Lines)
		}
		total += n
	}
}
