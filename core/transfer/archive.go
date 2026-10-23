package transfer

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Writer writes an archive: a gzipped tar of named entries.
type Writer struct {
	gz *gzip.Writer
	tw *tar.Writer
}

// NewWriter writes an archive to w.
func NewWriter(w io.Writer) *Writer {
	gz := gzip.NewWriter(w)
	return &Writer{gz: gz, tw: tar.NewWriter(gz)}
}

// AddFiles adds the files of dir named in rels (relative, with slashes)
// under prefix, as they are: their mode, and symlinks as links.
func (w *Writer) AddFiles(prefix, dir string, rels []string) error {
	for _, rel := range rels {
		if err := w.addFile(path.Join(prefix, rel), filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	return nil
}

// AddFile adds the file at src as name.
func (w *Writer) AddFile(name, src string) error { return w.addFile(name, src) }

func (w *Writer) addFile(name, src string) error {
	info, err := os.Lstat(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil // gone since it was listed
	}
	if err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(src); err != nil {
			return fmt.Errorf("transfer: %w", err)
		}
	} else if !info.Mode().IsRegular() {
		return nil // sockets, devices: nothing to carry
	}
	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	hdr.Name = name
	hdr.Uname, hdr.Gname, hdr.Uid, hdr.Gid = "", "", 0, 0
	if err := w.tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	if link != "" {
		return nil
	}
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	defer f.Close()
	if _, err := io.CopyN(w.tw, f, hdr.Size); err != nil {
		return fmt.Errorf("transfer: %s: %w", src, err)
	}
	return nil
}

// AddBytes adds data as the file name.
func (w *Writer) AddBytes(name string, data []byte) error {
	if err := w.tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	_, err := w.tw.Write(data)
	return err
}

// Close finishes the archive.
func (w *Writer) Close() error {
	if err := w.tw.Close(); err != nil {
		return err
	}
	return w.gz.Close()
}

// Entry is an entry of an archive being read: a file or a symlink.
type Entry struct {
	Name string      // with slashes, checked to stay inside the archive
	Mode os.FileMode // a file's permissions
	Link string      // a symlink's target; "" for a file
	io.Reader
}

// Read calls fn for each entry of the archive in r, in order.
func Read(r io.Reader, fn func(e Entry) error) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("transfer: %w", err)
		}
		name := path.Clean(hdr.Name)
		if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("transfer: %s leaves the archive", hdr.Name)
		}
		e := Entry{Name: name, Mode: hdr.FileInfo().Mode().Perm(), Reader: tr}
		switch hdr.Typeflag {
		case tar.TypeSymlink:
			e.Link = hdr.Linkname
		case tar.TypeReg:
		default:
			continue
		}
		if err := fn(e); err != nil {
			return err
		}
	}
}

// Extract writes e to dest/rel, rel being its name under a prefix: a
// file with its permissions, or a symlink.
func Extract(e Entry, dest, rel string) error {
	target := filepath.Join(dest, filepath.FromSlash(rel))
	if !strings.HasPrefix(target, filepath.Clean(dest)+string(filepath.Separator)) {
		return fmt.Errorf("transfer: %s leaves %s", rel, dest)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	_ = os.Remove(target) // a file the worktree had replaced
	if e.Link != "" {
		return os.Symlink(e.Link, target)
	}
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, e.Mode|0o200)
	if err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	if _, err := io.Copy(f, e); err != nil {
		f.Close()
		return fmt.Errorf("transfer: %w", err)
	}
	return f.Close()
}
