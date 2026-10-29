package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/proto"
)

const cloudUsage = `usage: enowx-app cloud COMMAND
  (none) | status | whoami   the team cloud this host is signed in to, as whom
  login [--name NAME]        sign this host in to enowX's cloud (or the one set with url):
                             shows a code to approve in a browser, or in the app; waits
  url URL | url off          a cloud of your own instead of https://cloud.enowx.ai, or off
  logout                     sign this host out
  devices                    your account's devices
  approve CODE               sign in the device showing CODE, as your account
  recovery new | recovery use KEY
                             your recovery key, shown once; on a new device, your teams' keys back with it`

// cloudRecovery makes the account's recovery key, or uses one.
func (r runner) cloudRecovery(args []string) error {
	switch {
	case len(args) == 1 && args[0] == "new":
		out, err := r.c.NewRecoveryKey()
		if err != nil {
			return err
		}
		fmt.Fprintln(r.out, out.Key)
		fmt.Fprintln(r.out, "Shown this once: keep it away from your devices. It replaces any key made before.")
		return nil
	case len(args) == 2 && args[0] == "use":
		out, err := r.c.UseRecoveryKey(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(r.out, "Got back %d of your teams' keys.\n", out.Recovered)
		return nil
	}
	return errors.New("usage: enowx-app cloud recovery new | cloud recovery use KEY")
}

const teamUsage = `usage: enowx-app team COMMAND
  (none) | list                    your teams, and your role in each
  create NAME                      make a team; you own it
  members TEAM                     a team's members and roles (TEAM: its name or ID)
  invite TEAM [--role member] [--email E]
                                   a one-time invite link (role admin, member or viewer)
  invites TEAM                     the invites still open
  join LINK|TOKEN                  join a team with an invite
  role TEAM MEMBER ROLE            change a member's role (MEMBER: their GitHub login or ID)
  remove TEAM MEMBER               take a member out of a team
  leave TEAM                       leave a team
  sync [TEAM]                      sync a team's space (its notes and canvases) with the cloud now, and say how it went
  projects [TEAM]                  the teams' projects (Git links), and where each is on this host
  project add TEAM LINK [--name N] add a project to a team by its Git link (admins)
  project rm TEAM PROJECT          take a project out of a team (admins)
  project clone TEAM PROJECT [--dir D]
                                   clone a team's project here, unless it is here already
  hosts                            the hosts the teams' members share, and where they are
  chat TEAM [--last N] | chat TEAM --send TEXT
                                   the team's chat, or a message to it
  audit TEAM                       the team's log: sign-ins, invites, roles, shared hosts entered, sessions watched and taken over (admins)
  share [TEAM [PROJECT...]]        what this host shares; share projects with a team (none: unshare)
  kit [TEAM] | kit add TEAM skill|command|subagent NAME | kit rm TEAM KIND NAME
                                   the skills, commands and subagents a team shares with its projects' sessions`

// cloud signs the host in to a team cloud and out, through its daemon.
func (r runner) cloud(args []string) error {
	cmd := "status"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "")
	arg, err := parseWithArg(fs, args)
	if err != nil {
		return errors.New(cloudUsage)
	}
	if cmd == "recovery" {
		return r.cloudRecovery(args)
	}
	switch cmd {
	case "status", "whoami":
		st, err := r.c.CloudStatus()
		if err != nil {
			return err
		}
		r.cloudStatus(st)
		return nil
	case "url":
		if arg == "" {
			return errors.New(cloudUsage)
		}
		if arg == "off" {
			arg = ""
		}
		st, err := r.c.SetCloudURL(arg)
		if err != nil {
			return err
		}
		r.cloudStatus(st)
		return nil
	case "login":
		return r.cloudLogin(*name)
	case "logout":
		st, err := r.c.CloudSignOut()
		if err != nil {
			return err
		}
		r.cloudStatus(st)
		return nil
	case "devices":
		var list []api.Device
		if err := r.c.CloudCall("GET", api.PathDevices, nil, &list); err != nil {
			return err
		}
		w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tKIND\tLAST SEEN")
		for _, d := range list {
			name := d.Name
			if d.Current {
				name += " (this host)"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", d.ID, name, d.Kind, d.LastSeen.Local().Format("2006-01-02 15:04"))
		}
		return w.Flush()
	case "approve":
		if arg == "" {
			return errors.New(cloudUsage)
		}
		var pending api.PendingDevice
		if err := r.c.CloudCall("GET", "/v1/auth/device/pending/"+arg, nil, &pending); err != nil {
			return err
		}
		if err := r.c.CloudCall("POST", api.PathDeviceApprove, api.DeviceApproveParams{UserCode: arg}, nil); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "%s (%s) is signed in to your account.\n", pending.Name, pending.Kind)
		return nil
	}
	return errors.New(cloudUsage)
}

