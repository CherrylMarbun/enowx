package ui

import "testing"

func TestEditorURLs(t *testing.T) {
	cases := []struct {
		id, host, path string
		line           int
		want           string
	}{
		{"vscode", "", "/Users/me/my app", 0, "vscode://file/Users/me/my%20app"},
		{"vscode", "", "/Users/me/app/main.go", 12, "vscode://file/Users/me/app/main.go:12"},
		{"cursor", "root@vps", "/home/enowx/app", 0, "cursor://vscode-remote/ssh-remote+root@vps/home/enowx/app"},
		{"zed", "", "/Users/me/app", 0, "zed://file/Users/me/app"},
		{"zed", "root@vps", "/home/enowx/app/a.go", 3, "zed://ssh/root@vps/home/enowx/app/a.go:3"},
	}
	for _, c := range cases {
		if got := editorURL(c.id, c.host, c.path, c.line); got != c.want {
			t.Errorf("%s %q %q: %s, want %s", c.id, c.host, c.path, got, c.want)
		}
	}
}
