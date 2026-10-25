package canvas

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
)

// PDF is img as a one-page PDF of its size (a pixel a point), for a
// frame of shapes the daemon draws itself.
func PDF(img image.Image) ([]byte, error) {
	var pic bytes.Buffer
	if err := jpeg.Encode(&pic, img, &jpeg.Options{Quality: 92}); err != nil {
		return nil, err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	content := fmt.Sprintf("q %d 0 0 %d 0 0 cm /Im0 Do Q", w, h)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>", w, h),
		"", // the image, a stream
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n", i+1)
		if i == 3 {
			fmt.Fprintf(&b, "<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n", w, h, pic.Len())
			b.Write(pic.Bytes())
			b.WriteString("\nendstream")
		} else {
			b.WriteString(obj)
		}
		b.WriteString("\nendobj\n")
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes(), nil
}
