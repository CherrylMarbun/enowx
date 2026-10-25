package daemon

import (
	"errors"

	"github.com/enowx/enowx/proto"
)

// Targets that come with their own machinery: Android devices (the
// mobile tool) and Docker containers.

func (s *Server) attachAndroid(tg *trafficTarget) error {
	return errors.New("traffic: Android devices aren't caught yet")
}

func (s *Server) detachAndroid(tg *trafficTarget) {}

func (s *Server) attachDocker(p proto.TrafficAttachParams) (proto.TrafficTarget, error) {
	return proto.TrafficTarget{}, errors.New("traffic: Docker containers aren't caught yet")
}
