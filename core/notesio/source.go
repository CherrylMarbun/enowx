package notesio

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// maxFile bounds a file read from a source, and maxSource all of them:
// what is imported is data from anywhere, never trusted to be small.
const (
	maxFile   = 64 << 20
	maxSource = 2 << 30
)

// source is the files of what is imported, by slash path, read when asked.
type source struct {
	name  string // the file or folder's name
	files []string
	read  func(name string) ([]byte, error)
	total int64
	close func()
}

func (s *source) has(name string) bool {
	_, ok := slices.BinarySearch(s.files, name)
	return ok
}

// open reads the file or folder at p: a folder's files, a .zip's (and
// those of the .zip files a .zip holds alone, as Notion's large exports
// are), or the one file.
func open(p string) (*source, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}
	s := &source{name: filepath.Base(p), close: func() {}}
	switch {
	case info.IsDir():
		root := os.DirFS(p)
		err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&fs.ModeSymlink != 0 {
				return nil // never followed out of the folder
			}
			if d.IsDir() {
				if name != "." && strings.HasPrefix(d.Name(), ".") && d.Name() != ".obsidian" {
					return fs.SkipDir
				}
				return nil
			}
			if fi, err := d.Info(); err == nil {
				s.total += fi.Size()
			}
			s.files = append(s.files, name)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("import: %w", err)
		}
		s.read = func(name string) ([]byte, error) { return readLimited(root, name) }
	case strings.EqualFold(filepath.Ext(p), ".zip"):
		f, err := os.Open(p)
		if err != nil {
			return nil, fmt.Errorf("import: %w", err)
		}
		// The archive stays open while the import reads it.
		s.close = func() { f.Close() }
		zr, err := zip.NewReader(f, info.Size())
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("import: %s isn't a .zip: %w", s.name, err)
		}
		files := map[string]*zip.File{}
		var nested [][]byte
		addZip(zr, files)
		if onlyZips(files) {
			for _, zf := range files {
				data, err := readZip(zf)
				if err != nil {
					f.Close()
					return nil, err
				}
				nested = append(nested, data)
			}
			files = map[string]*zip.File{}
			for _, data := range nested {
				inner, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil {
					f.Close()
					return nil, fmt.Errorf("import: %w", err)
				}
				addZip(inner, files)
			}
		}
		for name, zf := range files {
			s.files = append(s.files, name)
			s.total += int64(zf.UncompressedSize64)
		}
		s.read = func(name string) ([]byte, error) {
			zf, ok := files[name]
			if !ok {
				return nil, fs.ErrNotExist
			}
			return readZip(zf)
		}
	default:
		s.files = []string{filepath.Base(p)}
		s.total = info.Size()
		s.read = func(string) ([]byte, error) {
			if info.Size() > maxFile {
				return nil, fmt.Errorf("%s is larger than %d MB", filepath.Base(p), maxFile>>20)
			}
			return os.ReadFile(p)
		}
	}
	if s.total > maxSource {
		s.close()
		return nil, fmt.Errorf("import: %s holds more than %d GB", s.name, maxSource>>30)
	}
	slices.Sort(s.files)
	return s, nil
}

