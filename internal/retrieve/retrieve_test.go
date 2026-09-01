package retrieve

import (
	"testing"

	"github.com/yothgewalt/reharvester/internal/index"
)

// star wires doc 0 to many others, so a naive expansion that sums neighbour
// bonuses will let a hub outrank a direct hit.
type star struct {
	hub    int
	spokes []int
}

func (s star) Neighbors(doc int) []int {
	if doc == s.hub {
		return s.spokes
	}
	for _, sp := range s.spokes {
		if sp == doc {
			return []int{s.hub}
		}
	}
	return nil
}

// The document the retriever ranked first must survive expansion. Summing
// bonuses, or adding a bonus to a seed's own score, both break this.
func TestExpansionNeverDisplacesTopHit(t *testing.T) {
	seeds := make([]index.Hit, 20)
	for i := range seeds {
		seeds[i] = index.Hit{Doc: i, Score: 1 / (rrfK + float64(i+1))}
	}
	// Doc 19 is the weakest seed and neighbours the strongest one; docs 1..18
	// all neighbour doc 0 too, so a summing implementation would float them up.
	e := &Engine{Graph: star{hub: 0, spokes: []int{1, 2, 3, 4, 5, 15, 16, 17, 18, 19}}}
	got := e.expand(seeds, 10)
	if len(got) == 0 || got[0].Doc != 0 {
		t.Fatalf("top hit displaced by its own neighbourhood: got %v", got[:min(3, len(got))])
	}
}

// A neighbour of the best seed should still be able to enter the top ten,
// otherwise expansion cannot help recall at all.
func TestExpansionPromotesNeighbourOfBestSeed(t *testing.T) {
	seeds := make([]index.Hit, 20)
	for i := range seeds {
		seeds[i] = index.Hit{Doc: i, Score: 1 / (rrfK + float64(i+1))}
	}
	// Doc 99 was never retrieved but neighbours the top seed.
	e := &Engine{Graph: star{hub: 0, spokes: []int{99}}}
	got := e.expand(seeds, 10)
	found := false
	for _, h := range got {
		if h.Doc == 99 {
			found = true
		}
	}
	if !found {
		t.Errorf("neighbour of the best seed did not reach the top ten: %v", got)
	}
	if got[0].Doc != 0 {
		t.Errorf("top hit changed: %v", got[0])
	}
}

func TestTierLadder(t *testing.T) {
	if got := (&Engine{}).Tier(); got != T0 {
		t.Errorf("bare engine tier = %s, want T0", got)
	}
	if got := (&Engine{TF: &index.TFIDF{}}).Tier(); got != T1 {
		t.Errorf("with TF-IDF tier = %s, want T1", got)
	}
	if got := (&Engine{TF: &index.TFIDF{}, Graph: star{}}).Tier(); got != T2 {
		t.Errorf("with graph tier = %s, want T2", got)
	}
	// A dense index with no vectors is not a working T3 rung.
	if got := (&Engine{TF: &index.TFIDF{}, Graph: star{}, Dense: &index.Dense{}}).Tier(); got != T2 {
		t.Errorf("empty dense index should not claim T3, got %s", got)
	}
}

func TestDefaultMethodPerTier(t *testing.T) {
	for tier, want := range map[Tier]Method{T0: Overlap, T1: TFIDF, T2: TFIDF, T3: Dense} {
		if got := DefaultMethod(tier); got != want {
			t.Errorf("DefaultMethod(%s) = %s, want %s", tier, got, want)
		}
	}
}
