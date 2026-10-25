package cli

import (
	"errors"
	"flag"
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/enowx/enowx/proto"
)

// The rest of what the Traffic tool does, for agents: the rules' other
// actions, the interceptor's settings, its CA, pins and saved filters.

func headerList(hs []string) []proto.Header {
	var out []proto.Header
	for _, h := range hs {
		k, v, _ := strings.Cut(h, ":")
		out = append(out, proto.Header{Name: strings.TrimSpace(k), Value: strings.TrimSpace(v)})
	}
	return out
}

// transformFlags are a transform rule's flags, read into its request's
// and response's changes.
func transformFlags(fs *flag.FlagSet) func() (*proto.TrafficTransform, *proto.TrafficTransform, error) {
	var reqSet, reqRemove, resSet, resRemove, replace, resJSON multiFlag
	fs.Var(&reqSet, "req-set", "")
	fs.Var(&reqRemove, "req-remove", "")
	fs.Var(&resSet, "res-set", "")
	fs.Var(&resRemove, "res-remove", "")
	fs.Var(&replace, "replace", "")
	fs.Var(&resJSON, "res-json", "")
	resStatus := fs.Int("res-status", 0, "")
	reqURL := fs.String("req-url", "", "")
	return func() (*proto.TrafficTransform, *proto.TrafficTransform, error) {
		req := &proto.TrafficTransform{SetHeaders: headerList(reqSet), RemoveHeaders: reqRemove, URL: *reqURL}
		res := &proto.TrafficTransform{SetHeaders: headerList(resSet), RemoveHeaders: resRemove, Status: *resStatus}
		for _, r := range replace {
			from, to, ok := strings.Cut(r, "=>")
			if !ok {
				return nil, nil, fmt.Errorf("traffic: --replace %q: FROM=>TO", r)
			}
			res.Replace = append(res.Replace, proto.TrafficReplace{From: from, To: to})
		}
		for _, j := range resJSON {
			k, v, ok := strings.Cut(j, "=")
			if !ok {
				return nil, nil, fmt.Errorf("traffic: --res-json %q: a.b=VALUE (JSON, or text)", j)
			}
			if res.JSON == nil {
				res.JSON = map[string]string{}
			}
			res.JSON[k] = v
		}
		empty := func(t *proto.TrafficTransform) bool {
			return len(t.SetHeaders)+len(t.RemoveHeaders)+len(t.Replace)+len(t.JSON) == 0 && t.Status == 0 && t.URL == "" && t.Body == nil
		}
		if empty(req) {
			req = nil
		}
		if empty(res) {
			res = nil
		}
		return req, res, nil
	}
}

