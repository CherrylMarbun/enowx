// Builds core/iconlib/data from the icon libraries' npm packages, unpacked:
//
//	npm pack lucide-static @tabler/icons @phosphor-icons/core
//	(unpack each into lucide/, tabler/, phosphor/ of one folder)
//	node scripts/iconlib.mjs DIR
//
// Each library becomes <name>.json.gz: {icon: {t: [keywords], s: svg}},
// its SVG minified, with its LICENSE beside it.
import fs from "node:fs";
import path from "node:path";
import zlib from "node:zlib";

const src = process.argv[2];
const out = path.join(path.dirname(new URL(import.meta.url).pathname), "..", "core", "iconlib", "data");

function minify(svg) {
  return svg
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/\s+class="[^"]*"/g, "")
    .replace(/<path stroke="none" d="M0 0h24v24H0z" fill="none"\s*\/>/g, "")
    .replace(/\s+/g, " ")
    .replace(/>\s+</g, "><")
    .replace(/\s*\/>/g, "/>")
    .replace(/<svg ([^>]*?)\s*>/, (m, a) => "<svg " + a.trim() + ">")
    .trim();
}

function write(name, icons, license) {
  fs.mkdirSync(out, { recursive: true });
  const data = zlib.gzipSync(JSON.stringify(icons), { level: 9 });
  fs.writeFileSync(path.join(out, name + ".json.gz"), data);
  fs.copyFileSync(license, path.join(out, name + ".LICENSE"));
  console.log(name, Object.keys(icons).length, "icons,", data.length, "bytes");
}

{
  const dir = path.join(src, "lucide", "package");
  const tags = JSON.parse(fs.readFileSync(path.join(dir, "tags.json"), "utf8"));
  const icons = {};
  for (const f of fs.readdirSync(path.join(dir, "icons")).sort()) {
    if (!f.endsWith(".svg")) continue;
    const name = f.slice(0, -4);
    icons[name] = { t: tags[name] || [], s: minify(fs.readFileSync(path.join(dir, "icons", f), "utf8")) };
  }
  write("lucide", icons, path.join(dir, "LICENSE"));
}
{
  const dir = path.join(src, "tabler", "package");
  const meta = JSON.parse(fs.readFileSync(path.join(dir, "icons.json"), "utf8"));
  const icons = {};
  for (const f of fs.readdirSync(path.join(dir, "icons", "outline")).sort()) {
    if (!f.endsWith(".svg")) continue;
    const name = f.slice(0, -4);
    const m = meta[name] || {};
    const t = (m.tags || []).map(String);
    if (m.category) t.push(m.category.toLowerCase());
    icons[name] = { t, s: minify(fs.readFileSync(path.join(dir, "icons", "outline", f), "utf8")) };
  }
  write("tabler", icons, path.join(dir, "LICENSE"));
}
{
  const dir = path.join(src, "phosphor", "package");
  const { icons: meta } = await import(path.join(dir, "dist", "index.mjs"));
  const icons = {};
  for (const m of meta) {
    const f = path.join(dir, "assets", "regular", m.name + ".svg");
    if (!fs.existsSync(f)) continue;
    icons[m.name] = { t: [...m.tags, ...m.categories].map(String), s: minify(fs.readFileSync(f, "utf8")) };
  }
  write("phosphor", icons, path.join(dir, "LICENSE"));
}
