package unslop

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/enowx/enowx/proto"
)

// A redesign played back, as one page: each step's design over the last,
// revealed by a wave from the top left, the elements it changed boxed in
// red first, their sizes counting from the old value to the new, the
// step's name in a chip. Seekable (__story.at(ms)), so a video is captured
// frame by frame; and a slider to compare any two steps. The app shows it
// in the canvas's Replay and renders it to export.

// StoryStepMs is how long a step plays.
const StoryStepMs = 2200

// StoryGroup merges story's steps into at most max, the passes of each
// kept in order and their changes together, the documents at each
// group's end: a video of 3 to 5 steps reads better than one of 12.
func StoryGroup(st proto.UnslopStory, max int) proto.UnslopStory {
	n := len(st.Steps)
	if max <= 0 || n <= max {
		return st
	}
	out := st
	out.Docs, out.Steps = []string{st.Docs[0]}, nil
	for g := range max {
		from, to := g*n/max, (g+1)*n/max
		var step proto.UnslopStep
		for _, s := range st.Steps[from:to] {
			step.Passes = append(step.Passes, s.Passes...)
			step.Changes = append(step.Changes, s.Changes...)
			step.Version, step.Measured = s.Version, s.Measured
		}
		out.Steps = append(out.Steps, step)
		out.Docs = append(out.Docs, st.Docs[to])
	}
	return out
}

// StoryName is a step's name in the chip: its pass, or its first and last.
func StoryName(s proto.UnslopStep) string {
	switch len(s.Passes) {
	case 0:
		return ""
	case 1:
		return s.Passes[0]
	}
	return s.Passes[0] + " … " + s.Passes[len(s.Passes)-1]
}

// Crop is the part of a design a square video shows: where the step with
// the most changes changed most, widened to a square of at least a third
// of the design's width. Zero for the whole.
func Crop(st proto.UnslopStory) (x, y, w int) {
	best := -1
	for i, s := range st.Steps {
		if best < 0 || len(s.Changes) > len(st.Steps[best].Changes) {
			best = i
		}
	}
	if best < 0 || len(st.Steps[best].Changes) == 0 || st.Width == 0 {
		return 0, 0, 0
	}
	x0, y0, x1, y1 := st.Width, 1<<30, 0, 0
	for _, c := range st.Steps[best].Changes {
		x0, y0 = min(x0, c.X), min(y0, c.Y)
		x1, y1 = max(x1, c.X+c.W), max(y1, c.Y+c.H)
	}
	w = max(x1-x0, y1-y0, st.Width/3)
	w = min(w, st.Width)
	x = min(max(0, (x0+x1)/2-w/2), st.Width-w)
	y = max(0, (y0+y1)/2-w/2)
	return x, y, w
}

// StoryHTML is the page playing story back, credit the line its last
// frame says ("" for the scores).
func StoryHTML(st proto.UnslopStory, credit string) string {
	type step struct {
		Name    string               `json:"name"`
		Changes []proto.UnslopChange `json:"changes"`
	}
	steps := []step{}
	for _, s := range st.Steps {
		steps = append(steps, step{Name: StoryName(s), Changes: s.Changes})
	}
	if credit == "" && st.From > 0 && st.Score > 0 {
		credit = fmt.Sprintf("Unslopped in %d steps · %d → %d", len(st.Steps), st.From, st.Score)
	}
	data, _ := json.Marshal(map[string]any{"w": st.Width, "h": max(st.Height, 1), "steps": steps, "credit": credit, "stepMs": StoryStepMs})
	var frames strings.Builder
	for i, d := range st.Docs {
		fmt.Fprintf(&frames, `<iframe id="d%d" srcdoc="%s" scrolling="no"></iframe>`, i, html.EscapeString(d))
	}
	return strings.Replace(strings.Replace(storyPage, "{{FRAMES}}", frames.String(), 1), "{{DATA}}", string(data), 1)
}

