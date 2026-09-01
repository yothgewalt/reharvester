package paper

import "testing"

// Cases checked against the JS rule they mirror:
// label.trim().toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-|-$/g,"")
func TestSlug(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Attention Is All You Need", "attention-is-all-you-need"},
		{"  Padded  Title  ", "padded-title"},
		{"LoRA: Low-Rank Adaptation of LLMs", "lora-low-rank-adaptation-of-llms"},
		{"BM25 (Okapi) & TF-IDF", "bm25-okapi-tf-idf"},
		{"---leading and trailing---", "leading-and-trailing"},
		{"Naïve Bayes", "na-ve-bayes"},
		{"!!!", ""},
		{"", ""},
		{"2013--2026", "2013-2026"},
	} {
		if got := Slug(tc.in); got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripVersion(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"2301.12345v2", "2301.12345"},
		{"2301.12345", "2301.12345"},
		{"http://arxiv.org/abs/2301.12345v11", "2301.12345"},
		{"cs/0501001v1", "cs/0501001"},
		{"1234v", "1234v"}, // trailing v with no digits is not a version
	} {
		if got := StripVersion(tc.in); got != tc.want {
			t.Errorf("StripVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
