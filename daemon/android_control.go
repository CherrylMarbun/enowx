package daemon

import (
	"fmt"
	"strings"
	"time"

	"github.com/enowx/enowx/core/android"
	"github.com/enowx/enowx/proto"
)

// An agent's use of a device (enowx-app android screenshot, controls,
// tap…): adb on the device's host, the controls last read kept by device
// for the numbers they were given.

func (s *Server) androidScreenshot(p proto.AndroidDeviceParams) (proto.AndroidScreenshot, error) {
	sdk, err := s.androidSDK()
	if err != nil {
		return proto.AndroidScreenshot{}, err
	}
	png, err := sdk.Screenshot(p.Serial)
	return proto.AndroidScreenshot{PNG: png}, err
}

func (s *Server) androidControls(p proto.AndroidDeviceParams) (proto.AndroidControls, error) {
	sdk, err := s.androidSDK()
	if err != nil {
		return proto.AndroidControls{}, err
	}
	list, err := sdk.Controls(p.Serial)
	if err != nil {
		return proto.AndroidControls{}, err
	}
	s.android.mu.Lock()
	if s.android.controls == nil {
		s.android.controls = map[string][]android.Control{}
	}
	s.android.controls[p.Serial] = list
	s.android.mu.Unlock()
	return proto.AndroidControls{Text: android.ControlsText(list), Count: len(list)}, nil
}

// at is where act's Ref is, else its X, Y.
func (s *Server) at(p proto.AndroidAct) (int, int, error) {
	if p.Ref == 0 {
		return p.X, p.Y, nil
	}
	s.android.mu.Lock()
	list := s.android.controls[p.Serial]
	s.android.mu.Unlock()
	if p.Ref < 1 || p.Ref > len(list) {
		return 0, 0, fmt.Errorf("android: no control %d: read the controls (android controls) first", p.Ref)
	}
	c := list[p.Ref-1]
	return c.X, c.Y, nil
}

func (s *Server) androidAct(p proto.AndroidAct) error {
	sdk, err := s.androidSDK()
	if err != nil {
		return err
	}
	ms := func(def int) time.Duration {
		if p.Ms > 0 {
			return time.Duration(p.Ms) * time.Millisecond
		}
		return time.Duration(def) * time.Millisecond
	}
	switch p.Action {
	case proto.AndroidTap, proto.AndroidLongPress:
		x, y, err := s.at(p)
		if err != nil {
			return err
		}
		if p.Action == proto.AndroidLongPress {
			return sdk.Swipe(p.Serial, x, y, x, y, ms(800))
		}
		return sdk.Tap(p.Serial, x, y)
	case proto.AndroidSwipe:
		return sdk.Swipe(p.Serial, p.X, p.Y, p.X2, p.Y2, ms(300))
	case proto.AndroidType:
		if p.Ref != 0 {
			x, y, err := s.at(p)
			if err != nil {
				return err
			}
			if err := sdk.Tap(p.Serial, x, y); err != nil {
				return err
			}
			time.Sleep(300 * time.Millisecond) // the field takes focus
		}
		return sdk.TypeText(p.Serial, p.Text)
	case proto.AndroidPress:
		code, err := android.KeyCode(strings.TrimSpace(p.Key))
		if err != nil {
			return err
		}
		return sdk.Press(p.Serial, code)
	}
	return fmt.Errorf("android: no action %q", p.Action)
}
