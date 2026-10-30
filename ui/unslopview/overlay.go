package unslopview

import (
	"encoding/json"
	"time"

	"github.com/enowx/enowx/proto"
)

// A redesign's reads, shown in the page as they come (a canvas's design,
// the Browser's page): a line sweeps down, and each element a finding is
// about lights up as it passes, in its severity's color, with what is
// off. The next read draws them again, so what a pass fixed is no longer
// marked. Regions an agent each redesigns are outlined in their colors,
// and an element an agent changes is tagged with its pass.

type scanItem struct {
	Sel   string `json:"sel"`
	Sev   string `json:"sev"`
	Label string `json:"label"`
}

// enxStyle are the marks' animations, in CSS, so they run without
// requestAnimationFrame, which a page out of view doesn't call.
const enxStyle = `if(!document.getElementById('__enx_style')){var st=document.createElement('style');st.id='__enx_style';
st.textContent='@keyframes enxSweep{from{transform:translateY(-64px)}to{transform:translateY(var(--enx-h))}}@keyframes enxFade{to{opacity:0}}@keyframes enxIn{from{opacity:0;transform:scale(.97)}to{opacity:1;transform:none}}@keyframes enxTag{0%{opacity:0;transform:translateY(4px)}12%{opacity:1;transform:none}85%{opacity:1}100%{opacity:0}}';
document.head.appendChild(st)}`

// ScanScript draws a read's marks over the page: the items, at most six
// elements each, below a sweeping line, each lighting up as it passes.
const ScanScript = `(function(items){` + enxStyle + `
var old=document.getElementById('__enx_scan');if(old)old.remove();
var de=document.documentElement,H=Math.max(de.scrollHeight,innerHeight),W=de.scrollWidth;
var root=document.createElement('div');root.id='__enx_scan';
root.style.cssText='position:absolute;left:0;top:0;width:'+W+'px;height:'+H+'px;pointer-events:none;z-index:2147483646;overflow:hidden';
root.style.setProperty('--enx-h',H+'px');
document.body.appendChild(root);
var line=document.createElement('div');
line.style.cssText='position:absolute;left:0;right:0;top:0;height:64px;background:linear-gradient(180deg,rgba(99,102,241,0),rgba(99,102,241,.16));border-bottom:2px solid #818cf8;box-shadow:0 6px 18px rgba(99,102,241,.45);animation:enxSweep 1.6s cubic-bezier(.4,0,.2,1) both,enxFade .3s 1.6s both';
root.appendChild(line);
var colors={high:'#ef4444',medium:'#f59e0b',low:'#60a5fa'},n=0;
items.forEach(function(it){var els=[];try{els=document.querySelectorAll(it.sel)}catch(e){}
for(var i=0;i<els.length&&i<6;i++){var r=els[i].getBoundingClientRect();if(r.width<2||r.height<2)continue;
var top=r.top+scrollY,left=r.left+scrollX,c=colors[it.sev]||'#818cf8',d=document.createElement('div');
d.style.cssText='position:absolute;left:'+left+'px;top:'+top+'px;width:'+r.width+'px;height:'+r.height+'px;border:1.5px solid '+c+';background:'+c+'1f;border-radius:4px;animation:enxIn .3s '+(1.6*Math.min(top/H,1)).toFixed(2)+'s both';
var t=document.createElement('span');t.textContent=it.label;
t.style.cssText='position:absolute;left:-1.5px;bottom:100%;margin-bottom:2px;font:600 10px/16px system-ui,sans-serif;padding:0 6px;border-radius:4px;color:#fff;white-space:nowrap;background:'+c;
d.appendChild(t);root.appendChild(d);n++}});
return n})`

// RegionColors tell a fanned-out unslop's regions apart, by order.
var RegionColors = []string{"#f472b6", "#22d3ee", "#a3e635", "#fb923c", "#c084fc", "#facc15"}

