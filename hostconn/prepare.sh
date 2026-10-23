# Readies a server for enowxd before it is installed, as root where that
# is needed (root itself, else sudo or doas without a password), and says
# what to run otherwise:
#   - CA certificates, to download what terminals load over HTTPS
#     (minimal images and containers come without them);
#   - on musl systems (Alpine), the glibc compatibility layer: the library
#     terminals load is built for glibc.
need=""
for f in /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt /etc/ssl/ca-bundle.pem /etc/ssl/cert.pem; do
	[ -s "$f" ] && break
	f=""
done
[ -n "$f" ] || need="ca-certificates"
if command -v apk >/dev/null 2>&1 && ! ls /lib/ld-linux-* /lib64/ld-linux-* >/dev/null 2>&1; then
	need="$need gcompat libstdc++"
fi
[ -n "$need" ] || exit 0
if command -v apt-get >/dev/null 2>&1; then
	install="apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $need"
elif command -v dnf >/dev/null 2>&1; then
	install="dnf install -y -q $need"
elif command -v zypper >/dev/null 2>&1; then
	install="zypper -q install -y $need"
elif command -v pacman >/dev/null 2>&1; then
	install="pacman -Sy --needed --noconfirm $need"
elif command -v apk >/dev/null 2>&1; then
	install="apk add --no-cache $need"
else
	echo "enowxd needs$need on this server: install it, then connect again" >&2
	exit 3
fi
if [ "$(id -u)" = 0 ]; then
	sh -c "$install"
elif sudo -n true 2>/dev/null; then
	sudo -n sh -c "$install"
elif command -v doas >/dev/null 2>&1 && doas -n true 2>/dev/null; then
	doas -n sh -c "$install"
else
	echo "enowxd needs $need on this server: run '$install' as root, then connect again" >&2
	exit 3
fi
