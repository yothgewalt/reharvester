// Package index builds the lexical and dense representations of a corpus.
package index

import (
	"strings"
	"unicode"
)

// Runs splits text into maximal runs of adjacent content tokens.
//
// The discipline matters, and the paper reports why: an extractor that strips
// stopwords and then forms bigrams over the surviving stream invents phrases
// that never occurred. Joining "...models." to "LLMs have..." across a sentence
// boundary produced "models llms" as the corpus's fastest-rising bigram. So the
// text is cut on sentence and clause punctuation first, and a stopword ends the
// run it sits in rather than being silently closed over. An n-gram may only be
// formed from tokens that were genuinely adjacent in the source.
func Runs(text string) [][]string {
	text = stripLaTeX(text)
	var (
		runs [][]string
		cur  []string
		tok  []rune
	)
	flushTok := func() bool {
		if len(tok) == 0 {
			return true
		}
		w := strings.Trim(string(tok), "-")
		tok = tok[:0]
		if w == "" || len(w) < 2 || isStopword(w) || isLaTeXWord(w) || allDigits(w) {
			return false // a dropped token breaks the run
		}
		cur = append(cur, w)
		return true
	}
	breakRun := func() {
		if len(cur) > 0 {
			runs = append(runs, cur)
			cur = nil
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			tok = append(tok, unicode.ToLower(r))
		case r == '\'' || r == '’':
			// Elide apostrophes so "user's" indexes as "users".
		case r == '-' || r == '_':
			// Intra-word: "retrieval-augmented generation" must stay a bigram of
			// two tokens, not four.
			if len(tok) > 0 {
				tok = append(tok, '-')
			}
		case isClauseBreak(r):
			flushTok()
			breakRun()
		default:
			if !flushTok() {
				breakRun()
			}
		}
	}
	if !flushTok() {
		breakRun()
	}
	breakRun()
	return runs
}

// Terms returns every n-gram of length 1..maxN formed within a run. Multi-word
// grams are joined by a single space, matching how the paper reports them
// ("graph neural", "large language").
func Terms(text string, maxN int) []string {
	if maxN < 1 {
		maxN = 1
	}
	runs := Runs(text)
	out := make([]string, 0, 16)
	for _, run := range runs {
		for n := 1; n <= maxN; n++ {
			for i := 0; i+n <= len(run); i++ {
				if n == 1 {
					out = append(out, run[i])
				} else {
					out = append(out, strings.Join(run[i:i+n], " "))
				}
			}
		}
	}
	return out
}

// Unigrams is the tokenisation BM25 and the T0 inverted index use.
func Unigrams(text string) []string { return Terms(text, 1) }

func isClauseBreak(r rune) bool {
	switch r {
	case '.', '!', '?', ';', ':', ',', '(', ')', '[', ']', '{', '}',
		'"', '“', '”', '—', '–', '/', '\\', '|', '\n', '\r':
		return true
	}
	return false
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// stripLaTeX removes backslash control sequences and the math delimiters that
// survive into arXiv abstracts. Without this, \textbf and friends are indexed as
// ordinary words; the paper found textbf peaking in 2026 with burst weight 16.4.
func stripLaTeX(s string) string {
	if !strings.ContainsAny(s, "\\$") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\\':
			i++
			for i < len(s) && (s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z') {
				i++
			}
			b.WriteByte(' ')
		case c == '$':
			i++
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}
