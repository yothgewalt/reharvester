package harvest

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

func testTransport(srv *httptest.Server) *Client {
	return &Client{HTTP: srv.Client()}
}

func ids(ps []paper.Paper) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.ID
	}
	return out
}

func TestOpenSelectsSource(t *testing.T) {
	tests := []struct {
		opts    Options
		want    string
		wantErr string
	}{
		{opts: Options{}, want: "*harvest.Client"},
		{opts: Options{Name: "ArXiv"}, want: "*harvest.Client"},
		{opts: Options{Name: SourceKaggle, SnapshotPath: "x.json"}, want: "*harvest.Snapshot"},
		{opts: Options{Name: SourceKaggle}, wantErr: "needs the path"},
		{opts: Options{Name: SourceSemanticScholar}, want: "*harvest.SemanticScholar"},
		{opts: Options{Name: SourceOpenAlex}, want: "*harvest.OpenAlex"},
		{opts: Options{Name: SourceOAI}, want: "*harvest.OAI"},
		{opts: Options{Name: "scopus"}, wantErr: "unknown source"},
	}
	for _, tt := range tests {
		t.Run(tt.opts.Name, func(t *testing.T) {
			src, err := Open(tt.opts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to mention %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if got := fmt.Sprintf("%T", src); got != tt.want {
				t.Errorf("source = %s, want %s", got, tt.want)
			}
		})
	}
	s2, _ := Open(Options{Name: SourceSemanticScholar, Delay: 10 * time.Millisecond})
	if d := s2.(*SemanticScholar).c.Delay; d < time.Second {
		t.Errorf("semantic scholar delay = %s, must not go below its 1 request/s limit", d)
	}
}

func TestSemanticScholarPagesWithTokenAndMapsIDs(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" {
			t.Errorf("x-api-key = %q, want the configured key", r.Header.Get("x-api-key"))
		}
		queries = append(queries, r.URL.RawQuery)
		if r.URL.Query().Get("offset") == "0" {
			fmt.Fprintf(w, `{"total":3,"offset":0,"next":100,"data":[
				{"paperId":"a1","externalIds":{"ArXiv":"2401.00001v2","CorpusId":1},"title":"Arxiv one","abstract":%q,"publicationDate":"2025-03-04","fieldsOfStudy":["Physics"],"isOpenAccess":true,"authors":[{"name":"Ada"}]},
				{"paperId":"a2","title":"Too short","abstract":"tiny","year":2025}
			]}`, abstract)
			return
		}
		fmt.Fprintf(w, `{"total":3,"offset":100,"data":[{"paperId":"b1","title":"No arxiv","abstract":%q,"year":2025,"authors":[]}]}`, abstract)
	}))
	t.Cleanup(srv.Close)
	s := &SemanticScholar{c: testTransport(srv), Key: "k", BaseURL: srv.URL}

	got, err := s.Harvest(context.Background(), Query{Keywords: []string{"fighter jet"}, From: 2025, To: 2025, Max: 10}, nil)

	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	if want := []string{"2401.00001", "s2:b1"}; !slices.Equal(ids(got), want) {
		t.Fatalf("ids = %v, want %v", ids(got), want)
	}
	if got[0].Categories[0] != "Physics" || !got[0].OpenAccess || got[1].Year() != 2025 {
		t.Errorf("fields not mapped: %+v", got)
	}
	if len(queries) != 2 || !strings.Contains(queries[0], "year=2025") || !strings.Contains(queries[0], "query=fighter+jet") || !strings.Contains(queries[1], "offset=100") {
		t.Errorf("unexpected requests: %v", queries)
	}
}

