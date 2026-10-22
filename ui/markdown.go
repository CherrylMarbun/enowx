package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// markdown draws the Markdown agents answer in: paragraphs, headings,
// lists, fenced code, `code` and **bold**. Anything else shows as text.
func markdown(c *ui.Context, src string) {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			switch {
			case trimmed == "":
			case strings.HasPrefix(trimmed, "```"):
				var code []string
				for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
					code = append(code, lines[i])
				}
				ui.Text(c.Key(i), strings.Join(code, "\n")).Font(monoFont).FontSize(12).Padding(10, 12).Radius(8).
					Background(colBackground).Border(1, colBorder).FillWidth().Selectable()
			case strings.HasPrefix(trimmed, "#"):
				level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
				size := float32(16 - 1.5*float32(min(level, 4)))
				inline(c.Key(i), strings.TrimSpace(trimmed[level:])).Bold().FontSize(size)
			case isBullet(trimmed):
				var items []string
				for ; i < len(lines) && isBullet(strings.TrimSpace(lines[i])); i++ {
					items = append(items, strings.TrimSpace(lines[i]))
				}
				i--
				ui.Column(c.Key(i)).FillWidth().Gap(4).Children(func() {
					for j, item := range items {
						mark, text := bulletParts(item)
						ui.Row(c.Key(j)).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
							ui.Text(c, mark).TextColor(colMuted).Shrink(0)
							inline(c, text).Grow(1).Shrink(1)
						})
					}
				})
			default:
				// A paragraph runs until a blank line or another block.
				para := []string{trimmed}
				for i+1 < len(lines) {
					next := strings.TrimSpace(lines[i+1])
					if next == "" || strings.HasPrefix(next, "```") || strings.HasPrefix(next, "#") || isBullet(next) {
						break
					}
					para = append(para, next)
					i++
				}
				inline(c.Key(i), strings.Join(para, " ")).FillWidth()
			}
		}
	})
}

// inline draws a line with its `code` and **bold** spans.
func inline(c *ui.Context, s string) ui.Element {
	return ui.RichText(c).Selectable().TextColor(colText).Children(func() {
		for s != "" {
			i := strings.IndexAny(s, "`*")
			if i < 0 {
				ui.Text(c, s)
				return
			}
			if s[i] == '*' && !strings.HasPrefix(s[i:], "**") {
				ui.Text(c, s[:i+1])
				s = s[i+1:]
				continue
			}
			delim := s[i : i+1]
			if delim == "*" {
				delim = "**"
			}
			end := strings.Index(s[i+len(delim):], delim)
			if end < 0 {
				ui.Text(c, s)
				return
			}
			if i > 0 {
				ui.Text(c, s[:i])
			}
			inner := s[i+len(delim) : i+len(delim)+end]
			if delim == "`" {
				ui.Text(c, inner).Font(monoFont).Background(colRaised).TextColor(ui.Hex("#E6C07B"))
			} else {
				ui.Text(c, inner).Bold().TextColor(colTextStrong)
			}
			s = s[i+len(delim)+end+len(delim):]
		}
	})
}

func isBullet(s string) bool {
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") {
		return true
	}
	n := len(s) - len(strings.TrimLeft(s, "0123456789"))
	return n > 0 && strings.HasPrefix(s[n:], ". ")
}

func bulletParts(s string) (string, string) {
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") {
		return "•", s[2:]
	}
	n := len(s) - len(strings.TrimLeft(s, "0123456789"))
	return s[:n+1], s[n+2:]
}
