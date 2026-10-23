package ui

import (
	"testing"

	"github.com/enowx/enowx/proto"
)

// The project's folder goes whole into a folder there: by default the
// same place under the home folder, else the home folder.
func TestCopyGoesIntoAFolder(t *testing.T) {
	a := New(Options{})
	a.daemonInfo.Home = "/home/enowx"
	for project, want := range map[string]string{
		"/home/enowx/test":     "~/test",
		"/home/enowx/code/app": "~/code/app",
		"/srv/app":             "~/app",
	} {
		a.openCopy(proto.ProjectInfo{Path: project})
		if got := a.copying.dest(); got != want {
			t.Errorf("%s goes to %s, want %s", project, got, want)
		}
	}
	a.openCopy(proto.ProjectInfo{Path: "/srv/app"}) // the map's order is random
	a.copying.parent = "~/work"
	if got := a.copying.dest(); got != "~/work/app" {
		t.Errorf("into ~/work: %s", got)
	}
}
