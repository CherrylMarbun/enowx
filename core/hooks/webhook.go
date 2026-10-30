package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// webhookTimeout bounds a webhook's request.
const webhookTimeout = 10 * time.Second

// Expand fills in message's {agent}, {project}, {event}… from ev; an empty
// message says what happened.
func Expand(message string, ev proto.HookEvent) string {
	if strings.TrimSpace(message) == "" {
		message = "{event}: {agent} in {project}"
	}
	project := ev.Project
	if project == "" {
		project = ev.Dir
	}
	if i := strings.LastIndexAny(project, `/\`); i >= 0 {
		project = project[i+1:]
	}
	r := strings.NewReplacer(
		"{event}", ev.Event, "{agent}", ev.Agent, "{project}", project, "{dir}", ev.Dir,
		"{branch}", ev.Branch, "{session}", ev.Session, "{prompt}", ev.Prompt, "{tool}", ev.Tool,
		"{command}", ev.Command, "{paths}", strings.Join(ev.Paths, ", "), "{commit}", ev.Commit,
		"{message}", ev.Message, "{op}", ev.Op, "{port}", fmt.Sprint(ev.Port), "{text}", ev.Text, "{host}", ev.Host,
	)
	return r.Replace(message)
}

// webhook posts ev to h's URL: Discord's and Slack's webhooks get the
// message as they take it, any other address the event as JSON with the
// message. It returns the status code.
func webhook(h proto.Hook, ev proto.HookEvent) (int, error) {
	u, err := url.Parse(h.URL)
	if err != nil {
		return 0, err
	}
	text := Expand(h.Message, ev)
	var body any
	switch {
	case strings.HasSuffix(u.Host, "discord.com") || strings.HasSuffix(u.Host, "discordapp.com"):
		body = map[string]any{"content": text, "allowed_mentions": map[string]any{"parse": []string{}}}
	case u.Host == "hooks.slack.com":
		body = map[string]string{"text": text}
	default:
		body = struct {
			proto.HookEvent
			Text string `json:"message"`
		}{ev, text}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), webhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("webhook: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("webhook: %s", resp.Status)
	}
	return resp.StatusCode, nil
}
