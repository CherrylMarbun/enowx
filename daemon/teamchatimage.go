package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg" // decoded as sent
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/proto"
)

// Images in a team's chat: one sent is made WebP and small here (an
// animated GIF kept as it is), sealed with the team's key, and kept by the
// cloud by a random ID, the message naming it. One shown is fetched once,
// opened and kept in the space's blobs folder, which isn't synced.

const (
	maxUpload      = 12 << 20 // an image as the user picked it
	maxImageSide   = 1920     // pixels: larger ones are scaled down
	imageQuality   = 70
	maxAttachments = 4
	imageTime      = 3 * time.Minute
)

var attachmentID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func (s *Server) imageFile(team, id string) string {
	return filepath.Join(s.teamsDir(), team, "blobs", id)
}

// blobItem is what a file's seal is bound to, with its epoch.
func blobItem(id string) string { return "blob:" + id }

// shrinkImage is data as sent: WebP, scaled to maxImageSide, but an
// animated GIF as it was; its type and size.
func shrinkImage(data []byte) ([]byte, string, image.Point, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", image.Point{}, errors.New("teams: not an image: PNG, JPEG, GIF or WebP")
	}
	if cfg.Width*cfg.Height > 50_000_000 {
		return nil, "", image.Point{}, errors.New("teams: an image of 50 megapixels at most")
	}
	if format == "gif" {
		if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil && len(g.Image) > 1 {
			if len(data) > api.MaxImage {
				return nil, "", image.Point{}, fmt.Errorf("teams: an animated GIF is %d MB at most", api.MaxImage>>20)
			}
			return data, "image/gif", image.Pt(cfg.Width, cfg.Height), nil
		}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", image.Point{}, errors.New("teams: the image doesn't open")
	}
	b := img.Bounds()
	if w, h := b.Dx(), b.Dy(); max(w, h) > maxImageSide {
		if w >= h {
			w, h = maxImageSide, max(1, h*maxImageSide/w)
		} else {
			w, h = max(1, w*maxImageSide/h), maxImageSide
		}
		small := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(small, small.Bounds(), img, b, draw.Over, nil)
		img = small
	}
	var out bytes.Buffer
	if err := webp.Encode(&out, img, webp.Options{Quality: imageQuality, Method: 6}); err != nil {
		return nil, "", image.Point{}, fmt.Errorf("teams: make the image WebP: %w", err)
	}
	if out.Len() > api.MaxImage {
		return nil, "", image.Point{}, fmt.Errorf("teams: the image is over %d MB even made small", api.MaxImage>>20)
	}
	return out.Bytes(), "image/webp", img.Bounds().Size(), nil
}

// teamCloud is the host's cloud client and team's keys.
func (s *Server) teamCloud(ctx context.Context, team, role string) (*cloudclient.Client, teamKeyring, error) {
	st, err := s.cloud.store.Get()
	if err != nil {
		return nil, teamKeyring{}, err
	}
	c, err := cloudclient.New(st.URL, st.Token)
	if err != nil {
		return nil, teamKeyring{}, err
	}
	kr, err := s.teamKeys(ctx, c, team, role)
	return c, kr, err
}

