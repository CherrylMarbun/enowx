package notesio

import (
	"regexp"
	"strings"
)

// A note's Markdown as blocks, for writing it as HTML or PDF: the forms
// the Notes page edits (headings, lists, to-dos, quotes, code, tables,
// dividers, paragraphs).

type mdKind int

const (
	mdPara mdKind = iota
	mdHeading
	mdBullet
	mdNumber
	mdTodo
	mdQuote
	mdCode
	mdTable
	mdDivider
	mdImage
)

type mdBlock struct {
	kind   mdKind
	text   string
	level  int // a heading's 1–3; a list item's depth
	done   bool
	number int
	lang   string
	rows   [][]string
	src    string // an image's
}

var (
	mdNumberRe = regexp.MustCompile(`^(\d+)[.)] `)
	mdTodoRe   = regexp.MustCompile(`^[-*+] \[([ xX])\] `)
	mdImageRe  = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)\s]+)\)$`)
)

func parseMarkdown(md string) []mdBlock {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []mdBlock
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		rest := strings.TrimLeft(line, " \t")
		depth := (len(line) - len(rest)) / 2
		switch {
		case strings.TrimSpace(line) == "":
			continue
		case strings.HasPrefix(rest, "```"):
			b := mdBlock{kind: mdCode, lang: strings.TrimSpace(rest[3:])}
			var code []string
			for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "```"; i++ {
				code = append(code, lines[i])
			}
			b.text = strings.Join(code, "\n")
			out = append(out, b)
		case strings.HasPrefix(rest, "|"):
			b := mdBlock{kind: mdTable}
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				row := splitRow(lines[i])
				if isSeparator(row) {
					continue
				}
				b.rows = append(b.rows, row)
			}
			i--
			out = append(out, b)
		case rest == "---" || rest == "***" || rest == "___":
			out = append(out, mdBlock{kind: mdDivider})
		case strings.HasPrefix(rest, "### "):
			out = append(out, mdBlock{kind: mdHeading, level: 3, text: strings.TrimLeft(rest[4:], "# ")})
		case strings.HasPrefix(rest, "## "):
			out = append(out, mdBlock{kind: mdHeading, level: 2, text: rest[3:]})
		case strings.HasPrefix(rest, "# "):
			out = append(out, mdBlock{kind: mdHeading, level: 1, text: rest[2:]})
		case strings.HasPrefix(rest, ">"):
			out = append(out, mdBlock{kind: mdQuote, text: strings.TrimPrefix(strings.TrimPrefix(rest, ">"), " ")})
		case mdTodoRe.MatchString(rest):
			m := mdTodoRe.FindStringSubmatch(rest)
			out = append(out, mdBlock{kind: mdTodo, text: rest[len(m[0]):], done: m[1] != " ", level: depth})
		case len(rest) > 1 && strings.ContainsRune("-*+", rune(rest[0])) && rest[1] == ' ':
			out = append(out, mdBlock{kind: mdBullet, text: rest[2:], level: depth})
		case mdNumberRe.MatchString(rest):
			m := mdNumberRe.FindStringSubmatch(rest)
			n := 0
			for _, r := range m[1] {
				n = n*10 + int(r-'0')
			}
			out = append(out, mdBlock{kind: mdNumber, text: rest[len(m[0]):], level: depth, number: n})
		case mdImageRe.MatchString(strings.TrimSpace(rest)):
			m := mdImageRe.FindStringSubmatch(strings.TrimSpace(rest))
			out = append(out, mdBlock{kind: mdImage, text: m[1], src: m[2]})
		default:
			out = append(out, mdBlock{kind: mdPara, text: strings.TrimSpace(line)})
		}
	}
	return out
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			cur.WriteByte('|')
			i++
		case line[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(line[i])
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

func isSeparator(row []string) bool {
	for _, c := range row {
		if strings.Trim(c, ":- ") != "" || c == "" {
			return false
		}
	}
	return true
}

// Inline marks, read for HTML and taken off for PDF.
var (
	inlineImage = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	inlineLink  = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	inlineWiki  = regexp.MustCompile(`\[\[([^\]|#]+)(#[^\]|]*)?(\|[^\]]*)?\]\]`)
	inlineBold  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	inlineItal  = regexp.MustCompile(`\*([^*\s][^*]*)\*`)
	inlineStrk  = regexp.MustCompile(`~~([^~]+)~~`)
	inlineCode  = regexp.MustCompile("`([^`]+)`")
)

// plainText is a line's words without their marks.
func plainText(s string) string {
	s = inlineImage.ReplaceAllString(s, "$1")
	s = inlineWiki.ReplaceAllStringFunc(s, func(m string) string {
		sub := inlineWiki.FindStringSubmatch(m)
		if sub[3] != "" {
			return sub[3][1:]
		}
		return sub[1]
	})
	s = inlineLink.ReplaceAllString(s, "$1")
	s = inlineBold.ReplaceAllString(s, "$1")
	s = inlineStrk.ReplaceAllString(s, "$1")
	s = inlineCode.ReplaceAllString(s, "$1")
	s = inlineItal.ReplaceAllString(s, "$1")
	return s
}
