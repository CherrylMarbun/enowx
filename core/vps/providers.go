package vps

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Each provider's API, as its documentation describes it: Hetzner Cloud's
// (docs.hetzner.cloud), DigitalOcean's (docs.digitalocean.com/reference/
// api) and Vultr's (www.vultr.com/api). Servers get Ubuntu 24.04.

type hetzner struct{ *api }

func (h hetzner) Regions(ctx context.Context) ([]Option, error) {
	var out struct {
		Locations []struct {
			Name, Description, City, Country string
		} `json:"locations"`
	}
	if err := h.do(ctx, "GET", "/locations", nil, &out); err != nil {
		return nil, err
	}
	var opts []Option
	for _, l := range out.Locations {
		opts = append(opts, Option{ID: l.Name, Name: l.City + ", " + l.Country, Detail: l.Name})
	}
	return opts, nil
}

func (h hetzner) Sizes(ctx context.Context, region string) ([]Option, error) {
	var out struct {
		ServerTypes []struct {
			Name         string  `json:"name"`
			Cores        int     `json:"cores"`
			Memory       float64 `json:"memory"`
			Disk         int     `json:"disk"`
			Architecture string  `json:"architecture"`
			Deprecated   bool    `json:"deprecated"`
			Prices       []struct {
				Location string `json:"location"`
				Monthly  struct {
					Gross string `json:"gross"`
				} `json:"price_monthly"`
			} `json:"prices"`
		} `json:"server_types"`
	}
	if err := h.do(ctx, "GET", "/server_types?per_page=50", nil, &out); err != nil {
		return nil, err
	}
	var opts []Option
	for _, t := range out.ServerTypes {
		if t.Deprecated {
			continue
		}
		price, offered := 0.0, false
		for _, p := range t.Prices {
			if p.Location == region {
				price, _ = strconv.ParseFloat(p.Monthly.Gross, 64)
				offered = true
			}
		}
		if !offered {
			continue
		}
		opts = append(opts, Option{
			ID: t.Name, Name: strings.ToUpper(t.Name), Monthly: price,
			Detail: fmt.Sprintf("%d vCPU · %s GB · %d GB disk · %s", t.Cores, trimFloat(t.Memory), t.Disk, t.Architecture),
		})
	}
	sortByPrice(opts)
	return opts, nil
}

func (h hetzner) key(ctx context.Context, public string) (int64, error) {
	var keys struct {
		SSHKeys []struct {
			ID        int64  `json:"id"`
			PublicKey string `json:"public_key"`
		} `json:"ssh_keys"`
	}
	if err := h.do(ctx, "GET", "/ssh_keys?per_page=50", nil, &keys); err != nil {
		return 0, err
	}
	for _, k := range keys.SSHKeys {
		if keyBody(k.PublicKey) == keyBody(public) {
			return k.ID, nil
		}
	}
	var made struct {
		SSHKey struct {
			ID int64 `json:"id"`
		} `json:"ssh_key"`
	}
	err := h.do(ctx, "POST", "/ssh_keys", map[string]any{"name": uniqueKeyName(), "public_key": strings.TrimSpace(public)}, &made)
	return made.SSHKey.ID, err
}

type hetznerServer struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	PublicNet struct {
		IPv4 struct {
			IP string `json:"ip"`
		} `json:"ipv4"`
	} `json:"public_net"`
}

func (s hetznerServer) server() Server {
	return Server{ID: strconv.FormatInt(s.ID, 10), Name: s.Name, Status: s.Status, IPv4: s.PublicNet.IPv4.IP,
		Ready: s.Status == "running" && s.PublicNet.IPv4.IP != ""}
}

func (h hetzner) Create(ctx context.Context, p CreateParams) (Server, error) {
	key, err := h.key(ctx, p.PublicKey)
	if err != nil {
		return Server{}, err
	}
	var out struct {
		Server hetznerServer `json:"server"`
	}
	err = h.do(ctx, "POST", "/servers", map[string]any{
		"name": p.Name, "server_type": p.Size, "location": p.Region, "image": "ubuntu-24.04",
		"ssh_keys": []int64{key}, "start_after_create": true,
	}, &out)
	return out.Server.server(), err
}

func (h hetzner) Server(ctx context.Context, id string) (Server, error) {
	var out struct {
		Server hetznerServer `json:"server"`
	}
	err := h.do(ctx, "GET", "/servers/"+url.PathEscape(id), nil, &out)
	return out.Server.server(), err
}

func (h hetzner) Delete(ctx context.Context, id string) error {
	return h.do(ctx, "DELETE", "/servers/"+url.PathEscape(id), nil, nil)
}

