package notesio

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // images notes show
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// Notes as a PDF, written here: A4 pages of the standard fonts, the
// notes' images in it, each note starting a page.

const (
	pageW, pageH = 595.0, 842.0
	margin       = 56.0
	textW        = pageW - 2*margin
)

// pdfDoc lays out pages and writes the file.
type pdfDoc struct {
	pages  []*bytes.Buffer
	page   *bytes.Buffer
	y      float64  // where the next line's top goes, from the page's top
	images [][]byte // the image objects, written whole
}

func (d *pdfDoc) newPage() {
	d.page = &bytes.Buffer{}
	d.pages = append(d.pages, d.page)
	d.y = margin
}

// room starts a page unless h more points fit on this one.
func (d *pdfDoc) room(h float64) {
	if d.page == nil || d.y+h > pageH-margin {
		d.newPage()
	}
}

func escapePDF(b []byte) string {
	var s strings.Builder
	for _, c := range b {
		switch c {
		case '(', ')', '\\':
			s.WriteByte('\\')
			s.WriteByte(c)
		default:
			s.WriteByte(c)
		}
	}
	return s.String()
}

// text writes one line at x, the top of the line at the current y.
func (d *pdfDoc) text(line []byte, font pdfFont, size, x float64, gray float64) {
	base := pageH - d.y - size*0.8
	fmt.Fprintf(d.page, "%.3f g BT /F%d %.1f Tf %.2f %.2f Td (%s) Tj ET\n", gray, font+1, size, x, base, escapePDF(line))
}

// paragraph writes text wrapped in width w from x, and moves past it.
func (d *pdfDoc) paragraph(text string, font pdfFont, size, x, w, gray, after float64) {
	lead := size * 1.4
	for _, line := range wrap(encode(text), font, size, w) {
		d.room(lead)
		d.text(line, font, size, x, gray)
		d.y += lead
	}
	d.y += after
}

func (d *pdfDoc) rule(x1, x2, gray float64) {
	y := pageH - d.y
	fmt.Fprintf(d.page, "%.3f G 0.6 w %.2f %.2f m %.2f %.2f l S\n", gray, x1, y, x2, y)
}

// image places an image's data, scaled to fit, and says whether it could.
func (d *pdfDoc) image(data []byte) bool {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return false
	}
	var obj bytes.Buffer
	switch {
	case format == "jpeg" && (cfg.ColorModel == color.YCbCrModel || cfg.ColorModel == color.GrayModel):
		cs := "/DeviceRGB"
		if cfg.ColorModel == color.GrayModel {
			cs = "/DeviceGray"
		}
		fmt.Fprintf(&obj, "<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n",
			cfg.Width, cfg.Height, cs, len(data))
		obj.Write(data)
	default:
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return false
		}
		b := img.Bounds()
		var raw bytes.Buffer
		zw := zlib.NewWriter(&raw)
		row := make([]byte, 0, b.Dx()*3)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			row = row[:0]
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := img.At(x, y).RGBA()
				// Over white, as the page is.
				blend := func(v uint32) byte { return byte((v + (0xffff - a)) >> 8) }
				row = append(row, blend(r), blend(g), blend(bl))
			}
			zw.Write(row)
		}
		zw.Close()
		fmt.Fprintf(&obj, "<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n",
			b.Dx(), b.Dy(), raw.Len())
		obj.Write(raw.Bytes())
	}
	obj.WriteString("\nendstream")
	d.images = append(d.images, obj.Bytes())
	w, h := float64(cfg.Width)*0.75, float64(cfg.Height)*0.75
	if w > textW {
		w, h = textW, h*textW/w
	}
	if maxH := pageH - 2*margin - 20; h > maxH {
		w, h = w*maxH/h, maxH
	}
	d.room(h + 8)
	fmt.Fprintf(d.page, "q %.2f 0 0 %.2f %.2f %.2f cm /Im%d Do Q\n", w, h, margin, pageH-d.y-h, len(d.images))
	d.y += h + 10
	return true
}

func writePDF(s *notes.Store, list []proto.Note, w io.Writer) error {
	d := &pdfDoc{}
	for _, n := range list {
		d.newPage()
		d.paragraph(iconTitle(n), fontBold, 22, margin, textW, 0, 2)
		d.paragraph("Updated "+n.Updated.Format("2 Jan 2006"), fontRegular, 9, margin, textW, 0.45, 12)
		for _, bl := range parseMarkdown(n.Body) {
			d.block(s, bl)
		}
	}
	return d.write(w)
}

