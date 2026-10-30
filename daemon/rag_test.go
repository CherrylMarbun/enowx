package daemon

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

// ragEmbeddings is an OpenAI-compatible embeddings endpoint on this
// machine: a text's words hashed into 128 buckets.
func ragEmbeddings(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Input []string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		type item struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}
		var out struct {
			Data []item `json:"data"`
		}
		for i, text := range in.Input {
			v := make([]float32, 128)
			for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !(r >= 'a' && r <= 'z') }) {
				h := fnv.New32a()
				h.Write([]byte(w))
				v[h.Sum32()%128]++
			}
			out.Data = append(out.Data, item{Embedding: v, Index: i})
		}
		json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/v1"
}

// A project is indexed only once turned on and agreed to; a session's
// agent searches its own project by meaning, keeps a memory, and its
// prompt gets the parts that bear on it.
func TestRAGThroughTheDaemon(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv, sess := hookServer(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "billing"), 0o755)
	os.WriteFile(filepath.Join(dir, "billing", "invoice.go"), []byte("package billing\n\n// Total sums an invoice's lines with tax.\nfunc Total(lines []float64, tax float64) float64 {\n\treturn 0\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Shop\n\nOrders and refunds.\n"), 0o644)
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git: %s", out)
	}
	if err := srv.ragSettings.Set(proto.RAGSettings{Embedder: proto.RAGOpenAI, Model: "fake", BaseURL: ragEmbeddings(t)}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.ragIndex(dir); err == nil {
		t.Fatal("indexed while off")
	}
	st, _ := srv.setRAGProject(proto.RAGProjectParams{Project: dir, Settings: proto.RAGProject{On: true, Context: true}})
	if _, err := srv.ragIndex(dir); err == nil || !strings.Contains(err.Error(), "agree") {
		t.Fatalf("indexed without consent: %v", err)
	}
	if !strings.Contains(st.Embedder, "this device") {
		t.Fatalf("embedder host %q", st.Embedder)
	}
	srv.setRAGProject(proto.RAGProjectParams{Project: dir, Settings: proto.RAGProject{On: true, Context: true, Consent: st.Embedder}})
	if _, err := srv.ragIndex(dir); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the index", func() bool { s := srv.ragStatus(dir); return !s.Indexing && s.Chunks >= 2 })

	// The session's agent: its own project only.
	sess.Info()
	inProject, err := srv.sessions.Create(session.Spec{Command: []string{"sh", "-c", "cat"}, Dir: dir, Cols: 80, Rows: 24, Meta: session.Meta{Project: dir}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(session.EnvSession, inProject.ID())
	info := inProject.Info()
	hits, err := ragSearch(srv, proto.RAGSearchParams{RAGRef: proto.RAGRef{Session: info.ID}, Query: "invoice total tax"})
	if err != nil || len(hits) == 0 || hits[0].File != "billing/invoice.go" {
		t.Fatalf("search: %+v %v", hits, err)
	}
	if _, err := ragSearch(srv, proto.RAGSearchParams{RAGRef: proto.RAGRef{Session: info.ID, Project: "/elsewhere"}, Query: "x"}); err == nil {
		t.Fatal("a session searched another project")
	}
	code, ctx, _ := hookOf(`{"prompt":"fix the invoice total tax"}`, "--event", proto.HookPromptStart)
	if code != 0 || !strings.Contains(ctx, "billing/invoice.go") {
		t.Fatalf("prompt context: %q", ctx)
	}
	m, err := srv.ragSettings.Remember("x", "y", "z")
	if err != nil || m.ID == "" {
		t.Fatal(err)
	}
}

func ragSearch(srv *Server, p proto.RAGSearchParams) ([]proto.RAGHit, error) {
	dir, err := srv.ragDir(p.RAGRef)
	if err != nil {
		return nil, err
	}
	return srv.rag.Search(context.Background(), dir, p.Query, p.K, p.Scope)
}
