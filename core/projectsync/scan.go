package projectsync

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/core/transfer"
	"github.com/enowx/enowx/proto"
)

// Hasher hashes files, remembering each file's hash while its size and
// modification time stay, so scanning a large project again reads only
// what changed. It is safe for use by several goroutines.
type Hasher struct {
	mu    sync.Mutex
	known map[string]hashed
}

type hashed struct {
	size     int64
	modified time.Time
	hash     string
}

// Files lists the files of the folder dir that a sync carries (as a
// transfer does, without .git), with their hashes; env counts the .env
// files.
func (h *Hasher) Files(dir string, env bool) ([]proto.SyncFile, error) {
	rels, err := transfer.ProjectFiles(dir, env)
	if err != nil {
		return nil, err
	}
	out := make([]proto.SyncFile, 0, len(rels))
	for _, rel := range rels {
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			continue
		}
		f, err := h.File(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			continue // gone, or unreadable: left out
		}
		f.Path = rel
		out = append(out, f)
	}
	return out, nil
}

// File hashes the file at path: its content, or a symlink's target.
func (h *Hasher) File(path string) (proto.SyncFile, error) { return h.hash(path, "") }

// Conversation hashes an agent's conversation at path, recorded in the
// folder dir: the same on two hosts whatever their folders, which a
// transfer rewrites in it.
func (h *Hasher) Conversation(path, dir string) (proto.SyncFile, error) { return h.hash(path, dir) }

func (h *Hasher) hash(path, dir string) (proto.SyncFile, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return proto.SyncFile{}, err
	}
	f := proto.SyncFile{Size: info.Size(), Modified: info.ModTime().UTC()}
	key := path + "\x00" + dir
	h.mu.Lock()
	k, ok := h.known[key]
	h.mu.Unlock()
	if ok && k.size == f.Size && k.modified.Equal(f.Modified) {
		f.Hash = k.hash
		return f, nil
	}
	sum := sha256.New()
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return proto.SyncFile{}, err
		}
		io.WriteString(sum, "link:"+target)
	} else if dir != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return proto.SyncFile{}, err
		}
		sum.Write(transfer.RewritePaths(data, dir, "\x00"))
	} else {
		r, err := os.Open(path)
		if err != nil {
			return proto.SyncFile{}, err
		}
		_, err = io.Copy(sum, r)
		r.Close()
		if err != nil {
			return proto.SyncFile{}, err
		}
	}
	// An executable differs from the same bytes not executable.
	if info.Mode()&0o111 != 0 {
		io.WriteString(sum, "\x00x")
	}
	f.Hash = hex.EncodeToString(sum.Sum(nil))
	h.mu.Lock()
	if h.known == nil {
		h.known = map[string]hashed{}
	}
	h.known[key] = hashed{size: f.Size, modified: f.Modified, hash: f.Hash}
	h.mu.Unlock()
	return f, nil
}
