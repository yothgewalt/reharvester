package app

import "testing"

func TestFromModuleVersion(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"", ""},
		{"(devel)", ""},
		{"v0.2.5", "0.2.5"},
		{"v0.2.5+dirty", "0.2.5+dirty"},
		{"v0.2.6-0.20260913143912-de3c95f37b19", "0.2.6-dev+de3c95f"},
		{"v0.2.6-0.20260913143912-de3c95f37b19+dirty", "0.2.6-dev+de3c95f.dirty"},
		{"v1.0.0-rc.1.0.20260913143912-de3c95f37b19", "1.0.0-dev+de3c95f"},
		{"v1.0.0-rc.1", "1.0.0-rc.1"},
	} {
		if got := fromModuleVersion(tc.in); got != tc.want {
			t.Errorf("fromModuleVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
