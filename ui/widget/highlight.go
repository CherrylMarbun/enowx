package widget

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// keywords are the common keywords of the languages agents write most;
// one set serves all, which is enough for a light touch of color.
var keywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`break case catch class const continue def default defer do elif else
		enum except export extends false finally fn for from func function go if impl import in interface
		let loop map match mod mut new nil none null package pub return select self static struct super
		switch this throw true try type typeof use var void while with yield async await range chan
		local then end echo fi esac done`) {
		keywords[k] = true
	}
}

// highlight draws code inside a RichText: comments, strings, numbers and
// keywords colored, the rest as it is.
func highlight(c *ui.Context, lang, code string) {
	hash := map[string]bool{"sh": true, "bash": true, "zsh": true, "shell": true, "python": true, "py": true,
		"ruby": true, "rb": true, "yaml": true, "yml": true, "toml": true, "make": true, "makefile": true, "dockerfile": true}[strings.ToLower(lang)]
	if strings.ToLower(lang) == "diff" {
		for i, line := range strings.SplitAfter(code, "\n") {
			col := theme.Text
			switch {
			case strings.HasPrefix(line, "+"):
				col = theme.Done
			case strings.HasPrefix(line, "-"):
				col = theme.Error
			case strings.HasPrefix(line, "@@"):
				col = theme.Working
			}
			ui.Text(c.Key(i), line).TextColor(col)
		}
		return
	}
	n := 0
	emit := func(s string, col ui.Color) {
		if s != "" {
			ui.Text(c.Key(n), s).TextColor(col)
			n++
		}
	}
	var plain strings.Builder
	flush := func() { emit(plain.String(), theme.Text); plain.Reset() }
	for i := 0; i < len(code); {
		rest := code[i:]
		switch {
		case strings.HasPrefix(rest, "//") || strings.HasPrefix(rest, "--") && !hash || hash && rest[0] == '#':
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				end = len(rest)
			}
			flush()
			emit(rest[:end], theme.Muted)
			i += end
			continue
		case rest[0] == '"' || rest[0] == '\'' || rest[0] == '`':
			end := closingQuote(rest)
			flush()
			emit(rest[:end], theme.Done)
			i += end
			continue
		case isWordByte(rest[0]) && (i == 0 || !isWordByte(code[i-1])):
			end := 1
			for end < len(rest) && isWordByte(rest[end]) {
				end++
			}
			word := rest[:end]
			switch {
			case keywords[word]:
				flush()
				emit(word, theme.Working)
			case word[0] >= '0' && word[0] <= '9':
				flush()
				emit(word, theme.Warning)
			default:
				plain.WriteString(word)
			}
			i += end
			continue
		}
		plain.WriteByte(code[i])
		i++
	}
	flush()
}

// closingQuote is the length of the string literal rest starts with, to
// its closing quote or the end of the line.
func closingQuote(rest string) int {
	q := rest[0]
	for j := 1; j < len(rest); j++ {
		switch rest[j] {
		case '\\':
			j++
		case q:
			return j + 1
		case '\n':
			if q != '`' {
				return j
			}
		}
	}
	return len(rest)
}
