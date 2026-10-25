package iconlib

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Ref is an icon value parsed: a library's icon, or the user's own SVG,
// in a color of its own or the text's.
type Ref struct {
	Lib, Name string
	Custom    string // the user's SVG, sanitized
	Color     string // "#rrggbb", "" for the text's
}

var (
	colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	nameRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// Parse reads an icon value: "lib:name" or "svg:<base64>", either with
// "#rrggbb" after it for its color.
func Parse(s string) (Ref, error) {
	s = strings.TrimSpace(s)
	var r Ref
	if i := strings.LastIndexByte(s, '#'); i > 0 && colorRe.MatchString(s[i:]) {
		r.Color, s = strings.ToLower(s[i:]), s[:i]
	}
	kind, rest, ok := strings.Cut(s, ":")
	if !ok || rest == "" {
		return Ref{}, notARef(s)
	}
	if kind == "svg" {
		raw, err := base64.StdEncoding.DecodeString(rest)
		if err != nil {
			return Ref{}, errors.New("icon: svg: isn't base64")
		}
		clean, err := Sanitize(raw, false)
		if err != nil {
			return Ref{}, err
		}
		r.Custom = clean
		return r, nil
	}
	if libs[kind] == nil {
		return Ref{}, fmt.Errorf("icon: no library %q (there are %s)", kind, libraryIDs())
	}
	if !nameRe.MatchString(rest) {
		return Ref{}, fmt.Errorf("icon: %q isn't an icon's name", rest)
	}
	if _, ok := Lookup(kind, rest); !ok {
		hint := ""
		// A typo's start finds what was meant.
		var near []Icon
		for q := rest; len(near) == 0 && len(q) >= 3; q = q[:len(q)-1] {
			near = Search(q, []string{kind}, 3)
		}
		if len(near) > 0 {
			var refs []string
			for _, n := range near {
				refs = append(refs, n.Ref())
			}
			hint = "; near it: " + strings.Join(refs, ", ")
		}
		return Ref{}, fmt.Errorf("icon: %s has no %q%s", kind, rest, hint)
	}
	r.Lib, r.Name = kind, rest
	return r, nil
}

func notARef(s string) error {
	if IsEmoji(s) {
		ref, _ := FromEmoji(s)
		return fmt.Errorf("icon: icons are SVG, not emoji: for %s use %s, or find one with `icons search WORD`", s, ref)
	}
	return fmt.Errorf("icon: %q isn't an icon: give lib:name, like lucide:rocket (find one with `icons search WORD`)", s)
}

// String is the value r is stored as.
func (r Ref) String() string {
	var s string
	if r.Custom != "" {
		s = "svg:" + base64.StdEncoding.EncodeToString([]byte(r.Custom))
	} else if r.Lib != "" {
		s = r.Lib + ":" + r.Name
	}
	if s != "" && r.Color != "" {
		s += r.Color
	}
	return s
}

// SVG is r's drawing.
func (r Ref) SVG() string {
	if r.Custom != "" {
		return r.Custom
	}
	s, _ := Lookup(r.Lib, r.Name)
	return s
}

// Check is icon value s as stored, "" for none, or why it isn't one.
func Check(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	r, err := Parse(s)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

// Custom is the user's SVG as an icon value: sanitized, its colors the
// text's when currentColor.
func Custom(svg []byte, currentColor bool) (string, error) {
	clean, err := Sanitize(svg, currentColor)
	if err != nil {
		return "", err
	}
	if len(clean) > MaxIconSVG {
		return "", fmt.Errorf("icon: the SVG is %d KB once cleaned, more than an icon's %d", len(clean)>>10, MaxIconSVG>>10)
	}
	return Ref{Custom: clean}.String(), nil
}

// IsEmoji says s is an emoji or a few: a value of icons from before they
// were SVG.
func IsEmoji(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, ":") {
		return false
	}
	for _, r := range s {
		if isEmojiRune(r) {
			return true
		}
	}
	return false
}

func isEmojiRune(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF, r >= 0x2600 && r <= 0x27BF, r >= 0x2190 && r <= 0x21FF,
		r >= 0x2300 && r <= 0x23FF, r >= 0x2B00 && r <= 0x2BFF, r >= 0x2900 && r <= 0x297F,
		r == 0x2122, r == 0x2139, r == 0x3030, r == 0x303D, r == 0x3297, r == 0x3299, r == 0x00A9, r == 0x00AE:
		return true
	}
	return false
}

// LeadingEmoji splits a title starting with an emoji into it and the
// rest, as titles of before icons were SVG did ("📌 TODO").
func LeadingEmoji(title string) (emoji, rest string) {
	i := 0
	for i < len(title) {
		r, n := utf8.DecodeRuneInString(title[i:])
		if isEmojiRune(r) || r == 0xFE0F || r == 0x200D || (r >= 0x1F3FB && r <= 0x1F3FF) {
			i += n
			continue
		}
		break
	}
	if i == 0 {
		return "", title
	}
	rest = strings.TrimLeftFunc(title[i:], unicode.IsSpace)
	if rest == "" {
		return "", title // the title is the emoji alone: keep it
	}
	return title[:i], rest
}
