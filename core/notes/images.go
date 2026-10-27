package notes

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"github.com/enowx/enowx/proto"
)

// AddImage validates and stores an image belonging to an existing note.
func (s *Store) AddImage(id, name string, data []byte) (string, error) {
	if len(data) == 0 || len(data) > proto.MaxNoteImage {
		return "", fmt.Errorf("notes: choose an image up to 8 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
		return "", fmt.Errorf("notes: choose a PNG, JPEG, GIF, WebP or BMP image up to 40 megapixels")
	}
	switch format {
	case "png", "jpeg", "gif", "webp", "bmp":
	default:
		return "", fmt.Errorf("notes: this image format is not supported")
	}
	// The header is checked, not the pixels: decoding a 40-megapixel
	// image takes hundreds of MB, from a file of a few hundred KB, on a
	// server that may have little memory. The app shows what it can.
	// Use the decoded format so a misleading extension still renders correctly.
	name = assetName(name)
	name = strings.TrimSuffix(name, filepath.Ext(name)) + "." + format
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(id, false); err != nil {
		return "", err
	}
	return s.SaveAsset(id, name, data)
}
