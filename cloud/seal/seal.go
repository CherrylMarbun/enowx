// Package seal encrypts what a team shares on its members' devices, so
// the cloud keeps only ciphertext: a team's key sealed to each member
// device's public key, and the team's items sealed with that key.
package seal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// KeySize is a team key's size, in bytes (AES-256).
const KeySize = 32

// Domain separation: a key sealed for a device can't pass for an item, nor
// one version of the format for another.
const (
	infoKey  = "enowx-cloud-team-key-1"
	aadItem  = "enowx-cloud-item-1|"
	keyCheck = "enowx team key check"
)

// ErrOpen is the error for ciphertext that the key given doesn't open, or
// that was changed.
var ErrOpen = errors.New("seal: can't open: wrong key or changed data")

// DeviceKey is a device's key pair; its private half never leaves the
// device.
type DeviceKey struct{ priv *ecdh.PrivateKey }

// NewDeviceKey makes a device's key pair.
func NewDeviceKey() (*DeviceKey, error) {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("seal: device key: %w", err)
	}
	return &DeviceKey{k}, nil
}

// LoadDeviceKey is the key pair whose private half is priv, as Private
// returned it.
func LoadDeviceKey(priv []byte) (*DeviceKey, error) {
	k, err := ecdh.X25519().NewPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("seal: device key: %w", err)
	}
	return &DeviceKey{k}, nil
}

// Private is the private half, to keep on the device (keychain, 0600 file).
func (d *DeviceKey) Private() []byte { return d.priv.Bytes() }

// Public is the public half, which the cloud keeps for the device.
func (d *DeviceKey) Public() []byte { return d.priv.PublicKey().Bytes() }

// NewTeamKey makes a team's key for a new epoch.
func NewTeamKey() ([]byte, error) {
	k := make([]byte, KeySize)
	if _, err := rand.Read(k); err != nil {
		return nil, fmt.Errorf("seal: team key: %w", err)
	}
	return k, nil
}

// KeyCheck is team key key's check: HMAC-SHA256 of a fixed label, which
// tells the key from any other without giving it away. The device starting
// an epoch sends it once; the others compare their copies with it.
func KeyCheck(key []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(keyCheck))
	return m.Sum(nil)
}

// CheckKey says whether key matches check, as KeyCheck made it.
func CheckKey(key, check []byte) bool { return hmac.Equal(KeyCheck(key), check) }

// SealKey seals team key key to the device whose public key is to: an
// ephemeral X25519 key, HKDF-SHA256 over the shared secret and both public
// keys, AES-256-GCM. The result is the ephemeral public key, then the
// ciphertext.
func SealKey(key, to []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("seal: a team key is %d bytes, not %d", KeySize, len(key))
	}
	pub, err := ecdh.X25519().NewPublicKey(to)
	if err != nil {
		return nil, fmt.Errorf("seal: device's public key: %w", err)
	}
	eph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	shared, err := eph.ECDH(pub)
	if err != nil {
		return nil, err
	}
	aead, err := wrapAEAD(shared, eph.PublicKey().Bytes(), to)
	if err != nil {
		return nil, err
	}
	// The key is used once: a zero nonce is safe.
	out := append([]byte{}, eph.PublicKey().Bytes()...)
	return aead.Seal(out, make([]byte, aead.NonceSize()), key, nil), nil
}

// OpenKey opens a team key SealKey sealed to d.
func (d *DeviceKey) OpenKey(sealed []byte) ([]byte, error) {
	if len(sealed) < 32 {
		return nil, ErrOpen
	}
	eph, err := ecdh.X25519().NewPublicKey(sealed[:32])
	if err != nil {
		return nil, ErrOpen
	}
	shared, err := d.priv.ECDH(eph)
	if err != nil {
		return nil, ErrOpen
	}
	aead, err := wrapAEAD(shared, sealed[:32], d.Public())
	if err != nil {
		return nil, err
	}
	key, err := aead.Open(nil, make([]byte, aead.NonceSize()), sealed[32:], nil)
	if err != nil || len(key) != KeySize {
		return nil, ErrOpen
	}
	return key, nil
}

func wrapAEAD(shared, eph, to []byte) (cipher.AEAD, error) {
	k, err := hkdf.Key(sha256.New, shared, append(append([]byte{}, eph...), to...), infoKey, KeySize)
	if err != nil {
		return nil, err
	}
	return gcm(k)
}

// SealItem seals an item's data with team key key, bound to its ID and
// version: the server can't give one item, or an old version, for another.
// The result is the nonce, then the ciphertext.
func SealItem(key []byte, id string, version int64, data []byte) ([]byte, error) {
	aead, err := gcm(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, data, itemAAD(id, version)), nil
}

// OpenItem opens what SealItem sealed for item id at version.
func OpenItem(key []byte, id string, version int64, sealed []byte) ([]byte, error) {
	aead, err := gcm(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize() {
		return nil, ErrOpen
	}
	data, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], itemAAD(id, version))
	if err != nil {
		return nil, ErrOpen
	}
	return data, nil
}

func itemAAD(id string, version int64) []byte {
	return fmt.Appendf(nil, "%s%d|%s", aadItem, version, id)
}

func gcm(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("seal: a key is %d bytes, not %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
