// Package blobstore keeps the cloud's files (a team chat's images, sealed
// by the members' devices) by key: in a folder of the server's, or in an
// S3 bucket such as Cloudflare R2's.
package blobstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// ErrNotFound is a key the store doesn't hold.
var ErrNotFound = errors.New("blobstore: not found")

// Store holds files by key. Its methods are safe from any goroutine.
type Store interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// validKey is a key a store takes: path segments of letters, digits and
// dashes, so none leaves a folder or a bucket's prefix.
var validKey = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+)*$`)

func checkKey(key string) error {
	if !validKey.MatchString(key) {
		return fmt.Errorf("blobstore: no key %q", key)
	}
	return nil
}

// Dir is a store in a folder.
type Dir string

func (d Dir) file(key string) (string, error) {
	if err := checkKey(key); err != nil {
		return "", err
	}
	return filepath.Join(string(d), filepath.FromSlash(key)), nil
}

// Put writes key's file.
func (d Dir) Put(_ context.Context, key string, data []byte) error {
	f, err := d.file(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
		return fmt.Errorf("blobstore: %w", err)
	}
	tmp := f + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("blobstore: %w", err)
	}
	if err := os.Rename(tmp, f); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("blobstore: %w", err)
	}
	return nil
}

// Get reads key's file.
func (d Dir) Get(_ context.Context, key string) ([]byte, error) {
	f, err := d.file(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(f)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

// Delete removes key's file; one not there is no error.
func (d Dir) Delete(_ context.Context, key string) error {
	f, err := d.file(key)
	if err != nil {
		return err
	}
	if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("blobstore: %w", err)
	}
	return nil
}
