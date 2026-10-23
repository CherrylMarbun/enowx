package review

import (
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/git"
)

// Span is a run of lines of a file as its base has it, [Start, End).
type Span struct{ Start, End int }

// Touch is how a session changed a file: the lines of the base it
// changed, or Whole for a file it added.
type Touch struct {
	Spans []Span
	Whole bool
}

// Touched returns the files dir changed against base, as Diff compares,
// with the lines of the base each change covers.
func Touched(dir, base string) (map[string]Touch, error) {
	from, err := mergeBase(dir, base)
	if err != nil {
		return nil, err
	}
	out, err := git.Run(dir, "-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff", "-U0", "--no-renames", from, "--")
	if err != nil {
		return nil, err
	}
	touched := map[string]Touch{}
	file := ""
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			file = ""
			if p := strings.TrimPrefix(line, "+++ "); p != "/dev/null" {
				file = strings.TrimPrefix(p, "b/")
			}
		case strings.HasPrefix(line, "--- ") && file == "":
			// A deleted file names itself only here.
			if p := strings.TrimPrefix(line, "--- "); p != "/dev/null" {
				file = strings.TrimPrefix(p, "a/")
			}
		case strings.HasPrefix(line, "--- "):
		case strings.HasPrefix(line, "diff --git "):
			file = ""
		case strings.HasPrefix(line, "@@") && file != "":
			t := touched[file]
			t.Spans = append(t.Spans, oldSpan(line))
			touched[file] = t
		}
	}
	// Added files, tracked or not, conflict with any change to them.
	added, _ := git.Run(dir, "-c", "core.quotePath=false", "diff", "--name-only", "--diff-filter=A", from, "--")
	untracked, _ := git.Run(dir, "-c", "core.quotePath=false", "ls-files", "--others", "--exclude-standard")
	for _, list := range []string{added, untracked} {
		for _, p := range strings.Split(list, "\n") {
			if p != "" {
				touched[p] = Touch{Whole: true}
			}
		}
	}
	return touched, nil
}

// oldSpan reads the base's lines of a hunk header, "@@ -a,b +c,d @@": a
// change that only adds lines covers the line it adds after.
func oldSpan(header string) Span {
	f := strings.Fields(header)
	if len(f) < 2 {
		return Span{}
	}
	a, b, ok := strings.Cut(strings.TrimPrefix(f[1], "-"), ",")
	start, _ := strconv.Atoi(a)
	count := 1
	if ok {
		count, _ = strconv.Atoi(b)
	}
	if count == 0 {
		return Span{start, start + 1}
	}
	return Span{start, start + count}
}

// Overlap reports whether two changes to a file cover a line in common.
func Overlap(a, b Touch) bool {
	if a.Whole || b.Whole {
		return true
	}
	for _, x := range a.Spans {
		for _, y := range b.Spans {
			if x.Start < y.End && y.Start < x.End {
				return true
			}
		}
	}
	return false
}
