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
// on an emulator, rooted, into the user's certificates; on a phone, into
// Downloads, for the user to install. The note says what is left to do.
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
	trust := "Apps trust it only if they trust user certificates: a debug build's network_security_config can (<debug-overrides> with <certificates src=\"user\"/>). Flutter's HTTP client doesn't read the system proxy: set HttpOverrides in debug."
	if strings.HasPrefix(serial, "emulator-") {
		if out, err := s.adb(serial, "root"); err == nil && !strings.Contains(out, "cannot") {
			s.adb(serial, "wait-for-device")
			time.Sleep(time.Second)
			dir := "/data/misc/user/0/cacerts-added"
			s.shell(serial, "mkdir -p "+dir)
			if _, err := s.adb(serial, "push", tmp, dir+"/"+name); err == nil {
				s.shell(serial, "chmod 644 "+dir+"/"+name)
				return "enowX's CA is among the emulator's user certificates. " + trust, nil
			}
		}
	}
	if _, err := s.adb(serial, "push", tmp, "/sdcard/Download/enowx-traffic-ca.crt"); err != nil {
		return "", err
	}
	return "Install enowX's CA on the device: Settings → Security → Encryption & credentials → Install a certificate → CA certificate, then Downloads/enowx-traffic-ca.crt. " + trust, nil
}

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
