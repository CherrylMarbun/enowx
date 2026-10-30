package unslop

// LintScript measures a page as rendered, evaluated in it: what its
// computed styles and boxes show that its CSS alone can't (the contrast
// text really gets over the backgrounds under it, body text's size,
// leading and measure, entrances that never ran, kickers above headings,
// stripes, cards in cards, emoji icons, how many left edges the text
// starts from). It returns JSON for LintFindings: per rule the elements
// met (a selector and a label each) with what was measured, the
// thresholds left to LintFindings. The app's own marks (ids starting
// __enx) are hidden while it measures and never read; at most 8000
// elements are.
const LintScript = `(function(){
var ours=[].slice.call(document.querySelectorAll('[id^="__enx"]')),was=ours.map(function(e){return e.style.display});
ours.forEach(function(e){e.style.display='none'});
try{
var D=document,W=D.documentElement.clientWidth||innerWidth,H=innerHeight;
var out={vw:W,vh:H,contrast:[],gray:[],body:[],stuck:[],kickers:[],numbered:[],gaps:[],stripes:[],gradtext:[],nested:[],emoji:[],anchors:[],boxed:{texts:0,inBox:0,boxes:0,ex:[],at:[]}};
function add(a,x,cap){if(a.length<(cap||80))a.push(x)}
var CS=new Map();function st(e){var s=CS.get(e);if(!s){s=getComputedStyle(e);CS.set(e,s)}return s}
var cv=D.createElement('canvas');cv.width=cv.height=1;var cx=cv.getContext('2d',{willReadFrequently:true}),cc={};
function col(s){if(!s||s==='none')return [0,0,0,0];if(cc[s])return cc[s];
var c,m=/^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)\s*(?:[,\/]\s*([\d.]+)(%?))?\s*\)$/.exec(s);
if(m)c=[+m[1],+m[2],+m[3],m[4]===undefined?1:m[5]?m[4]/100:+m[4]];
else if(s==='transparent')c=[0,0,0,0];
else{cx.clearRect(0,0,1,1);cx.fillStyle='rgba(0,0,0,0)';cx.fillStyle=s;cx.fillRect(0,0,1,1);var d=cx.getImageData(0,0,1,1).data;c=[d[0],d[1],d[2],d[3]/255]}
return cc[s]=c}
function lin(v){v/=255;return v<=0.04045?v/12.92:Math.pow((v+0.055)/1.055,2.4)}
function lum(c){return 0.2126*lin(c[0])+0.7152*lin(c[1])+0.0722*lin(c[2])}
function chroma(c){var r=lin(c[0]),g=lin(c[1]),b=lin(c[2]);
var l=Math.cbrt(0.4122214708*r+0.5363325363*g+0.0514459929*b),m=Math.cbrt(0.2119034982*r+0.6806995451*g+0.1073969566*b),s=Math.cbrt(0.0883024619*r+0.2817188376*g+0.6299787005*b);
var A=1.9779984951*l-2.4285922050*m+0.4505937099*s,B=0.0259040371*l+0.7827717662*m-0.8086757660*s;return Math.sqrt(A*A+B*B)}
function over(t,b){var a=t[3];return [t[0]*a+b[0]*(1-a),t[1]*a+b[1]*(1-a),t[2]*a+b[2]*(1-a),1]}
function ratio(a,b){var x=lum(a),y=lum(b);return (Math.max(x,y)+0.05)/(Math.min(x,y)+0.05)}
function vivid(c){return c[3]>=0.4&&chroma(c)>=0.15}
var OP=new Map();function op(e){if(!e||e.nodeType!==1)return 1;var v=OP.get(e);if(v===undefined){var o=parseFloat(st(e).opacity);if(isNaN(o))o=1;v=o*op(e.parentElement);OP.set(e,v)}return v}
function bg(e){var layers=[];for(var a=e;a&&a.nodeType===1;a=a.parentElement){var s=st(a);if(s.backgroundImage&&s.backgroundImage!=='none')return null;var c=col(s.backgroundColor);if(c[3]>0){layers.push(c);if(c[3]>=0.999)break}}
var r=[255,255,255,1];for(var i=layers.length-1;i>=0;i--)r=over(layers[i],r);return r}
function own(e){var t='';for(var c=e.firstChild;c;c=c.nextSibling)if(c.nodeType===3)t+=c.textContent;return t.replace(/\s+/g,' ').trim()}
function txt(e){return (e.textContent||'').slice(0,300).replace(/\s+/g,' ').trim()}
var RG=D.createRange();
function trect(e){var r=null;for(var c=e.firstChild;c;c=c.nextSibling){if(c.nodeType!==3||!c.textContent.trim())continue;RG.selectNodeContents(c);var b=RG.getBoundingClientRect();if(!b.width&&!b.height)continue;
r=r?{left:Math.min(r.left,b.left),top:Math.min(r.top,b.top),right:Math.max(r.right,b.right),bottom:Math.max(r.bottom,b.bottom)}:{left:b.left,top:b.top,right:b.right,bottom:b.bottom}}
if(r){r.width=r.right-r.left;r.height=r.bottom-r.top}return r}
function esc(s){return CSS.escape(s)}
function sel(e){var parts=[],n=e;for(var d=0;n&&n.nodeType===1&&n!==D.body&&n!==D.documentElement&&d<5;n=n.parentElement,d++){
if(n.id){parts.unshift('#'+esc(n.id));return parts.join(' > ')}
var s=n.tagName.toLowerCase();for(var i=0;i<n.classList.length&&i<2;i++)s+='.'+esc(n.classList[i]);
var p=n.parentElement;if(p){var same=0,idx=0;for(var k=p.firstElementChild;k;k=k.nextElementSibling)if(k.tagName===n.tagName){same++;if(k===n)idx=same}if(same>1)s+=':nth-of-type('+idx+')'}
parts.unshift(s)}
if(n===D.body)parts.unshift('body');return parts.join(' > ')}
function lab(e,t){var s=e.tagName.toLowerCase();if(e.id)s+='#'+e.id;if(e.classList.length)s+='.'+e.classList[0];t=(t||'').replace(/\s+/g,' ').trim();if(t){if(t.length>32)t=t.slice(0,32)+'…';s+=' “'+t+'”'}return s}
function lh(s,fs){var v=s.lineHeight==='normal'?1.2*fs:parseFloat(s.lineHeight);return v>0?v:1.2*fs}
function shown(e){var r=e.getBoundingClientRect();return r.width>0&&r.height>0&&st(e).display!=='none'}
var SKIP={SCRIPT:1,STYLE:1,NOSCRIPT:1,TEMPLATE:1};
function skipped(n){return SKIP[n.tagName.toUpperCase()]||n.namespaceURI==='http://www.w3.org/2000/svg'||n.getAttribute('aria-hidden')==='true'||(n.id&&n.id.indexOf('__enx')===0)}
function prevShown(n){for(var k=n.previousElementSibling;k;k=k.previousElementSibling)if(!skipped(k)&&shown(k))return k;return null}
function nextShown(n){for(var k=n.nextElementSibling;k;k=k.nextElementSibling)if(!skipped(k)&&shown(k))return k;return null}
var els=[],tw=D.createTreeWalker(D.body,NodeFilter.SHOW_ELEMENT,{acceptNode:function(n){return skipped(n)?NodeFilter.FILTER_REJECT:NodeFilter.FILTER_ACCEPT}});
while(els.length<8000&&tw.nextNode())els.push(tw.currentNode);
var media=[];D.querySelectorAll('img,video,canvas,picture,iframe').forEach(function(m){if(m.closest('[id^="__enx"]'))return;var r=m.getBoundingClientRect();if(r.width*r.height>2000&&st(m).visibility!=='hidden'&&op(m)>0.05)media.push(r)});
function onMedia(t){var a=t.width*t.height;for(var i=0;i<media.length;i++){var m=media[i],w=Math.min(t.right,m.right)-Math.max(t.left,m.left),h=Math.min(t.bottom,m.bottom)-Math.max(t.top,m.top);if(w>0&&h>0&&w*h>=0.3*a)return true}return false}
var plates=[],faint=[];
function painted(s){return col(s.backgroundColor)[3]>0.5||s.backgroundImage!=='none'}
function plateUnder(e,t){var x=(t.left+t.right)/2,y=(t.top+t.bottom)/2,i,u;
if(x>=0&&y>=0&&x<=W&&y<=H){var under=D.elementsFromPoint(x,y);for(i=0;i<under.length;i++){u=under[i];if(u===e||u.contains(e)||e.contains(u)||(u.id&&u.id.indexOf('__enx')===0))continue;if(painted(st(u)))return true}return false}
for(i=0;i<plates.length;i++){u=plates[i];if(u.contains(e)||e.contains(u))continue;var r=u.getBoundingClientRect();if(x>=r.left&&x<=r.right&&y>=r.top&&y<=r.bottom)return true}return false}
function stuck(e,r,t){if(r.top>=H||r.bottom<=0||r.left>=W||r.right<=0)return;
if(!(e.tagName==='IMG'&&e.complete&&e.naturalWidth>0)&&!(r.width>=40&&r.height>=16&&t.length>=4))return;
if(st(e).visibility!=='visible'||e.closest('dialog,[role=tooltip],[role=menu],[hidden],details:not([open])'))return;
for(var a=e;a&&a!==D.documentElement;a=a.parentElement){var as=st(a);if(parseFloat(as.opacity)<0.5&&as.position!=='absolute'&&as.position!=='fixed'&&as.pointerEvents!=='none'){add(out.stuck,{s:sel(a),d:lab(e,t||e.getAttribute('alt')||'')});return}}}
var CJK=/[\u{3040}-\u{30ff}\u{3400}-\u{9fff}\u{ac00}-\u{d7af}\u{f900}-\u{faff}]/gu;
var EMO=/^(?:\p{Regional_Indicator}{2}|\p{Extended_Pictographic}(?:\u{fe0f}|\p{Emoji_Modifier})*(?:\u{200d}\p{Extended_Pictographic}(?:\u{fe0f}|\p{Emoji_Modifier})*)*)$/u;
function emojiIcon(t){if(!EMO.test(t))return false;var cp=t.codePointAt(0);return cp>=0x1F000||cp>=0x2600&&cp<=0x27BF&&'✓✔✗✘➜➔➝➞'.indexOf(t[0])<0||cp===0x2B50||cp===0x2B55}
var texts=[],heads=[],grads=[];
function text(e,s,t){var tr=trect(e);if(!tr||tr.width<2||tr.height<2)return;var o=op(e);
if(o<0.05){stuck(e,e.getBoundingClientRect(),t);return}
if(s.visibility!=='visible')return;
var fs=parseFloat(s.fontSize)||16,fw=parseInt(s.fontWeight,10)||400;texts.push({e:e,r:tr,t:t,s:s});
if(!e.closest('input,textarea,select,option,[disabled],[aria-disabled="true"]')){var b=bg(e),fc=col(s.color);
if(b&&fc[3]>0&&!onMedia(tr)){var f=over([fc[0],fc[1],fc[2],fc[3]*o],b),k=ratio(f,b),large=fs>=24||fs>=18.6&&fw>=700;
if(k<4.5){var it={s:sel(e),d:lab(e,t),ratio:+k.toFixed(2),large:large,plate:false};add(out.contrast,it,200);if(k<1.5)faint.push([it,e,tr])}
else{var bc=chroma(b),tc=chroma(f);if(bc>=0.2&&tc<0.1)add(out.gray,{s:sel(e),d:lab(e,t),bg:+bc.toFixed(3),text:+tc.toFixed(3),lum:+lum(f).toFixed(3)})}}}
var cj=(t.match(CJK)||[]).length,isC=cj>=30,l=lh(s,fs);
if((t.length>=60||isC)&&tr.height>=1.8*l&&!e.closest('code,pre,table,nav,button,label,figcaption,kbd'))
add(out.body,{s:sel(e),d:lab(e,t),size:+fs.toFixed(2),leading:+(l/fs).toFixed(3),cpl:Math.round(isC?tr.width/fs:tr.width/(0.5*fs)),cjk:isC},200);
if(emojiIcon(t)&&!e.closest('p,blockquote,[role=listbox],[role=menu],[popover],[aria-pressed]')){var host=e.closest('button')||e.parentElement;
if(!(host&&/^\S+\s*\d[\d.,kK]*$/.test(txt(host))))add(out.emoji,{s:sel(e),d:lab(e,t)})}}
function side(x,a,b,c,style,color){return x>=2&&x>=2*a&&x>=2*b&&x>=2*c&&(style==='solid'||style==='double')&&vivid(col(color))}
function stripe(e,s,r){if(e.tagName==='BLOCKQUOTE'||r.height<16||r.height>=0.8*H)return;
if(e.closest('nav,[role=navigation],[role=tablist],[aria-current]:not([aria-current="false"]),[aria-selected="true"],.active,.current,.selected'))return;
var L=parseFloat(s.borderLeftWidth)||0,R=parseFloat(s.borderRightWidth)||0,T=parseFloat(s.borderTopWidth)||0,B=parseFloat(s.borderBottomWidth)||0;
if(r.width>L+R+24){if(side(L,T,R,B,s.borderLeftStyle,s.borderLeftColor)){add(out.stripes,{s:sel(e),d:lab(e,txt(e)),how:'border-left '+L+'px'});return}
if(side(R,T,L,B,s.borderRightStyle,s.borderRightColor)){add(out.stripes,{s:sel(e),d:lab(e,txt(e)),how:'border-right '+R+'px'});return}}
if(r.height<24||r.width<=60)return;
['::before','::after'].some(function(ps){var p=getComputedStyle(e,ps);if(!p||p.content==='none'||p.content==='normal'||p.position!=='absolute'||p.display==='none')return false;
var pw=parseFloat(p.width);if(!(pw>=2&&pw<=8))return false;
var hh=/px$/.test(p.height)?parseFloat(p.height):/%$/.test(p.height)?parseFloat(p.height)/100*r.height:r.height-(parseFloat(p.top)||0)-(parseFloat(p.bottom)||0);if(hh<0.6*r.height)return false;
if(!vivid(col(p.backgroundColor))&&!/gradient/.test(p.backgroundImage))return false;
var lf=parseFloat(p.left),rt=parseFloat(p.right),at=Math.abs(lf)<=2?'left':Math.abs(rt)<=2?'right':'';if(!at)return false;
add(out.stripes,{s:sel(e),d:lab(e,txt(e)),how:ps+' at the '+at});return true})}
function rad(s){return Math.max(parseFloat(s.borderTopLeftRadius)||0,parseFloat(s.borderTopRightRadius)||0,parseFloat(s.borderBottomLeftRadius)||0,parseFloat(s.borderBottomRightRadius)||0)}
function edge(s,k){return (parseFloat(s['border'+k+'Width'])||0)>=1&&s['border'+k+'Style']!=='none'&&col(s['border'+k+'Color'])[3]>0.05}
function framed(s){return edge(s,'Top')&&edge(s,'Right')&&edge(s,'Bottom')&&edge(s,'Left')}
function shadow(s){return !!s.boxShadow&&s.boxShadow!=='none'}
function fill(s){return col(s.backgroundColor)[3]>0.05}
var NOCARD={BUTTON:1,A:1,INPUT:1,IMG:1,VIDEO:1,PICTURE:1,CANVAS:1,IFRAME:1,LABEL:1,SUMMARY:1,PRE:1,CODE:1,TEXTAREA:1,SELECT:1};
function plate(a,s,r){return rad(s)>=6&&(framed(s)||shadow(s)||fill(s))&&r.width*r.height<0.6*W*H}
function nested(e,s,r){if(r.width<120||r.height<56||rad(s)<6||NOCARD[e.tagName.toUpperCase()]||!(e.children.length||own(e)))return;
if(!(framed(s)||shadow(s)&&fill(s)))return;
if(r.height<=100&&e.querySelector('button')&&e.closest('form'))return;
for(var a=e.parentElement,i=0;a&&a!==D.body&&i<8;a=a.parentElement,i++){var as=st(a);
if(a.matches('dialog,[role=dialog],[role=listbox],[role=menu],[role=tooltip],[popover]')||as.position==='fixed')return;
var ar=a.getBoundingClientRect();
if(as.position==='absolute'&&a.parentElement){var pr=a.parentElement.getBoundingClientRect();if(ar.left<pr.left-1||ar.right>pr.right+1||ar.top<pr.top-1||ar.bottom>pr.bottom+1)return}
if(plate(a,as,ar)){add(out.nested,{s:sel(e),d:lab(e,txt(e)),outer:lab(a,'')});return}}}
els.forEach(function(e){try{var s=st(e);if(s.display==='none')return;var r=e.getBoundingClientRect();if(!r.width||!r.height)return;
var tag=e.tagName.toUpperCase();if(/^H[1-6]$/.test(tag))heads.push(e);
if(tag==='IMG'&&op(e)<0.05)stuck(e,r,'');
var t=own(e);if(t)text(e,s,t);
if(op(e)<0.05||s.visibility!=='visible')return;
stripe(e,s,r);nested(e,s,r);
if((s.position==='absolute'||s.position==='fixed')&&painted(s))plates.push(e);
var clip=s.backgroundClip+' '+s.getPropertyValue('-webkit-background-clip');
if(/text/.test(clip)&&/gradient/.test(s.backgroundImage)&&txt(e)&&!grads.some(function(g){return g.contains(e)})){grads.push(e);add(out.gradtext,{s:sel(e),d:lab(e,txt(e))})}
}catch(x){}});
faint.forEach(function(f){f[0].plate=plateUnder(f[1],f[2])});
var flagged=new Set(),NUM=/^(?:\d{1,2}|[IVXLC]{1,6})(?:\s*[.\/·•—–\-:|)]\s*.*)?$/;
function kicker(k,h,hr,hfs){
if(k.matches('nav,ol,[role=navigation]')||k.closest('nav,[role=navigation]')||k.querySelector('nav,[role=navigation]')||/breadcrumb/i.test((k.getAttribute('class')||'')+' '+(k.getAttribute('aria-label')||'')))return;
var MEDIA='a,button,input,select,textarea,img,video,picture,canvas,iframe';if(k.matches(MEDIA)||k.querySelector(MEDIA))return;
var t=txt(k);if(!t||t.length>48)return;
var kr=k.getBoundingClientRect();if(kr.bottom>hr.top+4||hr.top-kr.bottom>Math.max(32,1.2*hfs)||kr.left>=hr.right||kr.right<=hr.left)return;
var tn=k;while(tn.children.length===1&&!own(tn))tn=tn.firstElementChild;
var ts=st(tn),fs=parseFloat(ts.fontSize)||16;if(fs>16||fs>0.6*hfs)return;
RG.selectNodeContents(k);var rs=RG.getClientRects(),tops=[],l=lh(ts,fs);
for(var i=0;i<rs.length;i++){if(!rs[i].width)continue;var y=rs[i].top+rs[i].height/2;if(!tops.some(function(v){return Math.abs(v-y)<l*0.6}))tops.push(y)}
if(tops.length>1)return;
var item={s:sel(k),d:lab(k,t),h:lab(h,txt(h))};
if(NUM.test(t)){flagged.add(h);add(out.numbered,item);return}
var how=[];
if(ts.textTransform==='uppercase'||st(k).textTransform==='uppercase'||/[A-Z][^A-Z]*[A-Z][^A-Z]*[A-Z]/.test(t)&&!/[a-z]/.test(t))how.push('uppercase');
var ls=ts.letterSpacing==='normal'?0:parseFloat(ts.letterSpacing)||0;if(ls>=0.05*fs)how.push('tracked');
if(/mono|courier|consolas|menlo|monaco/i.test(ts.fontFamily))how.push('monospace');
[k,tn].some(function(x){var xs=st(x),xr=x.getBoundingClientRect();if(xr.height>0&&rad(xs)>=xr.height/3&&(col(xs.backgroundColor)[3]>0.1||(parseFloat(xs.borderTopWidth)||0)>=1&&col(xs.borderTopColor)[3]>0.1)){how.push('pill');return true}return false});
if(how.length){flagged.add(h);item.how=how.join(' ');add(out.kickers,item)}}
heads.forEach(function(h){try{var hs=st(h),hfs=parseFloat(hs.fontSize)||16;if(h.tagName.toUpperCase()>'H3'||hfs<20||op(h)<0.05||hs.visibility!=='visible')return;
var hr=h.getBoundingClientRect(),p=h,k=null;for(var i=0;i<4&&p&&p!==D.body;i++){k=prevShown(p);if(k)break;p=p.parentElement}
if(k)kicker(k,h,hr,hfs)}catch(x){}});
heads.forEach(function(h){try{if(flagged.has(h)||op(h)<0.05)return;var par=h.parentElement,ps=st(par);
if(/grid/.test(ps.display)||/flex/.test(ps.display)&&ps.flexDirection.indexOf('row')===0)return;
var pv=prevShown(h),nx=nextShown(h);if(!pv||!nx)return;
var hr=h.getBoundingClientRect(),above=hr.top-pv.getBoundingClientRect().bottom,below=nx.getBoundingClientRect().top-hr.bottom;
if(below>above)add(out.gaps,{s:sel(h),d:lab(h,txt(h)),above:Math.round(above),below:Math.round(below)})}catch(x){}});
function block(x){if(x.e.closest('nav,button,footer,[role=navigation]'))return false;var a=x.s.textAlign;return (a==='left'||a==='start'||a==='justify')&&(x.e.closest('h1,h2,h3,h4,h5,h6')||x.t.length>=20)}
var origins={};
texts.forEach(function(x){if(x.r.top>=2*H||x.r.bottom<=0||!block(x))return;var k=Math.round(x.r.left/4)*4,o=origins[k]||(origins[k]={x:k,n:0,d:lab(x.e,x.t),s:sel(x.e)});o.n++});
out.anchors=Object.keys(origins).map(function(k){return origins[k]}).sort(function(a,b){return b.n-a.n});
var boxes=new Set();
texts.forEach(function(x){if(x.r.top>=H||x.r.bottom<=0||!block(x))return;out.boxed.texts++;
for(var a=x.e,i=0;a&&a!==D.body&&i<10;a=a.parentElement,i++){var as=st(a);if(rad(as)>=4&&(framed(as)||shadow(as)||fill(as))&&a.getBoundingClientRect().width*a.getBoundingClientRect().height<0.6*W*H){out.boxed.inBox++;if(!boxes.has(a)){boxes.add(a);if(out.boxed.ex.length<3)out.boxed.ex.push(lab(a,txt(a)));if(out.boxed.at.length<24)out.boxed.at.push(sel(a))}return}}});
out.boxed.boxes=boxes.size;
return JSON.stringify(out)}finally{ours.forEach(function(e,i){e.style.display=was[i]})}})()`
