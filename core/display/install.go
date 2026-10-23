package display

// InstallFor is the shell command that installs Xpra on a host whose
// package manager is pm ("apt-get", "dnf", "zypper", "pacman", "apk"), as root
// or through sudo; "" for one it doesn't know.
func InstallFor(pm string, root bool) string {
	sudo := "sudo "
	if root {
		sudo = ""
	}
	switch pm {
	case "apt-get":
		// Debian 13 dropped Xpra: add xpra.org's repository where apt has
		// none, for the release the host runs.
		return sudo + `sh -c '. /etc/os-release; if ! apt-cache policy xpra | grep -q "Candidate: [0-9]"; then ` +
			`get() { wget -qO "$1" "$2" || curl -fsSLo "$1" "$2"; }; ` +
			`get /usr/share/keyrings/xpra.asc https://xpra.org/xpra.asc && ` +
			`get /etc/apt/sources.list.d/xpra.sources https://raw.githubusercontent.com/Xpra-org/xpra/master/packaging/repos/$VERSION_CODENAME/xpra.sources; fi; ` +
			`apt-get update && apt-get install -y xpra dbus-x11'`
	case "dnf":
		return sudo + "dnf install -y xpra"
	case "zypper":
		return sudo + "zypper install -y xpra"
	case "pacman":
		return sudo + "pacman -Sy --noconfirm xpra"
	case "apk":
		return sudo + "apk add --no-cache xpra dbus-x11"
	}
	return ""
}
