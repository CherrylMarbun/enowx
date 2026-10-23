// Package automation says when automations run: a prompt for an agent on
// a schedule ("weekdays 09:00", "hourly"), which the daemon starts in a
// session of its own. See docs/automations.md.
package automation

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule is when an automation runs, in the host's local time.
type Schedule struct {
	// Every is "hourly", "daily", "weekdays" or "weekly".
	Every string
	// Hour and Minute are the time of day (for hourly, Minute alone).
	Hour, Minute int
	// Weekday is the day of a weekly one.
	Weekday time.Weekday
}

// Parse reads a schedule as people write it: "hourly", "hourly :15",
// "daily 09:00", "weekdays 18:30", "weekly mon 08:00".
func Parse(s string) (Schedule, error) {
	f := strings.Fields(strings.ToLower(strings.TrimSpace(s)))
	if len(f) == 0 {
		return Schedule{}, fmt.Errorf("automation: no schedule")
	}
	sc := Schedule{Every: f[0]}
	rest := f[1:]
	switch sc.Every {
	case "hourly":
		if len(rest) == 0 {
			return sc, nil
		}
		m, err := strconv.Atoi(strings.TrimPrefix(rest[0], ":"))
		if err != nil || m < 0 || m > 59 || len(rest) > 1 {
			return Schedule{}, fmt.Errorf("automation: %q: hourly takes a minute, as :15", s)
		}
		sc.Minute = m
		return sc, nil
	case "weekly":
		if len(rest) != 2 {
			return Schedule{}, fmt.Errorf("automation: %q: weekly takes a day and a time, as mon 09:00", s)
		}
		day, ok := weekdays[rest[0][:min(3, len(rest[0]))]]
		if !ok {
			return Schedule{}, fmt.Errorf("automation: %q: unknown day %q", s, rest[0])
		}
		sc.Weekday = day
		rest = rest[1:]
	case "daily", "weekdays":
	default:
		return Schedule{}, fmt.Errorf("automation: %q: say hourly, daily, weekdays or weekly", s)
	}
	if len(rest) != 1 {
		return Schedule{}, fmt.Errorf("automation: %q: give a time, as 09:00", s)
	}
	h, m, ok := clock(rest[0])
	if !ok {
		return Schedule{}, fmt.Errorf("automation: %q: %q isn't a time of day", s, rest[0])
	}
	sc.Hour, sc.Minute = h, m
	return sc, nil
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

func clock(s string) (int, int, bool) {
	hs, ms, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(hs)
	m, err2 := strconv.Atoi(ms)
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// String writes the schedule as Parse reads it.
func (sc Schedule) String() string {
	at := fmt.Sprintf("%02d:%02d", sc.Hour, sc.Minute)
	switch sc.Every {
	case "hourly":
		return fmt.Sprintf("hourly :%02d", sc.Minute)
	case "weekly":
		return "weekly " + strings.ToLower(sc.Weekday.String()[:3]) + " " + at
	}
	return sc.Every + " " + at
}

// Next is the first time after t the schedule says to run, in t's zone.
func (sc Schedule) Next(t time.Time) time.Time {
	if sc.Every == "hourly" {
		n := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), sc.Minute, 0, 0, t.Location())
		if !n.After(t) {
			n = n.Add(time.Hour)
		}
		return n
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), sc.Hour, sc.Minute, 0, 0, t.Location())
	for i := 0; i < 8; i++ {
		n := day.AddDate(0, 0, i)
		if !n.After(t) {
			continue
		}
		switch sc.Every {
		case "weekdays":
			if n.Weekday() == time.Saturday || n.Weekday() == time.Sunday {
				continue
			}
		case "weekly":
			if n.Weekday() != sc.Weekday {
				continue
			}
		}
		return n
	}
	return day.AddDate(0, 0, 7) // not reached
}
