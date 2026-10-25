package usersync

// hunk is a change one side made to the base: base lines [bs, be)
// became its lines [ss, se).
type hunk struct{ bs, be, ss, se int }

// diff is the hunks turning a into b, line by line, from their longest
// common subsequence (Myers' algorithm, its middle-free form: notes are
// small).
func diff(a, b []string) []hunk {
	n, m := len(a), len(b)
	max := n + m
	if max == 0 {
		return nil
	}
	v := make([]int, 2*max+2)
	var trace [][]int
	found := false
	for d := 0; d <= max && !found; d++ {
		trace = append(trace, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || k != d && v[max+k-1] < v[max+k+1] {
				x = v[max+k+1]
			} else {
				x = v[max+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			v[max+k] = x
			if x >= n && y >= m {
				found = true
				break
			}
		}
	}
	// Back through the trace, the matched lines, last first.
	type pair struct{ x, y int }
	var matches []pair
	x, y := n, m
	for d := len(trace) - 1; d > 0; d-- {
		v := trace[d]
		k := x - y
		var pk int
		if k == -d || k != d && v[max+k-1] < v[max+k+1] {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := v[max+pk]
		py := px - pk
		for x > px && y > py {
			x, y = x-1, y-1
			matches = append(matches, pair{x, y})
		}
		x, y = px, py
	}
	for x > 0 && y > 0 {
		x, y = x-1, y-1
		matches = append(matches, pair{x, y})
	}
	var out []hunk
	i, j := 0, 0
	for k := len(matches) - 1; k >= -1; k-- {
		mx, my := n, m
		if k >= 0 {
			mx, my = matches[k].x, matches[k].y
		}
		if mx > i || my > j {
			out = append(out, hunk{i, mx, j, my})
		}
		i, j = mx+1, my+1
	}
	return out
}
