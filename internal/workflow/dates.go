package workflow

import (
	"fmt"
	"time"
	_ "time/tzdata" // Portable Windows ZIP does not depend on a system tz database.
)

// CanvasDate preserves omitted dates. Local wall times require the teacher's
// explicit timezone. Ambiguous/nonexistent DST wall times require an offset.
func CanvasDate(value *string, zone string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	if *value == "" {
		return value, nil
	} // Explicitly clearing a pulled date.
	if t, err := time.Parse(time.RFC3339, *value); err == nil {
		s := t.UTC().Format(time.RFC3339)
		return &s, nil
	}
	if zone == "" {
		return nil, fmt.Errorf("local date %q requires a configured timezone or RFC3339 offset", *value)
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("unknown timezone %q", zone)
	}
	layout := "2006-01-02T15:04"
	if len(*value) == 19 {
		layout = "2006-01-02T15:04:05"
	}
	t, err := time.ParseInLocation(layout, *value, loc)
	if err != nil {
		return nil, fmt.Errorf("date %q must be YYYY-MM-DDTHH:MM or RFC3339", *value)
	}
	if t.Format(layout) != *value {
		return nil, fmt.Errorf("local date %q does not exist in %s; supply a valid time with offset", *value, zone)
	}
	for _, shift := range []time.Duration{-2 * time.Hour, -time.Hour, -30 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour} {
		if t.Add(shift).In(loc).Format(layout) == *value {
			return nil, fmt.Errorf("local date %q is ambiguous in %s; include its UTC offset", *value, zone)
		}
	}
	s := t.UTC().Format(time.RFC3339)
	return &s, nil
}
