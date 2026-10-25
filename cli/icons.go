package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/enowx/enowx/core/iconlib"
)

const iconsUsage = `usage: enowx-app icons COMMAND
  search QUERY [--lib L[,L…]] [--limit N]   icons by name and keyword, best first: lib:name, then keywords
  libs                                       the libraries of icons (lucide, tabler, phosphor)
A note's or group's icon is lib:name (lucide:rocket), "#rrggbb" after it for a color; never an emoji.`

// icons finds the SVG icons notes and groups take.
func (r runner) icons(args []string) error {
	if len(args) == 0 {
		return errors.New(iconsUsage)
	}
	switch args[0] {
	case "libs":
		for _, l := range iconlib.Libraries {
			fmt.Fprintf(r.out, "%-9s %s, %d icons\n", l.ID, l.Name, len(iconlib.All(l.ID)))
		}
		return nil
	case "search":
		fs := flag.NewFlagSet("search", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		lib := fs.String("lib", "", "")
		limit := fs.Int("limit", 20, "")
		query, err := parseWithArg(fs, args[1:])
		if err != nil || query == "" {
			return errors.New(iconsUsage)
		}
		var libs []string
		if *lib != "" {
			libs = strings.Split(*lib, ",")
		}
		found := iconlib.Search(query, libs, *limit)
		if len(found) == 0 {
			return fmt.Errorf("no icon for %q: try a simpler word (rocket, check, book)", query)
		}
		for _, i := range found {
			fmt.Fprintf(r.out, "%-32s %s\n", i.Ref(), strings.Join(i.Tags, ", "))
		}
		return nil
	case "help":
		fmt.Fprintln(r.out, iconsUsage)
		return nil
	}
	return errors.New(iconsUsage)
}

// iconValue is the icon --icon or --icon-svg give: an icon value checked,
// or the user's SVG (a file, - for stdin, or the code itself) as one.
func iconValue(icon, svg string) (string, error) {
	if svg == "" {
		return iconlib.Check(icon)
	}
	var data []byte
	switch {
	case svg == "-":
		var err error
		if data, err = io.ReadAll(io.LimitReader(os.Stdin, iconlib.MaxSVG+1)); err != nil {
			return "", err
		}
	case strings.HasPrefix(strings.TrimSpace(svg), "<"):
		data = []byte(svg)
	default:
		var err error
		if data, err = os.ReadFile(svg); err != nil {
			return "", err
		}
	}
	return iconlib.Custom(data, true)
}

// iconShown is how an icon value reads in text.
func iconShown(icon string) string {
	if strings.HasPrefix(icon, "svg:") {
		return "svg (the user's own)"
	}
	return icon
}

// iconTools are the icons as MCP tools.
var iconTools = []mcpTool{
	{name: "icons_search", description: "Find SVG icons for notes and groups by name or keyword (rocket, bug, calendar): their lib:name values, best first, with keywords. Icons are never emoji.",
		props:    map[string]mcpProp{"query": {"string", "A word or two"}, "lib": {"string", "lucide, tabler or phosphor (comma-separated); all when left out"}},
		required: []string{"query"},
		args: func(a map[string]any) []string {
			return withFlag([]string{"search", str(a, "query")}, a, "lib", "lib")
		}},
}
