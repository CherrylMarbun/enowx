package widget

import (
	"strings"
	"testing"
)

// Files of the languages agents write are colored, each by its own
// lexer, and what spans lines stays colored on each.
func TestEveryCommonLanguageIsColored(t *testing.T) {
	for _, name := range strings.Fields(`index.html page.htm style.css app.scss main.js app.jsx main.ts App.tsx
		App.vue Page.svelte main.py app.rb main.go lib.rs Main.java Main.kt App.swift index.php main.c main.cpp
		Program.cs run.sh setup.zsh config.yaml config.yml Cargo.toml package.json query.sql init.lua main.dart
		Dockerfile Makefile README.md notes.mdx schema.graphql main.tf Main.hs app.ex core.clj main.scala
		script.ps1 app.r main.zig build.gradle pom.xml data.xml main.m nginx.conf .env diff.patch`) {
		if lexerFor(name, "", "") == nil {
			t.Errorf("%s isn't colored", name)
		}
	}
	html := "<!doctype html>\n<html>\n<!-- a\ncomment -->\n<body class=\"x\">Hi</body>\n</html>\n"
	lines := HighlightLines("index.html", "", html)
	if len(lines) != 6 {
		t.Fatalf("%d lines", len(lines))
	}
	if kindIn(lines[1], "html") != keywordSpan || kindIn(lines[3], "comment") != commentSpan || kindIn(lines[4], "class") != attrSpan {
		t.Fatalf("html colored as %+v", lines)
	}
	if HighlightLines("notes.txt", "", "just words") != nil {
		t.Fatal("text was colored")
	}
}

func kindIn(spans []Span, text string) spanKind {
	for _, s := range spans {
		if strings.Contains(s.Text, text) {
			return s.kind
		}
	}
	return 255
}
