package daemon

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/blobstore"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// An image sent to a team's chat: made WebP and no larger than allowed,
// kept by the cloud sealed, shown on another member's host, refused from
// a viewer, and gone from the cloud as its message is deleted.
func TestTeamChatImages(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	files := t.TempDir()
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test", Blobs: blobstore.Dir(files)}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	bob := signInHost(t, db, cloud.URL, 2, "bob")
	team, _ := db.CreateTeam(ctx, ada.acc, "Design")
	inv, _ := db.CreateInvite(ctx, team.ID, ada.acc, api.InviteParams{Role: api.RoleViewer})
	db.AcceptInvite(ctx, inv.Token, bob.acc)
	ada.srv.learnTeams(ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	bob.srv.learnTeams(bob.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleViewer}})
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)

	// A large photo, as a PNG: shades with the grain a camera's has.
	img := image.NewNRGBA(image.Rect(0, 0, 3000, 1500))
	grain := rand.New(rand.NewPCG(1, 2))
	for y := 0; y < 1500; y++ {
		for x := 0; x < 3000; x++ {
			n := uint8(grain.IntN(24))
			img.Set(x, y, color.NRGBA{uint8(x/12) + n, uint8(y/6) + n, 120 + n, 255})
		}
	}
	var shot bytes.Buffer
	png.Encode(&shot, img)
	if _, err := ada.c.UploadChatImage(proto.ChatUploadParams{Team: team.ID, Data: []byte("not an image")}); err == nil {
		t.Fatal("not an image was taken")
	}
	att, err := ada.c.UploadChatImage(proto.ChatUploadParams{Team: team.ID, Name: "screen.png", Data: shot.Bytes()})
	if err != nil {
		t.Fatal(err)
	}
	if att.Type != "image/webp" || att.Width != maxImageSide || att.Height != maxImageSide/2 || att.Size >= shot.Len() || att.Name != "screen.png" {
		t.Fatalf("the attachment: %+v (the PNG %d bytes)", att, shot.Len())
	}
	sent, err := ada.c.SendChat(proto.ChatSendParams{Team: team.ID, Attachments: []proto.ChatAttachment{att}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bob.c.UploadChatImage(proto.ChatUploadParams{Team: team.ID, Data: shot.Bytes()}); err == nil {
		t.Fatal("a viewer uploaded an image")
	}
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)
	list, _ := bob.c.TeamChat(team.ID, 0)
	if len(list) != 1 || len(list[0].Attachments) != 1 || list[0].Attachments[0].ID != att.ID {
		t.Fatalf("bob's chat: %+v", list)
	}
	data, err := bob.c.ChatImage(proto.ChatImageParams{Team: team.ID, ID: att.ID, Epoch: att.Epoch})
	if err != nil {
		t.Fatal(err)
	}
	if cfg, format, err := image.DecodeConfig(bytes.NewReader(data)); err != nil || format != "webp" || cfg.Width != maxImageSide {
		t.Fatalf("the image bob sees: %v %s %v", cfg, format, err)
	}
	kept, err := os.ReadFile(filepath.Join(files, "teams", team.ID, att.ID))
	if err != nil || bytes.Equal(kept, data) || bytes.Contains(kept, data[:64]) {
		t.Fatalf("the cloud keeps the image in the clear, or not at all: %v", err)
	}

	if err := ada.c.DeleteChat(team.ID, sent.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the image gone from the cloud", func() bool {
		_, err := os.Stat(filepath.Join(files, "teams", team.ID, att.ID))
		return os.IsNotExist(err)
	})
}
