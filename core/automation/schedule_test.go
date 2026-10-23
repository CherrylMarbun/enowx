package automation

import (
	"testing"
	"time"
)

func TestSchedulesSayWhenNext(t *testing.T) {
	// Friday 23 October 2026, 10:30.
	now := time.Date(2026, 10, 23, 10, 30, 0, 0, time.UTC)
	for in, want := range map[string]string{
		"hourly":              "2026-10-23 11:00",
		"hourly :45":          "2026-10-23 10:45",
		"daily 09:00":         "2026-10-24 09:00",
		"daily 18:00":         "2026-10-23 18:00",
		"weekdays 09:00":      "2026-10-26 09:00", // past Saturday and Sunday
		"weekly mon 08:30":    "2026-10-26 08:30",
		"weekly fri 11:00":    "2026-10-23 11:00",
		"Weekly Friday 09:00": "2026-10-30 09:00",
	} {
		sc, err := Parse(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got := sc.Next(now).Format("2006-01-02 15:04"); got != want {
			t.Errorf("%q next %s, want %s", in, got, want)
		}
		if back, err := Parse(sc.String()); err != nil || back != sc {
			t.Errorf("%q writes as %q, read back %+v, %v", in, sc.String(), back, err)
		}
	}
	for _, bad := range []string{"", "sometimes", "daily", "daily 25:00", "weekly 09:00", "weekly xyz 09:00", "hourly :75"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}
