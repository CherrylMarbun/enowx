package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	ragconfig "github.com/enowdev/enowx-rag/mcp-server/pkg/config"
	ragcore "github.com/enowdev/enowx-rag/mcp-server/pkg/core"
	"github.com/enowdev/enowx-rag/mcp-server/pkg/indexer"
	erag "github.com/enowdev/enowx-rag/mcp-server/pkg/rag"
	"github.com/enowdev/enowx-rag/mcp-server/pkg/ragbuild"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/core/projectlinks"
	"github.com/enowx/enowx/proto"
)

// ErrNotReady is an engine with no embedder set: nothing is indexed.
var ErrNotReady = errors.New("rag: no embedder set: Settings → RAG")

// chunkSize is the characters of a chunk, as enowx-rag cuts them.
const chunkSize = 1000

// defaultMaxFiles bounds a project's files indexed.
const defaultMaxFiles = 20000

// Engine is a host's RAG: its provider built from the settings once they
// change, keys asked of the vault (KeyOf). Safe from any goroutine.
type Engine struct {
	dir      string
	settings *Settings
	// KeyOf is the value of the vault's credential named cred.
	KeyOf func(cred string) (string, error)

	mu     sync.Mutex
	sig    string
	prov   erag.Provider
	svc    *ragcore.Service
	remote *server
}

// NewEngine is the RAG of the daemon in dir, by s.
func NewEngine(dir string, s *Settings) *Engine { return &Engine{dir: dir, settings: s} }

// signature is what the provider is built of: the settings but the
// projects'.
func signature(st proto.RAGSettings) string {
	st.Projects = nil
	data, _ := json.Marshal(st)
	return string(data)
}

// ready is the provider and service for the settings now, built again
// when they changed.
func (e *Engine) ready(ctx context.Context) (erag.Provider, *ragcore.Service, *server, error) {
	st := e.settings.Get()
	if st.Embedder == "" && st.Store != proto.RAGServer {
		return nil, nil, nil, ErrNotReady
	}
	sig := signature(st)
	e.mu.Lock()
	defer e.mu.Unlock()
	if sig == e.sig && (e.svc != nil || e.remote != nil) {
		return e.prov, e.svc, e.remote, nil
	}
	if e.prov != nil {
		e.prov.Close()
	}
	e.prov, e.svc, e.remote, e.sig = nil, nil, nil, ""
	if st.Store == proto.RAGServer {
		token := ""
		if st.ServerCred != "" && e.KeyOf != nil {
			t, err := e.KeyOf(st.ServerCred)
			if err != nil {
				return nil, nil, nil, err
			}
			token = t
		}
		e.remote, e.sig = &server{url: strings.TrimRight(st.ServerURL, "/"), token: token}, sig
		return nil, nil, e.remote, nil
	}
	prov, err := e.build(ctx, st)
	if err != nil {
		return nil, nil, nil, err
	}
	e.prov, e.svc, e.sig = prov, ragcore.NewService(prov, nil, indexer.NewIndexer(prov, chunkSize)), sig
	return e.prov, e.svc, nil, nil
}

