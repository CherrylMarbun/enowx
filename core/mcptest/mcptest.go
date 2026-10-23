// Package mcptest checks that an MCP server works where an agent would run
// it: it starts it (or reaches its URL), opens a Model Context Protocol
// session and lists its tools, or says why it couldn't.
package mcptest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/proto"
)

// Timeout bounds a test: a server run with npx may download first.
const Timeout = 45 * time.Second

// protocolVersion is the MCP version the test speaks; servers answer with
// theirs.
const protocolVersion = "2025-06-18"

// maxStderr is how much of what a server printed a failed test keeps.
const maxStderr = 4 << 10

type rpcResponse struct {
	ID     *int            `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func request(id int, method string, params any) []byte {
	m := map[string]any{"jsonrpc": "2.0", "method": method}
	if id > 0 {
		m["id"] = id
	}
	if params != nil {
		m["params"] = params
	}
	data, _ := json.Marshal(m)
	return data
}

var initialize = map[string]any{
	"protocolVersion": protocolVersion,
	"capabilities":    map[string]any{},
	"clientInfo":      map[string]any{"name": "enowx", "version": "1"},
}

// Test runs srv and lists its tools.
func Test(ctx context.Context, srv envsync.MCPServer) proto.MCPTest {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	start := time.Now()
	var res proto.MCPTest
	var err error
	if srv.URL != "" {
		res, err = testHTTP(ctx, srv)
	} else {
		res, err = testStdio(ctx, srv)
	}
	res.Took = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			res.Error = fmt.Sprintf("no answer within %s", Timeout)
		}
	} else {
		res.OK = true
	}
	return res
}

// testStdio starts the server's command and speaks to it over its stdin
// and stdout, a JSON message a line.
func testStdio(ctx context.Context, srv envsync.MCPServer) (proto.MCPTest, error) {
	var res proto.MCPTest
	cmd := exec.CommandContext(ctx, srv.Command, srv.Args...)
	cmd.Env = os.Environ()
	for k, v := range srv.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return res, fmt.Errorf("can't run %s: %w", srv.Command, err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		res.Stderr = tail(stderr.String())
	}()
	lines := bufio.NewScanner(stdout)
	lines.Buffer(make([]byte, 1<<20), 16<<20)
	call := func(id int, method string, params any) (json.RawMessage, error) {
		if _, err := stdin.Write(append(request(id, method, params), '\n')); err != nil {
			return nil, err
		}
		for lines.Scan() {
			var r rpcResponse
			if json.Unmarshal(lines.Bytes(), &r) != nil || r.ID == nil || *r.ID != id {
				continue // a log line, a notification
			}
			if r.Error != nil {
				return nil, errors.New(r.Error.Message)
			}
			return r.Result, nil
		}
		if err := lines.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("it exited before answering")
	}
	if err := handshake(&res, call, func() { _, _ = stdin.Write(append(request(0, "notifications/initialized", nil), '\n')) }); err != nil {
		res.Stderr = tail(stderr.String())
		return res, err
	}
	return res, nil
}

// testHTTP speaks to a server at its URL (Streamable HTTP): each message
// a POST, answered as JSON or as an event stream.
func testHTTP(ctx context.Context, srv envsync.MCPServer) (proto.MCPTest, error) {
	var res proto.MCPTest
	session := ""
	post := func(body []byte) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		for k, v := range srv.Headers {
			req.Header.Set(k, v)
		}
		return http.DefaultClient.Do(req)
	}
	call := func(id int, method string, params any) (json.RawMessage, error) {
		resp, err := post(request(id, method, params))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if s := resp.Header.Get("Mcp-Session-Id"); s != "" {
			session = s
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(tail(string(body))))
		}
		for _, msg := range messages(resp.Header.Get("Content-Type"), body) {
			var r rpcResponse
			if json.Unmarshal(msg, &r) != nil || r.ID == nil || *r.ID != id {
				continue
			}
			if r.Error != nil {
				return nil, errors.New(r.Error.Message)
			}
			return r.Result, nil
		}
		return nil, errors.New("no answer in the response")
	}
	err := handshake(&res, call, func() {
		if resp, err := post(request(0, "notifications/initialized", nil)); err == nil {
			resp.Body.Close()
		}
	})
	return res, err
}

// messages are the JSON-RPC messages of an HTTP response: its body, or
// the data of its events.
func messages(contentType string, body []byte) [][]byte {
	if !strings.HasPrefix(contentType, "text/event-stream") {
		return [][]byte{body}
	}
	var out [][]byte
	for _, line := range strings.Split(string(body), "\n") {
		if data, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "data:"); ok {
			out = append(out, []byte(strings.TrimSpace(data)))
		}
	}
	return out
}

// handshake opens the session and lists the tools, filling res.
func handshake(res *proto.MCPTest, call func(int, string, any) (json.RawMessage, error), initialized func()) error {
	raw, err := call(1, "initialize", initialize)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	var info struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(raw, &info)
	res.Server = strings.TrimSpace(info.ServerInfo.Name + " " + info.ServerInfo.Version)
	res.Protocol = info.ProtocolVersion
	initialized()
	raw, err = call(2, "tools/list", map[string]any{})
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	var tools struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	for _, t := range tools.Tools {
		res.Tools = append(res.Tools, proto.MCPTool{Name: t.Name, Description: firstLine(t.Description)})
	}
	return nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

func tail(s string) string {
	if len(s) > maxStderr {
		s = "…" + s[len(s)-maxStderr:]
	}
	return s
}
