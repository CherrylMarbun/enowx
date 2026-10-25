package widget

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// span is a run of inline Markdown with one style.
type span struct {
	text                       string
	code, bold, italic, struck bool
	url                        string
}

// Inline draws a line of Markdown: `code`, **bold**, *italics*,
// ~~strikethrough~~, [links](url) and bare http(s) links.
func Inline(c *ui.Context, s string) ui.Element { return InlineLinks(c, nil, s) }

// InlineLinks draws a line of Markdown as Inline does, its links opening
// through open first.
func InlineLinks(c *ui.Context, open LinkOpener, s string) ui.Element {
	return inline(c, open, s).Selectable()
}

// InlineLabel draws a line of Markdown as Inline does, but as a label: its
// text isn't selectable, so a press on it reaches what it is in, as a
// block a click opens or a drag moves.
func InlineLabel(c *ui.Context, s string) ui.Element { return inline(c, nil, s) }

func inline(c *ui.Context, open LinkOpener, s string) ui.Element {
	return ui.RichText(c).TextColor(theme.Text).Children(func() {
		for i, sp := range inlineSpans(s) {
			switch {
			case sp.url != "" && open != nil:
				l := ui.Link(c.Key(i), sp.text, "").TextColor(theme.Accent).Underline()
				if l.Clicked() && !open(sp.url) {
					c.OpenURL(sp.url)
				}
			case sp.url != "":
				ui.Link(c.Key(i), sp.text, sp.url).TextColor(theme.Accent).Underline()
			case sp.code:
				ui.Text(c, sp.text).Font(theme.MonoFont).FontSize(12).Background(theme.Raised).TextColor(theme.Accent)
			default:
				t := ui.Text(c, sp.text)
				if sp.bold {
					t.Bold().TextColor(theme.TextStrong)
				}
				if sp.italic {
					t.Italic()
				}
				if sp.struck {
					t.Strikethrough().TextColor(theme.Muted)
				}
			}
		}
	})
}

// inlineSpans splits s into its styled runs. Unclosed markers are text.
func inlineSpans(s string) []span {
	var out []span
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			out = append(out, span{text: plain.String()})
			plain.Reset()
		}
	}
	for i := 0; i < len(s); {
		rest := s[i:]
		switch {
		case rest[0] == '`':
			if end := strings.IndexByte(rest[1:], '`'); end >= 0 {
				flush()
				out = append(out, span{text: rest[1 : 1+end], code: true})
				i += end + 2
				continue
			}
		case strings.HasPrefix(rest, "**") || strings.HasPrefix(rest, "__"):
			if end := strings.Index(rest[2:], rest[:2]); end > 0 {
				flush()
				for _, inner := range inlineSpans(rest[2 : 2+end]) {
					inner.bold = true
					out = append(out, inner)
				}
				i += end + 4
				continue
			}
		case strings.HasPrefix(rest, "~~"):
			if end := strings.Index(rest[2:], "~~"); end > 0 {
				flush()
				out = append(out, span{text: rest[2 : 2+end], struck: true})
				i += end + 4
				continue
			}
		case (rest[0] == '*' || rest[0] == '_') && opensEmphasis(s, i):
			if end := strings.IndexByte(rest[1:], rest[0]); end > 0 && !unicode.IsSpace(rune(rest[end])) {
				flush()
				for _, inner := range inlineSpans(rest[1 : 1+end]) {
					inner.italic = true
					out = append(out, inner)
				}
				i += end + 2
				continue
			}
		case rest[0] == '[':
			if close := strings.Index(rest, "]("); close > 0 {
				if end := strings.IndexByte(rest[close+2:], ')'); end > 0 {
					flush()
					out = append(out, span{text: rest[1:close], url: rest[close+2 : close+2+end]})
					i += close + 3 + end
					continue
				}
			}
		case strings.HasPrefix(rest, "https://") || strings.HasPrefix(rest, "http://"):
			end := strings.IndexFunc(rest, func(r rune) bool { return unicode.IsSpace(r) || r == ')' || r == '>' })
			if end < 0 {
				end = len(rest)
			}
			url := strings.TrimRight(rest[:end], ".,;:")
			flush()
			out = append(out, span{text: url, url: url})
			i += len(url)
			continue
		}
		plain.WriteByte(s[i])
		i++
	}
	flush()
	return out
}

// opensEmphasis reports whether the * or _ at i can open italics: not in
// the middle of a word (snake_case), and followed by text.
func opensEmphasis(s string, i int) bool {
	if i+1 >= len(s) || unicode.IsSpace(rune(s[i+1])) {
		return false
	}
	return i == 0 || !isWordByte(s[i-1])
}

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