// spec is what st builds: its embedder (its key asked of the vault) and
// store.
func (e *Engine) spec(st proto.RAGSettings) (ragbuild.Spec, error) {
	var spec ragbuild.Spec
	var cfg *ragconfig.Config
	if st.Embedder == proto.RAGFromConfig || st.Store == proto.RAGFromConfig {
		c, err := ragconfig.Resolve()
		if err != nil {
			return spec, fmt.Errorf("rag: enowx-rag's settings: %w", err)
		}
		cfg = c
	}
	key := func() (string, error) {
		if st.KeyCred == "" {
			return "", nil
		}
		if e.KeyOf == nil {
			return "", errors.New("rag: no vault to read the key from")
		}
		return e.KeyOf(st.KeyCred)
	}
	switch st.Embedder {
	case proto.RAGFromConfig:
		spec.Embedder, spec.TEIURL = cfg.Embedder, cfg.TEIURL
		spec.VoyageAPIKey, spec.VoyageModel, spec.VoyageDim = cfg.Voyage.APIKey, cfg.Voyage.Model, cfg.Voyage.Dim
		spec.OpenAIAPIKey, spec.OpenAIModel, spec.OpenAIBaseURL, spec.OpenAIDim = cfg.OpenAI.APIKey, cfg.OpenAI.Model, cfg.OpenAI.BaseURL, cfg.OpenAI.Dim
	case proto.RAGVoyage:
		k, err := key()
		if err != nil {
			return spec, err
		}
		spec.Embedder, spec.VoyageAPIKey, spec.VoyageModel, spec.VoyageDim = "voyage", k, orString(st.Model, "voyage-4"), orInt(st.Dim, 1024)
	case proto.RAGOpenAI:
		k, err := key()
		if err != nil {
			return spec, err
		}
		spec.Embedder, spec.OpenAIAPIKey, spec.OpenAIModel, spec.OpenAIBaseURL, spec.OpenAIDim = "openai", k, st.Model, st.BaseURL, st.Dim
	case proto.RAGTEI:
		spec.Embedder, spec.TEIURL = "tei", st.BaseURL
	}
	switch st.Store {
	case proto.RAGFromConfig:
		spec.VectorStore, spec.QdrantURL, spec.QdrantAPIKey, spec.ChromaURL, spec.PGVectorDSN = cfg.VectorStore, cfg.QdrantURL, cfg.QdrantAPIKey, cfg.ChromaURL, cfg.PGVectorDSN
		if cfg.VectorStore == "sqlite" {
			spec.SQLitePath = cfg.StorePath()
		}
	default:
		// One file per embedder: another's vectors don't compare.
		sum := sha256.Sum256([]byte(spec.Embedder + "|" + spec.VoyageModel + spec.OpenAIModel + spec.OpenAIBaseURL + spec.TEIURL + fmt.Sprint(spec.VoyageDim, spec.OpenAIDim)))
		spec.VectorStore, spec.SQLitePath = "sqlite", filepath.Join(e.dir, "rag", "index-"+hex.EncodeToString(sum[:6])+".db")
	}
	return spec, nil
}

func (e *Engine) build(ctx context.Context, st proto.RAGSettings) (erag.Provider, error) {
	spec, err := e.spec(st)
	if err != nil {
		return nil, err
	}
	prov, err := ragbuild.BuildProvider(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("rag: %w", err)
	}
	return prov, nil
}

// Test embeds a line with settings st (not kept), and says the vector's
// size.
func (e *Engine) Test(ctx context.Context, st proto.RAGSettings) (int, error) {
	if st.Store == proto.RAGServer {
		srv := &server{url: strings.TrimRight(st.ServerURL, "/")}
		if st.ServerCred != "" && e.KeyOf != nil {
			t, err := e.KeyOf(st.ServerCred)
			if err != nil {
				return 0, err
			}
			srv.token = t
		}
		return 0, srv.ping(ctx)
	}
	spec, err := e.spec(st)
	if err != nil {
		return 0, err
	}
	emb, err := ragbuild.BuildEmbedder(spec)
	if err != nil {
		return 0, err
	}
	v, err := emb.Embed(ctx, []string{"enowX checks the embedder"})
	if err != nil {
		return 0, err
	}
	if len(v) == 0 {
		return 0, errors.New("rag: the embedder gave no vector")
	}
	return len(v[0]), nil
}

// EmbedderHost is where st's embedder sends text, for the user to agree
// to: a host, or "this device".
func (e *Engine) EmbedderHost(st proto.RAGSettings) string {
	spec, err := e.spec(st)
	if err != nil && st.Embedder != proto.RAGFromConfig {
		return st.Embedder
	}
	host := func(raw, fallback string) string {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			if h := u.Hostname(); h == "localhost" || h == "127.0.0.1" || h == "::1" {
				return "this device (" + u.Host + ")"
			}
			return u.Host
		}
		return fallback
	}
	switch strings.ToLower(spec.Embedder) {
	case "voyage":
		return "api.voyageai.com"
	case "openai":
		return host(spec.OpenAIBaseURL, "api.openai.com")
	case "tei":
		return host(spec.TEIURL, "a TEI server")
	}
	if st.Store == proto.RAGServer {
		return host(st.ServerURL, "an enowx-rag server")
	}
	return st.Embedder
}

