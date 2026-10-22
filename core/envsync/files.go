package envsync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/enowx/enowx/proto"
)

func joinHome(home, rel string) string { return filepath.Join(home, filepath.FromSlash(rel)) }

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// scanEntries lists the entries of a source folder: every visible file
// or folder, or for skills every folder with a SKILL.md.
func scanEntries(home string, s source) []proto.EnvItem {
	dir := s.location(home)
	entries, err := os.ReadDir(joinHome(home, dir))
	if err != nil {
		return nil
	}
	var items []proto.EnvItem
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !validName(name) {
			continue
		}
		rel := path.Join(dir, name)
		info, err := os.Stat(joinHome(home, rel)) // follows links, as agents do
		if err != nil {
			continue
		}
		if s.layout == layoutSkills && (!info.IsDir() || !exists(joinHome(home, path.Join(rel, "SKILL.md")))) {
			continue
		}
		items = append(items, fileItem(home, s, name, rel))
	}
	return items
}

func scanFile(home string, s source) []proto.EnvItem {
	rel := s.location(home)
	if info, err := os.Stat(joinHome(home, rel)); err != nil || info.IsDir() {
		return nil
	}
	return []proto.EnvItem{fileItem(home, s, path.Base(rel), rel)}
}

// fileItem describes the file or folder at rel.
func fileItem(home string, s source, name, rel string) proto.EnvItem {
	it := newItem(s, name, tilde(rel))
	files, err := readTree(home, rel)
	if err != nil {
		it.Warnings = append(it.Warnings, err.Error())
		it.Hash = hashParts([]byte(err.Error()))
		return it
	}
	var parts [][]byte
	for _, f := range files {
		it.Size += int64(len(f.Data))
		data := f.Data
		if isText(data) {
			data = []byte(rewriteHome(string(data), home, homeToken))
		}
		parts = append(parts, []byte(strings.TrimPrefix(f.Path, rel)), []byte{byte(f.Mode & 0o111)}, data)
		if why := secretFile(f); why != "" {
			it.Secrets = append(it.Secrets, why)
		}
	}
	if s.kind == proto.EnvSettings {
		it.Secrets = append(it.Secrets, settingsSecrets(files)...)
	}
	if it.Size > maxItemBytes {
		it.Warnings = append(it.Warnings, fmt.Sprintf("too large to sync (%d MB)", it.Size>>20))
	}
	it.Hash = hashParts(parts...)
	return it
}

// readTree reads the file at rel, or every file under the folder at rel,
// in a stable order. Links are followed; their targets are copied.
func readTree(home, rel string) ([]proto.EnvFile, error) {
	root := joinHome(home, rel)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		f, err := readFile(root, rel, info)
		if err != nil {
			return nil, err
		}
		return []proto.EnvFile{f}, nil
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	var files []proto.EnvFile
	var total int64
	err = filepath.WalkDir(real, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if p != real && (name == ".git" || name == "node_modules" || name == ".DS_Store") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() {
			return nil // broken links, sockets
		}
		sub, _ := filepath.Rel(real, p)
		f, err := readFile(p, path.Join(rel, filepath.ToSlash(sub)), info)
		if err != nil {
			return err
		}
		total += int64(len(f.Data))
		if total > maxItemBytes {
			return fmt.Errorf("too large to sync (over %d MB)", maxItemBytes>>20)
		}
		files = append(files, f)
		return nil
	})
	slices.SortFunc(files, func(a, b proto.EnvFile) int { return strings.Compare(a.Path, b.Path) })
	return files, err
}

func readFile(p, rel string, info fs.FileInfo) (proto.EnvFile, error) {
	if info.Size() > maxFileBytes {
		return proto.EnvFile{}, fmt.Errorf("%s is too large to sync", tilde(rel))
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return proto.EnvFile{}, err
	}
	return proto.EnvFile{Path: rel, Mode: uint32(info.Mode().Perm()), Data: data}, nil
}

func exportFiles(home string, s source, name string) ([]proto.EnvFile, error) {
	rel := itemPath(home, s, name)
	files, err := readTree(home, rel)
	if err != nil {
		return nil, fmt.Errorf("envsync: %s: %w", tilde(rel), err)
	}
	return files, nil
}

// itemPath is where the item called name lives under home.
func itemPath(home string, s source, name string) string {
	if s.layout == layoutFile {
		return s.location(home)
	}
	return path.Join(s.location(home), name)
}

// importFiles replaces the item with the payload's files, which must all
// lie inside the item.
func importFiles(home string, s source, name string, p proto.EnvPayload) (proto.EnvImportResult, error) {
	var res proto.EnvImportResult
	srcRel := ""
	if len(p.Files) > 0 {
		srcRel = p.Files[0].Path
	}
	// The exporting host may keep the source at another of its paths.
	from := ""
	for _, alt := range s.paths {
		if s.layout == layoutFile && srcRel == alt {
			from = alt
		} else if x := path.Join(alt, name); s.layout != layoutFile && (srcRel == x || strings.HasPrefix(srcRel, x+"/")) {
			from = x
		}
	}
	if from == "" && len(p.Files) > 0 {
		return res, fmt.Errorf("envsync: %s: files outside the item", p.Item.ID)
	}
	to := itemPath(home, s, name)
	files := make([]proto.EnvFile, 0, len(p.Files))
	for _, f := range p.Files {
		clean := path.Clean(f.Path)
		if clean != from && !strings.HasPrefix(clean, from+"/") {
			return res, fmt.Errorf("envsync: %s: %s is outside the item", p.Item.ID, f.Path)
		}
		f.Path = to + strings.TrimPrefix(clean, from)
		if isText(f.Data) {
			f.Data = []byte(rewriteHome(string(f.Data), p.Home, home))
		}
		files = append(files, f)
	}
	if s.layout == layoutFile {
		if len(files) != 1 {
			return res, fmt.Errorf("envsync: %s: want one file", p.Item.ID)
		}
		return res, writeFileAtomic(joinHome(home, to), files[0].Data, fs.FileMode(files[0].Mode)&0o777|0o600)
	}
	return res, replaceTree(home, to, files)
}

// replaceTree writes files into a fresh folder next to rel, then swaps it
// in, so a failed import leaves the old item whole.
func replaceTree(home, rel string, files []proto.EnvFile) error {
	dst := joinHome(home, rel)
	single := len(files) == 1 && files[0].Path == rel
	if single {
		f := files[0]
		if info, err := os.Lstat(dst); err == nil && info.IsDir() {
			if err := os.RemoveAll(dst); err != nil {
				return err
			}
		}
		return writeFileAtomic(dst, f.Data, fs.FileMode(f.Mode)&0o777|0o600)
	}
	tmp := dst + ".enowx-tmp"
	_ = os.RemoveAll(tmp)
	for _, f := range files {
		p := filepath.Join(tmp, filepath.FromSlash(strings.TrimPrefix(f.Path, rel+"/")))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, f.Data, fs.FileMode(f.Mode)&0o777|0o600); err != nil {
			_ = os.RemoveAll(tmp)
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// A link (a skill kept in a repository, say) is replaced, not followed.
	if err := os.RemoveAll(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(tmp, dst)
}

func writeFileAtomic(p string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".enowx-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// isText reports whether data looks like text, where home paths are moved.
func isText(data []byte) bool {
	return utf8.Valid(data) && !bytes.ContainsRune(data, 0)
}
