package harvest

import (
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yothgewalt/reharvester/internal/paper"
)

// localSelect picks from a full stream of records — the Kaggle snapshot, an
// OAI-PMH category listing — spending Query.Max the way splitHarvest does over
// the network: an equal share per keyword, then a per-year quota with slack
// passed to the next year, newest records first, deduplicated across keywords.
//
// A record matching several keywords counts for the first. Buckets are
// trimmed before that dedupe, so a later keyword can fall short of its share
// when its newest matches were claimed by an earlier one.
//
// A keyword matches as a case-insensitive whole-word phrase in the title or
// abstract; a category matches exactly, or as an archive prefix ("cs" matches
// "cs.LG").
type localSelect struct {
	kws      []string
	cats     []string
	from, to int
	share    int
	buckets  map[localBucket][]paper.Paper
	// Matched counts records that passed every filter, before the quota.
	Matched int
}

type localBucket struct{ kw, year int }

func newLocalSelect(q Query) *localSelect {
	max := q.Max
	if max <= 0 {
		max = DefaultMax
	}
	kws := nonEmpty(q.Keywords)
	for i := range kws {
		kws[i] = strings.ToLower(kws[i])
	}
	if len(kws) == 0 {
		kws = []string{""}
	}
	return &localSelect{
		kws: kws, cats: nonEmpty(q.Categories), from: q.From, to: q.To,
		share:   (max + len(kws) - 1) / len(kws),
		buckets: map[localBucket][]paper.Paper{},
	}
}

func (s *localSelect) byYear() bool { return s.from > 0 && s.to > s.from }

func (s *localSelect) offer(p paper.Paper) {
	y := p.Year()
	if (s.from > 0 && y < s.from) || (s.to > 0 && y > s.to) || !s.inCategories(p.Categories) {
		return
	}
	text := ""
	if s.kws[0] != "" {
		text = strings.ToLower(p.Title + " " + p.Abstract)
	}
	matched := false
	for i, kw := range s.kws {
		if kw != "" && !containsWord(text, kw) {
			continue
		}
		matched = true
		b := localBucket{kw: i}
		if s.byYear() {
			b.year = y
		}
		// A bucket never needs more than one share: that is all a keyword can
		// spend even when every other year is empty. Trim lazily at twice that.
		got := append(s.buckets[b], p)
		if len(got) > 2*s.share {
			got = newestFirst(got)[:s.share]
		}
		s.buckets[b] = got
	}
	if matched {
		s.Matched++
	}
}

func (s *localSelect) inCategories(cats []string) bool {
	if len(s.cats) == 0 {
		return true
	}
	for _, want := range s.cats {
		for _, c := range cats {
			if c == want || strings.HasPrefix(c, want+".") {
				return true
			}
		}
	}
	return false
}

func (s *localSelect) result() []paper.Paper {
	seen := map[string]struct{}{}
	var out []paper.Paper
	for i := range s.kws {
		var mine []paper.Paper
		if !s.byYear() {
			mine, _ = keep(nil, seen, s.share, newestFirst(s.buckets[localBucket{kw: i}]))
		} else {
			for y := s.to; y >= s.from && len(mine) < s.share; y-- {
				quota := len(mine) + yearQuota(s.share-len(mine), y-s.from+1)
				mine, _ = keep(mine, seen, quota, newestFirst(s.buckets[localBucket{kw: i, year: y}]))
			}
		}
		out = append(out, mine...)
	}
	return out
}

func newestFirst(ps []paper.Paper) []paper.Paper {
	sort.Slice(ps, func(i, j int) bool {
		if !ps[i].Published.Equal(ps[j].Published) {
			return ps[i].Published.After(ps[j].Published)
		}
		return ps[i].ID > ps[j].ID
	})
	return ps
}

// containsWord reports whether phrase occurs in text with no letter or digit
// directly on either side, so "jet" matches "a jet engine" but not "jetty".
// Both must already be lower case.
func containsWord(text, phrase string) bool {
	for off := 0; ; {
		i := strings.Index(text[off:], phrase)
		if i < 0 {
			return false
		}
		start, end := off+i, off+i+len(phrase)
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		after, _ := utf8.DecodeRuneInString(text[end:])
		if (start == 0 || !isWordRune(before)) && (end == len(text) || !isWordRune(after)) {
			return true
		}
		off = start + 1
	}
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// arxivIDDate is the submission month encoded in an arXiv identifier: YYMM for
// new-style ids (2407.06437) and for old-style ones after the slash
// (hep-ph/0701001). The zero time means the id carries none.
func arxivIDDate(id string) time.Time {
	if _, after, ok := strings.Cut(id, "/"); ok {
		id = after
	}
	if len(id) < 4 {
		return time.Time{}
	}
	yy, mm := atoi2(id[0:2]), atoi2(id[2:4])
	if yy < 0 || mm < 1 || mm > 12 {
		return time.Time{}
	}
	year := 2000 + yy
	if yy >= 91 {
		year = 1900 + yy
	}
	return time.Date(year, time.Month(mm), 1, 0, 0, 0, 0, time.UTC)
}

func atoi2(s string) int {
	if s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return -1
	}
	return int(s[0]-'0')*10 + int(s[1]-'0')
}
