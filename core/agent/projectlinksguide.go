package agent

// ProjectLinksGuide tells an agent how a project's copies on other hosts
// sync and how its work is pushed on its own, for the Guide.
const ProjectLinksGuide = `Copies on other hosts and auto pushes ("project help" says more; P is the project's folder, its name, or . for this one):
    "$ENOWX_BIN" project copy P --to HOST       # a copy on another host of the user's app, which remembers it came from here
    "$ENOWX_BIN" project links P                # its copies, how each syncs, how the last sync went; "project origin P" where it came from
    "$ENOWX_BIN" project sync P --on --interval 5m   # its copy syncs on its own (--realtime as files change; --direction one-way|two-way|reverse); --now syncs now
    "$ENOWX_BIN" project conflicts P            # files changed on both copies, kept as each has them: tell the user, don't pick yourself
    "$ENOWX_BIN" project autopush P --now       # commits the work to its own branch (enowx/autosync) and pushes it, never touching the user's branch; do it when you finish a piece of work the user may want safe; --on --interval 30m pushes on its own
Copies and syncs need the user's enowX app open (it alone reaches both hosts); a push that would carry a secret stops: remove the secret, never work around the check.`
