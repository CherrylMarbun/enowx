package notesio

import "golang.org/x/text/encoding/charmap"

// The PDF's fonts are the standard ones every reader has (Helvetica, its
// bold and oblique, Courier), in Windows-1252: their widths, for wrapping
// lines, and the bytes of text in them.

// helvetica and helveticaBold are the widths of characters 32 to 126, in
// thousandths of the font's size.
var helvetica = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}

var helveticaBold = [95]int{
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
	975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
	333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
	611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584,
}

type pdfFont int

const (
	fontRegular pdfFont = iota
	fontBold
	fontItalic
	fontMono
)

var fontNames = [...]string{"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Courier"}

// encode is s in Windows-1252, what it has no character for left out.
func encode(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch r {
		case '\t':
			out = append(out, "    "...)
			continue
		case '☐', '☑', '☒':
			continue
		}
		if b, ok := charmap.Windows1252.EncodeRune(r); ok {
			out = append(out, b)
		}
	}
	return out
}

// width is the width of text in font at size, in points.
func width(text []byte, font pdfFont, size float64) float64 {
	if font == fontMono {
		return float64(len(text)) * 0.6 * size
	}
	table := &helvetica
	if font == fontBold {
		table = &helveticaBold
	}
	w := 0
	for _, c := range text {
		switch {
		case c >= 32 && c <= 126:
			w += table[c-32]
		default:
			w += 556
		}
	}
	return float64(w) * size / 1000
}

// wrap breaks text into lines no wider than max.
func wrap(text []byte, font pdfFont, size, max float64) [][]byte {
	var lines [][]byte
	var line []byte
	for _, word := range splitWords(text) {
		try := word
		if len(line) > 0 {
			try = append(append(append([]byte{}, line...), ' '), word...)
		}
		if width(try, font, size) <= max || len(line) == 0 {
			line = try
			for width(line, font, size) > max && len(line) > 1 {
				// A word longer than a line: cut it.
				cut := len(line) - 1
				for cut > 1 && width(line[:cut], font, size) > max {
					cut--
				}
				lines = append(lines, line[:cut])
				line = append([]byte{}, line[cut:]...)
			}
			continue
		}
		lines = append(lines, line)
		line = append([]byte{}, word...)
	}
	if len(line) > 0 || len(lines) == 0 {
		lines = append(lines, line)
	}
	return lines
}

func splitWords(text []byte) [][]byte {
	var out [][]byte
	start := -1
	for i, c := range text {
		if c == ' ' {
			if start >= 0 {
				out = append(out, text[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, text[start:])
	}
	return out
}
