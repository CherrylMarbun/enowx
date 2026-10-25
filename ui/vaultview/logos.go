package vaultview

import (
	"embed"
	"strconv"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// Platforms' own marks (Simple Icons, CC0), by platform ID, each with its
// brand color; a platform without one shows its initials.

//go:embed logos/*.svg
var logoFiles embed.FS

var brandColors = map[string]string{
	"github": "181717", "gitlab": "FC6D26", "bitbucket": "0052CC", "gitea": "609926", "gcp": "4285F4",
	"digitalocean": "0080FF", "hetzner": "D50C2D", "vultr": "007BFC", "cloudflare": "F38020", "vercel": "000000",
	"netlify": "00C7B7", "fly": "24175B", "railway": "0B0D0E", "render": "000000", "coolify": "6B16ED",
	"supabase": "3FCF8E", "neon": "34D59A", "planetscale": "000000", "turso": "4FF8D2", "mongodb-atlas": "47A248",
	"upstash": "00E9A3", "postgres": "4169E1", "mysql": "4479A1", "redis": "FF4438", "dockerhub": "2496ED",
	"ghcr": "181717", "npm": "CB3837", "pypi": "3775A9", "crates": "000000", "anthropic": "191919",
	"gemini": "8E75B2", "openrouter": "94A3B8", "stripe": "635BFF", "discord": "5865F2", "telegram": "26A5E4",
	"sentry": "362D59", "tailscale": "242424", "certificate": "003A70", "env-file": "ECD53F",
}

var (
	logosMu sync.Mutex
	logos   = map[string]*ui.SVG{}
)

// logoOf is platform id's mark and brand color, nil when it has none.
func logoOf(id string) (*ui.SVG, ui.Color, bool) {
	hex, ok := brandColors[id]
	if !ok {
		return nil, ui.Color{}, false
	}
	logosMu.Lock()
	defer logosMu.Unlock()
	svg, ok := logos[id]
	if !ok {
		data, err := logoFiles.ReadFile("logos/" + id + ".svg")
		if err == nil {
			svg, _ = ui.ParseSVG(data)
		}
		logos[id] = svg
	}
	if svg == nil {
		return nil, ui.Color{}, false
	}
	n, _ := strconv.ParseUint(hex, 16, 32)
	return svg, ui.RGBA(uint8(n>>16), uint8(n>>8), uint8(n), 1), true
}

// readable is a brand color drawn on the surface: a black one as text, a
// dark one moved toward the text's color, so it reads in either theme.
func readable(c ui.Color) ui.Color {
	lum := 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	mix := func(a, b uint8) uint8 { return uint8(float64(a)*0.6 + float64(b)*0.4) }
	switch {
	case lum < 70:
		return theme.Text
	case lum < 140:
		return ui.RGBA(mix(c.R, theme.Text.R), mix(c.G, theme.Text.G), mix(c.B, theme.Text.B), 1)
	}
	return c
}