// Index indexes the project in dir as p says, with extra (its notes and
// memories): its files by enowx-rag's indexer, the extra apart; what
// changed is embedded again, what went is removed.
func (e *Engine) Index(ctx context.Context, dir string, p proto.RAGProject, extra []erag.Document) (proto.RAGStatus, error) {
	id := ProjectID(dir)
	prov, _, remote, err := e.ready(ctx)
	if err != nil {
		return proto.RAGStatus{}, err
	}
	if remote != nil {
		return remote.reindex(ctx, id, dir)
	}
	if err := prov.CreateCollection(ctx, id); err != nil {
		return proto.RAGStatus{}, err
	}
	idx := indexer.NewIndexer(prov, chunkSize)
	idx.Files = func(root string) ([]string, error) {
		return Files(root, p.Exclude, orInt(p.MaxFiles, defaultMaxFiles)), nil
	}
	res, err := idx.IndexProject(ctx, id, dir)
	if err != nil {
		return proto.RAGStatus{}, err
	}
	out := proto.RAGStatus{Project: dir, ID: id, Added: res.Indexed, Removed: res.Deleted, Skipped: res.Skipped, Files: res.FilesScanned}
	if res.StaleError != "" {
		out.Error = res.StaleError
	}
	added, removed, err := e.indexExtra(ctx, prov, id, extra)
	if err != nil {
		return out, err
	}
	out.Added, out.Removed = out.Added+added, out.Removed+removed
	if c, ok := prov.(ragcore.ProjectCounter); ok {
		out.Chunks, _ = c.CountPoints(ctx, id)
	}
	if tc, ok := prov.(erag.TokenCounter); ok {
		out.Tokens = tc.TokensUsed()
	}
	return out, nil
}

// extraDir tags the points that are notes and memories, apart from files.
const extraDir = "enowx-extra"

// indexExtra embeds the docs of extra that changed, and removes those
// gone.
func (e *Engine) indexExtra(ctx context.Context, prov erag.Provider, id string, extra []erag.Document) (added, removed int, err error) {
	have, err := prov.ListPoints(ctx, id, map[string]string{"source_dir": extraDir})
	if err != nil {
		return 0, 0, err
	}
	hashes := map[string]string{}
	for _, pt := range have {
		hashes[pt.DocID] = pt.ContentHash
	}
	var docs []erag.Document
	keep := map[string]bool{}
	for _, d := range extra {
		sum := sha256.Sum256([]byte(d.Content))
		h := hex.EncodeToString(sum[:8])
		keep[d.ID] = true
		if hashes[d.ID] == h {
			continue
		}
		meta := map[string]string{"source_dir": extraDir, "content_hash": h}
		for k, v := range d.Meta {
			meta[k] = v
		}
		docs = append(docs, erag.Document{ID: d.ID, Content: d.Content, Meta: meta})
	}
	for i := 0; i < len(docs); i += 100 {
		if err := prov.Index(ctx, id, docs[i:min(len(docs), i+100)]); err != nil {
			return added, 0, err
		}
		added += min(len(docs), i+100) - i
	}
	var gone []string
	for _, pt := range have {
		if !keep[pt.DocID] {
			gone = append(gone, pt.ID)
		}
	}
	if len(gone) > 0 {
		err = prov.DeletePoints(ctx, id, gone)
	}
	return added, len(gone), err
}

// Search finds query in the project in dir: k results (5 when 0), of
// scope's sources (all when none).
func (e *Engine) Search(ctx context.Context, dir, query string, k int, scope []string) ([]proto.RAGHit, error) {
	if k <= 0 {
		k = 5
	}
	id := ProjectID(dir)
	_, svc, remote, err := e.ready(ctx)
	if err != nil {
		return nil, err
	}
	want := k
	if len(scope) > 0 {
		want = k * 4
	}
	var results []erag.Result
	if remote != nil {
		results, err = remote.search(ctx, id, query, want)
	} else {
		results, err = svc.Search(ctx, id, query, ragcore.SearchOpts{K: want, Hybrid: true, Compress: true})
	}
	if err != nil {
		return nil, err
	}
	var out []proto.RAGHit
	for _, r := range results {
		h := hitOf(r)
		if len(scope) > 0 && !slices.Contains(scope, h.Source) {
			continue
		}
		out = append(out, h)
		if len(out) == k {
			break
		}
	}
	return out, nil
}

// hitOf is result r as a hit: its file and lines, its snippet without
// the header the indexer put first.
func hitOf(r erag.Result) proto.RAGHit {
	h := proto.RAGHit{File: r.Meta["source_file"], Symbol: r.Meta["symbol"], Score: r.Score, Source: r.Meta["source"]}
	fmt.Sscan(r.Meta["start_line"], &h.Start)
	fmt.Sscan(r.Meta["end_line"], &h.End)
	text := r.Content
	if strings.HasPrefix(text, "File: ") {
		if _, rest, ok := strings.Cut(text, "\n\n"); ok {
			text = rest
		}
	}
	if len(text) > 800 {
		text = text[:800] + "…"
	}
	h.Snippet = text
	if h.Source == "" {
		h.Source = SourceOf(h.File)
	}
	if h.Source == proto.RAGMemo {
		h.Memory = strings.TrimPrefix(r.Meta["doc_id"], "memory:")
	}
	return h
}

