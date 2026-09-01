package index

import "strings"

// stopwords is the standard English list, plus the discourse verbs and hedges
// that dominate scientific abstracts. The paper's specificity filter exists
// because terms like "underexplored" and "findings highlight" rise with writing
// convention rather than with any topic; this list handles the purely
// functional ones so the entropy filter only has to catch the rest.
var stopwords = toSet(`
a about above after again against all also am an and any are aren as at
be because been before being below between both but by
can cannot could couldn
did didn do does doesn doing don down during
each either else
few for from further
had hadn has hasn have haven having he her here hers herself him himself his how however
i if in into is isn it its itself
just
me more most much must my myself
no nor not now
of off on once only or other others ought our ours ourselves out over own
same shan she should shouldn so some such
than that the their theirs them themselves then there therefore these they this those through thus to too
under until up upon us
very
was wasn we were weren what when where whether which while who whom why will with within without won would wouldn
you your yours yourself yourselves
one two three
across along among around behind beside besides beyond
however moreover furthermore additionally notably particularly respectively
whereas thereby hence accordingly consequently nevertheless nonetheless
towards toward per eg ie etc et al
via using use used uses useful
show shows shown showing
present presents presented presenting
propose proposes proposed proposing
paper papers work works
result results
approach approaches
method methods
based
we
`)

// latexWords are control words that reach the token stream with their backslash
// already stripped by the source, so stripLaTeX cannot catch them.
var latexWords = toSet(`
textbf textit texttt emph textrm textsf mathbb mathcal mathbf mathrm mathit
begin end item label ref cite citep citet eqref
frac sqrt sum prod int lim log exp min max argmin argmax
alpha beta gamma delta epsilon theta lambda mu sigma tau phi psi omega
cdot times leq geq neq approx sim infty
left right quad qquad hspace vspace newline linebreak
section subsection subsubsection paragraph footnote
`)

func isStopword(w string) bool  { _, ok := stopwords[w]; return ok }
func isLaTeXWord(w string) bool { _, ok := latexWords[w]; return ok }

func toSet(s string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, w := range strings.Fields(s) {
		m[w] = struct{}{}
	}
	return m
}
