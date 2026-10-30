package unslop

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// items is a lint list of n elements, each with fields (JSON members
// without braces) besides its selector and label.
func items(n int, fields string) string {
	var out []string
	for i := range n {
		f := fmt.Sprintf(`"s":"#e%d","d":"p#e%d “text”"`, i, i)
		if fields != "" {
			f += "," + fields
		}
		out = append(out, "{"+f+"}")
	}
	return "[" + strings.Join(out, ",") + "]"
}

func fired(raw, rule string) (proto.UnslopFinding, bool) {
	for _, f := range LintFindings(raw, 1200) {
		if f.Rule == rule {
			return f, true
		}
	}
	return proto.UnslopFinding{}, false
}

// Each rule of the rendered lint fires past its threshold and not at it.
func TestLintThresholds(t *testing.T) {
	origins := func(n int) string {
		var o []string
		for i := range n {
			o = append(o, fmt.Sprintf(`{"x":%d,"n":2,"d":"p “a”","s":"#o%d"}`, i*40, i))
		}
		return `{"anchors":[` + strings.Join(o, ",") + `]}`
	}
	cases := []struct {
		rule, raw string
		want      bool
	}{
		{"contrast-low", `{"contrast":` + items(1, `"ratio":4.4`) + `}`, true},
		{"contrast-low", `{"contrast":` + items(1, `"ratio":4.5`) + `}`, false},
		{"contrast-low", `{"contrast":` + items(1, `"ratio":3.1,"large":true`) + `}`, false},
		{"contrast-low", `{"contrast":` + items(1, `"ratio":2.9,"large":true`) + `}`, true},
		{"contrast-low", `{"contrast":` + items(1, `"ratio":1.2,"plate":true`) + `}`, false},
		{"contrast-low", `{"contrast":` + items(1, `"ratio":1.2`) + `}`, true},
		{"small-body-text", `{"body":` + items(1, `"size":12.4,"leading":1.5`) + `}`, true},
		{"small-body-text", `{"body":` + items(1, `"size":12.5,"leading":1.5`) + `}`, false},
		{"tight-leading", `{"body":` + items(1, `"size":16,"leading":1.29`) + `}`, true},
		{"tight-leading", `{"body":` + items(1, `"size":16,"leading":1.3`) + `}`, false},
		{"tight-leading", `{"body":` + items(1, `"size":16,"leading":1.35,"cjk":true`) + `}`, true},
		{"long-measure", `{"body":` + items(1, `"size":16,"leading":1.5,"cpl":96`) + `}`, true},
		{"long-measure", `{"body":` + items(1, `"size":16,"leading":1.5,"cpl":95`) + `}`, false},
		{"long-measure", `{"body":` + items(1, `"size":16,"leading":1.6,"cpl":47,"cjk":true`) + `}`, true},
		{"long-measure", `{"body":` + items(1, `"size":16,"leading":1.6,"cpl":46,"cjk":true`) + `}`, false},
		{"stuck-hidden", `{"stuck":` + items(1, "") + `}`, true},
		{"stuck-hidden", `{"stuck":[]}`, false},
		{"gray-on-color", `{"gray":` + items(1, `"bg":0.25,"text":0.05,"lum":0.3`) + `}`, true},
		{"gray-on-color", `{"gray":` + items(1, `"bg":0.24,"text":0.05,"lum":0.3`) + `}`, false},
		{"gray-on-color", `{"gray":` + items(1, `"bg":0.3,"text":0.05,"lum":0.7`) + `}`, false},
		{"kicker-label", `{"kickers":` + items(1, `"how":"uppercase","h":"h2 “Pricing”"`) + `}`, false},
		{"kicker-label", `{"kickers":` + items(2, `"how":"uppercase","h":"h2 “Pricing”"`) + `}`, true},
		{"numbered-sections", `{"numbered":` + items(1, `"h":"h2 “Plans”"`) + `}`, false},
		{"numbered-sections", `{"numbered":` + items(2, `"h":"h2 “Plans”"`) + `}`, true},
		{"heading-gap", `{"gaps":` + items(1, `"above":10,"below":15`) + `}`, true},
		{"heading-gap", `{"gaps":` + items(1, `"above":12,"below":15`) + `}`, false},
		{"heading-gap", `{"gaps":` + items(1, `"above":0,"below":11`) + `}`, false},
		{"side-stripe", `{"stripes":` + items(1, `"how":"border-left 4px"`) + `}`, true},
		{"gradient-text", `{"gradtext":` + items(1, "") + `}`, true},
		{"nested-cards", `{"nested":` + items(1, `"outer":"div.card"`) + `}`, true},
		{"emoji-icons", `{"emoji":` + items(1, "") + `}`, false},
		{"emoji-icons", `{"emoji":` + items(2, "") + `}`, true},
		{"text-anchors", origins(4), false},
		{"text-anchors", origins(5), true},
		{"boxed-everything", `{"boxed":{"texts":10,"inBox":8,"boxes":6,"ex":["div.box"],"at":["#b"]}}`, true},
		{"boxed-everything", `{"boxed":{"texts":10,"inBox":7,"boxes":6}}`, false},
		{"boxed-everything", `{"boxed":{"texts":10,"inBox":10,"boxes":5}}`, false},
	}
	for _, c := range cases {
		if _, got := fired(c.raw, c.rule); got != c.want {
			t.Errorf("%s on %s: fired %v, want %v", c.rule, c.raw, got, c.want)
		}
	}
	if got := LintFindings("undefined", 1200); got != nil {
		t.Errorf("an answer not JSON: %+v", got)
	}
}

