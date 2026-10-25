package notesio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// Notion's own API, with the token of an integration the user shared
// their pages with: every page and database it can see, their blocks as
// Markdown, their uploaded images fetched. The token goes to Notion only.

const notionAPI = "https://api.notion.com"

type notionClient struct {
	ctx   context.Context
	base  string
	token string
	http  *http.Client
}

func (c *notionClient) call(method, p string, body any, out any) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	for try := 0; ; try++ {
		req, err := http.NewRequestWithContext(c.ctx, method, c.base+p, r)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Notion-Version", "2022-06-28")
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("notion: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxFile))
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("notion: %w", err)
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests && try < 5:
			// Notion allows a few requests a second: wait, then again.
			select {
			case <-c.ctx.Done():
				return c.ctx.Err()
			case <-time.After(time.Duration(try+1) * time.Second):
			}
			if body != nil {
				data, _ := json.Marshal(body)
				r = bytes.NewReader(data)
			}
			continue
		case resp.StatusCode == http.StatusUnauthorized:
			return errors.New("notion: the token was refused: check the integration's token, and that the pages are shared with it")
		case resp.StatusCode >= 300:
			var e struct {
				Message string `json:"message"`
			}
			json.Unmarshal(data, &e)
			return fmt.Errorf("notion: %s: %s", resp.Status, e.Message)
		}
		return json.Unmarshal(data, out)
	}
}

type notionRich struct {
	Plain       string `json:"plain_text"`
	Href        string `json:"href"`
	Type        string `json:"type"`
	Annotations struct {
		Bold, Italic, Strikethrough, Code bool
	} `json:"annotations"`
	Mention struct {
		Type string `json:"type"`
		Page struct {
			ID string `json:"id"`
		} `json:"page"`
	} `json:"mention"`
}

type notionObject struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Parent struct {
		Type       string `json:"type"`
		PageID     string `json:"page_id"`
		DatabaseID string `json:"database_id"`
	} `json:"parent"`
	Icon *struct {
		Type  string `json:"type"`
		Emoji string `json:"emoji"`
	} `json:"icon"`
	Title      []notionRich `json:"title"` // a database's
	Properties map[string]struct {
		Type  string       `json:"type"`
		Title []notionRich `json:"title"`
	} `json:"properties"`
	Created  time.Time `json:"created_time"`
	Edited   time.Time `json:"last_edited_time"`
	Archived bool      `json:"archived"`
}

type notionBlock struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	HasChildren bool   `json:"has_children"`
	raw         map[string]json.RawMessage
}

func (b *notionBlock) UnmarshalJSON(data []byte) error {
	type plain notionBlock
	if err := json.Unmarshal(data, (*plain)(b)); err != nil {
		return err
	}
	return json.Unmarshal(data, &b.raw)
}

// content is the part of a block of its type.
type notionContent struct {
	RichText []notionRich `json:"rich_text"`
	Checked  bool         `json:"checked"`
	Language string       `json:"language"`
	Title    string       `json:"title"`
	URL      string       `json:"url"`
	Caption  []notionRich `json:"caption"`
	Icon     *struct {
		Emoji string `json:"emoji"`
	} `json:"icon"`
	Cells    [][]notionRich `json:"cells"`
	Type     string         `json:"type"`
	External struct {
		URL string `json:"url"`
	} `json:"external"`
	File struct {
		URL string `json:"url"`
	} `json:"file"`
	Expression string `json:"expression"`
}

func plainID(id string) string { return strings.ReplaceAll(id, "-", "") }

func richMarkdown(rs []notionRich) string {
	var b strings.Builder
	for _, r := range rs {
		t := r.Plain
		if r.Type == "mention" && r.Mention.Type == "page" {
			b.WriteString(wikilink("notion:"+plainID(r.Mention.Page.ID), "", ""))
			continue
		}
		lead, trail := t[:len(t)-len(strings.TrimLeft(t, " "))], t[len(strings.TrimRight(t, " ")):]
		t = strings.TrimSpace(t)
		if t == "" {
			b.WriteString(lead + trail)
			continue
		}
		a := r.Annotations
		switch {
		case a.Code:
			t = "`" + t + "`"
		default:
			if a.Bold {
				t = "**" + t + "**"
			}
			if a.Italic {
				t = "*" + t + "*"
			}
			if a.Strikethrough {
				t = "~~" + t + "~~"
			}
		}
		if u := safeURL(r.Href); u != "" {
			t = "[" + t + "](" + u + ")"
		} else if r.Href != "" {
			if id := notionID(path.Base(r.Href)); id != "" {
				t = wikilink("notion:"+id, "", strings.Trim(t, "*~`"))
			}
		}
		b.WriteString(lead + t + trail)
	}
	return b.String()
}

func readNotionAPI(ctx context.Context, o Options) (*Batch, error) {
	if o.Token == "" {
		return nil, errors.New("import: Notion's API needs an integration's token")
	}
	c := &notionClient{ctx: ctx, base: o.NotionAPI, token: o.Token, http: &http.Client{Timeout: time.Minute}}
	if c.base == "" {
		c.base = notionAPI
	}
	var objects []notionObject
	for _, kind := range []string{"database", "page"} {
		cursor := ""
		for {
			body := map[string]any{"filter": map[string]string{"property": "object", "value": kind}, "page_size": 100}
			if cursor != "" {
				body["start_cursor"] = cursor
			}
			var res struct {
				Results    []notionObject `json:"results"`
				HasMore    bool           `json:"has_more"`
				NextCursor string         `json:"next_cursor"`
			}
			if err := c.call("POST", "/v1/search", body, &res); err != nil {
				return nil, err
			}
			objects = append(objects, res.Results...)
			if !res.HasMore || res.NextCursor == "" {
				break
			}
			cursor = res.NextCursor
		}
	}
	b := &Batch{From: proto.ImportNotionAPI}
	known := map[string]bool{}
	for _, ob := range objects {
		known[plainID(ob.ID)] = true
	}
	for _, ob := range objects {
		if ob.Archived {
			continue
		}
		id := plainID(ob.ID)
		d := Doc{Key: "notion:" + id, Source: "notion:" + id, Created: ob.Created, Updated: ob.Edited}
		for _, p := range []string{ob.Parent.PageID, ob.Parent.DatabaseID} {
			if p != "" && known[plainID(p)] {
				d.Parent = "notion:" + plainID(p)
			}
		}
		if ob.Icon != nil && ob.Icon.Type == "emoji" {
			d.Icon = ob.Icon.Emoji
		}
		if ob.Object == "database" {
			d.Title = richMarkdown(ob.Title)
			if d.Icon == "" {
				d.Icon = "🗃️"
			}
		} else {
			for _, p := range ob.Properties {
				if p.Type == "title" {
					d.Title = richMarkdown(p.Title)
				}
			}
			var body strings.Builder
			if err := c.blocks(ob.ID, 0, &body, &d, b); err != nil {
				return nil, err
			}
			d.Body = tidy(body.String())
		}
		if strings.TrimSpace(d.Title) == "" {
			d.Title = "Untitled"
		}
		b.Docs = append(b.Docs, d)
	}
	orderParents(b)
	return b, nil
}