// uploadImage makes an image small, seals it and has the cloud keep it,
// for a message the host's account sends.
func (s *Server) uploadImage(p proto.ChatUploadParams) (proto.ChatAttachment, error) {
	sp, err := s.memberOf(p.Team)
	if err != nil {
		return proto.ChatAttachment{}, errors.New("teams: viewers read the chat; members send images")
	}
	if len(p.Data) == 0 || len(p.Data) > maxUpload {
		return proto.ChatAttachment{}, fmt.Errorf("teams: an image is %d MB at most", maxUpload>>20)
	}
	data, kind, size, err := shrinkImage(p.Data)
	if err != nil {
		return proto.ChatAttachment{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), imageTime)
	defer cancel()
	c, kr, err := s.teamCloud(ctx, sp.Team, sp.Role)
	if err != nil {
		return proto.ChatAttachment{}, err
	}
	key, ok := kr.keys[kr.current]
	if !ok {
		return proto.ChatAttachment{}, errNoTeamKey
	}
	var b [16]byte
	rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	sealed, err := seal.SealItem(key, blobItem(id), int64(kr.current), data)
	if err != nil {
		return proto.ChatAttachment{}, err
	}
	if err := c.PutBlob(ctx, sp.Team, id, sealed); err != nil {
		if cloudclient.IsStatus(err, http.StatusNotImplemented) {
			return proto.ChatAttachment{}, errors.New("teams: this team's cloud keeps no files yet")
		}
		return proto.ChatAttachment{}, err
	}
	_ = writeTeamFile(s.imageFile(sp.Team, id), data) // shown at once, not fetched back
	name := strings.TrimSpace(filepath.Base(p.Name))
	if len(name) > 120 || name == "." {
		name = ""
	}
	return proto.ChatAttachment{ID: id, Name: name, Type: kind, Size: len(data), Width: size.X, Height: size.Y, Epoch: kr.current}, nil
}

// chatImage is attachment p's image: kept here, or fetched from the cloud
// and opened.
func (s *Server) chatImage(p proto.ChatImageParams) (proto.ChatImage, error) {
	sp, ok := s.spaceByTeam(p.Team)
	if !ok {
		return proto.ChatImage{}, fmt.Errorf("spaces: team %s isn't on this host", p.Team)
	}
	if !attachmentID.MatchString(p.ID) {
		return proto.ChatImage{}, fmt.Errorf("teams: no image %q", p.ID)
	}
	file := s.imageFile(sp.Team, p.ID)
	if data, err := os.ReadFile(file); err == nil {
		return proto.ChatImage{Data: data}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), imageTime)
	defer cancel()
	c, kr, err := s.teamCloud(ctx, sp.Team, sp.Role)
	if err != nil && len(kr.keys) == 0 {
		return proto.ChatImage{}, err
	}
	key, ok := kr.keys[p.Epoch]
	if !ok {
		return proto.ChatImage{}, errNoTeamKey
	}
	sealed, err := c.Blob(ctx, sp.Team, p.ID)
	if err != nil {
		return proto.ChatImage{}, err
	}
	data, err := seal.OpenItem(key, blobItem(p.ID), int64(p.Epoch), sealed)
	if err != nil {
		return proto.ChatImage{}, errors.New("teams: the image doesn't open with the team's key")
	}
	_ = writeTeamFile(file, data)
	return proto.ChatImage{Data: data}, nil
}

// checkAttachments says a message's attachments may be sent.
func checkAttachments(list []proto.ChatAttachment) error {
	if len(list) > maxAttachments {
		return fmt.Errorf("teams: %d images a message at most", maxAttachments)
	}
	for _, a := range list {
		if !attachmentID.MatchString(a.ID) || a.Epoch <= 0 {
			return errors.New("teams: an image that wasn't uploaded")
		}
	}
	return nil
}

// dropImages deletes a deleted message's images from the cloud and here.
func (s *Server) dropImages(m proto.ChatMessage) {
	if len(m.Attachments) == 0 {
		return
	}
	for _, a := range m.Attachments {
		if attachmentID.MatchString(a.ID) {
			os.Remove(s.imageFile(m.Team, a.ID))
		}
	}
	// The cloud deletes them for their sender or an admin only.
	st, err := s.cloud.store.Get()
	if err != nil {
		return
	}
	c, err := cloudclient.New(st.URL, st.Token)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), imageTime)
	defer cancel()
	for _, a := range m.Attachments {
		if attachmentID.MatchString(a.ID) {
			if err := c.DeleteBlob(ctx, m.Team, a.ID); err != nil {
				s.logger().Warn("teams: delete a message's image in the cloud", "team", m.Team, "id", a.ID, "err", err)
			}
		}
	}
}
