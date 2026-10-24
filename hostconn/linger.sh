# Lets the user enowxd runs as keep services without signing in (systemd's
# lingering), so the daemon, a user service (core/userservice), comes back
# after a reboot. As root, or with sudo or doas without a password; a
# server where it can't be done runs the daemon as a process instead.
command -v loginctl >/dev/null 2>&1 && [ -d /run/systemd/system ] || exit 0
u=${1:-$(id -un)}
[ -e "/var/lib/systemd/linger/$u" ] && exit 0
loginctl enable-linger "$u" 2>/dev/null ||
	sudo -n loginctl enable-linger "$u" 2>/dev/null ||
	doas -n loginctl enable-linger "$u" 2>/dev/null || true
