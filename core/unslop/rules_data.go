package unslop

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/enowx/enowx/proto"
)

// The check of sample data: figures a generator rounds (1,000, 50%) and
// times it repeats ("2 hours ago" on every row), where real data varies.
// Invented names and brands are the truth pass's (fake-data).

// roundRe are round figures: thousands of a round count (1,000, 20,000,
// 1,000,000) and round percentages (10%, +50%, 25%).
var roundRe = regexp.MustCompile(`(?:^|[^\d.,])([+−-]?(?:[1-9]0{0,2}(?:,000)+|(?:[1-9]0|100|25|75)%))`)

// agoRe is a relative time: "2 hours ago", "a day ago".
var agoRe = regexp.MustCompile(`(?i)\b(\d+|an?|one)\s+(second|minute|min|hour|hr|day|week|month|year)s?\s+ago\b`)

func sampleData(p page) (proto.UnslopFinding, bool) {
	var round, roundAt []string
	times := map[string][]string{}
	for _, tt := range p.texts {
		for _, m := range roundRe.FindAllStringSubmatchIndex(tt.s, -1) {
			if end := m[3]; end < len(tt.s) && tt.s[end] == '+' {
				continue // "10,000+" is fake-data's
			}
			round = append(round, "“"+tt.s[m[2]:m[3]]+"”")
			roundAt = append(roundAt, tt.target)
		}
		for _, m := range agoRe.FindAllString(tt.s, -1) {
			k := strings.ToLower(m)
			times[k] = append(times[k], tt.target)
		}
	}
	var ex, at []string
	if len(round) >= 3 {
		ex, at = append(ex, fmt.Sprintf("%d round figures: %s", len(round), strings.Join(unique(round)[:min(3, len(unique(round)))], ", "))), roundAt
	}
	keys := make([]string, 0, len(times))
	for k := range times {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(times[k]) >= 3 {
			ex = append(ex, fmt.Sprintf("“%s” %d times", k, len(times[k])))
			at = append(at, times[k]...)
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("data", "sample-data", proto.UnslopMedium, "Figures that look generated",
		"Show real figures, or a labelled placeholder; real data isn't round everywhere and its times differ.", ex, at...), true
}
