package notesio

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Notion's blocks as Markdown.

// blocks writes block id's children as Markdown, depth levels in.
func (c *notionClient) blocks(id string, depth int, out *strings.Builder, d *Doc, b *Batch) error {
	cursor := ""
	for {
		q := "/v1/blocks/" + url.PathEscape(id) + "/children?page_size=100"
		if cursor != "" {
			q += "&start_cursor=" + url.QueryEscape(cursor)
		}
		var res struct {
			Results    []notionBlock `json:"results"`
			HasMore    bool          `json:"has_more"`
			NextCursor string        `json:"next_cursor"`
		}
		if err := c.call("GET", q, nil, &res); err != nil {
			return err
		}
		for _, bl := range res.Results {
			if err := c.block(bl, depth, out, d, b); err != nil {
				return err
			}
		}
		if !res.HasMore || res.NextCursor == "" {
			return nil
		}
		cursor = res.NextCursor
	}
}

func (c *notionClient) block(bl notionBlock, depth int, out *strings.Builder, d *Doc, b *Batch) error {
	var ct notionContent
	json.Unmarshal(bl.raw[bl.Type], &ct)
	indent := strings.Repeat("  ", depth)
	text := richMarkdown(ct.RichText)
	nested := depth
	line := func(s string) { out.WriteString(s + "\n") }
	switch bl.Type {
	case "paragraph":
		line(indent + text)
	case "heading_1", "heading_2", "heading_3":
		line("\n" + strings.Repeat("#", int(bl.Type[8]-'0')) + " " + text)
	case "bulleted_list_item", "toggle":
		line(indent + "- " + text)
		nested++
	case "numbered_list_item":
		line(indent + "1. " + text)
		nested++
	case "to_do":
		mark := " "
		if ct.Checked {
			mark = "x"
		}
		line(indent + "- [" + mark + "] " + text)
		nested++
	case "quote":
		line("> " + text)
	case "callout":
		icon := ""
		if ct.Icon != nil && ct.Icon.Emoji != "" {
			icon = ct.Icon.Emoji + " "
		}
		line("> " + icon + text)
	case "code":
		line("```" + ct.Language + "\n" + text + "\n```")
	case "divider":
		line("---")
	case "child_page":
		line(indent + wikilink("notion:"+plainID(bl.ID), "", ""))
		return nil // its own page, read on its own
	case "child_database":
		line(indent + wikilink("notion:"+plainID(bl.ID), "", ""))
		return nil
	case "image":
		alt := richMarkdown(ct.Caption)
		if ct.Type == "external" {
			if u := safeURL(ct.External.URL); u != "" {
				line("![" + alt + "](" + u + ")")
			}
			break
		}
		data, name, err := c.fetch(ct.File.URL)
		if err != nil {
			b.unconverted(d.Title, "an image Notion didn't give: "+err.Error())
			break
		}
		d.Assets = append(d.Assets, Asset{Name: name, Data: data})
		line("![" + alt + "](asset:" + name + ")")
	case "bookmark", "link_preview", "embed", "video", "pdf", "file", "audio":
		u := ct.URL
		if u == "" {
			u = first(ct.External.URL, ct.File.URL)
		}
		if bl.Type != "bookmark" && bl.Type != "link_preview" {
			b.unconverted(d.Title, "an embed ("+bl.Type+"), kept as a link")
		}
		if s := safeURL(u); s != "" {
			line(indent + "[" + first(richMarkdown(ct.Caption), s) + "](" + s + ")")
		}
	case "equation":
		b.unconverted(d.Title, "an equation, kept as code")
		line("`" + ct.Expression + "`")
	case "table":
		var rows [][]string
		var res struct {
			Results []notionBlock `json:"results"`
		}
		if err := c.call("GET", "/v1/blocks/"+url.PathEscape(bl.ID)+"/children?page_size=100", nil, &res); err != nil {
			return err
		}
		for _, r := range res.Results {
			var rc notionContent
			json.Unmarshal(r.raw["table_row"], &rc)
			var row []string
			for _, cell := range rc.Cells {
				row = append(row, strings.ReplaceAll(richMarkdown(cell), "|", `\|`))
			}
			rows = append(rows, row)
		}
		line(markdownTable(rows))
		return nil
	case "column_list", "column", "synced_block", "template":
		// Their children, one after another.
	default:
		b.unconverted(d.Title, "a block of type "+bl.Type)
	}
	if bl.HasChildren {
		return c.blocks(bl.ID, nested, out, d, b)
	}
	return nil
}

// fetch downloads a file Notion hosts, and names it.
func (c *notionClient) fetch(u string) ([]byte, string, error) {
	if safeURL(u) == "" {
		return nil, "", errors.New("no address")
	}
	req, err := http.NewRequestWithContext(c.ctx, "GET", u, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.New(resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFile))
	if err != nil {
		return nil, "", err
	}
	p, _ := url.Parse(u)
	name := path.Base(p.Path)
	if name == "" || name == "/" || name == "." {
		name = "image"
	}
	return data, name, nil
}