func (r runner) cloudStatus(st proto.CloudStatus) {
	switch {
	case st.URL == "":
		fmt.Fprintln(r.out, "Not signed in to a cloud: enowx-app cloud login")
	case st.SignedIn:
		who := st.Account
		if st.GitHub != "" {
			who += " (@" + st.GitHub + ")"
		}
		fmt.Fprintf(r.out, "Signed in to %s as %s, this host being %q (%s).\n", st.URL, who, st.DeviceName, st.DeviceKind)
	case st.Pending != nil:
		fmt.Fprintf(r.out, "Signing in to %s: approve code %s", st.URL, st.Pending.UserCode)
		if st.Pending.VerifyURLComplete != "" {
			fmt.Fprintf(r.out, " at %s", st.Pending.VerifyURLComplete)
		}
		fmt.Fprintln(r.out, ".")
	default:
		fmt.Fprintf(r.out, "Not signed in to %s: enowx-app cloud login\n", st.URL)
	}
	if st.Err != "" {
		fmt.Fprintln(r.out, st.Err)
	}
}

// cloudLogin signs the host in, showing the code to approve and waiting
// for it; Ctrl+C gives up.
func (r runner) cloudLogin(name string) error {
	st, err := r.c.CloudSignIn(proto.CloudSignInParams{Kind: proto.CloudDeviceHost, Name: name})
	if err != nil {
		return err
	}
	if st.Pending == nil {
		return errors.New("cloud: no code came to sign in with")
	}
	fmt.Fprintf(r.out, "Code: %s\n", st.Pending.UserCode)
	if st.Pending.VerifyURLComplete != "" {
		fmt.Fprintf(r.out, "Open %s and approve it,\nor approve it in the enowX app (Settings → Cloud), or with enowx-app cloud approve %s on a host signed in.\n", st.Pending.VerifyURLComplete, st.Pending.UserCode)
	} else {
		fmt.Fprintf(r.out, "Approve it in the enowX app (Settings → Cloud), or with enowx-app cloud approve %s on a host signed in.\n", st.Pending.UserCode)
	}
	fmt.Fprintf(r.out, "Waiting (until %s)…\n", st.Pending.Expires.Local().Format("15:04"))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			r.c.CloudCancel()
			return errors.New("cloud: sign-in given up")
		case <-time.After(2 * time.Second):
		}
		st, err = r.c.CloudStatus()
		if err != nil {
			return err
		}
		switch {
		case st.SignedIn:
			r.cloudStatus(st)
			return nil
		case st.Err != "":
			return errors.New(st.Err)
		case st.Pending == nil:
			return errors.New("cloud: the sign-in was dropped")
		}
	}
}

