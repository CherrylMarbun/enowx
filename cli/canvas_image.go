package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// A design's own pictures, made or chosen by the canvas's agents (which
// write no files): an SVG it drew, or a picture found on the web, kept as
// the canvas's asset for its designs to use.

const imageUsage = `image CANVAS --name NAME (--svg TEXT|- | --url URL)   keep a picture for the canvas's designs: an SVG drawn, or one from the web;
                                   shapes show it as --src NAME, HTML as assets/NAME`

// maxWebImage is the largest picture taken from the web.
const maxWebImage = 10 << 20

var imageTools = mcpTool{name: "canvas_image", description: "Keep a picture for a canvas's designs, made for them: an SVG illustration you draw (svg), or a picture chosen from the web (url, https). Answers the name to use: HTML as assets/NAME, a shape as src. Make each direction's pictures in its own style; never gray boxes.",
	props:    map[string]mcpProp{"canvas": canvasProp, "name": {"string", "Its name, e.g. hero-ledger.svg"}, "svg": {"string", "The SVG, <svg …>…</svg>"}, "url": {"string", "An https URL of a PNG, JPEG, WebP or SVG"}},
	required: []string{"canvas", "name"},
	args: func(a map[string]any) []string {
		return withFlag(withFlag(withFlag([]string{"image", str(a, "canvas")}, a, "name", "name"), a, "svg", "svg"), a, "url", "url")
	}}

func (r runner) canvasImage(args []string) error {
	fs := flag.NewFlagSet("image", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "")
	svg := fs.String("svg", "", "")
	url := fs.String("url", "", "")
	pos, err := parseAll(fs, args)
	if err != nil || len(pos) != 1 || *name == "" || (*svg == "") == (*url == "") {
		return errors.New("usage: enowx-app canvas " + imageUsage)
	}
	c, err := r.findCanvas(pos[0])
	if err != nil {
		return err
	}
	var data []byte
	if *svg != "" {
		text, err := stdinOr(*svg)
		if err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if !strings.HasPrefix(text, "<svg") || !strings.HasSuffix(text, "</svg>") {
			return errors.New("canvas: an SVG starts with <svg and ends with </svg>")
		}
		data = []byte(text)
		*name = withExt(*name, ".svg")
	} else {
		var ext string
		if data, ext, err = webImage(*url); err != nil {
			return err
		}
		*name = withExt(*name, ext)
	}
	kept, err := r.c.AddCanvasAsset(c.ID, *name, data)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "Kept as %s: HTML as assets/%s, a shape's src %s.\n", kept, kept, kept)
	return nil
}

// webImage is the picture at https URL u, and its file extension.
func webImage(u string) ([]byte, string, error) {
	if !strings.HasPrefix(u, "https://") {
		return nil, "", errors.New("canvas: a picture's URL starts with https://")
	}
	cl := &http.Client{Timeout: 30 * time.Second}
	resp, err := cl.Get(u)
	if err != nil {
		return nil, "", fmt.Errorf("canvas: fetch the picture: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("canvas: fetch the picture: %s", resp.Status)
	}
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/svg+xml": ".svg", "image/gif": ".gif"}[kind]
	if ext == "" {
		return nil, "", fmt.Errorf("canvas: %s is %q, not a picture", u, kind)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxWebImage+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxWebImage {
		return nil, "", errors.New("canvas: the picture is over 10 MB")
	}
	return data, ext, nil
}

// withExt is name ending in ext.
func withExt(name, ext string) string {
	if strings.EqualFold(path.Ext(name), ext) {
		return name
	}
	return strings.TrimSuffix(name, path.Ext(name)) + ext
}
