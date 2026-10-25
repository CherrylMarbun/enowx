package notesio

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"mime"
	"path"
	"strings"
	"time"
)

// Evernote's .enex exports, a notebook each: a note's content is XHTML
// whose <en-media> show its resources, named by their MD5.

type enexExport struct {
	Notes []enexNote `xml:"note"`
}

type enexNote struct {
	Title     string         `xml:"title"`
	Content   string         `xml:"content"`
	Created   string         `xml:"created"`
	Updated   string         `xml:"updated"`
	Tags      []string       `xml:"tag"`
	Resources []enexResource `xml:"resource"`
}

type enexResource struct {
	Data     string `xml:"data"`
	Mime     string `xml:"mime"`
	FileName string `xml:"resource-attributes>file-name"`
}

func enexTime(s string) time.Time {
	t, _ := time.Parse("20060102T150405Z", strings.TrimSpace(s))
	return t
}

func readEnex(ctx context.Context, s *source, b *Batch) error {
	for _, f := range s.files {
		if !strings.EqualFold(path.Ext(f), ".enex") {
			continue
		}
		data, err := s.read(f)
		if err != nil {
			b.skip(f, err.Error())
			continue
		}
		var ex enexExport
		dec := xml.NewDecoder(bytes.NewReader(data))
		dec.Strict = false
		dec.Entity = xml.HTMLEntity
		if err := dec.Decode(&ex); err != nil {
			b.skip(f, "not an Evernote export: "+err.Error())
			continue
		}
		notebook := strings.TrimSuffix(path.Base(f), path.Ext(f))
		for i, n := range ex.Notes {
			if err := ctx.Err(); err != nil {
				return err
			}
			sum := sha1.Sum([]byte(notebook + "\x00" + n.Title + "\x00" + n.Created))
			d := Doc{Key: fmt.Sprintf("%s#%d", f, i), Source: "enex:" + hex.EncodeToString(sum[:8]),
				Group: notebook, Title: strings.TrimSpace(n.Title), Created: enexTime(n.Created), Updated: enexTime(n.Updated)}
			byHash := map[string]Asset{}
			for j, r := range n.Resources {
				raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(r.Data), ""))
				if err != nil {
					b.skip(d.Title, "an attachment that isn't base64")
					continue
				}
				h := md5.Sum(raw)
				name := r.FileName
				if name == "" {
					name = fmt.Sprintf("attachment-%d", j+1)
					if exts, _ := mime.ExtensionsByType(r.Mime); len(exts) > 0 {
						name += exts[0]
					}
				}
				byHash[hex.EncodeToString(h[:])] = Asset{Name: name, Data: raw}
			}
			c := &htmlConv{batch: b, where: d.Title}
			c.media = func(hash, kind string) string {
				a, ok := byHash[hash]
				if !ok {
					return ""
				}
				d.Assets = append(d.Assets, a)
				if strings.HasPrefix(kind, "image/") {
					return "![](asset:" + a.Name + ")"
				}
				return "[" + a.Name + "](asset:" + a.Name + ")"
			}
			body := c.convert(n.Content)
			if len(n.Tags) > 0 {
				body = "Tags: " + strings.Join(n.Tags, ", ") + "\n\n" + body
			}
			d.Body = body
			b.Docs = append(b.Docs, d)
		}
	}
	return nil
}
