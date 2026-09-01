package analyze

import (
	"math"
	"testing"
)

// The smoothing exists so a term absent from the early window is rankable at
// all: the unsmoothed log-ratio is infinite there, and that is exactly the
// interesting case.
func TestSmoothedLiftHandlesAbsentEarlyTerm(t *testing.T) {
	a := &Analytics{
		Years:      []int{2020, 2021},
		DocsByYear: []int32{100, 100},
		NumComms:   2,
		Terms: []TermStats{
			{Term: "new thing", Years: []int32{0, 20}, Comms: []int32{20, 0}, Total: 20},
			// Both terms must be topical enough to clear the specificity filter,
			// which is exercised separately below.
			{Term: "steady", Years: []int32{20, 20}, Comms: []int32{36, 4}, Total: 40},
		},
	}
	got := a.Trends(Window{2020, 2020}, Window{2021, 2021})
	if len(got) != 2 {
		t.Fatalf("want 2 trends, got %d", len(got))
	}
	if got[0].Term != "new thing" {
		t.Errorf("absent-early term should rank first, got %q", got[0].Term)
	}
	if math.IsInf(got[0].Log2Lift, 0) || math.IsNaN(got[0].Log2Lift) {
		t.Errorf("lift is not finite: %v", got[0].Log2Lift)
	}
	// ((20+0.5)/100) / ((0+0.5)/100) = 41
	if want := math.Log2(41); math.Abs(got[0].Log2Lift-want) > 1e-9 {
		t.Errorf("lift = %v, want %v", got[0].Log2Lift, want)
	}
	// A term with identical prevalence in both windows must not move.
	if math.Abs(got[1].Log2Lift) > 1e-9 {
		t.Errorf("steady term drifted: %v", got[1].Log2Lift)
	}
}

// A term spread evenly over every community approaches 1; a term confined to
// one community is 0.
func TestSpecificity(t *testing.T) {
	if got := Specificity([]int32{10, 10, 10, 10}); math.Abs(got-1) > 1e-9 {
		t.Errorf("uniform specificity = %v, want 1", got)
	}
	if got := Specificity([]int32{40, 0, 0, 0}); got != 0 {
		t.Errorf("concentrated specificity = %v, want 0", got)
	}
	skew := Specificity([]int32{30, 5, 3, 2})
	if skew <= 0 || skew >= 1 {
		t.Errorf("skewed specificity = %v, want strictly between 0 and 1", skew)
	}
}

func TestSpecificityFilterDropsUbiquitousTerms(t *testing.T) {
	a := &Analytics{
		Years:      []int{2020, 2021},
		DocsByYear: []int32{100, 100},
		NumComms:   4,
		Terms: []TermStats{
			{Term: "everywhere", Years: []int32{1, 40}, Comms: []int32{10, 10, 10, 11}, Total: 41},
			{Term: "topical", Years: []int32{1, 40}, Comms: []int32{41, 0, 0, 0}, Total: 41},
		},
	}
	got := a.Trends(Window{2020, 2020}, Window{2021, 2021})
	if len(got) != 1 || got[0].Term != "topical" {
		t.Errorf("specificity filter did not drop the ubiquitous term: %+v", got)
	}
}

// A flat series must not burst; a step change must.
func TestBurst(t *testing.T) {
	flat := &Analytics{Years: []int{1, 2, 3, 4}, DocsByYear: []int32{100, 100, 100, 100}}
	ts := TermStats{Term: "flat", Years: []int32{10, 10, 10, 10}, Total: 40}
	if w := flat.burst(&ts); w != 0 {
		t.Errorf("flat series burst = %v, want 0", w)
	}
	step := TermStats{Term: "step", Years: []int32{0, 0, 30, 30}, Total: 60}
	if w := flat.burst(&step); w <= 0 {
		t.Errorf("step change burst = %v, want > 0", w)
	}
}
