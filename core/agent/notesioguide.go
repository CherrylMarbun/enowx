package agent

// NotesIOGuide is the Guide's part on bringing notes in from other apps
// and taking them out, as its own lines so a feature switch can leave it
// out.
const NotesIOGuide = `    "$ENOWX_BIN" notes import ~/Downloads/Export.zip --dry-run   # notes from Notion, Obsidian, .md, Evernote, Keep, HTML; --dry-run lists them first, --group G
    "$ENOWX_BIN" notes export --all --format zip --out ~/backup   # or NOTE, --group G; md, zip (Obsidian/Notion), html, pdf, json (a backup import restores)`
