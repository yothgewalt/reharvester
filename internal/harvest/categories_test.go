package harvest

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestInferCategoriesWeighsEveryKeywordEqually(t *testing.T) {
	// "radar" matches two orders of magnitude more records than "fighter jet".
	// Raw counts would let it pick the scope on its own; the point of
	// normalising per keyword is that the rare term still gets a vote.
	c := newFakeArxiv(t,
		map[string]int{"radar": 7100, "fighter jet": 19},
		map[string]string{"radar": "eess.SP", "fighter jet": "eess.SY"})
	c.Delay = 0

	cats, profiles, err := c.InferCategories(context.Background(), []string{"radar", "fighter jet"})
	if err != nil {
		t.Fatalf("InferCategories: %v", err)
	}
	if !slices.Contains(cats, "eess.SP") || !slices.Contains(cats, "eess.SY") {
		t.Errorf("categories = %v, want both eess.SP and eess.SY", cats)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want one per keyword", len(profiles))
	}
	if profiles[0].Total != 7100 || profiles[1].Total != 19 {
		t.Errorf("profiles carry the wrong arXiv totals: %+v", profiles)
	}
}

func TestInferCategoriesDropsTheCsLockThatCausedOffTopicCorpora(t *testing.T) {
	// The reported bug: an aerospace query scoped to cs.* returned language
	// models. Inference must land on the aerospace categories instead.
	c := newFakeArxiv(t,
		map[string]int{"aerodynamic": 1996, "fighter jet": 19, "missiles": 150},
		map[string]string{
			"aerodynamic": "physics.flu-dyn", "fighter jet": "eess.SY", "missiles": "eess.SY",
		})
	c.Delay = 0

	cats, _, err := c.InferCategories(context.Background(),
		[]string{"aerodynamic", "fighter jet", "missiles"})
	if err != nil {
		t.Fatalf("InferCategories: %v", err)
	}
	// eess.SY is named by two of three keywords, so it outranks the other.
	if len(cats) == 0 || cats[0] != "eess.SY" {
		t.Errorf("categories = %v, want eess.SY first", cats)
	}
	for _, c := range cats {
		if strings.HasPrefix(c, "cs.") {
			t.Errorf("categories = %v, must not include a computer-science category", cats)
		}
	}
}

func TestInferCategoriesRecordsADeadKeyword(t *testing.T) {
	c := newFakeArxiv(t,
		map[string]int{"aerodynamic": 1996, "fighter jet": 0},
		map[string]string{"aerodynamic": "physics.flu-dyn"})
	c.Delay = 0

	_, profiles, err := c.InferCategories(context.Background(), []string{"aerodynamic", "fighter jet"})
	if err != nil {
		t.Fatalf("InferCategories: %v", err)
	}
	dead := profiles[1]
	if dead.Keyword != "fighter jet" || dead.Total != 0 || len(dead.Categories) != 0 {
		t.Errorf("dead keyword profile = %+v, want an empty profile for \"fighter jet\"", dead)
	}
}

func TestInferCategoriesFailsWhenNothingMatches(t *testing.T) {
	c := newFakeArxiv(t, map[string]int{"zzzz": 0, "qqqq": 0}, nil)
	c.Delay = 0

	_, _, err := c.InferCategories(context.Background(), []string{"zzzz", "qqqq"})
	if err == nil {
		t.Fatal("expected an error when no keyword matches anything")
	}
	if !strings.Contains(err.Error(), "zzzz") {
		t.Errorf("error should name the keywords, got %q", err)
	}
}

func TestInferCategoriesDeclinesToScopeOnThinEvidence(t *testing.T) {
	// A single category is not a scope worth imposing. Returning nothing sends
	// the harvest to all of arXiv, where the keyword still does the filtering.
	c := newFakeArxiv(t,
		map[string]int{"radar": 7100},
		map[string]string{"radar": "eess.SP"})
	c.Delay = 0

	cats, profiles, err := c.InferCategories(context.Background(), []string{"radar"})
	if err != nil {
		t.Fatalf("InferCategories: %v", err)
	}
	if len(cats) != 0 {
		t.Errorf("categories = %v, want none when only one category is evidenced", cats)
	}
	if len(profiles) != 1 {
		t.Errorf("got %d profiles, want 1", len(profiles))
	}
}

func TestInferCategoriesIgnoresBlankKeywords(t *testing.T) {
	c := newFakeArxiv(t, map[string]int{"radar": 7100}, nil)
	c.Delay = 0

	cats, profiles, err := c.InferCategories(context.Background(), []string{"  ", ""})
	if err != nil || cats != nil || profiles != nil {
		t.Errorf("blank keywords should yield (nil, nil, nil), got (%v, %v, %v)", cats, profiles, err)
	}
}
