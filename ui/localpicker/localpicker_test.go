package localpicker

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// The picker starts at home, goes into folders, picks files and folders
// across them, and hands them over.
func TestPickFilesAndFolders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, p := range []string{"Downloads/logo.png", "Downloads/brand/colors.css", "notes.txt", ".secret"} {
		f := filepath.Join(home, p)
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var p Picker
	var got []string
	p.Open("Add to docs", "Copied into ~/app/docs on This Mac.", func(paths []string) { got = paths })
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(theme.Background).TextColor(theme.Text).Children(func() { p.View(c) })
	}, 800, 700)
	tt.Frame()
	if !tt.HasText("Downloads") || !tt.HasText("notes.txt") || tt.HasText(".secret") {
		t.Fatalf("home: %q", tt.Texts())
	}
	if err := tt.Click("notes.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Downloads/"); err != nil { // the shortcut above the list
		if err := tt.Click("Downloads"); err != nil {
			t.Fatal(err)
		}
	}
	tt.Frame()
	if !tt.HasText("logo.png") || !tt.HasText("brand") {
		t.Fatalf("Downloads: %q", tt.Texts())
	}
	if err := tt.Click("logo.png"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// A folder is picked by its box; its name opens it.
	p.selected[filepath.Join(home, "Downloads", "brand")] = true
	tt.Frame()
	if dir := os.Getenv("SHOTS"); dir != "" {
		if f, err := os.Create(filepath.Join(dir, "local-picker.png")); err == nil {
			_ = png.Encode(f, tt.Image())
			f.Close()
		}
	}
	if err := tt.Click("Add 3"); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	tt.Frame()
	want := []string{filepath.Join(home, "Downloads", "brand"), filepath.Join(home, "Downloads", "logo.png"), filepath.Join(home, "notes.txt")}
	if !slices.Equal(got, want) || p.IsOpen() {
		t.Fatalf("picked %q, open %v", got, p.IsOpen())
	}
}
