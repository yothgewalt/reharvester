package app

import (
	"context"
	"strings"
	"testing"

	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/store"
)

func TestHarvestRefusesAnUnfilteredQuery(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Categories used to default to a fixed list, so a blank one was harmless.
	// Now that blank means "infer from the keywords", blank-and-no-keywords
	// would ask arXiv for everything it has.
	err = Harvest(context.Background(), st, "p", harvest.Query{From: 2026, To: 2026, Max: 10}, 0)
	if err == nil {
		t.Fatal("expected an error when neither categories nor keywords are given")
	}
	if !strings.Contains(err.Error(), "at least one category or keyword") {
		t.Errorf("error should say what is missing, got %q", err)
	}
}
