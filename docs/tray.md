# The tray

enowX keeps an icon in the menu bar (macOS) or the notification area (Windows, Linux) while it runs (`ui/tray.go`). Closing the window leaves the app there, following its sessions, so an agent that needs you still notifies you; **Settings → App → Keep in the menu bar when closed** turns that off, and closing then quits, as before. Quit from the icon's menu, the app menu or ⌘Q.

- **The icon:** the logo, tinted by macOS to suit the menu bar; next to it on macOS, how many sessions need input. Its tooltip says what the sessions are at.
- **A click** (macOS, Windows) opens a panel at the icon: the sessions that need input, then those working, then those done, the most recent first, up to eight. A session opens the window on it. **Open enowX**, **New session** (in the project shown), **Quit**.
- **A right click** shows its menu: Open enowX, New session, Quit.
- **Linux:** AppIndicator tells no clicks, so the icon has the menu only, with the sessions needing input in it.

Clicking a notification, or the Dock's icon on macOS, brings the window back.
