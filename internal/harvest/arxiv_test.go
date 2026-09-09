package harvest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// abstract is long enough to clear MinAbstractChars, which toPaper enforces.
var abstract = strings.Repeat("an abstract about the subject at hand. ", 10)

var (
	keywordRe = regexp.MustCompile(`all:"([^"]+)"`)
	yearRe    = regexp.MustCompile(`submittedDate:\[(\d{4})`)
)

// queriedKeyword reports the single keyword a sub-query asked for. Harvest is
// supposed to send exactly one per request; more than one means the keywords
// were OR-ed back into a single query, which is the bug this package guards
// against. Reported with Errorf because it runs on the test server goroutine,
// where Fatalf would not stop the test.
func queriedKeyword(t *testing.T, searchQuery string) (string, bool) {
	t.Helper()
	m := keywordRe.FindAllStringSubmatch(searchQuery, -1)
	if len(m) != 1 {
		t.Errorf("expected exactly one keyword per request, got %d in %q", len(m), searchQuery)
		return "", false
	}
	return m[0][1], true
}

// feedOf renders n entries in the primary category given. Ids are unique per
// keyword and per year window, as arXiv's are: a double that repeats ids across
// windows makes every page after the first look like a duplicate.
func feedOf(keyword, category, window string, offset, n, total int) string {
	var b strings.Builder
	b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom"` +
		` xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"` +
		` xmlns:arxiv="http://arxiv.org/schemas/atom">`)
	fmt.Fprintf(&b, `<opensearch:totalResults>%d</opensearch:totalResults>`, total)
	slug := strings.ReplaceAll(keyword, " ", "_")
	for i := range n {
		id := offset + i
		fmt.Fprintf(&b, `<entry>
			<id>http://arxiv.org/abs/%s-%s-%d</id>
			<title>%s paper %d</title>
			<summary>%s</summary>
			<published>%s-01-01T00:00:00Z</published>
			<author><name>Author One</name></author>
			<arxiv:primary_category term="%s"/>
			<category term="%s"/>
		</entry>`, slug, window, id, keyword, id, abstract, window, category, category)
	}
	b.WriteString(`</feed>`)
	return b.String()
}

// newFakeArxiv serves min(available[keyword], max_results) entries per request.
func newFakeArxiv(t *testing.T, available map[string]int, category map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kw, ok := queriedKeyword(t, r.URL.Query().Get("search_query"))
		if !ok {
			http.Error(w, "one keyword per request", http.StatusBadRequest)
			return
		}
		size, _ := strconv.Atoi(r.URL.Query().Get("max_results"))
		start, _ := strconv.Atoi(r.URL.Query().Get("start"))
		window := "2026"
		if m := yearRe.FindStringSubmatch(r.URL.Query().Get("search_query")); m != nil {
			window = m[1]
		}
		left := max(available[kw]-start, 0)
		cat := category[kw]
		if cat == "" {
			cat = "cs.LG"
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, feedOf(kw, cat, window, start, min(left, size), available[kw]))
	}))
	t.Cleanup(srv.Close)
	return &Client{HTTP: srv.Client(), BaseURL: srv.URL, PageSize: DefaultPageSize}
}