type digitalOcean struct{ *api }

func (d digitalOcean) Regions(ctx context.Context) ([]Option, error) {
	var out struct {
		Regions []struct {
			Slug, Name string
			Available  bool `json:"available"`
		} `json:"regions"`
	}
	if err := d.do(ctx, "GET", "/regions?per_page=200", nil, &out); err != nil {
		return nil, err
	}
	var opts []Option
	for _, r := range out.Regions {
		if r.Available {
			opts = append(opts, Option{ID: r.Slug, Name: r.Name, Detail: r.Slug})
		}
	}
	return opts, nil
}

func (d digitalOcean) Sizes(ctx context.Context, region string) ([]Option, error) {
	var out struct {
		Sizes []struct {
			Slug        string   `json:"slug"`
			Memory      int      `json:"memory"`
			VCPUs       int      `json:"vcpus"`
			Disk        int      `json:"disk"`
			Monthly     float64  `json:"price_monthly"`
			Regions     []string `json:"regions"`
			Available   bool     `json:"available"`
			Description string   `json:"description"`
		} `json:"sizes"`
	}
	if err := d.do(ctx, "GET", "/sizes?per_page=200", nil, &out); err != nil {
		return nil, err
	}
	var opts []Option
	for _, s := range out.Sizes {
		if !s.Available || !slices.Contains(s.Regions, region) {
			continue
		}
		opts = append(opts, Option{
			ID: s.Slug, Name: s.Slug, Monthly: s.Monthly,
			Detail: fmt.Sprintf("%d vCPU · %s GB · %d GB disk", s.VCPUs, trimFloat(float64(s.Memory)/1024), s.Disk),
		})
	}
	sortByPrice(opts)
	return opts, nil
}

func (d digitalOcean) key(ctx context.Context, public string) (int64, error) {
	var keys struct {
		SSHKeys []struct {
			ID        int64  `json:"id"`
			PublicKey string `json:"public_key"`
		} `json:"ssh_keys"`
	}
	if err := d.do(ctx, "GET", "/account/keys?per_page=200", nil, &keys); err != nil {
		return 0, err
	}
	for _, k := range keys.SSHKeys {
		if keyBody(k.PublicKey) == keyBody(public) {
			return k.ID, nil
		}
	}
	var made struct {
		SSHKey struct {
			ID int64 `json:"id"`
		} `json:"ssh_key"`
	}
	err := d.do(ctx, "POST", "/account/keys", map[string]any{"name": uniqueKeyName(), "public_key": strings.TrimSpace(public)}, &made)
	return made.SSHKey.ID, err
}

type droplet struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Networks struct {
		V4 []struct {
			IP   string `json:"ip_address"`
			Type string `json:"type"`
		} `json:"v4"`
	} `json:"networks"`
}

func (dr droplet) server() Server {
	s := Server{ID: strconv.FormatInt(dr.ID, 10), Name: dr.Name, Status: dr.Status}
	for _, n := range dr.Networks.V4 {
		if n.Type == "public" {
			s.IPv4 = n.IP
		}
	}
	s.Ready = dr.Status == "active" && s.IPv4 != ""
	return s
}

func (d digitalOcean) Create(ctx context.Context, p CreateParams) (Server, error) {
	key, err := d.key(ctx, p.PublicKey)
	if err != nil {
		return Server{}, err
	}
	var out struct {
		Droplet droplet `json:"droplet"`
	}
	err = d.do(ctx, "POST", "/droplets", map[string]any{
		"name": p.Name, "region": p.Region, "size": p.Size, "image": "ubuntu-24-04-x64", "ssh_keys": []int64{key},
	}, &out)
	return out.Droplet.server(), err
}

func (d digitalOcean) Server(ctx context.Context, id string) (Server, error) {
	var out struct {
		Droplet droplet `json:"droplet"`
	}
	err := d.do(ctx, "GET", "/droplets/"+url.PathEscape(id), nil, &out)
	return out.Droplet.server(), err
}

func (d digitalOcean) Delete(ctx context.Context, id string) error {
	return d.do(ctx, "DELETE", "/droplets/"+url.PathEscape(id), nil, nil)
}

type vultr struct{ *api }

func (v vultr) Regions(ctx context.Context) ([]Option, error) {
	var out struct {
		Regions []struct {
			ID, City, Country, Continent string
		} `json:"regions"`
	}
	if err := v.do(ctx, "GET", "/regions?per_page=500", nil, &out); err != nil {
		return nil, err
	}
	var opts []Option
	for _, r := range out.Regions {
		opts = append(opts, Option{ID: r.ID, Name: r.City + ", " + r.Country, Detail: r.ID})
	}
	return opts, nil
}

