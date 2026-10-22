package proto

import (
	"bytes"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	in := []Frame{
		{Type: TypeRequest, Payload: []byte(`{"id":1}`)},
		{Type: TypeOutput, Channel: 7, Payload: []byte("\x1b[31mhi")},
		{Type: TypeInput, Channel: 7},
	}
	for _, f := range in {
		if err := WriteFrame(&buf, f); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range in {
		got, err := ReadFrame(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if got.Type != want.Type || got.Channel != want.Channel || !bytes.Equal(got.Payload, want.Payload) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}

func TestReadFrameRejectsOversizedPayload(t *testing.T) {
	hdr := []byte{0xff, 0xff, 0xff, 0xff, byte(TypeOutput), 0, 0, 0, 1}
	if _, err := ReadFrame(bytes.NewReader(hdr)); err == nil {
		t.Fatal("want error for oversized frame")
	}
}
