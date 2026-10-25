package daemon

import (
	"context"
	"errors"
	"os"
	"slices"
	"time"

	"github.com/enowx/enowx/core/intercept"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/version"
)

// What the interceptor recorded, for the Traffic tool, the command line
// and agents' tools.

const (
	trafficPage     = 500
	trafficBodyPart = 1 << 20
	trafficSendWait = 2 * time.Minute
)

func (s *Server) trafficList(p proto.TrafficListParams) (proto.TrafficList, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficList{}, err
	}
	f, err := intercept.ParseFilter(p.Filter)
	if err != nil {
		return proto.TrafficList{}, err
	}
	limit := p.Limit
	if limit <= 0 || limit > trafficPage {
		limit = trafficPage
	}
	out := proto.TrafficList{Exchanges: []proto.TrafficSummary{}}
	for _, ex := range t.proxy.Store.All() {
		if ex.ID() <= p.After || !f.Match(ex) {
			continue
		}
		if len(out.Exchanges) == limit {
			out.More = true
			break
		}
		sum := ex.Summary()
		sum.URL = intercept.MaskURL(sum.URL)
		out.Exchanges = append(out.Exchanges, sum)
	}
	return out, nil
}

func (s *Server) trafficExchange(id int64) (*intercept.Exchange, error) {
	t, err := s.trafficOn()
	if err != nil {
		return nil, err
	}
	ex, ok := t.proxy.Store.Get(id)
	if !ok {
		return nil, errors.New("traffic: that exchange is gone (cleared, or past the capture's limit)")
	}
	return ex, nil
}

func (s *Server) trafficGet(p proto.TrafficRef) (proto.TrafficExchange, error) {
	ex, err := s.trafficExchange(p.ID)
	if err != nil {
		return proto.TrafficExchange{}, err
	}
	return ex.Details(p.Reveal), nil
}

func (s *Server) trafficBody(p proto.TrafficBodyParams) (proto.TrafficBody, error) {
	ex, err := s.trafficExchange(p.ID)
	if err != nil {
		return proto.TrafficBody{}, err
	}
	size := p.Size
	if size <= 0 || size > 4*trafficBodyPart {
		size = trafficBodyPart
	}
	return ex.Body(p.Which, !p.Raw, p.Reveal, p.Offset, size), nil
}

func (s *Server) trafficMessages(p proto.TrafficMessagesParams) ([]proto.TrafficMessage, error) {
	ex, err := s.trafficExchange(p.ID)
	if err != nil {
		return nil, err
	}
	limit := p.Limit
	if limit <= 0 || limit > 2000 {
		limit = 2000
	}
	return nonNil(ex.Messages(p.Offset, limit)), nil
}

func (s *Server) trafficPin(p proto.TrafficPinParams) error {
	for _, id := range p.IDs {
		ex, err := s.trafficExchange(id)
		if err != nil {
			continue
		}
		ex.SetPinned(p.Pinned)
	}
	return nil
}

func (s *Server) trafficClear(p proto.TrafficClearParams) error {
	t, err := s.trafficOn()
	if err != nil {
		return err
	}
	f, err := intercept.ParseFilter(p.Filter)
	if err != nil {
		return err
	}
	match := f.Match
	if p.Filter == "" {
		match = nil
	}
	t.proxy.Store.Clear(match, p.Pinned)
	s.broadcast(proto.EventTrafficExchange, proto.TrafficSummary{State: "cleared"})
	return nil
}

func (s *Server) trafficPaused() ([]proto.TrafficPaused, error) {
	t, err := s.trafficOn()
	if err != nil {
		return nil, err
	}
	return nonNil(t.proxy.Paused()), nil
}

func (s *Server) trafficResume(p proto.TrafficResumeParams) error {
	t, err := s.trafficOn()
	if err != nil {
		return err
	}
	return t.proxy.Resume(p.ID, p.Drop, p.Edit)
}

// trafficSend sends a request from this host, as the session's (or a
// compose source of its own).
func (s *Server) trafficSend(p proto.TrafficSendParams) (proto.TrafficSummary, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficSummary{}, err
	}
	src := intercept.Source{ID: "compose", Kind: "compose", Label: "Compose", Session: p.Session, Project: p.Project}
	ctx, cancel := context.WithTimeout(context.Background(), trafficSendWait)
	defer cancel()
	return t.proxy.Send(ctx, src, p)
}

// trafficExport writes the exchanges as HAR into a transfer, which the
// asker reads with transfer.read.
func (s *Server) trafficExport(p proto.TrafficExportParams) (proto.TransferInfo, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TransferInfo{}, err
	}
	var exs []*intercept.Exchange
	if len(p.IDs) > 0 {
		for _, id := range p.IDs {
			if ex, ok := t.proxy.Store.Get(id); ok {
				exs = append(exs, ex)
			}
		}
	} else {
		f, err := intercept.ParseFilter(p.Filter)
		if err != nil {
			return proto.TransferInfo{}, err
		}
		for _, ex := range t.proxy.Store.All() {
			if f.Match(ex) {
				exs = append(exs, ex)
			}
		}
	}
	data, err := intercept.ExportHAR(exs, version.String, p.Reveal)
	if err != nil {
		return proto.TransferInfo{}, err
	}
	id := newTransferID()
	file, _ := s.transferFile(id)
	if err := os.MkdirAll(s.transferDir(), 0o700); err != nil {
		return proto.TransferInfo{}, err
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return proto.TransferInfo{}, err
	}
	return proto.TransferInfo{ID: id, Size: int64(len(data)), Files: len(exs)}, nil
}

// trafficImport reads a HAR written with transfer.write.
func (s *Server) trafficImport(p proto.TrafficImportParams) (proto.TrafficList, error) {
	t, err := s.trafficOn()
	if err != nil {
		return proto.TrafficList{}, err
	}
	file, err := s.transferFile(p.ID)
	if err != nil {
		return proto.TrafficList{}, err
	}
	defer os.Remove(file)
	data, err := os.ReadFile(file)
	if err != nil {
		return proto.TrafficList{}, err
	}
	list, err := t.proxy.Store.ImportHAR(data, "import")
	if err != nil {
		return proto.TrafficList{}, err
	}
	slices.SortFunc(list, func(a, b proto.TrafficSummary) int { return int(a.ID - b.ID) })
	return proto.TrafficList{Exchanges: nonNil(list)}, nil
}
