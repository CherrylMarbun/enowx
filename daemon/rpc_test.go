package daemon

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// Every Method constant in package proto must have a handler, or clients
// calling it get "unknown method" from a daemon that should know it.
func TestEveryMethodHasHandler(t *testing.T) {
	files, err := filepath.Glob("../proto/*.go")
	if err != nil {
		t.Fatal(err)
	}
	methods := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, isValue := n.(*ast.ValueSpec)
			if !isValue {
				return true
			}
			for i, name := range spec.Names {
				if !strings.HasPrefix(name.Name, "Method") || i >= len(spec.Values) {
					continue
				}
				lit, isLit := spec.Values[i].(*ast.BasicLit)
				if !isLit {
					continue
				}
				method, _ := strconv.Unquote(lit.Value)
				methods++
				if _, found := handlers[method]; !found {
					t.Errorf("proto.%s (%q) has no handler", name.Name, method)
				}
			}
			return true
		})
	}
	if methods != len(handlers) {
		t.Errorf("proto declares %d methods, daemon handles %d", methods, len(handlers))
	}
}

// A newer app connects at the daemon's version; one older than the oldest
// it speaks is told so.
func TestHandshakeNegotiatesTheVersion(t *testing.T) {
	srv := startServer(t)
	hello := func(v int) (proto.Hello, error) {
		a, b := net.Pipe()
		defer b.Close()
		go srv.ServeConn(a)
		payload, _ := json.Marshal(proto.Hello{Protocol: v, Agent: "test"})
		if err := proto.WriteFrame(b, proto.Frame{Type: proto.TypeHello, Payload: payload}); err != nil {
			return proto.Hello{}, err
		}
		_ = b.SetReadDeadline(time.Now().Add(2 * time.Second))
		f, err := proto.ReadFrame(b)
		if err != nil {
			return proto.Hello{}, err
		}
		var h proto.Hello
		err = json.Unmarshal(f.Payload, &h)
		return h, err
	}
	if h, err := hello(proto.Version + 1); err != nil || h.Protocol != proto.Version {
		t.Fatalf("newer app: %+v, %v", h, err)
	}
	if h, err := hello(proto.MinVersion - 1); err == nil {
		t.Fatalf("an app older than the oldest version was answered: %+v", h)
	}
}