const storyPage = `<!doctype html><html><head><meta charset="utf-8"><style>
html,body{margin:0;height:100%;background:#0d0d0f;overflow:hidden;font:500 13px -apple-system,system-ui,sans-serif}
#view{position:absolute;inset:0;overflow:hidden}
#stage{position:absolute;left:0;top:0;transform-origin:0 0}
#stage iframe{position:absolute;left:0;top:0;border:0;background:#fff;visibility:hidden}
#marks{position:absolute;left:0;top:0;pointer-events:none;z-index:5}
.box{position:absolute;border:2px solid #ff3b30;border-radius:3px;box-sizing:border-box}
.tag{position:absolute;left:-2px;top:-22px;background:#ff3b30;color:#fff;font:600 11px ui-monospace,Menlo,monospace;padding:2px 6px;border-radius:3px;white-space:nowrap}
.box.fixed{border-color:rgba(255,255,255,.85)}.box.fixed .tag{background:#fff;color:#111}
#chip{position:absolute;left:20px;bottom:20px;z-index:6;background:rgba(15,15,17,.86);color:#fff;padding:8px 13px;border-radius:999px;font:600 14px -apple-system,system-ui,sans-serif;letter-spacing:.01em;box-shadow:0 4px 18px rgba(0,0,0,.35)}
#chip small{opacity:.6;font-weight:500;margin-right:8px}
#credit{position:absolute;left:50%;bottom:22px;transform:translateX(-50%);z-index:6;background:rgba(15,15,17,.88);color:#fff;padding:10px 18px;border-radius:999px;font:600 15px -apple-system,system-ui,sans-serif;opacity:0;white-space:nowrap;box-shadow:0 6px 24px rgba(0,0,0,.35)}
#slider{position:absolute;top:0;bottom:0;width:2px;z-index:7;background:#fff;display:none;cursor:ew-resize;box-shadow:0 0 0 1px rgba(0,0,0,.3)}
</style></head><body><div id="view"><div id="stage">{{FRAMES}}<div id="marks"></div></div><div id="chip"></div><div id="credit"></div><div id="slider"></div></div>
<script>(function(){
var S={{DATA}},W=S.w,H=S.h,N=S.steps.length,stage=document.getElementById('stage'),marks=document.getElementById('marks'),chip=document.getElementById('chip'),credit=document.getElementById('credit'),slider=document.getElementById('slider');
var fr=[];for(var i=0;i<=N;i++){var f=document.getElementById('d'+i);f.style.width=W+'px';f.style.height=H+'px';fr.push(f)}
marks.style.width=W+'px';marks.style.height=H+'px';stage.style.width=W+'px';stage.style.height=H+'px';
var crop=null,cmp=null;
function fit(){var vw=innerWidth,vh=innerHeight,s,x=0,y=0;if(crop){s=vw/crop.w;x=-crop.x*s;y=H*s-crop.y*s<vh?(vh-H*s)/2:-crop.y*s;if(H*s<vh)y=(vh-H*s)/2}else{s=Math.min(vw/W,vh/H);x=(vw-W*s)/2;y=Math.max(0,(vh-H*s)/2)}stage.style.transform='translate('+x+'px,'+y+'px) scale('+s+')'}
addEventListener('resize',fit);fit();
function ease(t){return t<.5?2*t*t:1-Math.pow(-2*t+2,2)/2}
function num(v){var m=/^(-?[\d.]+)(px|%|em|rem)?$/.exec(String(v).trim());return m?{n:parseFloat(m[1]),u:m[2]||''}:null}
function tween(a,b,t){var x=num(a),y=num(b);if(x&&y&&x.u==y.u){var v=x.n+(y.n-x.n)*t;return(Math.abs(y.n)>=10||Math.abs(x.n)>=10?Math.round(v):Math.round(v*10)/10)+y.u}return t<.5?a:b}
function label(c,t){if(c.added)return 'added';if(c.removed)return 'removed';var p=c.props.slice(0,2).map(function(p){return p.name+' '+(t>=1?p.to:tween(p.from,p.to,t))});return '#'+c.element+'  '+p.join(' · ')}
function show(i,clip){fr.forEach(function(f,j){f.style.visibility=j==i||j==clip?'visible':'hidden';f.style.clipPath='none';f.style.zIndex=j==clip?2:1})}
function boxes(step,t,fixed){marks.innerHTML='';if(!step)return;step.changes.slice(0,8).forEach(function(c){if(c.removed)return;var d=document.createElement('div');d.className='box'+(fixed?' fixed':'');d.style.left=c.x+'px';d.style.top=c.y+'px';d.style.width=Math.max(c.w,8)+'px';d.style.height=Math.max(c.h,8)+'px';var g=document.createElement('div');g.className='tag';if(c.y<26)g.style.top='2px',g.style.left='2px';g.textContent=label(c,t);d.appendChild(g);marks.appendChild(d)})}
function setChip(k){var s=S.steps[k-1];chip.style.display=s?'block':'none';if(s)chip.innerHTML='<small>'+k+'/'+N+'</small>'+s.name.replace(/</g,'&lt;')}
window.__story={steps:N,stepMs:S.stepMs,duration:(N*S.stepMs)+1600,
load:function(k){cmp=null;slider.style.display='none';credit.style.opacity=0;k=Math.max(0,Math.min(N,k));show(k);setChip(k);boxes(S.steps[k-1],1,true)},
at:function(ms){cmp=null;slider.style.display='none';var k=Math.floor(ms/S.stepMs);if(k>=N){show(N);chip.style.display='none';boxes(null);credit.textContent=S.credit||'';credit.style.opacity=Math.min(1,(ms-N*S.stepMs)/500);return}credit.style.opacity=0;var t=(ms-k*S.stepMs)/S.stepMs,step=S.steps[k];setChip(k+1);
if(t<.28){show(k);boxes(step,0,false);return}
var w=ease(Math.min(1,(t-.28)/.5));show(k,k+1);var r=w*(W+H)*1.05;fr[k+1].style.clipPath='polygon(0 0,'+r+'px 0,0 '+r+'px)';
boxes(step,w,w>=1)},
compare:function(a,b,x){cmp={a:a,b:b};chip.style.display='none';credit.style.opacity=0;boxes(null);show(a,b);var p=x==null?.5:x;fr[b].style.clipPath='inset(0 0 0 '+(p*W)+'px)';var st=stage.getBoundingClientRect();slider.style.display='block';slider.style.left=(st.left+p*st.width)+'px'},
crop:function(x,y,w){crop=w?{x:x,y:y,w:w}:null;fit()},
play:function(from){var self=this,t0=performance.now()-(from||0);cancelAnimationFrame(this._r);var f=function(){var ms=performance.now()-t0;self.at(Math.min(ms,self.duration));self.now=ms;if(ms<self.duration)self._r=requestAnimationFrame(f);else self.now=-1};f()},
stop:function(){cancelAnimationFrame(this._r);this.now=-1},now:-1};
var drag=false;slider.addEventListener('mousedown',function(){drag=true});addEventListener('mouseup',function(){drag=false});
addEventListener('mousemove',function(e){if(!drag||!cmp)return;var st=stage.getBoundingClientRect();__story.compare(cmp.a,cmp.b,Math.max(0,Math.min(1,(e.clientX-st.left)/st.width)))});
__story.load(N);
})()</script></body></html>`
