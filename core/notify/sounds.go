package notify

import (
	"bytes"
	"encoding/binary"
	"math"
	"slices"
	"sync"
)

// The sounds built in are synthesized here, a few notes each, short and
// soft: nothing is fetched, nothing licensed.

type note struct {
	freq, start, dur float64 // Hz, seconds
	gain             float64
}

var builtins = map[string][]note{
	"chime": {{880, 0, 0.35, 0.5}, {1318.5, 0.12, 0.5, 0.45}},                         // a turn done
	"ping":  {{1568, 0, 0.18, 0.45}, {1568, 0.22, 0.25, 0.45}},                        // input asked
	"alert": {{659.3, 0, 0.22, 0.55}, {523.3, 0.2, 0.22, 0.55}, {392, 0.4, 0.4, 0.5}}, // something failed
	"pop":   {{620, 0, 0.09, 0.6}},
	"bell":  {{1046.5, 0, 0.9, 0.4}, {2093, 0, 0.6, 0.12}},
	"soft":  {{523.3, 0, 0.45, 0.35}, {659.3, 0.15, 0.5, 0.3}},
}

// Builtins are the built-in sounds' names.
func Builtins() []string {
	var out []string
	for name := range builtins {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

const sampleRate = 44100

var rendered sync.Map // name → []byte

// Builtin is built-in sound name as a WAV, nil for none.
func Builtin(name string) []byte {
	if b, ok := rendered.Load(name); ok {
		return b.([]byte)
	}
	notes, ok := builtins[name]
	if !ok {
		return nil
	}
	end := 0.0
	for _, n := range notes {
		end = max(end, n.start+n.dur)
	}
	samples := make([]float64, int((end+0.05)*sampleRate))
	for _, n := range notes {
		first := int(n.start * sampleRate)
		count := int(n.dur * sampleRate)
		for i := 0; i < count && first+i < len(samples); i++ {
			t := float64(i) / sampleRate
			// A quick rise and a long fall, as a struck bell.
			env := math.Min(1, t/0.005) * math.Exp(-5*t/n.dur)
			v := math.Sin(2*math.Pi*n.freq*t) + 0.25*math.Sin(4*math.Pi*n.freq*t)
			samples[first+i] += n.gain * env * v / 1.25
		}
	}
	b := wav(samples)
	rendered.Store(name, b)
	return b
}

// wav is mono 16-bit PCM samples in [-1, 1] as a WAV file.
func wav(samples []float64) []byte {
	var buf bytes.Buffer
	data := len(samples) * 2
	w := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	buf.WriteString("RIFF")
	w(uint32(36 + data))
	buf.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1)) // PCM
	w(uint16(1)) // mono
	w(uint32(sampleRate))
	w(uint32(sampleRate * 2))
	w(uint16(2))
	w(uint16(16))
	buf.WriteString("data")
	w(uint32(data))
	for _, s := range samples {
		w(int16(math.Max(-1, math.Min(1, s)) * 32767))
	}
	return buf.Bytes()
}
