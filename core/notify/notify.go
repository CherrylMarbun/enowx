// Package notify keeps how the app notifies the user of agents: its
// settings, in the synced settings folder of a daemon, the sounds it has
// built in, and when to stay quiet.
package notify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// Kinds are the kinds of sound, the most urgent last.
var Kinds = []string{proto.NotifyChat, proto.NotifyDone, proto.NotifyInput, proto.NotifyError}

// Defaults are the settings before the user changed them.
func Defaults() proto.NotifySettings {
	return proto.NotifySettings{Volume: 0.7, Sounds: map[string]string{
		proto.NotifyChat: "pop", proto.NotifyDone: "chime", proto.NotifyInput: "ping", proto.NotifyError: "alert",
	}, QuietFrom: "22:00", QuietTo: "07:00"}
}

// MaxSound is the largest sound of the user's kept.
const MaxSound = 2 << 20

// Store is the settings in a daemon's folder: settings/notify.json, and
// the user's sounds in settings/sounds/. Safe for concurrent use.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Open is the store of daemon folder dir.
func Open(dir string) *Store { return &Store{dir: dir} }

func (s *Store) file() string { return filepath.Join(s.dir, "settings", "notify.json") }

// Get is the settings, the defaults filling what isn't set.
func (s *Store) Get() (proto.NotifySettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Defaults()
	data, err := os.ReadFile(s.file())
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("notify: %w", err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return Defaults(), fmt.Errorf("notify: settings: %w", err)
	}
	return Normal(out), nil
}

// Set keeps st, its gaps filled first.
func (s *Store) Set(st proto.NotifySettings) (proto.NotifySettings, error) {
	st = Normal(st)
	if err := Check(st); err != nil {
		return proto.NotifySettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return st, err
	}
	return st, writeFile(s.file(), data)
}

// Normal is st with its gaps filled and its lists in order.
func Normal(st proto.NotifySettings) proto.NotifySettings {
	d := Defaults()
	st.Volume = min(max(st.Volume, 0), 1)
	sounds := map[string]string{}
	for k, v := range st.Sounds {
		sounds[k] = v
	}
	for _, k := range Kinds {
		if sounds[k] == "" {
			sounds[k] = d.Sounds[k]
		}
	}
	st.Sounds = sounds
	if st.QuietFrom == "" {
		st.QuietFrom = d.QuietFrom
	}
	if st.QuietTo == "" {
		st.QuietTo = d.QuietTo
	}
	st.MutedAgents = slices.Compact(slices.Sorted(slices.Values(st.MutedAgents)))
	st.MutedSessions = slices.Compact(slices.Sorted(slices.Values(st.MutedSessions)))
	return st
}

var clockPattern = regexp.MustCompile(`^([01]?\d|2[0-3]):[0-5]\d$`)

// Check says what is wrong with st, nil when nothing is.
func Check(st proto.NotifySettings) error {
	for _, t := range []string{st.QuietFrom, st.QuietTo} {
		if t != "" && !clockPattern.MatchString(t) {
			return fmt.Errorf("notify: %q is not a time of day like 22:00", t)
		}
	}
	for k, name := range st.Sounds {
		if !slices.Contains(Kinds, k) {
			return fmt.Errorf("notify: no kind of sound %q: %s", k, strings.Join(Kinds, ", "))
		}
		if !Known(name) {
			return fmt.Errorf("notify: no sound %q: %s, none, or custom:FILE", name, strings.Join(Builtins(), ", "))
		}
	}
	return nil
}

// Known reports whether name is a sound: a built-in, none, or the user's.
func Known(name string) bool {
	if name == "none" || slices.Contains(Builtins(), name) {
		return true
	}
	file, ok := strings.CutPrefix(name, "custom:")
	return ok && validName(file)
}

// Quiet reports whether st keeps sounds quiet at t: off, or within its
// quiet hours.
func Quiet(st proto.NotifySettings, t time.Time) bool {
	if st.Off {
		return true
	}
	if !st.Quiet {
		return false
	}
	from, ok1 := minutes(st.QuietFrom)
	to, ok2 := minutes(st.QuietTo)
	if !ok1 || !ok2 || from == to {
		return false
	}
	now := t.Hour()*60 + t.Minute()
	if from < to {
		return now >= from && now < to
	}
	return now >= from || now < to // over midnight
}

func minutes(s string) (int, bool) {
	if !clockPattern.MatchString(s) {
		return 0, false
	}
	var h, m int
	fmt.Sscanf(s, "%d:%d", &h, &m)
	return h*60 + m, true
}

// Muted reports whether st silences session id of agent.
func Muted(st proto.NotifySettings, agent, id string) bool {
	return slices.Contains(st.MutedAgents, agent) || slices.Contains(st.MutedSessions, id)
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._ -]{0,80}\.(wav|mp3)$`)

func validName(name string) bool {
	return namePattern.MatchString(name) && !strings.Contains(name, "..")
}

// SaveSound keeps a sound of the user's, a WAV or an MP3, as name.
func (s *Store) SaveSound(name string, data []byte) error {
	if !validName(name) {
		return fmt.Errorf("notify: %q: a sound's name ends in .wav or .mp3, with letters, digits, dots and dashes", name)
	}
	if len(data) > MaxSound {
		return fmt.Errorf("notify: %s is %d KB: a sound is %d KB at most", name, len(data)>>10, MaxSound>>10)
	}
	if !isWAV(data) && !isMP3(data) {
		return fmt.Errorf("notify: %s is neither a WAV nor an MP3", name)
	}
	return writeFile(filepath.Join(s.dir, "settings", "sounds", name), data)
}

// Sound reads sound name: a built-in, or the user's ("custom:FILE").
func (s *Store) Sound(name string) ([]byte, error) {
	if file, ok := strings.CutPrefix(name, "custom:"); ok {
		if !validName(file) {
			return nil, fmt.Errorf("notify: no sound %q", name)
		}
		data, err := os.ReadFile(filepath.Join(s.dir, "settings", "sounds", file))
		if err != nil {
			return nil, fmt.Errorf("notify: %w", err)
		}
		return data, nil
	}
	if data := Builtin(name); data != nil {
		return data, nil
	}
	return nil, fmt.Errorf("notify: no sound %q", name)
}

func isWAV(b []byte) bool { return len(b) > 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WAVE" }

func isMP3(b []byte) bool {
	return len(b) > 3 && (string(b[:3]) == "ID3" || b[0] == 0xff && b[1]&0xe0 == 0xe0)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	return os.Rename(tmp, path)
}
