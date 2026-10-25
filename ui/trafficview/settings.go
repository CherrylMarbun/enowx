package trafficview

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// settingsState is the interceptor's settings as read, and as typed.
type settingsState struct {
	loaded, loading, saving bool
	st                      proto.TrafficSettings
	insecure, upstream      string
	maxEx, maxMB            string
	err                     string
}

func (v *View) loadSettings(o Options) {
	s := &v.settings
	if s.loaded || s.loading || o.Host == nil {
		return
	}
	s.loading = true
	h := o.Host
	go func() {
		st, err := h.TrafficSettings()
		o.Update(func() {
			s.loading, s.loaded = false, true
			if err != nil {
				s.err = client.Explain(err)
				return
			}
			s.set(st)
		})
	}()
}

func (s *settingsState) set(st proto.TrafficSettings) {
	s.st, s.err = st, ""
	s.insecure, s.upstream = strings.Join(st.Insecure, ", "), st.Upstream
	s.maxEx, s.maxMB = numText(st.MaxExchanges), numText(st.MaxMB)
}

// NotifiesErrors reports whether the user asked to be told of failed
// requests.
func (v *View) NotifiesErrors() bool { return v.settings.st.NotifyErrors }

func (v *View) settingsPage(c *ui.Context, o Options) {
	s := &v.settings
	v.loadSettings(o)
	ui.Scroll(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		ui.Column(c).FillWidth().Padding(12).Gap(12).Children(func() {
			ui.Text(c, "The interceptor on "+o.HostName).FontSize(12.5).FontWeight(600)
			if v.status.Fingerprint != "" {
				ui.Text(c, "CA SHA-256 "+v.status.Fingerprint).Font(theme.MonoFont).FontSize(10.5).TextColor(theme.Muted).Selectable()
			}
			widget.Field(c, "Don't check the certificates of", func() {
				ui.Column(c).FillWidth().Gap(4).Children(func() {
					widget.Input(c, &s.insecure, "*.test, staging.internal").FillWidth()
					ui.Text(c, "Development servers that sign their own. Loopback (localhost, 127.0.0.1) is never checked.").FontSize(11).TextColor(theme.Muted)
				})
			})
			widget.Field(c, "Send through a proxy", func() {
				widget.Input(c, &s.upstream, "http://proxy.company:3128 (none by default)").FillWidth()
			})
			ui.Row(c).FillWidth().Gap(10).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					widget.Field(c, "Keep at most (exchanges)", func() { widget.Input(c, &s.maxEx, "2000").FillWidth() })
				})
				ui.Column(c).Grow(1).Children(func() {
					widget.Field(c, "Bodies at most (MB)", func() { widget.Input(c, &s.maxMB, "256").FillWidth() })
				})
			})
			widget.Checkbox(c, &s.st.NotifyErrors, "Tell me when an app's request fails or is answered 5xx")
			if s.err != "" {
				ui.Text(c, s.err).FontSize(12).TextColor(theme.Error)
			}
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if widget.PrimaryButton(c, icon.Check, "Save", "").Disabled(s.saving).Clicked() {
					st := s.st
					st.Insecure = splitList(s.insecure)
					st.Upstream = strings.TrimSpace(s.upstream)
					var err error
					if st.MaxExchanges, err = parseNum(s.maxEx, "Exchanges"); err != nil {
						s.err = err.Error()
						return
					}
					if st.MaxMB, err = parseNum(s.maxMB, "MB"); err != nil {
						s.err = err.Error()
						return
					}
					s.saving = true
					h := o.Host
					go func() {
						saved, err := h.SetTrafficSettings(st)
						o.Update(func() {
							s.saving = false
							if err != nil {
								s.err = client.Explain(err)
								return
							}
							s.set(saved)
							v.notice = "Saved."
						})
					}()
				}
			})
		})
	})
}