// RegionsScript outlines each region, dashed in its color, with its
// agent and pass on a tag; a region done goes solid, then fades.
const RegionsScript = `(function(rs){
var old=document.getElementById('__enx_regions');if(old)old.remove();if(!rs.length)return;
var root=document.createElement('div');root.id='__enx_regions';
root.style.cssText='position:absolute;left:0;top:0;width:0;height:0;pointer-events:none;z-index:2147483645';
document.body.appendChild(root);
rs.forEach(function(r){var e=document.getElementById(r.id);if(!e)return;var b=e.getBoundingClientRect();
var d=document.createElement('div');
d.style.cssText='position:absolute;left:'+(b.left+scrollX-3)+'px;top:'+(b.top+scrollY-3)+'px;width:'+(b.width+6)+'px;height:'+(b.height+6)+'px;border:2px '+(r.done?'solid':'dashed')+' '+r.color+';border-radius:6px;opacity:'+(r.done?.45:1)+';transition:opacity .4s';
var t=document.createElement('span');t.textContent=r.label;
t.style.cssText='position:absolute;right:-2px;top:-2px;transform:translateY(-100%);font:600 10px/18px system-ui,sans-serif;padding:0 7px;border-radius:9px 9px 0 9px;color:#111;white-space:nowrap;background:'+r.color;
d.appendChild(t);root.appendChild(d)})})`

// PassTagScript tags element id, just changed, with label in color.
const PassTagScript = `(function(id,label,color){` + enxStyle + `var e=document.getElementById(id);if(!e||!label)return;
var r=e.getBoundingClientRect(),t=document.createElement('div');t.textContent=label;
t.style.cssText='position:absolute;left:'+(r.left+scrollX)+'px;top:'+Math.max(0,r.top+scrollY-20)+'px;font:600 10px/18px system-ui,sans-serif;padding:0 7px;border-radius:9px;color:'+(color?'#111':'#fff')+';background:'+(color||'#6366f1')+';box-shadow:0 2px 8px rgba(0,0,0,.25);pointer-events:none;z-index:2147483647;animation:enxTag 2.1s both';
document.body.appendChild(t);setTimeout(function(){t.remove()},2200)})`

// ScanItems are run's findings' elements, as ScanScript takes them.
func ScanItems(run *proto.CanvasUnslopRun) string {
	var items []scanItem
	for _, f := range run.Findings {
		for _, sel := range f.Targets {
			items = append(items, scanItem{Sel: sel, Sev: f.Severity, Label: f.Title})
		}
	}
	data, _ := json.Marshal(items)
	return string(data)
}

// Tag is how an agent's changes are tagged: its label and color ("" the
// accent's).
type Tag struct{ Label, Color string }

type regionMark struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Color string `json:"color"`
	Done  bool   `json:"done"`
}

// RegionMarks are run's regions as RegionsScript takes them, and the tag
// of each region agent's session.
func RegionMarks(run *proto.CanvasUnslopRun, agentName func(string) string) (string, map[string]Tag) {
	tags := map[string]Tag{}
	var marks []regionMark
	if run != nil && !run.Done {
		for i, r := range run.Regions {
			color := RegionColors[i%len(RegionColors)]
			label := "#" + r.ID
			if agentName != nil {
				if name := agentName(r.Session); name != "" {
					label = name + " · " + label
				}
			}
			switch {
			case r.Done:
				label += " · done"
			case r.Pass != "":
				label += " · " + r.Pass
			}
			marks = append(marks, regionMark{ID: r.ID, Label: label, Color: color, Done: r.Done})
			tags[r.Session] = Tag{Label: label, Color: color}
		}
	}
	data, _ := json.Marshal(marks)
	return string(data), tags
}

// JS quotes s as a JavaScript string.
func JS(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// scanReady wraps a page script to run once the page is loaded: a page
// still loading (the design just shown, the dev server reloading it)
// would drop what it draws.
func scanReady(code string) string {
	return `(function(){var go=function(){` + code + `};if(document.readyState==='complete'&&document.body){go()}else{addEventListener('load',go,{once:true})}return true})()`
}

// scanStill says the page still holds a read's marks.
const scanStill = `!!document.getElementById('__enx_scan')`

// DrawScan draws a read's marks (ScanItems) in a page through eval,
// once it is loaded, and again when a reload took them, a few times at
// most. It waits, so it runs off the main thread.
func DrawScan(eval func(code string) (any, error), items string) {
	code := scanReady(ScanScript + "(" + items + ")")
	for try := 0; try < 4; try++ {
		time.Sleep(300 * time.Millisecond)
		if _, err := eval(code); err != nil {
			continue
		}
		// The sweep takes 1.6 s; a page reloaded meanwhile lost it.
		time.Sleep(1200 * time.Millisecond)
		if still, err := eval(scanStill); err == nil && still == true {
			return
		}
	}
}
