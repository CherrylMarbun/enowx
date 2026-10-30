package unslopview

import (
	"encoding/json"

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

// ScanScript draws a read's marks over the page: the items, at most six
// elements each, below a sweeping line.
const ScanScript = `(function(items){
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
const PassTagScript = `(function(id,label,color){var e=document.getElementById(id);if(!e||!label)return;
var r=e.getBoundingClientRect(),t=document.createElement('div');t.textContent=label;
t.style.cssText='position:absolute;left:'+(r.left+scrollX)+'px;top:'+Math.max(0,r.top+scrollY-20)+'px;font:600 10px/18px system-ui,sans-serif;padding:0 7px;border-radius:9px;color:'+(color?'#111':'#fff')+';background:'+(color||'#6366f1')+';box-shadow:0 2px 8px rgba(0,0,0,.25);pointer-events:none;z-index:2147483647;opacity:0;transform:translateY(4px);transition:opacity .25s,transform .25s';
document.body.appendChild(t);requestAnimationFrame(function(){t.style.opacity=1;t.style.transform='none'});
setTimeout(function(){t.style.opacity=0;setTimeout(function(){t.remove()},300)},1800)})`

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
