package gravity

import (
	"testing"
	"time"
)

func TestParseScheduleValid(t *testing.T) {
	tests := []struct {
		expr string
	}{
		{"0 3 * * 0"},       // Sunday 03:00
		{"*/15 * * * *"},    // Every 15 minutes
		{"0 0 1 * *"},       // First of month midnight
		{"30 4 * * 1-5"},    // Weekdays 04:30
		{"0 0 * * 0,6"},     // Weekends midnight
		{"0 3 1,15 * *"},    // 1st and 15th at 03:00
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := ParseSchedule(tt.expr)
			if err != nil {
				t.Errorf("ParseSchedule(%q) error: %v", tt.expr, err)
			}
		})
	}
}

func TestParseScheduleInvalid(t *testing.T) {
	tests := []struct {
		expr string
	}{
		{""},
		{"* * *"},
		{"60 * * * *"},
		{"* 25 * * *"},
		{"* * 32 * *"},
		{"* * * 13 *"},
		{"* * * * 7"},
		{"not a cron"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := ParseSchedule(tt.expr)
			if err == nil {
				t.Errorf("ParseSchedule(%q) should have failed", tt.expr)
			}
		})
	}
}

func TestScheduleNext(t *testing.T) {
	sched, err := ParseSchedule("0 3 * * 0")
	if err != nil {
		t.Fatal(err)
	}

	// From a Wednesday
	from := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC) // Wednesday
	next := sched.Next(from)

	if next.Weekday() != time.Sunday {
		t.Errorf("next weekday = %s, want Sunday", next.Weekday())
	}
	if next.Hour() != 3 || next.Minute() != 0 {
		t.Errorf("next time = %02d:%02d, want 03:00", next.Hour(), next.Minute())
	}
	if !next.After(from) {
		t.Error("next should be after from")
	}
}

func TestScheduleNextEvery15Min(t *testing.T) {
	sched, err := ParseSchedule("*/15 * * * *")
	if err != nil {
		t.Fatal(err)
	}

	from := time.Date(2026, 9, 11, 10, 7, 0, 0, time.UTC)
	next := sched.Next(from)

	if next.Minute() != 15 {
		t.Errorf("next minute = %d, want 15", next.Minute())
	}
}

func TestStaggeredSchedules(t *testing.T) {
	s1, _ := ParseSchedule("0 3 * * 0")
	s2, _ := ParseSchedule("0 4 * * 0")

	from := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	n1 := s1.Next(from)
	n2 := s2.Next(from)

	diff := n2.Sub(n1)
	if diff != time.Hour {
		t.Errorf("stagger diff = %s, want 1h", diff)
	}
}