// SourceOf is what a project's file is: docs (Markdown, its docs folder,
// the agents' and the brief's) or code.
func SourceOf(file string) string {
	l := strings.ToLower(file)
	switch {
	case l == "":
		return ""
	case strings.HasSuffix(l, ".md") || strings.HasSuffix(l, ".mdx") || strings.HasSuffix(l, ".txt"),
		strings.HasPrefix(l, "docs/"), l == ".enowx/brief.md":
		return proto.RAGDocs
	}
	return proto.RAGCode
}

// Status is the index of the project in dir: its chunks.
func (e *Engine) Status(ctx context.Context, dir string) (proto.RAGStatus, error) {
	out := proto.RAGStatus{Project: dir, ID: ProjectID(dir)}
	prov, _, remote, err := e.ready(ctx)
	if err != nil {
		return out, err
	}
	out.Ready = true
	if remote != nil {
		out.Chunks, _ = remote.count(ctx, out.ID)
		return out, nil
	}
	if c, ok := prov.(ragcore.ProjectCounter); ok {
		out.Chunks, _ = c.CountPoints(ctx, out.ID)
	}
	return out, nil
}

// Drop deletes the index of the project in dir.
func (e *Engine) Drop(ctx context.Context, dir string) error {
	prov, _, remote, err := e.ready(ctx)
	if err != nil || remote != nil {
		return err
	}
	return prov.DeleteCollection(ctx, ProjectID(dir))
}

// Files are the files of the project in root to index, relative with
// slashes: git's (its ignores honored), else a walk; but those matching
// exclude, at most max.
func Files(root string, exclude []string, max int) []string {
	var files []string
	if out, err := git.Run(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard"); err == nil {
		for _, f := range strings.Split(out, "\x00") {
			if f != "" {
				files = append(files, f)
			}
		}
	} else {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() && (strings.HasPrefix(name, ".") && p != root || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "target") {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				if rel, err := filepath.Rel(root, p); err == nil {
					files = append(files, filepath.ToSlash(rel))
				}
			}
			return nil
		})
	}
	out := files[:0]
	for _, f := range files {
		if slices.ContainsFunc(exclude, func(pattern string) bool { return projectlinks.Match(pattern, f) }) {
			continue
		}
		out = append(out, f)
		if max > 0 && len(out) == max {
			break
		}
	}
	return out
}

// Plan is what indexing the project in dir sends: its files and roughly
// their tokens, and what never goes.
func (e *Engine) Plan(dir string, p proto.RAGProject) proto.RAGPlan {
	st := e.settings.Get()
	out := proto.RAGPlan{Embedder: st.Embedder, Host: e.EmbedderHost(st)}
	secrets, excluded := 0, len(Files(dir, nil, 0))
	files := Files(dir, p.Exclude, orInt(p.MaxFiles, defaultMaxFiles))
	excluded -= len(files)
	var bytes int64
	for _, f := range files {
		if indexer.IsSecret(f) {
			secrets++
			continue
		}
		if !textFile(f) {
			continue
		}
		if fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err == nil && fi.Size() <= 500<<10 {
			out.Files++
			bytes += fi.Size()
		}
	}
	out.Tokens = int(bytes / 4)
	out.Left = []string{fmt.Sprintf("%d files that hold secrets (.env, keys, certificates), and any chunk holding a token", secrets),
		"what .gitignore ignores, and files over 500 KB or not text"}
	if excluded > 0 {
		out.Left = append(out.Left, fmt.Sprintf("%d files you left out", excluded))
	}
	return out
}

// textFile says file is one enowx-rag indexes, by its kind.
func textFile(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt", ".swift", ".c", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php",
		".vue", ".svelte", ".sql", ".sh", ".bash", ".zsh", ".yml", ".yaml", ".toml", ".json", ".xml", ".html", ".css", ".scss",
		".md", ".txt", ".cfg", ".ini", ".conf", ".dockerfile", ".proto", ".graphql", ".gql":
		return true
	}
	switch strings.ToLower(path.Base(file)) {
	case "dockerfile", "makefile", "license", ".gitignore":
		return true
	}
	return false
}

func orString(s, or string) string {
	if s == "" {
		return or
	}
	return s
}

func orInt(n, or int) int {
	if n <= 0 {
		return or
	}
	return n
}
