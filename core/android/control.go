package android

import (
	"context"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// What an agent uses a device with, through adb: the screen as a PNG, its
// controls as text (uiautomator's view tree), and touches, swipes, keys
// and text sent as input.

// Screenshot is device serial's screen as a PNG.
func (s SDK) Screenshot(serial string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	out, err := s.command(ctx, s.ADB(), "-s", serial, "exec-out", "screencap", "-p").Output()
	if err != nil {
		return nil, fmt.Errorf("android: screenshot of %s: %w", serial, err)
	}
	if len(out) < 8 || string(out[1:4]) != "PNG" {
		return nil, fmt.Errorf("android: %s's screenshot isn't a PNG: %.80q", serial, out)
	}
	return out, nil
}

// Control is a control on a device's screen: what it says and where.
type Control struct {
	Ref                 int
	Class, Text, ID     string
	Desc                string
	X, Y                int // its center, where a tap goes
	Clickable, Editable bool
	Checked, Focused    bool
	Scrollable          bool
}

type uiNode struct {
	Class      string   `xml:"class,attr"`
	Text       string   `xml:"text,attr"`
	ID         string   `xml:"resource-id,attr"`
	Desc       string   `xml:"content-desc,attr"`
	Clickable  string   `xml:"clickable,attr"`
	Checkable  string   `xml:"checkable,attr"`
	Checked    string   `xml:"checked,attr"`
	Focused    string   `xml:"focused,attr"`
	Scrollable string   `xml:"scrollable,attr"`
	Bounds     string   `xml:"bounds,attr"`
	Nodes      []uiNode `xml:"node"`
}

var boundsRe = regexp.MustCompile(`\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]`)

// Controls are device serial's controls now: those that say something or
// can be used, in the screen's order, numbered from 1.
func (s SDK) Controls(serial string) ([]Control, error) {
	path := "/sdcard/enowx-ui.xml"
	if out, err := s.shell(serial, "uiautomator dump "+path); err != nil {
		return nil, fmt.Errorf("android: reading %s's screen: %s", serial, strings.TrimSpace(out))
	}
	out, err := s.shell(serial, "cat "+path)
	if err != nil {
		return nil, err
	}
	var root struct {
		Nodes []uiNode `xml:"node"`
	}
	if err := xml.Unmarshal([]byte(out), &root); err != nil {
		return nil, fmt.Errorf("android: %s's view tree: %w", serial, err)
	}
	var list []Control
	var walk func(ns []uiNode)
	walk = func(ns []uiNode) {
		for _, n := range ns {
			c := Control{Class: n.Class[strings.LastIndex(n.Class, ".")+1:], Text: n.Text, ID: n.ID, Desc: n.Desc,
				Clickable: n.Clickable == "true", Checked: n.Checked == "true", Focused: n.Focused == "true", Scrollable: n.Scrollable == "true",
				Editable: strings.HasSuffix(n.Class, "EditText")}
			if m := boundsRe.FindStringSubmatch(n.Bounds); m != nil {
				x1, _ := strconv.Atoi(m[1])
				y1, _ := strconv.Atoi(m[2])
				x2, _ := strconv.Atoi(m[3])
				y2, _ := strconv.Atoi(m[4])
				c.X, c.Y = (x1+x2)/2, (y1+y2)/2
			}
			if c.Text != "" || c.Desc != "" || c.Clickable || c.Editable || c.Scrollable || n.Checkable == "true" {
				c.Ref = len(list) + 1
				list = append(list, c)
			}
			walk(n.Nodes)
		}
	}
	walk(root.Nodes)
	return list, nil
}

// ControlsText is controls as lines an agent reads: "[3] Button
// "Sign in" (id login) clickable".
func ControlsText(list []Control) string {
	var b strings.Builder
	for _, c := range list {
		fmt.Fprintf(&b, "[%d] %s", c.Ref, c.Class)
		if c.Text != "" {
			fmt.Fprintf(&b, " %q", c.Text)
		}
		if c.Desc != "" && c.Desc != c.Text {
			fmt.Fprintf(&b, " (%s)", c.Desc)
		}
		if c.ID != "" {
			fmt.Fprintf(&b, " id=%s", c.ID[strings.LastIndex(c.ID, "/")+1:])
		}
		for _, f := range []struct {
			on   bool
			name string
		}{{c.Clickable, "clickable"}, {c.Editable, "editable"}, {c.Checked, "checked"}, {c.Focused, "focused"}, {c.Scrollable, "scrollable"}} {
			if f.on {
				b.WriteString(" " + f.name)
			}
		}
		fmt.Fprintf(&b, " @%d,%d\n", c.X, c.Y)
	}
	if b.Len() == 0 {
		return "Nothing on the screen says anything or can be used.\n"
	}
	return b.String()
}

// Tap touches device serial's screen at x, y.
func (s SDK) Tap(serial string, x, y int) error {
	_, err := s.shell(serial, fmt.Sprintf("input tap %d %d", x, y))
	return err
}

// Swipe drags from x1, y1 to x2, y2 over d; one place held long presses.
func (s SDK) Swipe(serial string, x1, y1, x2, y2 int, d time.Duration) error {
	_, err := s.shell(serial, fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, d.Milliseconds()))
	return err
}

// Keys are the names of keys an agent presses, by Android's key codes.
var Keys = map[string]int{
	"home": KeyHome, "back": KeyBack, "recents": KeyAppSwitch, "power": KeyPower,
	"volume-up": KeyVolumeUp, "volume-down": KeyVolumeDown, "enter": KeyEnter, "delete": KeyDelete,
	"tab": 61, "escape": 111, "up": 19, "down": 20, "left": 21, "right": 22, "menu": 82, "search": 84,
}

// KeyCode reads a key's name or number.
func KeyCode(name string) (int, error) {
	if c, ok := Keys[strings.ToLower(name)]; ok {
		return c, nil
	}
	if n, err := strconv.Atoi(name); err == nil && n > 0 {
		return n, nil
	}
	var names []string
	for k := range Keys {
		names = append(names, k)
	}
	return 0, fmt.Errorf("android: no key %q: a key code, or one of %s", name, strings.Join(names, ", "))
}

// Press presses a key on device serial.
func (s SDK) Press(serial string, code int) error {
	_, err := s.shell(serial, "input keyevent "+strconv.Itoa(code))
	return err
}

// TypeText types text into the field focused on device serial. adb's
// input types ASCII only; spaces and the shell's characters are escaped.
func (s SDK) TypeText(serial, text string) error {
	for _, r := range text {
		if r > 126 || (r < 32 && r != '\n') {
			return fmt.Errorf("android: adb types ASCII only: %q", r)
		}
	}
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			if err := s.Press(serial, KeyEnter); err != nil {
				return err
			}
		}
		if line == "" {
			continue
		}
		var b strings.Builder
		for _, r := range line {
			switch {
			case r == ' ':
				b.WriteString("%s")
			case strings.ContainsRune(`\'"`+"`"+`$&|;<>()*?!#~[]{}`, r):
				b.WriteString(`\` + string(r)) // for the device's shell
			default:
				b.WriteRune(r)
			}
		}
		if _, err := s.shell(serial, "input text "+b.String()); err != nil {
			return err
		}
	}
	return nil
}
