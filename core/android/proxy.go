package android

import (
	"crypto/md5"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Proxy sends device serial's HTTP to port of this host (adb reverse
// takes the device's localhost there), and adds ca to what it trusts:
// with root (an emulator, a rooted phone) among the system's
// certificates; else into Downloads, Settings opened for the user to
// install it. The note says what is left to do.
func (s SDK) Proxy(serial string, port int, ca string) (string, error) {
	p := strconv.Itoa(port)
	if _, err := s.adb(serial, "reverse", "tcp:"+p, "tcp:"+p); err != nil {
		return "", err
	}
	if _, err := s.shell(serial, "settings put global http_proxy 127.0.0.1:"+p); err != nil {
		return "", err
	}
	if ca == "" {
		return "", nil
	}
	block, _ := pem.Decode([]byte(ca))
	if block == nil {
		return "", errors.New("android: the CA isn't PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}
	name := subjectHashOld(cert) + ".0"
	tmp := filepath.Join(os.TempDir(), "enowx-"+name)
	if err := os.WriteFile(tmp, []byte(ca), 0o644); err != nil {
		return "", err
	}
	defer os.Remove(tmp)
	// As root, the CA goes among the system's, which every app trusts.
	if asRoot := s.rootShell(serial); asRoot != nil {
		if err := s.trustSystem(serial, tmp, name, asRoot); err == nil {
			return "enowX's CA is among the device's system certificates: every app trusts it, until the device restarts (catch it again then).", nil
		}
	}
	// Else the user installs it, from a copy in Downloads: Settings shows.
	if _, err := s.adb(serial, "push", tmp, "/sdcard/Download/enowx-traffic-ca.crt"); err != nil {
		return "", err
	}
	s.shell(serial, "am start -a android.settings.SECURITY_SETTINGS")
	return "On the device, Settings is open: Encryption & credentials → Install a certificate → CA certificate, then Downloads/enowx-traffic-ca.crt. " +
		"Apps trust it only if they trust user certificates: a debug build's network_security_config can (<debug-overrides> with <certificates src=\"user\"/>). " +
		"Flutter's HTTP client doesn't read the system proxy: set HttpOverrides in debug.", nil
}

// rootShell is how to run a command as root on device serial: adb's
// root (emulators of Google APIs images), else su (rooted phones); nil
// when neither works.
func (s SDK) rootShell(serial string) func(cmd string) (string, error) {
	if strings.HasPrefix(serial, "emulator-") {
		if out, err := s.adb(serial, "root"); err == nil && !strings.Contains(out, "cannot") {
			s.adb(serial, "wait-for-device")
			time.Sleep(time.Second)
			if id, _ := s.shell(serial, "id -u"); id == "0" {
				return func(cmd string) (string, error) { return s.shell(serial, cmd) }
			}
		}
	}
	if id, err := s.shell(serial, "su -c 'id -u'"); err == nil && strings.TrimSpace(id) == "0" {
		return func(cmd string) (string, error) {
			return s.shell(serial, "su -c '"+strings.ReplaceAll(cmd, "'", `'\''`)+"'")
		}
	}
	return nil
}

// trustSystem adds the CA at local path, named name (its subject hash),
// to device serial's system certificates, with root: a copy of them on a
// tmpfs over their folder, the CA added; from Android 14, where they come
// from the Conscrypt APEX, bound over that too, in the mount namespaces
// of zygote and the apps already running. It lasts until a restart.
func (s SDK) trustSystem(serial, path, name string, asRoot func(string) (string, error)) error {
	dir := "/data/local/tmp/enowx-ca"
	if _, err := s.shell(serial, "mkdir -p "+dir); err != nil {
		return err
	}
	if _, err := s.adb(serial, "push", path, dir+"/"+name); err != nil {
		return err
	}
	script := strings.ReplaceAll(trustScript, "$NAME", name)
	scriptPath := filepath.Join(os.TempDir(), "enowx-trust-ca.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return err
	}
	defer os.Remove(scriptPath)
	if _, err := s.adb(serial, "push", scriptPath, dir+"/trust.sh"); err != nil {
		return err
	}
	out, err := asRoot("sh " + dir + "/trust.sh")
	if err != nil {
		return err
	}
	if !strings.Contains(out, "trusted") {
		return fmt.Errorf("android: the CA wasn't added: %s", lastLines(out, 3))
	}
	return nil
}

// trustScript runs on the device as root; $NAME is the CA's file name.
const trustScript = `
dir=/data/local/tmp/enowx-ca
sys=/system/etc/security/cacerts
apex=/apex/com.android.conscrypt/cacerts
src=$sys
[ -d $apex ] && src=$apex
if [ -f $src/$NAME ] && cmp -s $src/$NAME $dir/$NAME; then echo trusted already; exit 0; fi
rm -rf $dir/certs && mkdir -p $dir/certs
cp $src/* $dir/certs/ || exit 1
mount -t tmpfs tmpfs $sys || exit 1
cp $dir/certs/* $sys/
cp $dir/$NAME $sys/
chown root:root $sys/*
chmod 644 $sys/*
chcon u:object_r:system_file:s0 $sys $sys/* 2>/dev/null
if [ -d $apex ]; then
  mount --bind $sys $apex
  zygotes=" $(pidof zygote zygote64) "
  for z in $zygotes; do
    nsenter --mount=/proc/$z/ns/mnt -- mount --bind $sys $apex 2>/dev/null
  done
  ps -A -o PID,PPID | while read pid ppid; do
    case "$zygotes" in *" $ppid "*) nsenter --mount=/proc/$pid/ns/mnt -- mount --bind $sys $apex 2>/dev/null ;; esac
  done
fi
echo trusted
`

// ProxyOff undoes Proxy's proxy setting and reverse.
func (s SDK) ProxyOff(serial string, port int) error {
	s.shell(serial, "settings put global http_proxy :0")
	_, err := s.adb(serial, "reverse", "--remove", "tcp:"+strconv.Itoa(port))
	return err
}

// subjectHashOld is OpenSSL's old subject hash, which Android names its
// certificate files by: the first four bytes of the subject's MD5, as a
// little-endian number.
func subjectHashOld(cert *x509.Certificate) string {
	sum := md5.Sum(cert.RawSubject)
	return fmt.Sprintf("%08x", binary.LittleEndian.Uint32(sum[:4]))
}
