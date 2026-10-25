// Package vaultview is the Credentials page: the user's credential vault,
// made, unlocked, its logins listed by platform, added, edited, checked,
// imported from a host's tools, and the questions it asks the user when a
// host wants one. It gets the vault and what to do from the app through
// Options; the app keeps the key.
package vaultview

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/vault"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The vault's states (Options.State).
const (
	StateNone   = "none"   // no vault yet: make one
	StateLocked = "locked" // its key isn't in memory
	StateOpen   = "open"
)

// Host is a host credentials may be used on.
type Host struct{ ID, Name string }

// Options are the vault as the page shows it, and what it does.
type Options struct {
	State       string
	Credentials []vault.Credential
	Hosts       []Host
	Current     Host // the host connected to
	Audit       []proto.VaultAuditEntry
	Testing     map[string]bool // credential IDs being checked
	Remember    bool            // the key is kept in the keychain
	Found       []proto.VaultFound
	Scanning    bool
	Actions     Actions
}

// Actions are what the page does; the app does them.
type Actions struct {
	Create         func(password string, remember bool) error
	Unlock         func(password string, remember bool) error
	Lock           func()
	Save           func(c vault.Credential) error // an empty ID adds it
	Remove         func(id string)
	Test           func(id string)
	SetDefault     func(id string)
	Revoke         func(id string, revoked bool)
	Scan           func()
	Import         func([]proto.VaultFound) error
	Login          func(platform string) // runs its own login in a session; nil for none
	ChangePassword func(old, password string) error
}

// View is the page's state, owned by the main thread.
type View struct {
	search   string
	password string
	confirm  string
	remember bool
	err      string
	form     *form
	importer *importer
	change   *change
	menu     widget.ContextMenu
	folded   map[string]bool
}

// OpenAdd opens the form adding a credential of platform ("" to pick).
func (v *View) OpenAdd(platform string) {
	f := &form{open: true, values: map[string]string{}, hosts: map[string]bool{}}
	if platform != "" {
		f.pick(vault.Lookup(platform))
	}
	v.form = f
}

// OpenImport opens the import dialog; Options.Found fills it.
func (v *View) OpenImport() { v.importer = &importer{open: true, picks: map[int]bool{}} }

// Body is the page under its header.
func (v *View) Body(c *ui.Context, o Options) {
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(860).Padding(20, 24).Gap(18).Children(func() {
			switch o.State {
			case StateNone:
				v.create(c, o)
			case StateLocked:
				v.unlock(c, o)
			default:
				v.open(c, o)
			}
		})
	})
	v.dialogs(c, o)
}

// Toolbar is the header's buttons: lock, import, add.
func (v *View) Toolbar(c *ui.Context, o Options) {
	if o.State != StateOpen {
		return
	}
	if widget.Button(c, widget.Secondary, icon.ArrowDown, "Import from "+o.Current.Name, "").Tooltip("Find the logins "+o.Current.Name+"'s tools keep (gh, aws, wrangler, docker, npm, SSH keys…)").Clicked() && o.Actions.Scan != nil {
		v.OpenImport()
		o.Actions.Scan()
	}
	if widget.PrimaryButton(c, icon.Plus, "Add credential", "").Clicked() {
		v.OpenAdd("")
	}
	if widget.IconButton(c, icon.Lock, "Lock the vault", false).Clicked() && o.Actions.Lock != nil {
		o.Actions.Lock()
	}
}

func (v *View) open(c *ui.Context, o Options) {
	now := time.Now()
	var soon []vault.Credential
	for _, cr := range o.Credentials {
		if cr.ExpiresSoon(now, 7*24*time.Hour) || cr.State(now) == proto.CredExpired {
			soon = append(soon, cr)
		}
	}
	if len(soon) > 0 {
		names := make([]string, len(soon))
		for i, cr := range soon {
			names[i] = cr.Name
		}
		widget.Notice(c, icon.Alert, theme.Warning, "Rotate soon", strings.Join(names, ", ")+" expire within a week or have: make new tokens and edit them here.", nil)
	}
	if len(o.Credentials) == 0 {
		widget.EmptyState(c, icon.Shield, "No credentials yet",
			"Add a token, a key or a login once; sessions on any host get it when they need it, after you allow that host.", func() {
				ui.Row(c).Gap(8).Children(func() {
					if widget.PrimaryButton(c, icon.Plus, "Add credential", "").Clicked() {
						v.OpenAdd("")
					}
					if widget.Button(c, widget.Secondary, icon.ArrowDown, "Import from "+o.Current.Name, "").Clicked() && o.Actions.Scan != nil {
						v.OpenImport()
						o.Actions.Scan()
					}
				})
			})
		return
	}
	widget.SearchField(c, &v.search, "Search credentials", icon.Search)
	q := strings.ToLower(strings.TrimSpace(v.search))
	if v.folded == nil {
		v.folded = map[string]bool{}
	}
	for _, g := range vault.Groups {
		var in []vault.Credential
		for _, cr := range o.Credentials {
			p := vault.Lookup(cr.Platform)
			if p.Group != g {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(cr.Name+" "+p.Name+" "+cr.Account), q) {
				continue
			}
			in = append(in, cr)
		}
		if len(in) == 0 {
			continue
		}
		slices.SortFunc(in, func(a, b vault.Credential) int { return strings.Compare(a.Name, b.Name) })
		open := !v.folded[g]
		ui.Column(c.Key(g)).FillWidth().Gap(8).Children(func() {
			widget.Group(c, g, len(in), &open, func() {
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					for _, cr := range in {
						v.row(c.Key(cr.ID), o, cr)
					}
				})
			})
		})
		v.folded[g] = !open
	}
	v.audit(c, o)
	if o.Actions.ChangePassword != nil {
		ui.Row(c).FillWidth().Children(func() {
			ui.Spacer(c)
			if widget.GhostButton(c, "Change master password").Clicked() {
				v.change = &change{open: true}
			}
		})
	}
}