func addZip(zr *zip.Reader, files map[string]*zip.File) {
	for _, zf := range zr.File {
		name := path.Clean(strings.ReplaceAll(zf.Name, `\`, "/"))
		if zf.FileInfo().IsDir() || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") || name == ".." ||
			strings.HasPrefix(name, "__MACOSX/") || path.Base(name) == ".DS_Store" {
			continue
		}
		files[name] = zf
	}
}

func onlyZips(files map[string]*zip.File) bool {
	if len(files) == 0 {
		return false
	}
	for name := range files {
		if !strings.EqualFold(path.Ext(name), ".zip") {
			return false
		}
	}
	return true
}

func readZip(zf *zip.File) ([]byte, error) {
	if zf.UncompressedSize64 > maxFile {
		return nil, fmt.Errorf("%s is larger than %d MB", zf.Name, maxFile>>20)
	}
	r, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, maxFile+1))
}

func readLimited(root fs.FS, name string) ([]byte, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err == nil && len(data) > maxFile {
		return nil, fmt.Errorf("%s is larger than %d MB", name, maxFile>>20)
	}
	return data, err
}

var notionName = regexp.MustCompile(`^(.*?) ?([0-9a-f]{32})(\.(md|html|csv))?$`)

// detect finds out what kind of source s is.
func detect(s *source) string {
	counts := map[string]int{}
	for _, f := range s.files {
		base := path.Base(f)
		ext := strings.ToLower(path.Ext(base))
		switch {
		case strings.HasPrefix(f, ".obsidian/") || strings.Contains(f, "/.obsidian/"):
			return proto.ImportObsidian
		case base == backupManifest:
			return proto.ImportBackup
		case ext == ".enex":
			return proto.ImportEnex
		case ext == ".json" && strings.Contains("/"+f, "/Keep/"):
			counts[proto.ImportKeep]++
		case ext == ".json":
			counts["json"]++
		}
		if (ext == ".md" || ext == ".html" || ext == ".csv") && notionName.MatchString(strings.TrimSuffix(base, path.Ext(base))+ext) &&
			notionID(base) != "" {
			counts[proto.ImportNotion]++
		}
		switch ext {
		case ".md", ".markdown", ".txt":
			counts[proto.ImportMarkdown]++
		case ".html", ".htm":
			counts[proto.ImportHTML]++
		}
	}
	switch {
	case counts[proto.ImportNotion] > 0:
		return proto.ImportNotion
	case counts[proto.ImportKeep] > 0:
		return proto.ImportKeep
	case len(s.files) == 1 && counts["json"] == 1:
		return proto.ImportBackup
	case counts[proto.ImportHTML] > counts[proto.ImportMarkdown]:
		return proto.ImportHTML
	case len(s.files) == 1 && counts[proto.ImportMarkdown] == 1:
		return proto.ImportMarkdown
	case counts[proto.ImportMarkdown] > 0:
		return proto.ImportObsidian
	}
	return ""
}

// notionID is the page ID a Notion export's file or folder name ends
// with, "" for none.
func notionID(name string) string {
	name = strings.TrimSuffix(name, path.Ext(name))
	if len(name) < 32 {
		return ""
	}
	id := name[len(name)-32:]
	for _, r := range id {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return ""
		}
	}
	return id
}

// Read reads the notes at p, a file or folder on this host.
func Read(ctx context.Context, p string, o Options) (*Batch, error) {
	if o.From == proto.ImportNotionAPI {
		return readNotionAPI(ctx, o)
	}
	s, err := open(p)
	if err != nil {
		return nil, err
	}
	defer s.close()
	from := o.From
	if from == "" {
		if from = detect(s); from == "" {
			return nil, fmt.Errorf("import: %s holds no notes enowX can read (Markdown, HTML, Notion, Evernote, Keep, a backup)", s.name)
		}
	}
	b := &Batch{From: from}
	switch from {
	case proto.ImportNotion:
		err = readNotion(ctx, s, b, o)
	case proto.ImportObsidian, proto.ImportMarkdown:
		err = readMarkdown(ctx, s, b)
	case proto.ImportEnex:
		err = readEnex(ctx, s, b)
	case proto.ImportKeep:
		err = readKeep(ctx, s, b)
	case proto.ImportHTML:
		err = readHTML(ctx, s, b)
	case proto.ImportBackup:
		err = readBackup(s, b)
	default:
		return nil, fmt.Errorf("import: no source %q (notion, notion-api, obsidian, md, enex, keep, html, enowx)", from)
	}
	if err != nil {
		return nil, err
	}
	if len(b.Docs) == 0 {
		why := ""
		if len(b.Skipped) > 0 {
			why = ": " + b.Skipped[0].What
		}
		return nil, errors.New("import: " + s.name + " holds no notes" + why)
	}
	return b, nil
}