func TestOpenAlexFollowsCursorAndRebuildsAbstracts(t *testing.T) {
	words := strings.Fields(abstract)
	index := map[string][]int{}
	for i, w := range words {
		index[w] = append(index[w], i)
	}
	inv, _ := json.Marshal(index)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if q.Get("api_key") != "k" || !strings.Contains(q.Get("filter"), `title_and_abstract.search:"radar"`) ||
			!strings.Contains(q.Get("filter"), "type:article|preprint|review") || q.Get("sort") != "relevance_score:desc" {
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
		if q.Get("cursor") == "*" {
			fmt.Fprintf(w, `{"meta":{"count":2,"next_cursor":"c2"},"results":[{"id":"https://openalex.org/W1","doi":"https://doi.org/10.48550/arXiv.2301.12345","display_name":"On radar","publication_date":"2024-06-01","abstract_inverted_index":%s,"authorships":[{"author":{"display_name":"Ada"}}],"primary_topic":{"subfield":{"display_name":"Aerospace Engineering"},"field":{"display_name":"Engineering"}},"open_access":{"is_oa":true}}]}`, inv)
			return
		}
		fmt.Fprintf(w, `{"meta":{"count":2,"next_cursor":null},"results":[{"id":"https://openalex.org/W2","display_name":"More radar","publication_date":"2024-01-01","abstract_inverted_index":%s}]}`, inv)
	}))
	t.Cleanup(srv.Close)
	o := &OpenAlex{c: testTransport(srv), Key: "k", BaseURL: srv.URL}

	got, err := o.Harvest(context.Background(), Query{Keywords: []string{"radar"}, Max: 10}, nil)

	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	if want := []string{"2301.12345", "openalex:W2"}; !slices.Equal(ids(got), want) {
		t.Fatalf("ids = %v, want %v", ids(got), want)
	}
	if got[0].Abstract != squash(abstract) {
		t.Errorf("abstract not rebuilt in order:\n%q", got[0].Abstract)
	}
	if !slices.Equal(got[0].Categories, []string{"Aerospace Engineering", "Engineering"}) {
		t.Errorf("categories = %v", got[0].Categories)
	}
	if calls != 2 {
		t.Errorf("made %d requests, want 2 (stop when next_cursor is null)", calls)
	}
}

func snapshotLine(id, title, cats, created string) string {
	b, _ := json.Marshal(map[string]any{
		"id": id, "title": title, "abstract": abstract, "categories": cats,
		"authors_parsed": [][]string{{"Lovelace", "Ada", ""}},
		"versions":       []map[string]string{{"version": "v1", "created": created}},
	})
	return string(b)
}

