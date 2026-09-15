package httpapi

import (
	"fmt"
	"testing"

	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/paper"
	"github.com/yothgewalt/reharvester/internal/pipeline"
)

// Only one-way arcs ship as nearest-to: mutual arcs are the similar-to edge,
// and an arc must survive a co-author edge on the same ordered pair.
func TestSnapshotShipsOnlyOneWayNearestArcsInsideTheCut(t *testing.T) {
	papers := make([]paper.Paper, 4)
	for i := range papers {
		papers[i] = paper.Paper{ID: fmt.Sprintf("x.%d", i), Title: fmt.Sprintf("p%d", i)}
	}
	g := &graph.Graph{
		N:         4,
		Edges:     []graph.Edge{{A: 0, B: 1, W: 0.9}},
		Nearest:   []graph.Edge{{A: 0, B: 1, W: 0.9}, {A: 1, B: 0, W: 0.9}, {A: 0, B: 2, W: 0.5}, {A: 3, B: 0, W: 0.4}},
		Coauthor:  []graph.Edge{{A: 0, B: 2, W: 1}},
		Community: []int32{-1, -1, -1, -1},
		PageRank:  []float64{0.4, 0.3, 0.2, 0.1},
		Bridge:    make([]float64, 4),
	}
	g.Index()
	s := BuildSnapshots(&pipeline.Project{Papers: papers, Graph: g}, 3)

	byRel := map[string][]GraphEdge{}
	seen := map[string]bool{}
	for _, e := range s.Knowledge.Edges {
		if seen[e.ID] {
			t.Errorf("duplicate edge id %q", e.ID)
		}
		seen[e.ID] = true
		if e.Source == "p3" || e.Target == "p3" {
			t.Errorf("edge %q touches p3, which is outside the cut", e.ID)
		}
		byRel[e.Relation] = append(byRel[e.Relation], e)
	}

	near := byRel["nearest-to"]
	if len(near) != 1 || near[0].ID != "nearest:p0->p2" || near[0].Source != "p0" || near[0].Target != "p2" {
		t.Errorf("nearest-to = %+v, want exactly p0->p2 with id nearest:p0->p2", near)
	}
	if co := byRel["co-authored-with"]; len(co) != 1 || co[0].ID != "p0->p2" {
		t.Errorf("co-authored-with = %+v, want p0->p2 kept alongside the arc", co)
	}
	if sim := byRel["similar-to"]; len(sim) != 1 || sim[0].ID != "p0->p1" {
		t.Errorf("similar-to = %+v, want p0->p1", sim)
	}
}
