package notesview

import "strings"

func (v *View) pasteImage(o Options, p *page) bool {
	if o.PasteImage == nil || p.mode == modeRead {
		return false
	}
	if p.uploading {
		return true
	}
	p.uploading = true
	if !o.PasteImage(p.id, v.imageDone(o, p)) {
		p.uploading = false
		return false
	}
	return true
}

// imageDone adds the image to the latest body, including edits made during upload.
func (v *View) imageDone(o Options, p *page) func(string, error) {
	return func(ref string, err error) {
		p.uploading = false
		if err != nil {
			v.err = err.Error()
			return
		}
		if ref == "" {
			return
		}
		p.rememberEdit()
		body := p.ed.markdown()
		if p.mode == modeSource {
			body = p.src
		}
		body = strings.TrimRight(body, "\n") + "\n\n![Image](" + ref + ")\n"
		p.ed = newEditor(body)
		p.ed.changed = true
		p.src = body
		v.err = ""
		if v.page != p {
			v.write(o, p) // switching pages during upload still keeps the image
		}
	}
}
