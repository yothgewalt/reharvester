package harvest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

// fakeLink serves papers named "<name>-<year>-<i>" and fails once calls
// reaches failOn (0 never fails), after handing back failAfter papers.
type fakeLink struct {
	name      string
	calls     int
	failOn    int
	failAfter int
	thin      int // when set, every span returns at most this many
	err       error
	titleAs   string // when set, titles use this name, to collide with another link
}

func (f *fakeLink) span(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress) ([]paper.Paper, error) {
	f.calls++
	n := max
	if f.thin > 0 {
		n = min(n, f.thin)
	}
	failing := f.failOn > 0 && f.calls >= f.failOn
	if failing {
		n = min(n, f.failAfter)
	}
	candidates := n
	if !failing && f.thin == 0 {
		candidates += 10 // spare records, as a real page has, for duplicates to be skipped over
	}
	titled := f.name
	if f.titleAs != "" {
		titled = f.titleAs
	}
	page := make([]paper.Paper, candidates)
	for i := range page {
		page[i] = paper.Paper{ID: fmt.Sprintf("%s-%d-%d", f.name, q.From, i), Title: fmt.Sprintf("%s %d %d", titled, q.From, i)}
	}
	out, _ := keep(nil, seen, n, page)
	if failing {
		if f.err != nil {
			return out, f.err
		}
		return out, errors.New(f.name + " is throttling")
	}
	return out, nil
}

func routerOf(links ...*fakeLink) *Router {
	r := &Router{}
	for _, l := range links {
		r.links = append(r.links, routeLink{name: l.name, span: l.span})
	}
	return r
}

func TestRouterMovesTheRestOfASpanToTheNextSourceAndSkipsTheFailedOne(t *testing.T) {
	a := &fakeLink{name: "a", failOn: 1, failAfter: 2}
	b := &fakeLink{name: "b"}
	r := routerOf(a, b)

	got, err := r.Harvest(context.Background(), Query{Keywords: []string{"k"}, From: 2025, To: 2026, Max: 6}, nil)

	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	// 2026's quota of 3: a hands back 2 then fails, b fills the last one.
	// 2025's quota of 3 goes straight to b.
	want := []string{"a-2026-0", "a-2026-1", "b-2026-0", "b-2025-0", "b-2025-1", "b-2025-2"}
	if !slices.Equal(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
	if a.calls != 1 {
		t.Errorf("failed source was called %d times, want it skipped after the first failure", a.calls)
	}
}

func TestRouterKeepsAShortAnswerFromAHealthySource(t *testing.T) {
	a := &fakeLink{name: "a", thin: 1}
	b := &fakeLink{name: "b"}

	got, err := routerOf(a, b).Harvest(context.Background(), Query{Keywords: []string{"k"}, Max: 5}, nil)

	if err != nil || len(got) != 1 || b.calls != 0 {
		t.Errorf("got %v, err %v, b called %d times; a thin answer must not fail over", ids(got), err, b.calls)
	}
}

func TestRouterReportsEveryFailureWhenAllSourcesAreDown(t *testing.T) {
	a := &fakeLink{name: "a", failOn: 1, failAfter: 1}
	b := &fakeLink{name: "b", failOn: 1}

	got, err := routerOf(a, b).Harvest(context.Background(), Query{Keywords: []string{"k"}, Max: 5}, nil)

	if err == nil || !strings.Contains(err.Error(), "a: a is throttling") || !strings.Contains(err.Error(), "b: b is throttling") {
		t.Fatalf("err = %v, want both failures named", err)
	}
	if want := []string{"a-0-0"}; !slices.Equal(ids(got), want) {
		t.Errorf("partial = %v, want %v kept", ids(got), want)
	}
}

func TestRouterDoesNotSwitchOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &fakeLink{name: "a", failOn: 1, err: context.Canceled}
	b := &fakeLink{name: "b"}

	_, err := routerOf(a, b).Harvest(ctx, Query{Keywords: []string{"k"}, Max: 5}, nil)

	if !errors.Is(err, context.Canceled) || b.calls != 0 {
		t.Errorf("err = %v, b called %d times; cancellation must stop, not fail over", err, b.calls)
	}
}

func TestTolerateKeepsStandaloneSourcesDegrading(t *testing.T) {
	a := &fakeLink{name: "a", failOn: 1, failAfter: 2}

	got, err := splitHarvest(context.Background(), Query{Keywords: []string{"k"}, From: 2025, To: 2026, Max: 6}, nil, tolerate(a.span))

	// A standalone source that fails after collecting something shortens the
	// corpus and moves on to the next year instead of stopping.
	if err != nil || len(got) != 4 || a.calls != 2 {
		t.Errorf("got %v, err %v, calls %d", ids(got), err, a.calls)
	}
}

func TestRouterCategoryProbeFallsBackWhenArxivIsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	blocked := &Client{HTTP: srv.Client(), BaseURL: srv.URL, MaxRetries: 1}
	r := &Router{arxiv: blocked, links: []routeLink{{name: SourceArxiv, span: blocked.harvestSpan}, {name: "b", span: (&fakeLink{name: "b"}).span}}}

	cats, _, err := r.InferCategories(context.Background(), []string{"radar"})

	if err != nil || cats != nil {
		t.Fatalf("cats = %v, err = %v; want an unscoped harvest, not a failure", cats, err)
	}
	if r.links[0].down == nil {
		t.Error("a blocked arXiv should be dropped from the chain")
	}
}

func TestRouterCategoryProbeKeepsArxivForDeadKeywords(t *testing.T) {
	dead := newFakeArxiv(t, map[string]int{"zzzz": 0}, nil)
	r := &Router{arxiv: dead, links: []routeLink{{name: SourceArxiv, span: dead.harvestSpan}}}

	_, _, err := r.InferCategories(context.Background(), []string{"zzzz"})

	if err != nil || r.links[0].down != nil {
		t.Errorf("err = %v, down = %v; keywords arXiv lacks must not drop arXiv", err, r.links[0].down)
	}
}

func TestOpenAutoBuildsAFastFailingChain(t *testing.T) {
	src, err := Open(Options{Name: SourceAuto, Delay: 2 * time.Second})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	r, ok := src.(*Router)
	if !ok {
		t.Fatalf("source = %T, want *Router", src)
	}
	var names []string
	for _, l := range r.links {
		names = append(names, l.name)
	}
	if !slices.Equal(names, []string{SourceArxiv, SourceOpenAlex, SourceSemanticScholar}) {
		t.Errorf("chain = %v", names)
	}
	if r.arxiv.MaxRetries != routerRetries {
		t.Errorf("arXiv retries = %d, want %d inside the router", r.arxiv.MaxRetries, routerRetries)
	}
	if _, ok := src.(CategoryInferrer); !ok {
		t.Error("the router must still scope keyword queries through arXiv")
	}
}

func TestRouterDedupesAcrossSourcesWithoutAYearRange(t *testing.T) {
	a := &fakeLink{name: "a", failOn: 1, failAfter: 2}
	b := &fakeLink{name: "b", titleAs: "a"} // b lists the same works under its own ids

	got, err := routerOf(a, b).Harvest(context.Background(), Query{Keywords: []string{"k"}, Max: 4}, nil)

	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	if want := []string{"a-0-0", "a-0-1", "b-0-2", "b-0-3"}; !slices.Equal(ids(got), want) {
		t.Errorf("ids = %v, want %v — b must skip the titles a already kept", ids(got), want)
	}
}
