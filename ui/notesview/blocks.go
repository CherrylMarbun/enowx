package notesview

import (
	"regexp"
	"strconv"
	"strings"
)

// A page's body is Markdown, edited a block at a time as a notebook's:
// each line is a block (a fenced code block or a table is one), which the
// editor shows as it reads and opens as text when clicked.

type kind int

const (
	kText kind = iota
	kH1
	kH2
	kH3
	kBullet
	kNumber
	kTodo
	kQuote
	kCode
	kTable
	kDivider
)

// block is one block of a page.
type block struct {
	kind   kind
	text   string // without its marks: a heading's words, a code block's code
	done   bool   // a to-do's
	indent int    // a list item's depth
	lang   string // a code block's
}

var (
	numberRe = regexp.MustCompile(`^\d+[.)] `)
	todoRe   = regexp.MustCompile(`^[-*+] \[([ xX])\] `)
)

// parseBlocks reads Markdown into blocks.
func parseBlocks(md string) []block {
	md = strings.TrimSuffix(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	if md == "" {
		return []block{{}}
	}
	lines := strings.Split(md, "\n")
	var out []block
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			b := block{kind: kCode, lang: strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))}
			var code []string
			for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "```"; i++ {
				code = append(code, lines[i])
			}
			b.text = strings.Join(code, "\n")
			out = append(out, b)
			continue
		case strings.HasPrefix(trimmed, "|"):
			rows := []string{line}
			for i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "|") {
				i++
				rows = append(rows, lines[i])
			}
			out = append(out, block{kind: kTable, text: strings.Join(rows, "\n")})
			continue
		}
		out = append(out, parseLine(line))
	}
	return out
}

// parseLine is a line's block.
func parseLine(line string) block {
	rest := strings.TrimLeft(line, " \t")
	indent := 0
	for _, r := range line[:len(line)-len(rest)] {
		if r == '\t' {
			indent += 2
		} else {
			indent++
		}
	}
	indent /= 2
	switch {
	case rest == "---" || rest == "***" || rest == "___":
		return block{kind: kDivider}
	case strings.HasPrefix(rest, "### "):
		return block{kind: kH3, text: strings.TrimLeft(rest[4:], "# ")}
	case strings.HasPrefix(rest, "## "):
		return block{kind: kH2, text: rest[3:]}
	case strings.HasPrefix(rest, "# "):
		return block{kind: kH1, text: rest[2:]}
	case strings.HasPrefix(rest, "> "), rest == ">":
		return block{kind: kQuote, text: strings.TrimPrefix(strings.TrimPrefix(rest, ">"), " ")}
	}
	if m := todoRe.FindStringSubmatch(rest); m != nil {
		return block{kind: kTodo, text: rest[len(m[0]):], done: m[1] != " ", indent: indent}
	}
	if len(rest) > 1 && strings.ContainsRune("-*+", rune(rest[0])) && rest[1] == ' ' {
		return block{kind: kBullet, text: rest[2:], indent: indent}
	}
	if m := numberRe.FindString(rest); m != "" {
		return block{kind: kNumber, text: rest[len(m):], indent: indent}
	}
	return block{kind: kText, text: line}
}

// renderBlocks writes blocks as Markdown, numbering numbered lists anew.
func renderBlocks(bs []block) string {
	var b strings.Builder
	numbers := map[int]int{} // the next number at each depth
	for i, bl := range bs {
		if bl.kind != kNumber {
			if bl.kind != kBullet && bl.kind != kTodo {
				clear(numbers)
			}
		} else {
			numbers[bl.indent]++
			for d := range numbers {
				if d > bl.indent {
					delete(numbers, d)
				}
			}
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(bl.markdown(numbers[bl.indent]))
	}
	out := b.String()
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return out + "\n"
}

// markdown is the block as Markdown; n is a numbered item's number.
func (b block) markdown(n int) string {
	pad := strings.Repeat("  ", b.indent)
	switch b.kind {
	case kH1:
		return "# " + b.text
	case kH2:
		return "## " + b.text
	case kH3:
		return "### " + b.text
	case kBullet:
		return pad + "- " + b.text
	case kNumber:
		return pad + strconv.Itoa(max(n, 1)) + ". " + b.text
	case kTodo:
		mark := " "
		if b.done {
			mark = "x"
		}
		return pad + "- [" + mark + "] " + b.text
	case kQuote:
		return "> " + b.text
	case kCode:
		return "```" + b.lang + "\n" + b.text + "\n```"
	case kDivider:
		return "---"
	}
	return b.text
}

// isList says the block is an item of a list, which indents.
func (b block) isList() bool { return b.kind == kBullet || b.kind == kNumber || b.kind == kTodo }

// shortcuts are what typed at a text block's start makes it another kind,
// as Markdown writes it.
var shortcuts = []struct {
	prefix string
	kind   kind
	done   bool
}{
	{"# ", kH1, false}, {"## ", kH2, false}, {"### ", kH3, false},
	{"- [ ] ", kTodo, false}, {"- [x] ", kTodo, true}, {"[] ", kTodo, false}, {"[ ] ", kTodo, false},
	{"- ", kBullet, false}, {"* ", kBullet, false}, {"1. ", kNumber, false}, {"> ", kQuote, false},
}

// typedShortcut turns a text block whose text starts with a shortcut into
// its kind, the shortcut gone, and says how many runes went (0: none).
func (b *block) typedShortcut() int {
	if b.kind != kText {
		return 0
	}
	for _, s := range shortcuts {
		if rest, ok := strings.CutPrefix(b.text, s.prefix); ok {
			b.kind, b.done, b.text = s.kind, s.done, rest
			return len(s.prefix)
		}
	}
	return 0
}

// next is the block Enter starts after b: a list goes on, the rest is
// text.
func (b block) next() block {
	if b.isList() {
		return block{kind: b.kind, indent: b.indent}
	}
	return block{}
}
