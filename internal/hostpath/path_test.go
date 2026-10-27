package hostpath

import (
	"net/url"
	"testing"
)

func TestPathsOnEitherHostOS(t *testing.T) {
	for _, tc := range []struct {
		input, clean string
		abs          bool
	}{
		{`C:\Users\Julian Fernando/.enowx-app/notes/assets/1fdcf71820cc/icon.png`, "C:/Users/Julian Fernando/.enowx-app/notes/assets/1fdcf71820cc/icon.png", true},
		{`C:\`, "C:/", true},
		{`\\server\share\image.png`, "//server/share/image.png", true},
		{"/home/user/notes/image.png", "/home/user/notes/image.png", true},
		{"images/../image.png", "image.png", false},
		{"C:image.png", "C:image.png", false},
	} {
		if IsAbs(tc.input) != tc.abs || Clean(tc.input) != tc.clean {
			t.Errorf("%q: abs=%v, clean=%q", tc.input, IsAbs(tc.input), Clean(tc.input))
		}
	}
	for raw, want := range map[string]string{
		"file:///C:/Users/Julian%20Fernando/icon.png": "C:/Users/Julian Fernando/icon.png",
		"file://server/share/icon.png":                "//server/share/icon.png",
		"file:///home/user/icon.png":                  "/home/user/icon.png",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := FileURL(u); got != want {
			t.Errorf("%s: %q", raw, got)
		}
	}
}
