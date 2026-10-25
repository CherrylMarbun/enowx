package agent

// BlocksGuide is how a note is laid out beyond plain Markdown
// (core/noteblocks), and read or changed a section at a time.
const BlocksGuide = `Notes can be laid out, not only long text: blocks the Notes page shows, plain Markdown elsewhere. A block opens with ":::name" on a line of its own and closes with ":::"; blocks nest.
    :::tabs / ::tab Summary / … / ::tab Details / … / :::        tabs, each "::tab Title" starts one
    :::toggle Title / … / :::                                     folded until opened
    :::columns / ::col / … / ::col / … / :::                      side by side (one column when narrow)
    :::callout info|warning|success|danger Title / … / :::        a highlighted box
    :::cards / ::card Title / a line, [link](url), {{status}} / ::card … / :::   a grid of cards
    ::toc   ::progress   ::pages   ::spacer   ::embed URL (an image, a link, canvas:ID)   one line each
    {{High}} is a badge; [[Page]], [[#Section]] and [[Page#Section]] link; - [ ] to-dos make progress bars.
A long note: "notes outline NOTE" lists its sections (headings, tabs, toggles, cards) with #IDs and to-dos done; "notes show NOTE --section S" reads one; "notes write NOTE --section S --body -" (or --append) changes one, the rest kept; "notes check NOTE --item TEXT" ticks a to-do (--undo unticks). MCP: notes_outline, notes_read and notes_write with section, notes_check.`
