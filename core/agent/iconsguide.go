package agent

// IconsGuide is what agents are told of notes' icons: SVG, never emoji.
const IconsGuide = `Icons of notes and groups are SVG icons, never emoji: lib:name, like lucide:rocket (libraries lucide, tabler, phosphor; "#rrggbb" after it for a color), given with --icon (the MCP tools' icon argument), or the user's own SVG with --icon-svg FILE. "$ENOWX_BIN" icons search WORD (MCP icons_search) finds names by name and keyword. In a note's text, :icon[lucide:check] draws an icon in the line, and a ` + "```svg" + ` code block a drawing.`
