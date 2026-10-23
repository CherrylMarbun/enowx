package mcptest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/enowx/enowx/core/envsync"
)

// answer is what a fake MCP server says to a request.
func answer(line []byte) (string, bool) {
	var req struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
	}
	if json.Unmarshal(line, &req) != nil || req.ID == 0 {
		return "", false
	}
	switch req.Method {
	case "initialize":
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"fake","version":"1.0"},"capabilities":{}}}`, req.ID), true
	case "tools/list":
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"search","description":"Searches.\nMore."}]}}`, req.ID), true
	}
	return "", false
}

// TestMain lets the test binary be a stdio MCP server.
func TestMain(m *testing.M) {
	if os.Getenv("FAKE_MCP") == "1" {
		fmt.Fprintln(os.Stderr, "fake server starting")
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			if out, ok := answer(sc.Bytes()); ok {
				fmt.Println("log line that isn't JSON")
				fmt.Println(out)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestStdioServer(t *testing.T) {
	res := Test(context.Background(), envsync.MCPServer{Command: os.Args[0], Env: map[string]string{"FAKE_MCP": "1"}})
	if !res.OK || res.Server != "fake 1.0" || len(res.Tools) != 1 || res.Tools[0].Description != "Searches." {
		t.Fatalf("test %+v", res)
	}
}

func TestStdioServerThatFails(t *testing.T) {
	res := Test(context.Background(), envsync.MCPServer{Command: "sh", Args: []string{"-c", "echo boom >&2; exit 3"}})
	if res.OK || !strings.Contains(res.Stderr, "boom") || res.Error == "" {
		t.Fatalf("test %+v", res)
	}
	if res := Test(context.Background(), envsync.MCPServer{Command: "no-such-mcp-server"}); res.OK || !strings.Contains(res.Error, "can't run") {
		t.Fatalf("missing command: %+v", res)
	}
}

func TestHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		out, ok := answer(body)
		if !ok {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Mcp-Session-Id", "s1")
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", out)
	}))
	defer srv.Close()
	res := Test(context.Background(), envsync.MCPServer{URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer k"}})
	if !res.OK || len(res.Tools) != 1 {
		t.Fatalf("test %+v", res)
	}
	if res := Test(context.Background(), envsync.MCPServer{URL: srv.URL}); res.OK || !strings.Contains(res.Error, "401") {
		t.Fatalf("without the key: %+v", res)
	}
}
