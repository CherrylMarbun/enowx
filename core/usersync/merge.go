package usersync

import (
	"slices"
	"strings"
)

// merge3 merges the changes a and b made to base, line by line, as
// ok == false when both changed the same lines (or lines next to each
// other) differently.
func merge3(base, a, b string) (string, bool) {
	if a == b {
		return a, true
	}
	if a == base {
		return b, true
	}
	if b == base {
		return a, true
	}
	bl, al, cl := lines(base), lines(a), lines(b)
	ha, hb := diff(bl, al), diff(bl, cl)
	type tagged struct {
		hunk
		side int
	}
	var all []tagged
	for _, h := range ha {
		all = append(all, tagged{h, 0})
	}
	for _, h := range hb {
		all = append(all, tagged{h, 1})
	}
	slices.SortStableFunc(all, func(x, y tagged) int { return x.bs - y.bs })
	var out []string
	pos := 0 // base lines up to here are out
	for i := 0; i < len(all); {
		gs, ge := all[i].bs, all[i].be
		sides := [2]bool{}
		j := i
		for ; j < len(all) && all[j].bs <= ge; j++ {
			ge = max(ge, all[j].be)
			sides[all[j].side] = true
		}
		out = append(out, bl[pos:gs]...)
		ta := region(al, ha, gs, ge)
		tb := region(cl, hb, gs, ge)
		switch {
		case !sides[1]:
			out = append(out, ta...)
		case !sides[0]:
			out = append(out, tb...)
		case slices.Equal(ta, tb):
			out = append(out, ta...)
		default:
			return "", false
		}
		pos, i = ge, j
	}
	out = append(out, bl[pos:]...)
	return strings.Join(out, ""), true
}

// region is side's lines standing for base lines [gs, ge), given the
// hunks it made: every hunk touching them is inside.
func region(side []string, hs []hunk, gs, ge int) []string {
	start, end := gs, ge
	for _, h := range hs {
		if h.be <= gs && h.bs < gs {
			start += (h.se - h.ss) - (h.be - h.bs)
		}
		if h.bs <= ge && h.be <= ge {
			end += (h.se - h.ss) - (h.be - h.bs)
		}
	}
	return side[start:end]
}

// lines splits s after each newline, keeping them, so joining gives s.
func lines(s string) []string {
	if s == "" {
		return nil
	}
	out := strings.SplitAfter(s, "\n")
	if out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}