func writeSnapshot(t *testing.T, zipped bool, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	body := strings.Join(lines, "\n") + "\n"
	if !zipped {
		p := filepath.Join(dir, "snap.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := filepath.Join(dir, "archive.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("arxiv-metadata-oai-snapshot.json")
	w.Write([]byte(body))
	zw.Close()
	f.Close()
	return p
}

func TestSnapshotFiltersLocally(t *testing.T) {
	lines := []string{
		snapshotLine("2601.00001", "A fighter jet study", "eess.SY cs.RO", "Mon, 5 Jan 2026 10:00:00 GMT"),
		snapshotLine("2501.00002", "Jetty flows", "eess.SY", "Sun, 5 Jan 2025 10:00:00 GMT"),
		snapshotLine("2502.00003", "Jet engines", "physics.flu-dyn", "Wed, 5 Feb 2025 10:00:00 GMT"),
		snapshotLine("2503.00004", "Jet noise", "cs.SD", "Wed, 5 Mar 2025 10:00:00 GMT"),
		snapshotLine("1901.00005", "Old jet", "eess.SY", "Sat, 5 Jan 2019 10:00:00 GMT"),
		"{not json",
	}
	for _, zipped := range []bool{false, true} {
		t.Run(fmt.Sprintf("zipped=%v", zipped), func(t *testing.T) {
			s := &Snapshot{Path: writeSnapshot(t, zipped, lines...)}

			got, err := s.Harvest(context.Background(), Query{
				Keywords: []string{"jet"}, Categories: []string{"eess", "physics.flu-dyn"},
				From: 2025, To: 2026, Max: 10,
			}, nil)

			if err != nil {
				t.Fatalf("Harvest: %v", err)
			}
			// jetty is not the word jet; cs.SD is out of category; 2019 is out of range.
			if want := []string{"2601.00001", "2502.00003"}; !slices.Equal(ids(got), want) {
				t.Fatalf("ids = %v, want %v", ids(got), want)
			}
			if got[0].Authors[0] != "Ada Lovelace" || got[0].Categories[0] != "eess.SY" || got[1].Published.Month() != time.February {
				t.Errorf("fields not mapped: %+v", got[0])
			}
		})
	}
}

func TestLocalSelectSpendsSharesLikeTheNetwork(t *testing.T) {
	sel := newLocalSelect(Query{Keywords: []string{"alpha", "beta"}, From: 2024, To: 2026, Max: 6})
	add := func(id, text string, year, month int) {
		p, _ := newPaper(id, text, text+" "+abstract, nil, []string{"cs.LG"},
			time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), true)
		sel.offer(p)
	}
	// alpha: plenty in 2026, nothing in 2025, one in 2024.
	for m := 1; m <= 5; m++ {
		add(fmt.Sprintf("a26-%d", m), "alpha", 2026, m)
	}
	add("a24", "alpha", 2024, 1)
	// beta: one paper also matching alpha, which alpha keeps first.
	add("both", "alpha beta", 2026, 12)
	add("b25", "beta", 2025, 1)

	got := ids(sel.result())

	// alpha's share is 3: 2026 gets ceil(3/3)=1, 2025 is empty so its slack
	// rolls to 2024, which has only one. The newest 2026 paper wins its slot.
	want := []string{"both", "a24", "b25"}
	if !slices.Equal(got, want) {
		t.Errorf("selection = %v, want %v", got, want)
	}
}

func oaiRecord(id, created, cats string) string {
	return fmt.Sprintf(`<record><header><identifier>oai:arXiv.org:%s</identifier></header><metadata>
		<arXiv xmlns="http://arxiv.org/OAI/arXiv/"><id>%s</id><created>%s</created>
		<authors><author><keyname>Lovelace</keyname><forenames>Ada</forenames></author></authors>
		<title>Flow over a jet</title><categories>%s</categories><abstract>%s</abstract></arXiv>
	</metadata></record>`, id, id, created, cats, abstract)
}

func TestOAIListsSetsAndFiltersLocally(t *testing.T) {
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		requests = append(requests, r.URL.RawQuery)
		w.Write([]byte(`<?xml version="1.0"?><OAI-PMH xmlns="http://www.openarchives.org/OAI/2.0/">`))
		switch {
		case q.Get("set") == "physics:physics:flu-dyn":
			fmt.Fprintf(w, `<ListRecords>%s<resumptionToken cursor="0">tok1</resumptionToken></ListRecords>`,
				oaiRecord("2407.06437", "2026-08-30", "physics.flu-dyn"))
		case q.Get("resumptionToken") == "tok1":
			fmt.Fprintf(w, `<ListRecords>%s<record><header status="deleted"></header></record><resumptionToken/></ListRecords>`,
				oaiRecord("1901.00001", "2019-01-01", "physics.flu-dyn"))
		case q.Get("set") == "cs:cs:RO":
			w.Write([]byte(`<error code="noRecordsMatch">nothing</error>`))
		default:
			t.Errorf("unexpected request %q", r.URL.RawQuery)
		}
		w.Write([]byte(`</OAI-PMH>`))
	}))
	t.Cleanup(srv.Close)
	o := &OAI{c: testTransport(srv), BaseURL: srv.URL}

	if _, err := o.Harvest(context.Background(), Query{Keywords: []string{"jet"}}, nil); err == nil {
		t.Fatal("expected an error without categories")
	}
	got, err := o.Harvest(context.Background(), Query{
		Keywords: []string{"jet"}, Categories: []string{"physics.flu-dyn", "cs.RO"}, From: 2020, Max: 10,
	}, nil)

	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	// The 2019 record is outside From; its id, not <created>, dates it.
	if want := []string{"2407.06437"}; !slices.Equal(ids(got), want) {
		t.Fatalf("ids = %v, want %v", ids(got), want)
	}
	if got[0].Year() != 2024 || got[0].Authors[0] != "Ada Lovelace" {
		t.Errorf("fields not mapped: %+v", got[0])
	}
	if len(requests) != 3 || !strings.Contains(requests[0], "from=2020-01-01") {
		t.Errorf("requests = %v", requests)
	}
}