// team works with the account's teams in the cloud, through the daemon.
func (r runner) team(args []string) error {
	cmd := "list"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "projects":
		return r.teamProjects(args)
	case "project":
		return r.teamProject(args)
	case "kit":
		return r.teamKitCmd(args)
	case "hosts":
		return r.teamHostsCmd()
	case "audit":
		return r.teamAuditCmd(args)
	case "chat":
		return r.teamChatCmd(args)
	case "share":
		return r.teamShareCmd(args)
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	role := fs.String("role", api.RoleMember, "")
	email := fs.String("email", "", "")
	rest, err := parseInterspersed(fs, args)
	if err != nil {
		return errors.New(teamUsage)
	}
	need := func(n int) error {
		if len(rest) < n {
			return errors.New(teamUsage)
		}
		return nil
	}
	switch cmd {
	case "list":
		var list []api.Team
		if err := r.c.CloudCall("GET", api.PathTeams, nil, &list); err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(r.out, "No teams: enowx-app team create NAME, or join one with an invite.")
			return nil
		}
		w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tROLE")
		for _, t := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.ID, t.Name, t.Role)
		}
		return w.Flush()
	case "sync":
		spaces, err := r.c.Spaces()
		if err != nil {
			return err
		}
		if len(spaces) == 0 {
			return errors.New("team: no team's space on this host (signed in? \"enowx-app team list\" lists your teams)")
		}
		for _, sp := range spaces {
			if len(rest) > 0 && sp.Team != rest[0] && !strings.EqualFold(sp.Name, strings.Join(rest, " ")) {
				continue
			}
			got, err := r.c.SyncSpace(sp.Team)
			switch {
			case err != nil:
				fmt.Fprintf(r.out, "%s: %v\n", sp.Name, err)
			case got.SyncErr != "":
				fmt.Fprintf(r.out, "%s: synced; %s\n", sp.Name, got.SyncErr)
			default:
				fmt.Fprintf(r.out, "%s: synced\n", sp.Name)
			}
		}
		return nil
	case "create":
		if err := need(1); err != nil {
			return err
		}
		name := strings.Join(rest, " ")
		var t api.Team
		if err := r.c.CloudCall("POST", api.PathTeams, api.TeamParams{Name: &name}, &t); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "Made team %s (%s); you own it.\n", t.Name, t.ID)
		return nil
	case "members":
		if err := need(1); err != nil {
			return err
		}
		t, err := r.findTeam(rest[0])
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "MEMBER\tGITHUB\tROLE\tJOINED")
		for _, m := range t.Members {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Account.Name, m.Account.GitHub, m.Role, m.Joined.Local().Format("2006-01-02"))
		}
		return w.Flush()
	case "invite":
		if err := need(1); err != nil {
			return err
		}
		t, err := r.findTeam(rest[0])
		if err != nil {
			return err
		}
		var in api.Invite
		if err := r.c.CloudCall("POST", teamPath(api.PathInvites, t.ID), api.InviteParams{Role: *role, Email: *email}, &in); err != nil {
			return err
		}
		link := in.URL
		if link == "" {
			link = in.Token
		}
		fmt.Fprintf(r.out, "Invite to %s as %s, for one person, until %s:\n%s\n", t.Name, in.Role, in.Expires.Local().Format("2006-01-02 15:04"), link)
		return nil
	case "invites":
		if err := need(1); err != nil {
			return err
		}
		t, err := r.findTeam(rest[0])
		if err != nil {
			return err
		}
		var list []api.Invite
		if err := r.c.CloudCall("GET", teamPath(api.PathInvites, t.ID), nil, &list); err != nil {
			return err
		}
		w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tROLE\tEMAIL\tEXPIRES")
		for _, in := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", in.ID, in.Role, in.Email, in.Expires.Local().Format("2006-01-02 15:04"))
		}
		return w.Flush()
	case "join":
		if err := need(1); err != nil {
			return err
		}
		var t api.Team
		if err := r.c.CloudCall("POST", api.PathJoin, api.JoinParams{Token: rest[0]}, &t); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "Joined %s as %s.\n", t.Name, t.Role)
		return nil
	case "role", "remove", "leave":
		n := map[string]int{"role": 3, "remove": 2, "leave": 1}[cmd]
		if err := need(n); err != nil {
			return err
		}
		t, err := r.findTeam(rest[0])
		if err != nil {
			return err
		}
		var who string
		if cmd == "leave" {
			st, err := r.c.CloudStatus()
			if err != nil {
				return err
			}
			who = st.AccountID
		} else if who, err = memberID(t, rest[1]); err != nil {
			return err
		}
		path := strings.Replace(teamPath(api.PathMember, t.ID), "{account}", who, 1)
		if cmd == "role" {
			return r.c.CloudCall("PATCH", path, api.RoleParams{Role: rest[2]}, nil)
		}
		return r.c.CloudCall("DELETE", path, nil, nil)
	}
	return errors.New(teamUsage)
}

func teamPath(p, team string) string { return strings.Replace(p, "{team}", team, 1) }

// findTeam is the team named, or of that ID, with its members.
func (r runner) findTeam(nameOrID string) (api.Team, error) {
	var list []api.Team
	if err := r.c.CloudCall("GET", api.PathTeams, nil, &list); err != nil {
		return api.Team{}, err
	}
	for _, t := range list {
		if t.ID == nameOrID || strings.EqualFold(t.Name, nameOrID) {
			var full api.Team
			err := r.c.CloudCall("GET", teamPath(api.PathTeam, t.ID), nil, &full)
			return full, err
		}
	}
	return api.Team{}, fmt.Errorf("cloud: you have no team %q (enowx-app team list)", nameOrID)
}

// memberID is the account ID of a team's member, by GitHub login, name or ID.
func memberID(t api.Team, who string) (string, error) {
	who = strings.TrimPrefix(who, "@")
	for _, m := range t.Members {
		if m.Account.ID == who || strings.EqualFold(m.Account.GitHub, who) || strings.EqualFold(m.Account.Name, who) {
			return m.Account.ID, nil
		}
	}
	return "", fmt.Errorf("cloud: %s has no member %q", t.Name, who)
}

// parseInterspersed parses fs's flags wherever they are among args, and
// returns the other arguments in order: team invite TEAM --role viewer.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var rest []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return rest, nil
		}
		rest = append(rest, fs.Arg(0))
		args = fs.Args()[1:]
	}
}
