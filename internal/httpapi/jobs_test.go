package httpapi

import (
	"testing"
	"time"
)

// The scheduler UI builds these three shapes; every one must fire when due and
// stay quiet otherwise.
func TestCronMatches(t *testing.T) {
	// 2026-08-31 is a Monday.
	mon0600 := time.Date(2026, 8, 31, 6, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		expr string
		at   time.Time
		want bool
	}{
		{"daily fires", "0 6 * * *", mon0600, true},
		{"daily wrong hour", "0 6 * * *", mon0600.Add(time.Hour), false},
		{"daily wrong minute", "0 6 * * *", mon0600.Add(time.Minute), false},
		{"weekly monday fires", "0 6 * * 1", mon0600, true},
		{"weekly sunday quiet", "0 6 * * 0", mon0600, false},
		{"monthly 31st fires", "0 6 31 * *", mon0600, true},
		{"monthly 1st quiet", "0 6 1 * *", mon0600, true == false},
		{"list of hours", "0 6,18 * * *", mon0600, true},
		{"malformed is never due", "0 6 * *", mon0600, false},
		{"empty is never due", "", mon0600, false},
	} {
		if got := cronMatches(tc.expr, tc.at); got != tc.want {
			t.Errorf("%s: cronMatches(%q, %v) = %v, want %v", tc.name, tc.expr, tc.at, got, tc.want)
		}
	}
}
