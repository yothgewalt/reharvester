package httpapi

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/yothgewalt/reharvester/internal/harvest"
)

func TestResolveHarvest(t *testing.T) {
	def := harvestDefaults{Source: "arxiv", Max: 2000, HasSnapshotPath: false}
	const year = 2026

	cases := []struct {
		name    string
		opts    harvestOptions
		def     harvestDefaults
		wantErr string // key that must be present in errs; "" means success
		check   func(t *testing.T, q harvest.Query, source, label string)
	}{
		{"all blank uses defaults", harvestOptions{}, def, "",
			func(t *testing.T, q harvest.Query, source, label string) {
				if source != "arxiv" || q.From != year-7 || q.To != year || q.Max != 2000 || label != "" {
					t.Errorf("got source=%q from=%d to=%d max=%d label=%q", source, q.From, q.To, q.Max, label)
				}
			}},
		{"explicit source normalised", harvestOptions{Source: " OpenAlex "}, def, "",
			func(t *testing.T, q harvest.Query, source, label string) {
				if source != "openalex" {
					t.Errorf("source = %q", source)
				}
			}},
		{"unknown source", harvestOptions{Source: "scopus"}, def, "source", nil},
		{"kaggle without snapshot path", harvestOptions{Source: "kaggle"}, def, "source", nil},
		{"kaggle with snapshot path ok", harvestOptions{Source: "kaggle"},
			harvestDefaults{Source: "arxiv", Max: 2000, HasSnapshotPath: true}, "", nil},
		{"oai without categories", harvestOptions{Source: "oai"}, def, "categories", nil},
		{"oai with categories ok", harvestOptions{Source: "oai", Categories: []string{"cs.AI"}}, def, "", nil},
		{"too many categories", harvestOptions{Categories: make([]string, 17)}, def, "categories", nil},
		{"name too long", harvestOptions{Name: string(make([]byte, 121))}, def, "name", nil},
		{"name within limit", harvestOptions{Name: "quantum computing"}, def, "",
			func(t *testing.T, q harvest.Query, source, label string) {
				if label != "quantum computing" {
					t.Errorf("label = %q", label)
				}
			}},
		{"from after to", harvestOptions{From: 2024, To: 2020}, def, "from", nil},
		{"explicit from/to kept", harvestOptions{From: 2015, To: 2018}, def, "",
			func(t *testing.T, q harvest.Query, source, label string) {
				if q.From != 2015 || q.To != 2018 {
					t.Errorf("from/to = %d/%d", q.From, q.To)
				}
			}},
		{"negative max errors", harvestOptions{Max: -1}, def, "max", nil},
		{"zero max uses default", harvestOptions{Max: 0}, def, "",
			func(t *testing.T, q harvest.Query, source, label string) {
				if q.Max != 2000 {
					t.Errorf("max = %d, want default 2000", q.Max)
				}
			}},
		{"huge max is capped", harvestOptions{Max: 999999}, def, "",
			func(t *testing.T, q harvest.Query, source, label string) {
				if q.Max != harvestMaxCap {
					t.Errorf("max = %d, want capped at %d", q.Max, harvestMaxCap)
				}
			}},
		{"categories pass through when source infers", harvestOptions{Categories: []string{"cs.AI", "cs.LG"}}, def, "",
			func(t *testing.T, q harvest.Query, source, label string) {
				if !reflect.DeepEqual(q.Categories, []string{"cs.AI", "cs.LG"}) {
					t.Errorf("categories = %v", q.Categories)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, source, label, errs := resolveHarvest(tc.opts, tc.def, year)
			if tc.wantErr != "" {
				if errs[tc.wantErr] == "" {
					t.Fatalf("errs = %v, want a message for %q", errs, tc.wantErr)
				}
				return
			}
			if len(errs) != 0 {
				t.Fatalf("unexpected errs: %v", errs)
			}
			if tc.check != nil {
				tc.check(t, q, source, label)
			}
		})
	}
}

func TestOptionsFromForm(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		want    harvestOptions
		wantErr string
	}{
		{"empty form is all defaults", url.Values{}, harvestOptions{}, ""},
		{"parses every field", url.Values{
			"name":       {" my project "},
			"source":     {" openalex "},
			"categories": {"cs.AI, cs.LG ,, stat.ML"},
			"from":       {"2015"},
			"to":         {"2020"},
			"max":        {"500"},
		}, harvestOptions{
			Name: "my project", Source: "openalex",
			Categories: []string{"cs.AI", "cs.LG", "stat.ML"},
			From:       2015, To: 2020, Max: 500,
		}, ""},
		{"blank numeric fields default to zero", url.Values{
			"from": {""}, "to": {""}, "max": {""},
		}, harvestOptions{}, ""},
		{"malformed from errors", url.Values{"from": {"soon"}}, harvestOptions{}, "from"},
		{"malformed max errors", url.Values{"max": {"lots"}}, harvestOptions{}, "max"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, errs := optionsFromForm(tc.form)
			if tc.wantErr != "" {
				if errs[tc.wantErr] == "" {
					t.Fatalf("errs = %v, want a message for %q", errs, tc.wantErr)
				}
				return
			}
			if len(errs) != 0 {
				t.Fatalf("unexpected errs: %v", errs)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
