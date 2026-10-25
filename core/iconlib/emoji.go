package iconlib

import "strings"

// FromEmoji is the library icon closest to emoji e, as notes' icons of
// before they were SVG move to; known is false when there is none close,
// and the default is given.
func FromEmoji(e string) (ref string, known bool) {
	key := strings.TrimSpace(strings.NewReplacer("️", "", "︎", "").Replace(e))
	if name, ok := emojiIcons[key]; ok {
		return "lucide:" + name, true
	}
	// A skin tone or a sequence: its first emoji's.
	for _, r := range key {
		if name, ok := emojiIcons[string(r)]; ok {
			return "lucide:" + name, true
		}
		break
	}
	return Default, false
}

// emojiIcons maps emoji to Lucide's icons.
var emojiIcons = map[string]string{
	"📝": "notebook-pen", "📌": "pin", "📍": "map-pin", "📋": "clipboard-list", "✅": "circle-check",
	"☑": "square-check", "✔": "check", "❌": "circle-x", "✖": "x", "❎": "square-x",
	"🗒": "notepad-text", "📓": "notebook", "📔": "notebook", "📒": "notebook", "📕": "book", "📗": "book",
	"📘": "book", "📙": "book", "📚": "library", "📖": "book-open", "📄": "file-text", "📃": "file-text",
	"📑": "files", "🗂": "folders", "📁": "folder", "📂": "folder-open", "🗃": "archive", "🗄": "archive",
	"💡": "lightbulb", "🎯": "target", "🚀": "rocket", "🐛": "bug", "🐞": "bug", "🔧": "wrench",
	"🔨": "hammer", "🛠": "hammer", "⚙": "settings", "🧪": "flask-conical", "🔬": "microscope",
	"📦": "package", "🔒": "lock", "🔓": "lock-open", "🔐": "lock-keyhole", "🔑": "key", "🗝": "key-round",
	"🌐": "globe", "🌍": "earth", "🌎": "earth", "🌏": "earth", "📱": "smartphone", "💻": "laptop",
	"🖥": "monitor", "⌨": "keyboard", "🖱": "mouse", "🖨": "printer", "📅": "calendar", "📆": "calendar-days",
	"🗓": "calendar-range", "⏰": "alarm-clock", "⏱": "timer", "⌛": "hourglass", "⏳": "hourglass",
	"🕐": "clock", "⭐": "star", "🌟": "sparkle", "✨": "sparkles", "🔥": "flame", "🧠": "brain",
	"🎨": "palette", "🖌": "paintbrush", "✏": "pencil", "🖊": "pen", "🖋": "pen-tool", "📈": "trending-up",
	"📉": "trending-down", "📊": "chart-column", "💬": "message-circle", "🗨": "message-square",
	"💭": "message-circle-more", "🤖": "bot", "🏠": "house", "🏡": "house", "🏢": "building-2",
	"🏭": "factory", "✈": "plane", "🚗": "car", "🚲": "bike", "🚢": "ship", "🚆": "train-front",
	"❤": "heart", "💖": "heart", "💔": "heart-crack", "👍": "thumbs-up", "👎": "thumbs-down",
	"👋": "hand", "✋": "hand", "👀": "eye", "👁": "eye", "👤": "user", "👥": "users", "🧑": "user",
	"👨": "user", "👩": "user", "🙂": "smile", "😀": "smile", "😃": "smile", "😄": "laugh", "😂": "laugh",
	"😊": "smile", "😐": "meh", "😞": "frown", "😢": "frown", "😡": "angry", "🤔": "message-circle-question",
	"❓": "circle-question-mark", "❔": "circle-question-mark", "❗": "circle-alert", "❕": "circle-alert",
	"⚠": "triangle-alert", "🚨": "siren", "🚫": "ban", "⛔": "circle-minus", "🛑": "octagon-x",
	"ℹ": "info", "💰": "wallet", "💵": "banknote", "💳": "credit-card", "💎": "gem", "🏆": "trophy",
	"🥇": "medal", "🎉": "party-popper", "🎊": "party-popper", "🎁": "gift", "🎮": "gamepad-2",
	"🎵": "music", "🎶": "music", "🎧": "headphones", "🎤": "mic", "📷": "camera", "📸": "camera",
	"🎥": "video", "📹": "video", "🎬": "clapperboard", "📺": "tv", "📻": "radio", "🔊": "volume-2",
	"🔇": "volume-x", "🔔": "bell", "🔕": "bell-off", "📣": "megaphone", "📢": "megaphone",
	"✉": "mail", "📧": "mail", "📨": "mail", "📩": "mail", "📬": "mailbox", "📮": "mailbox",
	"📞": "phone", "☎": "phone", "🔗": "link", "📎": "paperclip", "🖇": "paperclip", "✂": "scissors",
	"🔍": "search", "🔎": "search", "🧭": "compass", "🗺": "map", "🏔": "mountain", "⛰": "mountain",
	"🌲": "tree-pine", "🌳": "trees", "🌱": "sprout", "🌿": "leaf", "🍀": "clover", "🌸": "flower",
	"🌺": "flower-2", "🌻": "flower", "☀": "sun", "🌞": "sun", "🌙": "moon", "🌛": "moon", "⛅": "cloud-sun",
	"☁": "cloud", "🌧": "cloud-rain", "⛈": "cloud-lightning", "❄": "snowflake", "⚡": "zap",
	"🌈": "rainbow", "💧": "droplet", "🌊": "waves", "☕": "coffee", "🍕": "pizza", "🍔": "sandwich",
	"🍎": "apple", "🍏": "apple", "🍰": "cake-slice", "🎂": "cake", "🍺": "beer", "🍷": "wine",
	"🐶": "dog", "🐕": "dog", "🐱": "cat", "🐈": "cat", "🐦": "bird", "🐟": "fish", "🐢": "turtle",
	"🐍": "rat", "🦊": "squirrel", "🐝": "bug", "🏃": "footprints", "💪": "biceps-flexed",
	"🧘": "person-standing", "⚽": "volleyball", "🏀": "volleyball", "🛒": "shopping-cart",
	"🛍": "shopping-bag", "🏷": "tag", "🔖": "bookmark", "🎓": "graduation-cap", "🏫": "school",
	"🏥": "hospital", "💊": "pill", "🩺": "stethoscope", "♻": "recycle", "🔄": "refresh-cw",
	"🔁": "repeat", "🔃": "refresh-cw", "➕": "plus", "➖": "minus", "➡": "arrow-right",
	"⬅": "arrow-left", "⬆": "arrow-up", "⬇": "arrow-down", "↩": "undo-2", "↪": "redo-2",
	"🔜": "arrow-right", "🆕": "badge-plus", "🆗": "badge-check", "💯": "badge-check",
	"🧩": "puzzle", "🧱": "brick-wall", "🏗": "construction", "🚧": "construction", "🔌": "plug",
	"🔋": "battery-full", "💾": "save", "💿": "disc", "📀": "disc-3", "🗑": "trash-2", "🧹": "brush-cleaning",
	"🧾": "receipt", "📐": "triangle-right", "📏": "ruler", "🧮": "calculator", "🔢": "hash",
	"#": "hash", "🏁": "flag", "🚩": "flag", "🏳": "flag", "🛡": "shield", "⚔": "swords", "🗡": "sword",
	"🔫": "crosshair", "💣": "bomb", "🧲": "magnet", "🔭": "telescope", "🛰": "satellite", "🌌": "orbit",
	"👑": "crown", "🎩": "graduation-cap", "👕": "shirt", "💼": "briefcase", "🎒": "backpack",
	"🧳": "luggage", "☂": "umbrella", "🌂": "umbrella", "🕹": "joystick", "🎲": "dices", "♟": "chess-pawn",
	"🃏": "spade", "🀄": "layout-grid", "🖼": "image", "🌅": "sunrise", "🌇": "sunset", "🗿": "landmark",
	"🏛": "landmark", "⛪": "church", "🕌": "landmark", "🏰": "castle", "🗼": "tower-control",
	"🌉": "milestone", "⛺": "tent", "🏕": "tent", "🛏": "bed", "🛋": "sofa", "🚪": "door-open",
	"🪟": "app-window", "🧸": "baby", "🔮": "sparkles", "🪄": "wand-sparkles", "🧿": "eye",
	"📡": "radio-tower", "🔦": "flashlight", "🕯": "flame-kindling", "🧯": "fire-extinguisher",
	"📜": "scroll-text", "🗞": "newspaper", "📰": "newspaper", "🏴": "flag", "🐳": "container",
	"🐋": "container", "🐧": "terminal", "🍃": "leaf", "💥": "zap", "💫": "sparkles", "🌀": "tornado",
	"🔴": "circle", "🟠": "circle", "🟡": "circle", "🟢": "circle", "🔵": "circle", "🟣": "circle",
	"⚫": "circle", "⚪": "circle", "🟥": "square", "🟩": "square", "🟦": "square", "⬛": "square",
	"⬜": "square", "🔶": "diamond", "🔷": "diamond", "🔺": "triangle", "🔻": "triangle",
	"🧑‍💻": "laptop", "👨‍💻": "laptop", "👩‍💻": "laptop", "🏳‍🌈": "rainbow",
}
