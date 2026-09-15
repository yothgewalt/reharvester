package httpapi

import (
	"reflect"
	"testing"
)

func TestParseKeywords(t *testing.T) {
	five := []string{"dark matter", "wimp", "axion", "direct detection", "galactic halo"}
	for _, tc := range []struct {
		name string
		out  string
		want []string
	}{
		{"plain line", "dark matter, wimp, axion, direct detection, galactic halo", five},
		{"preamble and quotes", `Keywords: "Dark Matter", WIMP, axion, direct detection, galactic halo.`, five},
		{"numbered lines", "1. dark matter\n2. wimp\n3. axion\n4. direct detection\n5. galactic halo", five},
		{"extra keywords trimmed", "dark matter, wimp, axion, direct detection, galactic halo, neutrino", five},
		{"duplicates skipped", "dark matter, Dark Matter, wimp, axion, direct detection, galactic halo", five},
		{"leading digit kept", "- 5g networks, beamforming, mmwave, massive mimo, channel estimation",
			[]string{"5g networks", "beamforming", "mmwave", "massive mimo", "channel estimation"}},
		{"too few is empty", "dark matter, wimp", []string{}},
		{"sentence is not a keyword", "here is a long sentence that is clearly not a keyword at all, wimp, axion, halo, lensing", []string{}},
		{"empty", "", []string{}},
	} {
		if got := parseKeywords(tc.out); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: parseKeywords(%q) = %q, want %q", tc.name, tc.out, got, tc.want)
		}
	}
}
