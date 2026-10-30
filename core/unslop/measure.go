package unslop

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// MeasureScript reads a design as rendered: each element with an id, its
// box and the properties a pass changes, and the id of the nearest
// element with one around it. Its value is JSON, for Changes.
const MeasureScript = `(function(){var out={};var els=document.querySelectorAll('[id]');for(var i=0;i<els.length&&i<600;i++){var e=els[i];var r=e.getBoundingClientRect();if(r.width==0&&r.height==0)continue;var s=getComputedStyle(e);var p=e.parentElement&&e.parentElement.closest('[id]');out[e.id]={p:p?p.id:'',x:Math.round(r.left+scrollX),y:Math.round(r.top+scrollY),w:Math.round(r.width),h:Math.round(r.height),font:s.fontFamily.split(',')[0].replace(/["']/g,'').trim(),size:s.fontSize,weight:s.fontWeight,lh:s.lineHeight,ls:s.letterSpacing,color:s.color,bg:s.backgroundColor,pad:s.padding,gap:s.gap,radius:s.borderRadius,shadow:s.boxShadow=='none'?'none':'a shadow',border:s.borderStyle=='none'?'none':s.borderWidth+' '+s.borderColor}}return JSON.stringify(out)})()`

// measured is an element as MeasureScript read it.
type measured struct {
	P                                                       string
	X, Y, W, H                                              int
	Font, Size, Weight, LH, LS, Color, Bg, Pad, Gap, Radius string
	Shadow, Border                                          string
}

// props are an element's properties as a step changes them, named.
func (m measured) props() [][2]string {
	return [][2]string{{"font", m.Font}, {"size", m.Size}, {"weight", m.Weight}, {"line height", m.LH}, {"tracking", m.LS},
		{"color", m.Color}, {"background", m.Bg}, {"padding", m.Pad}, {"gap", m.Gap}, {"radius", m.Radius},
		{"shadow", m.Shadow}, {"border", m.Border}}
}

// maxChanges bounds what a step lists: the elements most changed.
const maxChanges = 40

// Changes is what changed between two reads of MeasureScript, before and
// after: each element's properties that changed, those it shares with the
// element around it left to that one (inherited, not its own), the most
// changed first; elements added and removed. Nil when a read doesn't read.
func Changes(before, after string) []proto.UnslopChange {
	var b, a map[string]measured
	if json.Unmarshal([]byte(before), &b) != nil || json.Unmarshal([]byte(after), &a) != nil {
		return nil
	}
	diff := func(id string) []proto.UnslopProp {
		was, is := b[id], a[id]
		var out []proto.UnslopProp
		for i, p := range is.props() {
			if from := was.props()[i][1]; from != p[1] {
				out = append(out, proto.UnslopProp{Name: p[0], From: from, To: p[1]})
			}
		}
		return out
	}
	var out []proto.UnslopChange
	for id, m := range a {
		if _, ok := b[id]; !ok {
			out = append(out, proto.UnslopChange{Element: id, Added: true, X: m.X, Y: m.Y, W: m.W, H: m.H})
			continue
		}
		props := diff(id)
		_, here := a[m.P]
		if _, had := b[m.P]; m.P != "" && here && had {
			inherited := diff(m.P)
			props = slices.DeleteFunc(props, func(p proto.UnslopProp) bool { return slices.Contains(inherited, p) })
		}
		if len(props) > 0 {
			out = append(out, proto.UnslopChange{Element: id, Props: props, X: m.X, Y: m.Y, W: m.W, H: m.H})
		}
	}
	for id, m := range b {
		if _, ok := a[id]; !ok {
			out = append(out, proto.UnslopChange{Element: id, Removed: true, X: m.X, Y: m.Y, W: m.W, H: m.H})
		}
	}
	slices.SortFunc(out, func(x, y proto.UnslopChange) int {
		if d := len(y.Props) - len(x.Props); d != 0 {
			return d
		}
		if x.Y != y.Y {
			return x.Y - y.Y
		}
		return strings.Compare(x.Element, y.Element)
	})
	if len(out) > maxChanges {
		out = out[:maxChanges]
	}
	return out
}