func (d *pdfDoc) block(s *notes.Store, bl mdBlock) {
	text := plainText(bl.text)
	indent := margin + float64(bl.level)*16
	switch bl.kind {
	case mdHeading:
		size := [...]float64{0, 17, 14, 12}[bl.level]
		d.y += 6
		d.paragraph(text, fontBold, size, margin, textW, 0, 4)
	case mdBullet, mdNumber:
		mark := "•"
		if bl.kind == mdNumber {
			mark = fmt.Sprintf("%d.", bl.number)
		}
		d.room(15)
		d.text(encode(mark), fontRegular, 10.5, indent, 0)
		d.paragraph(text, fontRegular, 10.5, indent+16, textW-(indent-margin)-16, 0, 2)
	case mdTodo:
		d.room(15)
		y := pageH - d.y - 11
		fmt.Fprintf(d.page, "0.4 G 0.8 w %.2f %.2f 9 9 re S\n", indent, y)
		if bl.done {
			fmt.Fprintf(d.page, "0 G 1.2 w %.2f %.2f m %.2f %.2f l %.2f %.2f l S\n", indent+2, y+4.5, indent+4, y+2, indent+7.5, y+7.5)
		}
		gray := 0.0
		if bl.done {
			gray = 0.45
		}
		d.paragraph(text, fontRegular, 10.5, indent+16, textW-(indent-margin)-16, gray, 2)
	case mdQuote:
		top := d.y
		d.paragraph(text, fontItalic, 10.5, margin+12, textW-12, 0.3, 0)
		fmt.Fprintf(d.page, "0.75 G 2 w %.2f %.2f m %.2f %.2f l S\n", margin+2, pageH-top, margin+2, pageH-d.y+4)
		d.y += 6
	case mdCode:
		for _, line := range strings.Split(bl.text, "\n") {
			d.paragraph(line, fontMono, 9, margin+8, textW-16, 0.15, 0)
		}
		d.y += 6
	case mdDivider:
		d.room(14)
		d.y += 6
		d.rule(margin, pageW-margin, 0.75)
		d.y += 8
	case mdImage:
		data, err := s.Asset(bl.src)
		if err != nil || !d.image(data) {
			d.paragraph("["+text+"]", fontItalic, 10, margin, textW, 0.45, 4)
		}
	case mdTable:
		d.table(bl.rows)
	default:
		d.paragraph(text, fontRegular, 10.5, margin, textW, 0, 6)
	}
}

// table writes rows as a grid of equal columns, the first row bold.
func (d *pdfDoc) table(rows [][]string) {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	if cols == 0 {
		return
	}
	cw := textW / float64(cols)
	const size, lead = 9.0, 12.0
	for i, r := range rows {
		font := fontRegular
		if i == 0 {
			font = fontBold
		}
		cells := make([][][]byte, cols)
		h := 1
		for c := range cols {
			text := ""
			if c < len(r) {
				text = plainText(r[c])
			}
			cells[c] = wrap(encode(text), font, size, cw-8)
			h = max(h, len(cells[c]))
		}
		rowH := float64(h)*lead + 6
		d.room(rowH)
		top := d.y
		for c, lines := range cells {
			for j, line := range lines {
				d.y = top + 3 + float64(j)*lead
				d.text(line, font, size, margin+float64(c)*cw+4, 0)
			}
		}
		fmt.Fprintf(d.page, "0.75 G 0.5 w %.2f %.2f %.2f %.2f re S\n", margin, pageH-top-rowH, textW, rowH)
		for c := 1; c < cols; c++ {
			x := margin + float64(c)*cw
			fmt.Fprintf(d.page, "%.2f %.2f m %.2f %.2f l S\n", x, pageH-top, x, pageH-top-rowH)
		}
		d.y = top + rowH
	}
	d.y += 8
}

// write writes the file: catalog, pages, fonts, images, then their
// offsets.
func (d *pdfDoc) write(w io.Writer) error {
	var out bytes.Buffer
	var offsets []int
	obj := func(body string) int {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
		return len(offsets)
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	pagesAt := len(offsets) // 2, written once the pages' objects are known
	offsets = append(offsets, 0)
	var fonts []string
	for i, name := range fontNames {
		n := obj(fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>", name))
		fonts = append(fonts, fmt.Sprintf("/F%d %d 0 R", i+1, n))
	}
	var images []string
	for i, img := range d.images {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n", len(offsets))
		out.Write(img)
		out.WriteString("\nendobj\n")
		images = append(images, fmt.Sprintf("/Im%d %d 0 R", i+1, len(offsets)))
	}
	resources := "<< /Font << " + strings.Join(fonts, " ") + " >> /XObject << " + strings.Join(images, " ") + " >> >>"
	var kids []string
	for _, p := range d.pages {
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		zw.Write(p.Bytes())
		zw.Close()
		offsets = append(offsets, out.Len())
		content := len(offsets)
		fmt.Fprintf(&out, "%d 0 obj\n<< /Length %d /Filter /FlateDecode >>\nstream\n", content, z.Len())
		out.Write(z.Bytes())
		out.WriteString("\nendstream\nendobj\n")
		page := obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources %s /Contents %d 0 R >>", pageW, pageH, resources, content))
		kids = append(kids, fmt.Sprintf("%d 0 R", page))
	}
	offsets[pagesAt] = out.Len()
	fmt.Fprintf(&out, "2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", strings.Join(kids, " "), len(kids))
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	_, err := w.Write(out.Bytes())
	return err
}
