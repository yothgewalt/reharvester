package httpapi

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// The client allows this request 20 s, so generation gets a little less.
const keywordsTimeout = 18 * time.Second

const (
	keywordsWant     = 5
	keywordsMaxWords = 6
	keywordsMaxField = 80
)

const keywordsSystem = "You suggest literature-search keywords for arXiv. " +
	"Reply with exactly 5 comma-separated keywords on one line and nothing else. " +
	"Each keyword is 1 to 4 words, lowercase, and all 5 belong to one coherent research topic."

var keywordNoise = regexp.MustCompile("^(?:\\d+[.)]|[-*•])?\\s*[\"'`]*|[\"'`.]+$")

// randomKeywords answers {"keywords": [...]} for ?field=. An empty list means
// no model could produce them and the caller should use its own; it is never a
// 5xx, because the client treats those as the backend being down.
func (s *Server) randomKeywords(w http.ResponseWriter, r *http.Request) {
	resp := struct {
		Keywords []string `json:"keywords"`
	}{Keywords: []string{}}

	field := strings.TrimSpace(r.URL.Query().Get("field"))
	if len(field) > keywordsMaxField {
		field = field[:keywordsMaxField]
	}
	llm := s.cfgNow().llm
	if llm == nil || field == "" {
		writeJSON(w, resp)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), keywordsTimeout)
	defer cancel()
	prompt := "Pick one specific, less obvious subtopic within " + field + " and give its 5 keywords."
	if out, err := llm.Generate(ctx, keywordsSystem, prompt); err == nil {
		resp.Keywords = parseKeywords(out)
	}
	writeJSON(w, resp)
}

// parseKeywords pulls exactly keywordsWant usable keywords out of model output,
// or returns an empty slice when there are not enough.
func parseKeywords(out string) []string {
	seen := map[string]bool{}
	var kws []string
	for _, part := range strings.FieldsFunc(out, func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
		if i := strings.LastIndex(part, ":"); i >= 0 {
			part = part[i+1:]
		}
		kw := strings.ToLower(keywordNoise.ReplaceAllString(strings.TrimSpace(part), ""))
		n := len(strings.Fields(kw))
		if n == 0 || n > keywordsMaxWords || seen[kw] {
			continue
		}
		seen[kw] = true
		kws = append(kws, kw)
		if len(kws) == keywordsWant {
			return kws
		}
	}
	return []string{}
}
