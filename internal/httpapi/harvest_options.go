package httpapi

import (
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/yothgewalt/reharvester/internal/harvest"
)

// harvestMaxCap bounds a requested max the same way settings.ValidMax bounds
// the configured one; a request over it is clamped rather than rejected.
const harvestMaxCap = 50000

// harvestNameLimit and harvestCategoriesLimit bound the two fields with no
// natural numeric range.
const (
	harvestNameLimit       = 120
	harvestCategoriesLimit = 16
)

// harvestOptions are the fields POST /api/v1/harvest/init accepts alongside
// keywords/abstract/pdf. A zero value in From, To or Max, or a blank Source,
// means "use the default".
type harvestOptions struct {
	Name       string
	Source     string
	Categories []string
	From       int
	To         int
	Max        int
}

// harvestDefaults are what resolveHarvest falls back to for a blank option.
type harvestDefaults struct {
	Source          string
	Max             int
	HasSnapshotPath bool
}

// resolveHarvest fills in opts against def and validates the result. year
// anchors From/To when both are blank (year-7..year). errs is keyed by the
// same field names the JSON and multipart request bodies use, so a handler
// can return it directly as the response's "errors" object.
func resolveHarvest(opts harvestOptions, def harvestDefaults, year int) (q harvest.Query, source, name string, errs map[string]string) {
	errs = map[string]string{}

	source = strings.ToLower(strings.TrimSpace(opts.Source))
	if source == "" {
		source = def.Source
	} else if !slices.Contains(harvest.Sources, source) {
		errs["source"] = "must be one of " + strings.Join(harvest.Sources, ", ")
	}
	if source == harvest.SourceKaggle && !def.HasSnapshotPath {
		errs["source"] = "the kaggle source needs an arXiv snapshot path — set one in Settings"
	}

	name = strings.TrimSpace(opts.Name)
	if len(name) > harvestNameLimit {
		errs["name"] = "must be 120 characters or fewer"
	}

	cats := opts.Categories
	switch {
	case len(cats) > harvestCategoriesLimit:
		errs["categories"] = "at most 16 categories"
	case source == harvest.SourceOAI && len(cats) == 0:
		errs["categories"] = "the oai source needs at least one category"
	}

	from, to := opts.From, opts.To
	if from == 0 {
		from = year - 7
	}
	if to == 0 {
		to = year
	}
	if from > to {
		errs["from"] = "from year must not be after to year"
	}

	max := opts.Max
	switch {
	case max < 0:
		errs["max"] = "must not be negative"
	case max == 0:
		max = def.Max
	case max > harvestMaxCap:
		max = harvestMaxCap
	}

	if len(errs) > 0 {
		return harvest.Query{}, "", "", errs
	}
	return harvest.Query{Categories: cats, From: from, To: to, Max: max}, source, name, nil
}

// optionsFromForm reads harvestOptions out of a multipart form's values.
// errs carries a message for any field that could not be parsed as a whole
// number; a blank field never errors, since blank means "use the default".
func optionsFromForm(v url.Values) (harvestOptions, map[string]string) {
	errs := map[string]string{}
	opts := harvestOptions{
		Name:       strings.TrimSpace(v.Get("name")),
		Source:     strings.TrimSpace(v.Get("source")),
		Categories: splitComma(v.Get("categories")),
		From:       parseFormInt(v.Get("from"), "from", errs),
		To:         parseFormInt(v.Get("to"), "to", errs),
		Max:        parseFormInt(v.Get("max"), "max", errs),
	}
	return opts, errs
}

func parseFormInt(s, field string, errs map[string]string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		errs[field] = "must be a whole number"
		return 0
	}
	return n
}
