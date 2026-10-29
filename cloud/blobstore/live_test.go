//go:build r2live

package blobstore

import (
	"bytes"
	"os"
	"testing"
)

// The S3 store against a real R2 bucket: go test -tags r2live with
// R2_ACCOUNT_ID, R2_BUCKET, R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY.
func TestR2Live(t *testing.T) {
	s := R2(os.Getenv("R2_ACCOUNT_ID"), os.Getenv("R2_BUCKET"), os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY"))
	key, data := "check/live-test", []byte("sealed bytes")
	if err := s.Put(t.Context(), key, data); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(t.Context(), key); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("get: %q %v", got, err)
	}
	if err := s.Delete(t.Context(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(t.Context(), key); err != ErrNotFound {
		t.Fatalf("after delete: %v", err)
	}
}
