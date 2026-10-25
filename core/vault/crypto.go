package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/scrypt"
)

// A vault file holds the credentials sealed with a random master key
// (AES-256-GCM), and the master key sealed with a key derived from the
// user's master password (scrypt). The master key is all the app keeps,
// in the system's keychain or in memory: hosts only ever see the file.

// FileVersion is the format written.
const FileVersion = 1

// scrypt's cost: about a tenth of a second on a laptop.
const (
	scryptN = 1 << 15
	scryptR = 8
	scryptP = 1
)

const keySize = 32

// Additional data, so a sealed key can't pass for sealed data.
var (
	aadKey  = []byte("enowx-vault-key-1")
	aadData = []byte("enowx-vault-data-1")
)

// ErrWrongPassword is Unlock's error for a password that doesn't open the
// vault.
var ErrWrongPassword = errors.New("vault: wrong master password")

// ErrWrongKey is Open's error for a key that isn't the vault's.
var ErrWrongKey = errors.New("vault: the key doesn't open this vault")

// File is a vault as kept on disk, encrypted.
type File struct {
	Version int       `json:"version"`
	Updated time.Time `json:"updated"`
	Salt    []byte    `json:"salt"`
	N       int       `json:"n"`
	R       int       `json:"r"`
	P       int       `json:"p"`
	Key     []byte    `json:"key"`  // the master key, sealed
	Data    []byte    `json:"data"` // the Vault, sealed
}

// Create makes an empty vault whose master password is password, and
// returns it with its master key.
func Create(password string) (*File, []byte, error) {
	if len(password) < 8 {
		return nil, nil, errors.New("vault: the master password needs at least 8 characters")
	}
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, nil, err
	}
	f := &File{Version: FileVersion, N: scryptN, R: scryptR, P: scryptP}
	if err := f.setPassword(key, password); err != nil {
		return nil, nil, err
	}
	if err := f.Seal(&Vault{}, key); err != nil {
		return nil, nil, err
	}
	return f, key, nil
}

// Parse reads a vault file.
func Parse(data []byte) (*File, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("vault: not a vault file: %w", err)
	}
	if f.Version != FileVersion {
		return nil, fmt.Errorf("vault: format %d is newer than this enowX knows: update it", f.Version)
	}
	return &f, nil
}

// Bytes is the file as kept on disk.
func (f *File) Bytes() []byte {
	data, _ := json.MarshalIndent(f, "", "  ")
	return data
}

// Unlock returns the master key the password opens.
func (f *File) Unlock(password string) ([]byte, error) {
	pk, err := scrypt.Key([]byte(password), f.Salt, f.N, f.R, f.P, keySize)
	if err != nil {
		return nil, err
	}
	key, err := open(pk, f.Key, aadKey)
	if err != nil {
		return nil, ErrWrongPassword
	}
	return key, nil
}

// Open decrypts the credentials with the master key.
func (f *File) Open(key []byte) (*Vault, error) {
	plain, err := open(key, f.Data, aadData)
	if err != nil {
		return nil, ErrWrongKey
	}
	var v Vault
	if err := json.Unmarshal(plain, &v); err != nil {
		return nil, fmt.Errorf("vault: unreadable after decrypting: %w", err)
	}
	return &v, nil
}

// Seal encrypts v into the file with the master key.
func (f *File) Seal(v *Vault, key []byte) error {
	plain, err := json.Marshal(v)
	if err != nil {
		return err
	}
	sealed, err := seal(key, plain, aadData)
	if err != nil {
		return err
	}
	f.Data, f.Updated = sealed, time.Now().UTC()
	return nil
}

// ChangePassword seals the master key with a new password, the old one
// proving the user may.
func (f *File) ChangePassword(old, password string) error {
	key, err := f.Unlock(old)
	if err != nil {
		return err
	}
	if len(password) < 8 {
		return errors.New("vault: the master password needs at least 8 characters")
	}
	return f.setPassword(key, password)
}

func (f *File) setPassword(key []byte, password string) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	pk, err := scrypt.Key([]byte(password), salt, f.N, f.R, f.P, keySize)
	if err != nil {
		return err
	}
	sealed, err := seal(pk, key, aadKey)
	if err != nil {
		return err
	}
	f.Salt, f.Key = salt, sealed
	return nil
}

// seal encrypts plain with key, the nonce first.
func seal(key, plain, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, aad), nil
}

func open(key, sealed, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("vault: sealed data too short")
	}
	n := gcm.NonceSize()
	return gcm.Open(nil, sealed[:n], sealed[n:], aad)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != keySize {
		return nil, errors.New("vault: a key is 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
