// Package paper holds the one record type every stage of the pipeline passes
// around, plus the slug rule that ties a paper to its identity in the web UI.
package paper

import (
	"strings"
	"time"
)

// Paper is one bibliographic record. ID is the version-stripped arXiv
// identifier (2301.12345v2 -> 2301.12345) and is what deduplication keys on.
// Categories[0] is the primary category; Task B relevance grading depends on
// that ordering being preserved from the source.
type Paper struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Abstract   string    `json:"abstract"`
	Authors    []string  `json:"authors"`
	Categories []string  `json:"categories"`
	Published  time.Time `json:"published"`
	OpenAccess bool      `json:"openAccess"`
}

func (p Paper) Year() int { return p.Published.Year() }

// Slug is the node id, the docId, and the wiki path for a paper, derived from
// its title. It mirrors the frontend rule in web/src/components/corpus/WikiPane.tsx
//
//	label.trim().toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-|-$/g,"")
//
// exactly. The UI resolves [[wikilinks]] by slugging the link text and matching
// it against graph node ids, so any divergence here silently turns every wiki
// link into dead text and makes GET /api/v1/wiki/raw/{docId} miss.
func Slug(label string) string {
	var b strings.Builder
	b.Grow(len(label))
	dash := false
	for _, r := range strings.TrimSpace(label) {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			dash = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		default:
			// [^a-z0-9]+ collapses to a single "-", so a run never yields "--".
			if !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	// Runs are already collapsed, so trimming one dash per end matches the
	// JS /^-|-$/g, which can only ever fire once at each end.
	return strings.TrimSuffix(strings.TrimPrefix(b.String(), "-"), "-")
}

// StripVersion turns an arXiv id or abs URL into the deduplication key.
func StripVersion(id string) string {
	if i := strings.LastIndex(id, "/abs/"); i >= 0 {
		id = id[i+len("/abs/"):]
	}
	if i := strings.LastIndexByte(id, 'v'); i > 0 {
		if rest := id[i+1:]; rest != "" && strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' }) < 0 {
			return id[:i]
		}
	}
	return id
}
