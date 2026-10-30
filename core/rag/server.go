package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	erag "github.com/enowdev/enowx-rag/mcp-server/pkg/rag"

	"github.com/enowx/enowx/proto"
)

// server is an enowx-rag server (enowx-rag --serve) a host or a team uses
// instead of its own store: searched over its REST API, its token from
// the vault; it indexes folders on its own host.
type server struct {
	url, token string
}

func (s *server) call(ctx context.Context, method, path string, body, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.url+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("rag: the enowx-rag server: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("rag: the enowx-rag server: %s: %s", resp.Status, bytes.TrimSpace(data[:min(len(data), 300)]))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (s *server) ping(ctx context.Context) error {
	return s.call(ctx, http.MethodGet, "/api/stats", nil, nil)
}

func (s *server) search(ctx context.Context, id, query string, k int) ([]erag.Result, error) {
	var out struct {
		Results []erag.Result `json:"results"`
	}
	err := s.call(ctx, http.MethodPost, "/api/search", map[string]any{"project_id": id, "query": query, "k": k, "hybrid": true, "compress": true}, &out)
	return out.Results, err
}

// reindex has the server index folder dir, a path on its own host.
func (s *server) reindex(ctx context.Context, id, dir string) (proto.RAGStatus, error) {
	var out struct {
		Indexed, Deleted, Skipped int
		Files                     int `json:"files_scanned"`
	}
	err := s.call(ctx, http.MethodPost, "/api/projects/"+url.PathEscape(id)+"/reindex", map[string]string{"directory": dir}, &out)
	st := proto.RAGStatus{Project: dir, ID: id, Added: out.Indexed, Removed: out.Deleted, Skipped: out.Skipped, Files: out.Files}
	st.Chunks, _ = s.count(ctx, id)
	return st, err
}

func (s *server) count(ctx context.Context, id string) (int, error) {
	var out struct {
		Chunks int `json:"chunks"`
		Count  int `json:"count"`
	}
	err := s.call(ctx, http.MethodGet, "/api/projects/"+url.PathEscape(id), nil, &out)
	return max(out.Chunks, out.Count), err
}
