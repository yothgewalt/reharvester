package harvest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

// Source names accepted by Open.
const (
	SourceArxiv           = "arxiv"
	SourceAuto            = "auto"
	SourceKaggle          = "kaggle"
	SourceSemanticScholar = "semanticscholar"
	SourceOpenAlex        = "openalex"
	SourceOAI             = "oai"
)

// Sources lists every name Open accepts, in the order to offer them.
var Sources = []string{SourceArxiv, SourceAuto, SourceKaggle, SourceSemanticScholar, SourceOpenAlex, SourceOAI}

// Source is anything that turns a Query into records. Harvest returns what it
// collected alongside any error, so callers keep a partial corpus.
//
// Only the arXiv API source and Router also implement CategoryInferrer. Query.Categories
// are arXiv category codes: the Kaggle snapshot and OAI-PMH filter on them,
// Semantic Scholar and OpenAlex ignore them.
type Source interface {
	Harvest(ctx context.Context, q Query, prog Progress) ([]paper.Paper, error)
}

// CategoryInferrer is the optional probe a caller uses to scope a keyword-only
// query to arXiv categories before harvesting. See Client.InferCategories.
type CategoryInferrer interface {
	InferCategories(ctx context.Context, keywords []string) ([]string, []CategoryProfile, error)
}

// Options selects and configures a Source.
type Options struct {
	// Name is one of Sources; empty means arXiv.
	Name string
	// Delay is the politeness delay between requests; zero or less means
	// DefaultDelay. Semantic Scholar never goes below its one-per-second limit.
	Delay time.Duration
	// SnapshotPath is the Kaggle arXiv metadata file, as .json or the .zip
	// Kaggle downloads. Required for SourceKaggle only.
	SnapshotPath string
	// OpenAlexKey and SemanticScholarKey authenticate those sources. Empty
	// falls back to OPENALEX_API_KEY and S2_API_KEY; both are optional.
	OpenAlexKey        string
	SemanticScholarKey string
}

// Open builds the named source. API keys come from Options, else from the
// environment: S2_API_KEY for Semantic Scholar (optional, but keyless calls
// share one crowded pool) and OPENALEX_API_KEY for OpenAlex (optional, keyless
// gets a tenth of the daily budget).
func Open(o Options) (Source, error) {
	delay := o.Delay
	if delay <= 0 {
		delay = DefaultDelay
	}
	transport := func(d time.Duration) *Client {
		c := NewClient()
		c.Delay = d
		return c
	}
	switch strings.ToLower(strings.TrimSpace(o.Name)) {
	case "", SourceArxiv:
		return transport(delay), nil
	case SourceKaggle:
		if strings.TrimSpace(o.SnapshotPath) == "" {
			return nil, fmt.Errorf("the kaggle source needs the path to arxiv-metadata-oai-snapshot.json (or its .zip); download it from https://www.kaggle.com/datasets/Cornell-University/arxiv")
		}
		return &Snapshot{Path: o.SnapshotPath}, nil
	case SourceSemanticScholar:
		return &SemanticScholar{c: transport(max(delay, time.Second)), Key: keyOr(o.SemanticScholarKey, "S2_API_KEY")}, nil
	case SourceOpenAlex:
		return &OpenAlex{c: transport(delay), Key: keyOr(o.OpenAlexKey, "OPENALEX_API_KEY")}, nil
	case SourceOAI:
		return &OAI{c: transport(delay)}, nil
	case SourceAuto:
		return newRouter(delay, o), nil
	}
	return nil, fmt.Errorf("unknown source %q: use one of %s", o.Name, strings.Join(Sources, ", "))
}

func keyOr(key, env string) string {
	if key = strings.TrimSpace(key); key != "" {
		return key
	}
	return strings.TrimSpace(os.Getenv(env))
}

// withKey returns a header carrying key, or nil when there is none.
func withKey(name, key string) http.Header {
	if key == "" {
		return nil
	}
	return http.Header{name: []string{key}}
}
