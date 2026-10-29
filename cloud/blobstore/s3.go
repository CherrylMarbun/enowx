package blobstore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// S3 is a store in an S3 bucket, its requests signed with AWS's signature
// version 4: Cloudflare R2's (R2) or any other's.
type S3 struct {
	// Endpoint is the service's address, as https://host.
	Endpoint string
	// Region is the bucket's: "auto" for R2.
	Region            string
	Bucket            string
	AccessKey, Secret string
	// Client sends the requests; http.DefaultClient when nil.
	Client *http.Client
}

// R2 is Cloudflare R2's bucket of account.
func R2(account, bucket, accessKey, secret string) *S3 {
	return &S3{Endpoint: "https://" + account + ".r2.cloudflarestorage.com", Region: "auto", Bucket: bucket, AccessKey: accessKey, Secret: secret}
}

// maxGet is the most Get reads of a file.
const maxGet = 64 << 20

// Put writes key's object.
func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.do(ctx, http.MethodPut, key, data)
	return err
}

// Get reads key's object.
func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	return s.do(ctx, http.MethodGet, key, nil)
}

// Delete removes key's object; one not there is no error.
func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.do(ctx, http.MethodDelete, key, nil)
	if err == ErrNotFound {
		return nil
	}
	return err
}

func (s *S3) do(ctx context.Context, method, key string, body []byte) ([]byte, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	u, err := url.Parse(strings.TrimRight(s.Endpoint, "/") + "/" + s.Bucket + "/" + key)
	if err != nil {
		return nil, fmt.Errorf("blobstore: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("blobstore: %w", err)
	}
	if body == nil {
		req.Body, req.ContentLength = http.NoBody, 0
	}
	s.sign(req, body, time.Now().UTC())
	c := s.Client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("blobstore: %s %s: %w", method, key, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGet))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode >= 300:
		// The service's message, cut short: it names no secret.
		return nil, fmt.Errorf("blobstore: %s %s: %s %.200s", method, key, resp.Status, data)
	case err != nil:
		return nil, fmt.Errorf("blobstore: %s %s: %w", method, key, err)
	}
	return data, nil
}

// sign signs req, with body, at t (signature version 4, its payload
// hashed).
func (s *S3) sign(req *http.Request, body []byte, t time.Time) {
	stamp, day := t.Format("20060102T150405Z"), t.Format("20060102")
	payload := sha256.Sum256(body)
	ph := hex.EncodeToString(payload[:])
	req.Header.Set("x-amz-date", stamp)
	req.Header.Set("x-amz-content-sha256", ph)
	canonical := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		"host:" + req.URL.Host + "\nx-amz-content-sha256:" + ph + "\nx-amz-date:" + stamp + "\n",
		"host;x-amz-content-sha256;x-amz-date",
		ph,
	}, "\n")
	scope := day + "/" + s.Region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	key := []byte("AWS4" + s.Secret)
	for _, part := range []string{day, s.Region, "s3", "aws4_request"} {
		key = mac(key, part)
	}
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.AccessKey+"/"+scope+
		", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+hex.EncodeToString(mac(key, toSign)))
}

func mac(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