func TestSearchQueryOmitsCategoryClauseWhenUnscoped(t *testing.T) {
	tests := []struct {
		name string
		q    Query
		want string
	}{
		{
			name: "no categories means no cat clause",
			q:    Query{Keywords: []string{"fighter jet"}},
			want: `(all:"fighter jet")`,
		},
		{
			name: "categories are ANDed onto the keywords",
			q:    Query{Categories: []string{"eess.SY", "cs.RO"}, Keywords: []string{"fighter jet"}},
			want: `(cat:eess.SY OR cat:cs.RO) AND (all:"fighter jet")`,
		},
		{
			name: "years become a submittedDate range",
			q:    Query{Keywords: []string{"radar"}, From: 2025, To: 2026},
			want: `(all:"radar") AND submittedDate:[202501010000 TO 202612312359]`,
		},
		{
			name: "blank categories and keywords are dropped",
			q:    Query{Categories: []string{" "}, Keywords: []string{"", "radar"}},
			want: `(all:"radar")`,
		},
		{
			name: "an empty query matches everything",
			q:    Query{},
			want: "all:*",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.q.SearchQuery(); got != tt.want {
				t.Errorf("SearchQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHarvestGivesEachKeywordAnEqualShare(t *testing.T) {
	kws := []string{"aerodynamic", "fighter jet", "radar", "missiles", "space"}
	// "space" is effectively unlimited on arXiv; under the old OR-ed query it
	// took the entire cap and the other four contributed nothing.
	c := newFakeArxiv(t, map[string]int{
		"aerodynamic": 5000, "fighter jet": 5000, "radar": 5000,
		"missiles": 5000, "space": 500000,
	}, nil)
	c.Delay = 0

	got, err := c.Harvest(context.Background(), Query{
		Keywords: kws, From: 2026, To: 2026, Max: 500,
	}, nil)
	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	if len(got) != 500 {
		t.Fatalf("harvested %d papers, want 500", len(got))
	}
	perKeyword := map[string]int{}
	for _, p := range got {
		perKeyword[strings.SplitN(p.Title, " paper ", 2)[0]]++
	}
	for _, kw := range kws {
		if perKeyword[kw] != 100 {
			t.Errorf("%q contributed %d papers, want 100 (all shares: %v)", kw, perKeyword[kw], perKeyword)
		}
	}
}

func TestHarvestDoesNotRedistributeAThinKeywordsShare(t *testing.T) {
	// "fighter jet" really does match only a handful of records on arXiv. Its
	// unused quota must not roll over to the broad terms, which is exactly how
	// one generic keyword ends up owning the corpus.
	c := newFakeArxiv(t, map[string]int{
		"aerodynamic": 5000, "fighter jet": 6, "radar": 5000, "missiles": 5000, "space": 500000,
	}, nil)
	c.Delay = 0

	got, err := c.Harvest(context.Background(), Query{
		Keywords: []string{"aerodynamic", "fighter jet", "radar", "missiles", "space"},
		From:     2026, To: 2026, Max: 500,
	}, nil)
	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	if len(got) != 406 {
		t.Fatalf("harvested %d papers, want 406 (four full shares of 100 plus six)", len(got))
	}
	perKeyword := map[string]int{}
	for _, p := range got {
		perKeyword[strings.SplitN(p.Title, " paper ", 2)[0]]++
	}
	if perKeyword["fighter jet"] != 6 {
		t.Errorf("thin keyword contributed %d papers, want 6", perKeyword["fighter jet"])
	}
	if perKeyword["space"] != 100 {
		t.Errorf("broad keyword contributed %d papers, want 100 — it absorbed the slack", perKeyword["space"])
	}
}

func TestHarvestReportsEachKeywordsContribution(t *testing.T) {
	c := newFakeArxiv(t, map[string]int{"aerodynamic": 5000, "fighter jet": 0}, nil)
	c.Delay = 0

	var msgs []string
	_, err := c.Harvest(context.Background(), Query{
		Keywords: []string{"aerodynamic", "fighter jet"}, From: 2026, To: 2026, Max: 200,
	}, func(_, _ int, msg string) { msgs = append(msgs, msg) })
	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{"aerodynamic: 100 records", "fighter jet: 0 records"} {
		if !strings.Contains(joined, want) {
			t.Errorf("progress never reported %q; got:\n%s", want, joined)
		}
	}
}

func TestHarvestKeepsSingleKeywordBehaviourUnchanged(t *testing.T) {
	c := newFakeArxiv(t, map[string]int{"radar": 5000}, nil)
	c.Delay = 0

	got, err := c.Harvest(context.Background(), Query{
		Keywords: []string{"radar"}, From: 2024, To: 2026, Max: 300,
	}, nil)
	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	if len(got) != 300 {
		t.Errorf("harvested %d papers, want 300", len(got))
	}
}
