package trafficview

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// composeForm is a request written by hand.
type composeForm struct {
	method  string
	url     string
	headers string
	body    string
	direct  bool
	sending bool
	err     string
}

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

// fill fills the form from a request caught, to change and send again.
func (f *composeForm) fill(ex proto.TrafficExchange, req proto.TrafficBody) {
	f.method, f.url, f.err = ex.Method, ex.URL, ""
	var hs []proto.Header
	for _, h := range ex.ReqHeaders {
		switch strings.ToLower(h.Name) {
		case "content-length", "host", "accept-encoding":
			continue
		}
		hs = append(hs, h)
	}
	f.headers = headerLines(hs)
	f.body = ""
	if utf8.Valid(req.Data) {
		f.body = string(req.Data)
	}
}

func (v *View) composePage(c *ui.Context, o Options) {
	f := &v.compose
	if f.method == "" {
		f.method = "GET"
	}
	ui.Scroll(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		ui.Column(c).FillWidth().Padding(12).Gap(10).Children(func() {
			ui.Text(c, "Sent from "+o.HostName+", recorded with the rest.").FontSize(11.5).TextColor(theme.Muted)
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				i := -1
				for k, m := range methods {
					if m == f.method {
						i = k
					}
				}
				if k := widget.Segmented(c, methods, i, nil); k >= 0 {
					f.method = methods[k]
				}
			})
			widget.Field(c, "URL", func() { widget.Input(c, &f.url, "https://api.example.com/v1/users").FillWidth() })
			widget.Field(c, "Headers (Name: value per line)", func() {
				ui.TextAreaBase(c, &f.headers).FillWidth().MinHeight(60).MaxHeight(200).Padding(8, 10).Font(theme.MonoFont).FontSize(11.5).
					Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border)
			})
			widget.Field(c, "Body", func() {
				ui.TextAreaBase(c, &f.body).FillWidth().MinHeight(90).MaxHeight(320).Padding(8, 10).Font(theme.MonoFont).FontSize(11.5).
					Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border)
			})
			widget.Checkbox(c, &f.direct, "Around the rules (no mocks or breakpoints)")
			if f.err != "" {
				ui.Text(c, f.err).FontSize(12).TextColor(theme.Error)
			}
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				label := "Send"
				if f.sending {
					label = "Sending…"
				}
				if widget.PrimaryButton(c, icon.ArrowRight, label, "").Disabled(f.sending || strings.TrimSpace(f.url) == "").Clicked() {
					v.send(o)
				}
			})
		})
	})
}

func (v *View) send(o Options) {
	f := &v.compose
	p := proto.TrafficSendParams{Method: f.method, URL: strings.TrimSpace(f.url), Headers: parseHeaderLines(f.headers), Direct: f.direct,
		Project: o.Session.Project, Session: o.Session.ID}
	if f.body != "" {
		p.Body = []byte(f.body)
	}
	f.sending, f.err = true, ""
	h := o.Host
	go func() {
		sum, err := h.TrafficSend(p)
		o.Update(func() {
			f.sending = false
			if err != nil {
				f.err = client.Explain(err)
				return
			}
			v.Apply(sum)
			v.page, v.all, v.filter = pageRequests, true, ""
			v.open(o, sum.ID)
		})
	}()
}