// A page's lint: findings in pass order, each in its pass and severity,
// counted once per element, a few examples and its elements to point at.
func TestLintFindings(t *testing.T) {
	raw := `{"vw":390,"vh":800,
	"contrast":` + items(5, `"ratio":2.1`) + `,
	"stuck":[{"s":"#hero","d":"p “Ship”"},{"s":"#hero","d":"h1 “Ship”"}],
	"kickers":` + items(3, `"how":"uppercase tracked","h":"h2 “Plans”"`) + `,
	"body":` + items(1, `"size":11,"leading":1.1,"cpl":120`) + `}`
	got := LintFindings(raw, 0)
	if r := rules(got); !slices.Equal(r, []string{"kicker-label", "small-body-text", "tight-leading", "long-measure", "stuck-hidden", "contrast-low"}) {
		t.Fatalf("rules: %v", r)
	}
	want := map[string][2]string{
		"kicker-label": {"02 Structure", proto.UnslopMedium}, "small-body-text": {"04 Type", proto.UnslopHigh},
		"long-measure": {"04 Type", proto.UnslopLow}, "stuck-hidden": {"10 Motion", proto.UnslopHigh}, "contrast-low": {"11 Access", proto.UnslopHigh},
	}
	for _, f := range got {
		if w, ok := want[f.Rule]; ok && (f.Pass != w[0] || f.Severity != w[1]) {
			t.Errorf("%s: %s %s, want %v", f.Rule, f.Pass, f.Severity, w)
		}
		if len(f.Examples) > 3 || len(f.Targets) == 0 || f.Fix == "" {
			t.Errorf("%s: examples %v, targets %v", f.Rule, f.Examples, f.Targets)
		}
		switch f.Rule {
		case "contrast-low":
			if f.Count != 5 || f.Examples[0] != "p#e0 “text” 2.10:1" {
				t.Errorf("contrast: %d %v", f.Count, f.Examples)
			}
		case "stuck-hidden":
			if f.Count != 1 || !slices.Equal(f.Targets, []string{"#hero"}) || !strings.Contains(f.Title, "390 px") {
				t.Errorf("stuck: %+v", f)
			}
		}
	}
}

// Measured rendered, a rule replaces the static read's guess of it: the
// rendered kicker, cards and gradient text stand, the static ones go;
// the rest of both stay, in pass order.
func TestWithRendered(t *testing.T) {
	static := []proto.UnslopFinding{
		{Rule: "eyebrow-pill", Pass: "02 Structure"}, {Rule: "cards-in-cards", Pass: "02 Structure"},
		{Rule: "gradient-text", Pass: "05 Color", Severity: proto.UnslopHigh}, {Rule: "emoji", Pass: "07 Icons"},
	}
	rendered := []proto.UnslopFinding{
		{Rule: "overflow", Pass: "12 Mobile"}, {Rule: "kicker-label", Pass: "02 Structure"},
		{Rule: "gradient-text", Pass: "05 Color", Severity: proto.UnslopMedium}, {Rule: "nested-cards", Pass: "06 Surfaces"},
	}
	got := WithRendered(static, rendered)
	if r := rules(got); !slices.Equal(r, []string{"kicker-label", "gradient-text", "nested-cards", "emoji", "overflow"}) || got[1].Severity != proto.UnslopMedium {
		t.Fatalf("merged: %v", r)
	}
	if r := rules(WithRendered(static, nil)); len(r) != len(static) {
		t.Fatalf("nothing rendered: %v", r)
	}
}

// Sample data: round figures and the same relative time, three times or
// more; a few, or times that differ, are real data's.
func TestSampleData(t *testing.T) {
	has := func(html, rule string) (proto.UnslopFinding, bool) {
		_, found := Scan(html, proto.CanvasMockup)
		for _, f := range found {
			if f.Rule == rule {
				return f, true
			}
		}
		return proto.UnslopFinding{}, false
	}
	if f, ok := has(`<p>1,000 users</p><p>+50% growth</p><p>10,000 events</p>`, "sample-data"); !ok || f.Pass != "09 Data" {
		t.Errorf("round figures: %+v", f)
	}
	if _, ok := has(`<p>1,000 users</p><p>+47% growth</p><p>10,000+ events</p><p>1,284 runs</p>`, "sample-data"); ok {
		t.Error("two round figures found")
	}
	if f, ok := has(`<li>Ada · 2 hours ago</li><li>Bo · 2 hours ago</li><li>Cy · 2 hours ago</li>`, "sample-data"); !ok || !strings.Contains(f.Examples[0], "3 times") {
		t.Errorf("the same time: %+v", f)
	}
	if _, ok := has(`<li>2 hours ago</li><li>5 hours ago</li><li>a day ago</li>`, "sample-data"); ok {
		t.Error("times that differ found")
	}
	if f, ok := has(`<h3>Card title</h3><p>Example Inc</p>`, "fake-data"); !ok || f.Count != 2 {
		t.Errorf("placeholders: %+v", f)
	}
	f, ok := has(`<a>Learn more</a><a>Learn more →</a><a>Learn more</a>`, "generic-cta")
	if !ok || f.Severity != proto.UnslopMedium || f.Examples[0] != "“learn more” 3 times" {
		t.Errorf("the same generic button: %+v", f)
	}
	if f, ok := has(`<a>Read more</a>`, "generic-cta"); !ok || f.Severity != proto.UnslopLow {
		t.Errorf("one generic button: %+v", f)
	}
}
