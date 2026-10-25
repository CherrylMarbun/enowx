package agent

// CanvasGuide is the canvas skill: how an agent designs on a canvas, which
// it reads when asked for a mockup, a wireframe or a diagram ("enowx-app
// canvas guide"), not in every session's guide, as it is long.
const CanvasGuide = `# Designing on an enowX canvas

A canvas is a mockup of a UI (HTML and CSS), a wireframe, a diagram or a free board, kept on this host and shown live in the user's enowX app (Canvas page): they watch each element appear as you write it, comment on elements, edit them, pause or stop you. Work an element at a time, so they see the design build up and their changes to other elements stay.

## Start
    "$ENOWX_BIN" canvas new "Checkout" --kind mockup --frame desktop --frame mobile   # prints its ID
    "$ENOWX_BIN" canvas open ID          # shows it in the app: do this first, so the user watches
    "$ENOWX_BIN" canvas show ID          # frames, elements by id, open comments
Kinds: mockup (HTML, real UI), wireframe (boxes and text, grey, no styling), diagram (shapes and arrows: flows, architecture, sequence), board (stickies, shapes, images). Frames named desktop (1440×900), tablet (768×1024) or mobile (390×844) take those sizes; --frame "Home:1280x800" any.

## Mockups: HTML an element at a time
1. The skeleton with the page's CSS, its regions empty, each with an id:
    "$ENOWX_BIN" canvas write ID --html - <<'EOF'
    <style>/* tokens and layout: see below */</style>
    <header id="nav"></header><main id="main"></main><footer id="footer"></footer>
    EOF
2. Then each piece, in reading order, in its own call: patch replaces element E, or adds the HTML at the end of --into P when no element has id E:
    "$ENOWX_BIN" canvas patch ID --element hero --into main --html '<section id="hero">…</section>'
3. Small changes: --text sets what an element holds; remove takes one out.
    "$ENOWX_BIN" canvas patch ID --element hero-title --text "Ship faster"
Give every element you may change later an id (kebab-case, unique). One <style> block in the skeleton; add rules with a patch of an element <style id="styles-cards">. No JavaScript beyond small interactions (tabs, a menu toggle, a modal opening): the canvas is interactive in the app's Interact mode, so buttons, links between frames (#frame-name), inputs and :hover states should work.
Responsive: one HTML per frame. Write desktop first, then the tablet and mobile frames (--frame mobile) as their own layouts, not a squeezed desktop: stacked columns, a menu button, 16px gutters, 44px tap targets.

## Many designs in one canvas
Asked for several designs or variations ("10 landing pages", "3 versions of the hero"), make them designs of one canvas, not one long page: each is a frame the user browses, compares side by side, stars and picks from. Number them from 1.
    "$ENOWX_BIN" canvas new-design ID --title "Bold hero" --description "Big type, one call to action" --page Home --variant B   # prints its number
    "$ENOWX_BIN" canvas write ID --frame 2 --html -          # then build design 2 as any mockup
    "$ENOWX_BIN" canvas new-design ID --from 5 --variant C   # a copy of design 5 to vary
    "$ENOWX_BIN" canvas designs ID      # numbers, pages × variants, the favorites and the winner the user marked
    "$ENOWX_BIN" canvas goto ID 3 | compare ID 2 5 | pick ID 4
Group a flow's screens with --page (Home, Login, Dashboard) and alternatives with --variant (A, B, C); give each design a title and a one-line description of what it tries. Work on what the user starred or picked when asked to "develop designs 3 and 7", and read the comments on each design. Between a flow's screens, link buttons to other designs so Interact mode moves between them: href="#design-2" or data-goto="Login". A design's link for notes and chats is canvas://ID/design-N.

## Wireframes, diagrams, boards: shapes
    "$ENOWX_BIN" canvas shape ID --type rect --id api --x 80 --y 120 --w 200 --h 80 --text "API" --fill "#dbeafe" --radius 10
    "$ENOWX_BIN" canvas shape ID --type arrow --from api --to db --text "SQL"
Types: rect, ellipse, diamond (decisions), text, sticky, line, arrow, image (--src an asset kept with "canvas asset ID logo.png"). Arrows follow the shapes they join when those move. Lay out on a grid of 8: equal gaps (40–80 between boxes), flows left to right or top to bottom, at most 7–9 boxes per frame; split bigger diagrams into frames. A legend sticky when colors mean something.

## Look at what you made
    "$ENOWX_BIN" canvas screenshot ID /tmp/canvas.png --frame mobile   # then read the PNG
Check spacing, alignment, overflow and contrast; fix and look again before saying it is done.

## The user's comments, pause and stop
"$ENOWX_BIN" canvas comments ID lists what they asked for on which element; do each, then "canvas resolve ID COMMENT". A change refused with "paused" means the user paused you: stop writing, tell them, and go on from "canvas show" when they resume. Every change is a version: "canvas history", "undo", "restore N".

## From design to code
Asked to make it real: read the project's components and tokens first, then build it in the project's stack (React, Vue, Svelte or plain HTML) matching the frames, one component per region, and compare with "canvas screenshot" against the app running in the Browser.

## Grid first
Before placing anything, give the design a layout grid; the user sees it as guides, shapes snap to it, and every design of the canvas then shares one rhythm:
    "$ENOWX_BIN" canvas grid ID --frame 1 --cols 12 --rows 8 --gap 24 --margin 48 \
      --area header:1,1,12,1 --area hero:1,2,7,3 --area visual:8,2,5,3 --area features:1,5,12,3 --area footer:1,8,12,1 --apply
--apply lays the design's HTML out on it (a <style> with the grid's CSS and an empty element per area): then fill each area with canvas patch --element hero …. Desktop: 12 columns, gap 24, margin 48–80; tablet (768): 8 columns, gap 16, margin 32; mobile (390): 4 columns, gap 16, margin 16. Shapes (wireframes, boards) snap to the grid's lines; place them by column and row, never at ad-hoc pixels. canvas grid ID prints the grid and its CSS again.

## Libraries in mockups
A mockup renders in the user's enowX app (a web view of their device), so libraries load from a CDN when the device is online. Pin an exact version, as below; never "latest". Offline, a CDN's script or stylesheet doesn't load: the page falls back to its own CSS, so keep the layout, colors and type in your own <style> (or Tailwind's classes with the CDN) and use libraries for what is extra (motion, charts, 3D). Icons work offline as inline SVG.
- Tailwind CSS (utility classes, in the browser):
    <script src="https://cdn.tailwindcss.com/3.4.16"></script>
    <script>tailwind.config={theme:{extend:{colors:{primary:'#2563eb'},fontFamily:{sans:['Inter','system-ui','sans-serif']}}}}</script>
    <section id="hero" class="mx-auto max-w-6xl px-6 py-20 grid gap-6 md:grid-cols-2"><h1 class="text-4xl font-semibold tracking-tight">…</h1></section>
- shadcn/ui-style tokens (with Tailwind or plain CSS), light and dark:
    <style>:root{--background:0 0% 100%;--foreground:222 47% 11%;--primary:221 83% 53%;--primary-foreground:0 0% 100%;--muted:210 40% 96%;--muted-foreground:215 16% 47%;--border:214 32% 91%;--radius:0.5rem}
    .dark{--background:222 47% 11%;--foreground:210 40% 98%;--muted:217 33% 17%;--border:217 33% 17%}
    body{background:hsl(var(--background));color:hsl(var(--foreground))} .btn{background:hsl(var(--primary));color:hsl(var(--primary-foreground));border-radius:var(--radius)}</style>
  with Tailwind: tailwind.config={theme:{extend:{colors:{background:'hsl(var(--background))',foreground:'hsl(var(--foreground))',primary:'hsl(var(--primary))',border:'hsl(var(--border))'},borderRadius:{lg:'var(--radius)'}}}}
- DaisyUI (components on Tailwind): <link href="https://cdn.jsdelivr.net/npm/daisyui@4.12.14/dist/full.min.css" rel="stylesheet"> before the Tailwind script; <button class="btn btn-primary">, <div class="card bg-base-100 shadow">, data-theme="light" on <html>.
- Bootstrap: <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet"> and <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script> for its dropdowns and modals.
- Icons: prefer inline SVG, which works offline and takes the text's color: "$ENOWX_BIN" icons search "rocket" finds names in Lucide, Tabler and Phosphor, "$ENOWX_BIN" canvas icon lucide:rocket --size 20 prints the SVG to paste. From a CDN instead: Lucide <script src="https://cdn.jsdelivr.net/npm/lucide@0.468.0/dist/umd/lucide.min.js"></script> with <i data-lucide="rocket"></i> and lucide.createIcons() after it; a Tabler icon as <img src="https://cdn.jsdelivr.net/npm/@tabler/icons@3.24.0/icons/outline/rocket.svg" width="20" height="20" alt="">. No emoji or icon fonts as icons.
- Fonts: <link rel="preconnect" href="https://fonts.googleapis.com"><link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">, with font-family:Inter,system-ui,sans-serif so it falls back offline. Two families at most: one for text, one for headings or code.
- Motion (anime.js): <script src="https://cdnjs.cloudflare.com/ajax/libs/animejs/3.2.2/anime.min.js"></script>
    <script>anime({targets:'#hero h1',translateY:[16,0],opacity:[0,1],duration:600,easing:'easeOutCubic'})</script>
  Keep it short (150–600 ms), on entrance and feedback only, and honor prefers-reduced-motion.
- Charts (Chart.js): <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js"></script>
    <canvas id="chart-revenue" height="240"></canvas>
    <script>new Chart(document.getElementById('chart-revenue'),{type:'line',data:{labels:['Jan','Feb','Mar','Apr'],datasets:[{label:'Revenue',data:[12,19,15,24],borderColor:'#2563eb',tension:.3}]},options:{plugins:{legend:{display:false}}}})</script>
- 3D (three.js), as a module:
    <script type="importmap">{"imports":{"three":"https://cdn.jsdelivr.net/npm/three@0.160.0/build/three.module.js"}}</script>
    <div id="scene" style="height:360px"></div>
    <script type="module">import * as THREE from 'three';const el=document.getElementById('scene');const r=new THREE.WebGLRenderer({antialias:true,alpha:true});r.setSize(el.clientWidth,el.clientHeight);el.append(r.domElement);const s=new THREE.Scene(),c=new THREE.PerspectiveCamera(45,el.clientWidth/el.clientHeight,.1,100);c.position.z=4;const m=new THREE.Mesh(new THREE.TorusKnotGeometry(.8,.28,128,16),new THREE.MeshNormalMaterial());s.add(m);r.setAnimationLoop(()=>{m.rotation.y+=.01;r.render(s,c)})</script>
Screenshots (canvas screenshot) are made by the app too, so they show libraries as the user sees them; give scripts that draw (charts, 3D) a fixed height, and draw on load, not on a click.

## Design rules
- Grid first (above): every block starts and ends on the grid's columns; no element positioned by guesswork.
- Spacing scale only: 4 8 12 16 24 32 48 64 96; the same gap between siblings of a kind; more space between sections than inside them.
- Type scale: 12/14/16/20/24/32/40/48, one family, line height 1.5 for text and 1.2 for headings, at most three sizes on a screen besides the headline.
- Contrast: text at least 4.5:1 on its background (3:1 for large text and icons); never grey on grey; test the dark variant too.
- Responsive: each device a design of its own on its own grid (desktop 12, tablet 8, mobile 4 columns), not a squeezed desktop; nothing wider than its frame; tap targets 44 px on mobile.
- Use the project's design system when there is one: Tailwind config (theme colors, fonts, radius), shadcn/ui (components.json, CSS variables --background, --foreground, --primary, --muted, --border, --radius), a tokens or theme file, existing components. Read them and reuse the names and values; otherwise use the defaults below.
- Defaults: font Inter, system-ui; text #0f172a, muted #64748b, border #e2e8f0, surface #ffffff on page #f8fafc, primary #2563eb (hover #1d4ed8), success #16a34a, danger #dc2626; radius 8px (cards 12px); shadow 0 1px 2px rgba(15,23,42,.06), 0 4px 12px rgba(15,23,42,.06).
- Type scale 12/14/16/20/24/32/40, line height 1.5 for text, 1.2 for headings; weights 400, 500, 600. Spacing on a 4px grid: 4 8 12 16 24 32 48 64. Content width 1120–1200px, centered.
- Hierarchy: one primary action per screen; secondary actions as outline or ghost buttons; real copy, not lorem ipsum: names, numbers, dates that fit the product.
- Contrast at least 4.5:1 for text; visible focus rings; labels on every input; empty, loading and error states where they matter.
- Icons are inline SVG (24×24 viewBox, stroke="currentColor", stroke-width 2, round caps), never emoji or icon fonts.
- Layout with flexbox and grid; no absolute positioning for layout; no fixed heights on text containers.

## Components (adapt the tokens)
Navbar:
    <header id="nav" style="display:flex;align-items:center;gap:24px;height:64px;padding:0 32px;border-bottom:1px solid #e2e8f0;background:#fff">
      <strong id="nav-logo">Acme</strong><nav id="nav-links" style="display:flex;gap:20px;color:#475569"><a>Product</a><a>Pricing</a></nav>
      <span style="flex:1"></span><button id="nav-cta" class="btn primary">Start free</button></header>
Buttons: .btn{height:40px;padding:0 16px;border-radius:8px;font:500 14px Inter;border:1px solid transparent;cursor:pointer} .primary{background:#2563eb;color:#fff} .outline{border-color:#e2e8f0;background:#fff;color:#0f172a}
Card:
    <article id="card-revenue" style="background:#fff;border:1px solid #e2e8f0;border-radius:12px;padding:20px;display:grid;gap:8px">
      <span style="color:#64748b;font-size:13px">Revenue</span><strong style="font-size:28px">$48,210</strong><span style="color:#16a34a;font-size:13px">+12% this month</span></article>
Form field:
    <label id="field-email" style="display:grid;gap:6px;font-size:14px;font-weight:500">Email
      <input type="email" placeholder="ada@example.com" style="height:40px;padding:0 12px;border:1px solid #e2e8f0;border-radius:8px;font:14px Inter"></label>
Table:
    <table id="orders" style="width:100%;border-collapse:collapse;font-size:14px"><thead><tr style="text-align:left;color:#64748b;border-bottom:1px solid #e2e8f0"><th style="padding:10px 12px">Order</th>…</tr></thead><tbody>…</tbody></table>
Modal:
    <div id="modal" style="position:fixed;inset:0;background:rgba(15,23,42,.4);display:grid;place-items:center">
      <div role="dialog" style="width:440px;background:#fff;border-radius:12px;padding:24px;display:grid;gap:16px">…</div></div>
Layouts: sidebar app (grid-template-columns:240px 1fr), dashboard (cards in grid-template-columns:repeat(auto-fit,minmax(220px,1fr)) gap 16px), landing (hero, logos, features in 3 columns, pricing, footer), settings (sections with a heading, a line of help and fields).
`
