package gravity

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Schedule struct {
	minutes  []bool // 0-59
	hours    []bool // 0-23
	doms     []bool // 1-31
	months   []bool // 1-12
	dows     []bool // 0-6 (Sunday=0)
}

func ParseSchedule(expr string) (*Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron: expected 5 fields, got %d", len(fields))
	}

	s := &Schedule{}
	var err error

	s.minutes, err = parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("cron minute: %w", err)
	}
	s.hours, err = parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("cron hour: %w", err)
	}
	s.doms, err = parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("cron day-of-month: %w", err)
	}
	s.months, err = parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("cron month: %w", err)
	}
	s.dows, err = parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("cron day-of-week: %w", err)
	}

	return s, nil
}

func (s *Schedule) Next(from time.Time) time.Time {
	t := from.Truncate(time.Minute).Add(time.Minute)

	for i := 0; i < 366*24*60; i++ {
		if s.months[int(t.Month())] &&
			s.doms[t.Day()] &&
			s.dows[int(t.Weekday())] &&
			s.hours[t.Hour()] &&
			s.minutes[t.Minute()] {
			return t
		}
		t = t.Add(time.Minute)
	}

	return time.Time{}
}

func parseField(field string, min, max int) ([]bool, error) {
	set := make([]bool, max+1)

	for _, part := range strings.Split(field, ",") {
		if err := parsePart(part, min, max, set); err != nil {
			return nil, err
		}
	}
	return set, nil
}

func parsePart(part string, min, max int, set []bool) error {
	stepParts := strings.SplitN(part, "/", 2)
	rangePart := stepParts[0]
	step := 1

	if len(stepParts) == 2 {
		s, err := strconv.Atoi(stepParts[1])
		if err != nil || s <= 0 {
			return fmt.Errorf("invalid step %q", stepParts[1])
		}
		step = s
	}

	if rangePart == "*" {
		for i := min; i <= max; i += step {
			set[i] = true
		}
		return nil
	}

	if dashIdx := strings.IndexByte(rangePart, '-'); dashIdx >= 0 {
		lo, err := strconv.Atoi(rangePart[:dashIdx])
		if err != nil {
			return fmt.Errorf("invalid range start %q", rangePart[:dashIdx])
		}
		hi, err := strconv.Atoi(rangePart[dashIdx+1:])
		if err != nil {
			return fmt.Errorf("invalid range end %q", rangePart[dashIdx+1:])
		}
		if lo < min || hi > max || lo > hi {
			return fmt.Errorf("range %d-%d out of bounds [%d,%d]", lo, hi, min, max)
		}
		for i := lo; i <= hi; i += step {
			set[i] = true
		}
		return nil
	}

	val, err := strconv.Atoi(rangePart)
	if err != nil {
		return fmt.Errorf("invalid value %q", rangePart)
	}
	if val < min || val > max {
		return fmt.Errorf("value %d out of bounds [%d,%d]", val, min, max)
	}
	set[val] = true
	return nil
}