func (r runner) trafficSettings(args []string) error {
	st, err := r.c.TrafficSettings()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "insecure\t%s\n", orNone(strings.Join(st.Insecure, ", ")))
		fmt.Fprintf(w, "upstream\t%s\n", orNone(st.Upstream))
		fmt.Fprintf(w, "max exchanges\t%d\n", st.MaxExchanges)
		fmt.Fprintf(w, "max MB\t%d\n", st.MaxMB)
		fmt.Fprintf(w, "notify errors\t%v\n", st.NotifyErrors)
		for name, addr := range st.Hosts {
			fmt.Fprintf(w, "host\t%s = %s\n", name, addr)
		}
		for _, c := range st.ClientCerts {
			fmt.Fprintf(w, "client cert\t%s %s %s\n", c.Host, c.Cert, c.Key)
		}
		return w.Flush()
	}
	if args[0] != "set" {
		return errors.New("usage: enowx-app traffic settings [set …] (traffic help)")
	}
	fs := flagSet("settings")
	insecure := fs.String("insecure", "\x00", "")
	upstream := fs.String("upstream", "\x00", "")
	var hostsFlag, certs multiFlag
	fs.Var(&hostsFlag, "host", "")
	fs.Var(&certs, "client-cert", "")
	maxEx := fs.Int("max-exchanges", -1, "")
	maxMB := fs.Int("max-mb", -1, "")
	notify := fs.String("notify-errors", "", "")
	clearHosts := fs.Bool("clear-hosts", false, "")
	clearCerts := fs.Bool("clear-client-certs", false, "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	// What isn't given stays as it is.
	if *insecure != "\x00" {
		st.Insecure = splitComma(*insecure)
	}
	if *upstream != "\x00" {
		st.Upstream = strings.TrimSpace(*upstream)
	}
	if *maxEx >= 0 {
		st.MaxExchanges = *maxEx
	}
	if *maxMB >= 0 {
		st.MaxMB = *maxMB
	}
	switch *notify {
	case "on", "true":
		st.NotifyErrors = true
	case "off", "false":
		st.NotifyErrors = false
	case "":
	default:
		return errors.New("traffic: --notify-errors on|off")
	}
	if *clearHosts {
		st.Hosts = nil
	}
	for _, h := range hostsFlag {
		name, addr, ok := strings.Cut(h, "=")
		if !ok {
			return fmt.Errorf("traffic: --host %q: NAME=ADDRESS", h)
		}
		if st.Hosts == nil {
			st.Hosts = map[string]string{}
		}
		st.Hosts[strings.TrimSpace(name)] = strings.TrimSpace(addr)
	}
	if *clearCerts {
		st.ClientCerts = nil
	}
	for _, c := range certs {
		f := strings.Fields(c)
		if len(f) != 3 {
			return fmt.Errorf("traffic: --client-cert %q: 'HOST CERT.pem KEY.pem'", c)
		}
		st.ClientCerts = append(st.ClientCerts, proto.TrafficClientCert{Host: f[0], Cert: f[1], Key: f[2]})
	}
	if _, err := r.c.SetTrafficSettings(st); err != nil {
		return err
	}
	fmt.Fprintln(r.out, "Saved.")
	return nil
}

func splitComma(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func (r runner) trafficCA(args []string) error {
	fs := flagSet("ca")
	pem := fs.Bool("pem", false, "")
	renew := fs.Bool("new", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	get := r.c.TrafficCA
	if *renew {
		get = r.c.TrafficNewCA
	}
	ca, err := get()
	if err != nil {
		return err
	}
	if *pem {
		fmt.Fprint(r.out, ca.PEM)
		return nil
	}
	fmt.Fprintf(r.out, "SHA-256 %s\nfile %s\nwith the system's roots %s\n", ca.Fingerprint, ca.File, ca.Bundle)
	if *renew {
		fmt.Fprintln(r.out, "A new CA: browsers, devices and containers attached before must be attached again to trust it.")
	}
	return nil
}

func (r runner) trafficPin(pin bool, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: enowx-app traffic pin|unpin ID [ID…]")
	}
	var ids []int64
	for _, a := range args {
		id, err := parseID(a)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	return r.c.TrafficPin(ids, pin)
}

func (r runner) trafficFilters(args []string) error {
	fs := flagSet("filters")
	project := fs.String("project", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	proj := r.project(*project)
	rules, err := r.c.TrafficRules(proj)
	if err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		for _, f := range rules.Filters {
			fmt.Fprintln(r.out, f)
		}
		return nil
	}
	if len(rest) < 2 || (rest[0] != "add" && rest[0] != "rm") {
		return errors.New("usage: enowx-app traffic filters [add|rm FILTER]")
	}
	f := strings.Join(rest[1:], " ")
	if rest[0] == "add" {
		if !slices.Contains(rules.Filters, f) {
			rules.Filters = append(rules.Filters, f)
		}
	} else {
		rules.Filters = slices.DeleteFunc(rules.Filters, func(x string) bool { return x == f })
	}
	rules.Project = proj
	_, err = r.c.SetTrafficRules(rules)
	return err
}
