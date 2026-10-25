package usersync

import "testing"

func TestMerge3(t *testing.T) {
	base := "a\nb\nc\nd\ne\n"
	for _, tc := range []struct {
		name, a, b, want string
		ok               bool
	}{
		{"apart", "A\nb\nc\nd\ne\n", "a\nb\nc\nd\nE\n", "A\nb\nc\nd\nE\n", true},
		{"same change", "a\nB\nc\nd\ne\n", "a\nB\nc\nd\ne\n", "a\nB\nc\nd\ne\n", true},
		{"one side", base, "a\nb\nx\ny\nd\ne\n", "a\nb\nx\ny\nd\ne\n", true},
		{"inserts apart", "a\nnew1\nb\nc\nd\ne\n", "a\nb\nc\nd\ne\nnew2\n", "a\nnew1\nb\nc\nd\ne\nnew2\n", true},
		{"delete and edit apart", "b\nc\nd\ne\n", "a\nb\nc\nD\ne\n", "b\nc\nD\ne\n", true},
		{"clash", "a\nX\nc\nd\ne\n", "a\nY\nc\nd\ne\n", "", false},
		{"same place inserts", "a\nb\nX\nc\nd\ne\n", "a\nb\nY\nc\nd\ne\n", "", false},
		{"no final newline", "a\nb\nc\nd\ne", "A\nb\nc\nd\ne\n", "A\nb\nc\nd\ne", true},
	} {
		got, ok := merge3(base, tc.a, tc.b)
		if ok != tc.ok || ok && got != tc.want {
			t.Errorf("%s: merge3 = %q, %v; want %q, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestDiffRoundTrips(t *testing.T) {
	cases := [][2]string{
		{"", "a\n"}, {"a\n", ""}, {"a\nb\nc\n", "c\nb\na\n"},
		{"x\ny\nz\n", "x\nq\nz\nw\n"}, {"1\n2\n3\n4\n5\n6\n", "0\n2\n3\n5\n6\n7\n"},
	}
	for _, c := range cases {
		a, b := lines(c[0]), lines(c[1])
		var out []string
		pos := 0
		for _, h := range diff(a, b) {
			out = append(out, a[pos:h.bs]...)
			out = append(out, b[h.ss:h.se]...)
			pos = h.be
		}
		out = append(out, a[pos:]...)
		got := ""
		for _, l := range out {
			got += l
		}
		if got != c[1] {
			t.Errorf("diff(%q, %q) applied = %q", c[0], c[1], got)
		}
	}
}

func TestMergeNoteHeaders(t *testing.T) {
	base := "---\ntitle: Plan\norder: 1\nupdated: 2026-10-01T00:00:00Z\n---\n\none\ntwo\nthree\n"
	a := "---\ntitle: Plan v2\norder: 1\nupdated: 2026-10-02T00:00:00Z\n---\n\nONE\ntwo\nthree\n"
	b := "---\ntitle: Plan\norder: 3\nicon: lucide:rocket\nupdated: 2026-10-03T00:00:00Z\n---\n\none\ntwo\nTHREE\n"
	got, ok := mergeFile("notes/abc.md", []byte(base), []byte(a), []byte(b))
	want := "---\ntitle: Plan v2\norder: 3\nupdated: 2026-10-03T00:00:00Z\nicon: lucide:rocket\n---\n\nONE\ntwo\nTHREE\n"
	if !ok || string(got) != want {
		t.Errorf("merged = %q, %v; want %q", got, ok, want)
	}
	clash := "---\ntitle: Other\norder: 1\nupdated: 2026-10-04T00:00:00Z\n---\n\none\ntwo\nthree\n"
	if _, ok := mergeFile("notes/abc.md", []byte(base), []byte(a), []byte(clash)); ok {
		t.Error("two titles merged")
	}
}

func TestMergeJSONLists(t *testing.T) {
	base := `[{"name":"TODO","icon":"a"},{"name":"Ideas"}]`
	a := `[{"name":"TODO","icon":"b"},{"name":"Ideas"},{"name":"Work"}]`
	b := `[{"name":"TODO","icon":"a"},{"name":"Home"}]`
	got, ok := mergeFile("notes/groups.json", []byte(base), []byte(a), []byte(b))
	if !ok {
		t.Fatal("didn't merge")
	}
	want := "[\n  {\n    \"icon\": \"b\",\n    \"name\": \"TODO\"\n  },\n  {\n    \"name\": \"Work\"\n  },\n  {\n    \"name\": \"Home\"\n  }\n]"
	if string(got) != want {
		t.Errorf("merged = %s", got)
	}
}
