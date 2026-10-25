// Package noteblocks reads the structure of a note's Markdown: its
// headings and the layout blocks written as directives (tabs, toggles,
// columns, callouts, cards, embeds), so a page can be shown as more than
// a long text, and read or changed a section at a time.
//
// A layout block opens with ":::name args" on a line of its own and closes
// with ":::"; blocks nest. Inside tabs, columns and cards, "::tab Title",
// "::col" and "::card Title" start each part. "::toc", "::progress",
// "::spacer", "::pages" and "::embed URL" stand alone. Inline, "[[Page]]",
// "[[#Section]]" and "[[Page#Section]]" link, and "{{text}}" is a badge.
// Outside enowX it all reads as plain Markdown.
package noteblocks

import (
	"regexp"
	"strings"
)

// Node is a run of plain Markdown lines (Name "") or a directive: a
// container with its parts, or a leaf standing alone.
type Node struct {
	Name  string   // the directive's name: tabs, toggle, callout, embed…
	Args  string   // what follows its name: a toggle's title, an embed's URL
	Leaf  bool     // "::name", without parts or a close
	Lines []string // a run's lines
	Parts []Part   // a container's parts; toggles and callouts have one
	// Start and End are the node's lines in the source, End excluded.
	Start, End int
}

// Part is a part of a container: a tab, a column, a card, or the whole
// content of a toggle or a callout.
type Part struct {
	Title string
	Nodes []Node
	// Line is the "::tab" line starting it, or the container's opening
	// line for a part without one; its content runs from Line+1 to End.
	Line, End int
}

// Separators are the "::name" lines that start a container's parts.
var Separators = map[string]bool{"tab": true, "col": true, "column": true, "card": true}

// Leaves are the "::name" lines that stand alone.
var Leaves = map[string]bool{"toc": true, "progress": true, "spacer": true, "embed": true, "pages": true}

var (
	openRe = regexp.MustCompile(`^:::\s*([a-z][a-z0-9-]*)\s*(.*)$`)
	leafRe = regexp.MustCompile(`^::([a-z][a-z0-9-]*)\s*(.*)$`)
)

// Lines splits Markdown into its lines, without a last empty one.
func Lines(md string) []string {
	md = strings.TrimSuffix(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	if md == "" {
		return nil
	}
	return strings.Split(md, "\n")
}

// IsOpen says line opens a container, and which.
func IsOpen(line string) (name, args string, ok bool) {
	m := openRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return "", "", false
	}
	return m[1], strings.TrimSpace(m[2]), true
}

// IsClose says line closes a container.
func IsClose(line string) bool { return strings.TrimSpace(line) == ":::" }

// IsLeaf says line is a directive standing alone, or a part's start
// (separator), and which.
func IsLeaf(line string) (name, args string, separator, ok bool) {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, ":::") {
		return "", "", false, false
	}
	m := leafRe.FindStringSubmatch(t)
	if m == nil {
		return "", "", false, false
	}
	switch {
	case Separators[m[1]]:
		return m[1], strings.TrimSpace(m[2]), true, true
	case Leaves[m[1]]:
		return m[1], strings.TrimSpace(m[2]), false, true
	}
	return "", "", false, false
}

// isFence says line opens or closes a fenced code block, in which nothing
// is a directive or a heading.
func isFence(line string) bool { return strings.HasPrefix(strings.TrimSpace(line), "```") }

// Parse reads Markdown into its nodes.
func Parse(md string) []Node {
	lines := Lines(md)
	nodes, _ := parseNodes(lines, 0, false)
	return nodes
}

// parseNodes reads nodes from line i until the end, or, in a container,
// its close or its next part's start, which it doesn't take.
func parseNodes(lines []string, i int, inside bool) ([]Node, int) {
	var out []Node
	var run *Node
	flush := func() {
		if run != nil {
			out = append(out, *run)
			run = nil
		}
	}
	add := func(at int) {
		if run == nil {
			run = &Node{Start: at}
		}
		run.Lines = append(run.Lines, lines[at])
		run.End = at + 1
	}
	for i < len(lines) {
		line := lines[i]
		if isFence(line) {
			add(i)
			for i++; i < len(lines); i++ {
				add(i)
				if isFence(lines[i]) {
					break
				}
			}
			i++
			continue
		}
		if inside && IsClose(line) {
			break
		}
		if name, args, ok := IsOpen(line); ok {
			flush()
			n := Node{Name: name, Args: args, Start: i}
			i = parseParts(lines, i, &n)
			out = append(out, n)
			continue
		}
		if name, args, sep, ok := IsLeaf(line); ok {
			if sep {
				if inside {
					break
				}
				add(i) // a part's start outside a container is text
				i++
				continue
			}
			flush()
			out = append(out, Node{Name: name, Args: args, Leaf: true, Start: i, End: i + 1})
			i++
			continue
		}
		add(i)
		i++
	}
	flush()
	return out, i
}

// blank says nodes are blank lines only.
func blank(nodes []Node) bool {
	for _, n := range nodes {
		if n.Name != "" {
			return false
		}
		for _, l := range n.Lines {
			if strings.TrimSpace(l) != "" {
				return false
			}
		}
	}
	return true
}

// parseParts reads container n opening at line open: its parts up to its
// close, and says the line after it.
func parseParts(lines []string, open int, n *Node) int {
	i := open + 1
	part := Part{Line: open}
	for {
		part.Nodes, i = parseNodes(lines, i, true)
		part.End = i
		// Blank lines before a container's first part are no part.
		if !blank(part.Nodes) || part.Line != open || i >= len(lines) || IsClose(lines[i]) {
			n.Parts = append(n.Parts, part)
		}
		if i >= len(lines) {
			n.End = i // unclosed: to the end
			return i
		}
		if IsClose(lines[i]) {
			n.End = i + 1
			return i + 1
		}
		// A part's start.
		_, title, _, _ := IsLeaf(lines[i])
		part = Part{Title: title, Line: i}
		i++
	}
}
