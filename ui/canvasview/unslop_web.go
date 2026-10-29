package canvasview

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/enowx/enowx/proto"
)

// A redesign's reads, shown in the page as they come: a line sweeps down
// the design, and each element a finding is about lights up as it passes,
// in its severity's color, with what is off. The next read draws them
// again, so what a pass fixed is no longer marked. While a pass is under
// way, each element the agent changes is tagged with the pass's name.

type scanItem struct {
	Sel   string `json:"sel"`
	Sev   string `json:"sev"`
	Label string `json:"label"`
}

// scanScript draws a read's marks over the page: the items, at most six
// elements each, below a sweeping line.
const scanScript = `(function(items){
var old=document.getElementById('__enx_scan');if(old)old.remove();
var de=document.documentElement,H=Math.max(de.scrollHeight,innerHeight),W=de.scrollWidth;
var root=document.createElement('div');root.id='__enx_scan';
root.style.cssText='position:absolute;left:0;top:0;width:'+W+'px;height:'+H+'px;pointer-events:none;z-index:2147483646;overflow:hidden';
document.body.appendChild(root);
var line=document.createElement('div');
line.style.cssText='position:absolute;left:0;right:0;top:0;height:64px;transform:translateY(-64px);background:linear-gradient(180deg,rgba(99,102,241,0),rgba(99,102,241,.16));border-bottom:2px solid #818cf8;box-shadow:0 6px 18px rgba(99,102,241,.45);transition:transform 1.6s cubic-bezier(.4,0,.2,1)';
root.appendChild(line);
requestAnimationFrame(function(){requestAnimationFrame(function(){line.style.transform='translateY('+H+'px)'})});
setTimeout(function(){line.style.opacity=0;line.style.transition='opacity .3s'},1650);
var colors={high:'#ef4444',medium:'#f59e0b',low:'#60a5fa'},n=0;
items.forEach(function(it){var els=[];try{els=document.querySelectorAll(it.sel)}catch(e){}
for(var i=0;i<els.length&&i<6;i++){var r=els[i].getBoundingClientRect();if(r.width<2||r.height<2)continue;
(function(r,it){var top=r.top+scrollY,left=r.left+scrollX,c=colors[it.sev]||'#818cf8';
setTimeout(function(){var d=document.createElement('div');
d.style.cssText='position:absolute;left:'+left+'px;top:'+top+'px;width:'+r.width+'px;height:'+r.height+'px;border:1.5px solid '+c+';background:'+c+'1f;border-radius:4px;opacity:0;transform:scale(.97);transition:opacity .3s,transform .3s';
var t=document.createElement('span');t.textContent=it.label;
t.style.cssText='position:absolute;left:-1.5px;bottom:100%;margin-bottom:2px;font:600 10px/16px system-ui,sans-serif;padding:0 6px;border-radius:4px;color:#fff;white-space:nowrap;background:'+c;
d.appendChild(t);root.appendChild(d);requestAnimationFrame(function(){d.style.opacity=1;d.style.transform='none'})},1600*Math.min(top/H,1))})(r,it);n++}});
return n})`

// passTagScript tags element id, just changed, with the pass under way.
const passTagScript = `(function(id,label){var e=document.getElementById(id);if(!e||!label)return;
var r=e.getBoundingClientRect(),t=document.createElement('div');t.textContent=label;
t.style.cssText='position:absolute;left:'+(r.left+scrollX)+'px;top:'+Math.max(0,r.top+scrollY-20)+'px;font:600 10px/18px system-ui,sans-serif;padding:0 7px;border-radius:9px;color:#fff;background:#6366f1;box-shadow:0 2px 8px rgba(0,0,0,.25);pointer-events:none;z-index:2147483647;opacity:0;transform:translateY(4px);transition:opacity .25s,transform .25s';
document.body.appendChild(t);requestAnimationFrame(function(){t.style.opacity=1;t.style.transform='none'});
setTimeout(function(){t.style.opacity=0;setTimeout(function(){t.remove()},300)},1800)})`

// scan draws run's latest read over the page, once per read of the design
// shown.
func (w *Web) scan(run *proto.CanvasUnslopRun, frame string) {
	if run == nil || run.Frame != frame || w.page == nil {
		w.scanned = ""
		return
	}
	key := run.Frame + "/" + strconv.Itoa(run.Scans) + "/" + w.shown
	if w.scanned == key {
		return
	}
	w.scanned = key
	var items []scanItem
	for _, f := range run.Findings {
		for _, sel := range f.Targets {
			items = append(items, scanItem{Sel: sel, Sev: f.Severity, Label: f.Title})
		}
	}
	data, _ := json.Marshal(items)
	page := w.page
	go func() {
		// A page reloaded just now takes a moment to hold the design.
		time.Sleep(350 * time.Millisecond)
		_, _ = page.Eval(scanScript + "(" + string(data) + ")")
	}()
}

// tagPass tags element id with the pass under way, if any.
func (w *Web) tagPass(id string) {
	if w.page == nil || w.pass == "" || id == "" {
		return
	}
	page, code := w.page, passTagScript+"("+js(id)+","+js(w.pass)+")"
	go func() { _, _ = page.Eval(code) }()
}
