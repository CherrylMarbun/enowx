package notes

import "testing"

func TestAssetReferencesAcrossHosts(t *testing.T) {
	const ref = "assets/1fdcf71820cc/icon.png"
	for _, link := range []string{
		ref,
		`C:\Users\Julian Fernando/.enowx-app/notes/` + ref,
		`C:/Users/Julian Fernando/.enowx-app/notes/` + ref,
		"file:///C:/Users/Julian%20Fernando/.enowx-app/notes/" + ref,
		"/home/user/.enowx-app/notes/" + ref,
		"</home/user/.enowx-app/notes/" + ref + ">",
	} {
		if got, ok := AssetReference(link); !ok || got != ref {
			t.Errorf("%q: %q, %v", link, got, ok)
		}
	}
	for _, link := range []string{"https://example.com/notes/" + ref, "assets/1fdcf71820cc/../secret.png", "assets/invalid/icon.png", "assets/1fdcf71820cc/", "/tmp/" + ref} {
		if got, ok := AssetReference(link); ok {
			t.Errorf("accepted unrelated path %q as %q", link, got)
		}
	}
}
