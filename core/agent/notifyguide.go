package agent

// NotifyGuide is what agents are told of the app's notification sounds.
const NotifyGuide = `Sounds: the enowX app plays one on the user's computer when an agent finishes its turn, needs input or fails, wherever it runs. "$ENOWX_BIN" notify sound done|input|error [--text T] plays one yourself (when the user asked to be told, or at a long task's end); "notify status" says whether sounds are on; "notify set" changes them when asked (--on/--off, --volume 0-100, --sound done=chime, --quiet 22:00-07:00|off).`
