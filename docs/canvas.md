# Canvas

The rail's **Canvas** page keeps canvases on the connected host: mockups of UIs in HTML and CSS, wireframes, diagrams (flows, architectures) and free boards. The user and agents design them together. An agent's changes reach every app connected to the host as it makes them, an element at a time, so the user watches the design build up, whether the agent runs on this Mac or on a server.

## Kinds and frames

A canvas is one of four kinds:

- **Mockup**: a UI in HTML and CSS, shown in a web view of this device, interactive.
- **Wireframe**: a UI in boxes and text.
- **Diagram**: shapes joined by arrows.
- **Board**: stickies, shapes and images, laid out freely.

A canvas has frames, its pages or the same page at a device's size. Frames named Desktop (1440×900), Tablet (768×1024) or Mobile (390×844) take those sizes; others take any. The toolbar's tabs switch frames, and **+** adds one at a device's size or removes the one shown.

A frame holds a mockup's HTML, its elements with `id` attributes, and shapes above it: rectangles, ellipses, decisions (diamonds), text, stickies, lines, arrows and images. Arrows join two shapes and follow them as they move.

## Designing

The stage shows the frame, scaled to fit.

- In **Design** mode, a click selects a shape and a drag moves it. The corner handle resizes it, and a double click edits its text in place. Delete removes it.
- The palette down the stage's side adds shapes. **Connect** draws an arrow from the selected shape to the next one clicked.
- **Layers** lists the frame's elements: the mockup's by id, and the shapes. The selected one shows its properties: a shape's text, fill and line colors, and a mockup element's HTML to edit and apply.
- A mockup previews at its frame's size, or at a desktop's, tablet's or phone's width.
- In **Interact** mode the mockup works as the real thing: buttons, links, forms and hover states.

Every change is made to one element of the canvas as the host has it now. A change the user makes while an agent designs never undoes the agent's work, and the agent's never undoes the user's.

## Agents designing live

When an agent changes a canvas, a bar under the header says who is designing and which element. The element changed is outlined a moment, with the agent's name beside it, and the page follows the agent to the frame it works on. Mockup elements go into the page in place, so it doesn't reload at each change. The list of canvases marks those being designed.

- **Pause** refuses the agent's changes until **Resume**. The agent is told it was paused, and goes on from what the canvas shows when resumed.
- **Stop** interrupts the agent, as Esc does in its terminal, and pauses.

Agents learn how to design from the canvas guide (`enowx-app canvas guide`), which the session guide points to rather than carrying it. It covers the steps, layout and type rules, a project's own design system first (Tailwind, shadcn/ui tokens), components to start from (navbar, card, form field, table, modal), responsive frames, and SVG icons only.

## Comments, versions, exports

- **Comments** go on the selected element, or on the frame. Agents read them (`canvas comments`), do them, and resolve them.
- **Versions** lists every change: whose it was, and what it changed. One compares with the version shown, the elements added, removed and changed, and **Restore** makes an old version the newest. Undo and Redo (⌘Z, Shift+⌘Z) step through versions.
- **Export** saves the frame as PNG, SVG, PDF or HTML into Downloads.
- **Share as a page** saves the whole canvas as one read-only HTML file, its images inside. There is no cloud: share the file.
- **Make code** starts an agent in one of the host's projects on building the canvas there, in the project's stack and components. It attaches the canvas to the project. **More** attaches a canvas to, or detaches it from, other projects.

## Rendering

The daemon draws a frame of shapes itself (PNG, PDF and SVG). Agents on a server see what they made without a screen or an app.

A frame with HTML needs a web engine. The daemon asks the app connected to the host and serving Browsers (`canvas.render` event), which loads the page in a web view behind its window, captures it, and answers (`canvas.renderReply`). Without such an app, rendering a mockup says to open it.

## Where they are kept

On the host, in `~/.enowx-app/canvas/<id>/`:

- `canvas.json`: the canvas as shown.
- `history/<n>.json`: every version, whole, the last 400.
- `history.json`: which version is shown, and what made each.
- `comments.json`.
- `assets/`: images that shapes show (`--src NAME`) and HTML shows (`assets/NAME`).

## Protocol

| Method | Does |
|---|---|
| `canvas.list`, `canvas.get` | The canvases, a canvas with its frames |
| `canvas.create`, `canvas.delete` | Make, delete |
| `canvas.edit` | A change: `meta`, `frame.add`/`set`/`remove`, `html.set`, `html.patch` (replace element `target`, or add into `into`), `html.remove`, `html.text`, `el.set`, `el.remove`; `session` marks an agent's |
| `canvas.history`, `canvas.version`, `canvas.undo`, `canvas.redo`, `canvas.restore` | Versions |
| `canvas.comments`, `canvas.comment` | Comments: add, resolve, reopen, delete |
| `canvas.control` | `pause`, `resume`, `stop`, `open` (shows it in the apps) |
| `canvas.asset`, `canvas.assetGet` | Keep and read an image |
| `canvas.render`, `canvas.renderReply` | A frame as PNG, PDF, SVG or HTML; the whole canvas as HTML |

Events: `canvas.changed` (a `CanvasChange`: the canvas, the op, frame and element, the session), `canvas.listed` (the list), `canvas.render` (to the app).
