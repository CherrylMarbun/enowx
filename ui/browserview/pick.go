package browserview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// Design mode: the user points at an element of the page and clicks it;
// the agent gets what it is (its selector, HTML and main styles) with a
// picture of it, to change it as asked.

// Picked is an element the user picked in a page.
type Picked struct {
	URL      string     `json:"url"`
	Selector string     `json:"selector"`
	HTML     string     `json:"html"`
	Styles   string     `json:"styles"`
	Text     string     `json:"text"`
	Rect     [4]float64 `json:"rect"`     // x, y, width, height in the viewport, CSS pixels
	Viewport float64    `json:"viewport"` // the viewport's width, CSS pixels
}

// Picker is a page that lets the user pick one of its elements: Pick
// returns once they click one, or nil when they press Escape or StopPick
// ends it.
type Picker interface {
	Pick(ctx context.Context) (*Picked, error)
	StopPick()
}

// StopPickScript ends picking (PickScript) as Escape does.
const StopPickScript = `window.__enowxPick && window.__enowxPick(null)`

// PickScript is the JavaScript that lets the user pick an element: it
// outlines the one under the pointer, takes the click for itself, and
// answers what was clicked, or null on Escape. Pages run it with their
// Eval, which awaits it.
const PickScript = `return await new Promise((resolve) => {
  const box = document.createElement('div');
  box.style.cssText = 'position:fixed;pointer-events:none;z-index:2147483647;outline:2px solid #e8a33d;background:rgba(232,163,61,.12);border-radius:2px;transition:all 60ms';
  document.documentElement.appendChild(box);
  let current = null;
  const place = (el) => {
    const r = el.getBoundingClientRect();
    Object.assign(box.style, {left: r.left + 'px', top: r.top + 'px', width: r.width + 'px', height: r.height + 'px'});
  };
  const move = (e) => { if (e.target !== box) { current = e.target; place(current); } };
  const selector = (el) => {
    const parts = [];
    for (let n = el; n && n.nodeType === 1 && parts.length < 6; n = n.parentElement) {
      if (n.id) { parts.unshift('#' + CSS.escape(n.id)); break; }
      let part = n.localName;
      const classes = [...n.classList].slice(0, 2).map((c) => '.' + CSS.escape(c)).join('');
      part += classes;
      const parent = n.parentElement;
      if (parent && !classes) {
        const same = [...parent.children].filter((c) => c.localName === n.localName);
        if (same.length > 1) part += ':nth-of-type(' + (same.indexOf(n) + 1) + ')';
      }
      parts.unshift(part);
    }
    return parts.join(' > ');
  };
  const keys = ['display', 'position', 'width', 'height', 'margin', 'padding', 'color', 'background-color',
    'font-family', 'font-size', 'font-weight', 'line-height', 'border', 'border-radius', 'gap',
    'flex-direction', 'justify-content', 'align-items', 'grid-template-columns', 'box-shadow'];
  const describe = (el) => {
    const r = el.getBoundingClientRect();
    const cs = getComputedStyle(el);
    return {
      url: location.href, selector: selector(el), html: el.outerHTML.slice(0, 4000),
      styles: keys.map((k) => k + ': ' + cs.getPropertyValue(k)).filter((l) => !/: (none|normal|auto|0px)?$/.test(l)).join(';\n'),
      text: (el.innerText || '').trim().slice(0, 300), rect: [r.left, r.top, r.width, r.height], viewport: innerWidth,
    };
  };
  const stop = (e) => { e.preventDefault(); e.stopPropagation(); };
  const done = (value) => {
    delete window.__enowxPick;
    removeEventListener('mousemove', move, true);
    removeEventListener('click', click, true);
    removeEventListener('mousedown', stop, true);
    removeEventListener('keydown', key, true);
    box.remove();
    resolve(value);
  };
  const click = (e) => { stop(e); done(describe(current || e.target)); };
  const key = (e) => { if (e.key === 'Escape') { stop(e); done(null); } };
  window.__enowxPick = done;
  addEventListener('mousemove', move, true);
  addEventListener('mousedown', stop, true);
  addEventListener('click', click, true);
  addEventListener('keydown', key, true);
})`

// Crop cuts the picked element out of a capture of the page's viewport,
// with a little room around it, as a PNG.
func (p Picked) Crop(capture []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(capture))
	if err != nil {
		return nil, fmt.Errorf("browserview: read the capture: %w", err)
	}
	b := img.Bounds()
	scale := 1.0
	if p.Viewport > 0 {
		scale = float64(b.Dx()) / p.Viewport
	}
	const room = 8
	r := image.Rect(
		int((p.Rect[0]-room)*scale), int((p.Rect[1]-room)*scale),
		int((p.Rect[0]+p.Rect[2]+room)*scale), int((p.Rect[1]+p.Rect[3]+room)*scale),
	).Add(b.Min).Intersect(b)
	if r.Empty() {
		return capture, nil // off screen: the whole view says more than nothing
	}
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return capture, nil
	}
	var out bytes.Buffer
	if err := png.Encode(&out, sub.SubImage(r)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Describe is what the agent reads about the element, before its picture.
func (p Picked) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "This element of %s (%s):\n\n```html\n%s\n```\n", p.URL, p.Selector, p.HTML)
	if p.Styles != "" {
		fmt.Fprintf(&b, "\nIts styles now:\n\n```css\n%s\n```\n", p.Styles)
	}
	b.WriteString("\nA picture of it: ")
	return b.String()
}