func TestHelpers(t *testing.T) {
	for cat, want := range map[string]string{
		"cs.LG": "cs:cs:LG", "physics.flu-dyn": "physics:physics:flu-dyn", "astro-ph.GA": "physics:astro-ph:GA",
		"hep-ph": "physics:hep-ph", "q-bio.BM": "q-bio:q-bio:BM", "stat": "stat:stat",
	} {
		if got := oaiSet(cat); got != want {
			t.Errorf("oaiSet(%q) = %q, want %q", cat, got, want)
		}
	}
	for id, want := range map[string]string{
		"2407.06437": "2024-07", "hep-ph/9901001": "1999-01", "math/0701001": "2007-01", "junk": "0001-01",
	} {
		if got := arxivIDDate(id).Format("2006-01"); got != want {
			t.Errorf("arxivIDDate(%q) = %s, want %s", id, got, want)
		}
	}
	for _, tc := range []struct {
		text, phrase string
		want         bool
	}{
		{"a jet engine", "jet", true}, {"jetty", "jet", false}, {"the jetty and a jet", "jet", true},
		{"fighter jet.", "fighter jet", true}, {"jet2", "jet", false}, {"über jet", "jet", true},
	} {
		if got := containsWord(tc.text, tc.phrase); got != tc.want {
			t.Errorf("containsWord(%q, %q) = %v, want %v", tc.text, tc.phrase, got, tc.want)
		}
	}
}

func TestNetworkErrorsDoNotLeakKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	httpc := srv.Client()
	httpc.Timeout = 50 * time.Millisecond
	o := &OpenAlex{c: &Client{HTTP: httpc}, Key: "sekrit-key", BaseURL: srv.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_, err := o.Harvest(ctx, Query{Keywords: []string{"radar"}, Max: 1}, nil)

	if err == nil {
		t.Fatal("expected the stalled request to fail")
	}
	if strings.Contains(err.Error(), "sekrit-key") {
		t.Errorf("error leaks the API key: %v", err)
	}
	if !strings.Contains(err.Error(), "api_key=REDACTED") {
		t.Errorf("expected the redacted request URL in the error, got: %v", err)
	}
}

func TestKeepDedupesTitlesButNotEmptySlugs(t *testing.T) {
	mk := func(id, title string) paper.Paper { return paper.Paper{ID: id, Title: title} }
	page := []paper.Paper{
		mk("W1", "Fighter Jets!"), mk("W2", "fighter jets"),
		mk("W3", "战斗机的空气动力学"), mk("W4", "高超音速飞行器"),
	}

	got, kept := keep(nil, map[string]struct{}{}, 10, page)

	if want := []string{"W1", "W3", "W4"}; !slices.Equal(ids(got), want) || kept != 3 {
		t.Errorf("kept %v (%d), want %v", ids(got), kept, want)
	}
}

func TestOpenPrefersOptionKeysOverEnvironment(t *testing.T) {
	t.Setenv("OPENALEX_API_KEY", "from-env")
	t.Setenv("S2_API_KEY", "s2-env")

	oa, _ := Open(Options{Name: SourceOpenAlex, OpenAlexKey: " typed "})
	s2, _ := Open(Options{Name: SourceSemanticScholar})

	if got := oa.(*OpenAlex).Key; got != "typed" {
		t.Errorf("openalex key = %q, want the option to win, trimmed", got)
	}
	if got := s2.(*SemanticScholar).Key; got != "s2-env" {
		t.Errorf("semantic scholar key = %q, want the environment fallback", got)
	}
}