func (v vultr) Sizes(ctx context.Context, region string) ([]Option, error) {
	var out struct {
		Plans []struct {
			ID        string   `json:"id"`
			VCPUs     int      `json:"vcpu_count"`
			RAM       int      `json:"ram"`
			Disk      int      `json:"disk"`
			Monthly   float64  `json:"monthly_cost"`
			Locations []string `json:"locations"`
		} `json:"plans"`
	}
	if err := v.do(ctx, "GET", "/plans?type=vc2&per_page=500", nil, &out); err != nil {
		return nil, err
	}
	var opts []Option
	for _, p := range out.Plans {
		if !slices.Contains(p.Locations, region) {
			continue
		}
		opts = append(opts, Option{
			ID: p.ID, Name: p.ID, Monthly: p.Monthly,
			Detail: fmt.Sprintf("%d vCPU · %s GB · %d GB disk", p.VCPUs, trimFloat(float64(p.RAM)/1024), p.Disk),
		})
	}
	sortByPrice(opts)
	return opts, nil
}

func (v vultr) key(ctx context.Context, public string) (string, error) {
	var keys struct {
		SSHKeys []struct {
			ID     string `json:"id"`
			SSHKey string `json:"ssh_key"`
		} `json:"ssh_keys"`
	}
	if err := v.do(ctx, "GET", "/ssh-keys?per_page=500", nil, &keys); err != nil {
		return "", err
	}
	for _, k := range keys.SSHKeys {
		if keyBody(k.SSHKey) == keyBody(public) {
			return k.ID, nil
		}
	}
	var made struct {
		SSHKey struct {
			ID string `json:"id"`
		} `json:"ssh_key"`
	}
	err := v.do(ctx, "POST", "/ssh-keys", map[string]any{"name": uniqueKeyName(), "ssh_key": strings.TrimSpace(public)}, &made)
	return made.SSHKey.ID, err
}

// ubuntu is the ID of Vultr's Ubuntu 24.04 for x64.
func (v vultr) ubuntu(ctx context.Context) (int, error) {
	var out struct {
		OS []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Arch string `json:"arch"`
		} `json:"os"`
	}
	if err := v.do(ctx, "GET", "/os?per_page=500", nil, &out); err != nil {
		return 0, err
	}
	for _, o := range out.OS {
		if strings.HasPrefix(o.Name, "Ubuntu 24.04") && o.Arch == "x64" {
			return o.ID, nil
		}
	}
	return 0, fmt.Errorf("vps: Vultr offers no Ubuntu 24.04 x64")
}

type instance struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	MainIP       string `json:"main_ip"`
	Status       string `json:"status"`
	PowerStatus  string `json:"power_status"`
	ServerStatus string `json:"server_status"`
}

func (in instance) server() Server {
	s := Server{ID: in.ID, Name: in.Label, Status: in.Status}
	if in.MainIP != "0.0.0.0" {
		s.IPv4 = in.MainIP
	}
	s.Ready = in.Status == "active" && in.PowerStatus == "running" && s.IPv4 != ""
	return s
}

func (v vultr) Create(ctx context.Context, p CreateParams) (Server, error) {
	key, err := v.key(ctx, p.PublicKey)
	if err != nil {
		return Server{}, err
	}
	osID, err := v.ubuntu(ctx)
	if err != nil {
		return Server{}, err
	}
	var out struct {
		Instance instance `json:"instance"`
	}
	err = v.do(ctx, "POST", "/instances", map[string]any{
		"region": p.Region, "plan": p.Size, "os_id": osID, "label": p.Name, "hostname": p.Name, "sshkey_id": []string{key},
	}, &out)
	return out.Instance.server(), err
}

func (v vultr) Server(ctx context.Context, id string) (Server, error) {
	var out struct {
		Instance instance `json:"instance"`
	}
	err := v.do(ctx, "GET", "/instances/"+url.PathEscape(id), nil, &out)
	return out.Instance.server(), err
}

func (v vultr) Delete(ctx context.Context, id string) error {
	return v.do(ctx, "DELETE", "/instances/"+url.PathEscape(id), nil, nil)
}

func sortByPrice(opts []Option) {
	slices.SortStableFunc(opts, func(a, b Option) int {
		switch {
		case a.Monthly < b.Monthly:
			return -1
		case a.Monthly > b.Monthly:
			return 1
		}
		return 0
	})
}

func trimFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// uniqueKeyName names the user's key at a provider; providers refuse two
// keys of one name, so a key added from another device gets its own.
func uniqueKeyName() string { return keyName + " " + strconv.FormatInt(timeNow().Unix(), 36) }
