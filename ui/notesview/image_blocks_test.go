package notesview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/enowx/enowx/ui/theme"
)

func imageTestData(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 240, 150))
	for y := range 150 {
		for x := range 240 {
			img.Set(x, y, color.RGBA{uint8(60 + x/2), uint8(80 + y/2), 180, 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func waitForTestImage(t *testing.T, tt *ui.Tester) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		tt.Frame()
		if !tt.HasText("Loading the image…") {
			if tt.HasText("The image didn't load") {
				t.Fatal("test image failed to load")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("test image never loaded")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestImageWithAdjacentTextKeepsPreviewWhileEditing(t *testing.T) {
	for _, width := range []int{720, 360} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			ref := "assets/image-text-" + strconv.Itoa(width) + "/shot.png"
			e := newEditor("![Image](" + ref + ") 1.pastikan\n")
			data := imageTestData(t)
			e.env = pageEnv{width: float32(width - 16), asset: func(string) ([]byte, error) { return data, nil }}
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Width(float32(width)).Height(500).Background(theme.Background).Children(func() { e.draw(c) })
			}, width, 500)
			waitForTestImage(t, tt)
			if !tt.HasText("1.pastikan") || tt.HasText("![Image]") {
				t.Fatalf("mixed image block rendered as Markdown: %q", tt.Texts())
			}
			if err := tt.Click("Block 1"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if tt.HasText("![Image]") {
				t.Fatal("editing exposed the image Markdown")
			}
			tt.Type(" check")
			tt.Frame()
			if !strings.Contains(e.markdown(), ") 1.pastikan check") {
				t.Fatalf("caption edit lost image or text: %q", e.markdown())
			}
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			body := e.markdown()
			e.blocks = parseBlocks(body)
			e.env.read = true
			tt.Frame()
			if !tt.HasText("1.pastikan check") || tt.HasText("![Image]") {
				t.Fatal("reopening in read mode lost the image or caption")
			}
			shot(t, tt, "image-with-text-"+strconv.Itoa(width)+".png")
		})
	}
}

func TestMentionImageFromTextAndOpenPreview(t *testing.T) {
	np := newNotesPage("![Image](assets/mention/shot.png) Caption\n\nSee @\n")
	data := imageTestData(t)
	opts := np.options()
	opts.Asset = func(string) ([]byte, error) { return data, nil }
	np.v.Select(opts, "p1")
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(1100).Height(700).Background(theme.Background).Children(func() { np.v.Draw(c, opts) })
	}, 1100, 700)
	waitForTestImage(t, tt)
	e := np.v.page.ed
	e.openAt(2, -1)
	tt.Frame()
	if !tt.HasText("Mention an image") {
		t.Fatalf("no image choices: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !strings.Contains(e.markdown(), "See [@Image 1](assets/mention/shot.png)") || len(e.blocks) != 3 {
		t.Fatalf("mention did not replace the query: %q", e.markdown())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if err := tt.Click("@Image 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if np.v.preview == nil || !np.v.preview.open || np.v.preview.ref != "assets/mention/shot.png" {
		t.Fatal("mention did not open its image")
	}
	shot(t, tt, "image-mention-preview.png")
}

func TestMentionImageInCaptionKeepsImage(t *testing.T) {
	e := newEditor("![Image](assets/caption-mention/shot.png) See @\n")
	data := imageTestData(t)
	e.env = pageEnv{width: 600, asset: func(string) ([]byte, error) { return data, nil }}
	tt := newTester(e)
	e.openAt(0, -1)
	tt.Frame()
	if !tt.HasText("Mention an image") {
		t.Fatal("caption did not offer image mentions")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !strings.Contains(e.markdown(), ") See [@Image 1](assets/caption-mention/shot.png)") {
		t.Fatalf("caption mention lost text: %q", e.markdown())
	}
}

func TestImageMentionButton(t *testing.T) {
	e := newEditor("![Image](assets/mention-button/shot.png)\n")
	data := imageTestData(t)
	e.env = pageEnv{width: 600, asset: func(string) ([]byte, error) { return data, nil }}
	tt := newTester(e)
	tt.Frame()
	if err := tt.Click("Mention Image 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(e.blocks) != 2 || !strings.Contains(e.blocks[1].text, "[@Image 1]") || e.open != 1 {
		t.Fatalf("button did not insert a mention: %q", e.markdown())
	}
}

func TestImageMentionMouseSelection(t *testing.T) {
	e := newEditor("![Image](assets/mention-mouse/shot.png)\n\nLook @\n")
	data := imageTestData(t)
	e.env = pageEnv{width: 600, asset: func(string) ([]byte, error) { return data, nil }}
	tt := newTester(e)
	e.openAt(2, -1)
	tt.Frame()
	frames(tt, 3)
	shot(t, tt, "image-mention-menu.png")
	if err := tt.Click("Choose image Image 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !strings.Contains(e.blocks[2].text, "Look [@Image 1]") {
		r, _ := tt.Find("Choose image Image 1")
		t.Fatalf("click did not select the mention: %q, open=%d menuIn=%v rect=%+v texts=%q", e.markdown(), e.open, e.menuIn, r, tt.Texts())
	}
}

func TestImageMentionFiltersAndIgnoresCode(t *testing.T) {
	e := newEditor("![Image](assets/catalog/first.png)\n![Diagram](assets/catalog/second.png)\n" +
		"`![Example](assets/catalog/code.png)`\n\\![Escaped](assets/catalog/escaped.png)\n" +
		"![Image](assets/catalog/first.png)\n\nLihat @second\n")
	if got := e.noteImages(); len(got) != 2 {
		t.Fatalf("code, escaped or repeated images were offered: %+v", got)
	}
	_, _, matches := e.imageMentionMatches("Lihat @second", 13)
	if len(matches) != 1 || matches[0].name != "Diagram" {
		t.Fatalf("file name search failed: %+v", matches)
	}
	for _, text := range []string{"mail@example.com", "`@image`", "[@Image](assets/catalog/first.png)"} {
		_, _, _, ok := imageMentionAt(text, len([]rune(text)))
		if ok {
			t.Fatalf("ordinary text was taken as a mention query: %q", text)
		}
	}
	start, stop, _, ok := imageMentionAt("猫 @", 3)
	if !ok {
		t.Fatal("mention after non-ASCII text was not recognized")
	}
	got, _ := insertImageMention("猫 @", start, stop, matches[0])
	if !strings.HasPrefix(got, "猫 [@Diagram]") {
		t.Fatalf("mention damaged non-ASCII text: %q", got)
	}
}
